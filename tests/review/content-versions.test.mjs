import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraft, submitDraft } from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { discardPreviewPath, discardPreviewInput } from '../helpers/discard-previews.mjs';

const versionPath = (id) => '/v1/content-versions/' + id;

// 丢弃预览保存的是实际旧版本；不能用当前草稿正文补出一个看似固定的页面。
test('manager reads the real immutable discard version after later S2 and no anonymous version access is allowed', async () => {
  const f = await reviewFixture();
  try {
    await writeDraft(f, '固定丢弃预览的S1正文');
    const preview = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
    assert.equal(preview.status, 200, preview.body);
    const path = versionPath(json(preview).draft_content_version_id);
    const first = await get(path, f.user.headers);
    assert.equal(first.status, 200, '冻结契约中的内容版本读取尚未接通');
    const content = json(first);
    assert.equal(content.id, json(preview).draft_content_version_id);
    assert.equal(content.unit_id, f.d.main.units[0].unit_id);
    assert.equal(content.structure_id, json(preview).structure_id);
    assert.match(JSON.stringify(content.body_json), /固定丢弃预览的S1正文/);
    await writeDraft(f, '预览后S2必须继续保留');
    assert.deepEqual(json(await get(path, f.user.headers)), content);
    assert.equal((await get(path)).status, 401);
    assert.equal((await get(versionPath('999999999'), f.user.headers)).status, 404);
  } finally {
    f.close();
  }
});

test('COMMENT reads fixed review content but never unsubmitted preserved draft; current EDIT revocation closes draft history access', async () => {
  const f = await reviewFixture(2),
    editor = f.f.members[0],
    commenter = f.f.members[1];
  try {
    await writeDraft(f, '已送审的公开内部上下文');
    const submission = (await submitDraft(f)).value;
    assert.equal(submission.state, 'SUCCEEDED');
    await writeDraft(f, '未送审的私有后来草稿');
    const preview = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
    assert.equal(preview.status, 200, preview.body);
    const draftPath = versionPath(json(preview).draft_content_version_id);
    const reviewPath = versionPath(submission.result.content_version_id);
    const grants = [
      {
        user_id: editor.id,
        scope_kind: 'UNIT',
        scope_id: f.d.main.units[0].unit_id,
        capability: 'EDIT',
      },
      { user_id: editor.id, scope_kind: 'DOCUMENT', scope_id: f.d.id, capability: 'COMMENT' },
      { user_id: commenter.id, scope_kind: 'DOCUMENT', scope_id: f.d.id, capability: 'COMMENT' },
    ];
    await waitAuthorization(
      await post(
        `/v1/documents/${f.d.id}/authorization-changes`,
        {
          operation_id: operation('fixed-version-grants'),
          expected_acl_revision: '1',
          changes: grants.map((grant) => ({ action: 'GRANT', grant })),
        },
        f.user.headers,
      ),
      f.user,
    );
    assert.equal((await get(reviewPath, commenter.headers)).status, 200);
    assert.equal(
      (await get(draftPath, commenter.headers)).status,
      403,
      '批注权不能变成未送审草稿读取权',
    );
    const draft = await get(draftPath, editor.headers);
    assert.equal(draft.status, 200, draft.body);
    assert.match(JSON.stringify(json(draft).body_json), /未送审的私有后来草稿/);
    await waitAuthorization(
      await post(
        `/v1/documents/${f.d.id}/authorization-changes`,
        {
          operation_id: operation('fixed-version-revoke'),
          expected_acl_revision: '2',
          changes: [{ action: 'REVOKE', grant: grants[0] }],
        },
        f.user.headers,
      ),
      f.user,
    );
    assert.equal((await get(reviewPath, editor.headers)).status, 200);
    assert.equal((await get(draftPath, editor.headers)).status, 403);
  } finally {
    f.close();
  }
});
