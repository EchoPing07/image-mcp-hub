package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Meta is the sidecar JSON stored next to each image.
type Meta struct {
	Model    string         `json:"model"`
	ModelID  string         `json:"model_id"`
	Prompt   string         `json:"prompt"`
	Params   map[string]any `json:"params,omitempty"`
	Time     time.Time      `json:"time"`
	Upstream map[string]any `json:"upstream,omitempty"`
	File     string         `json:"file"`
}

// Storage manages image files and their sidecar metadata on disk.
type Storage struct {
	dir string
}

// New creates the storage directory if needed.
func New(dir string) (*Storage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Storage{dir: dir}, nil
}

// Dir returns the on-disk directory.
func (s *Storage) Dir() string { return s.dir }

// Save writes the image bytes and a sidecar .meta.json. Returns the file name
// and the URL path (/images/<file>).
func (s *Storage) Save(data []byte, ext string, meta Meta) (filename, urlPath string, err error) {
	if ext == "" {
		ext = "png"
	}
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	ts := time.Now().Format("20060102-150405")
	name := fmt.Sprintf("%s_%s.%s", ts, uuid.NewString(), ext)
	full := filepath.Join(s.dir, name)
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", "", err
	}
	meta.File = name
	if meta.Time.IsZero() {
		meta.Time = time.Now()
	}
	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(s.dir, name+".meta.json"), metaBytes, 0o644); err != nil {
		// The image itself is saved and usable; a missing sidecar just means
		// the admin browser won't show prompt/params for this image. Log so
		// the failure isn't silently swallowed.
		log.Printf("storage: write meta for %s failed: %v", name, err)
	}
	return name, "/images/" + name, nil
}

// Item is one image plus its metadata for browsing.
type Item struct {
	Name string    `json:"name"`
	URL  string    `json:"url"`
	Time time.Time `json:"time"`
	Meta *Meta     `json:"meta,omitempty"`
}

// List returns all images newest-first, each with its sidecar meta if present.
func (s *Storage) List() ([]Item, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var items []Item
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".meta.json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		it := Item{Name: n, URL: "/images/" + n, Time: info.ModTime()}
		if mb, err := os.ReadFile(filepath.Join(s.dir, n+".meta.json")); err == nil {
			var mm Meta
			if json.Unmarshal(mb, &mm) == nil {
				it.Meta = &mm
			}
		}
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Time.After(items[j].Time) })
	return items, nil
}

// MetaFor returns the sidecar meta for an image file name.
func (s *Storage) MetaFor(name string) (*Meta, error) {
	mb, err := os.ReadFile(filepath.Join(s.dir, name+".meta.json"))
	if err != nil {
		return nil, err
	}
	var mm Meta
	if err := json.Unmarshal(mb, &mm); err != nil {
		return nil, err
	}
	return &mm, nil
}

// Delete removes an image and its sidecar meta.
func (s *Storage) Delete(name string) error {
	if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(filepath.Join(s.dir, name+".meta.json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Clean applies the two independent retention rules. A file is deleted if it
// is at least maxAgeDays old OR falls outside the newest maxCount. A zero
// value for a rule disables that rule; both zero keeps everything.
func (s *Storage) Clean(maxAgeDays, maxCount int) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	type imgFile struct {
		name string
		t    time.Time
	}
	var imgs []imgFile
	now := time.Now()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".meta.json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		imgs = append(imgs, imgFile{n, info.ModTime()})
	}
	// newest first
	sort.Slice(imgs, func(i, j int) bool { return imgs[i].t.After(imgs[j].t) })
	for i, f := range imgs {
		expired := maxAgeDays > 0 && now.Sub(f.t) >= time.Duration(maxAgeDays)*24*time.Hour
		overCount := maxCount > 0 && i >= maxCount
		if expired || overCount {
			_ = os.Remove(filepath.Join(s.dir, f.name))
			_ = os.Remove(filepath.Join(s.dir, f.name+".meta.json"))
		}
	}

	// Sweep orphan sidecars: a .meta.json whose image was deleted (by a rule
	// hit above, a crash between the two removes, or an external delete).
	// They'd otherwise leak forever since the loop above only lists non-meta
	// files.
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasSuffix(n, ".meta.json") {
			continue
		}
		base := strings.TrimSuffix(n, ".meta.json")
		if _, err := os.Stat(filepath.Join(s.dir, base)); os.IsNotExist(err) {
			_ = os.Remove(filepath.Join(s.dir, n))
		}
	}
}
