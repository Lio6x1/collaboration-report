import test from 'node:test';
import assert from 'node:assert/strict';
import {
  conflictedReviewFixture,
  conflictBody,
  conflictParagraph,
} from '../helpers/conflict-contexts.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { writeDraftBody } from '../helpers/review.mjs';
import { pollPull } from '../helpers/pull-main.mjs';
import {
  resolutionInput as resolution,
  withdrawConflict as withdraw,
} from '../helpers/conflict-resolutions.mjs';

test('resolution rejects changed original attempt, digest, round, conflict set and forbidden body before registering', async (t) => {
  const { f, db, main, fixed, block, id } = await conflictedReviewFixture();
  try {
    const context = json(await get(`/v1/conflict-contexts/${id}`, f.user.headers));
    await withdraw(f, fixed);
    for (const [name, status, edit] of [
      [
        'another attempt',
        409,
        (input) => {
          const attempt = BigInt(context.attempt_id.slice('merge-attempt-'.length)) + 1n;
          input.expected_attempt_id = 'merge-attempt-' + attempt.toString().padStart(20, '0');
          input.resolved_conflict_ids = input.resolved_conflict_ids.map((ref) =>
            ref.replace(
              context.attempt_id.slice('merge-attempt-'.length),
              attempt.toString().padStart(20, '0'),
            ),
          );
        },
      ],
      [
        'different digest',
        409,
        (input) => {
          input.expected_input_digest =
            context.input_digest === 'a'.repeat(64) ? 'b'.repeat(64) : 'a'.repeat(64);
        },
      ],
      [
        'different round',
        409,
        (input) => {
          input.expected_round_id = (BigInt(context.round_id) + 1n).toString();
        },
      ],
      [
        'missing full set',
        400,
        (input) => {
          input.resolved_conflict_ids = [];
        },
      ],
      [
        'duplicate conflict',
        400,
        (input) => {
          input.resolved_conflict_ids.push(input.resolved_conflict_ids[0]);
        },
      ],
      [
        'extra original ordinal',
        409,
        (input) => {
          input.resolved_conflict_ids.push(context.conflicts[0].id.replace(/0001$/, '9999'));
        },
      ],
      [
        'forbidden font',
        400,
        (input) => {
          input.resolution_content.content[0].content[0].marks = [
            { type: 'font', attrs: { family: '宋体' } },
          ];
        },
      ],
      [
        'duplicate stable identity',
        400,
        (input) => {
          input.resolution_content.content.push(conflictParagraph(block, '另一处重复身份'));
        },
      ],
      [
        'invalid document',
        400,
        (input) => {
          input.resolution_content = { type: 'doc', content: [] };
        },
      ],
    ]) {
      await t.test(name, async () => {
        const input = resolution(context, block);
        edit(input);
        const response = await post(
          `/v1/conflict-contexts/${id}/resolutions`,
          input,
          f.user.headers,
        );
        assert.equal(response.status, status, response.body);
        const [[receipt]] = await db.query(
          'SELECT COUNT(*) count FROM operation_receipt WHERE operation_id=?',
          [input.operation_id],
        );
        assert.equal(Number(receipt.count), 0, 'rejected precondition must not reserve operation');
      });
    }
    const request = json(await get(`/v1/review-requests/${fixed.request_id}`, f.user.headers));
    assert.equal(request.status, 'WITHDRAWN');
    assert.equal(request.current_round_id, fixed.round_id);
    assert.deepEqual(json(await get(`/v1/conflict-contexts/${id}`, f.user.headers)), context);
  } finally {
    main.close();
    f.close();
    await db.end();
  }
});

test('a current COMMENT reader can inspect the conflict but cannot register manual repair', async () => {
  const { f, db, main, fixed, block, id } = await conflictedReviewFixture({ members: 1 });
  try {
    const reader = f.f.members[0];
    const document = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    await waitAuthorization(
      await post(
        `/v1/documents/${f.d.id}/authorization-changes`,
        {
          operation_id: operation('resolution-comment-only'),
          expected_acl_revision: document.acl_revision,
          changes: [
            {
              action: 'GRANT',
              grant: {
                user_id: reader.id,
                scope_kind: 'DOCUMENT',
                scope_id: f.d.id,
                capability: 'COMMENT',
              },
            },
          ],
        },
        f.user.headers,
      ),
      f.user,
    );
    await withdraw(f, fixed);
    const read = await get(`/v1/conflict-contexts/${id}`, reader.headers);
    assert.equal(read.status, 200, read.body);
    const input = resolution(json(read), block);
    const response = await post(`/v1/conflict-contexts/${id}/resolutions`, input, reader.headers);
    assert.equal(response.status, 403, response.body);
    assert.equal(json(response).code, 'UNIT_EDIT_REQUIRED');
    const [[receipt]] = await db.query(
      'SELECT COUNT(*) count FROM operation_receipt WHERE operation_id=?',
      [input.operation_id],
    );
    assert.equal(Number(receipt.count), 0);
  } finally {
    main.close();
    f.close();
    await db.end();
  }
});

