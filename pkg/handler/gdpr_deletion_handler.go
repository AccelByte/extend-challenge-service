package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/utils/auth/validator"
	commonRepo "github.com/AccelByte/extend-challenge-common/pkg/repository"
)

// GDPRDeletionResponse is the JSON response for GDPR data deletion.
type GDPRDeletionResponse struct {
	UserID      string `json:"userId"`
	RowsDeleted int64  `json:"rowsDeleted"`
}

// gdprErrorResponse matches the standard API error format (errorCode + message).
type gdprErrorResponse struct {
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

// writeJSONError writes a JSON error response with the standard errorCode/message format.
func writeJSONError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp, _ := json.Marshal(gdprErrorResponse{ErrorCode: errorCode, Message: message})
	_, _ = w.Write(resp)
}

// GDPRDeletionHandler handles DELETE /v1/users/me/data for GDPR compliance.
// It deletes all goal progress data for the authenticated user.
type GDPRDeletionHandler struct {
	repo           commonRepo.GoalRepository
	namespace      string
	authEnabled    bool
	tokenValidator validator.AuthTokenValidator
	logger         *slog.Logger
	rateLimiter    sync.Map // map[string]time.Time — last request time per user
}

// NewGDPRDeletionHandler creates a new GDPR deletion handler.
// The ctx parameter controls the lifecycle of the background eviction goroutine
// that prevents memory leaks in the per-user rate limiter.
func NewGDPRDeletionHandler(
	ctx context.Context,
	repo commonRepo.GoalRepository,
	namespace string,
	authEnabled bool,
	tokenValidator validator.AuthTokenValidator,
	logger *slog.Logger,
) *GDPRDeletionHandler {
	h := &GDPRDeletionHandler{
		repo:           repo,
		namespace:      namespace,
		authEnabled:    authEnabled,
		tokenValidator: tokenValidator,
		logger:         logger,
	}
	go h.evictStaleEntries(ctx)
	return h
}

// evictStaleEntries periodically removes rate limiter entries older than 2 minutes.
// This prevents unbounded memory growth from the sync.Map rate limiter.
func (h *GDPRDeletionHandler) evictStaleEntries(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			h.rateLimiter.Range(func(key, value any) bool {
				if lastTime, ok := value.(time.Time); ok && now.Sub(lastTime) > 2*time.Minute {
					h.rateLimiter.Delete(key)
				}
				return true
			})
		}
	}
}

// ServeHTTP handles the GDPR deletion request.
func (h *GDPRDeletionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE method is allowed")
		return
	}

	userID, err := h.extractUserID(r)
	if err != nil {
		h.logger.Warn("GDPR deletion auth failure", "error", err)
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid authentication token")
		return
	}

	// Per-user rate limit: 1 request per minute
	now := time.Now()
	if lastVal, ok := h.rateLimiter.Load(userID); ok {
		if lastTime, valid := lastVal.(time.Time); valid && now.Sub(lastTime) < time.Minute {
			writeJSONError(w, http.StatusTooManyRequests, "RATE_LIMITED", "GDPR deletion rate limit exceeded, try again later")
			return
		}
	}
	deleted, err := h.repo.DeleteUserData(r.Context(), h.namespace, userID)
	if err != nil {
		h.logger.Error("GDPR deletion failed",
			"userId", userID,
			"error", err,
			"audit", true,
			"auditAction", "gdpr_user_data_deletion_failed",
			"namespace", h.namespace,
			"requestedAt", time.Now().UTC().Format(time.RFC3339),
		)
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete user data")
		return
	}

	// Only consume rate limit after successful deletion — if deletion failed,
	// the user should be able to retry immediately without waiting 1 minute.
	h.rateLimiter.Store(userID, now)

	h.logger.Info("GDPR deletion completed",
		"userId", userID,
		"rowsDeleted", deleted,
		"audit", true,
		"auditAction", "gdpr_user_data_deletion",
		"namespace", h.namespace,
		"requestedAt", time.Now().UTC().Format(time.RFC3339),
	)

	resp := GDPRDeletionResponse{
		UserID:      userID,
		RowsDeleted: deleted,
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		h.logger.Error("GDPR deletion response marshal failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to process response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respJSON)
}

// extractUserID extracts the user ID from the request using the same pattern as OptimizedChallengesHandler.
func (h *GDPRDeletionHandler) extractUserID(r *http.Request) (string, error) {
	if !h.authEnabled {
		userID := r.Header.Get("x-mock-user-id")
		if userID == "" {
			userID = "test-user-id"
		}
		return userID, nil
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", &authError{message: "missing authorization header"}
	}

	token := trimBearerPrefix(authHeader)
	if token == authHeader {
		return "", &authError{message: "invalid authorization header format"}
	}

	if h.tokenValidator == nil {
		return "", &authError{message: "token validator not initialized"}
	}

	err := h.tokenValidator.Validate(token, nil, &h.namespace, nil)
	if err != nil {
		return "", &authError{message: "invalid token", cause: err}
	}

	claims, err := decodeJWTClaims(token)
	if err != nil {
		return "", &authError{message: "failed to decode JWT claims", cause: err}
	}

	if claims.Sub == "" {
		return "", &authError{message: "user ID not found in token claims"}
	}

	return claims.Sub, nil
}

// trimBearerPrefix removes "Bearer " prefix from auth header.
func trimBearerPrefix(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && header[:len(prefix)] == prefix {
		return header[len(prefix):]
	}
	return header
}
