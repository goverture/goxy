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
	"github.com/goverture/goxy/limit"
	"github.com/goverture/goxy/pricing"
)

// sseLoggingBody wraps an SSE (text/event-stream) body, logging each JSON event line as it passes through
type sseLoggingBody struct {
	rc  io.ReadCloser
	buf bytes.Buffer // accumulate partial lines between Read calls
}

func (s *sseLoggingBody) Read(p []byte) (int, error) {
	n, err := s.rc.Read(p)
	if n > 0 {
		// Feed the newly read bytes into line processor
		s.processChunk(p[:n])
	}
	return n, err
}

func (s *sseLoggingBody) Close() error { return s.rc.Close() }

func (s *sseLoggingBody) processChunk(chunk []byte) {
	// Append to buffer
	s.buf.Write(chunk)
	for {
		data := s.buf.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx == -1 { // no complete line yet
			return
		}
		// Extract line including up to idx
		line := string(data[:idx])
		// Remove this line + newline from buffer
		s.buf.Next(idx + 1)
		s.handleLine(strings.TrimRight(line, "\r"))
	}
}

func (s *sseLoggingBody) handleLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if !strings.HasPrefix(line, "data:") {
		return
	}
	payload := strings.TrimSpace(line[len("data:"):])
	if payload == "[DONE]" { // termination event
		fmt.Println("[proxy] SSE event: [DONE]")
		return
	}
	// Attempt JSON parse if looks like JSON
	if strings.HasPrefix(payload, "{") || strings.HasPrefix(payload, "[") {
		var v interface{}
		if err := json.Unmarshal([]byte(payload), &v); err == nil {
			pretty, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println("[proxy] SSE event JSON:\n" + string(pretty))
			return
		}
	}
	// Fallback raw payload
	fmt.Println("[proxy] SSE raw event:", payload)
}

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
	// Spend limit manager
	mgr := limit.NewManager(config.Cfg.SpendLimitPerHour)
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
		// JSON (non-SSE) full-body logging
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
			var parsed map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &parsed); err == nil {
				pretty, _ := json.MarshalIndent(parsed, "", "  ")
				fmt.Println("[proxy] Upstream JSON response:\n" + string(pretty))
				// Attempt pricing if usage + model present
				modelName, _ := parsed["model"].(string)
				if usageRaw, ok := parsed["usage"].(map[string]interface{}); ok {
					u := pricing.Usage{}
					if v, ok := usageRaw["prompt_tokens"].(float64); ok {
						u.PromptTokens = int(v)
					}
					if v, ok := usageRaw["completion_tokens"].(float64); ok {
						u.CompletionTokens = int(v)
					}
					// nested prompt_tokens_details.cached_tokens
					if detailsRaw, ok := usageRaw["prompt_tokens_details"].(map[string]interface{}); ok {
						if v, ok := detailsRaw["cached_tokens"].(float64); ok {
							u.PromptCachedTokens = int(v)
						}
					}
					if pr, err := pricing.ComputePrice(modelName, u); err == nil {
						fmt.Println(pr.String())
						// accumulate cost toward spend limit (use API key passed via header earlier)
						if apiKey := resp.Request.Header.Get("X-Proxy-API-Key"); apiKey != "" {
							mgr.AddCost(apiKey, pr.TotalCostUSD)
						}
					}
				}
			} else {
				fmt.Println("[proxy] Failed to parse JSON response:", err)
			}
		} else if strings.Contains(ct, "text/event-stream") && resp.Body != nil {
			// Wrap SSE body to log events as they stream; do not buffer entire stream
			resp.Body = &sseLoggingBody{rc: resp.Body}
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

		// Spend limit check BEFORE proxy
		if allowed, windowEnd, spent, lim := mgr.Allow(auth); !allowed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, `{"error":"spend limit exceeded","limit_per_hour":%.2f,"spent_this_window":%.4f,"window_ends_at":"%s"}`, lim, spent, windowEnd.UTC().Format(time.RFC3339))
			return
		}

		// Pass stripped key to response modifier for accumulation
		if auth != "" {
			r.Header.Set("X-Proxy-API-Key", auth)
		}

		proxy.ServeHTTP(w, r)
	})
}
