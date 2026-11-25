-- 消费进度只能属于首次绑定的物理日志；同名主题重建后不可复用旧offset。
USE search_db;

CREATE TABLE consumer_log_identity (
  consumer_name VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  kafka_topic VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  cluster_id VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  topic_id BINARY(16) NOT NULL,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (consumer_name),
  CHECK (revision=1)
) ENGINE=InnoDB;
