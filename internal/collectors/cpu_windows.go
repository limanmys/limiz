package collectors

import (
	"time"
)

type CPUCollector struct{}

func NewCPUCollector() *CPUCollector { return &CPUCollector{} }
func (c *CPUCollector) Name() string { return "cpu" }

func (c *CPUCollector) Collect() []Metric {
	var metrics []Metric

	// Per-CPU times via Win32_PerfFormattedData_PerfOS_Processor
	perfData, err := psJSON(`Get-CimInstance Win32_PerfFormattedData_PerfOS_Processor | ` +
		`Select-Object Name, PercentUserTime, PercentPrivilegedTime, PercentIdleTime, ` +
		`PercentInterruptTime, PercentDPCTime | ConvertTo-Json`)
	if err == nil {
		for _, p := range perfData {
			cpuName := getString(p, "Name")
			if cpuName == "_Total" {
				cpuName = "total"
			} else {
				cpuName = "cpu" + cpuName
			}

			uptime := float64(time.Now().Unix() % 86400)
			modes := map[string]float64{
				"user":    getFloat(p, "PercentUserTime") / 100.0 * uptime,
				"system":  getFloat(p, "PercentPrivilegedTime") / 100.0 * uptime,
				"idle":    getFloat(p, "PercentIdleTime") / 100.0 * uptime,
				"irq":     getFloat(p, "PercentInterruptTime") / 100.0 * uptime,
				"softirq": getFloat(p, "PercentDPCTime") / 100.0 * uptime,
			}

			for mode, val := range modes {
				metrics = append(metrics, Metric{
					Name: "node_cpu_seconds_total", Help: "Seconds the CPUs spent in each mode.",
					Type: Counter, Labels: map[string]string{"cpu": cpuName, "mode": mode},
					Value: val,
				})
			}
		}
	}

	// CPU count
	cpuInfo, err := psJSON(`Get-CimInstance Win32_Processor | ` +
		`Select-Object NumberOfLogicalProcessors | ConvertTo-Json`)
	if err == nil {
		total := 0.0
		for _, ci := range cpuInfo {
			total += getFloat(ci, "NumberOfLogicalProcessors")
		}
		if total > 0 {
			metrics = append(metrics, Metric{
				Name: "node_cpu_count", Help: "Number of logical processors.",
				Type: Gauge, Value: total,
			})
		}
	}

	// Context switches and process count
	sysPerf, err := psJSON(`Get-CimInstance Win32_PerfFormattedData_PerfOS_System | ` +
		`Select-Object ContextSwitchesPersec, Processes | ConvertTo-Json`)
	if err == nil && len(sysPerf) > 0 {
		metrics = append(metrics,
			Metric{Name: "node_context_switches_per_sec", Help: "Context switches per second.", Type: Gauge, Value: getFloat(sysPerf[0], "ContextSwitchesPersec")},
			Metric{Name: "node_procs_running", Help: "Total number of processes.", Type: Gauge, Value: getFloat(sysPerf[0], "Processes")},
		)
	}

	return metrics
}
