package winsvc

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Windows API constants
const (
	SC_MANAGER_ALL_ACCESS     = 0xF003F
	SERVICE_ALL_ACCESS        = 0xF01FF
	SERVICE_WIN32_OWN_PROCESS = 0x00000010
	SERVICE_AUTO_START        = 0x00000002
	SERVICE_DEMAND_START      = 0x00000003
	SERVICE_ERROR_NORMAL      = 0x00000001

	SERVICE_CONTROL_STOP        = 0x00000001
	SERVICE_CONTROL_INTERROGATE = 0x00000004

	SERVICE_STOPPED       = 0x00000001
	SERVICE_START_PENDING = 0x00000002
	SERVICE_STOP_PENDING  = 0x00000003
	SERVICE_RUNNING       = 0x00000004

	NO_ERROR                     = 0
	ERROR_SERVICE_SPECIFIC_ERROR = 1066
)


var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procOpenSCManager              = advapi32.NewProc("OpenSCManagerW")
	procCloseServiceHandle         = advapi32.NewProc("CloseServiceHandle")
	procCreateService              = advapi32.NewProc("CreateServiceW")
	procOpenService                = advapi32.NewProc("OpenServiceW")
	procDeleteService              = advapi32.NewProc("DeleteService")
	procStartService               = advapi32.NewProc("StartServiceW")
	procControlService             = advapi32.NewProc("ControlService")
	procRegisterServiceCtrlHandler = advapi32.NewProc("RegisterServiceCtrlHandlerW")
	procSetServiceStatus           = advapi32.NewProc("SetServiceStatus")
	procStartServiceCtrlDispatcher = advapi32.NewProc("StartServiceCtrlDispatcherW")
	procChangeServiceConfig        = advapi32.NewProc("ChangeServiceConfigW")
	procChangeServiceConfig2       = advapi32.NewProc("ChangeServiceConfig2W")
	procRegisterEventSource        = advapi32.NewProc("RegisterEventSourceW")
	procDeregisterEventSource      = advapi32.NewProc("DeregisterEventSource")
	procReportEvent                = advapi32.NewProc("ReportEventW")
	procQueryServiceStatus         = advapi32.NewProc("QueryServiceStatus")
)

// SERVICE_STATUS for SetServiceStatus
type serviceStatus struct {
	ServiceType             uint32
	CurrentState            uint32
	ControlsAccepted        uint32
	Win32ExitCode           uint32
	ServiceSpecificExitCode uint32
	CheckPoint              uint32
	WaitHint                uint32
}

// SERVICE_TABLE_ENTRY for StartServiceCtrlDispatcher
type serviceTableEntry struct {
	ServiceName *uint16
	ServiceProc uintptr
}

// SERVICE_DESCRIPTION for ChangeServiceConfig2
type serviceDescription struct {
	Description *uint16
}

const SERVICE_CONFIG_DESCRIPTION = 1

// RunFunc is the function the service will execute.
type RunFunc func(stopCh <-chan struct{})

var (
	serviceName  string
	runFunc      RunFunc
	statusHandle uintptr
	stopCh       chan struct{}
	stoppedCh    chan struct{}
	mu           sync.Mutex
)

// IsWindowsService tries to detect if the process is running as a Windows service.
// Services are started by SCM with no console attached and the parent is services.exe.
func IsWindowsService() bool {
	// Check if stdin is a valid console handle
	// Services typically have no console
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	hwnd, _, _ := getConsoleWindow.Call()
	return hwnd == 0
}

// Install registers or updates the service with Windows SCM.
// Servis zaten varsa ChangeServiceConfig ile günceller (DeleteService çağırmaz).
// Servis yoksa CreateService ile yeni oluşturur.
func Install(name, displayName, description, binPath, configPath string) error {
	scm, _, err := procOpenSCManager.Call(0, 0, SC_MANAGER_ALL_ACCESS)
	if scm == 0 {
		return fmt.Errorf("OpenSCManager: %v", err)
	}
	defer procCloseServiceHandle.Call(scm)

	// Build binPath with --config and --run-service flags
	cmdLine := fmt.Sprintf(`"%s" --config "%s" --run-service`, binPath, configPath)

	namePtr, _ := syscall.UTF16PtrFromString(name)
	displayPtr, _ := syscall.UTF16PtrFromString(displayName)
	cmdPtr, _ := syscall.UTF16PtrFromString(cmdLine)

	// Try to open existing service first
	svc, _, _ := procOpenService.Call(scm, uintptr(unsafe.Pointer(namePtr)), SERVICE_ALL_ACCESS)
	if svc != 0 {
		// Service exists: update its config instead of deleting and recreating
		ret, _, err := procChangeServiceConfig.Call(
			svc,
			SERVICE_WIN32_OWN_PROCESS,
			SERVICE_AUTO_START,
			SERVICE_ERROR_NORMAL,
			uintptr(unsafe.Pointer(cmdPtr)),
			0, 0, 0, 0, 0,
			uintptr(unsafe.Pointer(displayPtr)),
		)
		if ret == 0 {
			procCloseServiceHandle.Call(svc)
			return fmt.Errorf("ChangeServiceConfig: %v", err)
		}
	} else {
		// Service does not exist: create it
		svc, _, err = procCreateService.Call(
			scm,
			uintptr(unsafe.Pointer(namePtr)),
			uintptr(unsafe.Pointer(displayPtr)),
			SERVICE_ALL_ACCESS,
			SERVICE_WIN32_OWN_PROCESS,
			SERVICE_AUTO_START,
			SERVICE_ERROR_NORMAL,
			uintptr(unsafe.Pointer(cmdPtr)),
			0, 0, 0, 0, 0,
		)
		if svc == 0 {
			return fmt.Errorf("CreateService: %v", err)
		}
	}
	defer procCloseServiceHandle.Call(svc)

	// Set description
	if description != "" {
		descPtr, _ := syscall.UTF16PtrFromString(description)
		sd := serviceDescription{Description: descPtr}
		procChangeServiceConfig2.Call(svc, SERVICE_CONFIG_DESCRIPTION, uintptr(unsafe.Pointer(&sd)))
	}

	return nil
}

