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
