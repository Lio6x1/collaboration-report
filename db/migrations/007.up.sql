-- Doc清理按固定context归属定位，再沿精确原operation_id扩展子输入。
-- 实际宽JSON扫描超过原三秒预算；派生列与payload同事务维护，不延长期限。
USE document_db;

ALTER TABLE durable_task
  ADD COLUMN cleanup_document VARCHAR(20) CHARACTER SET ascii COLLATE ascii_bin
    GENERATED ALWAYS AS (COALESCE(
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.capture.context.target.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.context.target.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.fixed.context.target.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.main.context.target.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.Context.target.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.binding.context.target.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.Main.context.target.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.Input.context.target.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.cut.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.observed_cut.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.document'))
    )) VIRTUAL,
  ADD INDEX task_document(cleanup_document,kind,id),
  ADD INDEX task_operation(operation_id,kind,id);
