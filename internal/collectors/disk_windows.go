package collectors

import (
	"strconv"
	"strings"
)

type DiskCollector struct{}

func NewDiskCollector() *DiskCollector { return &DiskCollector{} }
func (c *DiskCollector) Name() string  { return "disk" }

func (c *DiskCollector) Collect() []Metric {
	var metrics []Metric

	perfData, err := psJSON(`Get-CimInstance Win32_PerfFormattedData_PerfDisk_PhysicalDisk | ` +
		`Select-Object Name, DiskReadBytesPerSec, DiskWriteBytesPerSec, ` +
		`DiskReadsPerSec, DiskWritesPerSec, CurrentDiskQueueLength, ` +
		`AvgDiskSecPerRead, AvgDiskSecPerWrite | ConvertTo-Json`)
	if err != nil {
		return nil
	}

	for _, p := range perfData {
		name := getString(p, "Name")
		if name == "_Total" || name == "" {
			continue
		}

		diskLabel := sanitizeDiskName(name)
		labels := map[string]string{"device": diskLabel}

		metrics = append(metrics,
			Metric{Name: "node_disk_reads_completed_total", Help: "Disk reads per second.", Type: Gauge, Labels: labels, Value: getFloat(p, "DiskReadsPerSec")},
			Metric{Name: "node_disk_read_bytes_total", Help: "Disk read bytes per second.", Type: Gauge, Labels: labels, Value: getFloat(p, "DiskReadBytesPerSec")},
			Metric{Name: "node_disk_writes_completed_total", Help: "Disk writes per second.", Type: Gauge, Labels: labels, Value: getFloat(p, "DiskWritesPerSec")},
			Metric{Name: "node_disk_written_bytes_total", Help: "Disk written bytes per second.", Type: Gauge, Labels: labels, Value: getFloat(p, "DiskWriteBytesPerSec")},
			Metric{Name: "node_disk_io_now", Help: "Current disk queue length.", Type: Gauge, Labels: labels, Value: getFloat(p, "CurrentDiskQueueLength")},
			Metric{Name: "node_disk_read_latency_seconds", Help: "Average read latency.", Type: Gauge, Labels: labels, Value: getFloat(p, "AvgDiskSecPerRead")},
			Metric{Name: "node_disk_write_latency_seconds", Help: "Average write latency.", Type: Gauge, Labels: labels, Value: getFloat(p, "AvgDiskSecPerWrite")},
		)
	}

	return metrics
}

func sanitizeDiskName(name string) string {
	parts := strings.Fields(name)
	if len(parts) > 0 {
		if _, err := strconv.Atoi(parts[0]); err == nil {
			return "disk" + parts[0]
		}
	}
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)
}
