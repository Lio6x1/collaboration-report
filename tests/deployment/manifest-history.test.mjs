import assert from 'node:assert/strict';
import test from 'node:test';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { openReviewWorkspace, writeDraft } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';

// 该场景针对冻结HTTP入口遗漏及新增历史视图边界；复用真实MAIN、授权和
// 章节安装，不重做拆分范围图的既有反例矩阵。
test(
  'historical manifest keeps fixed S1 and original structure while partial EDIT and revocation cannot open whole history',
  { timeout: 90000 },
  async () => {
    const f = await organizationFixture(1);
    const d = await documentFixture(f);
    const other = await documentFixture(f);
    const member = f.members[0];
    let main;
    const metadata = async () => json(await get(`/v1/documents/${d.id}`, f.owner.headers));
    const read = async (manifest, user = f.owner) =>
      get(`/v1/documents/${d.id}/manifests/${manifest}`, user.headers);
    const change = async (changes) =>
      waitAuthorization(
        await post(
          `/v1/documents/${d.id}/authorization-changes`,
          {
            operation_id: operation('manifest-grant'),
            expected_acl_revision: (await metadata()).acl_revision,
            changes,
          },
          f.owner.headers,
        ),
        f.owner,
      );
    const edit = {
      user_id: member.id,
      scope_kind: 'UNIT',
      scope_id: d.main.units[0].unit_id,
      capability: 'EDIT',
    };
    const comment = {
      user_id: member.id,
      scope_kind: 'DOCUMENT',
      scope_id: d.id,
      capability: 'COMMENT',
    };
    try {
      await unlockStructurePreview({ d, user: f.owner });
      main = await openReviewWorkspace(f, d, f.owner, 'MAIN');
      await writeDraft(main, '历史清单必须保持可靠S1');
      const firstResponse = await get(`/v1/documents/${d.id}/main`, f.owner.headers);
      assert.equal(firstResponse.status, 200);
      const first = json(firstResponse);
      await writeDraft(main, '后来S2不能替换旧清单S1');
      const previous = await read(first.manifest_id);
      assert.equal(previous.status, 200, previous.body);
      const fixed = json(previous);
      assert.equal(fixed.structure_id, first.structure_id);
      assert.equal(fixed.style_id, first.style_id);
      assert.deepEqual(fixed.units, first.units);
      const body = await get(
        `/v1/documents/${d.id}/read-views/${fixed.read_view_id}/units/${fixed.units[0].unit_id}`,
        f.owner.headers,
      );
      assert.equal(body.status, 200);
      assert.match(JSON.stringify(json(body).body_json), /可靠S1/);
      assert.doesNotMatch(JSON.stringify(json(body).body_json), /后来S2/);
      assert.equal(
        (await get(`/v1/documents/${other.id}/manifests/${first.manifest_id}`, f.owner.headers))
          .status,
        404,
      );
      await change([{ action: 'GRANT', grant: edit }]);
      assert.equal(
        (await read(first.manifest_id, member)).status,
        200,
        '完整原单元EDIT范围可读自己的历史',
      );
      main.close();
      main = undefined;
      const created = await completedMainControl(
        await post(
          `/v1/documents/${d.id}/chapters`,
          {
            operation_id: operation('manifest-chapter'),
            expected_structure_id: (await metadata()).current_structure_id,
            title: '局部EDIT之外的另一章',
          },
          f.owner.headers,
        ),
        f.owner,
      );
      assert.equal(created.state, 'SUCCEEDED');
      const nowResponse = await get(`/v1/documents/${d.id}/main`, f.owner.headers);
      assert.equal(nowResponse.status, 200);
      const now = json(nowResponse);
      assert.equal(now.units.length, 2);
      assert.equal(
        (await read(now.manifest_id, member)).status,
        403,
        '局部EDIT不能通过read_view开放整份历史',
      );
      await change([{ action: 'GRANT', grant: comment }]);
      const shared = await read(now.manifest_id, member);
      assert.equal(shared.status, 200);
      const sharedView = json(shared);
      await change([{ action: 'REVOKE', grant: comment }]);
      assert.equal((await read(now.manifest_id, member)).status, 403);
      assert.equal(
        (
          await get(
            `/v1/documents/${d.id}/read-views/${sharedView.read_view_id}/units/${sharedView.units[1].unit_id}`,
            member.headers,
          )
        ).status,
        403,
        '旧历史输出准入不能随剩余局部EDIT复活',
      );
      const final = json(await read(first.manifest_id));
      assert.equal(final.structure_id, first.structure_id);
      assert.equal(
        final.document.current_structure_id,
        now.structure_id,
        '当前元数据与旧固定清单分别表达真实事实',
      );
      assert.deepEqual(final.units, first.units);
      console.log(
        'MANIFEST_HISTORY_ACTUAL_PASS: fixed S1 and old structure; actual cross-document 404; full original EDIT coverage; partial multi-unit denial; COMMENT withdrawal closes old view',
      );
    } finally {
      main?.close();
    }
  },
);
