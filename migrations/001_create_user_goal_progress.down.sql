-- Drop indexes first (dependency of table)
DROP INDEX IF EXISTS idx_user_goal_progress_user_active;
DROP INDEX IF EXISTS idx_user_goal_progress_user_challenge;

-- Drop table
DROP TABLE IF EXISTS user_goal_progress;
