//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

//go:embed limiz.exe
var embeddedLimiz []byte

var (
	advapi32                     = syscall.NewLazyDLL("advapi32.dll")
	procAllocateAndInitializeSid = advapi32.NewProc("AllocateAndInitializeSid")
	procCheckTokenMembership     = advapi32.NewProc("CheckTokenMembership")
	procFreeSid                  = advapi32.NewProc("FreeSid")
)

type sidIdentifierAuthority struct {
	Value [6]byte
}

const (
	installDir       = `C:\Program Files\limiz`
	configPath       = `C:\Program Files\limiz\config.json`
	baseConfigPath   = `C:\Program Files\limiz\base.config.json`
	dbPath           = `C:\Program Files\limiz\metrics.db`
	exeName          = "limiz.exe"
	metricPluginsDir = `C:\Program Files\limiz\plugins\metric`
	dataPluginsDir   = `C:\Program Files\limiz\plugins\data`
)

func main() {
	fmt.Println("=== Limiz Setup Wizard ===")
	fmt.Println()

	if !isAdmin() {
		fmt.Fprintln(os.Stderr, "ERROR: This program must be run with Administrator privileges.")
		fmt.Fprintln(os.Stderr, "Please right-click setup.exe and select 'Run as administrator'.")
		waitExit()
		os.Exit(1)
	}

	// Step 1: Create directories
	fmt.Printf("[1/4] Creating directories: %s\n", installDir)
	for _, dir := range []string{installDir, metricPluginsDir, dataPluginsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fatalf("Failed to create directory: %v", err)
		}
	}

	// Ensure base.config.json exists (always created empty — admin fills it to pre-configure)
	if _, err := os.Stat(baseConfigPath); os.IsNotExist(err) {
		fmt.Printf("[1/4] Creating base config: %s\n", baseConfigPath)
		if err := os.WriteFile(baseConfigPath, []byte("{}\n"), 0644); err != nil {
			fatalf("Failed to create base config: %v", err)
		}
	} else {
		fmt.Printf("[1/4] Base config preserved: %s\n", baseConfigPath)
	}

	dstExe := filepath.Join(installDir, exeName)

	// Step 2: Stop existing service if present (upgrade scenario — service is not removed, only stopped)
	fmt.Println("[2/4] Checking for existing service...")
	runCmdOptional(dstExe, "stop")
	// Wait for the service to release file locks
	time.Sleep(2 * time.Second)

	// Step 3: Write limiz.exe from embedded binary
	fmt.Printf("[3/4] Writing limiz.exe -> %s\n", dstExe)
	if err := os.WriteFile(dstExe, embeddedLimiz, 0755); err != nil {
		fatalf("Failed to write binary: %v", err)
	}

	// config.json is created via the web /configuration UI on first run.
	// Preserve existing config.json during upgrades.
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("[3/4] Existing config preserved: %s\n", configPath)
	} else {
		fmt.Printf("[3/4] Config will be set up via web UI: %s\n", configPath)
	}

	// Step 4: Install and start service
	fmt.Println("[4/4] Installing service...")
	runCmd(dstExe, "--config", configPath, "install")

	fmt.Println("[4/4] Starting service...")
	runCmd(dstExe, "start")

	fmt.Println()
	fmt.Println("=== Installation complete! ===")
	fmt.Printf("Service name  : Limiz\n")
	fmt.Printf("Binary        : %s\n", dstExe)
	fmt.Printf("Base config   : %s\n", baseConfigPath)
	fmt.Printf("Config        : %s\n", configPath)
	fmt.Printf("Database      : %s\n", dbPath)
	fmt.Println()
	fmt.Println("If config.json is empty, open http://localhost:9110/configuration to configure.")
	fmt.Println()
	fmt.Println("To check service status : sc.exe query Limiz")
	fmt.Println("To stop service         : limiz.exe stop")
	fmt.Println("To remove service       : limiz.exe uninstall")
	waitExit()
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatalf("Command failed [%s %v]: %v", name, args, err)
	}
}

// runCmdOptional runs the command; silently continues on error.
func runCmdOptional(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Run()
}

func isAdmin() bool {
	// NT AUTHORITY = {0,0,0,0,0,5}
	ntAuthority := sidIdentifierAuthority{Value: [6]byte{0, 0, 0, 0, 0, 5}}

	var adminSID uintptr
	ret, _, _ := procAllocateAndInitializeSid.Call(
		uintptr(unsafe.Pointer(&ntAuthority)),
		2,   // nSubAuthorityCount
		32,  // SECURITY_BUILTIN_DOMAIN_RID
		544, // DOMAIN_ALIAS_RID_ADMINS
		0, 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&adminSID)),
	)
	if ret == 0 {
		return false
	}
	defer procFreeSid.Call(adminSID)

	var isMember uint32
	ret, _, _ = procCheckTokenMembership.Call(
		0, // current token
		adminSID,
		uintptr(unsafe.Pointer(&isMember)),
	)
	return ret != 0 && isMember != 0
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\nHATA: "+format+"\n", args...)
	waitExit()
	os.Exit(1)
}

func waitExit() {
	fmt.Println("\nCikmak icin Enter'a basin...")
	fmt.Scanln()
}
