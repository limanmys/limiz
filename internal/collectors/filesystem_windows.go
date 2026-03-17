package collectors

type FilesystemCollector struct{}

func NewFilesystemCollector() *FilesystemCollector { return &FilesystemCollector{} }
func (c *FilesystemCollector) Name() string        { return "filesystem" }

func (c *FilesystemCollector) Collect() []Metric {
	var metrics []Metric

	// DriveType=3 filters to local fixed disks only
	diskData, err := psJSON(`Get-CimInstance Win32_LogicalDisk -Filter "DriveType=3" | ` +
		`Select-Object DeviceID, FileSystem, FreeSpace, Size | ConvertTo-Json`)
	if err != nil {
		return nil
	}

	for _, d := range diskData {
		deviceID := getString(d, "DeviceID") // e.g. "C:"
		fstype := getString(d, "FileSystem")  // e.g. "NTFS"
		totalSize := getFloat(d, "Size")
		freeSpace := getFloat(d, "FreeSpace")

		if totalSize == 0 {
			continue
		}

		labels := map[string]string{
			"device":     deviceID,
			"mountpoint": deviceID + `\`,
			"fstype":     fstype,
		}

		metrics = append(metrics,
			Metric{Name: "node_filesystem_size_bytes", Help: "Filesystem size in bytes.", Type: Gauge, Labels: labels, Value: totalSize},
			Metric{Name: "node_filesystem_free_bytes", Help: "Filesystem free space in bytes.", Type: Gauge, Labels: labels, Value: freeSpace},
			Metric{Name: "node_filesystem_avail_bytes", Help: "Filesystem available space in bytes.", Type: Gauge, Labels: labels, Value: freeSpace},
		)
	}

	return metrics
}
