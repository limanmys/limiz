package collectors

import (
	"os"
	"strconv"
	"strings"
)

type LoadAvgCollector struct{}

func NewLoadAvgCollector() *LoadAvgCollector { return &LoadAvgCollector{} }
func (c *LoadAvgCollector) Name() string     { return "loadavg" }

func (c *LoadAvgCollector) Collect() []Metric {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return nil
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return nil
	}

	var metrics []Metric
	names := []string{"node_load1", "node_load5", "node_load15"}
	helps := []string{"1 minute load average.", "5 minute load average.", "15 minute load average."}

	for i := 0; i < 3; i++ {
		val, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			continue
		}
		metrics = append(metrics, Metric{
			Name:  names[i],
			Help:  helps[i],
			Type:  Gauge,
			Value: val,
		})
	}

	return metrics
}
