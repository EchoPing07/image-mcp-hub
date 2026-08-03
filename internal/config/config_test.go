package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidName(t *testing.T) {
	good := []string{"a", "img", "wan_image", "A1", "x" + repeat("_", 62)}
	for _, n := range good {
		if !ValidName(n) {
			t.Errorf("ValidName(%q) = false, want true", n)
		}
	}
	bad := []string{"", "1abc", "_x", "has space", "dash-x", "中", "a" + repeat("b", 64)}
	for _, n := range bad {
		if ValidName(n) {
			t.Errorf("ValidName(%q) = true, want false", n)
		}
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

// TestNextKey_RoundRobin exercises the lazy cursor: it must rotate through the
// keys in order and wrap, without touching disk on each call (dirty only).
func TestNextKey_RoundRobin(t *testing.T) {
	mgr, _ := Load(filepath.Join(t.TempDir(), "config.json"))
	mgr.Reload(&Config{
		Server:  Server{Host: "127.0.0.1", Port: 12300, McpToken: "t", AdminPassword: "p"},
		Storage: Storage{Dir: "./data/images"},
		Models: []Model{{
			Name: "m", ModelID: "m", BaseURL: "http://x", APIKeys: []string{"A", "B", "C"},
		}},
	})
	want := []string{"A", "B", "C", "A", "B", "C"}
	for i, w := range want {
		got, err := mgr.NextKey("m")
		if err != nil {
			t.Fatalf("NextKey %d: %v", i, err)
		}
		if got != w {
			t.Fatalf("NextKey %d = %q, want %q", i, got, w)
		}
	}
	if got := mgr.Get().Models[0].KeyIndex; got != 0 {
		t.Fatalf("after full cycle KeyIndex = %d, want 0", got)
	}
	// dirty must be set after NextKey; not yet persisted to disk.
	mgr2, _ := Load(mgr.Path())
	if mgr2.Get().Models[0].KeyIndex != 0 {
		t.Fatalf("lazy cursor leaked to disk before Flush: %d", mgr2.Get().Models[0].KeyIndex)
	}
}

// TestFlush_PersistsCursor is the M3 guarantee: NextKey is lazy, Flush writes it.
func TestFlush_PersistsCursor(t *testing.T) {
	mgr, _ := Load(filepath.Join(t.TempDir(), "config.json"))
	mgr.Reload(&Config{
		Server:  Server{Host: "127.0.0.1", Port: 12300, McpToken: "t", AdminPassword: "p"},
		Storage: Storage{Dir: "./data/images"},
		Models: []Model{{
			Name: "m", ModelID: "m", BaseURL: "http://x", APIKeys: []string{"A", "B"},
		}},
	})
	if _, err := mgr.NextKey("m"); err != nil {
		t.Fatal(err)
	} // cursor 0 -> 1
	if err := mgr.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	mgr2, _ := Load(mgr.Path())
	if got := mgr2.Get().Models[0].KeyIndex; got != 1 {
		t.Fatalf("after Flush+reload KeyIndex = %d, want 1", got)
	}
	// A second Flush is a no-op when clean (no error, no rewrite semantics).
	if err := mgr.Flush(); err != nil {
		t.Fatalf("noop Flush: %v", err)
	}
}

// TestUpdate_PreservesConcurrentCursor is the H3 regression test. The old
// Get→mutate→Reload path read the cursor from a stale clone and clobbered an
// in-flight NextKey rotation. Update operates on the live config under the
// write lock, so the cursor survives an unrelated admin edit.
func TestUpdate_PreservesConcurrentCursor(t *testing.T) {
	mgr, _ := Load(filepath.Join(t.TempDir(), "config.json"))
	mgr.Reload(&Config{
		Server:  Server{Host: "127.0.0.1", Port: 12300, McpToken: "t", AdminPassword: "p"},
		Storage: Storage{Dir: "./data/images"},
		Models: []Model{{
			Name: "m", ModelID: "m", BaseURL: "http://x", APIKeys: []string{"A", "B"}, Description: "old",
		}},
	})
	// An in-flight tool call advances the live cursor 0 -> 1.
	if _, err := mgr.NextKey("m"); err != nil {
		t.Fatal(err)
	}
	// Meanwhile an admin edit rewrites the model (e.g. changes description),
	// preserving the cursor by reading it from the LIVE config (as
	// handleUpdateModel now does).
	if err := mgr.Update(func(c *Config) error {
		for i := range c.Models {
			if c.Models[i].Name == "m" {
				c.Models[i].Description = "new"
				if len(c.Models[i].APIKeys) > 0 {
					c.Models[i].KeyIndex = c.Models[i].KeyIndex % len(c.Models[i].APIKeys)
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := mgr.Get()
	if got.Models[0].KeyIndex != 1 {
		t.Fatalf("cursor clobbered: KeyIndex = %d, want 1 (H3 regression)", got.Models[0].KeyIndex)
	}
	if got.Models[0].Description != "new" {
		t.Fatalf("edit not applied: %q", got.Models[0].Description)
	}
	// Persisted too.
	mgr2, _ := Load(mgr.Path())
	if mgr2.Get().Models[0].KeyIndex != 1 {
		t.Fatalf("cursor not persisted after Update: %d", mgr2.Get().Models[0].KeyIndex)
	}
}

// TestUpdate_RollbackOnError verifies a failed mutation leaves the live config
// (and disk) untouched, so a validation error mid-closure can't half-apply.
func TestUpdate_RollbackOnError(t *testing.T) {
	mgr, _ := Load(filepath.Join(t.TempDir(), "config.json"))
	mgr.Reload(&Config{
		Server:  Server{Host: "127.0.0.1", Port: 12300, McpToken: "t", AdminPassword: "p"},
		Storage: Storage{Dir: "./data/images"},
		Models: []Model{{Name: "m", ModelID: "m", BaseURL: "http://x", APIKeys: []string{"A"}}},
	})
	before := mgr.Get()
	err := mgr.Update(func(c *Config) error {
		c.Models[0].Description = "should not stick"
		return errors.New("nope")
	})
	if err == nil {
		t.Fatal("expected error from Update closure")
	}
	after := mgr.Get()
	if after.Models[0].Description == "should not stick" {
		t.Fatal("mutation applied despite closure error")
	}
	if after.Models[0].Name != before.Models[0].Name {
		t.Fatal("config changed despite closure error")
	}
}

// TestConfigFilePermissions checks M6: the config file is written 0600 because
// it holds plaintext api keys and credentials.
func TestConfigFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not honored on Windows; verified on Linux/Docker")
	}
	mgr, _ := Load(filepath.Join(t.TempDir(), "config.json"))
	mgr.Reload(&Config{
		Server:  Server{Host: "127.0.0.1", Port: 12300, McpToken: "secret", AdminPassword: "pw"},
		Storage: Storage{Dir: "./data/images"},
		Models:  []Model{{Name: "m", ModelID: "m", BaseURL: "http://x", APIKeys: []string{"sk-leaked"}}},
	})
	info, err := os.Stat(mgr.Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config file mode = %o, want 0600", info.Mode().Perm())
	}
}
