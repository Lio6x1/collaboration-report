import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { reviewFixture, reviewDatabase, writeDraft } from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import {
  discardPreviewPath,
  discardPreviewInput,
  pollDiscardPreview,
} from '../helpers/discard-previews.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

const digest = (raw) => createHash('sha256').update(raw).digest('hex');

test(
  'discard preview resumes expired original proof after owner restart without another POST',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture(),
      database = await reviewDatabase();
    let stopped = false;
    try {
      await writeDraft(f, '自然31秒之后仍是原可靠草稿');
      const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      const input = discardPreviewInput(f);
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      const pending = await post(discardPreviewPath(f), input, f.user.headers);
      assert.equal(pending.status, 503, pending.body);
      assert.equal(json(pending).code, 'DISCARD_PREVIEW_PREPARING');
      const [[original]] = await database.query(
        "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='DISCARD_PREVIEW_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      const fixed = JSON.parse(original.raw);
      await new Promise((resolve) => setTimeout(resolve, 31000));
      assert.ok(fixed.Access.Proofs.every((proof) => Number(proof.expires_unix_ms) < Date.now()));
      const progress = await get(`/v1/operations/${input.operation_id}`, f.user.headers);
      assert.equal(progress.status, 200, progress.body);
      assert.equal(json(progress).state, 'PREPARING');
      restartOwnedRuntime();
      stopped = false;
      assert.equal(
        (await pollDiscardPreview(input.operation_id, f.user.headers)).state,
        'SUCCEEDED',
      );
      const [[same]] = await database.query(
        "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='DISCARD_PREVIEW_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      assert.equal(digest(same.raw), digest(original.raw));
      const [[result]] = await database.query(
        'SELECT result_json FROM operation_receipt WHERE operation_id=?',
        [input.operation_id],
      );
      const preview =
        typeof result.result_json === 'string'
          ? JSON.parse(result.result_json)
          : result.result_json;
      const [[version]] = await database.query(
        'SELECT canonical_body,source_kind FROM content_version WHERE id=? AND document_id=?',
        [preview.draft_content_version_id, f.d.id],
      );
      assert.equal(version.source_kind, 'RESTORE');
      assert.match(version.canonical_body.toString(), /自然31秒之后仍是原可靠草稿/);
      const after = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      for (const field of ['current_manifest_id', 'current_structure_id', 'control_revision'])
        assert.equal(after[field], before[field]);
    } finally {
      f.close();
      await database.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);

test(
  'logout explicitly rejects original pending discard preview without creating a draft version',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture(),
      database = await reviewDatabase();
    let stopped = false;
    try {
      await writeDraft(f, '撤权之前的可靠旧稿');
      const input = discardPreviewInput(f);
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      const pending = await post(discardPreviewPath(f), input, f.user.headers);
      assert.equal(pending.status, 503, pending.body);
      const loggedOut = await post(
        '/v1/account/logout',
        { operation_id: operation('discard-preview-logout') },
        f.user.headers,
      );
      assert.ok([200, 202].includes(loggedOut.status), loggedOut.body);
      restartOwnedRuntime();
      stopped = false;
      let fact;
      const deadline = Date.now() + 20000;
      do {
        [[fact]] = await database.query(
          'SELECT state,result_json FROM operation_receipt WHERE operation_id=?',
          [input.operation_id],
        );
        if (fact.state !== 'PREPARING') break;
        await new Promise((resolve) => setTimeout(resolve, 100));
      } while (Date.now() < deadline);
      assert.equal(fact.state, 'REJECTED');
      const result =
        typeof fact.result_json === 'string' ? JSON.parse(fact.result_json) : fact.result_json;
      assert.match(result.error.code, /IDENTITY|SESSION|SOURCE|ADMISSION/);
      const [[count]] = await database.query(
        'SELECT COUNT(*) total FROM content_version WHERE source_operation_id=?',
        [input.operation_id],
      );
      assert.equal(Number(count.total), 0);
      const [[branch]] = await database.query(
        'SELECT lifecycle,workspace_epoch FROM branch WHERE id=?',
        [f.session.branch_id],
      );
      assert.equal(branch.lifecycle, 'ACTIVE');
      assert.equal(String(branch.workspace_epoch), f.session.workspace_epoch);
      assert.equal((await get(`/v1/operations/${input.operation_id}`, f.user.headers)).status, 401);
    } finally {
      f.close();
      await database.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
