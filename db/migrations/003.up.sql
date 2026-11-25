-- 003只追加Search私有事实，保留001/002原字节和现有业务表。
-- 原消息身份与后来Doc权威结果分开保存；无法用一次跨ES/SQL/Kafka事务代替恢复。
USE search_db;

CREATE TABLE event_origin (
  consumer_name VARCHAR(40) NOT NULL,
  event_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  raw_digest BINARY(32) NOT NULL,
  original_row MEDIUMBLOB NOT NULL,
  kafka_topic VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  kafka_partition INT UNSIGNED NOT NULL,
  kafka_offset BIGINT UNSIGNED NOT NULL,
  applied_index VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NULL,
  applied_seq BIGINT UNSIGNED NULL,
  authority_digest BINARY(32) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (consumer_name,event_id),
  CONSTRAINT event_origin_receipt FOREIGN KEY (consumer_name,event_id)
    REFERENCES consumed_event(consumer_name,event_id),
  CHECK ((applied_index IS NULL AND applied_seq IS NULL AND authority_digest IS NULL) OR
    (applied_index IS NOT NULL AND applied_seq IS NOT NULL AND authority_digest IS NOT NULL)),
  CHECK (kafka_offset <= 9223372036854775807)
) ENGINE=InnoDB;

-- 同文档同时存在活跃索引和重建目标；不能用document_id单列主键覆盖另一物理代次。
CREATE TABLE index_application (
  physical_index VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  document_id BIGINT UNSIGNED NOT NULL,
  applied_seq BIGINT UNSIGNED NOT NULL,
  authority_digest BINARY(32) NOT NULL,
  release_id BIGINT UNSIGNED NULL,
  visibility VARCHAR(16) NOT NULL,
  visibility_policy VARCHAR(16) NOT NULL,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (physical_index,document_id),
  INDEX physical_progress(physical_index,visibility,document_id),
  CHECK (visibility IN ('PUBLIC','HIDDEN')),
  CHECK (visibility_policy IN ('PUBLIC','SPACE_MEMBERS','COLLABORATORS')),
  CHECK ((visibility='PUBLIC' AND release_id IS NOT NULL) OR
    (visibility='HIDDEN' AND release_id IS NULL))
) ENGINE=InnoDB;

-- next_offset只在整条记录所有行已成功后推进；实际Kafka commit仍是独立的随后动作。
CREATE TABLE consumer_checkpoint (
  consumer_name VARCHAR(40) NOT NULL,
  kafka_topic VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  partition_id INT UNSIGNED NOT NULL,
  next_offset BIGINT UNSIGNED NOT NULL,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (consumer_name,kafka_topic,partition_id),
  CHECK (next_offset <= 9223372036854775807)
) ENGINE=InnoDB;
