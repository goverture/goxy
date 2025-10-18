package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/goverture/goxy/pricing"
)

// maskAPIKey masks an API key to show only first 4 and last 4 characters
// e.g., "Bearer sk-1234567890abcdef" becomes "sk-1234...cdef"
func maskAPIKey(key string) string {
	if key == "" || key == "anonymous" {
		return key
	}

	// Handle "Bearer " prefix
	if len(key) > 7 && key[:7] == "Bearer " {
		token := key[7:] // Remove "Bearer " prefix
		if len(token) <= 8 {
			// Too short to mask meaningfully
			return "Bearer " + token[:4] + "..."
		}
		return "Bearer " + token[:4] + "..." + token[len(token)-4:]
	}

	// Handle raw token
	if len(key) <= 8 {
		// Too short to mask meaningfully
		return key[:4] + "..."
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// AdminHandler provides endpoints for monitoring usage and updating limits
type AdminHandler struct {
	manager *pricing.ManagerMoney
}

// NewAdminHandler creates a new admin handler with the given limit manager
func NewAdminHandler(manager *pricing.ManagerMoney) *AdminHandler {
	return &AdminHandler{manager: manager}
}

// UsageResponse represents the response for usage queries
// Uses legacy UsageInfo for JSON compatibility
type UsageResponse struct {
	Usage []pricing.UsageInfo `json:"usage"`
	Total int                 `json:"total"`
}

// LimitUpdateRequest represents the request to update spending limits
type LimitUpdateRequest struct {
	LimitUSD float64 `json:"limit_usd"`
}

// LimitUpdateResponse represents the response after updating limits
type LimitUpdateResponse struct {
	Message     string  `json:"message"`
	OldLimitUSD float64 `json:"old_limit_usd"`
	NewLimitUSD float64 `json:"new_limit_usd"`
}

// ServeHTTP handles admin requests
func (ah *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Add CORS headers for admin endpoints
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,PUT,OPTIONS")
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.URL.Path {
	case "/usage":
		ah.handleUsage(w, r)
	case "/limit":
		ah.handleLimit(w, r)
	case "/health":
		ah.HealthCheck(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":               "endpoint not found",
			"available_endpoints": "/usage, /limit, /health",
		})
	}
}

// handleUsage handles GET requests for usage information
func (ah *AdminHandler) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	// Return usage for all keys (no individual key queries for security)
	usageMoney := ah.manager.GetAllUsage()

	// Convert to legacy format for JSON compatibility and mask keys for security
	usage := make([]pricing.UsageInfo, len(usageMoney))
	for i := range usageMoney {
		usageMoney[i].Key = maskAPIKey(usageMoney[i].Key)
		usage[i] = usageMoney[i].ToLegacy()
	}

	response := UsageResponse{
		Usage: usage,
		Total: len(usage),
	}
	json.NewEncoder(w).Encode(response)
}

// handleLimit handles PUT requests for updating spending limits
func (ah *AdminHandler) handleLimit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	var req LimitUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	// Get current limit for response
	allUsage := ah.manager.GetAllUsage()
	var oldLimit float64
	if len(allUsage) > 0 {
		oldLimit = allUsage[0].Limit.ToUSD()
	} else {
		// If no keys tracked yet, get limit from a dummy call
		dummy := ah.manager.GetUsage("dummy")
		oldLimit = dummy.Limit.ToUSD()
	}

	// Update the limit using the Money-based API
	ah.manager.UpdateLimitFromUSD(req.LimitUSD)

	response := LimitUpdateResponse{
		Message:     "Spending limit updated successfully",
		OldLimitUSD: oldLimit,
		NewLimitUSD: req.LimitUSD,
	}

	json.NewEncoder(w).Encode(response)
}

// HealthCheck provides a simple health check endpoint
func (ah *AdminHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "goxy-admin",
	})
}
