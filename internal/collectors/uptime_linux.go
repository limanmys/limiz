package collectors

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type UptimeCollector struct{}

func NewUptimeCollector() *UptimeCollector { return &UptimeCollector{} }
func (c *UptimeCollector) Name() string    { return "uptime" }

func (c *UptimeCollector) Collect() []Metric {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return nil
	}

	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return nil
	}

	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return nil
	}

	bootTime := float64(time.Now().Unix()) - uptime

	return []Metric{
		{Name: "node_boot_time_seconds", Help: "Node boot time in seconds since epoch.", Type: Gauge, Value: bootTime},
		{Name: "node_time_seconds", Help: "Current system time in seconds since epoch.", Type: Gauge, Value: float64(time.Now().Unix())},
	}
}
