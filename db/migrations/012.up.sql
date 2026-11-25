-- 008的单worker预算约束保持原字节；这里仅扩大可配置工作池的硬上限。
-- 实际准入仍由同一预算行锁按256MiB*workers核算，最大8个worker=2GiB。
USE document_db;

ALTER TABLE export_budget
 DROP CHECK export_budget_chk_2,
 ADD CONSTRAINT export_pool_budget_bounds CHECK (reserved_bytes<=2147483648 AND used_bytes<=2147483648);
