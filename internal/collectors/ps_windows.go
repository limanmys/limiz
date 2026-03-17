package collectors

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

const psTimeout = 15 * time.Second

// psJSON runs a PowerShell command that outputs JSON and unmarshals the result.
// The command should use ConvertTo-Json so we get structured data back.
// Returns a slice of maps; a single object is wrapped into a slice.
func psJSON(psCommand string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), psTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", psCommand)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}

	// Try array first
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
		return arr, nil
	}

	// Single object → wrap in array
	var single map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
		return nil, err
	}
	return []map[string]interface{}{single}, nil
}

// getFloat safely extracts a float64 from a map value.
func getFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

// getString safely extracts a string from a map value.
func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
