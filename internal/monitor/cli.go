package monitor

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

// RunCLI reads the monitor database and prints a summary.
// durationStr can be "" (default 1h) or "2h", "6h", "12h" etc. Max 24h.
func RunCLI(dbPath string, durationStr string) {
	dur := 1 * time.Hour
	if durationStr != "" {
		parsed, err := time.ParseDuration(durationStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid duration: %q (example: 1h, 2h, 6h, 12h)\n", durationStr)
			os.Exit(1)
		}
		if parsed < 1*time.Minute {
			fmt.Fprintf(os.Stderr, "Duration must be at least 1m.\n")
			os.Exit(1)
		}
		if parsed > 24*time.Hour {
			fmt.Fprintf(os.Stderr, "Duration cannot exceed 24h.\n")
			os.Exit(1)
		}
		dur = parsed
	}

	now := time.Now()
	since := now.Add(-dur)

	records, err := ReadRecords(dbPath, since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read monitor data: %v\n", err)
		os.Exit(1)
	}

	if len(records) == 0 {
		fmt.Println("No records found in the last", formatDuration(dur)+".")
		fmt.Printf("Database: %s\n", effectivePath(dbPath))
		return
	}

	// Separate by type
	var metricRecs, dataRecs []Record
	var metricErrors, dataErrors []Record

	for _, r := range records {
		switch r.Type {
		case RecordMetric:
			metricRecs = append(metricRecs, r)
			if r.Error != "" {
				metricErrors = append(metricErrors, r)
			}
		case RecordData:
			dataRecs = append(dataRecs, r)
			if r.Error != "" {
				dataErrors = append(dataErrors, r)
			}
		}
	}

	sep := strings.Repeat("─", 60)

	fmt.Println(sep)
	fmt.Printf("  Limiz Monitor — Last %s\n", formatDuration(dur))
	fmt.Printf("  Period: %s — %s\n", since.Format("2006-01-02 15:04:05"), now.Format("15:04:05"))
	fmt.Printf("  Database: %s\n", effectivePath(dbPath))
	fmt.Println(sep)

	// Metric summary
	fmt.Println()
	fmt.Println("  METRIC COLLECTION DURATIONS")
	fmt.Println("  " + strings.Repeat("─", 40))
	if len(metricRecs) == 0 {
		fmt.Println("  No records")
	} else {
		printStats(metricRecs)
	}

	// Data summary
	fmt.Println()
	fmt.Println("  DATA COLLECTION DURATIONS")
	fmt.Println("  " + strings.Repeat("─", 40))
	if len(dataRecs) == 0 {
		fmt.Println("  No records")
	} else {
		printStats(dataRecs)
	}

	// Errors (last 24h worth or the requested window, whichever is bigger for errors)
	allErrors := append(metricErrors, dataErrors...)
	fmt.Println()
	fmt.Println("  ERRORS")
	fmt.Println("  " + strings.Repeat("─", 40))
	if len(allErrors) == 0 {
		fmt.Println("  No errors ✓")
	} else {
		// Show last 20 errors max
		start := 0
		if len(allErrors) > 20 {
			start = len(allErrors) - 20
			fmt.Printf("  ... %d more errors\n", start)
		}
		for _, e := range allErrors[start:] {
			ts := time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05")
			fmt.Printf("  [%s] [%s] %s\n", ts, e.Type, e.Error)
		}
		fmt.Printf("\n  Total errors: %d (metric: %d, data: %d)\n",
			len(allErrors), len(metricErrors), len(dataErrors))
	}

	// Hourly breakdown if duration > 1h
	if dur > 1*time.Hour {
		fmt.Println()
		fmt.Println("  HOURLY BREAKDOWN")
		fmt.Println("  " + strings.Repeat("─", 40))
		printHourlyBreakdown(records, since, now)
	}

	fmt.Println()
	fmt.Println(sep)
}

