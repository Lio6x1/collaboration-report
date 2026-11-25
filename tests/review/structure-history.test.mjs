import test from 'node:test';
import assert from 'node:assert/strict';
import { importMarkdown } from '../../packages/content/markdown.mjs';
import { fromPM } from '../../packages/content/index.mjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraftBody,
  submitDraft,
} from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';

test(
  'complete historical scope follows actual split-join-split edges and denies a partial current unit',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture(1),
      opened = [];
    try {
      await unlockStructurePreview(f);
      const main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      opened.push(main);
      let n = 0;
      const body = importMarkdown('## 第一节\n\n完整历史一\n\n## 第二节\n\n完整历史二', {
        newId: () => `history-range-${++n}`,
      }).body;
      await writeDraftBody(main, body);
      await writeDraftBody(f, body);
      const originalSubmission = (await submitDraft(f)).value;
      assert.equal(originalSubmission.state, 'SUCCEEDED', originalSubmission.error?.code);
      const original = originalSubmission.result;
      const withdraw = async (fixed) => {
        const response = await post(
          `/v1/review-requests/${fixed.request_id}/withdrawals`,
          { operation_id: operation('history-withdraw'), expected_round_id: fixed.round_id },
          f.user.headers,
        );
        assert.equal(response.status, 200, json(response).code);
      };
      await withdraw(original);
      const chapter = f.d.main.structure.entries[0].chapter_id;
      const apply = async (type) => {
        const document = json(await get(`/v1/documents/${f.d.id}`, f.user.headers)),
          input = {
            operation_id: operation('history-preview'),
            expected_structure_id: document.current_structure_id,
            expected_control_revision: document.control_revision,
            changes: [{ type, chapter_id: chapter }],
          };
        const preview = await post(
          `/v1/documents/${f.d.id}/structure-previews`,
          input,
          f.user.headers,
        );
        assert.equal(preview.status, 200, json(preview).code);
        const result = await completedMainControl(
          await post(
            `/v1/documents/${f.d.id}/structure-changes`,
            {
              ...input,
              operation_id: operation('history-confirm'),
              changes: input.changes.map((v) => ({
                ...v,
                preview_cut_ref: json(preview).preview_cut_ref,
              })),
            },
            f.user.headers,
          ),
          f.user,
        );
        assert.equal(result.state, 'SUCCEEDED', result.error?.code);
        const response = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
        assert.equal(response.status, 200, json(response).code);
        return json(response);
      };
      const grant = async (unit, action = 'GRANT') => {
        const document = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
        const response = await post(
          `/v1/documents/${f.d.id}/authorization-changes`,
          {
            operation_id: operation('history-grant'),
            expected_acl_revision: document.acl_revision,
            changes: [
              {
                action,
                grant: {
                  user_id: f.f.members[0].id,
                  scope_kind: 'UNIT',
                  scope_id: unit,
                  capability: 'EDIT',
                },
              },
            ],
          },
          f.user.headers,
        );
        assert.equal(response.status, 202, json(response).code);
        await waitAuthorization(response, f.user);
      };
      const history = (fixed) =>
        get(
          `/v1/review-requests/${fixed.request_id}/rounds/${fixed.round_id}`,
          f.f.members[0].headers,
        );
      const fixedContent = (fixed, user = f.f.members[0]) =>
        get('/v1/content-versions/' + fixed.content_version_id, user.headers);
      const split = await apply('SPLIT_LEVEL2');
      await grant(split.units[0].unit_id);
      assert.equal((await history(original)).status, 403);
      assert.equal((await fixedContent(original)).status, 403);
      const partialList = await get(
        `/v1/documents/${f.d.id}/review-requests`,
        f.f.members[0].headers,
      );
      assert.equal(partialList.status, 200, json(partialList).code);
      assert.ok(!json(partialList).items.some((v) => v.id === original.request_id));
      await grant(split.units[1].unit_id);
      const full = await history(original);
      assert.equal(full.status, 200, json(full).code);
      assert.equal(json(full).structure.id, f.d.main.structure.id);
      assert.deepEqual(json(await fixedContent(original)), json(full).content);
      const oldRequest = await get(
        `/v1/review-requests/${original.request_id}`,
        f.f.members[0].headers,
      );
      assert.equal(oldRequest.status, 200, json(oldRequest).code);
      assert.deepEqual(json(oldRequest).actions, []);
      const rounds = await get(
        `/v1/review-requests/${original.request_id}/rounds`,
        f.f.members[0].headers,
      );
      assert.equal(rounds.status, 200, json(rounds).code);
      assert.equal(json(rounds).items[0].structure.id, f.d.main.structure.id);
      const fullList = await get(`/v1/documents/${f.d.id}/review-requests`, f.f.members[0].headers);
      assert.equal(fullList.status, 200, json(fullList).code);
      assert.ok(json(fullList).items.some((v) => v.id === original.request_id));
      const forbiddenMutation = await post(
        `/v1/review-requests/${original.request_id}/withdrawals`,
        { operation_id: operation('history-write-denied'), expected_round_id: original.round_id },
        f.f.members[0].headers,
      );
      assert.equal(forbiddenMutation.status, 403, json(forbiddenMutation).code);
      const section = await openReviewWorkspace(
        f.f,
        { ...f.d, main: { ...split, units: [split.units[0]] } },
        f.user,
      );
      opened.push(section);
      // 新节草稿与MAIN相同，需实际保存一段S2才能产生可撤回的送审轮次。
      const sectionBody = fromPM(yDocToProsemirrorJSON(section.doc));
      sectionBody.content.push({
        type: 'paragraph',
        attrs: { id: 'history-section-S2' },
        content: [{ type: 'text', text: '原节送审追加' }],
      });
      await writeDraftBody(section, sectionBody);
      const sectionSubmission = (await submitDraft(section)).value;
      assert.equal(sectionSubmission.state, 'SUCCEEDED', sectionSubmission.error?.code);
      const sectionHistory = sectionSubmission.result;
      await withdraw(sectionHistory);
      await apply('JOIN_CHAPTER');
      const again = await apply('SPLIT_LEVEL2');
      const first = again.structure.entries.find((v) => v.title === '第一节');
      await grant(first.unit_id);
      const ownRange = await history(sectionHistory);
      assert.equal(ownRange.status, 200, json(ownRange).code);
      assert.equal((await history(original)).status, 403);
      for (const unit of again.units) {
        if (unit.unit_id !== first.unit_id) await grant(unit.unit_id);
      }
      assert.equal((await history(original)).status, 200);
      await grant(first.unit_id, 'REVOKE');
      assert.equal((await history(sectionHistory)).status, 403);
      assert.equal((await history(original)).status, 403);
      assert.equal((await fixedContent(sectionHistory)).status, 403);
      assert.equal((await fixedContent(original)).status, 403);
      // 删除最后的后代不产生空集合式“全覆盖”，旧历史仍需有效的完整文档资格。
      await apply('REMOVE');
      assert.equal((await history(original)).status, 403);
      const ownerHistory = await get(
        `/v1/review-requests/${original.request_id}/rounds/${original.round_id}`,
        f.user.headers,
      );
      assert.equal(ownerHistory.status, 200, json(ownerHistory).code);
      assert.equal(json(ownerHistory).structure.id, f.d.main.structure.id);
      assert.deepEqual(json(await fixedContent(original, f.user)), json(ownerHistory).content);
    } finally {
      for (const item of opened) item.close();
      f.close();
    }
  },
);
