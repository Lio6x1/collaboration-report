import test from 'node:test';
import assert from 'node:assert/strict';
import { fromPM } from '../../packages/content/index.mjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  reviewFixture,
  writeDraft,
  reviewDatabase,
  openReviewWorkspace,
} from '../helpers/review.mjs';
import { post, get, json, operation } from '../helpers/workspace.mjs';
import { pullInput, pullPath, pollPull, persistedPull } from '../helpers/pull-main.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

test(
  'the original pull continues after real short proof expiry and owner restart without another POST',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase();
    let stopped = false,
      next;
    try {
      await writeDraft(f, '原号拉取跨真实身份期限保持可靠草稿');
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      const input = pullInput(f, operation('recover-original-pull'));
      assert.equal((await post(pullPath(f), input, f.user.headers)).status, 202);
      await new Promise((resolve) => setTimeout(resolve, 31000));
      const pending = await get('/v1/operations/' + input.operation_id, f.user.headers);
      assert.equal(pending.status, 200);
      assert.equal(json(pending).state, 'PREPARING');
      const [[stored]] = await db.query(
        "SELECT CAST(payload AS CHAR) AS raw FROM durable_task WHERE kind='PULL_MAIN_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      const original = persistedPull(stored.raw);
      assert.ok(
        original.access.Proofs.every((proof) => Number(proof.expires_unix_ms) < Date.now()),
      );
      assert.equal(stored.raw.includes('__Host-collabSession'), false);
      restartOwnedRuntime();
      stopped = false;
      assert.equal((await pollPull(input.operation_id, f.user.headers)).state, 'SUCCEEDED');
      next = await openReviewWorkspace(f.f, f.d);
      assert.equal(BigInt(next.session.workspace_epoch), BigInt(f.session.workspace_epoch) + 1n);
      assert.match(
        JSON.stringify(fromPM(yDocToProsemirrorJSON(next.doc))),
        /原号拉取跨真实身份期限保持可靠草稿/,
      );
    } finally {
      next?.close();
      f.close();
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);

test(
  'committed pull remains WAITING while real Node installation is stopped and resumes the same lineage',
  { timeout: 180000 },
  async () => {
    let f = await reviewFixture();
    const db = await reviewDatabase(),
      node = await reviewDatabase('collaboration');
    let stopped = false,
      locked = false,
      next;
    try {
      await writeDraft(f, '支线安装中断也要保留可靠正文');
      // 两次真实拉取产生同kind左右锚点；空隙锁只覆盖INSTALLED种类，不能意外锁住预览插入。
      for (const prefix of ['install-anchor-a', 'install-anchor-c']) {
        const input = pullInput(f, operation(prefix));
        assert.equal((await post(pullPath(f), input, f.user.headers)).status, 202);
        assert.equal((await pollPull(input.operation_id, f.user.headers)).state, 'SUCCEEDED');
        const prior = f;
        f = await openReviewWorkspace(f.f, f.d);
        prior.close();
      }
      const input = pullInput(f, operation('install-anchor-b'));
      await node.beginTransaction();
      locked = true;
      await node.query(
        "SELECT business_key FROM durable_task WHERE kind='DRAFT_TRANSITION_INSTALLED' AND business_key=? FOR UPDATE",
        [input.operation_id],
      );
      assert.equal((await post(pullPath(f), input, f.user.headers)).status, 202);
      const waiting = await pollPull(input.operation_id, f.user.headers, { terminalOnly: true });
      assert.equal(waiting.state, 'WAITING');
      const lineage = waiting.result.entity_id;
      const [[receipt]] = await db.query(
        'SELECT state,SHA2(CAST(result_json AS CHAR),256) AS digest FROM operation_receipt WHERE operation_id=?',
        [input.operation_id],
      );
      assert.equal(receipt.state, 'SUCCEEDED');
      const [[branch]] = await db.query(
        'SELECT lifecycle,workspace_epoch,current_lineage_id FROM branch WHERE id=?',
        [f.session.branch_id],
      );
      assert.equal(branch.lifecycle, 'RESTORING');
      assert.equal(String(branch.current_lineage_id), lineage);
      const [[missing]] = await node.query(
        "SELECT COUNT(*) AS n FROM durable_task WHERE kind='DRAFT_TRANSITION_INSTALLED' AND business_key=?",
        [input.operation_id],
      );
      assert.equal(String(missing.n), '0');
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      await node.commit();
      locked = false;
      restartOwnedRuntime();
      stopped = false;
      const done = await pollPull(input.operation_id, f.user.headers);
      assert.equal(done.state, 'SUCCEEDED');
      assert.equal(done.result.entity_id, lineage);
      const [[after]] = await db.query(
        'SELECT SHA2(CAST(result_json AS CHAR),256) AS digest FROM operation_receipt WHERE operation_id=?',
        [input.operation_id],
      );
      assert.equal(after.digest, receipt.digest);
      next = await openReviewWorkspace(f.f, f.d);
      assert.equal(next.session.lineage_id, lineage);
      assert.equal(next.session.workspace_epoch, String(branch.workspace_epoch));
      assert.match(
        JSON.stringify(fromPM(yDocToProsemirrorJSON(next.doc))),
        /支线安装中断也要保留可靠正文/,
      );
    } finally {
      if (locked) await node.rollback();
      next?.close();
      f.close();
      await db.end();
      await node.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
