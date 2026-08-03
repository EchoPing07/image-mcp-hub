package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

// Server holds HTTP server settings.
type Server struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	McpToken      string `json:"mcp_token"`
	AdminPassword string `json:"admin_password"`
}

// Storage holds image storage + cleanup settings.
type Storage struct {
	Dir        string `json:"dir"`
	MaxAgeDays int    `json:"max_age_days"`
	MaxCount   int    `json:"max_count"`
}

// Model is one configured image-generation model = one MCP tool.
type Model struct {
	Name        string   `json:"name"`
	ModelID     string   `json:"model_id"`
	BaseURL     string   `json:"base_url"`
	APIKeys     []string `json:"api_keys"`
	KeyIndex    int      `json:"key_index"`
	Description string   `json:"description"`
}

// Config is the whole config.json document.
type Config struct {
	Server  Server  `json:"server"`
	Storage Storage `json:"storage"`
	Models  []Model `json:"models"`
}

var nameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,63}$`)

// ValidName reports whether alias is a valid MCP tool name.
func ValidName(name string) bool { return nameRe.MatchString(name) }

// Default returns the built-in default configuration.
func Default() *Config {
	return &Config{
		Server:  Server{Host: "0.0.0.0", Port: 12300, McpToken: "sk-123456", AdminPassword: "password"},
		Storage: Storage{Dir: "./data/images", MaxAgeDays: 0, MaxCount: 0},
		Models:  []Model{},
	}
}

// Manager owns the live config, persisted file, hot-reload, and key rotation.
type Manager struct {
	mu       sync.RWMutex
	path     string
	cfg      *Config
	dirty    bool // advanced key cursors not yet persisted
	onChange []func(*Config)
}

// Load reads config.json at path, creating a default one if absent.
func Load(path string) (*Manager, error) {
	m := &Manager{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := Default()
			if err := writeFile(path, cfg); err != nil {
				return nil, err
			}
			m.cfg = cfg
			return m, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.applyDefaults()
	m.cfg = &cfg
	return m, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 12300
	}
	if c.Server.McpToken == "" {
		c.Server.McpToken = "sk-123456"
	}
	if c.Server.AdminPassword == "" {
		c.Server.AdminPassword = "password"
	}
	if c.Storage.Dir == "" {
		c.Storage.Dir = "./data/images"
	}
	if c.Models == nil {
		c.Models = []Model{}
	}
}

func writeFile(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	// 0o600: config.json holds plaintext api keys, mcp token and admin password.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Get returns a deep copy of the current config.
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg.clone()
}

func (c *Config) clone() *Config {
	cp := *c
	cp.Models = make([]Model, len(c.Models))
	for i := range c.Models {
		cp.Models[i] = c.Models[i]
		cp.Models[i].APIKeys = append([]string(nil), c.Models[i].APIKeys...)
	}
	return &cp
}

// OnChange registers a listener fired after every Reload. Not fired for NextKey.
func (m *Manager) OnChange(fn func(*Config)) {
	m.mu.Lock()
	m.onChange = append(m.onChange, fn)
	m.mu.Unlock()
}

// persistLocked writes m.cfg to disk and clears the dirty flag. Caller holds m.mu.
func (m *Manager) persistLocked() error {
	if err := writeFile(m.path, m.cfg); err != nil {
		return err
	}
	m.dirty = false
	return nil
}

// Reload replaces the live config, persists it, and fires listeners. On persist
// failure the previous config is restored so memory and disk stay consistent.
// Listeners receive a clone snapshot, not the live pointer.
func (m *Manager) Reload(cfg *Config) error {
	cfg.applyDefaults()
	m.mu.Lock()
	old := m.cfg
	m.cfg = cfg
	if err := m.persistLocked(); err != nil {
		m.cfg = old
		m.mu.Unlock()
		return err
	}
	snapshot := cfg.clone()
	listeners := append([]func(*Config){}, m.onChange...)
	m.mu.Unlock()
	for _, fn := range listeners {
		fn(snapshot)
	}
	return nil
}

// Update applies fn to the live config under the write lock and persists the
// result atomically. On any error from fn or from persist, the prior config is
// restored, so concurrent NextKey rotations and admin edits can never clobber
// each other (the lost-update race the old Get→mutate→Reload path had).
// Listeners receive a clone snapshot.
func (m *Manager) Update(fn func(*Config) error) error {
	m.mu.Lock()
	snapshot := m.cfg.clone()
	if err := fn(m.cfg); err != nil {
		m.cfg = snapshot
		m.mu.Unlock()
		return err
	}
	if err := m.persistLocked(); err != nil {
		m.cfg = snapshot
		m.mu.Unlock()
		return err
	}
	listeners := append([]func(*Config){}, m.onChange...)
	clone := m.cfg.clone()
	m.mu.Unlock()
	for _, fn := range listeners {
		fn(clone)
	}
	return nil
}

// Flush persists any pending (dirty) state, primarily advanced key cursors from
// NextKey. It is driven by a periodic ticker and on shutdown so NextKey can
// stay O(1) and free of disk I/O while still surviving restarts. Safe to call
// concurrently and a no-op when clean.
func (m *Manager) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.dirty {
		return nil
	}
	if err := m.persistLocked(); err != nil {
		log.Printf("config: persist failed: %v", err)
		return err
	}
	return nil
}

// NextKey returns the next API key for modelName (round-robin) and advances
// the in-memory cursor. The cursor is persisted lazily by Flush rather than on
// every call, so a single generation no longer rewrites the whole config (and
// its plaintext keys) to disk. On a hard crash at most one flush window of
// rotation is lost; on graceful shutdown Flush is called before exit.
func (m *Manager) NextKey(modelName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.cfg.Models {
		if m.cfg.Models[i].Name != modelName {
			continue
		}
		mm := &m.cfg.Models[i]
		if len(mm.APIKeys) == 0 {
			return "", fmt.Errorf("model %q has no api keys", modelName)
		}
		idx := mm.KeyIndex % len(mm.APIKeys)
		key := mm.APIKeys[idx]
		mm.KeyIndex = (idx + 1) % len(mm.APIKeys)
		m.dirty = true
		return key, nil
	}
	return "", fmt.Errorf("model %q not found", modelName)
}

// Path returns the config file path.
func (m *Manager) Path() string { return m.path }
