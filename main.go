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

	httpAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	wsAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.WSPort)
	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           app.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	wsServer := &http.Server{
		Addr:              wsAddr,
		Handler:           app.WSRoutes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("heliproxy http listening on %s", httpAddr)
	log.Printf("heliproxy websocket listening on %s", wsAddr)
	log.Printf("config path: %s", configPath)
	if loaded.Created {
		log.Printf("created initial config")
	}
	log.Printf("rpc endpoint: http://localhost:%d/?api-key=%s", cfg.Server.Port, firstOrPlaceholder(cfg.Auth.ClientKeys))
	log.Printf("websocket endpoint: ws://localhost:%d/?api-key=%s", cfg.Server.WSPort, firstOrPlaceholder(cfg.Auth.ClientKeys))
	log.Printf("dashboard: http://localhost:%d/dashboard?api-key=%s", cfg.Server.Port, firstOrPlaceholder(cfg.Auth.AdminKeys))
	log.Printf("helius keys configured: %d", len(cfg.Helius.Keys))
	if len(cfg.Helius.Keys) == 0 {
		log.Printf("no helius keys configured; add one through dashboard or set HELIUS_API_KEYS before first run")
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()
	go func() {
		if err := wsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("websocket server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}
	if err := wsServer.Shutdown(ctx); err != nil {
		log.Printf("websocket shutdown error: %v", err)
	}
}

func firstOrPlaceholder(values []string) string {
	if len(values) == 0 || strings.TrimSpace(values[0]) == "" {
		return "<missing>"
	}
	return values[0]
}
