import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import { importMarkdown } from '../../../packages/content/markdown.mjs';
import {
  canonical,
  fromPM,
  normalizeContent,
  resourceIds,
} from '../../../packages/content/index.mjs';
import { hash } from '../src/domain/shared_protocol.mjs';
import { splitChapterDrafts } from '../src/domain/structure_content.mjs';
import { buildStructureMigration } from '../src/domain/structure_migration.mjs';
import { fixedDraft, draftBody } from '../src/domain/draft_protocol.mjs';
import { StructureInstallations } from '../src/application/structure_installations.mjs';

const operation = 'structure-migration-original';

function markdown(text) {
  let next = 0;
  return importMarkdown(text, {
    newId: () => `migration-block-${++next}`,
    resources: new Map([['https://example.test/p.png', '9007199254740997']]),
  }).body;
}

function content(body, unit, branch = '0') {
  const bytes = Buffer.from(canonical(body));
  return {
    document_id: '1',
    unit_id: unit,
    branch_id: branch,
    structure_id: '2',
    workspace_epoch: '1',
    operation_id: operation,
    body_json_utf8: bytes.toString('base64'),
    body_hash: hash(bytes).toString('base64'),
    byte_size: String(bytes.length),
    resource_ids: resourceIds(body),
  };
}

function fixture(main, draft = structuredClone(main), base = main) {
  return {
    binding: {
      context: { operation_id: operation, target: { document_id: '1', structure_id: '2' } },
    },
    source: {
      structure: {
        id: '2',
        document_id: '1',
        entries: [
          {
            node_id: '3',
            chapter_id: '3',
            unit_id: '4',
            kind: 'CHAPTER',
            title: '整章',
            sort_order: 1,
            split_level2: false,
          },
        ],
      },
      changes: [{ type: 'SPLIT_LEVEL2', chapter_id: '3' }],
      drafts: [
        {
          unit_id: '4',
          branch_id: '5',
          workspace_epoch: '1',
          lineage: { body: structuredClone(base) },
        },
      ],
    },
    main_units: [content(main, '4')],
    drafts: [content(draft, '4', '5')],
  };
}

test('fixed split candidates preserve rich MAIN, later S2, empty sections and complete source intervals', () => {
  const main = markdown(
    '## 空节\n\n## 富文本\n\n![图](https://example.test/p.png)\n\n### 内部标题\n\n|甲|乙|\n|---|---|\n|一|二|',
  );
  const draft = structuredClone(main);
  draft.content.unshift({
    type: 'paragraph',
    attrs: { id: 'S2-intro' },
    content: [{ type: 'text', text: '新增概述草稿' }],
  });
  draft.content.push({
    type: 'paragraph',
    attrs: { id: 'S2-section' },
    content: [{ type: 'text', text: '小节后续草稿' }],
  });
  const fixed = fixture(main, draft),
    original = canonical(fixed);
  const candidate = buildStructureMigration(fixed),
    change = candidate.changes[0];
  assert.equal(candidate.schema_version, 'structure-migration-v1');
  assert.deepEqual(change.source_units, ['4']);
  assert.deepEqual(
    change.targets.map((v) => v.kind),
    ['INTRODUCTION', 'SECTION', 'SECTION'],
  );
  assert.match(canonical(change.targets[0].draft_body), /新增概述草稿/);
  assert.doesNotMatch(canonical(change.targets[0].main_body), /新增概述草稿/);
  assert.match(canonical(change.targets[2].draft_body), /小节后续草稿/);
  assert.doesNotMatch(canonical(change.targets[2].main_body), /小节后续草稿/);
  assert.deepEqual(resourceIds(change.targets[2].main_body), ['9007199254740997']);
  // 范围证明描述全部原单元及稳定边界；不能只用当前仍存在的块ID推断历史范围。
  const ranges = change.targets.map((v) => v.source_ranges[0]);
  assert.deepEqual(
    ranges.map((v) => [v.start_boundary, v.end_boundary]),
    [
      [null, main.content[0].attrs.id],
      [main.content[0].attrs.id, main.content[1].attrs.id],
      [main.content[1].attrs.id, null],
    ],
  );
  assert.ok(ranges.every((v) => v.source_unit_id === '4' && v.source_scope === 'WHOLE_UNIT'));
  assert.equal(canonical(fixed), original);
  assert.equal(canonical(buildStructureMigration(fixed)), canonical(candidate));
});

