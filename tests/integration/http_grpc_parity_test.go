// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"extend-challenge-service/pkg/cache"
	"extend-challenge-service/pkg/handler"
	pb "extend-challenge-service/pkg/pb"

	commonCache "github.com/AccelByte/extend-challenge-common/pkg/cache"
	commonConfig "github.com/AccelByte/extend-challenge-common/pkg/config"
	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
	commonRepo "github.com/AccelByte/extend-challenge-common/pkg/repository"

	"google.golang.org/protobuf/encoding/protojson"
)

// TestHTTPGRPCParity_GetChallenges ensures that the optimized HTTP handler
// returns the same data as the gRPC handler for GET /v1/challenges.
//
// This test enforces feature parity between:
// - pkg/server/challenge_service_server.go (gRPC)
// - pkg/handler/optimized_challenges_handler.go (HTTP)
//
// See docs/ADR_001_OPTIMIZED_HTTP_HANDLER.md for architectural context.
func TestHTTPGRPCParity_GetChallenges_ActiveOnlyFalse(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "parity-test-user-1"

	// Call gRPC endpoint with auth context
	ctx := createAuthContext(userID, "test-namespace")
	grpcResp, err := client.GetUserChallenges(ctx, &pb.GetChallengesRequest{
		ActiveOnly: false,
	})
	require.NoError(t, err, "gRPC call should succeed")

	// Setup HTTP handler with same dependencies as gRPC server
	httpHandler := setupHTTPHandler(t)

	// Call HTTP endpoint using httptest (no real server needed)
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges?active_only=false", nil)
	req.Header.Set("x-mock-user-id", userID) // Mock auth - handler will extract this
	w := httptest.NewRecorder()

	httpHandler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "HTTP should return 200")

	// Parse HTTP response
	var httpData map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&httpData)
	require.NoError(t, err, "HTTP response should be valid JSON")

	// Convert gRPC response to JSON for comparison
	grpcJSON, err := protojson.Marshal(grpcResp)
	require.NoError(t, err, "gRPC response should marshal to JSON")

	var grpcData map[string]interface{}
	err = json.Unmarshal(grpcJSON, &grpcData)
	require.NoError(t, err, "gRPC JSON should unmarshal")

	// Verify same number of challenges
	httpChallenges, httpOk := httpData["challenges"].([]interface{})
	grpcChallenges, grpcOk := grpcData["challenges"].([]interface{})

	require.True(t, httpOk, "HTTP response should have challenges array")
	require.True(t, grpcOk, "gRPC response should have challenges array")

	assert.Equal(t, len(grpcChallenges), len(httpChallenges),
		"HTTP and gRPC should return same number of challenges")

	// Verify challenge IDs match (order may differ, so use sets)
	// Note: Both gRPC and HTTP now use camelCase (challengeId) - M3 JSON tag fix
	httpIDs := make(map[string]bool)
	for _, ch := range httpChallenges {
		chMap := ch.(map[string]interface{})
		if id, ok := chMap["challengeId"].(string); ok {
			httpIDs[id] = true
		}
	}

	grpcIDs := make(map[string]bool)
	for _, ch := range grpcChallenges {
		chMap := ch.(map[string]interface{})
		if id, ok := chMap["challengeId"].(string); ok {
			grpcIDs[id] = true
		}
	}

	assert.Equal(t, grpcIDs, httpIDs,
		"HTTP and gRPC should return same challenge IDs")

	t.Logf("✅ Feature parity verified: HTTP and gRPC return same data for active_only=false (user=%s)", userID)
}

