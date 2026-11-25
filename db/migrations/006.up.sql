-- Node固定正文副本必须按文档定位，不能在三秒清理预算内反复扫描全部大JSON。
-- 派生列与payload由MySQL原事务共同维护；不存在应用遗漏反向索引的窗口。
USE collaboration_db;

ALTER TABLE durable_task
  ADD COLUMN cleanup_document VARCHAR(20) CHARACTER SET ascii COLLATE ascii_bin
    GENERATED ALWAYS AS (COALESCE(
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.binding.context.target.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.context.target.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.query.context.target.document_id')),
      JSON_UNQUOTE(JSON_EXTRACT(payload,'$.document_id'))
    )) VIRTUAL,
  ADD INDEX task_document(cleanup_document,kind,id);
