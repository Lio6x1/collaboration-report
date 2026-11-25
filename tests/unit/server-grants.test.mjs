import test from 'node:test';
import assert from 'node:assert/strict';
import { canonicalGrant } from '../../scripts/server-grants.mjs';

// 漂移不能因SHOW GRANTS排序/反引号表现差异而误报，也不能把转授权能力抹掉。
test('server grant verification preserves the exact privilege boundary', () => {
  assert.equal(
    canonicalGrant(
      "GRANT UPDATE, SELECT ON `account_db`.`schema_migration` TO 'collab_account'@'%'",
    ),
    canonicalGrant("GRANT SELECT,UPDATE ON account_db.schema_migration TO 'collab_account'@'%'"),
  );
  for (const statement of [
    "GRANT SELECT ON account_db.schema_migration TO 'collab_account'@'%' WITH GRANT OPTION",
    "GRANT USAGE ON *.* TO 'collab_account'@'%' WITH GRANT OPTION",
  ]) {
    assert.throws(() => canonicalGrant(statement), /SERVER_ACCOUNT_GRANT_OPTION_NOT_ALLOWED/);
  }
});
