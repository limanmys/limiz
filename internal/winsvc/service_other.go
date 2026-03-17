//go:build !windows

package winsvc

// IsWindowsService always returns false on non-Windows platforms.
func IsWindowsService() bool { return false }

// RunFunc is the function signature for the service entry point.
type RunFunc func(stopCh <-chan struct{})

// RunService is a no-op on non-Windows platforms.
func RunService(name string, fn RunFunc) error { return nil }

// HandleServiceCommands is a no-op on non-Windows platforms.
func HandleServiceCommands(args []string, configPath string) bool { return false }

// WriteEventLog is a no-op on non-Windows platforms.
func WriteEventLog(source, message string) {}

// WriteEventLogLines is a no-op on non-Windows platforms.
func WriteEventLogLines(source string, lines []string, eventType uint32) {}
