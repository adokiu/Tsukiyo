-- 启用 pg_trgm 扩展（支持三字符模糊搜索索引）
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 用户表搜索索引：username 和 email 的 trigram GIN 索引，加速 ILIKE '%keyword%'
CREATE INDEX IF NOT EXISTS idx_users_username_trgm ON users USING gin (username gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_users_email_trgm ON users USING gin (email gin_trgm_ops);

-- 用户表状态索引
CREATE INDEX IF NOT EXISTS idx_users_status ON users (status) WHERE deleted_at IS NULL;

-- 用户表创建时间索引（ORDER BY created_at DESC）
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users (created_at DESC) WHERE deleted_at IS NULL;

-- 用户组成员关联表：按 user_id 查询索引
CREATE INDEX IF NOT EXISTS idx_user_group_members_user_id ON user_group_members (user_id);
