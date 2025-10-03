package handlers

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goverture/goxy/config"
)

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
