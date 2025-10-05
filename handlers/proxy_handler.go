package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/goverture/goxy/config"
)

// stripForwardingHeaders removes X-Forwarded-* and similar before the upstream call.
type stripForwardingHeaders struct{ base http.RoundTripper }

func (t stripForwardingHeaders) RoundTrip(r *http.Request) (*http.Response, error) {
	// Nuke any forwarding headers (proxy won’t add them back).
	r.Header.Del("X-Forwarded-For")
	r.Header.Del("X-Forwarded-Host")
	r.Header.Del("X-Forwarded-Proto")
	r.Header.Del("Forwarded")
	r.Header.Del("X-Real-IP")
	return t.base.RoundTrip(r)
}

func NewProxyHandler() http.Handler {
	upstreamURL := config.Cfg.OpenAIBaseURL
	upstream, err := url.Parse(upstreamURL)
	if err != nil {
		panic("invalid upstream URL: " + err.Error())
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	upHost := upstream.Host

	// Rewrite outbound request
	orig := proxy.Director
	proxy.Director = func(r *http.Request) {
		orig(r)

		// Ensure correct Host/SNI for upstream/WAF
		r.Host = upHost

		// Low-latency streaming: avoid gzip buffering
		r.Header.Del("Accept-Encoding")

		// Stable UA (some WAFs dislike empty UA)
		if r.Header.Get("User-Agent") == "" {
			r.Header.Set("User-Agent", "goprx/1.0")
		}

		// Inject Authorization from env only if client didn't send one
		// if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		// 	r.Header.Set("Authorization", "Bearer "+key)
		// }

		// Helpful for SSE
		r.Header.Set("Cache-Control", "no-cache")

		// Forward proto info if not present
		if r.Header.Get("X-Forwarded-Proto") == "" {
			if r.TLS != nil {
				r.Header.Set("X-Forwarded-Proto", "https")
			} else {
				r.Header.Set("X-Forwarded-Proto", "http")
			}
		}

		// Optional: org/project from env (don’t overwrite client)
		if org := os.Getenv("OPENAI_ORG"); org != "" && r.Header.Get("OpenAI-Organization") == "" {
			r.Header.Set("OpenAI-Organization", org)
		}
		if proj := os.Getenv("OPENAI_PROJECT"); proj != "" && r.Header.Get("OpenAI-Project") == "" {
			r.Header.Set("OpenAI-Project", proj)
		}
	}

	// Stream-friendly transport, wrapped to strip X-Forwarded-* headers
	baseTransport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		ForceAttemptHTTP2:   true, // gRPC/http2 streams pass through
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  true, // don't re-compress/buffer
	}
	proxy.Transport = stripForwardingHeaders{base: baseTransport}

	// Flush chunks quickly (good for token streams)
	proxy.FlushInterval = 50 * time.Millisecond

	// Add CORS on the way out (useful for browsers) + disable buffering on some proxies
	// Additionally, intercept JSON responses to log their contents before forwarding.
	proxy.ModifyResponse = func(resp *http.Response) error {
		// CORS headers
		if origin := resp.Request.Header.Get("Origin"); origin != "" {
			h := resp.Header
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Expose-Headers", "Content-Type, OpenAI-Processing-Ms")
			h.Set("X-Accel-Buffering", "no")
		}

		ct := resp.Header.Get("Content-Type")
		// Only attempt to parse if it's JSON and not an event stream (SSE)
		if strings.Contains(ct, "application/json") && !strings.Contains(ct, "text/event-stream") && resp.Body != nil {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				// Restore an empty body so downstream doesn't panic
				resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				return nil // best-effort logging; don't fail the response
			}

			// Reset body for downstream before any heavy processing to minimize latency impact
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// Try to parse JSON
			var parsed interface{}
			if err := json.Unmarshal(bodyBytes, &parsed); err == nil {
				pretty, _ := json.MarshalIndent(parsed, "", "  ")
				fmt.Println("[proxy] Upstream JSON response:\n" + string(pretty))
			} else {
				fmt.Println("[proxy] Failed to parse JSON response:", err)
			}
		}
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the authorization key from the request header
		auth := r.Header.Get("Authorization")

		// Remove the "Bearer " prefix if it exists
		if len(auth) > 7 && auth[:7] == "Bearer " {
			auth = auth[7:]
		}

		fmt.Println("Received API Key:", auth) // Debug log to verify the extracted key

		// Forward the request to the upstream server
		proxy.ServeHTTP(w, r)
	})
}
