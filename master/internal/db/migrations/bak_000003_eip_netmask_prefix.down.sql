ALTER TABLE eip_pools DROP COLUMN IF EXISTS netmask_prefix;
ALTER TABLE eip_allocations DROP COLUMN IF EXISTS alias;
ALTER TABLE eip_allocations DROP COLUMN IF EXISTS mapped_internal_ip;
