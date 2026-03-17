package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultDBPath is the default path for the monitor database.
	DefaultDBPath = "/var/lib/limiz/mon.db"

	// maxDays is the maximum number of days of data to retain.
	maxDays = 2
)

// RecordType identifies the kind of monitored operation.
type RecordType string

const (
	RecordMetric RecordType = "metric"
	RecordData   RecordType = "data"
)

// Record is a single monitoring entry written as a JSON line.
type Record struct {
	Timestamp  int64      `json:"ts"`          // unix timestamp (seconds)
	Type       RecordType `json:"type"`        // "metric" or "data"
	DurationMs float64    `json:"duration_ms"` // collection duration in milliseconds
	Error      string     `json:"error,omitempty"`
}

// Store handles writing and rotating the monitor database.
type Store struct {
	mu     sync.Mutex
	path   string
	file   *os.File
	enc    *json.Encoder
	dayTag string // YYYY-MM-DD of the current file
}

// NewStore opens (or creates) the monitor database file.
func NewStore(path string) (*Store, error) {
	if path == "" {
		path = DefaultDBPath
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("monitor: create directory %s: %w", dir, err)
	}

	s := &Store{
		path:   path,
		dayTag: time.Now().UTC().Format("2006-01-02"),
	}
	if err := s.openFile(); err != nil {
		return nil, err
	}

	// Clean old rotated files on startup
	s.cleanOld()

	return s, nil
}

func (s *Store) openFile() error {
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("monitor: open %s: %w", s.path, err)
	}
	s.file = f
	s.enc = json.NewEncoder(f)
	s.enc.SetEscapeHTML(false)
	return nil
}

// Write records a single monitoring entry. It rotates the file when the day changes.
func (s *Store) Write(rec Record) {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")
	if today != s.dayTag {
		s.rotateLocked()
		s.dayTag = today
	}

	if s.enc != nil {
		if err := s.enc.Encode(rec); err != nil {
			log.Printf("monitor: write error: %v", err)
		}
	}
}

// Close closes the monitor database file.
func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		s.file.Close()
		s.file = nil
		s.enc = nil
	}
}

func (s *Store) rotateLocked() {
	if s.file != nil {
		s.file.Close()
		s.file = nil
		s.enc = nil
	}

	// Rename current file with the old day tag
	ext := filepath.Ext(s.path)
	base := strings.TrimSuffix(s.path, ext)
	rotatedName := fmt.Sprintf("%s-%s%s", base, s.dayTag, ext)

	if err := os.Rename(s.path, rotatedName); err != nil {
		log.Printf("monitor: rotation rename failed: %v", err)
	} else {
		log.Printf("monitor: rotated -> %s", rotatedName)
	}

	s.cleanOld()

	// Reopen fresh file
	if err := s.openFile(); err != nil {
		log.Printf("monitor: reopen after rotation failed: %v", err)
	}
}

// cleanOld removes rotated files older than maxDays.
func (s *Store) cleanOld() {
	dir := filepath.Dir(s.path)
	ext := filepath.Ext(s.path)
	base := filepath.Base(strings.TrimSuffix(s.path, ext))

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	// Collect rotated files (pattern: base-YYYY-MM-DD.ext)
	type rotatedFile struct {
		path string
		date string
	}
	var rotated []rotatedFile

	prefix := base + "-"
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ext) {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ext)
		if _, err := time.Parse("2006-01-02", dateStr); err == nil {
			rotated = append(rotated, rotatedFile{
				path: filepath.Join(dir, name),
				date: dateStr,
			})
		}
	}

	if len(rotated) <= maxDays {
		return
	}

	// Sort by date ascending, remove oldest
	sort.Slice(rotated, func(i, j int) bool {
		return rotated[i].date < rotated[j].date
	})
	for i := 0; i < len(rotated)-maxDays; i++ {
		if err := os.Remove(rotated[i].path); err != nil {
			log.Printf("monitor: failed to remove old file %s: %v", rotated[i].path, err)
		} else {
			log.Printf("monitor: removed old file %s", rotated[i].path)
		}
	}
}

// ReadRecords reads all records from the current and rotated files
// that fall within the given time range [since, now].
func ReadRecords(dbPath string, since time.Time) ([]Record, error) {
	if dbPath == "" {
		dbPath = DefaultDBPath
	}

	var allRecords []Record

	// Read rotated files
	dir := filepath.Dir(dbPath)
	ext := filepath.Ext(dbPath)
	base := filepath.Base(strings.TrimSuffix(dbPath, ext))

	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("monitor: read directory %s: %w", dir, err)
	}

	prefix := base + "-"
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ext) {
			recs, err := readFile(filepath.Join(dir, name), since)
			if err != nil {
				log.Printf("monitor: read rotated file %s: %v", name, err)
				continue
			}
			allRecords = append(allRecords, recs...)
		}
	}

	// Read current file
	recs, err := readFile(dbPath, since)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("monitor: read %s: %w", dbPath, err)
	}
	allRecords = append(allRecords, recs...)

	// Sort by timestamp
	sort.Slice(allRecords, func(i, j int) bool {
		return allRecords[i].Timestamp < allRecords[j].Timestamp
	})

	return allRecords, nil
}

func readFile(path string, since time.Time) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sinceUnix := since.Unix()
	var records []Record
	dec := json.NewDecoder(f)

	for dec.More() {
		var rec Record
		if err := dec.Decode(&rec); err != nil {
			continue // skip malformed lines
		}
		if rec.Timestamp >= sinceUnix {
			records = append(records, rec)
		}
	}

	return records, nil
}
