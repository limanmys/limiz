package datas

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ---- Services ----

type ServicesProvider struct{}

func (p *ServicesProvider) Name() string { return "services" }

type ServiceInfo struct {
	Name        string `json:"name"`
	LoadState   string `json:"load_state"`
	ActiveState string `json:"active_state"`
	SubState    string `json:"sub_state"`
	Description string `json:"description"`
}

func (p *ServicesProvider) Collect() (any, error) {
	out, err := exec.Command("systemctl", "list-units", "--type=service", "--all", "--no-legend", "--no-pager").Output()
	if err != nil {
		return nil, fmt.Errorf("systemctl: %w", err)
	}
	var services []ServiceInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name := strings.TrimSuffix(fields[0], ".service")
		desc := ""
		if len(fields) > 4 {
			desc = strings.Join(fields[4:], " ")
		}
		services = append(services, ServiceInfo{
			Name:        name,
			LoadState:   fields[1],
			ActiveState: fields[2],
			SubState:    fields[3],
			Description: desc,
		})
	}
	return services, nil
}

// ---- Packages ----

type PackagesProvider struct{}

func (p *PackagesProvider) Name() string { return "packages" }

type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (p *PackagesProvider) Collect() (any, error) {
	// Try dpkg first (Debian/Ubuntu)
	if _, err := exec.LookPath("dpkg-query"); err == nil {
		return collectDpkg()
	}
	// Try rpm (RHEL/CentOS/Fedora)
	if _, err := exec.LookPath("rpm"); err == nil {
		return collectRpm()
	}
	return nil, fmt.Errorf("no supported package manager found (dpkg/rpm)")
}

func collectDpkg() ([]PackageInfo, error) {
	out, err := exec.Command("dpkg-query", "-W", "-f", "${Package}\t${Version}\t${Architecture}\n").Output()
	if err != nil {
		return nil, err
	}
	var pkgs []PackageInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		pkgs = append(pkgs, PackageInfo{Name: parts[0], Version: parts[1]})
	}
	return pkgs, nil
}

func collectRpm() ([]PackageInfo, error) {
	out, err := exec.Command("rpm", "-qa", "--queryformat", "%{NAME}\t%{VERSION}-%{RELEASE}\n").Output()
	if err != nil {
		return nil, err
	}
	var pkgs []PackageInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		pkgs = append(pkgs, PackageInfo{Name: parts[0], Version: parts[1]})
	}
	return pkgs, nil
}

// ---- Updates ----

type UpdatesProvider struct{}

func (p *UpdatesProvider) Name() string { return "updates" }

type UpdateInfo struct {
	Name       string `json:"name"`
	CurrentVer string `json:"current_version,omitempty"`
	NewVer     string `json:"new_version"`
}

func (p *UpdatesProvider) Collect() (any, error) {
	// apt-based
	if _, err := exec.LookPath("apt"); err == nil {
		return collectAptUpdates()
	}
	// dnf-based
	if _, err := exec.LookPath("dnf"); err == nil {
		return collectDnfUpdates()
	}
	// yum-based
	if _, err := exec.LookPath("yum"); err == nil {
		return collectYumUpdates()
	}
	return nil, fmt.Errorf("no supported update manager found (apt/dnf/yum)")
}

func collectAptUpdates() ([]UpdateInfo, error) {
	out, err := exec.Command("apt", "list", "--upgradable").Output()
	if err != nil {
		// apt returns exit code 0 but may have stderr, try anyway
		return []UpdateInfo{}, nil
	}
	var updates []UpdateInfo
	for _, line := range strings.Split(string(out), "\n") {
		// Format: "package/source version arch [upgradable from: old_version]"
		if !strings.Contains(line, "[upgradable") {
			continue
		}
		fields := strings.SplitN(line, "/", 2)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		rest := fields[1]
		parts := strings.Fields(rest)
		newVer := ""
		oldVer := ""
		if len(parts) >= 2 {
			newVer = parts[1]
		}
		// Extract old version from "[upgradable from: X.Y.Z]"
		fromIdx := strings.Index(rest, "from: ")
		if fromIdx != -1 {
			tail := rest[fromIdx+6:]
			oldVer = strings.TrimRight(tail, "]")
		}
		updates = append(updates, UpdateInfo{Name: name, CurrentVer: oldVer, NewVer: newVer})
	}
	return updates, nil
}

