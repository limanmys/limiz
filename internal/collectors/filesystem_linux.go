package collectors

import (
	"bufio"
	"os"
	"strings"
	"syscall"
)

type FilesystemCollector struct{}

func NewFilesystemCollector() *FilesystemCollector { return &FilesystemCollector{} }
func (c *FilesystemCollector) Name() string        { return "filesystem" }

// Filesystem types to ignore
var ignoredFSTypes = map[string]bool{
	"tmpfs": true, "devtmpfs": true, "devpts": true,
	"sysfs": true, "proc": true, "cgroup": true,
	"cgroup2": true, "pstore": true, "securityfs": true,
	"debugfs": true, "tracefs": true, "hugetlbfs": true,
	"mqueue": true, "fusectl": true, "binfmt_misc": true,
	"autofs": true, "configfs": true, "efivarfs": true,
	"overlay": true, "nsfs": true, "fuse.lxcfs": true,
}

func (c *FilesystemCollector) Collect() []Metric {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()

	var metrics []Metric
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		device := fields[0]
		mountpoint := fields[1]
		fstype := fields[2]

		if ignoredFSTypes[fstype] {
			continue
		}
		if !strings.HasPrefix(device, "/") {
			continue
		}
		if seen[mountpoint] {
			continue
		}
		seen[mountpoint] = true

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mountpoint, &stat); err != nil {
			continue
		}

		labels := map[string]string{
			"device":     device,
			"mountpoint": mountpoint,
			"fstype":     fstype,
		}

		blockSize := float64(stat.Bsize)

		metrics = append(metrics,
			Metric{Name: "node_filesystem_size_bytes", Help: "Filesystem size in bytes.", Type: Gauge, Labels: labels, Value: float64(stat.Blocks) * blockSize},
			Metric{Name: "node_filesystem_free_bytes", Help: "Filesystem free space (for root).", Type: Gauge, Labels: labels, Value: float64(stat.Bfree) * blockSize},
			Metric{Name: "node_filesystem_avail_bytes", Help: "Filesystem space available to non-root.", Type: Gauge, Labels: labels, Value: float64(stat.Bavail) * blockSize},
			Metric{Name: "node_filesystem_files", Help: "Total inodes.", Type: Gauge, Labels: labels, Value: float64(stat.Files)},
			Metric{Name: "node_filesystem_files_free", Help: "Free inodes.", Type: Gauge, Labels: labels, Value: float64(stat.Ffree)},
		)
	}

	return metrics
}
