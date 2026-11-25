# db/migrations/service-grants.sql

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/017-service-grants.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义私有库、约束、索引或逐表授权；编号迁移只向前执行，不能修改已安装摘要。

主要声明：完整定义见下方源码。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````sql
-- Create users/passwords separately from deployment secrets. No passwords in this file.
GRANT SELECT ON account_db.schema_migration TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`user` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`session` TO 'collab_account'@'%';
GRANT SELECT, INSERT, DELETE ON account_db.`verification_consumption` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization_member` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization_invitation` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization_application` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization_creation_grant` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`identity_admission` TO 'collab_account'@'%';
GRANT SELECT, INSERT, DELETE ON account_db.`admission_source` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`account_close` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization_transition` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`operation_receipt` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`durable_task` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`access_gate` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`authorization_change` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`authorization_ack` TO 'collab_account'@'%';
GRANT SELECT, INSERT ON account_db.`outbox` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`notification` TO 'collab_account'@'%';
GRANT SELECT, INSERT ON account_db.`audit_event` TO 'collab_account'@'%';
GRANT SELECT, INSERT, UPDATE ON account_db.`platform_governance_grant` TO 'collab_account'@'%';
GRANT SELECT ON document_db.schema_migration TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`space` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`knowledge_base` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`knowledge_base_member` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`knowledge_base_creation_grant` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`admin_retirement` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`structure_revision` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`style_revision` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`chapter` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`collaboration_unit` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`structure_entry` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`content_grant` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`content_version` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`draft_body_capacity` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE ON document_db.`draft_body_reservation` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`reading_body_capacity` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`main_manifest` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`manifest_entry` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`branch` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`branch_lineage` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`unit_lineage` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`review_request` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`review_round` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`request_submitter` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`review_action` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`change_request` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`unit_review_guard` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`operation_receipt` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`submission_operation` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`merge_operation` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`merge_attempt` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE, UPDATE(approval_action_id) ON document_db.`merge_record` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`main_commit_barrier` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`release` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`resource` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`resource_reference` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`resource_protection` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document_invitation` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document_application` TO 'collab_document'@'%';
GRANT SELECT, INSERT ON document_db.`document_sharing_archive` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`quota_account` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`quota_reservation` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`temporary_budget` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`comment_thread` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`comment` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`export_job` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`lifecycle_operation` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`read_view` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document_admission` TO 'collab_document'@'%';
GRANT SELECT, INSERT, DELETE ON document_db.`admission_source` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`collaboration_ticket` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`delivery_session` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`output_instance` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`access_gate` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`authorization_change` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`authorization_ack` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`durable_task` TO 'collab_document'@'%';
GRANT SELECT, INSERT ON document_db.`outbox` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`notification` TO 'collab_document'@'%';
GRANT SELECT, INSERT ON document_db.`audit_event` TO 'collab_document'@'%';
GRANT SELECT ON collaboration_db.schema_migration TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`room` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, DELETE ON collaboration_db.`update_log` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, DELETE, UPDATE(validated_at) ON collaboration_db.`checkpoint` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`collaboration_grant` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, DELETE ON collaboration_db.`grant_source` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`flush_receipt` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`capture_operation` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`main_barrier` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, DELETE ON collaboration_db.`install_receipt` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`delivery_session` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`access_gate` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`authorization_change` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`authorization_ack` TO 'collab_collaboration'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`durable_task` TO 'collab_collaboration'@'%';
GRANT SELECT ON search_db.schema_migration TO 'collab_search'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`consumed_event` TO 'collab_search'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`document_index_revision` TO 'collab_search'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`rebuild_job` TO 'collab_search'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`index_generation` TO 'collab_search'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`cursor_registry` TO 'collab_search'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`durable_task` TO 'collab_search'@'%';
-- 003新增事实仍仅对Search应用账号开放，不提供Doc/Account私有表读取权限。
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`event_origin` TO 'collab_search'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`index_application` TO 'collab_search'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`consumer_checkpoint` TO 'collab_search'@'%';
-- 004快照原条目只追加；两侧应用账号仍无对方私有库权限。
GRANT SELECT, INSERT, UPDATE ON document_db.`search_snapshot_job` TO 'collab_document'@'%';
GRANT SELECT, INSERT ON document_db.`search_snapshot_entry` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE ON search_db.`rebuild_binding` TO 'collab_search'@'%';
GRANT SELECT, INSERT, UPDATE ON search_db.`rebuild_progress` TO 'collab_search'@'%';
-- 005物理日志绑定不可改写或删除，应用只能首次追加并核对原身份。
GRANT SELECT, INSERT ON search_db.`consumer_log_identity` TO 'collab_search'@'%';
-- 008导出固定计划与临时额度只属于Doc，其他服务仍不能读取它的私有事实。
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`export_control` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`export_budget` TO 'collab_document'@'%';
-- 010：归属迁移与容量分配只属于 Doc 私有库，其他服务仍无表访问权限。
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`personal_transfer` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document_capacity` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`resource_space_allocation` TO 'collab_document'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document_resource_allocation` TO 'collab_document'@'%';
````

UTF-8/LF全文SHA-256：`1ad7530accb664253ed0b8dcee9ee88cb2c80ae468169eee37ad5a98a4889a4a`。[参考实现](../../../../db/migrations/service-grants.sql)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
