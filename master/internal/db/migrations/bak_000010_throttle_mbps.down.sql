ALTER TABLE instances DROP COLUMN IF EXISTS throttle_mbps;
ALTER TABLE instances DROP COLUMN IF EXISTS over_limit_action;
ALTER TABLE instances DROP COLUMN IF EXISTS is_over_limit;
