package datas

import (
	"encoding/json"
	"sync"
	"time"
)

// Provider is the interface all datas providers implement.
type Provider interface {
	Name() string
	Collect() (any, error)
}

// cacheIntervalProvider is an optional interface a Provider can implement
// to advertise a per-provider cache refresh interval.
type cacheIntervalProvider interface {
	CacheInterval() string
}

// DatasConfig controls the /datas endpoint behavior.
type DatasConfig struct {
	Enabled    bool               `json:"enabled"`
	Path       string             `json:"path,omitempty"`       // default "/datas"
	BasicAuth  *bool              `json:"basic_auth,omitempty"` // nil=auth (when top-level set), true=auth, false=no auth
	TLS        bool               `json:"tls,omitempty"`        // reuse main TLS
	Cache      *DatasCacheConfig  `json:"cache,omitempty"`
	Categories CategoriesConfig   `json:"categories,omitempty"`
	Plugins    *DataPluginsConfig `json:"plugins,omitempty"`
}

// DatasCacheConfig controls caching for the /datas endpoint.
// Interval is the global default — used by any provider that does not define its own.
type DatasCacheConfig struct {
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"` // e.g. "30s", "5m"
}

// CategoryConfig holds the enabled flag and an optional per-category cache interval.
// It accepts two JSON forms:
//
//	shorthand:  "services": true
//	full:       "services": { "enabled": true, "cache_interval": "5m" }
type CategoryConfig struct {
	Enabled       bool
	CacheInterval string
}

