-- Add Waaseyaa entity storage column to existing Cashier subscription_items table
ALTER TABLE subscription_items ADD COLUMN IF NOT EXISTS _data JSON NULL;
-- Add meter columns if not present (added by later Cashier migrations)
ALTER TABLE subscription_items ADD COLUMN IF NOT EXISTS stripe_meter_id VARCHAR(255) NULL;
ALTER TABLE subscription_items ADD COLUMN IF NOT EXISTS stripe_meter_event_name VARCHAR(255) NULL;
