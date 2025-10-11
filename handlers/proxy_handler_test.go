package handlers

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/goverture/goxy/config"
	"github.com/goverture/goxy/pricing"
)

// setupTestPricingConfig ensures pricing configuration is available for tests
func setupTestPricingConfig() {
	testConfig := &pricing.PricingConfig{
		Models: map[string]pricing.ModelPricing{
			"gpt-4o": {
				Prompt:     0.005,
				Completion: 0.015,
				Aliases:    []string{"gpt-4o-2024-08-06"},
			},
		},
		Default: &pricing.ModelPricing{
			Prompt:     0.01,
			Completion: 0.02,
		},
		CachedTokenDiscount: 0.1,
	}
	pricing.SetConfig(testConfig)
}

func TestProxy_ForwardsMethodPathQueryBodyAndHeaders(t *testing.T) {
	// Upstream fake server to capture what we receive
	var capturedMethod, capturedPath, capturedContentType, capturedBody string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.RequestURI() // includes path + query
		capturedContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)

		w.Header().Set("X-Upstream", "ok")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"from-upstream"}`))
	}))
	defer upstream.Close()

	// Set up the global config for the test - point to our test upstream
	config.Cfg = &config.Config{
		OpenAIBaseURL: upstream.URL,
	}

	h := NewProxyHandler()

	// Build a request that would hit our proxy
	body := bytes.NewBufferString(`{"hello":"world"}`)
	req := httptest.NewRequest(http.MethodPost, "http://myproxy.local/v1/blah?x=1&y=2", body)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// Assert what the client receives (proxied response)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
	if got := rr.Body.String(); got != `{"status":"from-upstream"}` {
		t.Fatalf("unexpected body: %q", got)
	}
	if rr.Header().Get("X-Upstream") != "ok" {
		t.Fatalf("missing/incorrect upstream header")
	}

	// Assert what the upstream saw (request integrity)
	if capturedMethod != http.MethodPost {
		t.Fatalf("method not forwarded, got %s", capturedMethod)
	}
	if capturedPath != "/v1/blah?x=1&y=2" {
		t.Fatalf("path/query not forwarded, got %s", capturedPath)
	}
	if capturedContentType != "application/json" {
		t.Fatalf("content-type lost, got %s", capturedContentType)
	}
	if capturedBody != `{"hello":"world"}` {
		t.Fatalf("body not forwarded, got %q", capturedBody)
	}
}

func TestProxy_LogsParsedJSONResponse(t *testing.T) {
	// Upstream server returning JSON we can predict
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"from-upstream"}`)) // valid JSON body
	}))
	defer upstream.Close()

	// Configure proxy
	config.Cfg = &config.Config{OpenAIBaseURL: upstream.URL}
	h := NewProxyHandler()

	// Capture stdout
	oldStdout := os.Stdout
	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe err: %v", err)
	}
	os.Stdout = wPipe

	// Perform request
	req := httptest.NewRequest(http.MethodGet, "http://proxy.local/v1/test", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// Restore stdout
	wPipe.Close()
	os.Stdout = oldStdout
	logged, _ := io.ReadAll(rPipe)

	// Basic response assertion to ensure normal proxy function
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); body != `{"status":"from-upstream"}` {
		t.Fatalf("unexpected proxied body: %q", body)
	}

	outStr := string(logged)
	if !strings.Contains(outStr, "[proxy] Upstream JSON response:") {
		t.Fatalf("expected log marker not found. Got logs: %s", outStr)
	}
	if !strings.Contains(outStr, `"status": "from-upstream"`) {
		t.Fatalf("expected pretty JSON in logs, got: %s", outStr)
	}
}

func TestProxy_SpendLimitExceeded(t *testing.T) {
	// Setup pricing configuration for tests
	setupTestPricingConfig()

	// Upstream server that returns pricing-eligible JSON (model + usage)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Cost per request (prompt tokens 200 @ gpt-4o prompt $0.005/1K) => 0.2 * 0.005 = $0.001
		w.Write([]byte(`{"model":"gpt-4o","usage":{"prompt_tokens":200,"completion_tokens":0}}`))
	}))
	defer upstream.Close()

	// Spend limit just above first request cost so second pushes over limit; third should be blocked
	config.Cfg = &config.Config{OpenAIBaseURL: upstream.URL, SpendLimitPerHour: 0.0015}
	h := NewProxyHandler()

	doReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://proxy.local/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer test-key-1")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	// First request (spend 0.001) allowed
	if rr := doReq(); rr.Code != http.StatusOK {
		t.Fatalf("first request unexpected status %d body=%s", rr.Code, rr.Body.String())
	}
	// Second request (spend now 0.002) still allowed (limit check before adding cost)
	if rr := doReq(); rr.Code != http.StatusOK {
		t.Fatalf("second request unexpected status %d body=%s", rr.Code, rr.Body.String())
	}
	// Third request should be blocked (spent already > limit)
	if rr := doReq(); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on third request, got %d body=%s", rr.Code, rr.Body.String())
	} else if !strings.Contains(rr.Body.String(), "spend limit exceeded") {
		t.Fatalf("expected spend limit message in body: %s", rr.Body.String())
	}
}

func TestProxy_ZeroLimitBlocksImmediately(t *testing.T) {
	// Upstream that would normally return low-cost JSON
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"model":"gpt-4o","usage":{"prompt_tokens":10,"completion_tokens":0}}`))
	}))
	defer upstream.Close()

	// Zero limit => every non-anonymous key blocked right away
	config.Cfg = &config.Config{OpenAIBaseURL: upstream.URL, SpendLimitPerHour: 0}
	h := NewProxyHandler()

	req := httptest.NewRequest(http.MethodGet, "http://proxy.local/v1/test", nil)
	req.Header.Set("Authorization", "Bearer zero-key")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for zero limit, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "spend limit exceeded") {
		t.Fatalf("expected spend limit message in body: %s", rr.Body.String())
	}
}
