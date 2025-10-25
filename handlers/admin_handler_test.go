package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goverture/goxy/persistence"
	"github.com/goverture/goxy/pricing"
)

// createTestManager creates a persistent limit manager for testing
func createTestManager(t *testing.T, limitUSD float64) *persistence.PersistentLimitManager {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	mgr, err := persistence.NewPersistentLimitManager(limitUSD, dbPath)
	if err != nil {
		t.Fatalf("Failed to create test persistent manager: %v", err)
	}
	return mgr
}

func TestAdminHandler_HealthCheck(t *testing.T) {
	mgr := createTestManager(t, 2.0)
	defer mgr.Close()
	adminHandler := NewAdminHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	adminHandler.HealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", response["status"])
	}

	if response["service"] != "goxy-admin" {
		t.Errorf("expected service 'goxy-admin', got %v", response["service"])
	}
}

func TestAdminHandler_HealthCheck_WrongMethod(t *testing.T) {
	mgr := createTestManager(t, 2.0)
	defer mgr.Close()
	adminHandler := NewAdminHandler(mgr)

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rr := httptest.NewRecorder()

	adminHandler.HealthCheck(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rr.Code)
	}
}

func TestAdminHandler_GetUsage_EmptyUsage(t *testing.T) {
	mgr := createTestManager(t, 2.0)
	defer mgr.Close()
	adminHandler := NewAdminHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/usage", nil)
	rr := httptest.NewRecorder()

	adminHandler.handleUsage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response UsageResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if response.Total != 0 {
		t.Errorf("expected total 0, got %d", response.Total)
	}

	if len(response.Usage) != 0 {
		t.Errorf("expected empty usage array, got %d items", len(response.Usage))
	}
}

func TestAdminHandler_GetUsage_WithMaskedKeys(t *testing.T) {
	mgr := createTestManager(t, 2.0)
	defer mgr.Close()
	adminHandler := NewAdminHandler(mgr)

	// Simulate some usage by adding cost for different keys with proper masking
	testKeys := []struct {
		raw    string
		masked string
	}{
		{"Bearer sk-1234567890abcdef1234567890abcdef", "Bearer sk-1...cdef"},
		{"Bearer sk-9876543210fedcba9876543210fedcba", "Bearer sk-9...dcba"},
		{"anonymous", "anonymous"},
	}

	for _, key := range testKeys {
		mgr.AddCostWithMaskedKey(key.raw, key.masked, pricing.NewMoneyFromUSD(0.1))
	}

	req := httptest.NewRequest(http.MethodGet, "/usage", nil)
	rr := httptest.NewRecorder()

	adminHandler.handleUsage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response UsageResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if response.Total != len(testKeys) {
		t.Errorf("expected total %d, got %d", len(testKeys), response.Total)
	}

	// Check that keys are properly masked
	for _, usage := range response.Usage {
		// Check that raw key values are not exposed
		if strings.Contains(usage.Key, "1234567890abcdef") || strings.Contains(usage.Key, "9876543210fedcba") {
			t.Errorf("API key not properly masked: %s", usage.Key)
		}

		// Check that Bearer tokens are masked correctly (should contain "...")
		if strings.HasPrefix(usage.Key, "Bearer sk-") && usage.Key != "anonymous" {
			if !strings.Contains(usage.Key, "...") {
				t.Errorf("Bearer token not properly masked: %s", usage.Key)
			}
		}
	}
}

func TestAdminHandler_GetUsage_WrongMethod(t *testing.T) {
	mgr := createTestManager(t, 2.0)
	defer mgr.Close()
	adminHandler := NewAdminHandler(mgr)

	req := httptest.NewRequest(http.MethodPost, "/usage", nil)
	rr := httptest.NewRecorder()

	adminHandler.handleUsage(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if response["error"] != "method not allowed" {
		t.Errorf("expected error message 'method not allowed', got %v", response["error"])
	}
}

func TestAdminHandler_UpdateLimit_Success(t *testing.T) {
	mgr := createTestManager(t, 2.0)
	defer mgr.Close()
	adminHandler := NewAdminHandler(mgr)

	requestBody := LimitUpdateRequest{LimitUSD: 5.0}
	body, _ := json.Marshal(requestBody)

	req := httptest.NewRequest("PUT", "/limit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	adminHandler.handleLimit(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response LimitUpdateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if response.OldLimitUSD != 2.0 {
		t.Errorf("expected old limit 2.0, got %f", response.OldLimitUSD)
	}

	if response.NewLimitUSD != 5.0 {
		t.Errorf("expected new limit 5.0, got %f", response.NewLimitUSD)
	}

	if response.Message != "Spending limit updated successfully" {
		t.Errorf("unexpected message: %s", response.Message)
	}

	// Verify the limit was actually updated
	usage := mgr.GetUsage("test-key")
	if usage.Limit.ToUSD() != 5.0 {
		t.Errorf("limit not updated in manager, expected 5.0, got %f", usage.Limit.ToUSD())
	}
}

func TestAdminHandler_UpdateLimit_InvalidJSON(t *testing.T) {
	mgr := createTestManager(t, 2.0)
	defer mgr.Close()
	adminHandler := NewAdminHandler(mgr)

	req := httptest.NewRequest("PUT", "/limit", bytes.NewBufferString("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	adminHandler.handleLimit(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if !strings.Contains(response["error"], "invalid JSON") {
		t.Errorf("expected invalid JSON error, got %v", response["error"])
	}
}

func TestAdminHandler_UpdateLimit_WrongMethod(t *testing.T) {
	mgr := createTestManager(t, 2.0)
	defer mgr.Close()
	adminHandler := NewAdminHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/limit", nil)
	rr := httptest.NewRecorder()

	adminHandler.handleLimit(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if response["error"] != "method not allowed" {
		t.Errorf("expected error message 'method not allowed', got %v", response["error"])
	}
}

func TestAdminHandler_ServeHTTP_NotFoundEndpoint(t *testing.T) {
	mgr := createTestManager(t, 2.0)
	defer mgr.Close()
	adminHandler := NewAdminHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rr := httptest.NewRecorder()

	adminHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if response["error"] != "endpoint not found" {
		t.Errorf("expected error message 'endpoint not found', got %v", response["error"])
	}

	if !strings.Contains(response["available_endpoints"], "/usage") {
		t.Errorf("available endpoints should mention /usage")
	}
}

func TestAdminHandler_ServeHTTP_OptionsRequest(t *testing.T) {
	mgr := createTestManager(t, 2.0)
	defer mgr.Close()
	adminHandler := NewAdminHandler(mgr)

	req := httptest.NewRequest(http.MethodOptions, "/usage", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	adminHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rr.Code)
	}

	// Check CORS headers
	if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected CORS origin header, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
	}

	if !strings.Contains(rr.Header().Get("Access-Control-Allow-Methods"), "PUT") {
		t.Errorf("expected PUT in allowed methods, got %s", rr.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestAdminHandler_ServeHTTP_CORSHeaders(t *testing.T) {
	mgr := createTestManager(t, 2.0)
	defer mgr.Close()
	adminHandler := NewAdminHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	adminHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Check CORS headers are set
	if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected CORS origin header, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
	}

	if rr.Header().Get("Vary") != "Origin" {
		t.Errorf("expected Vary header, got %s", rr.Header().Get("Vary"))
	}
}
