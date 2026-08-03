package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/EchoPing07/image-mcp-hub/internal/config"
	"github.com/EchoPing07/image-mcp-hub/internal/stats"
	"github.com/EchoPing07/image-mcp-hub/internal/storage"
)

// TestCallTool_EndToEnd drives the real /mcp streamable HTTP transport end
// to end. It proves three things the white-box tests above cannot, because
// they call callTool directly and bypass the transport:
//   - the Bearer token is actually enforced (wrong token is rejected);
//   - the WithHTTPContextFunc base-URL injection reaches the tool handler;
//   - therefore the returned URL is ABSOLUTE (http://host/images/...) and not
//     a bare relative path.
//
// If mcp-go ever stops threading the request context into the tool handler,
// the URL assertion below flips to relative and this test fails loudly.
func TestCallTool_EndToEnd(t *testing.T) {
	// mock upstream
	upMux := http.NewServeMux()
	upMux.HandleFunc("/v1/images/generations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 1,
			"data":    []map[string]any{{"b64_json": pngB64}},
		})
	})
	up := httptest.NewServer(upMux)
	defer up.Close()

	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := stats.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := config.Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	mgr.Reload(&config.Config{
		Server:  config.Server{Host: "127.0.0.1", Port: 12300, McpToken: "tok", AdminPassword: "p"},
		Storage: config.Storage{Dir: dir},
		Models: []config.Model{{
			Name: "mock_img", ModelID: "m", BaseURL: up.URL + "/v1", APIKeys: []string{"k"},
		}},
	})
	svc := New(mgr, store, st)

	ts := httptest.NewServer(svc.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Wrong Bearer token must be rejected (401 at initialize).
	badCli, err := client.NewStreamableHttpClient(ts.URL, transport.WithHTTPHeaders(map[string]string{
		"Authorization": "Bearer wrong",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := badCli.Start(ctx); err != nil {
		t.Fatalf("start bad client: %v", err)
	}
	if _, err := badCli.Initialize(ctx, mcp.InitializeRequest{Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ClientInfo:     mcp.Implementation{Name: "bad", Version: "1"},
	}}); err == nil {
		t.Fatal("expected initialize to fail with wrong token, got success")
	}
	badCli.Close()

	// 2. Correct token: initialize, call tool, expect an ABSOLUTE url.
	cli, err := client.NewStreamableHttpClient(ts.URL, transport.WithHTTPHeaders(map[string]string{
		"Authorization": "Bearer tok",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Start(ctx); err != nil {
		t.Fatalf("start client: %v", err)
	}
	defer cli.Close()
	if _, err := cli.Initialize(ctx, mcp.InitializeRequest{Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ClientInfo:     mcp.Implementation{Name: "e2e", Version: "1"},
	}}); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = "mock_img"
	req.Params.Arguments = map[string]any{"prompt": "red cube"}
	res, err := cli.CallTool(ctx, req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatal("no content")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	// The assertion that matters: the URL is absolute, rooted at the server's
	// host. A relative path would mean the context injection never ran.
	wantPrefix := ts.URL + "/images/"
	if !strings.HasPrefix(tc.Text, wantPrefix) {
		t.Fatalf("expected absolute url %q, got %q", wantPrefix, tc.Text)
	}
	file := strings.TrimPrefix(tc.Text, wantPrefix)
	if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
		t.Fatalf("image file missing: %v", err)
	}
}
