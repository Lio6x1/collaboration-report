import test from 'node:test';
import assert from 'node:assert/strict';
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import * as content from '../../../packages/content/index.mjs';

const paragraph = (id, text = '') => ({
  type: 'paragraph',
  attrs: { id },
  content: text ? [{ type: 'text', text }] : [],
});
const body = (...nodes) => ({ type: 'doc', content: nodes });
const open = (value) => prosemirrorJSONToYDoc(content.schema, content.toPM(value));
const inspect = (doc, options) => {
  assert.equal(typeof content.inspectDocument, 'function', '缺少无需克隆的共享完整校验入口');
  return content.inspectDocument(doc, options);
};

test('只读完整校验与隔离候选输出一致，保留正文、完整资源ID及规范字节且不改变host', () => {
  const host = open(
    body({
      ...paragraph('inspection-paragraph', ' 保留空白😀 '),
      content: [
        { type: 'text', text: ' 保留空白😀 ' },
        { type: 'image', attrs: { resourceId: '18446744073709551615' } },
        { type: 'image', attrs: { resourceId: '9007199254740993' } },
      ],
    }),
  );
  const before = Y.encodeStateAsUpdate(host),
    vector = Y.encodeStateVector(host);
  let candidate,
    destroyed = false,
    updates = 0;
  host.on('destroy', () => {
    destroyed = true;
  });
  host.on('update', () => {
    updates++;
  });
  try {
    candidate = content.prepareCandidate(host, new Uint8Array([0, 0]));
    const checked = inspect(host);
    assert.deepEqual(checked, {
      body: candidate.body,
      resources: candidate.resources,
      canonical: candidate.canonical,
    });
    assert.deepEqual(checked.resources, ['9007199254740993', '18446744073709551615']);
    assert.equal(checked.body.content[0].content[0].text, ' 保留空白😀 ');
    assert.deepEqual(Y.encodeStateAsUpdate(host), before);
    assert.deepEqual(Y.encodeStateVector(host), vector);
    assert.equal(destroyed, false);
    assert.equal(updates, 0);
    assert.notEqual(candidate.doc, host, '候选仍必须与host隔离');
  } finally {
    candidate?.doc.destroy();
    host.destroy();
  }
});

for (const missing of ['structs', 'deletes']) {
  test(`真实Yjs缺失${missing}依赖仍以DEPENDENCIES_MISSING拒绝，不能漏检或改变host`, () => {
    const source = open(body(paragraph('dependency-paragraph', '合法正文')));
    const host = new Y.Doc();
    try {
      Y.applyUpdate(host, Y.encodeStateAsUpdate(source));
      source.getText('dependency').insert(0, 'A');
      const vector = Y.encodeStateVector(source);
      if (missing === 'structs') source.getText('dependency').insert(1, 'B');
      else source.getText('dependency').delete(0, 1);
      // 不交付A，只交付后续B或删除集：pending来自真实编码更新，不直接伪造store字段。
      Y.applyUpdate(host, Y.encodeStateAsUpdate(source, vector));
      assert.ok(missing === 'structs' ? host.store.pendingStructs : host.store.pendingDs);
      const before = Y.encodeStateAsUpdate(host);
      assert.throws(() => inspect(host), { code: 'DEPENDENCIES_MISSING' });
      assert.deepEqual(Y.encodeStateAsUpdate(host), before);
      assert.throws(() => content.prepareCandidate(host, new Uint8Array([0, 0])), {
        code: 'DEPENDENCIES_MISSING',
      });
    } finally {
      source.destroy();
      host.destroy();
    }
  });
}

test('只读入口仍拒绝重复块ID、非法表格几何及BRANCH字体，MAIN规则保持一致', () => {
  const invalid = [
    body(paragraph('duplicate'), paragraph('duplicate')),
    body({
      type: 'table',
      attrs: { id: 'table' },
      content: [
        {
          type: 'table_row',
          attrs: { id: 'row' },
          content: [
            {
              type: 'table_cell',
              attrs: { id: 'cell', colspan: 1, rowspan: 2, colwidth: null },
              content: [paragraph('cell-paragraph')],
            },
          ],
        },
      ],
    }),
    body({
      ...paragraph('font'),
      content: [
        {
          type: 'text',
          text: '正文',
          marks: [{ type: 'font', attrs: { family: 'SimSun' } }],
        },
      ],
    }),
  ];
  for (const value of invalid) {
    const host = open(value),
      before = Y.encodeStateAsUpdate(host);
    try {
      assert.throws(() => inspect(host, { workspaceKind: 'BRANCH' }), { code: 'INVALID_CONTENT' });
      assert.deepEqual(Y.encodeStateAsUpdate(host), before);
    } finally {
      host.destroy();
    }
  }
  const host = open(invalid[2]);
  let candidate;
  try {
    candidate = content.prepareCandidate(host, new Uint8Array([0, 0]), { workspaceKind: 'MAIN' });
    assert.deepEqual(inspect(host, { workspaceKind: 'MAIN' }).body, candidate.body);
  } finally {
    candidate?.doc.destroy();
    host.destroy();
  }
});

test('只读入口仍执行16MiB完整正文限制，不能通过省掉克隆放宽恢复校验', () => {
  // 各块都小于单text的一百万字符上限，独立ID/合法结构只因整篇超过16MiB而拒绝。
  const text = 'x'.repeat(999999),
    blocks = Array.from({ length: 17 }, (_, index) => paragraph(`oversized-${index}`, text));
  assert.doesNotThrow(() => content.normalizeContent(body(blocks[0])));
  assert.ok(Buffer.byteLength(content.canonical(body(...blocks))) > 16 * 1024 * 1024);
  const host = open(body(...blocks));
  try {
    assert.throws(() => inspect(host), { code: 'INVALID_CONTENT' });
  } finally {
    host.destroy();
  }
});

test('共享校验后的prepareCandidate仍隔离真实非法更新，不安装到host', () => {
  const host = open(body(paragraph('saved', '已保存'))),
    client = new Y.Doc();
  try {
    Y.applyUpdate(client, Y.encodeStateAsUpdate(host));
    const duplicate = new Y.XmlElement('paragraph');
    duplicate.setAttribute('id', 'saved');
    client.getXmlFragment('prosemirror').push([duplicate]);
    const before = Y.encodeStateAsUpdate(host);
    assert.throws(
      () =>
        content.prepareCandidate(host, Y.encodeStateAsUpdate(client, Y.encodeStateVector(host))),
      {
        code: 'INVALID_CONTENT',
      },
    );
    assert.deepEqual(Y.encodeStateAsUpdate(host), before);
  } finally {
    client.destroy();
    host.destroy();
  }
});
