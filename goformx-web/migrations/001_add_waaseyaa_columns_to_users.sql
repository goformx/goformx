-- Add Waaseyaa entity storage column to existing Laravel users table
-- Safe: additive only, no drops, no renames
ALTER TABLE users ADD COLUMN IF NOT EXISTS _data JSON NULL;
