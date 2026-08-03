package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(s *Storage, name string, age time.Duration) {
	path := filepath.Join(s.dir, name)
	_ = os.WriteFile(path, []byte("x"), 0o644)
	t := time.Now().Add(-age)
	_ = os.Chtimes(path, t, t)
}

func TestClean_CountRule(t *testing.T) {
	s, _ := New(t.TempDir())
	writeFile(s, "a.png", 0)
	writeFile(s, "b.png", 0)
	writeFile(s, "c.png", 0)
	// newest-first keep 2: a,b,c sorted by mtime all ~now; keep 2 newest, drop oldest
	s.Clean(0, 2)
	files, _ := os.ReadDir(s.dir)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestClean_AgeRule(t *testing.T) {
	s, _ := New(t.TempDir())
	writeFile(s, "old.png", 48*time.Hour)
	writeFile(s, "new.png", 1*time.Hour)
	s.Clean(2, 0) // 2 days
	if _, err := os.Stat(filepath.Join(s.dir, "old.png")); !os.IsNotExist(err) {
		t.Fatalf("old.png should be deleted")
	}
	if _, err := os.Stat(filepath.Join(s.dir, "new.png")); err != nil {
		t.Fatalf("new.png should remain")
	}
}

func TestClean_DisabledKeepsAll(t *testing.T) {
	s, _ := New(t.TempDir())
	writeFile(s, "old.png", 365*24*time.Hour)
	writeFile(s, "new.png", 0)
	s.Clean(0, 0) // both rules off
	files, _ := os.ReadDir(s.dir)
	if len(files) != 2 {
		t.Fatalf("expected 2 files kept, got %d", len(files))
	}
}

func TestClean_DeletesSidecar(t *testing.T) {
	s, _ := New(t.TempDir())
	_, _, _ = s.Save([]byte("img"), "png", Meta{Model: "m", Prompt: "p"})
	items, _ := s.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	s.Clean(0, 0) // disabled -> keeps
	items, _ = s.List()
	if len(items) != 1 {
		t.Fatalf("disabled clean should keep image")
	}
	s.Clean(0, 0) // still disabled
	// force age rule to delete everything
	writeFile(s, items[0].Name, 10*24*time.Hour)
	// also age the sidecar
	sidecar := filepath.Join(s.dir, items[0].Name+".meta.json")
	_ = os.Chtimes(sidecar, time.Now().Add(-10*24*time.Hour), time.Now().Add(-10*24*time.Hour))
	s.Clean(5, 0)
	if _, err := os.Stat(filepath.Join(s.dir, items[0].Name)); !os.IsNotExist(err) {
		t.Fatalf("image should be deleted by age")
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("sidecar should be deleted with image")
	}
}
