// Package stats collects per-model request statistics for the admin dashboard.
//
// Stats are kept in memory and periodically persisted to a JSON file next to
// the data directory (data/stats.json by default), so totals survive restarts.
package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	maxRecent = 100 // recent calls kept in memory
	dailyDays = 30  // daily buckets kept for the trend chart
)

// CallRecord is one finished tool call, reported by the MCP layer.
type CallRecord struct {
	Model      string
	OK         bool
	DurationMS int64
	Images     int
	Error      string
}

// ModelStats aggregates counters for a single model name.
type ModelStats struct {
	Requests int64 `json:"requests"`
	Success  int64 `json:"success"`
	Failures int64 `json:"failures"`
	Images   int64 `json:"images"`
	TotalMS  int64 `json:"total_ms"`
}

// DayStat is one calendar day of counters (keyed by "2006-01-02").
type DayStat struct {
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
	Success  int64  `json:"success"`
	Failures int64  `json:"failures"`
}

// RecentCall is one entry in the recent-activity list (newest first).
type RecentCall struct {
	Time       time.Time `json:"time"`
	Model      string    `json:"model"`
	OK         bool      `json:"ok"`
	DurationMS int64     `json:"duration_ms"`
	Images     int       `json:"images"`
	Error      string    `json:"error,omitempty"`
}

// Snapshot is the read-only view served to the admin API.
type Snapshot struct {
	TotalRequests int64                  `json:"total_requests"`
	TotalSuccess  int64                  `json:"total_success"`
	TotalFailures int64                  `json:"total_failures"`
	TotalImages   int64                  `json:"total_images"`
	TotalMS       int64                  `json:"total_ms"`
	Since         time.Time              `json:"since"`
	Models        map[string]*ModelStats `json:"models"`
	Daily         []DayStat              `json:"daily"`
	Recent        []RecentCall           `json:"recent"`
}

// file is the on-disk shape of the persisted stats.
type file struct {
	Since  time.Time              `json:"since"`
	Models map[string]*ModelStats `json:"models"`
	Daily  map[string]DayStat     `json:"daily"`
	Recent []RecentCall           `json:"recent"`
}

// Stats owns the counters. All methods are safe for concurrent use.
type Stats struct {
	mu     sync.Mutex
	path   string
	since  time.Time
	models map[string]*ModelStats
	daily  map[string]DayStat
	recent []RecentCall
	dirty  bool
}

// New loads existing stats from dir/stats.json, or starts fresh if absent.
func New(dir string) (*Stats, error) {
	path := filepath.Join(dir, "stats.json")
	s := &Stats{
		path:   path,
		since:  time.Now(),
		models: map[string]*ModelStats{},
		daily:  map[string]DayStat{},
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if !f.Since.IsZero() {
		s.since = f.Since
	}
	if f.Models != nil {
		s.models = f.Models
	}
	if f.Daily != nil {
		s.daily = f.Daily
	}
	s.recent = f.Recent
	return s, nil
}

// Record adds one finished call to the counters.
func (s *Stats) Record(rec CallRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty = true

	ms := s.models[rec.Model]
	if ms == nil {
		ms = &ModelStats{}
		s.models[rec.Model] = ms
	}
	ms.Requests++
	ms.TotalMS += rec.DurationMS
	if rec.OK {
		ms.Success++
		ms.Images += int64(rec.Images)
	} else {
		ms.Failures++
	}

	day := time.Now().Format("2006-01-02")
	d := s.daily[day]
	d.Date = day
	d.Requests++
	if rec.OK {
		d.Success++
	} else {
		d.Failures++
	}
	s.daily[day] = d

	rc := RecentCall{
		Time:       time.Now(),
		Model:      rec.Model,
		OK:         rec.OK,
		DurationMS: rec.DurationMS,
		Images:     rec.Images,
		Error:      rec.Error,
	}
	s.recent = append([]RecentCall{rc}, s.recent...)
	if len(s.recent) > maxRecent {
		s.recent = s.recent[:maxRecent]
	}
}

// Snapshot returns a deep copy of the current counters, with the daily series
// padded to the last 30 days (ascending) for charting.
func (s *Stats) Snapshot() *Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := &Snapshot{
		Since:  s.since,
		Models: make(map[string]*ModelStats, len(s.models)),
	}
	for k, v := range s.models {
		cp := *v
		snap.Models[k] = &cp
		snap.TotalRequests += v.Requests
		snap.TotalSuccess += v.Success
		snap.TotalFailures += v.Failures
		snap.TotalImages += v.Images
		snap.TotalMS += v.TotalMS
	}

	now := time.Now()
	for i := dailyDays - 1; i >= 0; i-- {
		key := now.AddDate(0, 0, -i).Format("2006-01-02")
		d, ok := s.daily[key]
		if !ok {
			d = DayStat{Date: key}
		}
		snap.Daily = append(snap.Daily, d)
	}

	snap.Recent = append([]RecentCall(nil), s.recent...)
	return snap
}

// Save persists the counters if anything changed since the last save.
func (s *Stats) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f := file{Since: s.since, Models: s.models, Daily: s.daily, Recent: s.recent}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	s.dirty = false
	return nil
}
