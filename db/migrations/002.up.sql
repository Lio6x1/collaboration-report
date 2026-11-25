-- 001原字节和已安装SHA保持不变；这里只添加物理索引，不删除业务或恢复事实。
-- 基础非终态核验只需state/kind，避免读取大JSON所在的聚簇记录。
ALTER TABLE account_db.durable_task ADD INDEX task_pending(state,kind);
ALTER TABLE document_db.durable_task ADD INDEX task_pending(state,kind);
ALTER TABLE collaboration_db.durable_task ADD INDEX task_pending(state,kind);
ALTER TABLE search_db.durable_task ADD INDEX task_pending(state,kind);

-- 按确切业务种类筛选小ID；领取仍以主键当前读锁和同一事务原payload为准。
ALTER TABLE document_db.durable_task ADD INDEX task_kind_due(kind,state,retry_at,lease_until,id);

-- 二级索引隐含scope_key主键；关闭证明的两侧都不需要加载JSON字段。
ALTER TABLE account_db.access_gate ADD INDEX gate_recovery(state,last_revocation_id,max_epoch);
ALTER TABLE document_db.access_gate ADD INDEX gate_recovery(state,last_revocation_id,max_epoch);
ALTER TABLE collaboration_db.access_gate ADD INDEX gate_recovery(state,last_revocation_id,max_epoch);
ALTER TABLE account_db.authorization_change ADD INDEX auth_recovery(revocation_id,scope_key,epoch,state);
ALTER TABLE document_db.authorization_change ADD INDEX auth_recovery(revocation_id,scope_key,epoch,state);
ALTER TABLE collaboration_db.authorization_change ADD INDEX auth_recovery(revocation_id,scope_key,epoch,state);
