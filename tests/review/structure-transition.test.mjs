import test from 'node:test';
import assert from 'node:assert/strict';
import { importMarkdown } from '../../packages/content/markdown.mjs';
import { canonical, fromPM } from '../../packages/content/index.mjs';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraftBody,
  reviewDatabase,
} from '../helpers/review.mjs';
import {
  get,
  post,
  json,
  operation,
  organizationFixture,
  documentFixture,
} from '../helpers/workspace.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { unlockStructurePreview, structurePreviewInput } from '../helpers/structure-previews.mjs';

test(
  'actual split and join install new common drafts, retire old identities and preserve accepted MAIN separately',
  { timeout: 90000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase();
    const opened = [];
    try {
      await unlockStructurePreview(f);
      const main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      opened.push(main);
      let next = 0;
      const accepted = importMarkdown(
        '## 第一节\n\n已接纳一\n\n## 第二节\n\n### 内部标题\n\n**已接纳二**',
        { newId: () => `transition-block-${++next}` },
      ).body;
      const pending = structuredClone(accepted);
      pending.content.push({
        type: 'paragraph',
        attrs: { id: 'transition-S2' },
        content: [{ type: 'text', text: '未审核后续草稿' }],
      });
      await writeDraftBody(main, accepted);
      await writeDraftBody(f, pending);
      const originalUnit = f.d.main.units[0].unit_id;
      const apply = async (type) => {
        const document = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
        const input = structurePreviewInput(f, document, type);
        const response = await post(
          `/v1/documents/${f.d.id}/structure-previews`,
          input,
          f.user.headers,
        );
        assert.equal(response.status, 200, json(response).code);
        const preview = json(response);
        const result = await completedMainControl(
          await post(
            `/v1/documents/${f.d.id}/structure-changes`,
            {
              ...input,
              operation_id: operation('structure-confirm'),
              changes: input.changes.map((v) => ({
                ...v,
                preview_cut_ref: preview.preview_cut_ref,
              })),
            },
            f.user.headers,
          ),
          f.user,
        );
        assert.equal(result.state, 'SUCCEEDED', result.error?.code);
        assert.equal(result.result.main_sync_state, 'READY');
        const current = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
        assert.equal(current.status, 200, json(current).code);
        return json(current);
      };
      const split = await apply('SPLIT_LEVEL2');
      const root = split.structure.entries.find((v) => v.kind === 'CHAPTER');
      assert.equal(root.split_level2, true);
      assert.equal(root.unit_id, undefined);
      assert.equal(split.units.length, 2);
      assert.ok(split.units.every((v) => v.unit_id !== originalUnit));
      for (const unit of split.units) {
        const [[version]] = await db.query(
          'SELECT canonical_body FROM content_version WHERE id=? AND document_id=? AND unit_id=?',
          [unit.content_version_id, f.d.id, unit.unit_id],
        );
        assert.doesNotMatch(version.canonical_body.toString(), /未审核后续草稿/);
      }
      const children = [];
      for (const unit of split.units) {
        const child = await openReviewWorkspace(
          f.f,
          { ...f.d, main: { ...split, units: [unit] } },
          f.user,
        );
        opened.push(child);
        children.push(child);
      }
      // WSS OPEN读取安装后的真实检查点；不能仅凭Go RESTORING元数据声称新支线已经可用。
      const { yDocToProsemirrorJSON } = await import('y-prosemirror');
      assert.match(canonical(children.map((v) => yDocToProsemirrorJSON(v.doc))), /未审核后续草稿/);
      const [[old]] = await db.query(
        'SELECT retired_structure_id FROM collaboration_unit WHERE id=? AND document_id=?',
        [originalUnit, f.d.id],
      );
      assert.equal(String(old.retired_structure_id), split.structure.id);
      const [[lineage]] = await db.query(
        'SELECT COUNT(*) AS n FROM unit_lineage WHERE source_unit_id=? AND source_document_id=?',
        [originalUnit, f.d.id],
      );
      assert.equal(lineage.n, '2');
      const joined = await apply('JOIN_CHAPTER');
      assert.equal(joined.units.length, 1);
      assert.equal(joined.structure.entries[0].split_level2, false);
      assert.notEqual(joined.units[0].unit_id, originalUnit);
      assert.ok(split.units.every((v) => v.unit_id !== joined.units[0].unit_id));
      const [[joinedVersion]] = await db.query(
        'SELECT canonical_body FROM content_version WHERE id=? AND document_id=? AND unit_id=?',
        [joined.units[0].content_version_id, f.d.id, joined.units[0].unit_id],
      );
      assert.equal(joinedVersion.canonical_body.toString(), canonical(accepted));
      const branch = await openReviewWorkspace(f.f, { ...f.d, main: joined }, f.user);
      opened.push(branch);
      assert.equal(canonical(fromPM(yDocToProsemirrorJSON(branch.doc))), canonical(pending));
    } finally {
      for (const workspace of opened) workspace.close();
      f.close();
      await db.end();
    }
  },
);

