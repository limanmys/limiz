package localwriter

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/limanmys/limiz/internal/collectors"
)

// JSONLBackend writes metrics as JSON Lines (.jsonl).
// Each line is a self-contained JSON object. Readable by Python, PowerShell,
// jq, and any language with JSON support.
type JSONLBackend struct {
	file *os.File
	enc  *json.Encoder
}

// jsonlRecord is a single metric record in JSONL format.
type jsonlRecord struct {
	Timestamp string  `json:"timestamp"`
	TsUnix    float64 `json:"ts_unix"`
	Name      string  `json:"name"`
	Labels    string  `json:"labels"`
	Value     float64 `json:"value"`
	Type      string  `json:"type"`
}

func (b *JSONLBackend) Extension() string { return ".jsonl" }

func (b *JSONLBackend) Open(path string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open jsonl file: %w", err)
	}
	b.file = f
	b.enc = json.NewEncoder(f)
	b.enc.SetEscapeHTML(false)
	return nil
}

func (b *JSONLBackend) Write(ts time.Time, metrics []collectors.Metric) error {
	if b.file == nil {
		return fmt.Errorf("jsonl file not open")
	}

	timestamp := ts.UTC().Format(time.RFC3339)
	tsUnix := float64(ts.UnixMilli()) / 1000.0

	for _, m := range metrics {
		rec := jsonlRecord{
			Timestamp: timestamp,
			TsUnix:    tsUnix,
			Name:      m.Name,
			Labels:    formatLabels(m.Labels),
			Value:     m.Value,
			Type:      string(m.Type),
		}
		if err := b.enc.Encode(rec); err != nil {
			return fmt.Errorf("encode jsonl: %w", err)
		}
	}

	// Flush to disk
	return b.file.Sync()
}

func (b *JSONLBackend) Close() error {
	if b.file != nil {
		err := b.file.Close()
		b.file = nil
		b.enc = nil
		return err
	}
	return nil
}

// formatLabels converts label map to a deterministic key=value,key=value string.
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, labels[k]))
	}
	return strings.Join(parts, ",")
}
