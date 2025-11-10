package integration

import (
	"database/sql"
	"testing"
	"time"

	pb "extend-challenge-service/pkg/pb"
)

// seedCompletedGoal inserts a completed goal into the database
func seedCompletedGoal(t *testing.T, db *sql.DB, userID, goalID, challengeID string) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO user_goal_progress
		(user_id, goal_id, challenge_id, namespace, progress, status, completed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, userID, goalID, challengeID, "test-namespace", 10, "completed", time.Now(), time.Now(), time.Now())

	if err != nil {
		t.Fatalf("Failed to seed completed goal: %v", err)
	}
}

// seedInProgressGoal inserts an in-progress goal into the database
func seedInProgressGoal(t *testing.T, db *sql.DB, userID, goalID, challengeID string, currentProgress, targetProgress int) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO user_goal_progress
		(user_id, goal_id, challenge_id, namespace, progress, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, userID, goalID, challengeID, "test-namespace", currentProgress, "in_progress", time.Now(), time.Now())

	if err != nil {
		t.Fatalf("Failed to seed in-progress goal: %v", err)
	}
}

// seedClaimedGoal inserts an already-claimed goal into the database
func seedClaimedGoal(t *testing.T, db *sql.DB, userID, goalID, challengeID string) {
	t.Helper()

	now := time.Now()
	_, err := db.Exec(`
		INSERT INTO user_goal_progress
		(user_id, goal_id, challenge_id, namespace, progress, status, completed_at, claimed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, userID, goalID, challengeID, "test-namespace", 10, "claimed", now, now, now, now)

	if err != nil {
		t.Fatalf("Failed to seed claimed goal: %v", err)
	}
}

// seedNotStartedGoal inserts a not-started goal (no progress)
// Note: Currently unused but kept for potential future test scenarios
func seedNotStartedGoal(t *testing.T, db *sql.DB, userID, goalID, challengeID string) { //nolint:unused
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO user_goal_progress
		(user_id, goal_id, challenge_id, namespace, progress, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, userID, goalID, challengeID, "test-namespace", 0, "not_started", time.Now(), time.Now())

	if err != nil {
		t.Fatalf("Failed to seed not-started goal: %v", err)
	}
}

// findChallenge finds a challenge by ID in the response
func findChallenge(challenges []*pb.Challenge, challengeID string) *pb.Challenge {
	for _, c := range challenges {
		if c.ChallengeId == challengeID {
			return c
		}
	}
	return nil
}

// findGoal finds a goal by ID in a challenge's goals
func findGoal(goals []*pb.Goal, goalID string) *pb.Goal {
	for _, g := range goals {
		if g.GoalId == goalID {
			return g
		}
	}
	return nil
}
