package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/EchoPing07/image-mcp-hub/internal/config"
	"github.com/EchoPing07/image-mcp-hub/internal/stats"
	"github.com/EchoPing07/image-mcp-hub/internal/storage"
)

// 1x1 transparent PNG.
const pngB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M8AAAMBAQDJ/pLvAAAAAElFTkSuQmCC"

func TestCallTool_FullFlow(t *testing.T) {
	// mock upstream: returns one b64_json image
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/images/generations", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-key-A" && auth != "Bearer sk-key-B" {
			http.Error(w, "bad key", http.StatusUnauthorized)
			return
		}
		// ensure response_format is NOT sent by the hub
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["response_format"]; ok {
			t.Errorf("hub must not send response_format")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 1,
			"data":    []map[string]any{{"b64_json": pngB64}},
		})
	})
	up := httptest.NewServer(mux)
	defer up.Close()

	// temp storage
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := stats.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// config manager with one model pointing at the mock (base_url includes /v1)
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	mgr.Reload(&config.Config{
		Server:  config.Server{Host: "127.0.0.1", Port: 12300, McpToken: "tok", AdminPassword: "p"},
		Storage: config.Storage{Dir: dir},
		Models: []config.Model{{
			Name:    "mock_img",
			ModelID: "mock-1",
			BaseURL: up.URL + "/v1", // exercises the /v1-suffix branch
			APIKeys: []string{"sk-key-A", "sk-key-B"},
		}},
	})

	svc := New(mgr, store, st)

	// call the tool handler directly (white-box)
	ctx := context.WithValue(context.Background(), baseURLKey, "http://hub.local:12300")
	req := mcp.CallToolRequest{}
	req.Params.Name = "mock_img"
	req.Params.Arguments = map[string]any{"prompt": "a red cube", "size": "1024x1024"}

	res, err := svc.callTool(ctx, req)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result: %+v", res)
	}
	if len(res.Content) == 0 {
		t.Fatal("no content")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	url := tc.Text
	if !strings.HasPrefix(url, "http://hub.local:12300/images/") {
		t.Fatalf("unexpected url: %s", url)
	}

	// file + sidecar exist
	file := strings.TrimPrefix(url, "http://hub.local:12300/images/")
	if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
		t.Fatalf("image file missing: %v", err)
	}
	mb, err := os.ReadFile(filepath.Join(dir, file+".meta.json"))
	if err != nil {
		t.Fatalf("meta file missing: %v", err)
	}
	var meta storage.Meta
	if err := json.Unmarshal(mb, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Model != "mock_img" || meta.Prompt != "a red cube" || meta.ModelID != "mock-1" {
		t.Fatalf("unexpected meta: %+v", meta)
	}
	if meta.Params["size"] != "1024x1024" {
		t.Fatalf("params not stored: %+v", meta.Params)
	}

	// key_index advanced and persisted (0 -> 1)
	got := mgr.Get()
	if got.Models[0].KeyIndex != 1 {
		t.Fatalf("key_index = %d, want 1", got.Models[0].KeyIndex)
	}

	// the saved bytes decode back to the original PNG
	raw, _ := base64.StdEncoding.DecodeString(pngB64)
	saved, _ := os.ReadFile(filepath.Join(dir, file))
	if string(saved) != string(raw) {
		t.Fatalf("saved image bytes mismatch")
	}

	// stats recorded: 1 success, 1 image
	snap := st.Snapshot()
	if snap.TotalRequests != 1 || snap.TotalSuccess != 1 || snap.TotalFailures != 0 || snap.TotalImages != 1 {
		t.Fatalf("stats totals: %+v", snap)
	}
	ms := snap.Models["mock_img"]
	if ms == nil || ms.Requests != 1 || ms.Success != 1 {
		t.Fatalf("per-model stats: %+v", ms)
	}
	if len(snap.Recent) != 1 || !snap.Recent[0].OK || snap.Recent[0].Model != "mock_img" {
		t.Fatalf("recent calls: %+v", snap.Recent)
	}
}

func TestCallTool_URLBranch(t *testing.T) {
	// base_url WITHOUT /v1 suffix -> hub appends /v1/images/generations
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/images/generations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 1,
			"data":    []map[string]any{{"b64_json": pngB64}},
		})
	})
	up := httptest.NewServer(mux)
	defer up.Close()

	dir := t.TempDir()
	store, _ := storage.New(dir)
	st, _ := stats.New(t.TempDir())
	mgr, _ := config.Load(filepath.Join(t.TempDir(), "config.json"))
	mgr.Reload(&config.Config{
		Server:  config.Server{Host: "127.0.0.1", Port: 12300, McpToken: "tok", AdminPassword: "p"},
		Storage: config.Storage{Dir: dir},
		Models: []config.Model{{
			Name: "u", ModelID: "m", BaseURL: up.URL, APIKeys: []string{"k"},
		}},
	})
	svc := New(mgr, store, st)

	req := mcp.CallToolRequest{}
	req.Params.Name = "u"
	req.Params.Arguments = map[string]any{"prompt": "x"}
	res, err := svc.callTool(context.Background(), req)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got: %+v", res.Content)
	}
}
