package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EchoPing07/image-mcp-hub/internal/config"
	"github.com/EchoPing07/image-mcp-hub/internal/stats"
	"github.com/EchoPing07/image-mcp-hub/internal/storage"
)

func newTestService(t *testing.T) (*Service, *config.Manager) {
	t.Helper()
	mgr, err := config.Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	mgr.Reload(&config.Config{
		Server:  config.Server{Host: "0.0.0.0", Port: 12300, McpToken: "tok", AdminPassword: "pw"},
		Storage: config.Storage{Dir: t.TempDir()},
		Models:  []config.Model{},
	})
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	st, err := stats.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(mgr, store, st), mgr
}

// adminServer mounts the admin routes exactly like main.go does.
func adminServer(t *testing.T, svc *Service) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/admin/api/", http.StripPrefix("/admin/api", svc.Routes()))
	return httptest.NewServer(mux)
}

// jarClient returns an http.Client that keeps session cookies.
func jarClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

// do is a tiny helper that sends JSON and returns status + body.
func do(c *http.Client, method, url string, body any) (*http.Response, []byte) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp, out
}

func TestLogin_WrongPassword_401(t *testing.T) {
	svc, _ := newTestService(t)
	srv := adminServer(t, svc)
	defer srv.Close()
	c := jarClient()
	resp, _ := do(c, http.MethodPost, srv.URL+"/admin/api/login", map[string]string{"password": "nope"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password: status %d, want 401", resp.StatusCode)
	}
}

func TestLogin_CorrectThenSessionWorks(t *testing.T) {
	svc, _ := newTestService(t)
	srv := adminServer(t, svc)
	defer srv.Close()
	c := jarClient()

	// Wrong first (covers M4: constant-time compare still rejects cleanly).
	resp, _ := do(c, http.MethodPost, srv.URL+"/admin/api/login", map[string]string{"password": "nope"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong: %d", resp.StatusCode)
	}
	// Correct.
	resp, _ = do(c, http.MethodPost, srv.URL+"/admin/api/login", map[string]string{"password": "pw"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: status %d, want 200", resp.StatusCode)
	}
	// Session cookie carries: protected endpoint now works.
	resp, _ = do(c, http.MethodGet, srv.URL+"/admin/api/config", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /config after login: %d, want 200", resp.StatusCode)
	}
	// Without a session, protected endpoint must 401.
	fresh := jarClient()
	resp, _ = do(fresh, http.MethodGet, srv.URL+"/admin/api/config", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /config without session: %d, want 401", resp.StatusCode)
	}
}

// TestReadJSON_OversizedBody_413 covers H4: an unauthenticated /login with a
// body over the cap is rejected with 413, never buffered fully.
func TestReadJSON_OversizedBody_413(t *testing.T) {
	svc, _ := newTestService(t)
	srv := adminServer(t, svc)
	defer srv.Close()
	c := jarClient()
	// 2 MiB JSON string — over the 1 MiB admin cap.
	resp, _ := do(c, http.MethodPost, srv.URL+"/admin/api/login",
		map[string]string{"password": strings.Repeat("x", 2<<20)})
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized /login: status %d, want 413", resp.StatusCode)
	}
}

// TestCreateModel_DuplicateName_409 covers the H3 atomic uniqueness check: the
// name-conflict detection happens under the config write lock, so two rapid
// creates of the same name can't both succeed.
func TestCreateModel_DuplicateName_409(t *testing.T) {
	svc, _ := newTestService(t)
	srv := adminServer(t, svc)
	defer srv.Close()
	c := jarClient()
	resp, _ := do(c, http.MethodPost, srv.URL+"/admin/api/login", map[string]string{"password": "pw"})
	if resp.StatusCode != http.StatusOK {
		t.Fatal("login failed")
	}
	body := map[string]any{
		"name": "dupe", "model_id": "m", "base_url": "http://x", "api_keys": []string{"k"}, "description": "d",
	}
	resp, _ = do(c, http.MethodPost, srv.URL+"/admin/api/models", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first create: %d, want 201", resp.StatusCode)
	}
	resp, _ = do(c, http.MethodPost, srv.URL+"/admin/api/models", body)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate create: %d, want 409", resp.StatusCode)
	}
}

func TestDeleteModel_NotFound_404(t *testing.T) {
	svc, _ := newTestService(t)
	srv := adminServer(t, svc)
	defer srv.Close()
	c := jarClient()
	do(c, http.MethodPost, srv.URL+"/admin/api/login", map[string]string{"password": "pw"})
	resp, _ := do(c, http.MethodDelete, srv.URL+"/admin/api/models/missing", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("delete missing: %d, want 404", resp.StatusCode)
	}
}

// TestPutConfig_RestartRequired covers L7: changing host/port/storage.dir
// reports restart_required=true; changing only hot-reloadable fields reports
// false.
func TestPutConfig_RestartRequired(t *testing.T) {
	svc, mgr := newTestService(t)
	srv := adminServer(t, svc)
	defer srv.Close()
	c := jarClient()
	do(c, http.MethodPost, srv.URL+"/admin/api/login", map[string]string{"password": "pw"})

	base := mgr.Get()

	// Port change -> restart required.
	resp, body := do(c, http.MethodPut, srv.URL+"/admin/api/config", map[string]any{
		"server":  map[string]any{"host": base.Server.Host, "port": 9999, "mcp_token": base.Server.McpToken, "admin_password": base.Server.AdminPassword},
		"storage": map[string]any{"dir": base.Storage.Dir, "max_age_days": base.Storage.MaxAgeDays, "max_count": base.Storage.MaxCount},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /config (port): %d", resp.StatusCode)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["restart_required"] != true {
		t.Fatalf("port change: restart_required = %v, want true", got["restart_required"])
	}

	// Token-only change -> no restart required.
	resp, body = do(c, http.MethodPut, srv.URL+"/admin/api/config", map[string]any{
		"server":  map[string]any{"host": base.Server.Host, "port": 9999, "mcp_token": "new-token", "admin_password": base.Server.AdminPassword},
		"storage": map[string]any{"dir": base.Storage.Dir, "max_age_days": base.Storage.MaxAgeDays, "max_count": base.Storage.MaxCount},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /config (token): %d", resp.StatusCode)
	}
	got = nil
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["restart_required"] != false {
		t.Fatalf("token-only change: restart_required = %v, want false", got["restart_required"])
	}
}

// TestUpdateModel_LocksModelID verifies model_id is immutable after creation:
// it is the stable identity keying request statistics, so a rename must not
// orphan historical data. The tool name (alias) stays freely editable.
func TestUpdateModel_LocksModelID(t *testing.T) {
	svc, _ := newTestService(t)
	srv := adminServer(t, svc)
	defer srv.Close()
	c := jarClient()
	do(c, http.MethodPost, srv.URL+"/admin/api/login", map[string]string{"password": "pw"})

	resp, body := do(c, http.MethodPost, srv.URL+"/admin/api/models", map[string]any{
		"name": "m", "model_id": "original", "base_url": "http://x", "api_keys": []string{"k"}, "description": "d",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	var created config.Model
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("create did not assign a model ID")
	}

	// Rename the tool AND attempt to change model_id and ID.
	resp, body = do(c, http.MethodPut, srv.URL+"/admin/api/models/m", map[string]any{
		"id": "forged", "name": "renamed", "model_id": "changed", "base_url": "http://x", "api_keys": []string{"k"}, "description": "d",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update: %d %s", resp.StatusCode, body)
	}
	var got config.Model
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.ModelID != "original" {
		t.Fatalf("model_id changed: got %q, want %q (locked after creation)", got.ModelID, "original")
	}
	if got.ID != created.ID {
		t.Fatalf("ID changed: got %q, want %q (locked after creation)", got.ID, created.ID)
	}
	if got.Name != "renamed" {
		t.Fatalf("name not updated: got %q", got.Name)
	}

	// Confirm via the list endpoint too.
	resp, body = do(c, http.MethodGet, srv.URL+"/admin/api/models", nil)
	var list []config.Model
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ModelID != "original" || list[0].Name != "renamed" || list[0].ID != created.ID {
		t.Fatalf("list after rename: %+v", list)
	}
}

// TestStats_MultiChannelSameModelID is the core fix for the identity question:
// two configured models may share the same upstream model_id (different
// channels/base_urls). Stats must stay SEPARATE (keyed by the stable per-entry
// ID), and the dashboard must disambiguate them by appending the tool name so
// neither channel's counts are lost into the other.
func TestStats_MultiChannelSameModelID(t *testing.T) {
	svc, mgr := newTestService(t)
	srv := adminServer(t, svc)
	defer srv.Close()
	c := jarClient()
	do(c, http.MethodPost, srv.URL+"/admin/api/login", map[string]string{"password": "pw"})

	// Two channels for the same upstream model_id, different tool names.
	for _, body := range []map[string]any{
		{"name": "wan_a", "model_id": "wan2.7", "base_url": "http://a", "api_keys": []string{"k"}, "description": "d"},
		{"name": "wan_b", "model_id": "wan2.7", "base_url": "http://b", "api_keys": []string{"k"}, "description": "d"},
	} {
		resp, _ := do(c, http.MethodPost, srv.URL+"/admin/api/models", body)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create %s: %d", body["name"], resp.StatusCode)
		}
	}

	cfg := mgr.Get()
	if len(cfg.Models) != 2 || cfg.Models[0].ID == "" || cfg.Models[1].ID == "" || cfg.Models[0].ID == cfg.Models[1].ID {
		t.Fatalf("expected two distinct non-empty IDs: %+v", cfg.Models)
	}
	idA, idB := cfg.Models[0].ID, cfg.Models[1].ID

	// One successful call on channel A, one failure on channel B — same model_id.
	svc.stats.Record(stats.CallRecord{Model: idA, Label: "wan2.7", OK: true, Images: 1})
	svc.stats.Record(stats.CallRecord{Model: idB, Label: "wan2.7", OK: false, Error: "boom"})

	// Dashboard snapshot comes through the display transform.
	resp, body := do(c, http.MethodGet, srv.URL+"/admin/api/stats", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stats: %d", resp.StatusCode)
	}
	type statModel struct {
		Label    string `json:"label"`
		Requests int64  `json:"requests"`
		Success  int64  `json:"success"`
		Failures int64  `json:"failures"`
		Images   int64  `json:"images"`
		TotalMS  int64  `json:"total_ms"`
	}
	var snap struct {
		TotalRequests int64                `json:"total_requests"`
		Models        map[string]statModel `json:"models"`
	}
	if err := json.Unmarshal(body, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.TotalRequests != 2 {
		t.Fatalf("total requests = %d, want 2", snap.TotalRequests)
	}
	// Same model_id, two channels: rows must be disambiguated with the name.
	a, b := snap.Models["wan2.7 · wan_a"], snap.Models["wan2.7 · wan_b"]
	if a.Requests != 1 || a.Success != 1 {
		t.Fatalf("channel A stats wrong: %+v", a)
	}
	if b.Requests != 1 || b.Failures != 1 {
		t.Fatalf("channel B stats wrong: %+v", b)
	}
}
