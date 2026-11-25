-- 正文预留与阅读副本分账。旧迁移保持原字节，运行时仍按原上限拒绝新增。
USE document_db;

CREATE TABLE draft_body_capacity (
 room_id BIGINT UNSIGNED NOT NULL,
 workspace_epoch BIGINT UNSIGNED NOT NULL,
 document_id BIGINT UNSIGNED NOT NULL,
 space_id BIGINT UNSIGNED NOT NULL,
 bytes BIGINT UNSIGNED NOT NULL,
 PRIMARY KEY(room_id,workspace_epoch), INDEX draft_capacity_document(document_id,room_id,workspace_epoch)
) ENGINE=InnoDB;

CREATE TABLE draft_body_reservation (
 operation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 document_id BIGINT UNSIGNED NOT NULL,
 space_id BIGINT UNSIGNED NOT NULL,
 room_id BIGINT UNSIGNED NOT NULL,
 workspace_epoch BIGINT UNSIGNED NOT NULL,
 body_bytes BIGINT UNSIGNED NOT NULL,
 body_hash BINARY(32) NOT NULL,
 bytes BIGINT UNSIGNED NOT NULL,
 state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 PRIMARY KEY(operation_id), INDEX draft_reservation_document(document_id,state,operation_id),
 CHECK(state IN ('RESERVED','COMMITTED','RELEASED'))
) ENGINE=InnoDB;

CREATE TABLE reading_body_capacity (
 version_id BIGINT UNSIGNED NOT NULL,
 document_id BIGINT UNSIGNED NOT NULL,
 bytes BIGINT UNSIGNED NOT NULL,
 PRIMARY KEY(version_id), INDEX reading_capacity_document(document_id,version_id),
 FOREIGN KEY(version_id) REFERENCES content_version(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- 仅纠正有实际 READ_VIEW manifest 且没有业务 manifest 引用的版本费用。
-- 正文和固定阅读历史不改写；这份派生副本继续登记实际运行容量。
INSERT INTO reading_body_capacity(version_id,document_id,bytes)
 SELECT v.id,v.document_id,v.byte_size FROM content_version v
 WHERE EXISTS(SELECT 1 FROM manifest_entry e JOIN main_manifest m ON m.id=e.manifest_id
   WHERE e.content_version_id=v.id AND m.product_key='READ_VIEW')
 AND NOT EXISTS(SELECT 1 FROM manifest_entry e JOIN main_manifest m ON m.id=e.manifest_id
   WHERE e.content_version_id=v.id AND m.product_key<>'READ_VIEW');

UPDATE quota_account q JOIN (
 SELECT d.space_id,SUM(r.bytes) bytes FROM reading_body_capacity r
 JOIN document_capacity d ON d.document_id=r.document_id GROUP BY d.space_id
) c ON c.space_id=q.space_id
 SET q.used_bytes=q.used_bytes-c.bytes,q.revision=q.revision+1;

UPDATE document_capacity d JOIN (
 SELECT document_id,SUM(bytes) bytes FROM reading_body_capacity GROUP BY document_id
) c ON c.document_id=d.document_id SET d.bytes=d.bytes-c.bytes;