func collectDnfUpdates() ([]UpdateInfo, error) {
	out, err := exec.Command("dnf", "check-update", "--quiet").Output()
	// dnf returns exit code 100 if updates available, 0 if none
	if err != nil && len(out) == 0 {
		return []UpdateInfo{}, nil
	}
	return parseDnfYumOutput(string(out)), nil
}

func collectYumUpdates() ([]UpdateInfo, error) {
	out, err := exec.Command("yum", "check-update", "--quiet").Output()
	if err != nil && len(out) == 0 {
		return []UpdateInfo{}, nil
	}
	return parseDnfYumOutput(string(out)), nil
}

func parseDnfYumOutput(output string) []UpdateInfo {
	var updates []UpdateInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Skip header lines
		if strings.HasPrefix(fields[0], "Last") || strings.HasPrefix(fields[0], "Obsoleting") {
			continue
		}
		updates = append(updates, UpdateInfo{
			Name:   fields[0],
			NewVer: fields[1],
		})
	}
	return updates
}

// ---- Disk Health ----

type DiskHealthProvider struct{}

func (p *DiskHealthProvider) Name() string { return "disk_health" }

type DiskHealthInfo struct {
	Device      string `json:"device"`
	Model       string `json:"model,omitempty"`
	Serial      string `json:"serial,omitempty"`
	SMARTStatus string `json:"smart_status,omitempty"`
	TempC       string `json:"temperature_c,omitempty"`
	PowerOnHrs  string `json:"power_on_hours,omitempty"`
}

func (p *DiskHealthProvider) Collect() (any, error) {
	// Enumerate block devices
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, err
	}
	var disks []DiskHealthInfo
	for _, e := range entries {
		name := e.Name()
		// Skip loop, ram, dm devices
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") ||
			strings.HasPrefix(name, "dm-") || strings.HasPrefix(name, "sr") {
			continue
		}
		d := DiskHealthInfo{Device: "/dev/" + name}
		d.Model = readSysStr(filepath.Join("/sys/block", name, "device/model"))
		d.Serial = readSysStr(filepath.Join("/sys/block", name, "device/serial"))

		// Try smartctl if available
		if _, lookErr := exec.LookPath("smartctl"); lookErr == nil {
			if sOut, sErr := exec.Command("smartctl", "-H", "-A", "/dev/"+name).Output(); sErr == nil {
				lines := strings.Split(string(sOut), "\n")
				for _, l := range lines {
					if strings.Contains(l, "SMART overall-health") || strings.Contains(l, "SMART Health Status") {
						parts := strings.SplitN(l, ":", 2)
						if len(parts) == 2 {
							d.SMARTStatus = strings.TrimSpace(parts[1])
						}
					}
					if strings.Contains(l, "Temperature_Celsius") || strings.Contains(l, "Current Drive Temperature") {
						fields := strings.Fields(l)
						if len(fields) > 0 {
							d.TempC = fields[len(fields)-1]
						}
					}
					if strings.Contains(l, "Power_On_Hours") {
						fields := strings.Fields(l)
						if len(fields) > 0 {
							d.PowerOnHrs = fields[len(fields)-1]
						}
					}
				}
			}
		}
		disks = append(disks, d)
	}
	return disks, nil
}

