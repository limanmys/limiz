// dir-size: Limiz plugin — measures the size and file count of specified directories.
//
// Usage:
//
//	dir-size --collect --path /var/log [--path /var/lib/limiz ...]
//	dir-size --describe
//
// Output (Prometheus exposition format):
//
//	plugin_dir_size_bytes{path="/var/log"} 1.073741824e+09
//	plugin_dir_file_count{path="/var/log"} 1247
//
// Works on both Linux and Windows — no external dependencies.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// pathList allows the --path flag to be used multiple times.
type pathList []string

func (p *pathList) String() string     { return fmt.Sprintf("%v", []string(*p)) }
func (p *pathList) Set(v string) error { *p = append(*p, v); return nil }

func main() {
	var paths pathList
	collect := flag.Bool("collect", false, "Collect metrics and write to stdout")
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
		printMetrics(paths)
	default:
		fmt.Fprintln(os.Stderr, "Usage: dir-size --collect --path <dir> [--path <dir2> ...]")
		os.Exit(2)
	}
}

func printDescribe() {
	fmt.Println(`{
  "name": "dir-size",
  "version": "1.0.0",
  "description": "Measures directory size (bytes) and file count",
  "author":  "limiz",
  "platform": ["linux", "windows"]
}`)
}

func printMetrics(paths []string) {
	type result struct {
		path  string
		size  int64
		count int64
		err   error
	}

	results := make([]result, len(paths))
	for i, p := range paths {
		size, count, err := measureDir(p)
		results[i] = result{path: p, size: size, count: count, err: err}
	}

	// plugin_dir_size_bytes
	fmt.Println("# HELP plugin_dir_size_bytes Total directory size (bytes)")
	fmt.Println("# TYPE plugin_dir_size_bytes gauge")
	for _, r := range results {
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not read %s: %v\n", r.path, r.err)
			continue
		}
		fmt.Printf("plugin_dir_size_bytes{path=\"%s\"} %g\n", r.path, float64(r.size))
	}

	fmt.Println()

	// plugin_dir_file_count
	fmt.Println("# HELP plugin_dir_file_count Total number of files in directory")
	fmt.Println("# TYPE plugin_dir_file_count gauge")
	for _, r := range results {
		if r.err != nil {
			continue
		}
		fmt.Printf("plugin_dir_file_count{path=\"%s\"} %g\n", r.path, float64(r.count))
	}
}

// measureDir recursively scans the directory and returns total size and file count.
// Does not follow symbolic links; skips inaccessible subdirectories.
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
			// Skip inaccessible subdirectory, continue
			fmt.Fprintf(os.Stderr, "warning: skipped '%s': %v\n", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		// Skip symbolic links
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
