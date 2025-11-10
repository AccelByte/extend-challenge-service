package mapper

import (
	"fmt"
	"sync"
	"time"

	pb "extend-challenge-service/pkg/pb"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// Object pools for protobuf message reuse (reduces allocations by ~9.3%)
var (
	goalPool = sync.Pool{
		New: func() interface{} {
			return &pb.Goal{}
		},
	}

	requirementPool = sync.Pool{
		New: func() interface{} {
			return &pb.Requirement{}
		},
	}

	rewardPool = sync.Pool{
		New: func() interface{} {
			return &pb.Reward{}
		},
	}

	challengePool = sync.Pool{
		New: func() interface{} {
			return &pb.Challenge{}
		},
	}
)

// ChallengeToProto converts domain Challenge to protobuf Challenge (Decision Q2)
// Returns error for validation failures (early validation, Decision Q2a)
// Uses object pooling to reduce allocations
func ChallengeToProto(challenge *domain.Challenge, userProgress map[string]*domain.UserGoalProgress) (*pb.Challenge, error) {
	if challenge == nil {
		return nil, fmt.Errorf("challenge cannot be nil")
	}

	// Get from pool and reset
	pbChallenge := challengePool.Get().(*pb.Challenge)
	pbChallenge.ChallengeId = challenge.ID
	pbChallenge.Name = challenge.Name
	pbChallenge.Description = challenge.Description
	// Reuse slice capacity if possible
	if cap(pbChallenge.Goals) >= len(challenge.Goals) {
		pbChallenge.Goals = pbChallenge.Goals[:0]
	} else {
		pbChallenge.Goals = make([]*pb.Goal, 0, len(challenge.Goals))
	}

	for _, goal := range challenge.Goals {
		pbGoal, err := GoalToProto(goal, userProgress)
		if err != nil {
			return nil, fmt.Errorf("failed to convert goal %s: %w", goal.ID, err)
		}
		pbChallenge.Goals = append(pbChallenge.Goals, pbGoal)
	}

	return pbChallenge, nil
}

// GoalToProto converts domain Goal to protobuf Goal with user progress (Decision Q2)
// Computes progress for daily goals from completed_at timestamp (Decision FQ2)
// Uses object pooling to reduce allocations
func GoalToProto(goal *domain.Goal, userProgress map[string]*domain.UserGoalProgress) (*pb.Goal, error) {
	if goal == nil {
		return nil, fmt.Errorf("goal cannot be nil")
	}

	// Get user progress for this goal (if exists)
	progress, exists := userProgress[goal.ID]

	// Convert requirement
	pbRequirement, err := RequirementToProto(&goal.Requirement)
	if err != nil {
		return nil, fmt.Errorf("failed to convert requirement: %w", err)
	}

	// Convert reward
	pbReward, err := RewardToProto(&goal.Reward)
	if err != nil {
		return nil, fmt.Errorf("failed to convert reward: %w", err)
	}

	// Get from pool and reset
	pbGoal := goalPool.Get().(*pb.Goal)
	pbGoal.GoalId = goal.ID
	pbGoal.Name = goal.Name
	pbGoal.Description = goal.Description
	pbGoal.Requirement = pbRequirement
	pbGoal.Reward = pbReward
	// Reuse slice capacity for prerequisites
	if cap(pbGoal.Prerequisites) >= len(goal.Prerequisites) {
		pbGoal.Prerequisites = pbGoal.Prerequisites[:0]
	} else {
		pbGoal.Prerequisites = make([]string, 0, len(goal.Prerequisites))
	}
	pbGoal.Prerequisites = append(pbGoal.Prerequisites, goal.Prerequisites...)

	// Set progress fields from user progress (if exists)
	if exists && progress != nil {
		pbGoal.Progress = ComputeProgress(goal, progress)
		pbGoal.Status = string(progress.Status)
		pbGoal.Locked = false // Will be computed by PrerequisiteChecker

		// Format timestamps using AppendFormat to reuse buffer (reduces allocations)
		var buf []byte
		if progress.CompletedAt != nil && !progress.CompletedAt.IsZero() {
			buf = progress.CompletedAt.UTC().AppendFormat(buf[:0], time.RFC3339)
			pbGoal.CompletedAt = string(buf)
		}
		if progress.ClaimedAt != nil && !progress.ClaimedAt.IsZero() {
			buf = progress.ClaimedAt.UTC().AppendFormat(buf[:0], time.RFC3339)
			pbGoal.ClaimedAt = string(buf)
		}
	} else {
		// No progress yet
		pbGoal.Progress = 0
		pbGoal.Status = string(domain.GoalStatusNotStarted)
		pbGoal.Locked = len(goal.Prerequisites) > 0 // Will be refined by PrerequisiteChecker
		pbGoal.CompletedAt = ""
		pbGoal.ClaimedAt = ""
	}

	return pbGoal, nil
}

// ComputeProgress computes the progress value for display (Decision FQ2)
// For daily goals: Returns 1 if completed today, 0 otherwise
// For other goals: Returns the actual progress value from DB
func ComputeProgress(goal *domain.Goal, progress *domain.UserGoalProgress) int32 {
	if goal == nil || progress == nil {
		return 0
	}

	// Daily goals: Check if completed today (Decision Q9, FQ2)
	if goal.Type == domain.GoalTypeDaily {
		// If completed_at is today (UTC), show progress = 1
		// Compare dates directly without string allocation
		if progress.CompletedAt != nil && !progress.CompletedAt.IsZero() {
			cy, cm, cd := progress.CompletedAt.UTC().Date()
			ty, tm, td := time.Now().UTC().Date()
			if cy == ty && cm == tm && cd == td {
				return 1
			}
		}
		// Not completed today
		return 0
	}

	// Absolute and increment goals: Return actual progress from DB
	// #nosec G115 - Progress values are validated at config load time, safe to convert
	return int32(progress.Progress)
}

// RequirementToProto converts domain Requirement to protobuf Requirement
// Uses object pooling to reduce allocations
func RequirementToProto(req *domain.Requirement) (*pb.Requirement, error) {
	if req == nil {
		return nil, fmt.Errorf("requirement cannot be nil")
	}

	// Get from pool and reset
	pbReq := requirementPool.Get().(*pb.Requirement)
	pbReq.StatCode = req.StatCode
	pbReq.Operator = req.Operator
	// #nosec G115 - Target values are validated at config load time, safe to convert
	pbReq.TargetValue = int32(req.TargetValue)

	return pbReq, nil
}

// RewardToProto converts domain Reward to protobuf Reward
// Uses object pooling to reduce allocations
func RewardToProto(reward *domain.Reward) (*pb.Reward, error) {
	if reward == nil {
		return nil, fmt.Errorf("reward cannot be nil")
	}

	// Validate reward type (Decision Q2a: early validation)
	if reward.Type != string(domain.RewardTypeItem) && reward.Type != string(domain.RewardTypeWallet) {
		return nil, fmt.Errorf("invalid reward type: %s (must be ITEM or WALLET)", reward.Type)
	}

	// Get from pool and reset
	pbReward := rewardPool.Get().(*pb.Reward)
	pbReward.Type = reward.Type
	pbReward.RewardId = reward.RewardID
	// #nosec G115 - Reward quantities are validated at config load time, safe to convert
	pbReward.Quantity = int32(reward.Quantity)

	return pbReward, nil
}

// ChallengesToProto converts a slice of domain Challenges to protobuf Challenges
func ChallengesToProto(challenges []*domain.Challenge, userProgress map[string]*domain.UserGoalProgress) ([]*pb.Challenge, error) {
	if challenges == nil {
		return nil, fmt.Errorf("challenges cannot be nil")
	}

	pbChallenges := make([]*pb.Challenge, 0, len(challenges))
	for _, challenge := range challenges {
		pbChallenge, err := ChallengeToProto(challenge, userProgress)
		if err != nil {
			return nil, fmt.Errorf("failed to convert challenge %s: %w", challenge.ID, err)
		}
		pbChallenges = append(pbChallenges, pbChallenge)
	}

	return pbChallenges, nil
}

// ReturnChallengeToPool returns a protobuf Challenge and all nested objects back to their pools.
// Must be called after the challenge has been serialized to JSON.
func ReturnChallengeToPool(challenge *pb.Challenge) {
	if challenge == nil {
		return
	}

	// Return nested goals first
	for _, goal := range challenge.Goals {
		ReturnGoalToPool(goal)
	}

	// Return the challenge itself
	challengePool.Put(challenge)
}

// ReturnGoalToPool returns a protobuf Goal and its nested objects back to their pools.
// Must be called after the goal has been serialized to JSON.
func ReturnGoalToPool(goal *pb.Goal) {
	if goal == nil {
		return
	}

	// Return nested objects first
	if goal.Requirement != nil {
		requirementPool.Put(goal.Requirement)
	}
	if goal.Reward != nil {
		rewardPool.Put(goal.Reward)
	}

	// Return the goal itself
	goalPool.Put(goal)
}

// ReturnChallengesToPool returns a slice of protobuf Challenges back to their pools.
// Must be called after all challenges have been serialized to JSON.
func ReturnChallengesToPool(challenges []*pb.Challenge) {
	for _, challenge := range challenges {
		ReturnChallengeToPool(challenge)
	}
}

// ReturnRewardToPool returns a protobuf Reward back to the pool.
// Must be called after the reward has been serialized to JSON.
func ReturnRewardToPool(reward *pb.Reward) {
	if reward == nil {
		return
	}
	rewardPool.Put(reward)
}