func readSysStr(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// ---- Hardware ----

type HardwareProvider struct{}

func (p *HardwareProvider) Name() string { return "hardware" }

type HardwareInfo struct {
	Hostname      string `json:"hostname"`
	Architecture  string `json:"architecture"`
	CPUModel      string `json:"cpu_model"`
	CPUCores      int    `json:"cpu_cores"`
	TotalMemory   string `json:"total_memory"`
	BoardVendor   string `json:"board_vendor,omitempty"`
	BoardName     string `json:"board_name,omitempty"`
	BIOSVendor    string `json:"bios_vendor,omitempty"`
	BIOSVersion   string `json:"bios_version,omitempty"`
	ProductName   string `json:"product_name,omitempty"`
	ProductSerial string `json:"product_serial,omitempty"`
	MachineID     string `json:"machine_id,omitempty"`
}

func (p *HardwareProvider) Collect() (any, error) {
	info := HardwareInfo{
		Architecture: runtime.GOARCH,
	}
	info.Hostname, _ = os.Hostname()
	info.CPUCores = runtime.NumCPU()

	// CPU model from /proc/cpuinfo
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "model name") {
				parts := strings.SplitN(scanner.Text(), ":", 2)
				if len(parts) == 2 {
					info.CPUModel = strings.TrimSpace(parts[1])
					break
				}
			}
		}
		f.Close()
	}

	// Total memory
	if f, err := os.Open("/proc/meminfo"); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "MemTotal:") {
				parts := strings.Fields(scanner.Text())
				if len(parts) >= 2 {
					if kb, err := strconv.ParseFloat(parts[1], 64); err == nil {
						info.TotalMemory = fmt.Sprintf("%.1f GB", kb/1024/1024)
					}
				}
				break
			}
		}
		f.Close()
	}

	// DMI info
	info.BoardVendor = readSysStr("/sys/class/dmi/id/board_vendor")
	info.BoardName = readSysStr("/sys/class/dmi/id/board_name")
	info.BIOSVendor = readSysStr("/sys/class/dmi/id/bios_vendor")
	info.BIOSVersion = readSysStr("/sys/class/dmi/id/bios_version")
	info.ProductName = readSysStr("/sys/class/dmi/id/product_name")
	info.ProductSerial = readSysStr("/sys/class/dmi/id/product_serial")
	info.MachineID = readSysStr("/etc/machine-id")

	return info, nil
}

// ---- OS ----

type OSProvider struct{}

func (p *OSProvider) Name() string { return "os" }

type OSInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	ID         string `json:"id"`
	PrettyName string `json:"pretty_name"`
	Kernel     string `json:"kernel"`
}

func (p *OSProvider) Collect() (any, error) {
	info := OSInfo{}

	// /etc/os-release
	if f, err := os.Open("/etc/os-release"); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			k, v := parseKV(line)
			switch k {
			case "NAME":
				info.Name = v
			case "VERSION":
				info.Version = v
			case "ID":
				info.ID = v
			case "PRETTY_NAME":
				info.PrettyName = v
			}
		}
		f.Close()
	}

	// Kernel
	if out, err := exec.Command("uname", "-r").Output(); err == nil {
		info.Kernel = strings.TrimSpace(string(out))
	}

	return info, nil
}

func parseKV(line string) (string, string) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], strings.Trim(parts[1], "\"")
}

// ---- Ports ----

type PortsProvider struct{}

func (p *PortsProvider) Name() string { return "ports" }

type PortInfo struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	PID      int    `json:"pid,omitempty"`
	Process  string `json:"process,omitempty"`
}

func (p *PortsProvider) Collect() (any, error) {
	var ports []PortInfo

	// Try ss first
	if _, err := exec.LookPath("ss"); err == nil {
		ports = collectSS()
	} else {
		// Fallback: parse /proc/net/tcp and /proc/net/udp
		ports = append(ports, parseProcNet("/proc/net/tcp", "tcp")...)
		ports = append(ports, parseProcNet("/proc/net/tcp6", "tcp6")...)
		ports = append(ports, parseProcNet("/proc/net/udp", "udp")...)
		ports = append(ports, parseProcNet("/proc/net/udp6", "udp6")...)
	}

	return ports, nil
}

