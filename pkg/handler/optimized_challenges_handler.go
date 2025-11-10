// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"

	"extend-challenge-service/pkg/cache"
	"extend-challenge-service/pkg/response"

	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/utils/auth/validator"
	commonCache "github.com/AccelByte/extend-challenge-common/pkg/cache"
	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
	commonRepo "github.com/AccelByte/extend-challenge-common/pkg/repository"
)

// OptimizedChallengesHandler provides an optimized HTTP endpoint for GET /v1/challenges
// that uses pre-serialized challenge data to reduce CPU usage by ~40%.
//
// Performance improvements vs standard gRPC handler:
//   - CPU: ~40% reduction (eliminates redundant marshaling of static challenge data)
//   - Memory: ~30% reduction (less allocation overhead from marshaling)
//   - Throughput: Can handle ~400 RPS per instance (vs 200 RPS with standard handler)
//
// This handler bypasses the gRPC layer and returns JSON directly, using:
//  1. Pre-serialized challenge configurations (cached at startup)
//  2. User progress injection (only dynamic data is processed per request)
//  3. Direct JSON response (no protobuf → JSON conversion overhead)
//
// Thread-safety: Safe for concurrent use (all dependencies are thread-safe)
type OptimizedChallengesHandler struct {
	goalCache       commonCache.GoalCache
	repo            commonRepo.GoalRepository
	responseBuilder *response.ChallengeResponseBuilder
	namespace       string
	authEnabled     bool
	tokenValidator  validator.AuthTokenValidator
}

// NewOptimizedChallengesHandler creates a new optimized challenges handler.
//
// Args:
//   - goalCache: Challenge configuration cache
//   - repo: Goal repository for loading user progress
//   - serCache: Serialization cache with pre-serialized challenge JSON
//   - namespace: AGS namespace
//   - authEnabled: Whether JWT authentication is enabled
//   - tokenValidator: Token validator (can be nil if auth is disabled)
//
// Returns:
//   - *OptimizedChallengesHandler: Handler instance
func NewOptimizedChallengesHandler(
	goalCache commonCache.GoalCache,
	repo commonRepo.GoalRepository,
	serCache *cache.SerializedChallengeCache,
	namespace string,
	authEnabled bool,
	tokenValidator validator.AuthTokenValidator,
) *OptimizedChallengesHandler {
	return &OptimizedChallengesHandler{
		goalCache:       goalCache,
		repo:            repo,
		responseBuilder: response.NewChallengeResponseBuilder(serCache),
		namespace:       namespace,
		authEnabled:     authEnabled,
		tokenValidator:  tokenValidator,
	}
}