func TestHTTPGRPCParity_GetChallenges_ActiveOnlyTrue(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "parity-test-user-2"

	// Call gRPC endpoint with active_only=true and auth context
	ctx := createAuthContext(userID, "test-namespace")
	grpcResp, err := client.GetUserChallenges(ctx, &pb.GetChallengesRequest{
		ActiveOnly: true,
	})
	require.NoError(t, err, "gRPC call should succeed")

	// Setup HTTP handler with same dependencies as gRPC server
	httpHandler := setupHTTPHandler(t)

	// Call HTTP endpoint using httptest
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges?active_only=true", nil)
	req.Header.Set("x-mock-user-id", userID) // Mock auth - handler will extract this
	w := httptest.NewRecorder()

	httpHandler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "HTTP should return 200")

	// Parse HTTP response
	var httpData map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&httpData)
	require.NoError(t, err, "HTTP response should be valid JSON")

	// Convert gRPC response to JSON for comparison
	grpcJSON, err := protojson.Marshal(grpcResp)
	require.NoError(t, err, "gRPC response should marshal to JSON")

	var grpcData map[string]interface{}
	err = json.Unmarshal(grpcJSON, &grpcData)
	require.NoError(t, err, "gRPC JSON should unmarshal")

	// Verify same number of challenges
	httpChallenges := httpData["challenges"].([]interface{})
	grpcChallenges := grpcData["challenges"].([]interface{})
	assert.Equal(t, len(grpcChallenges), len(httpChallenges),
		"HTTP and gRPC should return same number of challenges for active_only=true")

	t.Logf("✅ Feature parity verified: HTTP and gRPC return same data for active_only=true (user=%s)", userID)
}

// setupHTTPHandler creates the optimized HTTP handler with the same dependencies
// as the gRPC server for feature parity testing.
func setupHTTPHandler(t *testing.T) *handler.OptimizedChallengesHandler {
	// Load the same challenge config used by gRPC server
	configPath := "../../config/challenges.test.json"
	configLoader := commonConfig.NewConfigLoader(configPath, logger)
	challengeConfig, err := configLoader.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load challenge config: %v", err)
	}

	// Create dependencies (reusing same instances for consistency)
	goalCache := commonCache.NewInMemoryGoalCache(challengeConfig, configPath, logger)
	goalRepo := commonRepo.NewPostgresGoalRepository(testDB)

	// Create serialization cache (needed by HTTP handler but not gRPC)
	serCache := cache.NewSerializedChallengeCache()

	// Convert protobuf challenges to the format expected by WarmUp
	pbChallenges := make([]*pb.Challenge, len(challengeConfig.Challenges))
	for i, ch := range challengeConfig.Challenges {
		pbChallenges[i] = convertToPbChallenge(ch)
	}

	err = serCache.WarmUp(pbChallenges)
	if err != nil {
		t.Fatalf("Failed to warm up serialization cache: %v", err)
	}

	// Create HTTP handler with auth disabled (using x-mock-user-id header for testing)
	return handler.NewOptimizedChallengesHandler(
		goalCache,
		goalRepo,
		serCache,
		"test-namespace",
		false, // authEnabled = false (use mock header)
		nil,   // tokenValidator = nil (not needed when auth disabled)
	)
}

// convertToPbChallenge converts a domain challenge to protobuf format
func convertToPbChallenge(ch *commonDomain.Challenge) *pb.Challenge {
	pbGoals := make([]*pb.Goal, len(ch.Goals))
	for i, goal := range ch.Goals {
		pbGoals[i] = &pb.Goal{
			GoalId:      goal.ID,
			Name:        goal.Name,
			Description: goal.Description,
			Requirement: &pb.Requirement{
				StatCode:    goal.Requirement.StatCode,
				Operator:    goal.Requirement.Operator,
				TargetValue: int32(goal.Requirement.TargetValue),
			},
			Reward: &pb.Reward{
				Type:     goal.Reward.Type,
				RewardId: goal.Reward.RewardID,
				Quantity: int32(goal.Reward.Quantity),
			},
		}
	}

	return &pb.Challenge{
		ChallengeId: ch.ID,
		Name:        ch.Name,
		Description: ch.Description,
		Goals:       pbGoals,
	}
}
