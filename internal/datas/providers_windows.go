package datas

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const psTimeout = 15 * time.Second

// psJSON runs a PowerShell command and unmarshals JSON output.
// Stderr is included in the error message for easier debugging.
func psJSON(script string, dst any) error {
	ctx, cancel := context.WithTimeout(context.Background(), psTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return err
	}
	return json.Unmarshal(out, dst)
}

// psString runs a PowerShell command and returns trimmed string output.
func psString(script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), psTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// psInt runs a PowerShell command and returns an integer.
func psInt(script string) (int, error) {
	s, err := psString(script)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(s)
}

// ----- Services -----

type ServicesProvider struct{}

func (p *ServicesProvider) Name() string { return "services" }
func (p *ServicesProvider) Collect() (any, error) {
	type svcInfo struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Status      int    `json:"status_code"`
		StatusText  string `json:"status"`
		StartType   int    `json:"start_type_code"`
		StartText   string `json:"start_type"`
	}

	type rawSvc struct {
		Name        string `json:"Name"`
		DisplayName string `json:"DisplayName"`
		Status      int    `json:"Status"`
		StartType   int    `json:"StartType"`
	}

	var raw []rawSvc
	err := psJSON(`@(Get-Service) | Select-Object Name,DisplayName,Status,StartType | ConvertTo-Json -Compress`, &raw)
	if err != nil {
		return nil, fmt.Errorf("get services: %w", err)
	}

	result := make([]svcInfo, 0, len(raw))
	for _, s := range raw {
		result = append(result, svcInfo{
			Name:        s.Name,
			DisplayName: s.DisplayName,
			Status:      s.Status,
			StatusText:  statusToString(s.Status),
			StartType:   s.StartType,
			StartText:   startTypeToString(s.StartType),
		})
	}
	return result, nil
}

func statusToString(s int) string {
	switch s {
	case 1:
		return "Stopped"
	case 2:
		return "StartPending"
	case 3:
		return "StopPending"
	case 4:
		return "Running"
	case 5:
		return "ContinuePending"
	case 6:
		return "PausePending"
	case 7:
		return "Paused"
	default:
		return "Unknown"
	}
}

func startTypeToString(s int) string {
	switch s {
	case 0:
		return "Boot"
	case 1:
		return "System"
	case 2:
		return "Automatic"
	case 3:
		return "Manual"
	case 4:
		return "Disabled"
	default:
		return "Unknown"
	}
}

// ----- Packages -----

type PackagesProvider struct{}

func (p *PackagesProvider) Name() string { return "packages" }
func (p *PackagesProvider) Collect() (any, error) {
	type pkgInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	type rawPkg struct {
		DisplayName    string `json:"DisplayName"`
		DisplayVersion string `json:"DisplayVersion"`
	}

	script := `
$paths = @(
  'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
  'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
)
@(Get-ItemProperty $paths -ErrorAction SilentlyContinue |
  Where-Object { $_.DisplayName } |
  Select-Object DisplayName, DisplayVersion |
  Sort-Object DisplayName) |
  ConvertTo-Json -Compress
`
	var raw []rawPkg
	if err := psJSON(script, &raw); err != nil {
		return nil, fmt.Errorf("get packages: %w", err)
	}

	result := make([]pkgInfo, 0, len(raw))
	for _, r := range raw {
		result = append(result, pkgInfo{
			Name:    r.DisplayName,
			Version: r.DisplayVersion,
		})
	}
	return result, nil
}

// ----- Updates -----

type UpdatesProvider struct{}

func (p *UpdatesProvider) Name() string { return "updates" }
func (p *UpdatesProvider) Collect() (any, error) {
	type updInfo struct {
		Title string `json:"title"`
	}

	script := `
$s = New-Object -ComObject Microsoft.Update.Session
$u = $s.CreateUpdateSearcher()
$r = $u.Search("IsInstalled=0")
@($r.Updates | ForEach-Object { [pscustomobject]@{Title=$_.Title} }) | ConvertTo-Json -Compress
`
	var raw []updInfo
	if err := psJSON(script, &raw); err != nil {
		return nil, fmt.Errorf("get updates: %w", err)
	}
	return raw, nil
}

// ----- Disk Health -----

type DiskHealthProvider struct{}

func (p *DiskHealthProvider) Name() string { return "disk_health" }
func (p *DiskHealthProvider) Collect() (any, error) {
	type diskInfo struct {
		DeviceID    string `json:"device_id"`
		Model       string `json:"model"`
		MediaType   int    `json:"media_type_code"`
		MediaText   string `json:"media_type"`
		HealthState string `json:"health_status"`
		Size        uint64 `json:"size_bytes"`
	}

	type rawDisk struct {
		DeviceID     string `json:"DeviceId"`
		FriendlyName string `json:"FriendlyName"`
		MediaType    int    `json:"MediaType"`
		HealthStatus int    `json:"HealthStatus"`
		Size         uint64 `json:"Size"`
	}

	script := `@(Get-PhysicalDisk) | Select-Object DeviceId,FriendlyName,MediaType,HealthStatus,Size | ConvertTo-Json -Compress`
	var raw []rawDisk
	if err := psJSON(script, &raw); err != nil {
		return nil, fmt.Errorf("get disk health: %w", err)
	}

	result := make([]diskInfo, 0, len(raw))
	for _, d := range raw {
		hs := "Unknown"
		switch d.HealthStatus {
		case 0:
			hs = "Healthy"
		case 1:
			hs = "Warning"
		case 2:
			hs = "Unhealthy"
		}
		result = append(result, diskInfo{
			DeviceID:    d.DeviceID,
			Model:       d.FriendlyName,
			MediaType:   d.MediaType,
			MediaText:   mediaTypeToString(d.MediaType),
			HealthState: hs,
			Size:        d.Size,
		})
	}
	return result, nil
}

