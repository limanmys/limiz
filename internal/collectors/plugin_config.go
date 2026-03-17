package collectors

// PluginsConfig corresponds to the "plugins" field in config.json.
type PluginsConfig struct {
	Enabled        bool         `json:"enabled"`
	Dir            string       `json:"dir"`
	DefaultTimeout string       `json:"default_timeout"`
	Items          []PluginItem `json:"items"`
}

// PluginItem defines a single plugin.
type PluginItem struct {
	Name    string   `json:"name"`
	Exec    string   `json:"exec"`
	Args    []string `json:"args"`
	Timeout string   `json:"timeout"`
	Enabled bool     `json:"enabled"`
}
