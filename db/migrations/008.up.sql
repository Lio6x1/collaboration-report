-- 导出任务的权威来源/固定依赖与临时制品单独记账，不占用用户资源额度。
-- 原export_job定义不变；原号、实际文件校验与取消排空由这一实施扩展持久保存。
USE document_db;

CREATE TABLE export_control (
  job_id BIGINT UNSIGNED NOT NULL,
  fingerprint BINARY(32) NOT NULL,
  snapshot MEDIUMBLOB NOT NULL,
  reserved_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
  artifact_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  artifact_size BIGINT UNSIGNED NOT NULL DEFAULT 0,
  artifact_checksum BINARY(32) NULL,
  expires_at DATETIME(6) NOT NULL,
  PRIMARY KEY(job_id),
  INDEX export_control_expiry(expires_at,job_id)
) ENGINE=InnoDB;

CREATE TABLE export_budget (
  id INT UNSIGNED NOT NULL,
  reserved_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
  used_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY(id),
  CHECK (id=1),
  CHECK (reserved_bytes<=268435456 AND used_bytes<=268435456)
) ENGINE=InnoDB;
