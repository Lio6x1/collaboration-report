-- 只归档管理列表记录；不删除邀请、申请或已授予权限，既有审批仍可追溯。
USE document_db;
CREATE TABLE document_sharing_archive (
 record_kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 record_id BIGINT UNSIGNED NOT NULL,
 document_id BIGINT UNSIGNED NOT NULL,
 actor_id BIGINT UNSIGNED NOT NULL,
 archived_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 PRIMARY KEY(record_kind,record_id),
 INDEX sharing_archive_document(document_id,record_kind,record_id),
 CONSTRAINT sharing_archive_kind CHECK(record_kind IN ('INVITATION','APPLICATION')),
 CONSTRAINT sharing_archive_document_fk FOREIGN KEY(document_id) REFERENCES document(id) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB;
