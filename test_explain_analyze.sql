-- M3 Phase 5: EXPLAIN ANALYZE Performance Verification
-- This script tests the performance of event processing queries with is_active filtering

-- Setup: Create test data
INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, is_active, assigned_at)
VALUES
    ('test-user-1', 'test-goal-1', 'challenge-1', 'test', 5, 'in_progress', true, NOW()),
    ('test-user-2', 'test-goal-2', 'challenge-1', 'test', 3, 'in_progress', false, NOW());

\echo '=================================================='
\echo 'Test 1: IncrementProgress - Update Active Goal'
\echo '=================================================='
EXPLAIN (ANALYZE, BUFFERS, TIMING, VERBOSE)
INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, updated_at)
VALUES ('test-user-1', 'test-goal-1', 'challenge-1', 'test', 10, 'completed', NOW())
ON CONFLICT (user_id, goal_id) DO UPDATE SET
    progress = EXCLUDED.progress,
    status = EXCLUDED.status,
    updated_at = NOW()
WHERE user_goal_progress.status != 'claimed'
  AND user_goal_progress.is_active = true;

\echo ''
\echo '=================================================='
\echo 'Test 2: IncrementProgress - Try Update Inactive Goal (Should Skip)'
\echo '=================================================='
EXPLAIN (ANALYZE, BUFFERS, TIMING, VERBOSE)
INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, updated_at)
VALUES ('test-user-2', 'test-goal-2', 'challenge-1', 'test', 10, 'completed', NOW())
ON CONFLICT (user_id, goal_id) DO UPDATE SET
    progress = EXCLUDED.progress,
    status = EXCLUDED.status,
    updated_at = NOW()
WHERE user_goal_progress.status != 'claimed'
  AND user_goal_progress.is_active = true;

\echo ''
\echo '=================================================='
\echo 'Test 3: Verify Results'
\echo '=================================================='
SELECT
    user_id,
    goal_id,
    progress,
    is_active,
    CASE
        WHEN user_id = 'test-user-1' AND progress = 10 THEN '✓ Active goal updated'
        WHEN user_id = 'test-user-2' AND progress = 3 THEN '✓ Inactive goal NOT updated'
        ELSE '✗ UNEXPECTED RESULT'
    END as result
FROM user_goal_progress
WHERE user_id IN ('test-user-1', 'test-user-2')
ORDER BY user_id;

\echo ''
\echo '=================================================='
\echo 'Test 4: BatchIncrementProgress Pattern'
\echo '=================================================='
-- Simulate batch increment with UNNEST pattern
EXPLAIN (ANALYZE, BUFFERS, TIMING, VERBOSE)
INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, updated_at)
SELECT * FROM (VALUES
    ('batch-user-1', 'batch-goal-1', 'challenge-1', 'test', 15, 'in_progress', NOW()),
    ('batch-user-2', 'batch-goal-2', 'challenge-1', 'test', 20, 'completed', NOW())
) AS v(user_id, goal_id, challenge_id, namespace, progress, status, updated_at)
ON CONFLICT (user_id, goal_id) DO UPDATE SET
    progress = user_goal_progress.progress + EXCLUDED.progress,
    status = EXCLUDED.status,
    updated_at = NOW()
WHERE user_goal_progress.status != 'claimed'
  AND user_goal_progress.is_active = true;

\echo ''
\echo '=================================================='
\echo 'Performance Summary'
\echo '=================================================='
\echo 'Expected Results:'
\echo '  - Execution Time: < 1ms per query'
\echo '  - Index Used: PRIMARY KEY (user_id, goal_id)'
\echo '  - Conflict Filter: status != claimed AND is_active = true'
\echo '  - Active goals: Updated'
\echo '  - Inactive goals: Skipped (no update)'
\echo '=================================================='
