-- 原资源归属保持不可变；计费空间及精确文档转让资格使用独立事实。
USE document_db;

CREATE TABLE personal_transfer (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
 document_id BIGINT UNSIGNED NOT NULL,
 document_title VARCHAR(240) NOT NULL,
 source_user_id BIGINT UNSIGNED NOT NULL,
 target_user_id BIGINT UNSIGNED NOT NULL,
 source_space_id BIGINT UNSIGNED NOT NULL,
 expected_control_revision BIGINT UNSIGNED NOT NULL,
 revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
 state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 offer_operation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 accept_operation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
 fixed_plan MEDIUMBLOB NULL,
 expires_at DATETIME(6) NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 PRIMARY KEY(id), UNIQUE KEY transfer_offer(offer_operation_id), UNIQUE KEY transfer_accept(accept_operation_id),
 INDEX transfer_document(document_id,state), INDEX transfer_incoming(target_user_id,id), INDEX transfer_outgoing(source_user_id,id),
 CHECK (state IN ('PENDING','ACCEPTING','ACCEPTED','CANCELLED','REJECTED','EXPIRED'))
) ENGINE=InnoDB;

CREATE TABLE resource_space_allocation (
 resource_id BIGINT UNSIGNED NOT NULL,
 space_id BIGINT UNSIGNED NOT NULL,
 bytes BIGINT UNSIGNED NOT NULL,
 state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 operation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 PRIMARY KEY(resource_id,space_id), INDEX allocation_space(space_id,state,resource_id),
 CHECK (state IN ('ACTIVE','RELEASED'))
) ENGINE=InnoDB;

INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id)
 SELECT r.id,r.space_id,r.byte_size,'ACTIVE',r.upload_operation_id FROM resource r
 JOIN quota_reservation q ON q.operation_id=r.upload_operation_id AND q.state='COMMITTED'
 WHERE r.state IN ('READY','DELETING');

CREATE TABLE document_resource_allocation (
 resource_id BIGINT UNSIGNED NOT NULL,
 document_id BIGINT UNSIGNED NOT NULL,
 space_id BIGINT UNSIGNED NOT NULL,
 transfer_id BIGINT UNSIGNED NOT NULL,
 state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 PRIMARY KEY(resource_id,document_id,space_id), INDEX document_allocation(document_id,space_id,state),
 CHECK (state IN ('ACTIVE','RELEASED'))
) ENGINE=InnoDB;

CREATE TABLE document_capacity (
 document_id BIGINT UNSIGNED NOT NULL,
 space_id BIGINT UNSIGNED NOT NULL,
 bytes BIGINT UNSIGNED NOT NULL,
 node_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
 operation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 PRIMARY KEY(document_id), INDEX capacity_space(space_id,document_id)
) ENGINE=InnoDB;

-- 历史 canonical 正文以前未计入 quota；按真实字节补账，即使结果已经超额。
-- 运行时依旧允许查看/释放这份真实账目，只拒绝新增；绝不自动抬高 limit。
INSERT INTO document_capacity(document_id,space_id,bytes,node_bytes,operation_id)
 SELECT d.id,d.space_id,SUM(v.byte_size),0,'migration-010-body-capacity'
 FROM document d JOIN content_version v ON v.document_id=d.id
 WHERE d.lifecycle<>'PURGED' GROUP BY d.id,d.space_id;

UPDATE quota_account q JOIN (
 SELECT space_id,SUM(bytes) bytes FROM document_capacity GROUP BY space_id
) c ON c.space_id=q.space_id
 SET q.used_bytes=q.used_bytes+c.bytes,q.revision=q.revision+1;
