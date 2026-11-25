import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraft,
  reviewDatabase,
} from '../helpers/review.mjs';
import {
  get,
  post,
  json,
  operation,
  waitAuthorization,
  createUser,
} from '../helpers/workspace.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';

async function publicationInput(document, user, action, release) {
  const current = json(await get(`/v1/documents/${document}`, user.headers));
  return {
    operation_id: operation('public-change'),
    expected_publication_revision: current.publication_epoch,
    action,
    ...(release ? { release_id: release } : {}),
  };
}

async function changePublication(document, user, input) {
  const response = await post(`/v1/documents/${document}/publication`, input, user.headers);
  assert.equal(response.status, 202, json(response).code);
  const value = await pollPublication(input.operation_id, user);
  assert.equal(value.state, 'SUCCEEDED', value.error?.code);
  return value;
}

async function visibility(f, policy, knowledgeBase = false) {
  const path = knowledgeBase ? `/v1/knowledge-bases/${f.f.kb}` : `/v1/documents/${f.d.id}`;
  const current = json(await get(path, f.user.headers));
  return waitAuthorization(
    await post(
      path + '/visibility',
      {
        operation_id: operation('public-visibility'),
        expected_visibility_revision: current.visibility_revision,
        policy,
      },
      f.user.headers,
    ),
    f.user,
  );
}

async function publicFixture() {
  const f = await reviewFixture(1);
  try {
    await unlockStructurePreview(f);
    f.main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    await writeDraft(f.main, '正式发布的可靠主线S1');
    const first = await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
    f.first = first.result.entity_id;
    const close = f.close;
    f.close = () => {
      f.main.close();
      close();
    };
    return f;
  } catch (error) {
    f.main?.close();
    f.close();
    throw error;
  }
}

async function publicValue(f, headers = {}, suffix = '') {
  const response = await get(`/v1/public/documents/${f.d.id}` + suffix, headers);
  assert.equal(response.status, 200, json(response).code);
  return json(response);
}

async function reopenMain(f) {
  // 可见范围变更按协议关闭旧ACL准入；后续编辑必须重新取得真实票据，不能复用旧WSS。
  f.main.close();
  const response = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
  assert.equal(response.status, 200, json(response).code);
  f.main = await openReviewWorkspace(f.f, { id: f.d.id, main: json(response) }, f.user, 'MAIN');
}

// 使用实际身份、可见范围和固定正文；公开入口只展示当前指针，MAIN后续输入和改名不能改写旧正式版。
test('anonymous current reading follows publication switches and never falls back after unpublish', async () => {
  const f = await publicFixture();
  try {
    assert.equal((await get(`/v1/public/documents/${f.d.id}`)).status, 401);
    await visibility(f, 'PUBLIC', true);
    await visibility(f, 'PUBLIC');
    const first = await publicValue(f);
    assert.equal(first.release_id, f.first);
    assert.equal(first.title, '协作报告');
    assert.ok(JSON.stringify(first.units).includes('可靠主线S1'));
    await reopenMain(f);
    await writeDraft(f.main, '后来保存的主线S2');
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const renamed = await post(
      `/v1/documents/${f.d.id}`,
      {
        operation_id: operation('public-rename'),
        expected_control_revision: current.control_revision,
        title: '下一份正式版标题',
      },
      f.user.headers,
      'PATCH',
    );
    assert.equal(renamed.status, 200, json(renamed).code);
    assert.deepEqual(await publicValue(f), first);
    const second = await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
    const secondValue = await publicValue(f, {}, '?release_id=' + f.first);
    assert.equal(secondValue.release_id, second.result.entity_id);
    assert.equal(secondValue.title, '下一份正式版标题');
    assert.ok(JSON.stringify(secondValue.units).includes('主线S2'));
    await changePublication(
      f.d.id,
      f.user,
      await publicationInput(f.d.id, f.user, 'SWITCH', f.first),
    );
    const switched = await publicValue(f);
    assert.equal(switched.release_id, first.release_id);
    assert.equal(switched.title, first.title);
    assert.deepEqual(switched.units, first.units);
    await changePublication(f.d.id, f.user, await publicationInput(f.d.id, f.user, 'UNPUBLISH'));
    assert.equal((await get(`/v1/public/documents/${f.d.id}`)).status, 404);
    assert.equal((await get(`/v1/public/documents/${f.d.id}`, f.user.headers)).status, 404);
  } finally {
    f.close();
  }
});

