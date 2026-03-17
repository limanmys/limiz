package collectors

import (
	"bytes"
	"context"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ExecPlugin wraps an externally executed binary as a Collector.
// When Collect() is called, it runs the plugin binary as a subprocess
// and parses stdout in Prometheus metric format.
type ExecPlugin struct {
	item       PluginItem
	cfg        *PluginsConfig
	logSecOnce sync.Once
}

// NewExecPlugin returns a configured ExecPlugin instance.
func NewExecPlugin(item PluginItem, cfg *PluginsConfig) *ExecPlugin {
	return &ExecPlugin{item: item, cfg: cfg}
}

// Name satisfies the Collector interface.
func (p *ExecPlugin) Name() string {
	return p.item.Name
}

// Collect performs security verification, runs the plugin, and returns metrics.
func (p *ExecPlugin) Collect() []Metric {
	binaryPath := p.resolveBinary()

	if err := VerifyPlugin(p.item.Name, binaryPath); err != nil {
		p.logSecOnce.Do(func() {
			log.Printf("[SECURITY] Plugin rejected [%s]: %v", p.item.Name, err)
		})
		return nil
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
			log.Printf("[plugin:%s] stderr: %s", p.item.Name, strings.TrimSpace(stderr.String()))
		}
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("[plugin:%s] timeout (%s) exceeded", p.item.Name, p.timeout())
		} else {
			log.Printf("[plugin:%s] execution error: %v", p.item.Name, err)
		}
		return nil
	}

	metrics := parsePrometheusText(stdout.String())
	if len(metrics) == 0 {
		log.Printf("[plugin:%s] warning: no metrics produced", p.item.Name)
	}
	return metrics
}

// BinaryPath returns the full path of the plugin binary.
// Used by main.go for startup checks.
func (p *ExecPlugin) BinaryPath() string {
	return p.resolveBinary()
}

// resolveBinary computes the full path of the plugin.
//
//   - If only a name is given (e.g. "dir-size") → cfg.Dir/dir-size(.exe)
//   - If a relative/absolute path is given → used as-is, .exe appended on Windows
func (p *ExecPlugin) resolveBinary() string {
	execName := p.item.Exec

	// Append .exe extension on Windows if not present
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(execName), ".exe") {
		execName += ".exe"
	}

	// If no path separator → look in plugins directory
	if !strings.ContainsAny(execName, `/\`) {
		return filepath.Join(p.cfg.Dir, execName)
	}

	return execName
}

// timeout tries item.Timeout first, then cfg.DefaultTimeout, and defaults to 10s.
func (p *ExecPlugin) timeout() time.Duration {
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
