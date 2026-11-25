import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';

async function originalContext(db, id) {
  const [[row]] = await db.query(
    "SELECT payload FROM durable_task WHERE kind='PUBLICATION_CONTEXT' AND business_key=?",
    [id],
  );
  assert.ok(row, '原发布管理上下文必须持久存在');
  const value = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
  assert.equal(
    /"Token"|"token"|"cookie"/.test(JSON.stringify(value)),
    false,
    '持久恢复不保存可再次登录的会话Token',
  );
  return value;
}

// 等真实30秒准入经过，再重启本owner；恢复只能续接原号、原会话和原来源，不能重新POST取得新权限。
test(
  'original publication resumes after natural identity expiry and owned restart',
  { timeout: 180000 },
  async () => {
    const f = await organizationFixture(0),
      d = await documentFixture(f),
      db = await reviewDatabase();
    let stopped = false;
    try {
      const input = await releaseInput(d.id, f.owner);
      stopOwnedRole('collaboration');
      stopped = true;
      const made = await post(`/v1/documents/${d.id}/releases`, input, f.owner.headers);
      assert.equal(made.status, 202, json(made).code);
      const original = await originalContext(db, input.operation_id);
      await new Promise((resolve) => setTimeout(resolve, 31000));
      assert.ok(original.access.Proofs.every((proof) => proof.expires_unix_ms < Date.now()));
      const pending = await get(`/v1/operations/${input.operation_id}`, f.owner.headers);
      assert.equal(pending.status, 200, json(pending).code);
      assert.equal(json(pending).state, 'PREPARING');
      restartOwnedRuntime();
      stopped = false;
      const result = await pollPublication(input.operation_id, f.owner);
      assert.equal(result.state, 'SUCCEEDED', result.error?.code);
      assert.equal(
        JSON.stringify(await originalContext(db, input.operation_id)) === JSON.stringify(original),
        true,
        '原绑定不能被续期的新证明改写',
      );
      const [[counts]] = await db.query(
        'SELECT (SELECT COUNT(*) FROM `release` WHERE operation_id=?) releases,(SELECT COUNT(*) FROM outbox WHERE event_id=?) events',
        [input.operation_id, input.operation_id],
      );
      assert.equal(Number(counts.releases), 1);
      assert.equal(Number(counts.events), 1);
    } finally {
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);

// Node不可达不能把正式业务部分提交；权威MAIN/title已改变时原预约必须明确拒绝，不能无限重试旧来源。
test(
  'publication rejects changed authoritative title and control while Node is unavailable',
  { timeout: 180000 },
  async () => {
    const f = await organizationFixture(0),
      d = await documentFixture(f),
      db = await reviewDatabase();
    let stopped = false;
    try {
      const input = await releaseInput(d.id, f.owner);
      stopOwnedRole('collaboration');
      stopped = true;
      const made = await post(`/v1/documents/${d.id}/releases`, input, f.owner.headers);
      assert.equal(made.status, 202, json(made).code);
      const renamed = await post(
        `/v1/documents/${d.id}`,
        {
          operation_id: operation('publication-rename'),
          expected_control_revision: input.expected_control_revision,
          title: '后来权威标题',
        },
        f.owner.headers,
        'PATCH',
      );
      assert.equal(renamed.status, 200, json(renamed).code);
      const result = await pollPublication(input.operation_id, f.owner);
      assert.equal(result.state, 'REJECTED');
      assert.ok(['MAIN_CONTROL_CHANGED', 'MAIN_SOURCE_CHANGED'].includes(result.error.code));
      const [[counts]] = await db.query(
        'SELECT (SELECT COUNT(*) FROM `release` WHERE operation_id=?) releases,(SELECT COUNT(*) FROM outbox WHERE event_id=?) events',
        [input.operation_id, input.operation_id],
      );
      assert.equal(Number(counts.releases), 0);
      assert.equal(Number(counts.events), 0);
    } finally {
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);

// 业务已经提交仍须等待真实旧来源关闭；实际Node退出与恢复不重复发布，也不把期限未知冒充完成。
test(
  'committed publication stays pending through blocked closure and owned Node exit',
  { timeout: 180000 },
  async () => {
    const f = await organizationFixture(0),
      d = await documentFixture(f),
      db = await reviewDatabase(),
      node = await reviewDatabase('collaboration');
    let locked = false,
      stopped = false;
    try {
      await publishRelease(d.id, f.owner, await releaseInput(d.id, f.owner));
      const input = await releaseInput(d.id, f.owner);
      await node.beginTransaction();
      locked = true;
      await node.query('SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE', [
        `publication:${d.id}`,
      ]);
      const made = await post(`/v1/documents/${d.id}/releases`, input, f.owner.headers);
      assert.equal(made.status, 202, json(made).code);
      const deadline = Date.now() + 10000;
      let committed = false;
      while (Date.now() < deadline) {
        const [[row]] = await db.query('SELECT COUNT(*) n FROM `release` WHERE operation_id=?', [
          input.operation_id,
        ]);
        if (Number(row.n) === 1) {
          committed = true;
          break;
        }
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
      assert.equal(committed, true, '关闭未知前必须观察真实已提交正式版');
      const pending = await get(`/v1/operations/${input.operation_id}`, f.owner.headers);
      assert.equal(pending.status, 200, json(pending).code);
      assert.equal(json(pending).state, 'PREPARING');
      const [[closure]] = await db.query(
        'SELECT state FROM authorization_change WHERE revocation_id=?',
        [input.operation_id],
      );
      assert.equal(closure.state, 'FENCING');
      stopOwnedRole('collaboration');
      stopped = true;
      await node.rollback();
      locked = false;
      restartOwnedRuntime();
      stopped = false;
      assert.equal((await pollPublication(input.operation_id, f.owner)).state, 'SUCCEEDED');
      const [[counts]] = await db.query(
        'SELECT (SELECT COUNT(*) FROM `release` WHERE operation_id=?) releases,(SELECT COUNT(*) FROM outbox WHERE event_id=?) events',
        [input.operation_id, input.operation_id],
      );
      assert.equal(Number(counts.releases), 1);
      assert.equal(Number(counts.events), 1);
    } finally {
      if (locked) await node.rollback();
      await node.end();
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