test('overlapping reliable S2 rejects the repair and releases its own control without replacing the draft', async () => {
  const { f, db, main, fixed, block, id } = await conflictedReviewFixture();
  try {
    const context = json(await get(`/v1/conflict-contexts/${id}`, f.user.headers));
    await withdraw(f, fixed);
    await writeDraftBody(f, conflictBody(conflictParagraph(block, '同处已经可靠保存的另一份S2')));
    const [[before]] = await db.query(
      'SELECT CAST(workspace_epoch AS CHAR) epoch,CAST(current_lineage_id AS CHAR) lineage FROM branch WHERE id=?',
      [f.session.branch_id],
    );
    const input = resolution(context, block);
    const response = await post(`/v1/conflict-contexts/${id}/resolutions`, input, f.user.headers);
    assert.equal(response.status, 202, response.body);
    const result = await pollPull(input.operation_id, f.user.headers);
    assert.equal(result.state, 'REJECTED', JSON.stringify(result));
    assert.equal(result.error.code, 'RESOLUTION_CONFLICT');
    const [[after]] = await db.query(
      'SELECT CAST(workspace_epoch AS CHAR) epoch,CAST(current_lineage_id AS CHAR) lineage,lifecycle FROM branch WHERE id=?',
      [f.session.branch_id],
    );
    assert.equal(after.epoch, before.epoch);
    assert.equal(after.lineage, before.lineage);
    assert.equal(after.lifecycle, 'ACTIVE');
    const [[gate]] = await db.query(
      "SELECT JSON_UNQUOTE(JSON_EXTRACT(payload,'$.operation_id')) owner FROM durable_task WHERE kind='BRANCH_CONTROL_GATE' AND business_key=?",
      [f.session.branch_id],
    );
    assert.equal(gate.owner, '');
    // 原房间仍可可靠保存，证明拒绝没有留下控制占位或覆写竞争输入。
    await writeDraftBody(
      f,
      conflictBody(
        conflictParagraph(block, '同处已经可靠保存的另一份S2'),
        conflictParagraph('after-rejected-repair', '拒绝后仍可保存'),
      ),
    );
  } finally {
    main.close();
    f.close();
    await db.end();
  }
});

test('invalid proposed table geometry terminates the original repair and releases only its own control', async () => {
  const { f, db, main, fixed, block, id } = await conflictedReviewFixture();
  try {
    const context = json(await get(`/v1/conflict-contexts/${id}`, f.user.headers));
    await withdraw(f, fixed);
    const input = resolution(context, block);
    // Schema允许跨行，但不存在第二行：这是人工正文的确定几何错误，不是坏固定来源或网络未知。
    input.resolution_content = conflictBody({
      type: 'table',
      attrs: { id: 'invalid-table' },
      content: [
        {
          type: 'table_row',
          attrs: { id: 'invalid-row' },
          content: [
            {
              type: 'table_cell',
              attrs: { id: 'invalid-cell', colspan: 1, rowspan: 2, colwidth: null },
              content: [conflictParagraph('inside-invalid-table', '格')],
            },
          ],
        },
      ],
    });
    const registered = await post(`/v1/conflict-contexts/${id}/resolutions`, input, f.user.headers);
    assert.equal(registered.status, 202, registered.body);
    const result = await pollPull(input.operation_id, f.user.headers);
    assert.equal(result.state, 'REJECTED', JSON.stringify(result));
    assert.equal(result.error.code, 'RESOLUTION_CONTENT_INVALID');
    const [[branch]] = await db.query(
      "SELECT lifecycle,CAST(workspace_epoch AS CHAR) epoch,JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.operation_id')) owner FROM branch b JOIN durable_task t ON t.kind='BRANCH_CONTROL_GATE' AND t.business_key=CAST(b.id AS CHAR) WHERE b.id=?",
      [f.session.branch_id],
    );
    assert.equal(branch.lifecycle, 'ACTIVE');
    assert.equal(branch.epoch, f.session.workspace_epoch);
    assert.equal(branch.owner, '');
    await writeDraftBody(f, conflictBody(conflictParagraph(block, '原计算的固定提交S1')));
    assert.deepEqual(json(await get(`/v1/conflict-contexts/${id}`, f.user.headers)), context);
  } finally {
    main.close();
    f.close();
    await db.end();
  }
});