test(
  'saved input or a newly opened zero-tail branch rejects the original preview without replacing its cut',
  { timeout: 90000 },
  async () => {
    for (const mode of ['SAVED', 'LATE_OPEN']) {
      const organization = await organizationFixture(0),
        d = await documentFixture(organization),
        f = { f: organization, d, user: organization.owner };
      let branch;
      const db = await reviewDatabase(),
        nodeDB = await reviewDatabase('collaboration');
      try {
        await unlockStructurePreview(f);
        if (mode === 'SAVED') branch = await openReviewWorkspace(organization, d, f.user);
        const document = json(await get(`/v1/documents/${d.id}`, f.user.headers)),
          input = structurePreviewInput(f, document, 'REMOVE');
        const response = await post(
          `/v1/documents/${d.id}/structure-previews`,
          input,
          f.user.headers,
        );
        assert.equal(response.status, 200, json(response).code);
        const preview = json(response);
        const [[before]] = await db.query(
          'SELECT payload FROM durable_task WHERE kind="STRUCTURE_PREVIEW_FIXED" AND business_key=?',
          [input.operation_id],
        );
        const original = canonical(
          typeof before.payload === 'string' ? JSON.parse(before.payload) : before.payload,
        );
        if (mode === 'LATE_OPEN') branch = await openReviewWorkspace(organization, d, f.user);
        else
          await writeDraftBody(branch, {
            type: 'doc',
            content: [
              {
                type: 'paragraph',
                attrs: { id: 'preview-later-input' },
                content: [{ type: 'text', text: '预览后的可靠输入' }],
              },
            ],
          });
        const id = operation('changed-confirm');
        const result = await completedMainControl(
          await post(
            `/v1/documents/${d.id}/structure-changes`,
            {
              ...input,
              operation_id: id,
              changes: input.changes.map((v) => ({
                ...v,
                preview_cut_ref: preview.preview_cut_ref,
              })),
            },
            f.user.headers,
          ),
          f.user,
        );
        assert.equal(result.state, 'REJECTED');
        assert.equal(result.error.code, 'STRUCTURE_PREVIEW_CHANGED');
        const current = json(await get(`/v1/documents/${d.id}`, f.user.headers));
        assert.equal(current.current_structure_id, document.current_structure_id);
        assert.equal(current.current_manifest_id, document.current_manifest_id);
        assert.equal(current.control_revision, document.control_revision);
        const [[fixed]] = await db.query(
          'SELECT payload FROM durable_task WHERE kind="STRUCTURE_PREVIEW_FIXED" AND business_key=?',
          [input.operation_id],
        );
        const payload =
          typeof fixed.payload === 'string' ? JSON.parse(fixed.payload) : fixed.payload;
        assert.equal(canonical(payload), original);
        const [[prepared]] = await nodeDB.query(
          'SELECT COUNT(*) AS n FROM durable_task WHERE kind="STRUCTURE_PREPARATION" AND business_key=?',
          [id],
        );
        assert.equal(prepared.n, '0');
      } finally {
        branch?.close();
        await db.end();
        await nodeDB.end();
      }
    }
  },
);

test(
  'removing the final chapter installs a real empty manifest and permits a fresh chapter',
  { timeout: 90000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase();
    try {
      await unlockStructurePreview(f);
      await writeDraftBody(f, {
        type: 'doc',
        content: [
          {
            type: 'paragraph',
            attrs: { id: 'removed-draft' },
            content: [{ type: 'text', text: '被删除但保留的可靠草稿' }],
          },
        ],
      });
      const document = json(await get(`/v1/documents/${f.d.id}`, f.user.headers)),
        input = structurePreviewInput(f, document, 'REMOVE');
      const response = await post(
        `/v1/documents/${f.d.id}/structure-previews`,
        input,
        f.user.headers,
      );
      assert.equal(response.status, 200, json(response).code);
      const preview = json(response),
        id = operation('remove-final-chapter');
      const result = await completedMainControl(
        await post(
          `/v1/documents/${f.d.id}/structure-changes`,
          {
            ...input,
            operation_id: id,
            changes: input.changes.map((v) => ({ ...v, preview_cut_ref: preview.preview_cut_ref })),
          },
          f.user.headers,
        ),
        f.user,
      );
      assert.equal(result.state, 'SUCCEEDED');
      assert.equal(result.result.main_sync_state, 'READY');
      const mainResponse = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
      assert.equal(mainResponse.status, 200, json(mainResponse).code);
      const main = json(mainResponse);
      assert.deepEqual(main.units, []);
      assert.deepEqual(main.structure.entries, []);
      const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      const chapter = await completedMainControl(
        await post(
          `/v1/documents/${f.d.id}/chapters`,
          {
            operation_id: operation('after-empty-chapter'),
            expected_structure_id: current.current_structure_id,
            title: '重新开始',
          },
          f.user.headers,
        ),
        f.user,
      );
      assert.equal(chapter.state, 'SUCCEEDED');
      const restored = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
      assert.equal(restored.units.length, 1);
      assert.notEqual(restored.units[0].unit_id, f.d.main.units[0].unit_id);
      const [[old]] = await db.query('SELECT lifecycle FROM branch WHERE id=?', [
        f.session.branch_id,
      ]);
      assert.equal(old.lifecycle, 'RETIRED');
    } finally {
      f.close();
      await db.end();
    }
  },
);
