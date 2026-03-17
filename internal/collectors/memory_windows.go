package collectors

type MemoryCollector struct{}

func NewMemoryCollector() *MemoryCollector { return &MemoryCollector{} }
func (c *MemoryCollector) Name() string    { return "memory" }

func (c *MemoryCollector) Collect() []Metric {
	var metrics []Metric

	osInfo, err := psJSON(`Get-CimInstance Win32_OperatingSystem | ` +
		`Select-Object TotalVisibleMemorySize, FreePhysicalMemory, ` +
		`TotalVirtualMemorySize, FreeVirtualMemory | ConvertTo-Json`)
	if err != nil || len(osInfo) == 0 {
		return nil
	}

	o := osInfo[0]
	totalPhys := getFloat(o, "TotalVisibleMemorySize") * 1024  // KB -> bytes
	freePhys := getFloat(o, "FreePhysicalMemory") * 1024
	totalVirt := getFloat(o, "TotalVirtualMemorySize") * 1024
	freeVirt := getFloat(o, "FreeVirtualMemory") * 1024

	metrics = append(metrics,
		Metric{Name: "node_memory_MemTotal_bytes", Help: "Total physical memory.", Type: Gauge, Value: totalPhys},
		Metric{Name: "node_memory_MemFree_bytes", Help: "Free physical memory.", Type: Gauge, Value: freePhys},
		Metric{Name: "node_memory_MemAvailable_bytes", Help: "Available physical memory.", Type: Gauge, Value: freePhys},
		Metric{Name: "node_memory_SwapTotal_bytes", Help: "Total swap (page file).", Type: Gauge, Value: totalVirt - totalPhys},
		Metric{Name: "node_memory_SwapFree_bytes", Help: "Free swap (page file).", Type: Gauge, Value: freeVirt - freePhys},
	)

	// Committed/cache details from performance counters
	memPerf, err := psJSON(`Get-CimInstance Win32_PerfFormattedData_PerfOS_Memory | ` +
		`Select-Object CacheBytes, CommittedBytes, PoolNonpagedBytes, PoolPagedBytes, ` +
		`AvailableBytes | ConvertTo-Json`)
	if err == nil && len(memPerf) > 0 {
		m := memPerf[0]
		metrics = append(metrics,
			Metric{Name: "node_memory_Cached_bytes", Help: "Cache memory bytes.", Type: Gauge, Value: getFloat(m, "CacheBytes")},
			Metric{Name: "node_memory_Committed_bytes", Help: "Committed memory bytes.", Type: Gauge, Value: getFloat(m, "CommittedBytes")},
			Metric{Name: "node_memory_PoolNonpaged_bytes", Help: "Non-paged pool bytes.", Type: Gauge, Value: getFloat(m, "PoolNonpagedBytes")},
			Metric{Name: "node_memory_PoolPaged_bytes", Help: "Paged pool bytes.", Type: Gauge, Value: getFloat(m, "PoolPagedBytes")},
		)
	}

	return metrics
}
