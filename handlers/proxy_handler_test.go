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
	"github.com/goverture/goxy/persistence"
	"github.com/goverture/goxy/pricing"
	"github.com/goverture/goxy/utils"
)

// setupTestPricingConfig ensures pricing configuration is available for tests
func setupTestPricingConfig() {
	testConfig := &pricing.PricingConfig{
		Models: map[string]pricing.ModelPricing{
			"gpt-4o": {
				Prompt:       5.0,
				CachedPrompt: 2.5,
				Completion:   15.0,
			},
		},
		Default: &pricing.ModelPricing{
			Prompt:       10.0,
			CachedPrompt: 1.0,
			Completion:   20.0,
		},
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

	mgr, err := persistence.NewPersistentLimitManager(2.0, ":memory:")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()
	h := NewProxyHandler(mgr)

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
	mgr, err := persistence.NewPersistentLimitManager(2.0, ":memory:")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()
	h := NewProxyHandler(mgr)

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
		// Cost per request (prompt tokens 200 @ gpt-4o prompt $5.0/1M) => 200 * 5.0/1M = $0.001
		w.Write([]byte(`{"model":"gpt-4o","usage":{"prompt_tokens":200,"completion_tokens":0}}`))
	}))
	defer upstream.Close()

	// Spend limit just above first request cost so second pushes over limit; third should be blocked
	config.Cfg = &config.Config{OpenAIBaseURL: upstream.URL, SpendLimitPerHour: 0.0015}
	mgr, err := persistence.NewPersistentLimitManager(0.0015, ":memory:")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()
	h := NewProxyHandler(mgr)

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
	mgr, err := persistence.NewPersistentLimitManager(0, ":memory:")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()
	h := NewProxyHandler(mgr)

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

