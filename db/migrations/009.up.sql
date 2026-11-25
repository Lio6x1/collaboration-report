-- 平台留存治理资格属于Account权威事实，不能借用组织ADMIN或写入Doc私有库。
-- 撤权先暂停此资格，再沿原来源关闭任务取得真实ACK；重新授予使用更高代次。
USE account_db;

CREATE TABLE platform_governance_grant (
  user_id BIGINT UNSIGNED NOT NULL,
  epoch BIGINT UNSIGNED NOT NULL DEFAULT 1,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'CLOSED',
  changed_operation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(user_id),
  CHECK (epoch>0 AND revision>0),
  CHECK (state IN ('ACTIVE','REVOKING','CLOSED'))
) ENGINE=InnoDB;
