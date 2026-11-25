import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraftBody,
  writeDraft,
  submitDraft,
} from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { uploadPNG, binaryResourceGet, resourcePNG } from '../helpers/resource.mjs';

test(
  '支线历史批注图片只向当前原讨论读者开放，未讨论草稿与撤权均拒绝',
  { timeout: 60000 },
  async () => {
    let f = await reviewFixture(1);
    try {
      const member = f.f.members[0];
      async function permission(action) {
        const current = await get(`/v1/documents/${f.d.id}`, f.user.headers);
        assert.equal(current.status, 200, current.body);
        const changed = await waitAuthorization(
          await post(
            `/v1/documents/${f.d.id}/authorization-changes`,
            {
              operation_id: operation('branch-comment-permission'),
              expected_acl_revision: json(current).acl_revision,
              changes: [
                {
                  action,
                  grant: {
                    user_id: member.id,
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
        assert.equal(changed.state, 'SUCCEEDED');
      }
      await permission('GRANT');
      const old = f;
      f = await openReviewWorkspace(old.f, old.d, old.user);
      old.close();
      const image = await uploadPNG(f);
      assert.equal(
        (
          await writeDraftBody(f, {
            type: 'doc',
            content: [
              {
                type: 'paragraph',
                attrs: { id: 'branch-picture' },
                content: [
                  {
                    type: 'image',
                    attrs: {
                      resourceId: image,
                      alt: '原讨论图片',
                      title: null,
                      width: 1,
                      height: 1,
                    },
                  },
                ],
              },
            ],
          })
        ).type,
        'SAVED',
      );
      const path =
        `/v1/resources/${image}/content?` +
        new URLSearchParams({
          document_id: f.d.id,
          context_kind: 'BRANCH',
          context_id: f.session.branch_id,
        });
      // COMMENT不能读取任意当前草稿，即使资源确实存在且当前房间引用它。
      assert.equal((await binaryResourceGet(path, member.headers)).status, 403);
      const submitted = (await submitDraft(f)).value;
      assert.equal(submitted.state, 'SUCCEEDED');
      const created = await post(
        `/v1/documents/${f.d.id}/comment-threads`,
        {
          operation_id: operation('branch-picture-thread'),
          context_kind: 'BRANCH',
          context_id: f.session.branch_id,
          anchor: {
            schema_version: 'anchor-v1',
            content_version_id: submitted.result.content_version_id,
          },
          body: '查看这个固定版本中的图片',
          mentions: [],
        },
        f.user.headers,
      );
      assert.equal(created.status, 201, created.body);
      const thread = json(created).result.entity_id;
      assert.equal((await writeDraft(f, '当前草稿已移除图片')).type, 'SAVED');
      const original = await binaryResourceGet(path, member.headers);
      assert.equal(original.status, 200, original.body.toString('utf8'));
      assert.deepEqual(original.body, resourcePNG);
      assert.equal((await get(`/v1/comment-threads/${thread}`, member.headers)).status, 200);
      await permission('REVOKE');
      assert.equal((await binaryResourceGet(path, member.headers)).status, 403);
      assert.equal((await get(`/v1/comment-threads/${thread}`, member.headers)).status, 403);
    } finally {
      f.close();
    }
  },
);
