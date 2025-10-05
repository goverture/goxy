package handlers

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

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

func TestProxy_LogsSSEStream(t *testing.T) {
	// Simulated SSE upstream (two JSON events + [DONE])
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		events := []string{
			`data: {"step":1,"msg":"hello"}`,
			`data: {"step":2,"msg":"world"}`,
			`data: [DONE]`,
		}
		for _, e := range events {
			_, _ = w.Write([]byte(e + "\n"))
			if flusher != nil { flusher.Flush() }
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	config.Cfg = &config.Config{OpenAIBaseURL: upstream.URL}
	h := NewProxyHandler()

	// Capture stdout for logging assertions
	oldStdout := os.Stdout
	rPipe, wPipe, err := os.Pipe()
	if err != nil { t.Fatalf("pipe err: %v", err) }
	os.Stdout = wPipe

	req := httptest.NewRequest(http.MethodGet, "http://proxy.local/v1/stream", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// Finish capture
	wPipe.Close()
	os.Stdout = oldStdout
	logged, _ := io.ReadAll(rPipe)
	outStr := string(logged)

	// Response should be the concatenated SSE data as written (we won't assert exact because newline timing) but status 200
	if rr.Code != http.StatusOK { t.Fatalf("expected 200, got %d", rr.Code) }
	// Ensure logging of each JSON event
	if !strings.Contains(outStr, `"step": 1`) {
		t.Fatalf("missing first event JSON log: %s", outStr)
	}
	if !strings.Contains(outStr, `"step": 2`) {
		t.Fatalf("missing second event JSON log: %s", outStr)
	}
	if !strings.Contains(outStr, "SSE event: [DONE]") {
		t.Fatalf("missing done marker log: %s", outStr)
	}
}
