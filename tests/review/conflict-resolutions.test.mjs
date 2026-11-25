import test from 'node:test';
import assert from 'node:assert/strict';
import {
  conflictedReviewFixture,
  conflictBody,
  conflictParagraph,
} from '../helpers/conflict-contexts.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { openReviewWorkspace, writeDraftBody, submitDraft } from '../helpers/review.mjs';
import { pollPull } from '../helpers/pull-main.mjs';
import { pollMerge } from '../helpers/merge.mjs';

async function readMainBodies(f, view) {
  return Promise.all(
    view.units.map(async (unit) => {
      const response = await get(
        `/v1/documents/${f.d.id}/read-views/${view.read_view_id}/units/${unit.unit_id}`,
        f.user.headers,
      );
      assert.equal(response.status, 200, response.body);
      const content = json(response);
      assert.equal(content.id, unit.content_version_id, '正文必须属于该次实际固定阅读版本');
      return content;
    }),
  );
}

test('manual conflict repair preserves S2 and unseen MAIN additions before normal resubmission and approval', async () => {
  const { f, db, main, fixed, block, id } = await conflictedReviewFixture();
  let repaired;
  try {
    const read = await get(`/v1/conflict-contexts/${id}`, f.user.headers);
    assert.equal(read.status, 200, read.body);
    const context = json(read);
    const repairId = operation('conflict-resolution');
    const input = {
      operation_id: repairId,
      expected_attempt_id: context.attempt_id,
      expected_input_digest: context.input_digest,
      expected_round_id: fixed.round_id,
      resolution_content: conflictBody(conflictParagraph(block, '人工修复后的可审核正文')),
      resolved_conflict_ids: context.conflicts.map((value) => value.id),
    };
    const path = `/v1/conflict-contexts/${id}/resolutions`;
    // 请求仍审核中时不能把修复当作批准，必须先明确撤回。
    const reviewing = await post(path, input, f.user.headers);
    assert.equal(reviewing.status, 409, reviewing.body);
    const withdrawn = await post(
      `/v1/review-requests/${fixed.request_id}/withdrawals`,
      { operation_id: operation('repair-withdraw'), expected_round_id: fixed.round_id },
      f.user.headers,
    );
    assert.equal(withdrawn.status, 200, withdrawn.body);
    await writeDraftBody(
      f,
      conflictBody(
        conflictParagraph(block, '原计算的固定提交S1'),
        conflictParagraph('repair-later-s2', '修复页面打开后可靠保存的S2'),
      ),
    );
    // 原冲突页面没展示的后来MAIN新增，不能因修复重设基线而被解释成用户删除。
    // 放在原块之前，与S2的块后新增属于不同锚点；同一锚点两组新增应继续报INSERT_ORDER。
    await writeDraftBody(
      main,
      conflictBody(
        conflictParagraph('repair-unseen-main', '原冲突之后尚未拉入的MAIN新增'),
        conflictParagraph(block, '原计算的可靠MAIN修改'),
      ),
    );
    const beforeDocument = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const before = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    const beforeBodies = await readMainBodies(f, before);
    const registered = await post(path, input, f.user.headers);
    assert.equal(registered.status, 202, registered.body);
    const result = await pollPull(repairId, f.user.headers);
    assert.equal(result.state, 'SUCCEEDED', JSON.stringify(result));
    const replay = await post(path, input, f.user.headers);
    assert.equal(replay.status, 202, replay.body);
    assert.equal(json(replay).operation_id, repairId);
    const current = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    const currentDocument = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    // 解锁MAIN的两次可靠阅读会各自物化READ_VIEW，不能把阅读版本误当业务指针。
    // 真正不能改变的是正式主线控制事实及完整正文；旧视图也不能被新候选覆盖。
    for (const field of [
      'current_manifest_id',
      'current_structure_id',
      'current_style_id',
      'control_revision',
      'main_locked',
      'current_release_id',
      'publication_state',
    ])
      assert.deepEqual(currentDocument[field], beforeDocument[field], `修复不能改变${field}`);
    for (const field of ['structure_id', 'style_id', 'structure', 'style'])
      assert.deepEqual(current[field], before[field]);
    assert.deepEqual(
      current.units.map((unit) => unit.unit_id),
      before.units.map((unit) => unit.unit_id),
      '人工修复不能替换主线单元范围',
    );
    assert.deepEqual(
      (await readMainBodies(f, current)).map((content) => content.body_json),
      beforeBodies.map((content) => content.body_json),
      '可靠主线的整个正文保持不变，包括原冲突后尚未拉入的MAIN新增',
    );
    assert.deepEqual(await readMainBodies(f, before), beforeBodies, '原固定阅读内容仍完整不可变');
    const request = json(await get(`/v1/review-requests/${fixed.request_id}`, f.user.headers));
    assert.equal(request.status, 'WITHDRAWN');
    assert.equal(request.current_round_id, fixed.round_id);
    const [[records]] = await db.query(
      'SELECT COUNT(*) count FROM merge_record WHERE request_id=?',
      [fixed.request_id],
    );
    assert.equal(Number(records.count), 0);
    repaired = await openReviewWorkspace(f.f, f.d, f.user);
    assert.notEqual(repaired.session.workspace_epoch, f.session.workspace_epoch);
    const [lineages] = await db.query(
      'SELECT v.canonical_body FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id JOIN content_version v ON v.id=l.base_version_id WHERE b.id=?',
      [repaired.session.branch_id],
    );
    assert.equal(lineages.length, 1);
    assert.deepEqual(JSON.parse(lineages[0].canonical_body.toString()), context.main.body_json);
    const submitted = (
      await submitDraft(repaired, {
        mode: 'RESUBMIT',
        request_id: fixed.request_id,
        expected_round_id: fixed.round_id,
      })
    ).value;
    assert.equal(submitted.state, 'SUCCEEDED', JSON.stringify(submitted));
    const round = submitted.result;
    const [[version]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      round.content_version_id,
    ]);
    const body = version.canonical_body.toString();
    assert.match(body, /人工修复后的可审核正文/);
    assert.match(body, /修复页面打开后可靠保存的S2/);
    const mergeId = operation('approve-repaired-round');
    const approved = await post(
      `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
      {
        operation_id: mergeId,
        round_id: round.round_id,
        content_version_id: round.content_version_id,
        own_changes_to_resolve: [],
      },
      f.user.headers,
    );
    assert.equal(approved.status, 202, approved.body);
    const merged = await pollMerge(mergeId, f.user.headers, db);
    assert.equal(merged.state, 'SUCCEEDED', JSON.stringify(merged));
    const [[final]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      merged.result.content_version_id,
    ]);
    assert.match(final.canonical_body.toString(), /人工修复后的可审核正文/);
    assert.match(final.canonical_body.toString(), /修复页面打开后可靠保存的S2/);
    assert.match(final.canonical_body.toString(), /原冲突之后尚未拉入的MAIN新增/);
    const original = await get(`/v1/conflict-contexts/${id}`, f.user.headers);
    assert.equal(original.status, 200, original.body);
    assert.deepEqual(json(original), context);
  } finally {
    repaired?.close();
    main.close();
    f.close();
    await db.end();
  }
});