func collectSS() []PortInfo {
	seen := make(map[string]bool)
	var ports []PortInfo
	for _, pi := range append(collectSSProto("tcp"), collectSSProto("udp")...) {
		key := pi.Protocol + ":" + pi.Address + ":" + strconv.Itoa(pi.Port)
		if seen[key] {
			continue
		}
		seen[key] = true
		ports = append(ports, pi)
	}
	return ports
}

func collectSSProto(proto string) []PortInfo {
	flag := "-t"
	if proto == "udp" {
		flag = "-u"
	}
	// -l: dinleyenler, -n: numeric, -p: process (no -H for compatibility)
	out, err := exec.Command("ss", flag, "-lnp").Output()
	if err != nil {
		return nil
	}
	var ports []PortInfo
	for i, line := range strings.Split(string(out), "\n") {
		if i == 0 {
			continue // skip header line
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		// Format: State Recv-Q Send-Q Local Peer [Process]
		local := fields[3]
		addr, portStr := splitHostPort(local)
		port, _ := strconv.Atoi(portStr)
		if port == 0 {
			continue
		}
		pi := PortInfo{Protocol: proto, Address: addr, Port: port}
		for _, f := range fields[4:] {
			if strings.HasPrefix(f, "users:") {
				pi.Process, pi.PID = parseSSUsers(f)
				break
			}
		}
		ports = append(ports, pi)
	}
	return ports
}

func splitHostPort(s string) (string, string) {
	// Handle [::]:port and addr:port
	if strings.HasPrefix(s, "[") {
		// IPv6
		idx := strings.LastIndex(s, "]:")
		if idx != -1 {
			return s[:idx+1], s[idx+2:]
		}
		return s, ""
	}
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s, ""
	}
	return s[:idx], s[idx+1:]
}

func parseSSUsers(field string) (string, int) {
	// users:(("process",pid=1234,fd=5))
	start := strings.Index(field, "((")
	end := strings.LastIndex(field, "))")
	if start == -1 || end == -1 {
		return "", 0
	}
	inner := field[start+2 : end]
	// First group: ("name",pid=N,fd=M) — take only the first process
	if idx := strings.Index(inner, "),("); idx != -1 {
		inner = inner[:idx]
	}
	parts := strings.Split(inner, ",")
	var name string
	var pid int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "pid=") {
			pid, _ = strconv.Atoi(strings.TrimPrefix(p, "pid="))
		} else if strings.HasPrefix(p, "fd=") {
			// skip
		} else if name == "" {
			name = strings.Trim(p, "\"")
		}
	}
	return name, pid
}

func parseProcNet(path, proto string) []PortInfo {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var ports []PortInfo
	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		// State: 0A = LISTEN for tcp
		if proto == "tcp" || proto == "tcp6" {
			if fields[3] != "0A" {
				continue
			}
		}
		localAddr := fields[1]
		parts := strings.SplitN(localAddr, ":", 2)
		if len(parts) != 2 {
			continue
		}
		portHex := parts[1]
		port64, _ := strconv.ParseInt(portHex, 16, 32)
		port := int(port64)
		if port == 0 {
			continue
		}

		addr := hexToIP(parts[0])
		ports = append(ports, PortInfo{
			Protocol: proto,
			Address:  addr,
			Port:     port,
		})
	}
	return ports
}

func hexToIP(hex string) string {
	if len(hex) == 8 {
		// IPv4: little-endian
		b := make([]byte, 4)
		for i := 0; i < 4; i++ {
			val, _ := strconv.ParseUint(hex[6-2*i:8-2*i], 16, 8)
			b[i] = byte(val)
		}
		return net.IP(b).String()
	}
	// IPv6
	if len(hex) == 32 {
		b := make([]byte, 16)
		for g := 0; g < 4; g++ {
			for i := 0; i < 4; i++ {
				val, _ := strconv.ParseUint(hex[g*8+6-2*i:g*8+8-2*i], 16, 8)
				b[g*4+i] = byte(val)
			}
		}
		return net.IP(b).String()
	}
	return hex
}
