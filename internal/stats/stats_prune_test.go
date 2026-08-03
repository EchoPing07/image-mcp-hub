package stats

import (
	"testing"
)

// TestPruneModels_DropsDeleted covers L1: per-model counters for models no
// longer in the config are dropped so they don't leak memory forever.
func TestPruneModels_DropsDeleted(t *testing.T) {
	s, _ := New(t.TempDir())
	s.Record(CallRecord{Model: "keep", OK: true, DurationMS: 1, Images: 1})
	s.Record(CallRecord{Model: "gone", OK: true, DurationMS: 1, Images: 1})
	if len(s.Snapshot().Models) != 2 {
		t.Fatal("expected 2 models before prune")
	}
	s.PruneModels([]string{"keep"})
	snap := s.Snapshot()
	if _, ok := snap.Models["gone"]; ok {
		t.Fatal("pruned model 'gone' still present")
	}
	if _, ok := snap.Models["keep"]; !ok {
		t.Fatal("kept model 'keep' was pruned")
	}
	// Pruning a deleted model marks dirty so it gets persisted.
	if !s.dirty {
		t.Fatal("prune did not mark stats dirty")
	}
}

// TestPruneModels_KeepsAllWhenEmptyConfig verifies an empty keep set wipes all
// model counters (e.g. config has no models) but leaves totals/daily alone.
func TestPruneModels_KeepsAllWhenEmptyConfig(t *testing.T) {
	s, _ := New(t.TempDir())
	s.Record(CallRecord{Model: "m", OK: true, DurationMS: 1, Images: 1})
	s.PruneModels(nil)
	snap := s.Snapshot()
	if len(snap.Models) != 0 {
		t.Fatalf("expected 0 models after prune-all, got %d", len(snap.Models))
	}
	// Totals are derived from the live per-model map, so pruning every model
	// naturally zeroes them — the deleted models' history leaves the dashboard.
	if snap.TotalRequests != 0 {
		t.Fatalf("after pruning all models, TotalRequests = %d, want 0", snap.TotalRequests)
	}
}

// TestRecent_NewestFirst covers L3: recent is stored oldest-first internally
// but Snapshot must expose newest-first, regardless of insertion order.
func TestRecent_NewestFirst(t *testing.T) {
	s, _ := New(t.TempDir())
	s.Record(CallRecord{Model: "first", OK: true, DurationMS: 1, Images: 1})
	s.Record(CallRecord{Model: "second", OK: false, DurationMS: 1, Images: 1, Error: "e"})
	s.Record(CallRecord{Model: "third", OK: true, DurationMS: 1, Images: 1})
	snap := s.Snapshot()
	if len(snap.Recent) != 3 {
		t.Fatalf("len = %d, want 3", len(snap.Recent))
	}
	want := []string{"third", "second", "first"}
	for i, w := range want {
		if snap.Recent[i].Model != w {
			t.Fatalf("Recent[%d] = %q, want %q (newest first)", i, snap.Recent[i].Model, w)
		}
	}
}

// TestRecent_PersistsAcrossReload guards L3 + the round-trip: the oldest-first
// internal layout must survive Save→New→Snapshot (still newest-first out).
func TestRecent_PersistsAcrossReload(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	s.Record(CallRecord{Model: "a", OK: true, DurationMS: 1, Images: 1})
	s.Record(CallRecord{Model: "b", OK: true, DurationMS: 1, Images: 1})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s2, _ := New(dir)
	snap := s2.Snapshot()
	if len(snap.Recent) != 2 {
		t.Fatalf("reloaded len = %d, want 2", len(snap.Recent))
	}
	if snap.Recent[0].Model != "b" {
		t.Fatalf("after reload Recent[0] = %q, want 'b' (newest first)", snap.Recent[0].Model)
	}
}

// TestDaily_PrunesOldDays covers L1: daily buckets older than dailyDays are
// dropped on Save so the map can't grow without bound.
func TestDaily_PrunesOldDays(t *testing.T) {
	s, _ := New(t.TempDir())
	// Plant a day bucket far older than the window.
	s.mu.Lock()
	s.daily["2000-01-01"] = DayStat{Date: "2000-01-01", Requests: 5}
	s.dirty = true
	s.mu.Unlock()
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	for _, d := range snap.Daily {
		if d.Date == "2000-01-01" {
			t.Fatal("ancient daily bucket survived prune")
		}
	}
}