func TestProxy_UnauthenticatedRequestsBypassLimits(t *testing.T) {
	// Setup pricing configuration for tests
	setupTestPricingConfig()

	// Upstream server that returns pricing-eligible JSON
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Large cost per request to test that unauthenticated requests bypass limits
		w.Write([]byte(`{"model":"gpt-4o","usage":{"prompt_tokens":1000,"completion_tokens":0}}`))
	}))
	defer upstream.Close()

	// Very low spend limit that would normally block requests
	config.Cfg = &config.Config{OpenAIBaseURL: upstream.URL, SpendLimitPerHour: 0.000001}
	mgr, err := persistence.NewPersistentLimitManager(0.000001, ":memory:")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()
	h := NewProxyHandler(mgr)

	doUnauthenticatedReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://proxy.local/v1/chat/completions", nil)
		// No Authorization header
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	// Both unauthenticated requests should be allowed (bypass spend limits)
	if rr := doUnauthenticatedReq(); rr.Code != http.StatusOK {
		t.Fatalf("first unauthenticated request unexpected status %d body=%s", rr.Code, rr.Body.String())
	}

	if rr := doUnauthenticatedReq(); rr.Code != http.StatusOK {
		t.Fatalf("second unauthenticated request unexpected status %d body=%s", rr.Code, rr.Body.String())
	}

	// But authenticated requests with the same key should be limited
	doAuthenticatedReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://proxy.local/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer test-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	// First authenticated request should be allowed
	if rr := doAuthenticatedReq(); rr.Code != http.StatusOK {
		t.Fatalf("first authenticated request unexpected status %d body=%s", rr.Code, rr.Body.String())
	}

	// Second authenticated request should be blocked due to accumulated cost
	if rr := doAuthenticatedReq(); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on second authenticated request, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestProxy_ConcurrentRequestsCostAccumulation(t *testing.T) {
	// Setup pricing configuration for tests
	setupTestPricingConfig()

	// Upstream server that returns pricing-eligible JSON with known cost
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Each request costs: 100 tokens * $5.0/1M = $0.0005
		w.Write([]byte(`{"model":"gpt-4o","usage":{"prompt_tokens":100,"completion_tokens":0}}`))
	}))
	defer upstream.Close()

	// Use a temporary file for database to test persistence
	tmpFile := "/tmp/test_concurrency.db"
	defer os.Remove(tmpFile)

	// Set high enough limit to allow all concurrent requests
	config.Cfg = &config.Config{OpenAIBaseURL: upstream.URL, SpendLimitPerHour: 1.0}
	mgr, err := persistence.NewPersistentLimitManager(1.0, tmpFile)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	h := NewProxyHandler(mgr)

	// Number of concurrent requests to make
	numRequests := 10
	expectedCostPerRequest := pricing.NewMoneyFromUSD(0.0005) // 100 tokens * $5.0/1M
	expectedTotalCost := expectedCostPerRequest.Multiply(int64(numRequests))

	// Use a single auth key for all requests so they accumulate
	authKey := "Bearer test-concurrent-key"
	hashedAuthKey := utils.HashAuthKey(authKey)

	// Channel to collect results
	results := make(chan *httptest.ResponseRecorder, numRequests)

	// Start all requests concurrently
	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodPost, "http://proxy.local/v1/chat/completions", nil)
			req.Header.Set("Authorization", authKey)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			results <- rr
		}()
	}

	// Collect all results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		rr := <-results
		if rr.Code == http.StatusOK {
			successCount++
		} else {
			t.Logf("Request %d failed with status %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	// All requests should have succeeded since we set a high limit
	if successCount != numRequests {
		t.Fatalf("expected %d successful requests, got %d", numRequests, successCount)
	}

	// Check that the total accumulated cost is correct
	usage := mgr.GetUsage(hashedAuthKey)
	actualTotalCost := usage.Spent

	// The cost should be exactly expected (Money is precise integer arithmetic)
	if actualTotalCost != expectedTotalCost {
		t.Fatalf("expected total cost $%.8f, got $%.8f (difference: $%.8f)",
			expectedTotalCost.ToUSD(),
			actualTotalCost.ToUSD(),
			pricing.Money(expectedTotalCost-actualTotalCost).ToUSD())
	}

	t.Logf("Concurrency test passed: %d requests, expected cost $%.8f, actual cost $%.8f",
		numRequests, expectedTotalCost.ToUSD(), actualTotalCost.ToUSD())

	// Test database persistence - close and reload
	mgr.Close()

	mgr2, err := persistence.NewPersistentLimitManager(1.0, tmpFile)
	if err != nil {
		t.Fatalf("failed to create second manager: %v", err)
	}
	defer mgr2.Close()

	// Verify costs were loaded from database
	usage2 := mgr2.GetUsage(hashedAuthKey)
	if usage2.Spent != expectedTotalCost {
		t.Fatalf("database persistence failed: expected $%.8f, got $%.8f",
			expectedTotalCost.ToUSD(), usage2.Spent.ToUSD())
	}

	t.Logf("Database persistence verified: loaded $%.8f from database", usage2.Spent.ToUSD())
}

func TestProxy_HighConcurrencyStressTest(t *testing.T) {
	// Setup pricing configuration for tests
	setupTestPricingConfig()

	// Upstream server that returns pricing-eligible JSON with known cost
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Each request costs: 50 tokens * $5.0/1M = $0.00025
		w.Write([]byte(`{"model":"gpt-4o","usage":{"prompt_tokens":50,"completion_tokens":0}}`))
	}))
	defer upstream.Close()

	// Set high enough limit to allow all concurrent requests
	config.Cfg = &config.Config{OpenAIBaseURL: upstream.URL, SpendLimitPerHour: 1.0}
	mgr, err := persistence.NewPersistentLimitManager(1.0, ":memory:")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()
	h := NewProxyHandler(mgr)

	// High concurrency test - 100 requests
	numRequests := 100
	expectedCostPerRequest := pricing.NewMoneyFromUSD(0.00025) // 50 tokens * $5.0/1M
	expectedTotalCost := expectedCostPerRequest.Multiply(int64(numRequests))

	// Use a single auth key for all requests so they accumulate
	authKey := "Bearer test-stress-key"
	hashedAuthKey := utils.HashAuthKey(authKey)

	// Channel to collect results
	results := make(chan *httptest.ResponseRecorder, numRequests)

	// Start all requests concurrently
	for i := 0; i < numRequests; i++ {
		go func(reqNum int) {
			req := httptest.NewRequest(http.MethodPost, "http://proxy.local/v1/chat/completions", nil)
			req.Header.Set("Authorization", authKey)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			results <- rr
		}(i)
	}

	// Collect all results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		rr := <-results
		if rr.Code == http.StatusOK {
			successCount++
		} else {
			t.Logf("Request %d failed with status %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	// All requests should have succeeded since we set a high limit
	if successCount != numRequests {
		t.Fatalf("expected %d successful requests, got %d", numRequests, successCount)
	}

	// Check that the total accumulated cost is correct
	usage := mgr.GetUsage(hashedAuthKey)
	actualTotalCost := usage.Spent

	// The cost should be exactly expected (Money is precise integer arithmetic)
	if actualTotalCost != expectedTotalCost {
		t.Fatalf("expected total cost $%.8f, got $%.8f (difference: $%.8f)",
			expectedTotalCost.ToUSD(),
			actualTotalCost.ToUSD(),
			pricing.Money(expectedTotalCost-actualTotalCost).ToUSD())
	}

	t.Logf("High concurrency stress test passed: %d requests, expected cost $%.8f, actual cost $%.8f",
		numRequests, expectedTotalCost.ToUSD(), actualTotalCost.ToUSD())
}