func printStats(recs []Record) {
	var sum, min, max float64
	min = math.MaxFloat64

	for _, r := range recs {
		d := r.DurationMs
		sum += d
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}

	avg := sum / float64(len(recs))

	// p95
	durations := make([]float64, len(recs))
	for i, r := range recs {
		durations[i] = r.DurationMs
	}
	sortFloat64s(durations)
	p95Idx := int(float64(len(durations)) * 0.95)
	if p95Idx >= len(durations) {
		p95Idx = len(durations) - 1
	}
	p95 := durations[p95Idx]

	errCount := 0
	for _, r := range recs {
		if r.Error != "" {
			errCount++
		}
	}

	fmt.Printf("  Total       : %d records\n", len(recs))
	fmt.Printf("  Average     : %.2f ms\n", avg)
	fmt.Printf("  Min / Max / P95 : %.2f ms / %.2f ms / %.2f ms\n", min, max, p95)
	if errCount > 0 {
		fmt.Printf("  Errors      : %d\n", errCount)
	}
}

func printHourlyBreakdown(records []Record, since, now time.Time) {
	// Group by hour buckets
	type hourBucket struct {
		hour        string
		metricCount int
		metricAvgMs float64
		metricSumMs float64
		dataCount   int
		dataAvgMs   float64
		dataSumMs   float64
		errors      int
	}

	buckets := make(map[string]*hourBucket)
	var bucketKeys []string

	for _, r := range records {
		ts := time.Unix(r.Timestamp, 0)
		key := ts.Format("2006-01-02 15:00")
		b, ok := buckets[key]
		if !ok {
			b = &hourBucket{hour: key}
			buckets[key] = b
			bucketKeys = append(bucketKeys, key)
		}

		switch r.Type {
		case RecordMetric:
			b.metricCount++
			b.metricSumMs += r.DurationMs
		case RecordData:
			b.dataCount++
			b.dataSumMs += r.DurationMs
		}
		if r.Error != "" {
			b.errors++
		}
	}

	// Sort keys
	sortStrings(bucketKeys)

	fmt.Printf("  %-18s %8s %10s %8s %10s %6s\n",
		"Hour", "M.Count", "M.Avg(ms)", "D.Count", "D.Avg(ms)", "Errors")
	fmt.Printf("  %-18s %8s %10s %8s %10s %6s\n",
		strings.Repeat("─", 18),
		strings.Repeat("─", 8),
		strings.Repeat("─", 10),
		strings.Repeat("─", 8),
		strings.Repeat("─", 10),
		strings.Repeat("─", 6))

	for _, key := range bucketKeys {
		b := buckets[key]
		mAvg := float64(0)
		if b.metricCount > 0 {
			mAvg = b.metricSumMs / float64(b.metricCount)
		}
		dAvg := float64(0)
		if b.dataCount > 0 {
			dAvg = b.dataSumMs / float64(b.dataCount)
		}

		errStr := "—"
		if b.errors > 0 {
			errStr = fmt.Sprintf("%d", b.errors)
		}

		fmt.Printf("  %-18s %8d %10.2f %8d %10.2f %6s\n",
			b.hour, b.metricCount, mAvg, b.dataCount, dAvg, errStr)
	}
}

func formatDuration(d time.Duration) string {
	if d >= 24*time.Hour {
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d day(s)", days)
	}
	if d >= time.Hour {
		hours := int(d.Hours())
		return fmt.Sprintf("%d hour(s)", hours)
	}
	mins := int(d.Minutes())
	return fmt.Sprintf("%d minute(s)", mins)
}

func effectivePath(dbPath string) string {
	if dbPath == "" {
		return DefaultDBPath
	}
	return dbPath
}

func sortFloat64s(a []float64) {
	// Simple insertion sort (slices are typically small)
	for i := 1; i < len(a); i++ {
		key := a[i]
		j := i - 1
		for j >= 0 && a[j] > key {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = key
	}
}

func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		key := a[i]
		j := i - 1
		for j >= 0 && a[j] > key {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = key
	}
}
