package collectors

import (
	"time"
)

type UptimeCollector struct{}

func NewUptimeCollector() *UptimeCollector { return &UptimeCollector{} }
func (c *UptimeCollector) Name() string    { return "uptime" }

func (c *UptimeCollector) Collect() []Metric {
	var metrics []Metric

	osInfo, err := psJSON(`Get-CimInstance Win32_OperatingSystem | ` +
		`Select-Object @{N='BootEpoch';E={[int][double]::Parse((Get-Date $_.LastBootUpTime -UFormat %s))}} | ` +
		`ConvertTo-Json`)
	if err == nil && len(osInfo) > 0 {
		bootEpoch := getFloat(osInfo[0], "BootEpoch")
		if bootEpoch > 0 {
			metrics = append(metrics, Metric{
				Name: "node_boot_time_seconds", Help: "Node boot time in seconds since epoch.",
				Type: Gauge, Value: bootEpoch,
			})
		}
	}

	metrics = append(metrics, Metric{
		Name: "node_time_seconds", Help: "Current system time in seconds since epoch.",
		Type: Gauge, Value: float64(time.Now().Unix()),
	})

	return metrics
}