func (c *CategoryConfig) UnmarshalJSON(data []byte) error {
	// Accept plain boolean shorthand: "services": true
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		c.Enabled = b
		return nil
	}
	// Accept full object: "services": { "enabled": true, "cache_interval": "5m" }
	var obj struct {
		Enabled       bool   `json:"enabled"`
		CacheInterval string `json:"cache_interval,omitempty"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	c.Enabled = obj.Enabled
	c.CacheInterval = obj.CacheInterval
	return nil
}

func (c *CategoryConfig) MarshalJSON() ([]byte, error) {
	if c.CacheInterval == "" {
		return json.Marshal(c.Enabled)
	}
	return json.Marshal(struct {
		Enabled       bool   `json:"enabled"`
		CacheInterval string `json:"cache_interval,omitempty"`
	}{c.Enabled, c.CacheInterval})
}

// CategoriesConfig controls which data categories are collected.
type CategoriesConfig struct {
	Services   *CategoryConfig `json:"services,omitempty"`
	Packages   *CategoryConfig `json:"packages,omitempty"`
	Updates    *CategoryConfig `json:"updates,omitempty"`
	DiskHealth *CategoryConfig `json:"disk_health,omitempty"`
	Hardware   *CategoryConfig `json:"hardware,omitempty"`
	OS         *CategoryConfig `json:"os,omitempty"`
	Ports      *CategoryConfig `json:"ports,omitempty"`
}

func catEnabled(c *CategoryConfig) bool {
	return c != nil && c.Enabled
}

func catInterval(c *CategoryConfig) string {
	if c == nil {
		return ""
	}
	return c.CacheInterval
}

// providerEntry pairs a provider with its optional cache interval override.
type providerEntry struct {
	provider      Provider
	cacheInterval string // "" = use global default
}

// Registry manages datas providers and produces the combined JSON output.
type Registry struct {
	entries []providerEntry
}

// NewRegistry creates a Registry with providers based on the category config.
func NewRegistry(cats CategoriesConfig) *Registry {
	r := &Registry{}
	if catEnabled(cats.Services) {
		r.entries = append(r.entries, providerEntry{&ServicesProvider{}, catInterval(cats.Services)})
	}
	if catEnabled(cats.Packages) {
		r.entries = append(r.entries, providerEntry{&PackagesProvider{}, catInterval(cats.Packages)})
	}
	if catEnabled(cats.Updates) {
		r.entries = append(r.entries, providerEntry{&UpdatesProvider{}, catInterval(cats.Updates)})
	}
	if catEnabled(cats.DiskHealth) {
		r.entries = append(r.entries, providerEntry{&DiskHealthProvider{}, catInterval(cats.DiskHealth)})
	}
	if catEnabled(cats.Hardware) {
		r.entries = append(r.entries, providerEntry{&HardwareProvider{}, catInterval(cats.Hardware)})
	}
	if catEnabled(cats.OS) {
		r.entries = append(r.entries, providerEntry{&OSProvider{}, catInterval(cats.OS)})
	}
	if catEnabled(cats.Ports) {
		r.entries = append(r.entries, providerEntry{&PortsProvider{}, catInterval(cats.Ports)})
	}
	return r
}

// RegisterPlugin adds a data plugin provider to the registry.
// If the plugin implements cacheIntervalProvider, its interval override is used.
func (r *Registry) RegisterPlugin(p Provider) {
	interval := ""
	if cip, ok := p.(cacheIntervalProvider); ok {
		interval = cip.CacheInterval()
	}
	r.entries = append(r.entries, providerEntry{provider: p, cacheInterval: interval})
}

// CollectJSON gathers data from all providers and returns JSON bytes.
// Used when cache is disabled (live collection on every request).
func (r *Registry) CollectJSON() []byte {
	result := make(map[string]any)
	now := time.Now()
	result["timestamp"] = now.UTC().Format(time.RFC3339)
	result["timezone"] = now.Format("-07:00")
	for _, e := range r.entries {
		data, err := e.provider.Collect()
		if err != nil {
			result[e.provider.Name()] = map[string]string{"error": err.Error()}
		} else {
			result[e.provider.Name()] = data
		}
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	return out
}

// perProviderState holds the last collected result for a single provider.
type perProviderState struct {
	mu     sync.RWMutex
	data   any
	errMsg string
	ready  bool // true once at least one collection has completed
}

// DatasCache manages per-provider periodic collection with independent refresh intervals.
// Each provider runs in its own goroutine. Providers without an interval override
// use the global interval passed to NewDatasCache.
type DatasCache struct {
	registry       *Registry
	globalInterval time.Duration
	states         map[string]*perProviderState
	stopCh         chan struct{}
	OnRefresh      func(elapsed time.Duration, err error) // called after each per-provider refresh
}

// NewDatasCache creates a new cache. globalInterval is used for providers that have
// no per-provider cache_interval configured.
func NewDatasCache(registry *Registry, globalInterval time.Duration) *DatasCache {
	states := make(map[string]*perProviderState, len(registry.entries))
	for _, e := range registry.entries {
		states[e.provider.Name()] = &perProviderState{}
	}
	return &DatasCache{
		registry:       registry,
		globalInterval: globalInterval,
		states:         states,
		stopCh:         make(chan struct{}),
	}
}

// Start launches one background goroutine per provider.
// The first collection for each provider runs immediately (non-blocking).
func (dc *DatasCache) Start() {
	for _, e := range dc.registry.entries {
		e := e // capture
		interval := dc.resolveInterval(e.cacheInterval)
		go dc.runProvider(e.provider, interval)
	}
}

// resolveInterval returns the per-provider override if valid, otherwise the global interval.
func (dc *DatasCache) resolveInterval(override string) time.Duration {
	if override != "" {
		if d, err := time.ParseDuration(override); err == nil && d >= 1*time.Second {
			return d
		}
	}
	return dc.globalInterval
}

func (dc *DatasCache) runProvider(p Provider, interval time.Duration) {
	state := dc.states[p.Name()]
	dc.collect(p, state) // first collection immediately inside goroutine
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			dc.collect(p, state)
		case <-dc.stopCh:
			return
		}
	}
}

func (dc *DatasCache) collect(p Provider, state *perProviderState) {
	start := time.Now()
	data, err := p.Collect()
	elapsed := time.Since(start)

	state.mu.Lock()
	if err != nil {
		state.errMsg = err.Error()
		state.data = nil
	} else {
		state.data = data
		state.errMsg = ""
	}
	state.ready = true
	state.mu.Unlock()

	if dc.OnRefresh != nil {
		dc.OnRefresh(elapsed, err)
	}
}

// Get assembles and returns the current cached JSON output.
// Returns {"status":"initializing"} if any provider has not yet completed its first collection.
func (dc *DatasCache) Get() []byte {
	result := make(map[string]any)
	now := time.Now()
	result["timestamp"] = now.UTC().Format(time.RFC3339)
	result["timezone"] = now.Format("-07:00")

	for _, e := range dc.registry.entries {
		state := dc.states[e.provider.Name()]
		state.mu.RLock()
		if !state.ready {
			state.mu.RUnlock()
			return []byte(`{"status":"initializing"}`)
		}
		if state.errMsg != "" {
			result[e.provider.Name()] = map[string]string{"error": state.errMsg}
		} else {
			result[e.provider.Name()] = state.data
		}
		state.mu.RUnlock()
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return out
}

// Stop halts all per-provider background goroutines.
func (dc *DatasCache) Stop() {
	close(dc.stopCh)
}
