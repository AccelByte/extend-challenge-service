-- Test BatchIncrementProgress performance issue
-- This script demonstrates the correlated subquery problem

-- Setup: Create test data
TRUNCATE TABLE user_goal_progress;

INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, is_active, assigned_at)
SELECT
    'user-' || i,
    'goal-' || i,
    'challenge-1',
    'test',
    5,
    'in_progress',
    true,
    NOW()
FROM generate_series(1, 100) AS i;

\echo '=================================================='
\echo 'Test 1: CURRENT BatchIncrementProgress Query (with correlated subqueries)'
\echo '=================================================='

-- Current implementation (simplified to show the problem)
EXPLAIN (ANALYZE, BUFFERS, TIMING)
WITH input_data AS (
    SELECT
        t.user_id,
        t.goal_id,
        t.challenge_id,
        t.namespace,
        t.delta,
        t.target_value,
        t.is_daily
    FROM UNNEST(
        ARRAY['user-1', 'user-2', 'user-3', 'user-4', 'user-5']::VARCHAR(100)[],
        ARRAY['goal-1', 'goal-2', 'goal-3', 'goal-4', 'goal-5']::VARCHAR(100)[],
        ARRAY['challenge-1', 'challenge-1', 'challenge-1', 'challenge-1', 'challenge-1']::VARCHAR(100)[],
        ARRAY['test', 'test', 'test', 'test', 'test']::VARCHAR(100)[],
        ARRAY[3, 3, 3, 3, 3]::INT[],
        ARRAY[10, 10, 10, 10, 10]::INT[],
        ARRAY[false, false, false, false, false]::BOOLEAN[]
    ) AS t(user_id, goal_id, challenge_id, namespace, delta, target_value, is_daily)
)
INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, created_at, updated_at)
SELECT user_id, goal_id, challenge_id, namespace, delta, 'in_progress', NOW(), NOW()
FROM input_data
ON CONFLICT (user_id, goal_id) DO UPDATE SET
    -- PROBLEM: Correlated subquery that runs for EACH updated row
    progress = user_goal_progress.progress + (
        SELECT delta FROM UNNEST(
            ARRAY[3, 3, 3, 3, 3]::INT[],
            ARRAY['goal-1', 'goal-2', 'goal-3', 'goal-4', 'goal-5']::VARCHAR(100)[]
        ) AS u(delta, gid)
        WHERE u.gid = user_goal_progress.goal_id
        LIMIT 1
    ),
    status = CASE
        WHEN user_goal_progress.progress + (
            SELECT delta FROM UNNEST(
                ARRAY[3, 3, 3, 3, 3]::INT[],
                ARRAY['goal-1', 'goal-2', 'goal-3', 'goal-4', 'goal-5']::VARCHAR(100)[]
            ) AS u(delta, gid)
            WHERE u.gid = user_goal_progress.goal_id
            LIMIT 1
        ) >= (
            SELECT target_value FROM UNNEST(
                ARRAY[10, 10, 10, 10, 10]::INT[],
                ARRAY['goal-1', 'goal-2', 'goal-3', 'goal-4', 'goal-5']::VARCHAR(100)[]
            ) AS u(target_value, gid)
            WHERE u.gid = user_goal_progress.goal_id
            LIMIT 1
        ) THEN 'completed'
        ELSE 'in_progress'
    END,
    updated_at = NOW()
WHERE user_goal_progress.status != 'claimed'
  AND user_goal_progress.is_active = true;

\echo ''
\echo '=================================================='
\echo 'Test 2: OPTIMIZED Query (join with CTE instead of correlated subqueries)'
\echo '=================================================='

-- Reset data
UPDATE user_goal_progress SET progress = 5, status = 'in_progress' WHERE user_id LIKE 'user-%';

-- Optimized implementation using JOIN
EXPLAIN (ANALYZE, BUFFERS, TIMING)
WITH input_data AS (
    SELECT
        t.user_id,
        t.goal_id,
        t.challenge_id,
        t.namespace,
        t.delta,
        t.target_value,
        t.is_daily
    FROM UNNEST(
        ARRAY['user-1', 'user-2', 'user-3', 'user-4', 'user-5']::VARCHAR(100)[],
        ARRAY['goal-1', 'goal-2', 'goal-3', 'goal-4', 'goal-5']::VARCHAR(100)[],
        ARRAY['challenge-1', 'challenge-1', 'challenge-1', 'challenge-1', 'challenge-1']::VARCHAR(100)[],
        ARRAY['test', 'test', 'test', 'test', 'test']::VARCHAR(100)[],
        ARRAY[3, 3, 3, 3, 3]::INT[],
        ARRAY[10, 10, 10, 10, 10]::INT[],
        ARRAY[false, false, false, false, false]::BOOLEAN[]
    ) AS t(user_id, goal_id, challenge_id, namespace, delta, target_value, is_daily)
),
updates AS (
    SELECT
        ugp.user_id,
        ugp.goal_id,
        ugp.progress + inp.delta AS new_progress,
        CASE
            WHEN ugp.progress + inp.delta >= inp.target_value THEN 'completed'
            ELSE 'in_progress'
        END AS new_status,
        CASE
            WHEN ugp.progress + inp.delta >= inp.target_value AND ugp.completed_at IS NULL THEN NOW()
            ELSE ugp.completed_at
        END AS new_completed_at
    FROM user_goal_progress ugp
    INNER JOIN input_data inp ON ugp.user_id = inp.user_id AND ugp.goal_id = inp.goal_id
    WHERE ugp.status != 'claimed'
      AND ugp.is_active = true
)
UPDATE user_goal_progress
SET
    progress = updates.new_progress,
    status = updates.new_status,
    completed_at = updates.new_completed_at,
    updated_at = NOW()
FROM updates
WHERE user_goal_progress.user_id = updates.user_id
  AND user_goal_progress.goal_id = updates.goal_id;

\echo ''
\echo '=================================================='
\echo 'Verification'
\echo '=================================================='
SELECT user_id, goal_id, progress, status FROM user_goal_progress WHERE user_id LIKE 'user-%' ORDER BY user_id LIMIT 10;

\echo ''
\echo '=================================================='
\echo 'Performance Summary'
\echo '=================================================='
\echo 'Current: Correlated subqueries (O(n*m) complexity)'
\echo 'Optimized: JOIN with CTE (O(n) complexity)'
\echo ''
\echo 'Expected improvement: 10-50x faster for large batches'
\echo '=================================================='
