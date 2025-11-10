// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"database/sql"
	"time"

	"extend-challenge-service/pkg/common"
	"extend-challenge-service/pkg/mapper"
	pb "extend-challenge-service/pkg/pb"
	"extend-challenge-service/pkg/service"

	"github.com/AccelByte/extend-challenge-common/pkg/cache"
	"github.com/AccelByte/extend-challenge-common/pkg/client"
	"github.com/AccelByte/extend-challenge-common/pkg/repository"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ChallengeServiceServer implements the gRPC ServiceServer interface
type ChallengeServiceServer struct {
	pb.UnimplementedServiceServer

	goalCache    cache.GoalCache
	repo         repository.GoalRepository
	rewardClient client.RewardClient
	db           *sql.DB
	namespace    string
}

// NewChallengeServiceServer creates a new challenge service server
func NewChallengeServiceServer(
	goalCache cache.GoalCache,
	repo repository.GoalRepository,
	rewardClient client.RewardClient,
	db *sql.DB,
	namespace string,
) *ChallengeServiceServer {
	return &ChallengeServiceServer{
		goalCache:    goalCache,
		repo:         repo,
		rewardClient: rewardClient,
		db:           db,
		namespace:    namespace,
	}
}

// extractUserIDFromContext extracts the authenticated user ID from the request context.
// The user ID is populated by the auth interceptor after JWT validation.
// This is a simple wrapper around common.GetUserIDFromContext() for backward compatibility.
func extractUserIDFromContext(ctx context.Context) (string, error) {
	return common.GetUserIDFromContext(ctx)
}

// GetUserChallenges retrieves all challenges for the authenticated user with current progress
//
// ⚠️ IMPORTANT: GET /v1/challenges uses OptimizedChallengesHandler for performance (2x throughput)
// When modifying this handler, also update pkg/handler/optimized_challenges_handler.go
// See docs/ADR_001_OPTIMIZED_HTTP_HANDLER.md for details on feature parity requirements
func (s *ChallengeServiceServer) GetUserChallenges(
	ctx context.Context,
	req *pb.GetChallengesRequest,
) (*pb.GetChallengesResponse, error) {
	// Extract user ID from JWT token (already validated by interceptor)
	userID, err := extractUserIDFromContext(ctx)
	if err != nil {
		logrus.WithError(err).Error("Failed to extract user ID from context")
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"user_id":     userID,
		"namespace":   s.namespace,
		"active_only": req.ActiveOnly, // M3 Phase 4
	}).Info("Getting user challenges")

	// Get challenges with progress using service layer
	// M3 Phase 4: Pass activeOnly parameter from request
	challengesWithProgress, err := service.GetUserChallengesWithProgress(
		ctx,
		userID,
		s.namespace,
		s.goalCache,
		s.repo,
		req.ActiveOnly,
	)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":   userID,
			"namespace": s.namespace,
			"error":     err,
		}).Error("Failed to get user challenges")
		return nil, status.Error(codes.Internal, "failed to retrieve challenges")
	}

	// Convert to protobuf response
	protoChallenges := make([]*pb.Challenge, 0, len(challengesWithProgress))
	for _, cwp := range challengesWithProgress {
		protoChallenge, err := mapper.ChallengeToProto(cwp.Challenge, cwp.UserProgress)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"user_id":      userID,
				"challenge_id": cwp.Challenge.ID,
				"error":        err,
			}).Error("Failed to convert challenge to proto")
			return nil, status.Error(codes.Internal, "failed to convert challenge data")
		}
		protoChallenges = append(protoChallenges, protoChallenge)
	}

	// NOTE: Cannot return objects to pool here - gRPC serializes AFTER handler returns
	// This would cause a race condition. Pooling only helps if done at a different layer.

	logrus.WithFields(logrus.Fields{
		"user_id":         userID,
		"namespace":       s.namespace,
		"challenge_count": len(protoChallenges),
	}).Info("Successfully retrieved user challenges")

	return &pb.GetChallengesResponse{
		Challenges: protoChallenges,
	}, nil
}

