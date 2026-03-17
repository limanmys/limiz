package datas

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ExecDataPlugin wraps an externally executed binary as a Provider.
// When Collect() is called, it runs the plugin binary as a subprocess
// and parses stdout as JSON.
type ExecDataPlugin struct {
	item       DataPluginItem
	cfg        *DataPluginsConfig
	logSecOnce sync.Once
}

// NewExecDataPlugin returns a configured ExecDataPlugin instance.
func NewExecDataPlugin(item DataPluginItem, cfg *DataPluginsConfig) *ExecDataPlugin {
	return &ExecDataPlugin{item: item, cfg: cfg}
}

// Name satisfies the Provider interface.
func (p *ExecDataPlugin) Name() string {
	return p.item.Name
}

// Collect performs security verification, runs the plugin, and returns JSON data.
func (p *ExecDataPlugin) Collect() (any, error) {
	binaryPath := p.resolveBinary()

	if err := VerifyDataPlugin(p.item.Name, binaryPath); err != nil {
		p.logSecOnce.Do(func() {
			log.Printf("[SECURITY] Data plugin rejected [%s]: %v", p.item.Name, err)
		})
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout())
	defer cancel()

	args := append([]string{"--collect"}, p.item.Args...)
	cmd := exec.CommandContext(ctx, binaryPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			log.Printf("[data-plugin:%s] stderr: %s", p.item.Name, strings.TrimSpace(stderr.String()))
		}
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("[data-plugin:%s] timeout (%s) exceeded", p.item.Name, p.timeout())
		} else {
			log.Printf("[data-plugin:%s] execution error: %v", p.item.Name, err)
		}
		return nil, err
	}

	// Parse as JSON
	var result any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		log.Printf("[data-plugin:%s] JSON parse error: %v", p.item.Name, err)
		return nil, err
	}
	return result, nil
}

// BinaryPath returns the full path of the plugin binary.
// Used by main.go for startup checks.
func (p *ExecDataPlugin) BinaryPath() string {
	return p.resolveBinary()
}

// resolveBinary computes the full path of the plugin.
func (p *ExecDataPlugin) resolveBinary() string {
	execName := p.item.Exec

	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(execName), ".exe") {
		execName += ".exe"
	}

	if !strings.ContainsAny(execName, `/\`) {
		return filepath.Join(p.cfg.Dir, execName)
	}

	return execName
}

// CacheInterval returns the per-plugin cache interval override (may be empty).
// Implements the cacheIntervalProvider interface consumed by Registry.RegisterPlugin.
func (p *ExecDataPlugin) CacheInterval() string {
	return p.item.CacheInterval
}

// timeout tries item.Timeout first, then cfg.DefaultTimeout, and defaults to 10s.
func (p *ExecDataPlugin) timeout() time.Duration {
	for _, s := range []string{p.item.Timeout, p.cfg.DefaultTimeout} {
		if s == "" {
			continue
		}
		d, err := time.ParseDuration(s)
		if err == nil && d > 0 {
			return d
		}
	}
	return 10 * time.Second
}
