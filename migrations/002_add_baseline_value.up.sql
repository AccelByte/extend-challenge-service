-- M5: Add baseline_value column for relative progress mode
-- This is idempotent: IF NOT EXISTS prevents errors on fresh databases
-- where 001 already includes the column
ALTER TABLE user_goal_progress ADD COLUMN IF NOT EXISTS baseline_value INT NULL;

COMMENT ON COLUMN user_goal_progress.baseline_value IS 'M5: Stat value at goal activation for relative progress (NULL = absolute mode or not yet initialized)';
