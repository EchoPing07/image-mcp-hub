package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/EchoPing07/image-mcp-hub/internal/config"
	"github.com/EchoPing07/image-mcp-hub/internal/stats"
	"github.com/EchoPing07/image-mcp-hub/internal/storage"
)

const sessionCookie = "ims_session"
const sessionTTL = 24 * time.Hour

// Service implements the /admin/api/* JSON endpoints.
type Service struct {
	cfg   *config.Manager
	store *storage.Storage
	stats *stats.Stats
	mu    sync.Mutex
	sess  map[string]time.Time
}

func New(cfg *config.Manager, store *storage.Storage, st *stats.Stats) *Service {
	return &Service{cfg: cfg, store: store, stats: st, sess: map[string]time.Time{}}
}

// Routes returns a mux mounted at the given prefix (e.g. "/admin/api/").
func (s *Service) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("GET /session", s.auth(s.handleSession))

	mux.HandleFunc("GET /stats", s.auth(s.handleStats))

	mux.HandleFunc("GET /config", s.auth(s.handleGetConfig))
	mux.HandleFunc("PUT /config", s.auth(s.handlePutConfig))

	mux.HandleFunc("GET /models", s.auth(s.handleListModels))
	mux.HandleFunc("POST /models", s.auth(s.handleCreateModel))
	mux.HandleFunc("PUT /models/{name}", s.auth(s.handleUpdateModel))
	mux.HandleFunc("DELETE /models/{name}", s.auth(s.handleDeleteModel))

	mux.HandleFunc("GET /images", s.auth(s.handleListImages))
	mux.HandleFunc("GET /images/{name}/meta", s.auth(s.handleImageMeta))
	mux.HandleFunc("DELETE /images/{name}", s.auth(s.handleDeleteImage))
	return mux
}

// ---- helpers ----

func (s *Service) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || !s.validSession(c.Value) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Service) validSession(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.sess[id]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.sess, id)
		return false
	}
	return true
}

func (s *Service) newSession() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	s.mu.Lock()
	s.sess[id] = time.Now().Add(sessionTTL)
	s.mu.Unlock()
	return id
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

// ---- session ----

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Password != s.cfg.Get().Server.AdminPassword {
		writeErr(w, http.StatusUnauthorized, "wrong password")
		return
	}
	id := s.newSession()
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: id, Path: "/", HttpOnly: true,
		MaxAge:   int(sessionTTL.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.mu.Lock()
		delete(s.sess, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Service) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- stats ----

func (s *Service) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.stats.Snapshot())
}

// ---- config ----

func (s *Service) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	c := s.cfg.Get()
	// api keys are returned for management; this admin surface is password-protected.
	writeJSON(w, http.StatusOK, c)
}

func (s *Service) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Server  config.Server  `json:"server"`
		Storage config.Storage `json:"storage"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Server.Port < 0 || body.Server.Port > 65535 {
		writeErr(w, http.StatusBadRequest, "invalid port")
		return
	}
	cur := s.cfg.Get()
	cur.Server = body.Server
	cur.Storage = body.Storage
	if err := s.cfg.Reload(cur); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cur)
}

// ---- models ----

func (s *Service) handleListModels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Get().Models)
}

func (s *Service) handleCreateModel(w http.ResponseWriter, r *http.Request) {
	var m config.Model
	if !readJSON(w, r, &m) {
		return
	}
	if !config.ValidName(m.Name) {
		writeErr(w, http.StatusBadRequest, "name must match ^[a-zA-Z][a-zA-Z0-9_]{0,63}$")
		return
	}
	cur := s.cfg.Get()
	for _, ex := range cur.Models {
		if ex.Name == m.Name {
			writeErr(w, http.StatusConflict, "model name already exists")
			return
		}
	}
	m.KeyIndex = 0
	m.APIKeys = cleanKeys(m.APIKeys)
	cur.Models = append(cur.Models, m)
	if err := s.cfg.Reload(cur); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Service) handleUpdateModel(w http.ResponseWriter, r *http.Request) {
	oldName := r.PathValue("name")
	var m config.Model
	if !readJSON(w, r, &m) {
		return
	}
	if !config.ValidName(m.Name) {
		writeErr(w, http.StatusBadRequest, "name must match ^[a-zA-Z][a-zA-Z0-9_]{0,63}$")
		return
	}
	cur := s.cfg.Get()
	idx := -1
	for i, ex := range cur.Models {
		if ex.Name == oldName {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "model not found")
		return
	}
	// name uniqueness (excluding self)
	if m.Name != oldName {
		for i, ex := range cur.Models {
			if i != idx && ex.Name == m.Name {
				writeErr(w, http.StatusConflict, "model name already exists")
				return
			}
		}
	}
	m.APIKeys = cleanKeys(m.APIKeys)
	// preserve rotation cursor, clamped to the new key list
	if len(m.APIKeys) > 0 {
		m.KeyIndex = cur.Models[idx].KeyIndex % len(m.APIKeys)
	} else {
		m.KeyIndex = 0
	}
	cur.Models[idx] = m
	if err := s.cfg.Reload(cur); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Service) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	cur := s.cfg.Get()
	out := cur.Models[:0]
	found := false
	for _, ex := range cur.Models {
		if ex.Name == name {
			found = true
			continue
		}
		out = append(out, ex)
	}
	if !found {
		writeErr(w, http.StatusNotFound, "model not found")
		return
	}
	cur.Models = out
	if err := s.cfg.Reload(cur); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func cleanKeys(keys []string) []string {
	out := keys[:0]
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k != "" {
			out = append(out, k)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// ---- images ----

func (s *Service) handleListImages(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Service) handleImageMeta(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	meta, err := s.store.MetaFor(name)
	if err != nil {
		writeErr(w, http.StatusNotFound, "meta not found")
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Service) handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.store.Delete(name); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// DescTemplates returns ready-made tool descriptions the Web UI can offer.
func DescTemplates() []string {
	return []string{
		"Generate an image from a text prompt and return a local image URL.",
		"Create a high-quality image based on the description. Supports size, quality and style options.",
		"Generate images from a prompt. Pass size (e.g. 1024x1024) and n to control output.",
		"Text-to-image tool. Returns one or more local image URLs.",
	}
}
