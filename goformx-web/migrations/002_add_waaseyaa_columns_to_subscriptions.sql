-- Add Waaseyaa entity storage column to existing Cashier subscriptions table
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS _data JSON NULL;