// InitializePlayer assigns default goals to a new player or syncs existing player with config changes
func (s *ChallengeServiceServer) InitializePlayer(
	ctx context.Context,
	req *pb.InitializeRequest,
) (*pb.InitializeResponse, error) {
	// Extract user ID from JWT token (already validated by interceptor)
	userID, err := extractUserIDFromContext(ctx)
	if err != nil {
		logrus.WithError(err).Error("Failed to extract user ID from context")
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"user_id":   userID,
		"namespace": s.namespace,
	}).Info("Initializing player with default goals")

	// Call business logic
	result, err := service.InitializePlayer(
		ctx,
		userID,
		s.namespace,
		s.goalCache,
		s.repo,
	)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":   userID,
			"namespace": s.namespace,
			"error":     err,
		}).Error("Failed to initialize player")
		return nil, status.Error(codes.Internal, "failed to initialize player")
	}

	// Convert to protobuf response
	protoAssignedGoals := make([]*pb.AssignedGoal, 0, len(result.AssignedGoals))
	for _, assignedGoal := range result.AssignedGoals {
		protoGoal, err := assignedGoalToProto(assignedGoal)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"user_id": userID,
				"goal_id": assignedGoal.GoalID,
				"error":   err,
			}).Error("Failed to convert assigned goal to proto")
			continue
		}
		protoAssignedGoals = append(protoAssignedGoals, protoGoal)
	}

	logrus.WithFields(logrus.Fields{
		"user_id":         userID,
		"namespace":       s.namespace,
		"new_assignments": result.NewAssignments,
		"total_active":    result.TotalActive,
	}).Info("Successfully initialized player")

	return &pb.InitializeResponse{
		AssignedGoals: protoAssignedGoals,
		// #nosec G115 - NewAssignments will never exceed int32 max (limited by goal count)
		NewAssignments: int32(result.NewAssignments),
		// #nosec G115 - TotalActive will never exceed int32 max (limited by goal count)
		TotalActive: int32(result.TotalActive),
	}, nil
}

// SetGoalActive allows players to manually control goal assignment (M3 Phase 3)
func (s *ChallengeServiceServer) SetGoalActive(
	ctx context.Context,
	req *pb.SetGoalActiveRequest,
) (*pb.SetGoalActiveResponse, error) {
	// Extract user ID from JWT token (already validated by interceptor)
	userID, err := extractUserIDFromContext(ctx)
	if err != nil {
		logrus.WithError(err).Error("Failed to extract user ID from context")
		return nil, err
	}

	// Validate request
	if req.ChallengeId == "" {
		return nil, status.Error(codes.InvalidArgument, "challenge_id is required")
	}

	if req.GoalId == "" {
		return nil, status.Error(codes.InvalidArgument, "goal_id is required")
	}

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"challenge_id": req.ChallengeId,
		"goal_id":      req.GoalId,
		"is_active":    req.IsActive,
		"namespace":    s.namespace,
	}).Info("Setting goal active status")

	// Call business logic
	result, err := service.SetGoalActive(
		ctx,
		userID,
		req.ChallengeId,
		req.GoalId,
		s.namespace,
		req.IsActive,
		s.goalCache,
		s.repo,
	)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": req.ChallengeId,
			"goal_id":      req.GoalId,
			"is_active":    req.IsActive,
			"namespace":    s.namespace,
			"error":        err,
		}).Error("Failed to set goal active status")
		return nil, status.Error(codes.Internal, "failed to set goal active status")
	}

	// Convert response
	response := &pb.SetGoalActiveResponse{
		ChallengeId: result.ChallengeID,
		GoalId:      result.GoalID,
		IsActive:    result.IsActive,
		Message:     result.Message,
	}

	// Convert assigned_at timestamp (nullable)
	if result.AssignedAt != nil {
		response.AssignedAt = result.AssignedAt.UTC().Format(time.RFC3339)
	}

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"challenge_id": req.ChallengeId,
		"goal_id":      req.GoalId,
		"is_active":    result.IsActive,
		"namespace":    s.namespace,
	}).Info("Successfully set goal active status")

	return response, nil
}

