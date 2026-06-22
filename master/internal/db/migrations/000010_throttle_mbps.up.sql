ALTER TABLE instances ADD COLUMN IF NOT EXISTS throttle_mbps int DEFAULT 0;
ALTER TABLE instances ADD COLUMN IF NOT EXISTS over_limit_action varchar(16) DEFAULT '';
ALTER TABLE instances ADD COLUMN IF NOT EXISTS is_over_limit boolean DEFAULT false;
