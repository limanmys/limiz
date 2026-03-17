package collectors

type LoadAvgCollector struct{}

func NewLoadAvgCollector() *LoadAvgCollector { return &LoadAvgCollector{} }
func (c *LoadAvgCollector) Name() string     { return "loadavg" }

// Windows has no load average. ProcessorQueueLength is the closest equivalent.
func (c *LoadAvgCollector) Collect() []Metric {
	sysPerf, err := psJSON(`Get-CimInstance Win32_PerfFormattedData_PerfOS_System | ` +
		`Select-Object ProcessorQueueLength | ConvertTo-Json`)
	if err != nil || len(sysPerf) == 0 {
		return nil
	}

	queueLen := getFloat(sysPerf[0], "ProcessorQueueLength")

	return []Metric{
		{Name: "node_load1", Help: "Processor queue length (Windows approximation of load1).", Type: Gauge, Value: queueLen},
		{Name: "node_load5", Help: "Processor queue length (Windows approximation of load5).", Type: Gauge, Value: queueLen},
		{Name: "node_load15", Help: "Processor queue length (Windows approximation of load15).", Type: Gauge, Value: queueLen},
	}
}
