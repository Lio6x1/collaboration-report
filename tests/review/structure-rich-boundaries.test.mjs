import test from 'node:test';
import assert from 'node:assert/strict';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import { importMarkdown } from '../../packages/content/markdown.mjs';
import { canonical, fromPM, resourceIds, normalizeContent } from '../../packages/content/index.mjs';
import { draftBody } from '../../services/collaboration/src/domain/draft_protocol.mjs';
import {
  reviewFixture,
  reviewDatabase,
  openReviewWorkspace,
  writeDraftBody,
  submitDraft,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { unlockStructurePreview, structurePreviewInput } from '../helpers/structure-previews.mjs';

test(
  'actual repeated split and join preserve introduction, edited empty section, MAIN fonts and pinned images',
  { timeout: 150000 },
  async () => {
    const f = await reviewFixture();
    const database = await reviewDatabase();
    const opened = [];
    try {
      await unlockStructurePreview(f);
      // READY元数据仅用于真实引用/保护/迁移链；本用例不把它当成OSS上传成功。
      const key = operation('rich-structure-resource');
      const [created] = await database.query(
        "INSERT INTO resource(space_id,uploader_id,storage_key,checksum,byte_size,media_type,state,upload_operation_id) VALUES(?,?,?,UNHEX(REPEAT('00',32)),1,'image/png','READY',?)",
        [f.f.space, f.user.id, key, key],
      );
      const resource = String(created.insertId);
      let next = 0;
      const accepted = importMarkdown(
        '概述正文\n\n## 空节\n\n## 富文本节\n\n**正文**与 $x^2$\n\n![迁移图片](https://example.test/structure.png)\n\n| 列 | 值 |\n| --- | --- |\n| 一 | 二 |',
        {
          newId: () => `rich-structure-${++next}`,
          resources: new Map([['https://example.test/structure.png', resource]]),
        },
      ).body;
      const emptyHeading = accepted.content.find((node) => node.type === 'heading');
      emptyHeading.content[0].marks = [{ type: 'font', attrs: { family: 'SimSun' } }];
      accepted.content[0].content[0].marks = [{ type: 'font', attrs: { family: 'SimSun' } }];
      const pending = draftBody(accepted);
      pending.content.push({
        type: 'paragraph',
        attrs: { id: 'rich-structure-S2' },
        content: [{ type: 'text', text: '富文本节尚未审核的S2' }],
      });
      const main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      opened.push(main);
      await writeDraftBody(main, accepted);
      await writeDraftBody(f, pending);
      const apply = async (type) => {
        const document = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
        const input = structurePreviewInput(f, document, type);
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
              operation_id: operation('rich-structure-confirm'),
              changes: input.changes.map((change) => ({
                ...change,
                preview_cut_ref: json(preview).preview_cut_ref,
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
      for (let iteration = 0; iteration < 2; iteration++) {
        const split = await apply('SPLIT_LEVEL2');
        assert.equal(split.units.length, 3);
        assert.deepEqual(
          split.structure.entries.filter((entry) => entry.unit_id).map((entry) => entry.kind),
          ['INTRODUCTION', 'SECTION', 'SECTION'],
        );
        const empty = split.structure.entries.find((entry) => entry.title === '空节');
        for (const unit of split.units) {
          const child = await openReviewWorkspace(
            f.f,
            { ...f.d, main: { ...split, units: [unit] } },
            f.user,
          );
          opened.push(child);
          const body = normalizeContent(fromPM(yDocToProsemirrorJSON(child.doc)));
          assert.doesNotMatch(canonical(body), /"type":"font"/);
          const entry = split.structure.entries.find((entry) => entry.unit_id === unit.unit_id);
          if (iteration === 0 && entry.kind === 'INTRODUCTION') {
            const unchanged = (await submitDraft(child)).value;
            assert.equal(
              unchanged.state,
              'REJECTED',
              'font projection must not invent a new review',
            );
            assert.equal(unchanged.error.code, 'SUBMISSION_NO_CHANGES');
          }
          if (unit.unit_id === empty.unit_id) {
            if (iteration === 0) {
              assert.equal(body.content.length, 1);
              assert.equal(body.content[0].type, 'paragraph');
              assert.deepEqual(body.content[0].content, []);
              body.content[0].content = [{ type: 'text', text: '原空节新增可靠草稿' }];
              await writeDraftBody(child, body);
              const position = pending.content.findIndex(
                (node) => node.attrs?.id === emptyHeading.attrs.id,
              );
              pending.content.splice(position + 1, 0, structuredClone(body.content[0]));
            } else assert.match(canonical(body), /原空节新增可靠草稿/);
          }
          if (resourceIds(body).length) {
            assert.deepEqual(resourceIds(body), [resource]);
            const [[pin]] = await database.query(
              "SELECT COUNT(*) n FROM resource_reference WHERE resource_id=? AND document_id=? AND owner_kind='DRAFT' AND owner_ref=?",
              [resource, f.d.id, child.session.room_id + ':' + child.session.workspace_epoch],
            );
            assert.equal(Number(pin.n), 1);
          }
        }
        const joined = await apply('JOIN_CHAPTER');
        assert.equal(joined.units.length, 1);
        const [[version]] = await database.query(
          'SELECT canonical_body FROM content_version WHERE id=? AND document_id=? AND unit_id=?',
          [joined.units[0].content_version_id, f.d.id, joined.units[0].unit_id],
        );
        // 管理字体与原空节在MAIN精确保留；修改后的占位只进入草稿，合章不能再消除它。
        assert.equal(version.canonical_body.toString(), canonical(accepted));
        const branch = await openReviewWorkspace(f.f, { ...f.d, main: joined }, f.user);
        opened.push(branch);
        assert.equal(
          canonical(normalizeContent(fromPM(yDocToProsemirrorJSON(branch.doc)))),
          canonical(pending),
        );
        const [[versionPin]] = await database.query(
          "SELECT COUNT(*) n FROM resource_reference WHERE resource_id=? AND document_id=? AND owner_kind='VERSION' AND owner_ref=?",
          [resource, f.d.id, joined.units[0].content_version_id],
        );
        assert.equal(Number(versionPin.n), 1);
      }
    } finally {
      for (const workspace of opened) workspace.close();
      f.close();
      await database.end();
    }
  },
);
