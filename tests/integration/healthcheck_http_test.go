package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthCheck_HTTP tests the basic health check endpoint via HTTP
func TestHealthCheck_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Make HTTP GET request to /healthz
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "Health check should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	// Verify response format
	status, ok := resp["status"].(string)
	require.True(t, ok, "Response should have 'status' field")
	assert.Equal(t, "healthy", status, "Status should be 'healthy'")
}

// TestHealthCheck_ResponseFormat_HTTP verifies the JSON structure of health check response
func TestHealthCheck_ResponseFormat_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"),
		"Content-Type should be application/json")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	// Verify required fields
	assert.Contains(t, resp, "status", "Response should contain 'status' field")
	assert.Equal(t, "healthy", resp["status"], "Status should be 'healthy'")
}

// TestHealthCheck_NoAuth_HTTP verifies that health check does not require authentication
func TestHealthCheck_NoAuth_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Make request WITHOUT x-mock-user-id header
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	// Note: No authentication header set
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Health check should succeed even without auth
	assert.Equal(t, http.StatusOK, w.Code,
		"Health check should not require authentication")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp["status"])
}