// ClaimGoalReward claims reward for a completed goal
func (s *ChallengeServiceServer) ClaimGoalReward(
	ctx context.Context,
	req *pb.ClaimRewardRequest,
) (*pb.ClaimRewardResponse, error) {
	// Extract user ID from JWT token (already validated by interceptor)
	userID, err := extractUserIDFromContext(ctx)
	if err != nil {
		logrus.WithError(err).Error("Failed to extract user ID from context")
		return nil, err
	}

	// Validate request
	if req.ChallengeId == "" {
		return nil, status.Error(codes.InvalidArgument, "challenge_id is required")
	}

	if req.GoalId == "" {
		return nil, status.Error(codes.InvalidArgument, "goal_id is required")
	}

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"goal_id":      req.GoalId,
		"challenge_id": req.ChallengeId,
		"namespace":    s.namespace,
	}).Info("Claiming goal reward")

	// Call claim service
	result, err := service.ClaimGoalReward(
		ctx,
		userID,
		req.GoalId,
		req.ChallengeId,
		s.namespace,
		s.goalCache,
		s.repo,
		s.rewardClient,
	)
	if err != nil {
		// Map domain errors to gRPC status codes
		return nil, mapper.MapErrorToGRPCStatus(err)
	}

	// Convert reward to proto
	protoReward, err := mapper.RewardToProto(&result.Reward)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"goal_id":      req.GoalId,
			"challenge_id": req.ChallengeId,
			"error":        err,
		}).Error("Failed to convert reward to proto")
		return nil, status.Error(codes.Internal, "failed to convert reward data")
	}

	// NOTE: Cannot return objects to pool here - gRPC serializes AFTER handler returns
	// This would cause a race condition. Pooling only helps if done at a different layer.

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"goal_id":      req.GoalId,
		"challenge_id": req.ChallengeId,
		"reward_type":  result.Reward.Type,
		"reward_id":    result.Reward.RewardID,
	}).Info("Successfully claimed goal reward")

	return &pb.ClaimRewardResponse{
		GoalId:    result.GoalID,
		Status:    result.Status,
		Reward:    protoReward,
		ClaimedAt: result.ClaimedAt.Format(time.RFC3339),
	}, nil
}

// HealthCheck verifies service and database health
func (s *ChallengeServiceServer) HealthCheck(
	ctx context.Context,
	req *pb.HealthCheckRequest,
) (*pb.HealthCheckResponse, error) {
	// Check database connectivity with 2-second timeout
	healthCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := s.db.PingContext(healthCtx); err != nil {
		logrus.WithError(err).Error("Database health check failed")
		return nil, status.Error(codes.Unavailable, "database connectivity check failed")
	}

	return &pb.HealthCheckResponse{
		Status: "healthy",
	}, nil
}

// assignedGoalToProto converts a service.AssignedGoal to a protobuf AssignedGoal message.
//
// This helper function is used by the InitializePlayer RPC to convert the service layer's
// AssignedGoal model to the protobuf representation for the gRPC response.
//
// Parameters:
// - goal: The service layer's AssignedGoal model
//
// Returns:
// - *pb.AssignedGoal: The protobuf representation
// - error: Conversion error (e.g., timestamp formatting)
func assignedGoalToProto(goal *service.AssignedGoal) (*pb.AssignedGoal, error) {
	if goal == nil {
		return nil, nil
	}

	protoGoal := &pb.AssignedGoal{
		ChallengeId: goal.ChallengeID,
		GoalId:      goal.GoalID,
		Name:        goal.Name,
		Description: goal.Description,
		IsActive:    goal.IsActive,
		// #nosec G115 - Progress values are bounded by config target values (safe to convert)
		Progress: int32(goal.Progress),
		// #nosec G115 - Target values are validated at config load time (safe to convert)
		Target: int32(goal.Target),
		Status: goal.Status,
	}

	// Convert assigned_at timestamp (nullable)
	if goal.AssignedAt != nil {
		protoGoal.AssignedAt = goal.AssignedAt.UTC().Format(time.RFC3339)
	}

	// Convert expires_at timestamp (nullable, NULL in M3)
	if goal.ExpiresAt != nil {
		protoGoal.ExpiresAt = goal.ExpiresAt.UTC().Format(time.RFC3339)
	}

	// Convert requirement
	protoRequirement, err := mapper.RequirementToProto(&goal.Requirement)
	if err == nil {
		protoGoal.Requirement = protoRequirement
	}

	// Convert reward
	protoReward, err := mapper.RewardToProto(&goal.Reward)
	if err == nil {
		protoGoal.Reward = protoReward
	}

	return protoGoal, nil
}
