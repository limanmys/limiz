// folder-size: Limiz data plugin — returns directory size and file count as JSON.
//
// Usage:
//
// folder-size --collect --path /var/log [--path /var/lib/limiz ...]
// folder-size --describe
//
// Output (JSON):
//
// [
//
//	{"path": "/var/log", "size_bytes": 1073741824, "file_count": 1247},
//	{"path": "/tmp",     "size_bytes": 524288,     "file_count": 42}
//
// ]
//
// Works on both Linux and Windows — no external dependencies.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// pathList allows the --path flag to be used multiple times.
type pathList []string

func (p *pathList) String() string     { return fmt.Sprintf("%v", []string(*p)) }
func (p *pathList) Set(v string) error { *p = append(*p, v); return nil }

type folderEntry struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SizeHuman string `json:"size_human"`
	FileCount int64  `json:"file_count"`
	Error     string `json:"error,omitempty"`
}

func main() {
	var paths pathList
	collect := flag.Bool("collect", false, "Collect data and write to stdout as JSON")
	describe := flag.Bool("describe", false, "Show plugin metadata")
	flag.Var(&paths, "path", "Directory to measure (can be specified multiple times)")
	flag.Parse()

	switch {
	case *describe:
		printDescribe()
	case *collect:
		if len(paths) == 0 {
			fmt.Fprintln(os.Stderr, "error: at least one --path must be specified")
			os.Exit(1)
		}
		printJSON(paths)
	default:
		fmt.Fprintln(os.Stderr, "Usage: folder-size --collect --path <dir> [--path <dir2> ...]")
		os.Exit(2)
	}
}

func printDescribe() {
	fmt.Println(`{
  "name": "folder-size",
  "type": "data",
  "version": "1.0.0",
  "description": "Returns directory size (bytes) and file count as JSON",
  "author":  "limiz",
  "platform": ["linux", "windows"]
}`)
}

func printJSON(paths []string) {
	entries := make([]folderEntry, 0, len(paths))
	for _, p := range paths {
		size, count, err := measureDir(p)
		e := folderEntry{
			Path:      p,
			SizeBytes: size,
			SizeHuman: humanSize(size),
			FileCount: count,
		}
		if err != nil {
			e.Error = err.Error()
			fmt.Fprintf(os.Stderr, "warning: could not read %s: %v\n", p, err)
		}
		entries = append(entries, e)
	}

	out, _ := json.MarshalIndent(entries, "", "  ")
	fmt.Println(string(out))
}

func humanSize(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// measureDir recursively scans the directory and returns total size and file count.
func measureDir(root string) (totalBytes, fileCount int64, err error) {
	info, err := os.Stat(root)
	if err != nil {
		return 0, 0, fmt.Errorf("directory inaccessible: %w", err)
	}
	if !info.IsDir() {
		return 0, 0, fmt.Errorf("'%s' is not a directory", root)
	}

	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipped '%s': %v\n", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		totalBytes += fi.Size()
		fileCount++
		return nil
	})

	return totalBytes, fileCount, walkErr
}
