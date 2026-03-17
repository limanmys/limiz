package collectors

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type DiskCollector struct{}

func NewDiskCollector() *DiskCollector { return &DiskCollector{} }
func (c *DiskCollector) Name() string  { return "disk" }

func (c *DiskCollector) Collect() []Metric {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil
	}
	defer f.Close()

	var metrics []Metric
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}

		device := fields[2]

		// Skip partitions (keep whole disks like sda, vda, nvme0n1)
		// Simple heuristic: skip if last char is a digit AND it contains a letter before it
		// This skips sda1, sda2, etc. but keeps sda, nvme0n1
		if isPartition(device) {
			continue
		}

		parseField := func(idx int) float64 {
			v, _ := strconv.ParseFloat(fields[idx], 64)
			return v
		}

		labels := map[string]string{"device": device}

		// Fields from kernel docs:
		// [3] reads completed
		// [4] reads merged
		// [5] sectors read
		// [6] time reading (ms)
		// [7] writes completed
		// [8] writes merged
		// [9] sectors written
		// [10] time writing (ms)
		// [11] I/Os currently in progress
		// [12] time doing I/Os (ms)
		// [13] weighted time doing I/Os (ms)

		metrics = append(metrics,
			Metric{Name: "node_disk_reads_completed_total", Help: "Total reads completed.", Type: Counter, Labels: labels, Value: parseField(3)},
			Metric{Name: "node_disk_reads_merged_total", Help: "Total reads merged.", Type: Counter, Labels: labels, Value: parseField(4)},
			Metric{Name: "node_disk_read_bytes_total", Help: "Total bytes read.", Type: Counter, Labels: labels, Value: parseField(5) * 512},
			Metric{Name: "node_disk_read_time_seconds_total", Help: "Total read time.", Type: Counter, Labels: labels, Value: parseField(6) / 1000},
			Metric{Name: "node_disk_writes_completed_total", Help: "Total writes completed.", Type: Counter, Labels: labels, Value: parseField(7)},
			Metric{Name: "node_disk_writes_merged_total", Help: "Total writes merged.", Type: Counter, Labels: labels, Value: parseField(8)},
			Metric{Name: "node_disk_written_bytes_total", Help: "Total bytes written.", Type: Counter, Labels: labels, Value: parseField(9) * 512},
			Metric{Name: "node_disk_write_time_seconds_total", Help: "Total write time.", Type: Counter, Labels: labels, Value: parseField(10) / 1000},
			Metric{Name: "node_disk_io_now", Help: "I/Os currently in progress.", Type: Gauge, Labels: labels, Value: parseField(11)},
			Metric{Name: "node_disk_io_time_seconds_total", Help: "Total time doing I/Os.", Type: Counter, Labels: labels, Value: parseField(12) / 1000},
			Metric{Name: "node_disk_io_time_weighted_seconds_total", Help: "Weighted time doing I/Os.", Type: Counter, Labels: labels, Value: parseField(13) / 1000},
		)
	}

	return metrics
}

func isPartition(dev string) bool {
	if len(dev) == 0 {
		return false
	}
	// nvme devices: nvme0n1 is a disk, nvme0n1p1 is a partition
	if strings.Contains(dev, "nvme") {
		return strings.Contains(dev, "p") && dev[len(dev)-1] >= '0' && dev[len(dev)-1] <= '9' &&
			strings.LastIndex(dev, "p") > strings.LastIndex(dev, "n")
	}
	// Traditional: sda is disk, sda1 is partition
	lastChar := dev[len(dev)-1]
	if lastChar >= '0' && lastChar <= '9' {
		// Check if there's a letter before the digit sequence
		for i := len(dev) - 1; i >= 0; i-- {
			if dev[i] < '0' || dev[i] > '9' {
				if dev[i] >= 'a' && dev[i] <= 'z' {
					return true
				}
				break
			}
		}
	}
	return false
}
