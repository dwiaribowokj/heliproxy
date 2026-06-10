package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	configPath := resolveConfigPath()
	store := NewConfigStore(configPath)
	loaded, err := store.LoadOrCreate()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	cfg := loaded.Config
	app := NewApp(store, cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           app.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("heliproxy listening on %s", addr)
	log.Printf("config path: %s", configPath)
	if loaded.Created {
		log.Printf("created initial config")
	}
	log.Printf("rpc endpoint: http://localhost:%d/?api-key=%s", cfg.Server.Port, firstOrPlaceholder(cfg.Auth.ClientKeys))
	log.Printf("dashboard: http://localhost:%d/dashboard?api-key=%s", cfg.Server.Port, firstOrPlaceholder(cfg.Auth.AdminKeys))
	log.Printf("helius keys configured: %d", len(cfg.Helius.Keys))
	if len(cfg.Helius.Keys) == 0 {
		log.Printf("no helius keys configured; add one through dashboard or set HELIUS_API_KEYS before first run")
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func firstOrPlaceholder(values []string) string {
	if len(values) == 0 || strings.TrimSpace(values[0]) == "" {
		return "<missing>"
	}
	return values[0]
}
