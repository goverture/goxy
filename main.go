package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goverture/goxy/config"
	"github.com/goverture/goxy/handlers"
	"github.com/goverture/goxy/persistence"
)

var (
	// Version information - will be set during build
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Print version information
	log.Printf("GoXY v%s (built %s, commit %s)", Version, BuildTime, GitCommit)

	// Parse CLI flags
	config.Cfg = config.ParseConfig()

	// Print the config
	log.Printf("Config: %+v", config.Cfg)

	// Create persistent limit manager with SQLite database
	dbPath := "goxy_usage.db" // Store in current directory
	limitMgr, err := persistence.NewPersistentLimitManager(config.Cfg.SpendLimitPerHour, dbPath)
	if err != nil {
		log.Fatalf("Failed to create persistent limit manager: %v", err)
	}
	defer limitMgr.Close()

	// Create proxy handler and admin handler
	proxyHandler := handlers.NewProxyHandler(limitMgr)
	h := cors(proxyHandler)

	// Create admin handler
	adminHandler := handlers.NewAdminHandler(limitMgr)

	// Setup proxy server
	addr := ":" + itoa(config.Cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	// Setup admin server
	adminAddr := ":" + itoa(config.Cfg.AdminPort)
	adminSrv := &http.Server{
		Addr:         adminAddr,
		Handler:      adminHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Proxying to %s on %s", config.Cfg.OpenAIBaseURL, srv.Addr)
	log.Printf("Admin API listening on %s", adminSrv.Addr)

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start admin server in background
	go func() {
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Admin server failed: %v", err)
		}
	}()

	// Start main proxy server in background
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Proxy server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-quit
	log.Println("Shutting down servers...")

	// Graceful shutdown (limited time)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv.Shutdown(ctx)
	adminSrv.Shutdown(ctx)

	log.Println("Servers stopped.")
}

// itoa is a minimal int to string conversion for port formatting
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

// Simple CORS (browser-friendly)
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, OpenAI-Beta")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
