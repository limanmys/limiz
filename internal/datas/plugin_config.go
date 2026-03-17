package datas

// DataPluginsConfig corresponds to the "datas.plugins" field in config.json.
type DataPluginsConfig struct {
	Enabled        bool             `json:"enabled"`
	Dir            string           `json:"dir"`
	DefaultTimeout string           `json:"default_timeout"`
	Items          []DataPluginItem `json:"items"`
}

// DataPluginItem defines a single data plugin.
type DataPluginItem struct {
	Name          string   `json:"name"`
	Exec          string   `json:"exec"`
	Args          []string `json:"args"`
	Timeout       string   `json:"timeout"`
	CacheInterval string   `json:"cache_interval,omitempty"` // overrides global datas.cache.interval for this plugin
	Enabled       bool     `json:"enabled"`
}