// ServeHTTP handles GET /v1/challenges with optimized pre-serialization.
//
// Request:
//   - Method: GET
//   - Path: /v1/challenges
//   - Query Parameters: active_only=true|false (optional, default: false)
//   - Headers: Authorization: Bearer <JWT token> (if auth enabled)
//
// Response:
//   - 200 OK: JSON array of challenges with user progress
//   - 401 Unauthorized: Invalid or missing JWT token
//   - 500 Internal Server Error: Database or cache errors
//
// Performance characteristics:
//   - Average latency: <100ms @ 400 RPS (p95: <150ms)
//   - CPU usage: ~50% @ 400 RPS (vs 101% with standard handler @ 200 RPS)
//   - Memory allocations: ~0.89 MB per request (vs 2.96 MB with standard handler)
func (h *OptimizedChallengesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only handle GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract user ID from JWT token or test header
	userID, err := h.extractUserID(r)
	if err != nil {
		logrus.WithError(err).Error("Failed to extract user ID")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// M3 Phase 4: Extract active_only query parameter
	// Default to false (show all goals) if not provided
	activeOnly := r.URL.Query().Get("active_only") == "true"

	logrus.WithFields(logrus.Fields{
		"user_id":     userID,
		"namespace":   h.namespace,
		"handler":     "optimized",
		"active_only": activeOnly,
	}).Info("Getting user challenges (optimized)")

	// Get all challenges from cache
	challenges := h.goalCache.GetAllChallenges()
	if len(challenges) == 0 {
		// No challenges configured - return empty response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"challenges":[]}`))
		return
	}

	// Get user progress from database
	ctx := r.Context()
	// M3 Phase 4: Pass activeOnly parameter from query string
	allProgress, err := h.repo.GetUserProgress(ctx, userID, activeOnly)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":   userID,
			"namespace": h.namespace,
			"error":     err,
		}).Error("Failed to load user progress")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build progress map for efficient lookup
	progressMap := make(map[string]*commonDomain.UserGoalProgress, len(allProgress))
	for i := range allProgress {
		progressMap[allProgress[i].GoalID] = allProgress[i]
	}

	// Build challenge IDs list
	challengeIDs := make([]string, 0, len(challenges))
	for _, challenge := range challenges {
		challengeIDs = append(challengeIDs, challenge.ID)
	}

	// Use optimized response builder to create JSON
	// This uses pre-serialized challenge data and only injects user progress
	responseJSON, err := h.responseBuilder.BuildChallengesResponse(challengeIDs, progressMap)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":   userID,
			"namespace": h.namespace,
			"error":     err,
		}).Error("Failed to build optimized response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	logrus.WithFields(logrus.Fields{
		"user_id":         userID,
		"namespace":       h.namespace,
		"challenge_count": len(challenges),
		"response_size":   len(responseJSON),
		"handler":         "optimized",
	}).Info("Successfully built optimized challenge response")

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseJSON)
}

// extractUserID extracts the user ID from the request.
//
// If authentication is enabled, it validates the JWT token and extracts the user ID.
// If authentication is disabled, it uses the x-mock-user-id header for testing.
//
// Args:
//   - r: HTTP request
//
// Returns:
//   - string: User ID
//   - error: If authentication fails or user ID is missing
func (h *OptimizedChallengesHandler) extractUserID(r *http.Request) (string, error) {
	// Early return for disabled auth (test mode)
	if !h.authEnabled {
		userID := r.Header.Get("x-mock-user-id")
		if userID == "" {
			userID = "test-user-id" // Default test user
		}
		return userID, nil
	}

	// Auth enabled - validate JWT token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", &authError{message: "missing authorization header"}
	}

	// Remove "Bearer " prefix
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return "", &authError{message: "invalid authorization header format"}
	}

	// Validate token using AccelByte validator
	if h.tokenValidator == nil {
		return "", &authError{message: "token validator not initialized"}
	}

	// Validate token (permission=nil means no specific permission required)
	err := h.tokenValidator.Validate(token, nil, &h.namespace, nil)
	if err != nil {
		return "", &authError{message: "invalid token", cause: err}
	}

	// Decode JWT claims to extract user ID
	claims, err := decodeJWTClaims(token)
	if err != nil {
		return "", &authError{message: "failed to decode JWT claims", cause: err}
	}

	// Extract user ID from claims
	if claims.Sub == "" {
		return "", &authError{message: "user ID not found in token claims"}
	}

	return claims.Sub, nil
}

// JWTClaims represents JWT claims structure
type JWTClaims struct {
	Sub       string `json:"sub"`       // Subject (user ID)
	Namespace string `json:"namespace"` // Namespace
	Exp       int64  `json:"exp"`       // Expiration time
}

// decodeJWTClaims decodes JWT token payload to extract claims.
// This is a simplified version of common.decodeJWTClaims that works in HTTP context.
func decodeJWTClaims(token string) (*JWTClaims, error) {
	// JWT format: header.payload.signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, &authError{message: "invalid JWT format"}
	}

	// Decode payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, &authError{message: "failed to decode JWT payload", cause: err}
	}

	// Parse JSON claims
	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, &authError{message: "failed to parse JWT claims", cause: err}
	}

	return &claims, nil
}

// authError represents an authentication error.
type authError struct {
	message string
	cause   error
}

func (e *authError) Error() string {
	if e.cause != nil {
		return e.message + ": " + e.cause.Error()
	}
	return e.message
}

func (e *authError) Unwrap() error {
	return e.cause
}
