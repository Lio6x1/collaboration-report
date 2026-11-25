import test from 'node:test';
import assert from 'node:assert/strict';
import { mergedReverseFixture, pollReverse } from '../helpers/reverse-changes.mjs';
import {
  reviewDatabase,
  collaborationRPC,
  openReviewWorkspace,
  writeDraftBody,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import { persistedPull } from '../helpers/pull-main.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';

const paragraph = (id, text) => ({
  type: 'paragraph',
  attrs: { id },
  content: [{ type: 'text', text }],
});
const doc = (...content) => ({ type: 'doc', content });

for (const mode of ['DRAFT', 'MAIN']) {
  test(
    `reverse rejects ${mode} changes after its fixed candidate without sampling a replacement`,
    { timeout: 90000 },
    async () => {
      const { f, db, record, originalBlockId } = await mergedReverseFixture(),
        holder = await reviewDatabase(),
        peer = collaborationRPC();
      let held = false,
        main;
      try {
        if (mode !== 'DRAFT') await unlockStructurePreview(f);
        if (mode === 'MAIN') main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
        const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers)),
          id = operation('reverse-fixed-' + mode.toLowerCase()),
          input = {
            operation_id: id,
            expected_manifest_id: current.current_manifest_id,
            merge_record_id: String(record.id),
            reason: '原固定候选之后的新输入不能被覆盖',
          };
        // 先持有实际恢复行，再等202；READ COMMITTED不阻挡登记插入，也不改写任务状态或generation。
        await holder.query('SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED');
        await holder.beginTransaction();
        held = true;
        const accepted = post(`/v1/documents/${f.d.id}/reverse-changes`, input, f.user.headers);
        let task;
        const registration = Date.now() + 10000;
        do {
          [[task]] = await holder.query(
            "SELECT state FROM durable_task WHERE kind='REVERSE_RECOVER' AND business_key=? FOR UPDATE",
            [id],
          );
          if (task) break;
          await new Promise((resolve) => setTimeout(resolve, 1));
        } while (Date.now() < registration);
        assert.ok(task, 'actual original recovery row must be registered');
        assert.ok(['QUEUED', 'RETRY'].includes(task.state), JSON.stringify(task));
        const response = await accepted;
        assert.equal(response.status, 202, response.body);
        const [[row]] = await holder.query(
          "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='REVERSE_CONTEXT' AND business_key=?",
          [id],
        );
        const fixed = persistedPull(row.raw),
          request = {
            context: {
              ...fixed.Context,
              request_fingerprint: Buffer.from(fixed.Context.request_fingerprint, 'base64'),
            },
            expected_branch_id: fixed.Scope.Workspace.Workspace,
            expected_structure_id: fixed.Scope.Structure,
            expected_lineage_id: fixed.Scope.Workspace.Lineage,
          };
        const first = await rpcCall(peer, 'CaptureDraftPreview', request);
        assert.equal(first.state, 'FIXED');
        if (mode === 'DRAFT')
          await writeDraftBody(
            f,
            doc(
              paragraph(originalBlockId, '这项已经合并的修改需要反向'),
              paragraph('reverse-after-fixed-s2', '候选之后可靠保存的S2'),
            ),
          );
        else if (mode === 'MAIN')
          await writeDraftBody(
            main,
            doc(
              paragraph(originalBlockId, '这项已经合并的修改需要反向'),
              paragraph('reverse-after-fixed-main', '候选之后可靠保存的MAIN新增'),
            ),
          );
        assert.deepEqual(await rpcCall(peer, 'CaptureDraftPreview', request), first);
        await holder.commit();
        held = false;
        const result = await pollReverse(id, f.user.headers);
        assert.equal(result.state, 'REJECTED', JSON.stringify(result));
        assert.equal(result.error.code, mode === 'DRAFT' ? 'DRAFT_CHANGED' : 'MAIN_CHANGED');
        const [[saved]] = await db.query(
          "SELECT payload FROM durable_task WHERE kind='REVERSE_FIXED' AND business_key=?",
          [id],
        );
        const value = typeof saved.payload === 'string' ? JSON.parse(saved.payload) : saved.payload;
        assert.equal(value.input_digest, Buffer.from(first.input_digest).toString('base64'));
        assert.doesNotMatch(
          Buffer.from(value.draft.body_json_utf8, 'base64').toString(),
          /候选之后可靠保存的S2/,
        );
        const [[branch]] = await db.query(
          'SELECT lifecycle,workspace_epoch FROM branch WHERE id=?',
          [f.session.branch_id],
        );
        assert.equal(branch.lifecycle, 'ACTIVE');
        assert.equal(String(branch.workspace_epoch), f.session.workspace_epoch);
        const [[rounds]] = await db.query(
          'SELECT COUNT(*) n FROM review_round WHERE submission_operation_id=?',
          [id],
        );
        assert.equal(Number(rounds.n), 0);
        assert.equal(
          json(await get(`/v1/documents/${f.d.id}`, f.user.headers)).current_manifest_id,
          current.current_manifest_id,
        );
        // 原代仍可继续可靠保存，拒绝没有安装新代或替换新尾部。
        await writeDraftBody(
          f,
          doc(
            paragraph(originalBlockId, '原操作拒绝后继续保存'),
            paragraph('reverse-kept-s2', '保留候选之后的可靠输入'),
          ),
        );
      } finally {
        if (held) await holder.rollback();
        main?.close();
        peer.close();
        f.close();
        await holder.end();
        await db.end();
      }
    },
  );
}
