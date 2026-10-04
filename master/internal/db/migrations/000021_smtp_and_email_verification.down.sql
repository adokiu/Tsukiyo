DROP TABLE IF EXISTS email_verifications;
DROP TABLE IF EXISTS smtp_configs;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