// Uninstall stops and removes the service from SCM.
// Servisin tam olarak durmasını bekledikten sonra siler.
func Uninstall(name string) error {
	scm, _, err := procOpenSCManager.Call(0, 0, SC_MANAGER_ALL_ACCESS)
	if scm == 0 {
		return fmt.Errorf("OpenSCManager: %v", err)
	}
	defer procCloseServiceHandle.Call(scm)

	namePtr, _ := syscall.UTF16PtrFromString(name)
	svc, _, err := procOpenService.Call(scm, uintptr(unsafe.Pointer(namePtr)), SERVICE_ALL_ACCESS)
	if svc == 0 {
		return fmt.Errorf("OpenService: %v", err)
	}
	defer procCloseServiceHandle.Call(svc)

	// Stop isteği gönder
	var ss serviceStatus
	procControlService.Call(svc, SERVICE_CONTROL_STOP, uintptr(unsafe.Pointer(&ss)))

	// Servis STOPPED olana kadar bekle (max 15 saniye)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		var qs serviceStatus
		procQueryServiceStatus.Call(svc, uintptr(unsafe.Pointer(&qs)))
		if qs.CurrentState == SERVICE_STOPPED {
			break
		}
	}

	ret, _, err := procDeleteService.Call(svc)
	if ret == 0 {
		return fmt.Errorf("DeleteService: %v", err)
	}
	return nil
}

// Start sends a start command to an installed service.
func Start(name string) error {
	scm, _, err := procOpenSCManager.Call(0, 0, SC_MANAGER_ALL_ACCESS)
	if scm == 0 {
		return fmt.Errorf("OpenSCManager: %v", err)
	}
	defer procCloseServiceHandle.Call(scm)

	namePtr, _ := syscall.UTF16PtrFromString(name)
	svc, _, err := procOpenService.Call(scm, uintptr(unsafe.Pointer(namePtr)), SERVICE_ALL_ACCESS)
	if svc == 0 {
		return fmt.Errorf("OpenService: %v", err)
	}
	defer procCloseServiceHandle.Call(svc)

	ret, _, err := procStartService.Call(svc, 0, 0)
	if ret == 0 {
		return fmt.Errorf("StartService: %v", err)
	}
	return nil
}

// Stop sends a stop command to a running service.
func Stop(name string) error {
	scm, _, err := procOpenSCManager.Call(0, 0, SC_MANAGER_ALL_ACCESS)
	if scm == 0 {
		return fmt.Errorf("OpenSCManager: %v", err)
	}
	defer procCloseServiceHandle.Call(scm)

	namePtr, _ := syscall.UTF16PtrFromString(name)
	svc, _, err := procOpenService.Call(scm, uintptr(unsafe.Pointer(namePtr)), SERVICE_ALL_ACCESS)
	if svc == 0 {
		return fmt.Errorf("OpenService: %v", err)
	}
	defer procCloseServiceHandle.Call(svc)

	var ss serviceStatus
	ret, _, err := procControlService.Call(svc, SERVICE_CONTROL_STOP, uintptr(unsafe.Pointer(&ss)))
	if ret == 0 {
		return fmt.Errorf("ControlService(STOP): %v", err)
	}
	return nil
}

// RunService starts the SCM dispatcher. Blocks until the service is stopped.
func RunService(name string, fn RunFunc) error {
	serviceName = name
	runFunc = fn
	stopCh = make(chan struct{})
	stoppedCh = make(chan struct{})

	namePtr, _ := syscall.UTF16PtrFromString(name)

	table := [2]serviceTableEntry{
		{ServiceName: namePtr, ServiceProc: syscall.NewCallback(serviceMain)},
		{ServiceName: nil, ServiceProc: 0}, // null terminator
	}

	ret, _, err := procStartServiceCtrlDispatcher.Call(uintptr(unsafe.Pointer(&table[0])))
	if ret == 0 {
		return fmt.Errorf("StartServiceCtrlDispatcher: %v", err)
	}
	return nil
}

