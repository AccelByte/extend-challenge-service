CREATE INDEX IF NOT EXISTS idx_user_goal_progress_expires_at
ON user_goal_progress(expires_at)
WHERE expires_at IS NOT NULL;
