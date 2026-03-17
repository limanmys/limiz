package localwriter

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/limanmys/limiz/internal/collectors"
)

// Config holds local write configuration.
type Config struct {
	Enabled   bool   `json:"enabled"`
	Format    string `json:"format,omitempty"`     // reserved for future use
	Interval  string `json:"interval"`             // e.g. "5m", "30s", "1h"
	DBPath    string `json:"db_path"`              // e.g. "./metrics.db"
	Rotate    string `json:"rotate"`               // e.g. "24h", "7d", "0" to disable
	MaxFiles  int    `json:"max_files"`            // max rotated files to keep (default 5)
	BatchSize int    `json:"batch_size,omitempty"` // metrics per transaction (default 500)
}

// Backend is the storage interface for metric persistence.
type Backend interface {
	Open(path string) error
	Write(ts time.Time, metrics []collectors.Metric) error
	Close() error
	Extension() string // ".db" or ".jsonl"
}

// Writer periodically collects metrics and writes them via a Backend.
type Writer struct {
	cfg      Config
	registry *collectors.Registry
	backend  Backend
	mu       sync.Mutex
	stopCh   chan struct{}
	wg       sync.WaitGroup

	interval time.Duration
	rotate   time.Duration
	dbStart  time.Time
}

// New creates a new Writer. Call Start() to begin writing.
func New(cfg Config, registry *collectors.Registry) (*Writer, error) {
	interval, err := parseDuration(cfg.Interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval %q: %w", cfg.Interval, err)
	}
	if interval < 5*time.Second {
		return nil, fmt.Errorf("interval must be >= 5s, got %s", interval)
	}

	var rotateDur time.Duration
	if cfg.Rotate != "" && cfg.Rotate != "0" {
		rotateDur, err = parseDuration(cfg.Rotate)
		if err != nil {
			return nil, fmt.Errorf("invalid rotate %q: %w", cfg.Rotate, err)
		}
	}

	if cfg.MaxFiles <= 0 {
		cfg.MaxFiles = 5
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}

	// Select backend
	backend := selectBackend(cfg.Format)
	log.Printf("Local writer: using %T backend", backend)

	// Normalize and fix path extension
	if cfg.DBPath == "" {
		cfg.DBPath = "metrics" + backend.Extension()
	}
	cfg.DBPath = filepath.Clean(cfg.DBPath)

	// Ensure parent directory exists
	dir := filepath.Dir(cfg.DBPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	if err := backend.Open(cfg.DBPath); err != nil {
		return nil, err
	}

	w := &Writer{
		cfg:      cfg,
		registry: registry,
		backend:  backend,
		interval: interval,
		rotate:   rotateDur,
		stopCh:   make(chan struct{}),
		dbStart:  time.Now(),
	}

	return w, nil
}

// Start begins the periodic metric writing goroutine.
func (w *Writer) Start() {
	w.wg.Add(1)
	go w.loop()
	log.Printf("Local writer started: format=%s interval=%s path=%s rotate=%s max_files=%d",
		w.backendName(), w.interval, w.cfg.DBPath, w.cfg.Rotate, w.cfg.MaxFiles)
}

// Stop gracefully stops the writer and closes the backend.
func (w *Writer) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.backend != nil {
		w.backend.Close()
	}
	log.Println("Local writer stopped.")
}

func (w *Writer) loop() {
	defer w.wg.Done()

	// Write immediately on start
	w.writeSnapshot()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if w.rotate > 0 && time.Since(w.dbStart) >= w.rotate {
				if err := w.rotateFile(); err != nil {
					log.Printf("Local writer rotation error: %v", err)
				}
			}
			w.writeSnapshot()
		case <-w.stopCh:
			w.writeSnapshot()
			return
		}
	}
}

func (w *Writer) writeSnapshot() {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	allMetrics := w.registry.CollectRaw()
	if len(allMetrics) == 0 {
		return
	}

	if err := w.backend.Write(now, allMetrics); err != nil {
		log.Printf("Local writer error: %v", err)
	}
}

func (w *Writer) rotateFile() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.backend.Close()

	// Rename with timestamp
	ts := time.Now().UTC().Format("20060102-150405")
	ext := filepath.Ext(w.cfg.DBPath)
	base := strings.TrimSuffix(w.cfg.DBPath, ext)
	rotatedName := fmt.Sprintf("%s-%s%s", base, ts, ext)

	if err := os.Rename(w.cfg.DBPath, rotatedName); err != nil {
		log.Printf("Local writer: rotation rename failed: %v", err)
	} else {
		log.Printf("Local writer: rotated -> %s", rotatedName)
		w.cleanOldFiles(base, ext)
	}

	// Reopen fresh
	if err := w.backend.Open(w.cfg.DBPath); err != nil {
		return fmt.Errorf("reopen after rotation: %w", err)
	}
	w.dbStart = time.Now()
	return nil
}

func (w *Writer) cleanOldFiles(base, ext string) {
	dir := filepath.Dir(base)
	if dir == "" {
		dir = "."
	}
	prefix := filepath.Base(base) + "-"

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var rotated []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ext) && name != filepath.Base(w.cfg.DBPath) {
			rotated = append(rotated, filepath.Join(dir, name))
		}
	}

	sort.Strings(rotated)

	for len(rotated) > w.cfg.MaxFiles {
		oldest := rotated[0]
		if err := os.Remove(oldest); err != nil {
			log.Printf("Local writer: failed to remove old file %s: %v", oldest, err)
		} else {
			log.Printf("Local writer: removed old file %s", oldest)
		}
		rotated = rotated[1:]
	}
}

func (w *Writer) backendName() string {
	return fmt.Sprintf("%T", w.backend)
}

// parseDuration supports Go durations plus "d" for days.
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(numStr, "%d", &days); err != nil {
			return 0, fmt.Errorf("invalid days: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