test('split virtually pulls new MAIN H2 while preserving independent pending edits and original fixed bytes', () => {
  // 同一候选样本也交给Go确认/安装验证，防止只在Node内部成功而跨服务形状不兼容。
  const vector = JSON.parse(
      readFileSync(
        new URL('../../../tests/fixtures/structure-aligned-split.json', import.meta.url),
      ),
    ),
    fixed = vector.fixed,
    original = canonical(fixed),
    migration = buildStructureMigration(fixed),
    split = migration.changes[0];
  assert.deepEqual(migration, vector.migration);
  assert.deepEqual(
    split.targets.map((v) => v.title),
    ['原节', '主线新增节'],
  );
  assert.match(canonical(split.targets[0].draft_body), /支线独立改动/);
  assert.match(canonical(split.targets[0].draft_body), /尚未审核追加/);
  assert.doesNotMatch(canonical(split.targets[0].main_body), /支线独立改动|尚未审核追加/);
  assert.deepEqual(split.targets[1].draft_body, split.targets[1].main_body);
  assert.deepEqual(resourceIds(split.targets[1].draft_body), ['9007199254740997']);
  // 对齐后的来源块包含同一固定MAIN引入的块，仍指向原整章；不是伪造原支线capture。
  assert.deepEqual(
    split.targets[1].source_ranges[0].draft_block_ids,
    split.targets[1].source_ranges[0].main_block_ids,
  );
  assert.equal(split.targets[1].source_ranges[0].source_unit_id, '4');
  assert.equal(canonical(fixed), original);
});

test('a common draft that has never contained H2 can split after MAIN adds sections', () => {
  const base = markdown('原概述'),
    main = markdown('原概述\n\n## 主线新节\n\n新节正文'),
    draft = structuredClone(base);
  draft.content[0].content[0].text = '概述未审核修改';
  const split = buildStructureMigration(fixture(main, draft, base)).changes[0];
  assert.deepEqual(
    split.targets.map((target) => target.kind),
    ['INTRODUCTION', 'SECTION'],
  );
  assert.match(canonical(split.targets[0].draft_body), /概述未审核修改/);
  assert.match(canonical(split.targets[0].main_body), /原概述/);
  assert.deepEqual(split.targets[1].draft_body, split.targets[1].main_body);
});

test('virtual split refuses real three-way conflict and requires the actual lineage base', () => {
  const base = markdown('## 原节\n\nabcdef'),
    main = structuredClone(base),
    draft = structuredClone(base);
  main.content[1].content[0].text = 'MAIN';
  draft.content[1].content[0].text = 'DRAFT';
  const fixed = fixture(main, draft, base),
    original = canonical(fixed);
  assert.throws(() => buildStructureMigration(fixed), /STRUCTURE_DRAFT_CONFLICT/);
  assert.equal(canonical(fixed), original);
  const missingBase = fixture(base);
  delete missingBase.source.drafts[0].lineage;
  assert.throws(() => buildStructureMigration(missingBase), /STRUCTURE_LINEAGE_UNKNOWN/);
});

