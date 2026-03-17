package collectors

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type MemoryCollector struct{}

func NewMemoryCollector() *MemoryCollector { return &MemoryCollector{} }
func (c *MemoryCollector) Name() string    { return "memory" }

// Fields we want from /proc/meminfo mapped to metric names
var memFields = map[string]string{
	"MemTotal":     "node_memory_MemTotal_bytes",
	"MemFree":      "node_memory_MemFree_bytes",
	"MemAvailable": "node_memory_MemAvailable_bytes",
	"Buffers":      "node_memory_Buffers_bytes",
	"Cached":       "node_memory_Cached_bytes",
	"SwapTotal":    "node_memory_SwapTotal_bytes",
	"SwapFree":     "node_memory_SwapFree_bytes",
	"SwapCached":   "node_memory_SwapCached_bytes",
	"Active":       "node_memory_Active_bytes",
	"Inactive":     "node_memory_Inactive_bytes",
	"Dirty":        "node_memory_Dirty_bytes",
	"Writeback":    "node_memory_Writeback_bytes",
	"Slab":         "node_memory_Slab_bytes",
	"SReclaimable": "node_memory_SReclaimable_bytes",
	"SUnreclaim":   "node_memory_SUnreclaim_bytes",
	"Mapped":       "node_memory_Mapped_bytes",
	"Shmem":        "node_memory_Shmem_bytes",
	"HugePages_Total": "node_memory_HugePages_Total",
	"HugePages_Free":  "node_memory_HugePages_Free",
	"Hugepagesize":    "node_memory_Hugepagesize_bytes",
}

func (c *MemoryCollector) Collect() []Metric {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil
	}
	defer f.Close()

	var metrics []Metric
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSuffix(parts[0], ":")
		metricName, ok := memFields[key]
		if !ok {
			continue
		}

		val, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			continue
		}

		// Convert kB to bytes (meminfo reports in kB), except HugePages counts
		if len(parts) >= 3 && parts[2] == "kB" {
			val *= 1024
		}

		metrics = append(metrics, Metric{
			Name:  metricName,
			Help:  "Memory information field " + key + ".",
			Type:  Gauge,
			Value: val,
		})
	}

	return metrics
}
