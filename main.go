package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/EchoPing07/image-mcp-hub/internal/admin"
	"github.com/EchoPing07/image-mcp-hub/internal/config"
	"github.com/EchoPing07/image-mcp-hub/internal/mcpserver"
	"github.com/EchoPing07/image-mcp-hub/internal/stats"
	"github.com/EchoPing07/image-mcp-hub/internal/storage"
	"github.com/EchoPing07/image-mcp-hub/internal/web"
)

func main() {
	cfgPath := "config.json"
	if p := os.Getenv("IMAGE_MCP_HUB_CONFIG"); p != "" {
		cfgPath = p
	}

	cfgMgr, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	cfg := cfgMgr.Get()

	store, err := storage.New(cfg.Storage.Dir)
	if err != nil {
		log.Fatalf("init storage: %v", err)
	}

	// Stats live next to the data directory (data/stats.json by default).
	st, err := stats.New(filepath.Dir(cfg.Storage.Dir))
	if err != nil {
		log.Fatalf("init stats: %v", err)
	}

	mcpSvc := mcpserver.New(cfgMgr, store, st)
	adminSvc := admin.New(cfgMgr, store, st)

	// Periodic cleanup using the live (hot-reloadable) retention rules.
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for range t.C {
			c := cfgMgr.Get()
			store.Clean(c.Storage.MaxAgeDays, c.Storage.MaxCount)
		}
	}()

	// Periodically persist request statistics.
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			if err := st.Save(); err != nil {
				log.Printf("stats: save failed: %v", err)
			}
		}
	}()

	mux := http.NewServeMux()

	// /mcp — MCP streamable HTTP, Bearer-protected.
	mcpHandler := mcpSvc.Handler()
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/mcp/", mcpHandler)

	// /admin/api/* — admin JSON API.
	mux.Handle("/admin/api/", http.StripPrefix("/admin/api", adminSvc.Routes()))

	// /admin and /admin/ — SPA.
	spa := http.StripPrefix("/admin", web.Handler())
	mux.Handle("/admin", spa)
	mux.Handle("/admin/", spa)

	// /images/ — public image access (UUID names prevent enumeration).
	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(store.Dir()))))

	addr := cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port)
	log.Printf("image-mcp-hub listening on %s", addr)
	log.Printf("  MCP:   http://%s/mcp", addr)
	log.Printf("  admin: http://%s/admin", addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Printf("shutting down...")
	_ = st.Save()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
