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
	"strconv"
	"strings"
	"time"

	"github.com/goverture/goxy/config"
	"github.com/goverture/goxy/limit"
	"github.com/goverture/goxy/pricing"
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

func NewProxyHandler() (http.Handler, *limit.Manager) {
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

		// Low-latency: avoid gzip buffering
		r.Header.Del("Accept-Encoding")

		// Stable UA (some WAFs dislike empty UA)
		if r.Header.Get("User-Agent") == "" {
			r.Header.Set("User-Agent", "goprx/1.0")
		}

		// Inject Authorization from env only if client didn't send one
		// if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		// 	r.Header.Set("Authorization", "Bearer "+key)
		// }
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

	// Transport configuration
	baseTransport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	proxy.Transport = stripForwardingHeaders{base: baseTransport}

	// Add CORS on the way out (useful for browsers) + disable buffering on some proxies
	// Additionally, intercept JSON responses to log their contents before forwarding.
	proxy.ModifyResponse = func(resp *http.Response) error {
		// CORS headers
		if origin := resp.Request.Header.Get("Origin"); origin != "" {
			h := resp.Header
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Expose-Headers", "Content-Type, OpenAI-Processing-Ms")
		}

		ct := resp.Header.Get("Content-Type")
		// JSON full-body logging
		if strings.Contains(ct, "application/json") && resp.Body != nil {
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
						// accumulate cost toward spend limit (use Authorization header as-is, or blank for unauthenticated)
						auth := resp.Request.Header.Get("Authorization")
						mgr.AddCost(auth, pr.TotalCostUSD)
					}
				}
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
		// Extract the authorization header as-is
		auth := r.Header.Get("Authorization")

		// Just warn if no auth header, don't block the request
		if auth == "" {
			fmt.Println("Warning: No Authorization header provided")
		}

		// Spend limit check BEFORE proxy (always check, use blank string for unauthenticated)
		if allowed, windowEnd, spent, lim := mgr.Allow(auth); !allowed {
			// Compute seconds until reset (window end)
			secUntil := int(time.Until(windowEnd).Seconds())
			if secUntil < 0 {
				secUntil = 0
			}
			// Standard header for 429 retry guidance
			w.Header().Set("Retry-After", strconv.Itoa(secUntil))
			// Draft/RFC 9333 style RateLimit headers (informational)
			// RateLimit-Limit: total allowed per window (here monetary, USD)
			// RateLimit-Remaining: remaining allowance (0 when blocked)
			// RateLimit-Reset: seconds until window resets
			w.Header().Set("RateLimit-Limit", fmt.Sprintf("%.2f", lim))
			w.Header().Set("RateLimit-Remaining", "0")
			w.Header().Set("RateLimit-Reset", strconv.Itoa(secUntil))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, `{"error":"spend limit exceeded","limit_per_hour":%.2f,"spent_this_window":%.4f,"window_ends_at":"%s","retry_after_seconds":%d}`, lim, spent, windowEnd.UTC().Format(time.RFC3339), secUntil)
			return
		}

		proxy.ServeHTTP(w, r)
	}), mgr
}
