import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraft, submitDraft } from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';

// 反例：若读取时使用活跃草稿，送审后继续编辑就会替换审核者正在审的正文。
test('review reads expose fixed round content, immutable history and revision-bound action pages', async () => {
  const f = await reviewFixture();
  try {
    await writeDraft(f, '第一轮固定正文');
    const first = (await submitDraft(f)).value.result,
      path = '/v1/review-requests/' + first.request_id;
    await writeDraft(f, '第二轮待提交草稿');
    const read = await get(path, f.f.owner.headers);
    assert.equal(read.status, 200, json(read).code);
    assert.equal(json(read).current_round.content_version_id, first.content_version_id);
    assert.match(JSON.stringify(json(read).current_round.content.body_json), /第一轮固定正文/);
    assert.doesNotMatch(JSON.stringify(json(read).current_round.content.body_json), /第二轮/);
    assert.equal(json(read).current_round.style.id, f.d.main.style.id);
    assert.equal(json(read).current_round.structure.id, f.d.main.structure.id);
    const listed = await get('/v1/documents/' + f.d.id + '/review-requests', f.f.owner.headers);
    assert.equal(listed.status, 200);
    assert.equal(json(listed).items[0].id, first.request_id);
    const mine = await get('/v1/me/review-requests', f.f.owner.headers);
    assert.equal(mine.status, 200);
    assert.ok(json(mine).items.some((x) => x.id === first.request_id));
    const change = await post(
      path + '/actions',
      {
        operation_id: operation('change'),
        round_id: first.round_id,
        content_version_id: first.content_version_id,
        type: 'REQUEST_CHANGES',
        reason: '补齐来源',
      },
      f.f.owner.headers,
    );
    assert.equal(change.status, 200);
    assert.equal(
      (
        await post(
          path + '/withdrawals',
          { operation_id: operation('withdraw'), expected_round_id: first.round_id },
          f.f.owner.headers,
        )
      ).status,
      200,
    );
    const second = (
      await submitDraft(f, {
        mode: 'RESUBMIT',
        request_id: first.request_id,
        expected_round_id: first.round_id,
      })
    ).value.result;
    const rounds = await get(path + '/rounds?limit=1', f.f.owner.headers);
    assert.equal(rounds.status, 200);
    assert.equal(json(rounds).items.length, 1);
    assert.equal(json(rounds).items[0].id, first.round_id);
    assert.ok(json(rounds).has_more && json(rounds).next_cursor);
    const next = await get(
      path + '/rounds?limit=1&cursor=' + encodeURIComponent(json(rounds).next_cursor),
      f.f.owner.headers,
    );
    assert.equal(next.status, 200);
    assert.equal(json(next).items[0].id, second.round_id);
    const old = await get(path + '/rounds/' + first.round_id, f.f.owner.headers);
    assert.equal(old.status, 200);
    assert.match(JSON.stringify(json(old).content.body_json), /第一轮固定正文/);
    const changes = await get(path + '/change-requests', f.f.owner.headers);
    assert.equal(changes.status, 200);
    assert.equal(json(changes).items[0].round_id, first.round_id);
    assert.equal(json(changes).items[0].status, 'OPEN');
    assert.equal(json(changes).items[0].reason, '补齐来源');
    const actions = await get(path + '/actions?limit=1', f.f.owner.headers);
    assert.equal(actions.status, 200);
    assert.equal(json(actions).items[0].type, 'REQUEST_CHANGES');
    assert.equal(json(actions).items[0].change_request_id, json(change).result.entity_id);
    const cursor = json(actions).next_cursor;
    assert.ok(cursor);
    assert.equal(
      (
        await post(
          path + '/change-requests/' + json(change).result.entity_id + '/resolve',
          {
            operation_id: operation('resolve'),
            expected_round_id: second.round_id,
            expected_change_revision: '1',
            reason: '已解决',
          },
          f.f.owner.headers,
        )
      ).status,
      200,
    );
    assert.equal(
      (await get(path + '/actions?limit=1&cursor=' + encodeURIComponent(cursor), f.f.owner.headers))
        .status,
      409,
    );
  } finally {
    f.close();
  }
});

test('comment scope reads submitted history only while current grant remains valid', async () => {
  const f = await reviewFixture(1);
  try {
    await writeDraft(f, '当前权限控制的历史正文');
    const fixed = (await submitDraft(f)).value.result,
      reader = f.f.members[0],
      path = '/v1/review-requests/' + fixed.request_id;
    assert.equal((await get(path, reader.headers)).status, 403);
    const unseen = await get('/v1/me/review-requests', reader.headers);
    assert.equal(unseen.status, 200, json(unseen).code);
    assert.equal(json(unseen).items.length, 0);
    const grant = {
      user_id: reader.id,
      scope_kind: 'DOCUMENT',
      scope_id: f.d.id,
      capability: 'COMMENT',
    };
    const change = async (action) => {
      const d = json(await get('/v1/documents/' + f.d.id, f.f.owner.headers));
      await waitAuthorization(
        await post(
          '/v1/documents/' + f.d.id + '/authorization-changes',
          {
            operation_id: operation('acl'),
            expected_acl_revision: d.acl_revision,
            changes: [{ action, grant }],
          },
          f.f.owner.headers,
        ),
        f.f.owner,
      );
    };
    await change('GRANT');
    assert.equal((await get(path + '/rounds/' + fixed.round_id, reader.headers)).status, 200);
    const reading = json(await get(path, reader.headers));
    assert.ok(
      !reading.actions.includes('REQUEST_CHANGES') && !reading.actions.includes('WITHDRAW'),
    );
    assert.equal(
      (
        await post(
          path + '/actions',
          {
            operation_id: operation('comment-review'),
            round_id: fixed.round_id,
            content_version_id: fixed.content_version_id,
            type: 'REQUEST_CHANGES',
            reason: '批注身份没有审核权',
          },
          reader.headers,
        )
      ).status,
      403,
    );
    assert.equal(
      (
        await post(
          `/v1/documents/${f.d.id}/units/${f.grant.context.target.unit_id}/edit-sessions`,
          {
            operation_id: operation('draft-read'),
            workspace_kind: 'BRANCH',
            client_session_id: operation('client'),
            expected_structure_id: f.grant.context.target.structure_id,
          },
          reader.headers,
        )
      ).status,
      403,
    );
    await change('REVOKE');
    assert.equal((await get(path + '/rounds/' + fixed.round_id, reader.headers)).status, 403);
    assert.equal((await get(path + '/change-requests', reader.headers)).status, 403);
    assert.equal(json(await get('/v1/me/review-requests', reader.headers)).items.length, 0);
  } finally {
    f.close();
  }
});
