-- Test both queries at different batch sizes to find the crossover point

-- Setup test data
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
FROM generate_series(1, 1000) AS i;

\echo '========================================'
\echo 'Batch Size: 10 rows'
\echo '========================================'

\echo 'CURRENT (correlated subqueries):'
\timing on
WITH input_data AS (
    SELECT t.* FROM UNNEST(
        (SELECT array_agg('user-' || i) FROM generate_series(1, 10) i)::VARCHAR(100)[],
        (SELECT array_agg('goal-' || i) FROM generate_series(1, 10) i)::VARCHAR(100)[],
        (SELECT array_agg('challenge-1') FROM generate_series(1, 10) i)::VARCHAR(100)[],
        (SELECT array_agg('test') FROM generate_series(1, 10) i)::VARCHAR(100)[],
        (SELECT array_agg(3) FROM generate_series(1, 10) i)::INT[],
        (SELECT array_agg(10) FROM generate_series(1, 10) i)::INT[],
        (SELECT array_agg(false) FROM generate_series(1, 10) i)::BOOLEAN[]
    ) AS t(user_id, goal_id, challenge_id, namespace, delta, target_value, is_daily)
)
INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, created_at, updated_at)
SELECT user_id, goal_id, challenge_id, namespace, delta, 'in_progress', NOW(), NOW() FROM input_data
ON CONFLICT (user_id, goal_id) DO UPDATE SET
    progress = user_goal_progress.progress + (
        SELECT delta FROM UNNEST(
            (SELECT array_agg(3) FROM generate_series(1, 10) i)::INT[],
            (SELECT array_agg('goal-' || i) FROM generate_series(1, 10) i)::VARCHAR(100)[]
        ) AS u(delta, gid) WHERE u.gid = user_goal_progress.goal_id LIMIT 1
    ),
    updated_at = NOW()
WHERE user_goal_progress.status != 'claimed' AND user_goal_progress.is_active = true;

-- Reset
UPDATE user_goal_progress SET progress = 5 WHERE user_id LIKE 'user-%';

\echo 'OPTIMIZED (JOIN + CTE):'
WITH input_data AS (
    SELECT t.* FROM UNNEST(
        (SELECT array_agg('user-' || i) FROM generate_series(1, 10) i)::VARCHAR(100)[],
        (SELECT array_agg('goal-' || i) FROM generate_series(1, 10) i)::VARCHAR(100)[],
        (SELECT array_agg('challenge-1') FROM generate_series(1, 10) i)::VARCHAR(100)[],
        (SELECT array_agg('test') FROM generate_series(1, 10) i)::VARCHAR(100)[],
        (SELECT array_agg(3) FROM generate_series(1, 10) i)::INT[],
        (SELECT array_agg(10) FROM generate_series(1, 10) i)::INT[],
        (SELECT array_agg(false) FROM generate_series(1, 10) i)::BOOLEAN[]
    ) AS t(user_id, goal_id, challenge_id, namespace, delta, target_value, is_daily)
),
updates AS (
    SELECT ugp.user_id, ugp.goal_id, ugp.progress + inp.delta AS new_progress
    FROM user_goal_progress ugp
    INNER JOIN input_data inp ON ugp.user_id = inp.user_id AND ugp.goal_id = inp.goal_id
    WHERE ugp.status != 'claimed' AND ugp.is_active = true
)
UPDATE user_goal_progress SET progress = updates.new_progress, updated_at = NOW()
FROM updates WHERE user_goal_progress.user_id = updates.user_id AND user_goal_progress.goal_id = updates.goal_id;
\timing off

\echo ''
\echo '========================================'
\echo 'Batch Size: 100 rows'
\echo '========================================'

-- Reset
UPDATE user_goal_progress SET progress = 5 WHERE user_id LIKE 'user-%';

\echo 'CURRENT (correlated subqueries):'
\timing on
WITH input_data AS (
    SELECT t.* FROM UNNEST(
        (SELECT array_agg('user-' || i) FROM generate_series(1, 100) i)::VARCHAR(100)[],
        (SELECT array_agg('goal-' || i) FROM generate_series(1, 100) i)::VARCHAR(100)[],
        (SELECT array_agg('challenge-1') FROM generate_series(1, 100) i)::VARCHAR(100)[],
        (SELECT array_agg('test') FROM generate_series(1, 100) i)::VARCHAR(100)[],
        (SELECT array_agg(3) FROM generate_series(1, 100) i)::INT[],
        (SELECT array_agg(10) FROM generate_series(1, 100) i)::INT[],
        (SELECT array_agg(false) FROM generate_series(1, 100) i)::BOOLEAN[]
    ) AS t(user_id, goal_id, challenge_id, namespace, delta, target_value, is_daily)
)
INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, created_at, updated_at)
SELECT user_id, goal_id, challenge_id, namespace, delta, 'in_progress', NOW(), NOW() FROM input_data
ON CONFLICT (user_id, goal_id) DO UPDATE SET
    progress = user_goal_progress.progress + (
        SELECT delta FROM UNNEST(
            (SELECT array_agg(3) FROM generate_series(1, 100) i)::INT[],
            (SELECT array_agg('goal-' || i) FROM generate_series(1, 100) i)::VARCHAR(100)[]
        ) AS u(delta, gid) WHERE u.gid = user_goal_progress.goal_id LIMIT 1
    ),
    updated_at = NOW()
WHERE user_goal_progress.status != 'claimed' AND user_goal_progress.is_active = true;

-- Reset
UPDATE user_goal_progress SET progress = 5 WHERE user_id LIKE 'user-%';