test('aligned candidates install exact checkpoints with MAIN lineage and protected image references', async () => {
  const vector = JSON.parse(
      readFileSync(
        new URL('../../../tests/fixtures/structure-aligned-split.json', import.meta.url),
      ),
    ),
    fixed = { ...vector.fixed, migration: vector.migration };
  fixed.input_digest = hash(canonical(fixed)).toString('base64');
  const original = canonical(fixed),
    context = { operation_id: 'aligned-confirm-original', target: { document_id: '1' } },
    prepared = {
      authority: { reference: vector.input.operation_id, digest: fixed.input_digest },
      branches: [],
    },
    targets = fixed.migration.changes[0].targets,
    mainUnits = [],
    transition = {
      preview_cut_ref: prepared.authority.reference,
      preview_digest: prepared.authority.digest,
      branches: targets.map((target, i) => {
        const unit = String(20 + i),
          branch = String(50 + i),
          room = String(60 + i),
          version = String(30 + i),
          lineage = {
            lineage_id: String(40 + i),
            base_manifest_id: '10',
            base_version_id: version,
            body: draftBody(target.main_body),
            provenance: { accepted_captures: [], block_sources: [] },
          },
          cut = { room_id: room, workspace_epoch: '1', durable_seq: '0' },
          content = fixedDraft(
            target.draft_body,
            {
              ...context,
              target: {
                ...context.target,
                unit_id: unit,
                structure_id: '11',
                branch_id: branch,
                workspace_epoch: '1',
              },
            },
            cut,
            lineage,
            context.operation_id + ':' + target.key,
          );
        lineage.provenance = JSON.parse(Buffer.from(content.provenance_json_utf8, 'base64'));
        mainUnits.push({
          unit_id: unit,
          content_version_id: version,
          content: { body_json_utf8: Buffer.from(canonical(target.main_body)) },
        });
        return {
          candidate_key: target.key,
          unit_id: unit,
          branch_id: branch,
          room_id: room,
          epoch: '1',
          lineage_id: lineage.lineage_id,
          content,
          lineage_context: lineage,
        };
      }),
    },
    checkpoints = [],
    lineages = [];
  // 只替换仓储与检查点I/O，实际安装用例执行内容/谱系校验及ProseMirror编解码，不接触真实房间。
  const connection = {
    async query({ sql }, args) {
      if (sql.startsWith('SELECT payload FROM durable_task WHERE kind=?'))
        return [[{ payload: args[0] === 'STRUCTURE_PREPARATION' ? prepared : fixed }]];
      if (sql.startsWith('SELECT * FROM room WHERE id=')) return [[]];
      if (sql.startsWith('INSERT INTO room(')) return [{ affectedRows: 1 }];
      if (sql.includes("VALUES('ROOM_LINEAGE_CONTEXT'")) {
        lineages.push(JSON.parse(args[1]));
        return [{ affectedRows: 1 }];
      }
      assert.fail('unexpected installation query: ' + sql);
    },
  };
  const installer = new StructureInstallations({
    async checkpoint(_connection, _room, doc, seq) {
      assert.equal(seq, '0');
      checkpoints.push(normalizeContent(fromPM(yDocToProsemirrorJSON(doc))));
    },
  });
  const cuts = await installer.install(connection, context, transition, mainUnits);
  assert.equal(cuts.length, targets.length);
  assert.deepEqual(
    checkpoints,
    targets.map((target) => target.draft_body),
  );
  assert.deepEqual(
    lineages.map((lineage) => lineage.body),
    targets.map((target) => target.main_body),
  );
  assert.match(canonical(checkpoints[0]), /支线独立改动|尚未审核追加/);
  assert.doesNotMatch(canonical(lineages[0].body), /支线独立改动|尚未审核追加/);
  assert.deepEqual(transition.branches[1].content.resource_ids, ['9007199254740997']);
  assert.deepEqual(resourceIds(checkpoints[1]), ['9007199254740997']);
  assert.equal(canonical(fixed), original);
});

test('join reconstructs stored boundary nodes and independently preserves MAIN and draft edits', () => {
  const main = markdown('## 第一节\n\n已接纳一\n\n## 第二节\n\n已接纳二');
  const draft = structuredClone(main);
  draft.content.push({
    type: 'paragraph',
    attrs: { id: 'join-S2' },
    content: [{ type: 'text', text: '待审核追加' }],
  });
  const pieces = splitChapterDrafts(main, draft),
    fixed = fixture(main);
  fixed.source.changes = [{ type: 'JOIN_CHAPTER', chapter_id: '3' }];
  fixed.source.structure.entries = [
    {
      node_id: '3',
      chapter_id: '3',
      kind: 'CHAPTER',
      title: '整章',
      sort_order: 1,
      split_level2: true,
    },
    ...pieces.map((p, i) => ({
      node_id: String(10 + i),
      parent_node_id: '3',
      chapter_id: '3',
      unit_id: String(20 + i),
      kind: p.kind,
      title: p.title,
      sort_order: i + 1,
      split_level2: false,
    })),
  ];
  fixed.source.unit_boundaries = pieces.map((p, i) => ({
    unit_id: String(20 + i),
    heading: p.heading,
    empty_block_id: p.emptyBlockId,
  }));
  fixed.source.drafts = pieces.map((p, i) => ({
    unit_id: String(20 + i),
    branch_id: String(30 + i),
    workspace_epoch: '1',
  }));
  fixed.main_units = pieces.map((p, i) => content(p.main, String(20 + i)));
  fixed.drafts = pieces.map((p, i) => content(p.draft, String(20 + i), String(30 + i)));
  const change = buildStructureMigration(fixed).changes[0];
  assert.deepEqual(change.source_units, ['20', '21']);
  assert.equal(change.targets.length, 1);
  assert.equal(canonical(change.targets[0].main_body), canonical(main));
  assert.equal(canonical(change.targets[0].draft_body), canonical(draft));
  assert.deepEqual(
    change.targets[0].source_ranges.map((v) => v.source_unit_id),
    ['20', '21'],
  );
  // 旧节以后再拆分时，授权证明需要合章中的实际区间，不能凭标题或仍存的块猜测。
  assert.deepEqual(
    change.targets[0].source_ranges.map((v) => [
      v.target_scope,
      v.target_start_boundary,
      v.target_end_boundary,
    ]),
    [
      ['INTERVAL', pieces[0].heading.attrs.id, pieces[1].heading.attrs.id],
      ['INTERVAL', pieces[1].heading.attrs.id, null],
    ],
  );
  const missing = structuredClone(fixed);
  missing.source.unit_boundaries.pop();
  assert.throws(() => buildStructureMigration(missing), /STRUCTURE_BOUNDARY_UNKNOWN/);
});

