package stats

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordSnapshotAndSave(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	s.Record(CallRecord{Model: "m1", OK: true, DurationMS: 100, Images: 2})
	s.Record(CallRecord{Model: "m1", OK: false, DurationMS: 50, Error: "boom"})
	s.Record(CallRecord{Model: "m2", OK: true, DurationMS: 30, Images: 1})

	snap := s.Snapshot()
	if snap.TotalRequests != 3 || snap.TotalSuccess != 2 || snap.TotalFailures != 1 || snap.TotalImages != 3 || snap.TotalMS != 180 {
		t.Fatalf("totals: %+v", snap)
	}
	m1 := snap.Models["m1"]
	if m1 == nil || m1.Requests != 2 || m1.Success != 1 || m1.Failures != 1 || m1.TotalMS != 150 {
		t.Fatalf("m1: %+v", m1)
	}
	if len(snap.Daily) != dailyDays {
		t.Fatalf("daily len = %d, want %d", len(snap.Daily), dailyDays)
	}
	if snap.Daily[dailyDays-1].Requests != 3 {
		t.Fatalf("last day requests = %d, want 3", snap.Daily[dailyDays-1].Requests)
	}
	if len(snap.Recent) != 3 || snap.Recent[0].Model != "m2" || snap.Recent[1].Error != "boom" {
		t.Fatalf("recent: %+v", snap.Recent)
	}

	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// reload from disk
	s2, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	snap2 := s2.Snapshot()
	if snap2.TotalRequests != 3 || snap2.TotalSuccess != 2 || snap2.TotalFailures != 1 || snap2.TotalImages != 3 {
		t.Fatalf("reloaded totals: %+v", snap2)
	}
	if len(snap2.Recent) != 3 {
		t.Fatalf("reloaded recent len = %d", len(snap2.Recent))
	}
}

func TestRecentRingCapacity(t *testing.T) {
	s, _ := New(t.TempDir())
	for i := 0; i < maxRecent+20; i++ {
		s.Record(CallRecord{Model: "m", OK: true, DurationMS: 1, Images: 1})
	}
	if len(s.Snapshot().Recent) != maxRecent {
		t.Fatalf("recent len = %d, want %d", len(s.Snapshot().Recent), maxRecent)
	}
}

func TestNoopSave(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	if err := s.Save(); err != nil { // nothing recorded, should be a no-op
		t.Fatalf("save: %v", err)
	}
	// corrupt file should surface a parse error
	if err := os.WriteFile(filepath.Join(dir, "stats.json"), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir); err == nil {
		t.Fatal("expected parse error for corrupt stats.json")
	}
}
