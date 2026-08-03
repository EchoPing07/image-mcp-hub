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
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
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

// Reload replaces the live config, persists it, and fires listeners.
func (m *Manager) Reload(cfg *Config) error {
	cfg.applyDefaults()
	if err := writeFile(m.path, cfg); err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = cfg
	listeners := append([]func(*Config){}, m.onChange...)
	m.mu.Unlock()
	for _, fn := range listeners {
		fn(cfg)
	}
	return nil
}

// NextKey returns the next API key for modelName (round-robin) and advances
// the persisted cursor. Best-effort persistence: rotation always happens in
// memory; a write failure is logged but does not block the call.
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
		if err := writeFile(m.path, m.cfg); err != nil {
			log.Printf("config: persist key_index for %s failed: %v", modelName, err)
		}
		return key, nil
	}
	return "", fmt.Errorf("model %q not found", modelName)
}

// Path returns the config file path.
func (m *Manager) Path() string { return m.path }
