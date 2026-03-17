package collectors

import (
	"bufio"
	"strconv"
	"strings"
)

// parsePrometheusText parses the Prometheus exposition format
// written to stdout by a plugin and returns []Metric.
//
// Supported line formats:
//
//	# HELP metric_name help text
//	# TYPE metric_name gauge|counter
//	metric_name{label1="val1",label2="val2"} 42.5
//	metric_name 100
func parsePrometheusText(text string) []Metric {
	var metrics []Metric
	helpMap := make(map[string]string)
	typeMap := make(map[string]MetricType)

	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "# HELP ") {
			// "# HELP metric_name help text"
			rest := line[7:]
			if idx := strings.IndexByte(rest, ' '); idx != -1 {
				helpMap[rest[:idx]] = rest[idx+1:]
			}
			continue
		}

		if strings.HasPrefix(line, "# TYPE ") {
			// "# TYPE metric_name gauge"
			parts := strings.Fields(line[7:])
			if len(parts) == 2 {
				typeMap[parts[0]] = MetricType(parts[1])
			}
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue // other comment lines
		}

		m, ok := parseMetricLine(line, helpMap, typeMap)
		if ok {
			metrics = append(metrics, m)
		}
	}

	return metrics
}

func parseMetricLine(line string, helpMap map[string]string, typeMap map[string]MetricType) (Metric, bool) {
	var name, valueStr string
	var labels map[string]string

	if braceOpen := strings.IndexByte(line, '{'); braceOpen != -1 {
		name = line[:braceOpen]
		rest := line[braceOpen+1:]

		braceClose := strings.IndexByte(rest, '}')
		if braceClose == -1 {
			return Metric{}, false
		}

		labels = parseLabels(rest[:braceClose])
		valueStr = strings.TrimSpace(rest[braceClose+1:])
	} else {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return Metric{}, false
		}
		name = parts[0]
		valueStr = parts[1]
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return Metric{}, false
	}

	return Metric{
		Name:   name,
		Help:   helpMap[name],
		Type:   typeMap[name],
		Labels: labels,
		Value:  value,
	}, true
}

// parseLabels converts a `key="val",key2="val2"` string into a map.
// Correctly handles commas inside quoted values.
func parseLabels(s string) map[string]string {
	labels := make(map[string]string)
	if s == "" {
		return labels
	}

	for _, pair := range splitLabelPairs(s) {
		eq := strings.IndexByte(pair, '=')
		if eq == -1 {
			continue
		}
		key := strings.TrimSpace(pair[:eq])
		val := strings.TrimSpace(pair[eq+1:])
		val = strings.Trim(val, `"`)
		if key != "" {
			labels[key] = val
		}
	}

	return labels
}

// splitLabelPairs splits label pairs without splitting on commas inside quotes.
func splitLabelPairs(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false

	for _, c := range s {
		switch c {
		case '"':
			inQuote = !inQuote
			cur.WriteRune(c)
		case ',':
			if inQuote {
				cur.WriteRune(c)
			} else {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(c)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}
