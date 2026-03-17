package collectors

import (
	"fmt"
	"strings"
)

// Collector is the interface all metric collectors implement.
type Collector interface {
	Name() string
	Collect() []Metric
}

// MetricType represents the Prometheus metric type.
type MetricType string

const (
	Gauge   MetricType = "gauge"
	Counter MetricType = "counter"
)

// Metric represents a single Prometheus metric with optional labels.
type Metric struct {
	Name   string
	Help   string
	Type   MetricType
	Labels map[string]string
	Value  float64
}

// Registry holds all registered collectors.
type Registry struct {
	collectors []Collector
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(c Collector) {
	r.collectors = append(r.collectors, c)
}

// CollectRaw gathers metrics from all collectors and returns them as structs.
func (r *Registry) CollectRaw() []Metric {
	var all []Metric
	for _, c := range r.collectors {
		metrics := c.Collect()
		all = append(all, metrics...)
	}
	return all
}

// Collect gathers metrics from all collectors and formats them
// in Prometheus exposition format.
func (r *Registry) Collect() string {
	var sb strings.Builder

	for _, c := range r.collectors {
		metrics := c.Collect()
		if len(metrics) == 0 {
			continue
		}

		// Group by metric name for HELP/TYPE headers
		seen := make(map[string]bool)
		for _, m := range metrics {
			if !seen[m.Name] {
				sb.WriteString(fmt.Sprintf("# HELP %s %s\n", m.Name, m.Help))
				sb.WriteString(fmt.Sprintf("# TYPE %s %s\n", m.Name, m.Type))
				seen[m.Name] = true
			}
			if len(m.Labels) > 0 {
				labelParts := make([]string, 0, len(m.Labels))
				for k, v := range m.Labels {
					labelParts = append(labelParts, fmt.Sprintf(`%s="%s"`, k, v))
				}
				sb.WriteString(fmt.Sprintf("%s{%s} %g\n", m.Name, strings.Join(labelParts, ","), m.Value))
			} else {
				sb.WriteString(fmt.Sprintf("%s %g\n", m.Name, m.Value))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
