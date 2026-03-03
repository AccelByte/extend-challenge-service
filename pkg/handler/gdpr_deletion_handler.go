package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
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
}

// NewGDPRDeletionHandler creates a new GDPR deletion handler.
func NewGDPRDeletionHandler(
	repo commonRepo.GoalRepository,
	namespace string,
	authEnabled bool,
	tokenValidator validator.AuthTokenValidator,
	logger *slog.Logger,
) *GDPRDeletionHandler {
	return &GDPRDeletionHandler{
		repo:           repo,
		namespace:      namespace,
		authEnabled:    authEnabled,
		tokenValidator: tokenValidator,
		logger:         logger,
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
