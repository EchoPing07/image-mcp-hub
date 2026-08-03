package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/EchoPing07/image-mcp-hub/internal/config"
	"github.com/EchoPing07/image-mcp-hub/internal/stats"
	"github.com/EchoPing07/image-mcp-hub/internal/storage"
	"github.com/EchoPing07/image-mcp-hub/internal/upstream"
)

type ctxKey int

const baseURLKey ctxKey = 1

// Service ties the MCP server to the live config, storage and stats.
type Service struct {
	cfg   *config.Manager
	store *storage.Storage
	stats *stats.Stats
	srv   *server.MCPServer
}

// New builds the MCP server and registers a tool per configured model.
func New(cfg *config.Manager, store *storage.Storage, st *stats.Stats) *Service {
	s := &Service{cfg: cfg, store: store, stats: st}
	s.srv = server.NewMCPServer("image-mcp-hub", "1.0.0",
		server.WithToolCapabilities(true),
	)
	s.rebuild(cfg.Get())
	cfg.OnChange(s.rebuild)
	return s
}

// Handler returns an http.Handler for /mcp. It first checks the Bearer token,
// then delegates to the streamable HTTP transport.
func (s *Service) Handler() http.Handler {
	streamable := server.NewStreamableHTTPServer(s.srv,
		server.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
				scheme = xfp
			}
			host := r.Host
			if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
				host = xfh
			}
			if host != "" {
				ctx = context.WithValue(ctx, baseURLKey, scheme+"://"+host)
			}
			return ctx
		}),
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.cfg.Get().Server.McpToken
		auth := r.Header.Get("Authorization")
		if auth == "" || auth != "Bearer "+token {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		streamable.ServeHTTP(w, r)
	})
}

func (s *Service) rebuild(c *config.Config) {
	tools := make([]server.ServerTool, 0, len(c.Models))
	for _, m := range c.Models {
		tools = append(tools, server.ServerTool{Tool: buildTool(m), Handler: s.callTool})
	}
	s.srv.SetTools(tools...)
}

func buildTool(m config.Model) mcp.Tool {
	desc := strings.TrimSpace(m.Description)
	if desc == "" {
		desc = "Generate an image from a text prompt and return a local image URL."
	}
	return mcp.NewTool(m.Name,
		mcp.WithDescription(desc),
		mcp.WithString("prompt", mcp.Required(), mcp.Description("Text prompt describing the image to generate")),
		mcp.WithString("size", mcp.Description("Image size, e.g. 1024x1024")),
		mcp.WithNumber("n", mcp.Description("Number of images to generate")),
		mcp.WithString("quality", mcp.Description("Generation quality")),
		mcp.WithString("style", mcp.Description("Generation style")),
		mcp.WithString("background", mcp.Description("Image background")),
		mcp.WithString("output_format", mcp.Description("Output file format")),
	)
}

func (s *Service) callTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start := time.Now()
	name := req.Params.Name
	record := func(ok bool, images int, errMsg string) {
		s.stats.Record(stats.CallRecord{
			Model:      name,
			OK:         ok,
			DurationMS: time.Since(start).Milliseconds(),
			Images:     images,
			Error:      trunc(errMsg, 240),
		})
	}

	c := s.cfg.Get()
	var model *config.Model
	for i := range c.Models {
		if c.Models[i].Name == name {
			model = &c.Models[i]
			break
		}
	}
	if model == nil {
		record(false, 0, "unknown tool: "+name)
		return mcp.NewToolResultError("unknown tool: " + name), nil
	}

	args := req.GetArguments()
	prompt, _ := args["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		record(false, 0, "prompt is required")
		return mcp.NewToolResultError("prompt is required"), nil
	}

	params := map[string]any{}
	for _, k := range []string{"size", "n", "quality", "style", "background", "output_format"} {
		if v, ok := args[k]; ok && v != nil {
			params[k] = v
		}
	}

	key, err := s.cfg.NextKey(name)
	if err != nil {
		record(false, 0, err.Error())
		return mcp.NewToolResultError(err.Error()), nil
	}

	res, err := upstream.Generate(ctx, model.BaseURL, key, upstream.GenerateRequest{
		Model:  model.ModelID,
		Prompt: prompt,
		Params: params,
	})
	if err != nil {
		record(false, 0, "upstream error: "+err.Error())
		return mcp.NewToolResultError(fmt.Sprintf("upstream error: %v", err)), nil
	}

	base := ""
	if v, ok := ctx.Value(baseURLKey).(string); ok {
		base = v
	}
	var urls []string
	for _, img := range res.Images {
		_, urlPath, err := s.store.Save(img.Data, img.Ext, storage.Meta{
			Model:    name,
			ModelID:  model.ModelID,
			Prompt:   prompt,
			Params:   params,
			Upstream: img.Upstream,
		})
		if err != nil {
			record(false, 0, "save image: "+err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("save image: %v", err)), nil
		}
		if base != "" {
			urls = append(urls, base+urlPath)
		} else {
			urls = append(urls, urlPath)
		}
	}
	record(true, len(urls), "")
	return mcp.NewToolResultText(strings.Join(urls, "\n")), nil
}

// trunc limits an error message kept for the recent-activity list.
func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
