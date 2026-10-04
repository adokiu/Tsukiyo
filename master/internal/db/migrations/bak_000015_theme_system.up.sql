-- 主题配置表
CREATE TABLE IF NOT EXISTS theme_configurations (
    short VARCHAR(64) PRIMARY KEY,
    data TEXT DEFAULT '{}'
);

-- 站点配置表新增字段
ALTER TABLE site_configs ADD COLUMN IF NOT EXISTS theme VARCHAR(64) DEFAULT 'default';
ALTER TABLE site_configs ADD COLUMN IF NOT EXISTS admin_entry_path VARCHAR(32);
ALTER TABLE site_configs ADD COLUMN IF NOT EXISTS auto_release_days INTEGER NOT NULL DEFAULT 7;
