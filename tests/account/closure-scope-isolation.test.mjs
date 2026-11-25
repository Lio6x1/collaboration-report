import test from 'node:test';
import assert from 'node:assert/strict';
import { createUser, get, post, json, operation } from '../helpers/account.mjs';
import { reviewFixture, reviewDatabase, flushDraft } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';

async function awaitOriginal(db, id, budget = 8000) {
  const deadline = Date.now() + budget;
  while (Date.now() < deadline) {
    const [[row]] = await db.query('SELECT state FROM operation_receipt WHERE operation_id=?', [
      id,
    ]);
    if (row?.state === 'SUCCEEDED') return;
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  assert.fail('ORIGINAL_CLOSURE_BLOCKED_BY_UNRELATED_HISTORY');
}

for (const role of ['document', 'collaboration']) {
  test(
    'actual ' + role + ' unrelated closed history row cannot block a different session closure',
    { timeout: 60000 },
    async () => {
      const f = await reviewFixture(),
        other = await createUser();
      const db = await reviewDatabase(role),
        account = await reviewDatabase('account');
      let locked = false;
      try {
        let historical;
        if (role === 'document') {
          await unlockStructurePreview(f);
          const response = await get('/v1/documents/' + f.d.id + '/main', f.user.headers);
          assert.equal(response.status, 200, json(response).code);
          historical = json(response).read_view_id;
        } else historical = (await flushDraft(f)).receipt_id;
        assert.ok(historical);
        const first = operation('scope-history-logout');
        assert.equal(
          (await post('/v1/account/logout', { operation_id: first }, f.user.headers)).status,
          202,
        );
        await awaitOriginal(account, first);
        const table = role === 'document' ? 'read_view' : 'flush_receipt';
        const key = role === 'document' ? 'view_id' : 'receipt_id';
        const [[before]] = await db.query('SELECT state FROM ' + table + ' WHERE ' + key + '=?', [
          historical,
        ]);
        assert.equal(before.state, 'CLOSED');
        // 锁住真实已关闭的另一范围历史；不能通过无关全库UPDATE让新原号等待此行。
        await db.beginTransaction();
        locked = true;
        await db.query('SELECT state FROM ' + table + ' WHERE ' + key + '=? FOR UPDATE', [
          historical,
        ]);
        const second = operation('scope-independent-logout');
        assert.equal(
          (await post('/v1/account/logout', { operation_id: second }, other.headers)).status,
          202,
        );
        await awaitOriginal(account, second, 3000);
        assert.equal((await get('/v1/account/me', other.headers)).status, 401);
        const [[after]] = await db.query('SELECT state FROM ' + table + ' WHERE ' + key + '=?', [
          historical,
        ]);
        assert.equal(after.state, 'CLOSED');
      } finally {
        if (locked) await db.rollback();
        f.close();
        await db.end();
        await account.end();
      }
    },
  );
}
