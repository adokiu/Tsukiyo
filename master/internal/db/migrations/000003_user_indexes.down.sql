-- 删除用户表搜索索引
DROP INDEX IF EXISTS idx_users_username_trgm;
DROP INDEX IF EXISTS idx_users_email_trgm;

-- 删除用户表状态索引
DROP INDEX IF EXISTS idx_users_status;

-- 删除用户表创建时间索引
DROP INDEX IF EXISTS idx_users_created_at;

-- 删除用户组成员关联表索引
DROP INDEX IF EXISTS idx_user_group_members_user_id;
