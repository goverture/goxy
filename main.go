package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/goverture/goxy/config"
	"github.com/goverture/goxy/handlers"
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

	h := cors(handlers.NewProxyHandler())

	addr := ":" + itoa(config.Cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Proxying to %+v on %s", config.Cfg, srv.Addr)
	log.Fatal(srv.ListenAndServe())
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
