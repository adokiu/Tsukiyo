-- EIP 池添加动态IP绑定字段
ALTER TABLE eip_pools ADD COLUMN IF NOT EXISTS dynamic_binding BOOLEAN NOT NULL DEFAULT false;
