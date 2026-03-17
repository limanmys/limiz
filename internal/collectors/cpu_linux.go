package collectors

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type CPUCollector struct{}

func NewCPUCollector() *CPUCollector { return &CPUCollector{} }
func (c *CPUCollector) Name() string { return "cpu" }

// CPU modes in /proc/stat order
var cpuModes = []string{"user", "nice", "system", "idle", "iowait", "irq", "softirq", "steal"}

func (c *CPUCollector) Collect() []Metric {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil
	}
	defer f.Close()

	var metrics []Metric
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Match per-CPU lines: cpu0, cpu1, ... (skip aggregate "cpu" line)
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		cpuName := fields[0]
		// Skip the aggregate line, keep per-cpu
		if cpuName == "cpu" {
			// Also emit total
			cpuName = "total"
		}

		for i, mode := range cpuModes {
			if i+1 >= len(fields) {
				break
			}
			val, err := strconv.ParseFloat(fields[i+1], 64)
			if err != nil {
				continue
			}
			// Convert from USER_HZ (typically 100) to seconds
			metrics = append(metrics, Metric{
				Name: "node_cpu_seconds_total",
				Help: "Seconds the CPUs spent in each mode.",
				Type: Counter,
				Labels: map[string]string{
					"cpu":  cpuName,
					"mode": mode,
				},
				Value: val / 100.0,
			})
		}
	}

	// Also add CPU count
	cpuCount := 0
	for _, m := range metrics {
		if m.Labels["mode"] == "user" && m.Labels["cpu"] != "total" {
			cpuCount++
		}
	}
	metrics = append(metrics, Metric{
		Name:  "node_cpu_count",
		Help:  "Number of CPUs.",
		Type:  Gauge,
		Value: float64(cpuCount),
	})

	// Context switches and processes
	f2, err := os.Open("/proc/stat")
	if err == nil {
		defer f2.Close()
		scanner2 := bufio.NewScanner(f2)
		for scanner2.Scan() {
			line := scanner2.Text()
			if strings.HasPrefix(line, "ctxt ") {
				if val, err := strconv.ParseFloat(strings.Fields(line)[1], 64); err == nil {
					metrics = append(metrics, Metric{
						Name: "node_context_switches_total", Help: "Total context switches.", Type: Counter, Value: val,
					})
				}
			}
			if strings.HasPrefix(line, "processes ") {
				if val, err := strconv.ParseFloat(strings.Fields(line)[1], 64); err == nil {
					metrics = append(metrics, Metric{
						Name: "node_forks_total", Help: "Total forks.", Type: Counter, Value: val,
					})
				}
			}
			if strings.HasPrefix(line, "procs_running ") {
				if val, err := strconv.ParseFloat(strings.Fields(line)[1], 64); err == nil {
					metrics = append(metrics, Metric{
						Name: "node_procs_running", Help: "Number of processes in runnable state.", Type: Gauge, Value: val,
					})
				}
			}
			if strings.HasPrefix(line, "procs_blocked ") {
				if val, err := strconv.ParseFloat(strings.Fields(line)[1], 64); err == nil {
					metrics = append(metrics, Metric{
						Name: "node_procs_blocked", Help: "Number of processes blocked waiting for I/O.", Type: Gauge, Value: val,
					})
				}
			}
		}
	}

	_ = fmt.Sprintf // avoid import error
	return metrics
}