test('removal retains exact retired scope without inventing a replacement unit', () => {
  const fixed = fixture(markdown('整章正文'));
  fixed.source.changes = [{ type: 'REMOVE', chapter_id: '3' }];
  const change = buildStructureMigration(fixed).changes[0];
  assert.deepEqual(change.source_units, ['4']);
  assert.deepEqual(change.targets, []);
  assert.equal(fixed.main_units.length, 1);
  assert.equal(fixed.drafts.length, 1);
});

test('candidate generation rejects changed draft boundaries, body tampering and incomplete scope', () => {
  const main = markdown('## 第一节\n\n原文'),
    draft = structuredClone(main);
  draft.content[0].attrs.id = 'new-boundary';
  assert.throws(() => buildStructureMigration(fixture(main, draft)), /DRAFT_STRUCTURE_CHANGED/);
  const tampered = fixture(main);
  tampered.main_units[0].body_json_utf8 = Buffer.from(canonical(draft)).toString('base64');
  assert.throws(() => buildStructureMigration(tampered), /STRUCTURE_CONTENT_UNKNOWN/);
  const missing = fixture(main);
  missing.drafts = [];
  assert.throws(() => buildStructureMigration(missing), /STRUCTURE_SCOPE_UNKNOWN/);
  const foreign = fixture(main);
  foreign.drafts[0].document_id = '99';
  assert.throws(() => buildStructureMigration(foreign), /STRUCTURE_CONTENT_UNKNOWN/);
});

test('overlapping range actions and unexpected rooms cannot produce ambiguous lineage', () => {
  const fixed = fixture(markdown('## 小节\n\n正文'));
  fixed.source.changes.push({ type: 'REMOVE', chapter_id: '3' });
  assert.throws(() => buildStructureMigration(fixed), /STRUCTURE_RANGE_OVERLAP/);
  const extra = fixture(markdown('## 小节\n\n正文'));
  extra.drafts.push({ ...extra.drafts[0], unit_id: '99' });
  assert.throws(() => buildStructureMigration(extra), /STRUCTURE_SCOPE_UNKNOWN/);
});

test('joining retains MAIN heading fonts while draft uses its permitted formatting', () => {
  const accepted = markdown('## 标题\n\n正文');
  accepted.content[0].content[0].marks = [{ type: 'font', attrs: { family: 'SimSun' } }];
  const pending = structuredClone(accepted);
  delete pending.content[0].content[0].marks;
  const pieces = splitChapterDrafts(accepted, pending),
    fixed = fixture(accepted, pending);
  fixed.source.changes = [{ type: 'JOIN_CHAPTER', chapter_id: '3' }];
  fixed.source.structure.entries = [
    {
      node_id: '3',
      chapter_id: '3',
      kind: 'CHAPTER',
      title: '整章',
      sort_order: 1,
      split_level2: true,
    },
    {
      node_id: '10',
      parent_node_id: '3',
      chapter_id: '3',
      unit_id: '4',
      kind: 'SECTION',
      title: pieces[0].title,
      sort_order: 1,
      split_level2: false,
    },
  ];
  fixed.source.unit_boundaries = [{ unit_id: '4', heading: pieces[0].heading }];
  fixed.main_units = [content(pieces[0].main, '4')];
  fixed.drafts = [content(pieces[0].draft, '4', '5')];
  const target = buildStructureMigration(fixed).changes[0].targets[0];
  assert.equal(canonical(target.main_body), canonical(accepted));
  assert.equal(canonical(target.draft_body), canonical(pending));
});