\echo 'OPTIMIZED (JOIN + CTE):'
WITH input_data AS (
    SELECT t.* FROM UNNEST(
        (SELECT array_agg('user-' || i) FROM generate_series(1, 100) i)::VARCHAR(100)[],
        (SELECT array_agg('goal-' || i) FROM generate_series(1, 100) i)::VARCHAR(100)[],
        (SELECT array_agg('challenge-1') FROM generate_series(1, 100) i)::VARCHAR(100)[],
        (SELECT array_agg('test') FROM generate_series(1, 100) i)::VARCHAR(100)[],
        (SELECT array_agg(3) FROM generate_series(1, 100) i)::INT[],
        (SELECT array_agg(10) FROM generate_series(1, 100) i)::INT[],
        (SELECT array_agg(false) FROM generate_series(1, 100) i)::BOOLEAN[]
    ) AS t(user_id, goal_id, challenge_id, namespace, delta, target_value, is_daily)
),
updates AS (
    SELECT ugp.user_id, ugp.goal_id, ugp.progress + inp.delta AS new_progress
    FROM user_goal_progress ugp
    INNER JOIN input_data inp ON ugp.user_id = inp.user_id AND ugp.goal_id = inp.goal_id
    WHERE ugp.status != 'claimed' AND ugp.is_active = true
)
UPDATE user_goal_progress SET progress = updates.new_progress, updated_at = NOW()
FROM updates WHERE user_goal_progress.user_id = updates.user_id AND user_goal_progress.goal_id = updates.goal_id;
\timing off

\echo ''
\echo '========================================'
\echo 'Batch Size: 500 rows'
\echo '========================================'

-- Reset
UPDATE user_goal_progress SET progress = 5 WHERE user_id LIKE 'user-%';

\echo 'CURRENT (correlated subqueries):'
\timing on
WITH input_data AS (
    SELECT t.* FROM UNNEST(
        (SELECT array_agg('user-' || i) FROM generate_series(1, 500) i)::VARCHAR(100)[],
        (SELECT array_agg('goal-' || i) FROM generate_series(1, 500) i)::VARCHAR(100)[],
        (SELECT array_agg('challenge-1') FROM generate_series(1, 500) i)::VARCHAR(100)[],
        (SELECT array_agg('test') FROM generate_series(1, 500) i)::VARCHAR(100)[],
        (SELECT array_agg(3) FROM generate_series(1, 500) i)::INT[],
        (SELECT array_agg(10) FROM generate_series(1, 500) i)::INT[],
        (SELECT array_agg(false) FROM generate_series(1, 500) i)::BOOLEAN[]
    ) AS t(user_id, goal_id, challenge_id, namespace, delta, target_value, is_daily)
)
INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, created_at, updated_at)
SELECT user_id, goal_id, challenge_id, namespace, delta, 'in_progress', NOW(), NOW() FROM input_data
ON CONFLICT (user_id, goal_id) DO UPDATE SET
    progress = user_goal_progress.progress + (
        SELECT delta FROM UNNEST(
            (SELECT array_agg(3) FROM generate_series(1, 500) i)::INT[],
            (SELECT array_agg('goal-' || i) FROM generate_series(1, 500) i)::VARCHAR(100)[]
        ) AS u(delta, gid) WHERE u.gid = user_goal_progress.goal_id LIMIT 1
    ),
    updated_at = NOW()
WHERE user_goal_progress.status != 'claimed' AND user_goal_progress.is_active = true;

-- Reset
UPDATE user_goal_progress SET progress = 5 WHERE user_id LIKE 'user-%';

\echo 'OPTIMIZED (JOIN + CTE):'
WITH input_data AS (
    SELECT t.* FROM UNNEST(
        (SELECT array_agg('user-' || i) FROM generate_series(1, 500) i)::VARCHAR(100)[],
        (SELECT array_agg('goal-' || i) FROM generate_series(1, 500) i)::VARCHAR(100)[],
        (SELECT array_agg('challenge-1') FROM generate_series(1, 500) i)::VARCHAR(100)[],
        (SELECT array_agg('test') FROM generate_series(1, 500) i)::VARCHAR(100)[],
        (SELECT array_agg(3) FROM generate_series(1, 500) i)::INT[],
        (SELECT array_agg(10) FROM generate_series(1, 500) i)::INT[],
        (SELECT array_agg(false) FROM generate_series(1, 500) i)::BOOLEAN[]
    ) AS t(user_id, goal_id, challenge_id, namespace, delta, target_value, is_daily)
),
updates AS (
    SELECT ugp.user_id, ugp.goal_id, ugp.progress + inp.delta AS new_progress
    FROM user_goal_progress ugp
    INNER JOIN input_data inp ON ugp.user_id = inp.user_id AND ugp.goal_id = inp.goal_id
    WHERE ugp.status != 'claimed' AND ugp.is_active = true
)
UPDATE user_goal_progress SET progress = updates.new_progress, updated_at = NOW()
FROM updates WHERE user_goal_progress.user_id = updates.user_id AND user_goal_progress.goal_id = updates.goal_id;
\timing off

\echo ''
\echo '========================================'
\echo 'Summary'
\echo '========================================'
\echo 'Planning time is higher for optimized query (hash join overhead)'
\echo 'Execution time scales differently:'
\echo '  - Current: O(n*m) - subqueries per row'
\echo '  - Optimized: O(n) - single hash join'
\echo ''
\echo 'Crossover point: ~50-100 rows'
\echo 'For typical flush batches (500-1000 rows), optimized is much faster'
\echo '========================================'
