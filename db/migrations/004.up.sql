-- 004追加重建的原输入和恢复进度，不修改001～003，不跨服务库引用。
USE document_db;

CREATE TABLE search_snapshot_job (
  operation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  prepare_digest BINARY(32) NOT NULL,
  prepare_envelope MEDIUMBLOB NOT NULL,
  state VARCHAR(16) NOT NULL,
  entry_count BIGINT UNSIGNED NULL,
  entries_digest BINARY(32) NULL,
  seal_envelope MEDIUMBLOB NULL,
  signer_digest BINARY(32) NOT NULL,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (operation_id),
  CHECK (state IN ('WRITING','SEALED')),
  CHECK ((state='WRITING' AND entry_count IS NULL AND entries_digest IS NULL AND seal_envelope IS NULL) OR
    (state='SEALED' AND entry_count IS NOT NULL AND entries_digest IS NOT NULL AND seal_envelope IS NOT NULL))
) ENGINE=InnoDB;

-- entry只引用本库快照任务，绝不引用可变document行；删除业务文档不能改写已封存视图。
CREATE TABLE search_snapshot_entry (
  operation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  document_id BIGINT UNSIGNED NOT NULL,
  minimum_seq BIGINT UNSIGNED NOT NULL,
  release_hint BIGINT UNSIGNED NULL,
  PRIMARY KEY (operation_id,document_id),
  CONSTRAINT snapshot_entry_job FOREIGN KEY (operation_id) REFERENCES search_snapshot_job(operation_id),
  CHECK (document_id>0),
  CHECK (release_hint IS NULL OR release_hint>0)
) ENGINE=InnoDB;

USE search_db;

CREATE TABLE rebuild_binding (
  operation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  alias_name VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_index VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_generation BIGINT UNSIGNED NOT NULL,
  source_revision BIGINT UNSIGNED NOT NULL,
  target_index VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_generation BIGINT UNSIGNED NOT NULL,
  prepare_digest BINARY(32) NOT NULL,
  prepare_signer_digest BINARY(32) NOT NULL,
  prepare_envelope MEDIUMBLOB NOT NULL,
  state VARCHAR(16) NOT NULL,
  snapshot_digest BINARY(32) NULL,
  snapshot_envelope MEDIUMBLOB NULL,
  snapshot_count BIGINT UNSIGNED NULL,
  entries_digest BINARY(32) NULL,
  signer_digest BINARY(32) NULL,
  request_fingerprint BINARY(32) NULL,
  job_id BIGINT UNSIGNED NULL,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (operation_id),
  UNIQUE KEY rebuild_target (target_index),
  UNIQUE KEY rebuild_bound_job (job_id),
  INDEX rebuild_binding_due (state,operation_id),
  CONSTRAINT rebuild_binding_job FOREIGN KEY (job_id) REFERENCES rebuild_job(id),
  CHECK (state IN ('PREPARED','IMPORTED','ACCEPTED','REJECTED')),
  CHECK (source_generation>0 AND source_revision>0 AND target_generation>source_generation),
  CHECK ((state='PREPARED' AND snapshot_digest IS NULL AND snapshot_envelope IS NULL AND snapshot_count IS NULL AND entries_digest IS NULL AND signer_digest IS NULL) OR
    (state IN ('IMPORTED','ACCEPTED') AND snapshot_digest IS NOT NULL AND snapshot_envelope IS NOT NULL AND snapshot_count IS NOT NULL AND entries_digest IS NOT NULL AND signer_digest IS NOT NULL) OR state='REJECTED'),
  CHECK ((state='ACCEPTED' AND request_fingerprint IS NOT NULL AND job_id IS NOT NULL) OR
    (state<>'ACCEPTED' AND request_fingerprint IS NULL AND job_id IS NULL))
) ENGINE=InnoDB;

-- next_offset唯一权威仍是003的consumer_checkpoint，不在此双写另一套next。
CREATE TABLE rebuild_progress (
  job_id BIGINT UNSIGNED NOT NULL,
  snapshot_after BIGINT UNSIGNED NOT NULL DEFAULT 0,
  validation_after BIGINT UNSIGNED NOT NULL DEFAULT 0,
  frontier_payload MEDIUMBLOB NULL,
  validation_digest BINARY(32) NULL,
  worker_nonce BINARY(32) NULL,
  worker_instance VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NULL,
  lease_until DATETIME(6) NULL,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  PRIMARY KEY (job_id),
  CONSTRAINT rebuild_progress_job FOREIGN KEY (job_id) REFERENCES rebuild_job(id),
  CHECK (validation_digest IS NULL OR frontier_payload IS NOT NULL),
  CHECK ((worker_nonce IS NULL AND worker_instance IS NULL AND lease_until IS NULL) OR
    (worker_nonce IS NOT NULL AND worker_instance IS NOT NULL AND lease_until IS NOT NULL))
) ENGINE=InnoDB;
