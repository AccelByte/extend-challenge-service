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
		(user_id, goal_id, challenge_id, namespace, progress, status, is_active, completed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, userID, goalID, challengeID, "test-namespace", 10, "completed", false, time.Now(), time.Now(), time.Now())

	if err != nil {
		t.Fatalf("Failed to seed completed goal: %v", err)
	}
}

// seedInProgressGoal inserts an in-progress goal into the database
func seedInProgressGoal(t *testing.T, db *sql.DB, userID, goalID, challengeID string, currentProgress, targetProgress int) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO user_goal_progress
		(user_id, goal_id, challenge_id, namespace, progress, status, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, userID, goalID, challengeID, "test-namespace", currentProgress, "in_progress", false, time.Now(), time.Now())

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
		(user_id, goal_id, challenge_id, namespace, progress, status, is_active, assigned_at, completed_at, claimed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, userID, goalID, challengeID, "test-namespace", 10, "claimed", true, now, now, now, now, now)

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
		(user_id, goal_id, challenge_id, namespace, progress, status, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, userID, goalID, challengeID, "test-namespace", 0, "not_started", false, time.Now(), time.Now())

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

// seedInProgressActiveGoal inserts an in-progress AND active goal into the database
func seedInProgressActiveGoal(t *testing.T, db *sql.DB, userID, goalID, challengeID string, currentProgress, targetProgress int) {
	t.Helper()

	now := time.Now()
	_, err := db.Exec(`
		INSERT INTO user_goal_progress
		(user_id, goal_id, challenge_id, namespace, progress, status, is_active, assigned_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, userID, goalID, challengeID, "test-namespace", currentProgress, "in_progress", true, now, now, now)

	if err != nil {
		t.Fatalf("Failed to seed in-progress active goal: %v", err)
	}
}

// seedCompletedActiveGoal inserts a completed AND active goal into the database (for claim tests)
func seedCompletedActiveGoal(t *testing.T, db *sql.DB, userID, goalID, challengeID string) {
	t.Helper()

	now := time.Now()
	_, err := db.Exec(`
		INSERT INTO user_goal_progress
		(user_id, goal_id, challenge_id, namespace, progress, status, is_active, assigned_at, completed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, userID, goalID, challengeID, "test-namespace", 10, "completed", true, now, now, now, now)

	if err != nil {
		t.Fatalf("Failed to seed completed active goal: %v", err)
	}
}

// seedActiveGoal inserts an active goal into the database (M4 helper)
func seedActiveGoal(t *testing.T, db *sql.DB, userID, goalID, challengeID string) {
	t.Helper()

	now := time.Now()
	_, err := db.Exec(`
		INSERT INTO user_goal_progress
		(user_id, goal_id, challenge_id, namespace, progress, status, is_active, assigned_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, userID, goalID, challengeID, "test-namespace", 0, "not_started", true, now, now, now)

	if err != nil {
		t.Fatalf("Failed to seed active goal: %v", err)
	}
}

// getGoalActiveStatus queries the database to check if a goal is active (M4 helper)
func getGoalActiveStatus(t *testing.T, db *sql.DB, userID, goalID string) bool {
	t.Helper()

	var isActive bool
	err := db.QueryRow(`
		SELECT is_active FROM user_goal_progress
		WHERE user_id = $1 AND goal_id = $2
	`, userID, goalID).Scan(&isActive)

	if err == sql.ErrNoRows {
		return false // Goal not assigned
	}

	if err != nil {
		t.Fatalf("Failed to query goal active status: %v", err)
	}

	return isActive
}

// countActiveGoals counts how many active goals a user has for a challenge (M4 helper)
func countActiveGoals(t *testing.T, db *sql.DB, userID, challengeID string) int {
	t.Helper()

	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM user_goal_progress
		WHERE user_id = $1 AND challenge_id = $2 AND is_active = true
	`, userID, challengeID).Scan(&count)

	if err != nil {
		t.Fatalf("Failed to count active goals: %v", err)
	}

	return count
}

// findSelectedGoal finds a selected goal by ID in the response (M4 helper)
func findSelectedGoal(goals []*pb.SelectedGoal, goalID string) *pb.SelectedGoal {
	for _, g := range goals {
		if g.GoalId == goalID {
			return g
		}
	}
	return nil
}
