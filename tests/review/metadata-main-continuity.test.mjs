import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, openReviewWorkspace, writeDraft } from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';

// 元数据更新也增加Doc控制修订：若没有实际Node上下文续接，已有可靠MAIN会
// 变成不可读或从旧业务清单重新播种。正文、后续编辑和发布必须保留真实SAVED尾部。
for (const kind of ['RENAME', 'RESPONSIBILITY']) {
  test(`active reliable MAIN remains readable, editable and publishable after ${kind}`, async () => {
    const f = await reviewFixture(kind === 'RESPONSIBILITY' ? 1 : 0);
    let main, reopened;
    try {
      await unlockStructurePreview(f);
      main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      await writeDraft(main, '元数据变化前已保存的S1');
      const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      const id = operation('metadata-' + kind.toLowerCase());
      if (kind === 'RENAME') {
        const response = await post(
          `/v1/documents/${f.d.id}`,
          {
            operation_id: id,
            expected_control_revision: before.control_revision,
            title: '新的文档标题',
          },
          f.user.headers,
          'PATCH',
        );
        assert.equal(response.status, 200, json(response).code);
      } else {
        await waitAuthorization(
          await post(
            `/v1/documents/${f.d.id}/responsible-transfers`,
            {
              operation_id: id,
              expected_revision: before.control_revision,
              target_user_id: f.f.members[0].id,
            },
            f.user.headers,
          ),
          f.user,
        );
      }
      const response = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
      assert.equal(response.status, 200, json(response).code);
      const view = json(response);
      const content = await get(
        `/v1/documents/${f.d.id}/read-views/${view.read_view_id}/units/${view.units[0].unit_id}`,
        f.user.headers,
      );
      assert.equal(content.status, 200, json(content).code);
      assert.ok(JSON.stringify(json(content).body_json).includes('已保存的S1'));
      assert.equal(BigInt(view.document.control_revision), BigInt(before.control_revision) + 1n);
      if (kind === 'RENAME') assert.equal(view.document.title, '新的文档标题');
      main.close();
      main = undefined;
      reopened = await openReviewWorkspace(f.f, { id: f.d.id, main: view }, f.user, 'MAIN');
      await writeDraft(reopened, '元数据变化后继续可靠保存的S2');
      const published = await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
      assert.ok(published.result.entity_id);
    } finally {
      reopened?.close();
      main?.close();
      f.close();
    }
  });
}