func serviceMain(argc uint32, argv **uint16) uintptr {
	namePtr, _ := syscall.UTF16PtrFromString(serviceName)

	statusHandle, _, _ = procRegisterServiceCtrlHandler.Call(
		uintptr(unsafe.Pointer(namePtr)),
		syscall.NewCallback(serviceHandler),
	)
	if statusHandle == 0 {
		return 1
	}

	setServiceStatus(SERVICE_START_PENDING, 0)
	setServiceStatus(SERVICE_RUNNING, SERVICE_CONTROL_STOP)

	// Run the actual application
	go runFunc(stopCh)

	// Wait until stopped
	<-stoppedCh

	setServiceStatus(SERVICE_STOPPED, 0)
	return 0
}

func serviceHandler(control uint32) uintptr {
	switch control {
	case SERVICE_CONTROL_STOP:
		setServiceStatus(SERVICE_STOP_PENDING, 0)
		WriteEventLog(serviceName, serviceName+" servisi durduruluyor.")
		close(stopCh)
		// Give the app time to shutdown gracefully
		time.Sleep(3 * time.Second)
		close(stoppedCh)
	case SERVICE_CONTROL_INTERROGATE:
		// Just return current status
	}
	return 0
}

// WriteEventLog writes a single informational message to the Windows Application Event Log.
func WriteEventLog(source, message string) {
	WriteEventLogLines(source, []string{message}, EVENTLOG_INFORMATION_TYPE)
}

// WriteEventLogLines writes multiple lines to the Windows Application Event Log.
// eventType should be EVENTLOG_INFORMATION_TYPE or EVENTLOG_ERROR_TYPE.
// Each line becomes a separate <Data> element in Event Viewer's XML Details tab.
func WriteEventLogLines(source string, lines []string, eventType uint32) {
	if len(lines) == 0 {
		return
	}
	srcPtr, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return
	}
	h, _, _ := procRegisterEventSource.Call(0, uintptr(unsafe.Pointer(srcPtr)))
	if h == 0 {
		return
	}
	defer procDeregisterEventSource.Call(h)

	ptrs := make([]*uint16, 0, len(lines))
	for _, line := range lines {
		p, err := syscall.UTF16PtrFromString(line)
		if err != nil {
			continue
		}
		ptrs = append(ptrs, p)
	}
	if len(ptrs) == 0 {
		return
	}
	procReportEvent.Call(
		h,
		uintptr(eventType),
		0, // category
		0, // event ID
		0, // user SID
		uintptr(len(ptrs)),
		0, // raw data size
		uintptr(unsafe.Pointer(&ptrs[0])),
		0, // raw data
	)
}

func setServiceStatus(state, accepts uint32) {
	ss := serviceStatus{
		ServiceType:      SERVICE_WIN32_OWN_PROCESS,
		CurrentState:     state,
		ControlsAccepted: accepts,
	}
	if state == SERVICE_START_PENDING || state == SERVICE_STOP_PENDING {
		ss.WaitHint = 10000
		ss.CheckPoint = 1
	}
	procSetServiceStatus.Call(statusHandle, uintptr(unsafe.Pointer(&ss)))
}

// HandleServiceCommands processes service management subcommands.
// Returns true if a service command was handled (caller should exit).
func HandleServiceCommands(args []string, configPath string) bool {
	if len(args) == 0 {
		return false
	}

	cmd := strings.ToLower(args[0])
	svcName := "Limiz"
	displayName := "Limiz"
	description := "Prometheus-compatible system metrics exporter with local SQLite storage"

	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine executable path: %v\n", err)
		return true
	}

	switch cmd {
	case "install":
		if configPath == "" {
			fmt.Fprintln(os.Stderr, "Error: --config is required for service install")
			fmt.Fprintln(os.Stderr, "Usage: limiz.exe install --config C:\\path\\to\\config.json")
			return true
		}
		fmt.Printf("Installing service %q...\n", svcName)
		if err := Install(svcName, displayName, description, exePath, configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintln(os.Stderr, "Hint: Run as Administrator.")
			os.Exit(1)
		}
		fmt.Println("Service installed successfully.")
		fmt.Println("Start with: limiz.exe start")
		fmt.Printf("Config:     %s\n", configPath)
		fmt.Printf("Binary:     %s\n", exePath)
		return true

	case "uninstall":
		fmt.Printf("Uninstalling service %q...\n", svcName)
		if err := Uninstall(svcName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintln(os.Stderr, "Hint: Run as Administrator.")
			os.Exit(1)
		}
		fmt.Println("Service uninstalled successfully.")
		return true

	case "start":
		fmt.Printf("Starting service %q...\n", svcName)
		if err := Start(svcName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service started.")
		return true

	case "stop":
		fmt.Printf("Stopping service %q...\n", svcName)
		if err := Stop(svcName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service stopped.")
		return true

	case "status":
		fmt.Println("Use: sc.exe query Limiz")
		return true
	}

	return false
}