func mediaTypeToString(t int) string {
	switch t {
	case 3:
		return "HDD"
	case 4:
		return "SSD"
	case 5:
		return "SCM"
	default:
		return "Unspecified"
	}
}

// ----- Hardware -----

type HardwareProvider struct{}

func (p *HardwareProvider) Name() string { return "hardware" }
func (p *HardwareProvider) Collect() (any, error) {
	type hwInfo struct {
		Hostname     string `json:"hostname"`
		CPUModel     string `json:"cpu_model"`
		CPUCores     int    `json:"cpu_cores"`
		TotalMemory  string `json:"total_memory"`
		Manufacturer string `json:"manufacturer"`
		Model        string `json:"model"`
		Serial       string `json:"serial"`
		BiosVendor   string `json:"bios_vendor"`
		BiosVersion  string `json:"bios_version"`
	}

	hostname, _ := psString(`(Get-CimInstance Win32_ComputerSystem).Name`)
	cpuModel, _ := psString(`(Get-CimInstance Win32_Processor | Select-Object -First 1).Name`)
	cpuCores, _ := psInt(`(Get-CimInstance Win32_Processor | Measure-Object -Property NumberOfLogicalProcessors -Sum).Sum`)
	memBytes, _ := psInt(`[math]::Round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory)`)
	manufacturer, _ := psString(`(Get-CimInstance Win32_ComputerSystem).Manufacturer`)
	model, _ := psString(`(Get-CimInstance Win32_ComputerSystem).Model`)
	serial, _ := psString(`(Get-CimInstance Win32_BIOS).SerialNumber`)
	biosVendor, _ := psString(`(Get-CimInstance Win32_BIOS).Manufacturer`)
	biosVersion, _ := psString(`(Get-CimInstance Win32_BIOS).SMBIOSBIOSVersion`)

	totalMem := fmt.Sprintf("%.1f GB", float64(memBytes)/1073741824)

	return hwInfo{
		Hostname:     hostname,
		CPUModel:     cpuModel,
		CPUCores:     cpuCores,
		TotalMemory:  totalMem,
		Manufacturer: manufacturer,
		Model:        model,
		Serial:       serial,
		BiosVendor:   biosVendor,
		BiosVersion:  biosVersion,
	}, nil
}

// ----- OS -----

type OSProvider struct{}

func (p *OSProvider) Name() string { return "os" }
func (p *OSProvider) Collect() (any, error) {
	type osInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Build   string `json:"build"`
		Arch    string `json:"arch"`
	}

	name, _ := psString(`(Get-CimInstance Win32_OperatingSystem).Caption`)
	version, _ := psString(`(Get-CimInstance Win32_OperatingSystem).Version`)
	build, _ := psString(`(Get-CimInstance Win32_OperatingSystem).BuildNumber`)
	arch, _ := psString(`(Get-CimInstance Win32_OperatingSystem).OSArchitecture`)

	return osInfo{
		Name:    strings.TrimSpace(name),
		Version: version,
		Build:   build,
		Arch:    arch,
	}, nil
}

// ----- Ports -----

type PortsProvider struct{}

func (p *PortsProvider) Name() string { return "ports" }
func (p *PortsProvider) Collect() (any, error) {
	type portInfo struct {
		Protocol string `json:"protocol"`
		Address  string `json:"address"`
		Port     int    `json:"port"`
		PID      int    `json:"pid"`
		Process  string `json:"process,omitempty"`
		State    string `json:"state,omitempty"`
	}

	type rawTCP struct {
		LocalAddress string `json:"LocalAddress"`
		LocalPort    int    `json:"LocalPort"`
		OwningPID    int    `json:"OwningProcess"`
		State        int    `json:"State"`
	}

	type rawUDP struct {
		LocalAddress string `json:"LocalAddress"`
		LocalPort    int    `json:"LocalPort"`
		OwningPID    int    `json:"OwningProcess"`
	}

	var tcpRaw []rawTCP
	_ = psJSON(`@(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue) | Select-Object LocalAddress,LocalPort,OwningProcess,State | ConvertTo-Json -Compress`, &tcpRaw)

	var udpRaw []rawUDP
	_ = psJSON(`@(Get-NetUDPEndpoint -ErrorAction SilentlyContinue) | Select-Object LocalAddress,LocalPort,OwningProcess | ConvertTo-Json -Compress`, &udpRaw)

	// Build PID -> process name map
	type rawProc struct {
		Id          int    `json:"Id"`
		ProcessName string `json:"ProcessName"`
	}
	var procs []rawProc
	_ = psJSON(`@(Get-Process) | Select-Object Id,ProcessName | ConvertTo-Json -Compress`, &procs)

	procMap := make(map[int]string)
	for _, pp := range procs {
		procMap[pp.Id] = pp.ProcessName
	}

	var result []portInfo
	for _, t := range tcpRaw {
		result = append(result, portInfo{
			Protocol: "tcp",
			Address:  t.LocalAddress,
			Port:     t.LocalPort,
			PID:      t.OwningPID,
			Process:  procMap[t.OwningPID],
			State:    "LISTEN",
		})
	}
	for _, u := range udpRaw {
		result = append(result, portInfo{
			Protocol: "udp",
			Address:  u.LocalAddress,
			Port:     u.LocalPort,
			PID:      u.OwningPID,
			Process:  procMap[u.OwningPID],
		})
	}
	return result, nil
}