test('public audience uses actual document and knowledge-base intersection and never downgrades an invalid cookie', async () => {
  const f = await publicFixture(),
    outsider = await createUser();
  try {
    await visibility(f, 'PUBLIC');
    assert.equal((await get(`/v1/public/documents/${f.d.id}`)).status, 401);
    assert.equal((await publicValue(f, f.user.headers)).release_id, f.first);
    assert.equal((await get(`/v1/public/documents/${f.d.id}`, f.f.members[0].headers)).status, 403);
    await visibility(f, 'SPACE_MEMBERS', true);
    assert.equal((await get(`/v1/public/documents/${f.d.id}`)).status, 401);
    assert.equal((await get(`/v1/public/documents/${f.d.id}`, outsider.headers)).status, 403);
    assert.equal((await publicValue(f, f.f.members[0].headers)).release_id, f.first);
    assert.equal((await get(`/v1/documents/${f.d.id}/main`, f.f.members[0].headers)).status, 403);
    await visibility(f, 'PUBLIC', true);
    assert.equal((await publicValue(f)).release_id, f.first);
    assert.equal(
      (await get(`/v1/public/documents/${f.d.id}`, { Cookie: '__Host-collabSession=invalid' }))
        .status,
      401,
    );
    for (const cookie of [
      '__Host-collabSession = invalid',
      '__Host-collabSession=""',
      outsider.headers.cookie + '; __Host-collabSession=invalid',
    ]) {
      assert.equal((await get(`/v1/public/documents/${f.d.id}`, { Cookie: cookie })).status, 401);
    }
    const logout = await post(
      '/v1/account/logout',
      { operation_id: operation('public-logout') },
      outsider.headers,
    );
    assert.equal(logout.status, 202, json(logout).code);
    assert.equal((await get(`/v1/public/documents/${f.d.id}`, outsider.headers)).status, 401);
    assert.equal((await publicValue(f)).release_id, f.first);
  } finally {
    f.close();
  }
});

test('release history and fixed historical reading require current internal access despite public visibility', async () => {
  const f = await publicFixture();
  try {
    await visibility(f, 'PUBLIC', true);
    await visibility(f, 'PUBLIC');
    await reopenMain(f);
    await writeDraft(f.main, '第二份正式历史正文');
    const second = await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
    const path = `/v1/documents/${f.d.id}/releases`;
    assert.equal((await get(path)).status, 401);
    assert.equal((await get(path, f.f.members[0].headers)).status, 403);
    assert.equal((await get(path + '/' + f.first, f.f.members[0].headers)).status, 403);
    const page = await get(path + '?limit=1', f.user.headers);
    assert.equal(page.status, 200, json(page).code);
    const value = json(page);
    assert.equal(value.items.length, 1);
    assert.equal(value.has_more, true);
    assert.ok(value.next_cursor);
    const rest = await get(
      path + '?limit=1&cursor=' + encodeURIComponent(value.next_cursor),
      f.user.headers,
    );
    assert.equal(rest.status, 200, json(rest).code);
    const all = [...value.items, ...json(rest).items].map((item) => item.id).sort();
    assert.deepEqual(all, [f.first, second.result.entity_id].sort());
    const resized = await get(
      path + '?limit=2&cursor=' + encodeURIComponent(value.next_cursor),
      f.user.headers,
    );
    assert.equal(resized.status, 409, json(resized).code);
    const historical = await get(path + '/' + f.first, f.user.headers);
    assert.equal(historical.status, 200, json(historical).code);
    assert.equal(json(historical).release_id, f.first);
    assert.ok(JSON.stringify(json(historical).units).includes('可靠主线S1'));
    assert.ok(!JSON.stringify(json(historical).units).includes('第二份正式历史正文'));
    assert.equal((await publicValue(f)).release_id, second.result.entity_id);
  } finally {
    f.close();
  }
});

test('anonymous delivery records bind actual release and publication epoch with SQL NULL admission', async () => {
  const f = await publicFixture(),
    db = await reviewDatabase();
  try {
    await visibility(f, 'PUBLIC', true);
    await visibility(f, 'PUBLIC');
    const value = await publicValue(f);
    const [deliveries] = await db.query(
      'SELECT CAST(publication_epoch AS CHAR) epoch,CAST(release_id AS CHAR) releaseId,admission_id IS NULL anonymous FROM delivery_session WHERE document_id=? AND publication_epoch IS NOT NULL',
      [f.d.id],
    );
    assert.ok(deliveries.length >= 2, 'ACTUAL_GO_AND_EDGE_ANONYMOUS_DELIVERY_MISSING');
    assert.ok(
      deliveries.every(
        (row) =>
          String(row.anonymous) === '1' &&
          row.epoch === value.publication_epoch &&
          row.releaseId === value.release_id,
      ),
    );
  } finally {
    f.close();
    await db.end();
  }
});
