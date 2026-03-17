package collectors

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type NetworkCollector struct{}

func NewNetworkCollector() *NetworkCollector { return &NetworkCollector{} }
func (c *NetworkCollector) Name() string     { return "network" }

func (c *NetworkCollector) Collect() []Metric {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil
	}
	defer f.Close()

	var metrics []Metric
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		// Skip header lines
		if lineNum <= 2 {
			continue
		}

		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])
		if iface == "lo" {
			continue // skip loopback
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		parse := func(idx int) float64 {
			v, _ := strconv.ParseFloat(fields[idx], 64)
			return v
		}

		labels := map[string]string{"device": iface}

		// Receive fields (0-7)
		metrics = append(metrics,
			Metric{Name: "node_network_receive_bytes_total", Help: "Network bytes received.", Type: Counter, Labels: labels, Value: parse(0)},
			Metric{Name: "node_network_receive_packets_total", Help: "Network packets received.", Type: Counter, Labels: labels, Value: parse(1)},
			Metric{Name: "node_network_receive_errs_total", Help: "Network receive errors.", Type: Counter, Labels: labels, Value: parse(2)},
			Metric{Name: "node_network_receive_drop_total", Help: "Network receive drops.", Type: Counter, Labels: labels, Value: parse(3)},
		)

		// Transmit fields (8-15)
		metrics = append(metrics,
			Metric{Name: "node_network_transmit_bytes_total", Help: "Network bytes transmitted.", Type: Counter, Labels: labels, Value: parse(8)},
			Metric{Name: "node_network_transmit_packets_total", Help: "Network packets transmitted.", Type: Counter, Labels: labels, Value: parse(9)},
			Metric{Name: "node_network_transmit_errs_total", Help: "Network transmit errors.", Type: Counter, Labels: labels, Value: parse(10)},
			Metric{Name: "node_network_transmit_drop_total", Help: "Network transmit drops.", Type: Counter, Labels: labels, Value: parse(11)},
		)
	}

	return metrics
}
