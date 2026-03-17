package collectors

import (
	"strings"
)

type NetworkCollector struct{}

func NewNetworkCollector() *NetworkCollector { return &NetworkCollector{} }
func (c *NetworkCollector) Name() string     { return "network" }

func (c *NetworkCollector) Collect() []Metric {
	var metrics []Metric

	perfData, err := psJSON(`Get-CimInstance Win32_PerfFormattedData_Tcpip_NetworkInterface | ` +
		`Select-Object Name, BytesReceivedPersec, BytesSentPersec, ` +
		`PacketsReceivedPersec, PacketsSentPersec, ` +
		`PacketsReceivedErrors, PacketsOutboundErrors | ConvertTo-Json`)
	if err != nil {
		return nil
	}

	for _, p := range perfData {
		name := getString(p, "Name")
		if strings.Contains(strings.ToLower(name), "loopback") {
			continue
		}

		rxBytes := getFloat(p, "BytesReceivedPersec")
		txBytes := getFloat(p, "BytesSentPersec")
		if rxBytes == 0 && txBytes == 0 {
			continue
		}

		safeName := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
				return r
			}
			return '_'
		}, name)

		labels := map[string]string{"device": safeName}

		metrics = append(metrics,
			Metric{Name: "node_network_receive_bytes_total", Help: "Network bytes received per sec.", Type: Gauge, Labels: labels, Value: rxBytes},
			Metric{Name: "node_network_receive_packets_total", Help: "Network packets received per sec.", Type: Gauge, Labels: labels, Value: getFloat(p, "PacketsReceivedPersec")},
			Metric{Name: "node_network_receive_errs_total", Help: "Network receive errors.", Type: Gauge, Labels: labels, Value: getFloat(p, "PacketsReceivedErrors")},
			Metric{Name: "node_network_transmit_bytes_total", Help: "Network bytes sent per sec.", Type: Gauge, Labels: labels, Value: txBytes},
			Metric{Name: "node_network_transmit_packets_total", Help: "Network packets sent per sec.", Type: Gauge, Labels: labels, Value: getFloat(p, "PacketsSentPersec")},
			Metric{Name: "node_network_transmit_errs_total", Help: "Network transmit errors.", Type: Gauge, Labels: labels, Value: getFloat(p, "PacketsOutboundErrors")},
		)
	}

	return metrics
}
