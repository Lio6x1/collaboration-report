import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  reviewDatabase,
  openReviewWorkspace,
  writeDraftBody,
  submitDraft,
} from '../helpers/review.mjs';
import { conflictBody, conflictParagraph } from '../helpers/conflict-contexts.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { pollMerge } from '../helpers/merge.mjs';
import { uploadPNG, binaryResourceGet, resourcePNG } from '../helpers/resource.mjs';
import { resourceIds } from '../../packages/content/index.mjs';

test(
  '原冲突MAIN图片随固定轮次可读，后续删图不改原输入且过期后不能借轮次继续下载',
  { timeout: 90000 },
  async () => {
    const f = await reviewFixture(1);
    let db;
    let main;
    try {
      db = await reviewDatabase();
      const [[initial]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
        f.d.main.units[0].content_version_id,
      ]);
      const block = JSON.parse(initial.canonical_body.toString()).content[0].attrs.id;
      const resource = await uploadPNG(f);
      await unlockStructurePreview(f);
      main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');

      // 原送审和原基线均不含图片，只有真实并发MAIN修改含图，才能识别错误借用轮次自身引用。
      const mainParagraph = conflictParagraph(block, '原冲突MAIN的独立图片');
      mainParagraph.content.push({
        type: 'image',
        attrs: { resourceId: resource, alt: '原MAIN图片', title: null, width: 1, height: 1 },
      });
      assert.equal((await writeDraftBody(main, conflictBody(mainParagraph))).type, 'SAVED');
      assert.equal(
        (await writeDraftBody(f, conflictBody(conflictParagraph(block, '固定提交的另一修改'))))
          .type,
        'SAVED',
      );
      const submitted = (await submitDraft(f)).value;
      assert.equal(submitted.state, 'SUCCEEDED', JSON.stringify(submitted));
      const fixed = submitted.result;
      const mergeID = operation('resource-conflict');
      const merging = await post(
        `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
        {
          operation_id: mergeID,
          round_id: fixed.round_id,
          content_version_id: fixed.content_version_id,
          own_changes_to_resolve: [],
        },
        f.user.headers,
      );
      assert.equal(merging.status, 202, merging.body);
      const result = await pollMerge(mergeID, f.user.headers, db);
      assert.equal(result.state, 'REJECTED', JSON.stringify(result));
      assert.equal(result.error.code, 'MERGE_CONFLICT_REQUIRES_RESOLUTION');
      assert.equal(result.result.conflict_context_ref, mergeID);

      // 经真实读取固定原计算VERSION，而非预置冲突任务或资源引用。
      const response = await get(`/v1/conflict-contexts/${mergeID}`, f.user.headers);
      assert.equal(response.status, 200, response.body);
      const context = json(response);
      assert.equal(context.round_id, fixed.round_id);
      assert.deepEqual(resourceIds(context.base.body_json), []);
      assert.deepEqual(resourceIds(context.submitted.body_json), []);
      assert.deepEqual(resourceIds(context.main.body_json), [resource]);
      const path =
        `/v1/resources/${resource}/content?` +
        new URLSearchParams({
          document_id: f.d.id,
          context_kind: 'REVIEW',
          context_id: fixed.round_id,
        });
      const downloaded = await binaryResourceGet(path, f.user.headers);
      assert.equal(downloaded.status, 200, downloaded.body.toString('utf8'));
      assert.equal(downloaded.headers['content-type'], 'image/png');
      assert.deepEqual(downloaded.body, resourcePNG);

      // 当前MAIN删图确认SAVED后，原冲突VERSION仍保留原字节，不能退回当前MAIN集合授权。
      assert.equal(
        (await writeDraftBody(main, conflictBody(conflictParagraph(block, '后续MAIN已删除图片'))))
          .type,
        'SAVED',
      );
      const retained = await binaryResourceGet(path, f.user.headers);
      assert.equal(retained.status, 200, retained.body.toString('utf8'));
      assert.deepEqual(retained.body, resourcePNG);
      const denied = await binaryResourceGet(path, f.f.members[0].headers);
      assert.equal(denied.status, 403, denied.body.toString('utf8'));

      // 仅推进本测试MERGE收据期限，保留真实VERSION和资源事实，验证过期资格不能被旧轮次续用。
      const [expired] = await db.query(
        "UPDATE operation_receipt SET expires_at=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE operation_id=? AND kind='MERGE'",
        [mergeID],
      );
      assert.equal(expired.affectedRows, 1);
      const unavailable = await binaryResourceGet(path, f.user.headers);
      assert.equal(unavailable.status, 403, unavailable.body.toString('utf8'));
    } finally {
      main?.close();
      f.close();
      await db?.end();
    }
  },
);
