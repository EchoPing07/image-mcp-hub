package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/EchoPing07/image-mcp-hub/internal/config"
	"github.com/EchoPing07/image-mcp-hub/internal/stats"
	"github.com/EchoPing07/image-mcp-hub/internal/storage"
	"github.com/google/uuid"
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
	s := &Service{cfg: cfg, store: store, stats: st, sess: map[string]time.Time{}}
	go s.sweepSessions()
	return s
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

// sweepSessions periodically drops expired sessions so dead ids don't leak in
// memory (validSession only cleans on next access, which may never happen).
func (s *Service) sweepSessions() {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for range t.C {
		s.mu.Lock()
		now := time.Now()
		for id, exp := range s.sess {
			if now.After(exp) {
				delete(s.sess, id)
			}
		}
		s.mu.Unlock()
	}
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

// httpError carries an HTTP status out of a config.Manager.Update closure so
// the handler can map it back to the right response code.
type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

func errStatus(status int, msg string) *httpError { return &httpError{status: status, msg: msg} }

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	// Cap request bodies to stop an unauthenticated /login (or any admin
	// endpoint) from OOMing the server with a huge JSON payload.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeErr(w, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
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
	if subtle.ConstantTimeCompare([]byte(body.Password), []byte(s.cfg.Get().Server.AdminPassword)) != 1 {
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
	writeJSON(w, http.StatusOK, s.displayStats(s.stats.Snapshot()))
}

// displayStats reshapes a stats.Snapshot so the dashboard sees human-readable
// model_id labels as map keys and in Recent.Model, instead of the internal
// config Model.ID keys. Labels come from the live config; entries no longer in
// the config fall back to the label captured at record time. When two
// configured models share the same model_id (multi-channel setup), each label
// is disambiguated with its tool name so the rows stay distinguishable.
//
// The output keeps the same JSON shape the frontend expects (in particular the
// `model` field on recent entries and string map keys), so the SPA needs no
// changes.
func (s *Service) displayStats(snap *stats.Snapshot) any {
	cfg := s.cfg.Get()
	labelOf := map[string]string{} // entry ID -> model_id
	nameOf := map[string]string{}  // entry ID -> tool name
	dup := map[string]int{}        // model_id occurrence count among live models
	for _, m := range cfg.Models {
		labelOf[m.ID] = m.ModelID
		nameOf[m.ID] = m.Name
		dup[m.ModelID]++
	}
	resolve := func(entryID, storedLabel string) string {
		if label, ok := labelOf[entryID]; ok {
			if dup[label] > 1 {
				return label + " · " + nameOf[entryID]
			}
			return label
		}
		if storedLabel != "" {
			return storedLabel
		}
		return entryID
	}

	type dispModel struct {
		Label    string `json:"label"`
		Requests int64  `json:"requests"`
		Success  int64  `json:"success"`
		Failures int64  `json:"failures"`
		Images   int64  `json:"images"`
		TotalMS  int64  `json:"total_ms"`
	}
	type dispRecent struct {
		Time       time.Time `json:"time"`
		Model      string    `json:"model"`
		OK         bool      `json:"ok"`
		DurationMS int64     `json:"duration_ms"`
		Images     int       `json:"images"`
		Error      string    `json:"error,omitempty"`
	}

	out := struct {
		TotalRequests int64                 `json:"total_requests"`
		TotalSuccess  int64                 `json:"total_success"`
		TotalFailures int64                 `json:"total_failures"`
		TotalImages   int64                 `json:"total_images"`
		TotalMS       int64                 `json:"total_ms"`
		Since         time.Time             `json:"since"`
		Models        map[string]*dispModel `json:"models"`
		Daily         []stats.DayStat       `json:"daily"`
		Recent        []dispRecent          `json:"recent"`
	}{
		TotalRequests: snap.TotalRequests,
		TotalSuccess:  snap.TotalSuccess,
		TotalFailures: snap.TotalFailures,
		TotalImages:   snap.TotalImages,
		TotalMS:       snap.TotalMS,
		Since:         snap.Since,
		Models:        map[string]*dispModel{},
		Daily:         snap.Daily,
	}
	for entryID, ms := range snap.Models {
		label := resolve(entryID, ms.Label)
		if ex := out.Models[label]; ex != nil {
			ex.Requests += ms.Requests
			ex.Success += ms.Success
			ex.Failures += ms.Failures
			ex.Images += ms.Images
			ex.TotalMS += ms.TotalMS
		} else {
			out.Models[label] = &dispModel{
				Label: label, Requests: ms.Requests, Success: ms.Success,
				Failures: ms.Failures, Images: ms.Images, TotalMS: ms.TotalMS,
			}
		}
	}
	out.Recent = make([]dispRecent, 0, len(snap.Recent))
	for _, r := range snap.Recent {
		out.Recent = append(out.Recent, dispRecent{
			Time: r.Time, Model: resolve(r.Model, r.Label),
			OK: r.OK, DurationMS: r.DurationMS, Images: r.Images, Error: r.Error,
		})
	}
	return out
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
	var restartRequired bool
	err := s.cfg.Update(func(c *config.Config) error {
		// port, host and storage.dir are bound at process start; everything
		// else (token, password, cleanup, models) hot-reloads immediately.
		restartRequired = c.Server.Host != body.Server.Host ||
			c.Server.Port != body.Server.Port ||
			c.Storage.Dir != body.Storage.Dir
		c.Server = body.Server
		c.Storage = body.Storage
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	c := s.cfg.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"server":            c.Server,
		"storage":           c.Storage,
		"models":            c.Models,
		"restart_required":  restartRequired,
	})
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
		writeErr(w, http.StatusBadRequest, "name must match ^[a-zA-Z][a-zA-Z0-9._-]{0,63}$")
		return
	}
	// Validate + mutate under the config write lock so a concurrent create
	// (or an in-flight NextKey) can't race the uniqueness check or clobber it.
	err := s.cfg.Update(func(c *config.Config) error {
		for _, ex := range c.Models {
			if ex.Name == m.Name {
				return errStatus(http.StatusConflict, "model name already exists")
			}
		}
		m.KeyIndex = 0
		m.APIKeys = cleanKeys(m.APIKeys)
		// Assign the stable internal ID at creation. It is the stats key and never
		// user-editable afterwards.
		m.ID = uuid.NewString()
		c.Models = append(c.Models, m)
		return nil
	})
	if err != nil {
		var he *httpError
		if errors.As(err, &he) {
			writeErr(w, he.status, he.msg)
			return
		}
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
		writeErr(w, http.StatusBadRequest, "name must match ^[a-zA-Z][a-zA-Z0-9._-]{0,63}$")
		return
	}
	err := s.cfg.Update(func(c *config.Config) error {
		idx := -1
		for i, ex := range c.Models {
			if ex.Name == oldName {
				idx = i
				break
			}
		}
		if idx < 0 {
			return errStatus(http.StatusNotFound, "model not found")
		}
		if m.Name != oldName {
			for i, ex := range c.Models {
				if i != idx && ex.Name == m.Name {
					return errStatus(http.StatusConflict, "model name already exists")
				}
			}
		}
		m.APIKeys = cleanKeys(m.APIKeys)
		// Preserve the rotation cursor read from the LIVE config (under the
		// lock), clamped to the new key list. The old clone-based code read it
		// from a stale snapshot and clobbered in-flight NextKey rotations.
		if len(m.APIKeys) > 0 {
			m.KeyIndex = c.Models[idx].KeyIndex % len(m.APIKeys)
		} else {
			m.KeyIndex = 0
		}
		// model_id is immutable after creation: it is the human-readable label used
		// in the dashboard and image metadata, so changing it would orphan the
		// displayed identity. The tool name (alias) stays freely editable. The ID
		// is the true stable stats key and is likewise preserved untouched.
		m.ModelID = c.Models[idx].ModelID
		m.ID = c.Models[idx].ID
		c.Models[idx] = m
		return nil
	})
	if err != nil {
		var he *httpError
		if errors.As(err, &he) {
			writeErr(w, he.status, he.msg)
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Service) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	err := s.cfg.Update(func(c *config.Config) error {
		found := false
		for _, ex := range c.Models {
			if ex.Name == name {
				found = true
				break
			}
		}
		if !found {
			return errStatus(http.StatusNotFound, "model not found")
		}
		out := c.Models[:0]
		for _, ex := range c.Models {
			if ex.Name == name {
				continue
			}
			out = append(out, ex)
		}
		c.Models = out
		return nil
	})
	if err != nil {
		var he *httpError
		if errors.As(err, &he) {
			writeErr(w, he.status, he.msg)
			return
		}
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
