DROP TABLE IF EXISTS theme_configurations;
ALTER TABLE site_configs DROP COLUMN IF EXISTS theme;
ALTER TABLE site_configs DROP COLUMN IF EXISTS admin_entry_path;
ALTER TABLE site_configs DROP COLUMN IF EXISTS auto_release_days;
