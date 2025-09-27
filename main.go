package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/goverture/goxy/handlers"
	"github.com/joho/godotenv"
)

func main() {
	// Load variables from a local .env file if present (ignored if missing).
	// This lets you put TARGET, OPENAI_API_KEY, etc. in a .env for local dev.
	if err := godotenv.Load(); err != nil {
		// Only log if the file exists but couldn't be parsed; a missing file is fine.
		// We check for existence manually to avoid noisy logs when there's simply no .env.
		if _, statErr := os.Stat(".env"); statErr == nil {
			log.Printf("warning: could not load .env: %v", err)
		}
	}

	target := os.Getenv("TARGET") // e.g. https://someapi.com
	if target == "" {
		log.Fatal("set TARGET, e.g. TARGET=https://someapi.com")
	}
	up, err := url.Parse(target)
	if err != nil {
		log.Fatalf("invalid TARGET: %v", err)
	}

	h := cors(handlers.NewProxyHandler(up))

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Proxying to %s on %s", up, srv.Addr)
	log.Fatal(srv.ListenAndServe())
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