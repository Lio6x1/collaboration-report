import assert from 'node:assert/strict';
import { createHash, randomBytes } from 'node:crypto';
import test from 'node:test';
import { pause } from '../../scripts/runtime-lib.mjs';
import { get, json, organizationFixture } from '../helpers/workspace.mjs';
import { searchDatabase } from '../helpers/search.mjs';
import { until } from '../helpers/authorization-fence.mjs';
import {
  checkedSearchPage,
  pageVisibility,
  publishSearchDocument,
  searchPagePath,
} from '../helpers/search-pages.mjs';

async function originalChain(database, query) {
  const [rows] = await database.execute(
    `SELECT state,CAST(UNIX_TIMESTAMP(expires_at)*1000 AS UNSIGNED) expiry
     FROM cursor_registry WHERE query_hash=?`,
    [createHash('sha256').update(query).digest()],
  );
  assert.ok(rows.length <= 1, 'PAGE_EXPIRY_CREATED_ANOTHER_CHAIN');
  return rows[0];
}

test(
  '真实HTTP页续接和重放不续期，自然原期限后外层游标拒绝且配额释放',
  { timeout: 90000 },
  async () => {
    const f = await organizationFixture(0);
    f.q = 'pageexpiry' + randomBytes(12).toString('hex');
    f.docs = [];
    await pageVisibility(`/v1/knowledge-bases/${f.kb}`, f.owner, 'PUBLIC');
    const database = await searchDatabase();
    try {
      // 两篇均由真实创建、WSS SAVED、发布及原消费者产生；不改SQL期限或注入假时钟。
      for (let index = 0; index < 2; index++) {
        f.docs.push(
          await publishSearchDocument(
            f,
            { q: f.q, title: `${f.q} 固定标题${index}`, body: `${f.q} 正式正文${index}` },
            database,
          ),
        );
      }
      const before = Date.now();
      const first = await checkedSearchPage(f, null, { limit: 1 });
      assert.equal(first.items.length, 1);
      assert.equal(first.has_more, true);
      const original = await originalChain(database, f.q);
      assert.equal(original?.state, 'OPEN');
      const expiry = Number(original.expiry);
      assert.ok(Number.isSafeInteger(expiry) && expiry > Date.now());
      assert.ok(
        expiry > before + 26000 && expiry <= Date.now() + 30000,
        'PAGE_ORIGINAL_TTL_INVALID',
      );

      const suffix = await checkedSearchPage(f, null, { limit: 1, cursor: first.next_cursor });
      assert.equal(suffix.items.length, 1);
      assert.notEqual(suffix.items[0].document_id, first.items[0].document_id);
      assert.equal(suffix.has_more, false);
      assert.deepEqual(
        await checkedSearchPage(f, null, { limit: 1, cursor: first.next_cursor }),
        suffix,
      );
      const replayed = await originalChain(database, f.q);
      assert.equal(replayed?.state, 'OPEN', 'EOF_MUST_RETAIN_ORIGINAL_CHAIN_FOR_REPLY_LOSS');
      assert.equal(replayed.expiry, original.expiry, 'REPLAY_MUST_NOT_EXTEND_ABSOLUTE_TTL');

      // 等实际SQL记录的原期限自然过去。测试清理观察预算与每个HTTP三秒预算分开；
      // 不续PIT、不重试产品失败、不删除仍OPEN/CLOSING的登记行来制造配额释放。
      await pause(Math.max(0, expiry + 100 - Date.now()));
      const expired = await get(searchPagePath(f.q, { limit: 1, cursor: first.next_cursor }));
      assert.equal(expired.status, 409);
      const error = json(expired);
      assert.equal(error.code, 'CURSOR_EXPIRED');
      assert.equal(Object.hasOwn(error, 'items'), false);
      assert.equal(Object.hasOwn(error, 'next_cursor'), false);
      await until(
        async () => {
          const remaining = await originalChain(database, f.q);
          if (!remaining) return true;
          assert.equal(remaining.expiry, original.expiry);
          return remaining.state === 'CLOSED';
        },
        'EXPIRED_PAGE_ORIGINAL_QUOTA_NOT_RELEASED',
        10000,
      );
    } finally {
      await database.end();
    }
  },
);
