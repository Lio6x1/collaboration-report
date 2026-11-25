# packages/content/index.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/360-index.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`atomFields`、`toPM`、`fromPM`、`canonical`、`normalizeContent`、`resourceIds`、`inspectDocument`、`prepareCandidate`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { Schema } from 'prosemirror-model';
import { schema as basic } from 'prosemirror-schema-basic';
import { addListNodes } from 'prosemirror-schema-list';
import { tableNodes, TableMap } from 'prosemirror-tables';
import * as Y from 'yjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import validateSchema from './validate.mjs';

const atoms = new Set(['formula', 'formula_block', 'image']);
const atomAttributes = {
  atomStrong: { default: false },
  atomEm: { default: false },
  atomCode: { default: false },
  atomStrike: { default: false },
  atomLink: { default: null },
};
const invalid = () => Object.assign(new Error('INVALID_CONTENT'), { code: 'INVALID_CONTENT' });
let nodes = addListNodes(basic.spec.nodes, 'paragraph block*', 'block').append(
  tableNodes({ tableGroup: 'block', cellContent: 'block+' }),
);
// 段落格式存为独立属性，协作时无需用正文空格伪造缩进；剪贴板只接收固定枚举。
const paragraphDOM = (n) => ({
  style: `text-align:${n.attrs.align};text-indent:${n.attrs.firstLineIndent ? '2em' : '0'}`,
});
const paragraphFromDOM = (element) => ({
  align: ['left', 'center', 'right'].includes(element.style.textAlign)
    ? element.style.textAlign
    : 'left',
  firstLineIndent: element.style.textIndent === '2em',
});
for (const name of ['paragraph', 'heading']) {
  const spec = nodes.get(name);
  nodes = nodes.update(name, {
    ...spec,
    attrs: { ...spec.attrs, align: { default: 'left' }, firstLineIndent: { default: false } },
    toDOM: (n) => [name === 'heading' ? `h${n.attrs.level}` : 'p', paragraphDOM(n), 0],
    parseDOM: spec.parseDOM.map((rule) => ({
      ...rule,
      getAttrs: (element) => ({ ...rule.attrs, ...paragraphFromDOM(element) }),
    })),
  });
}
const atomStyle = (n) =>
  [
    n.attrs.atomStrong ? 'font-weight:700' : '',
    n.attrs.atomEm ? 'font-style:italic' : '',
    n.attrs.atomStrike ? 'text-decoration:line-through' : '',
  ]
    .filter(Boolean)
    .join(';');
// 浏览器剪贴板走DOM而非JSON，必须显式携带每项原子属性，CSS不足以表达code/link。
const atomDOM = (n) => ({
  'data-atom-strong': String(n.attrs.atomStrong),
  'data-atom-em': String(n.attrs.atomEm),
  'data-atom-code': String(n.attrs.atomCode),
  'data-atom-strike': String(n.attrs.atomStrike),
  'data-atom-link': n.attrs.atomLink?.href ?? null,
  'data-atom-link-title': n.attrs.atomLink?.title ?? null,
  style: atomStyle(n),
});
const atomFromDOM = (e) => ({
  atomStrong: e.getAttribute('data-atom-strong') === 'true',
  atomEm: e.getAttribute('data-atom-em') === 'true',
  atomCode: e.getAttribute('data-atom-code') === 'true',
  atomStrike: e.getAttribute('data-atom-strike') === 'true',
  atomLink: /^(https?:\/\/|mailto:)/.test(e.getAttribute('data-atom-link') ?? '')
    ? { href: e.getAttribute('data-atom-link'), title: e.getAttribute('data-atom-link-title') }
    : null,
});
nodes = nodes.addToEnd('formula', {
  inline: true,
  group: 'inline',
  atom: true,
  marks: '',
  attrs: { latex: { default: '' }, ...atomAttributes },
  toDOM: (n) => ['span', { 'data-formula': n.attrs.latex, ...atomDOM(n) }, n.attrs.latex],
  parseDOM: [
    {
      tag: 'span[data-formula]',
      getAttrs: (e) => ({ latex: e.getAttribute('data-formula'), ...atomFromDOM(e) }),
    },
  ],
});
nodes = nodes.addToEnd('formula_block', {
  group: 'block',
  atom: true,
  marks: '',
  attrs: { latex: { default: '' }, ...atomAttributes },
  toDOM: (n) => ['div', { 'data-formula-block': n.attrs.latex, ...atomDOM(n) }, n.attrs.latex],
  parseDOM: [
    {
      tag: 'div[data-formula-block]',
      getAttrs: (e) => ({ latex: e.getAttribute('data-formula-block'), ...atomFromDOM(e) }),
    },
  ],
});
nodes = nodes.update('image', {
  ...nodes.get('image'),
  marks: '',
  attrs: {
    src: { default: '' },
    resourceId: { default: null },
    alt: { default: null },
    title: { default: null },
    width: { default: null },
    height: { default: null },
    ...atomAttributes,
  },
  toDOM: (n) => [
    'img',
    {
      src: n.attrs.src,
      'data-resource-id': n.attrs.resourceId,
      alt: n.attrs.alt,
      title: n.attrs.title,
      width: n.attrs.width,
      height: n.attrs.height,
      ...atomDOM(n),
    },
  ],
  parseDOM: [
    {
      tag: 'img[data-resource-id]',
      getAttrs: (e) => ({
        src: '/controlled/' + e.getAttribute('data-resource-id'),
        resourceId: e.getAttribute('data-resource-id'),
        alt: e.getAttribute('alt'),
        title: e.getAttribute('title'),
        width: e.hasAttribute('width') ? Number(e.getAttribute('width')) : null,
        height: e.hasAttribute('height') ? Number(e.getAttribute('height')) : null,
        ...atomFromDOM(e),
      }),
    },
  ],
});
nodes = nodes.update('code_block', {
  ...nodes.get('code_block'),
  attrs: { language: { default: null } },
  toDOM: (n) => ['pre', { 'data-code-language': n.attrs.language }, ['code', 0]],
  parseDOM: [
    {
      tag: 'pre',
      preserveWhitespace: 'full',
      getAttrs: (e) => ({ language: e.getAttribute('data-code-language') }),
    },
  ],
});
// 块身份用于来源、复制和三方合并。这里不自动修复缺失ID，恶意增量必须明确被拒绝。
nodes.forEach((name, spec) => {
  if (name !== 'doc' && name !== 'text' && !spec.inline)
    nodes = nodes.update(name, { ...spec, attrs: { ...spec.attrs, id: { default: null } } });
});
export const schema = new Schema({
  nodes,
  marks: basic.spec.marks
    .addToEnd('strike', { toDOM: () => ['s', 0], parseDOM: [{ tag: 's' }, { tag: 'del' }] })
    .addToEnd('font', {
      attrs: { family: {} },
      toDOM: (m) => ['span', { style: `font-family:${m.attrs.family}` }, 0],
    }),
});

// 原版y-prosemirror对原子mark不保证保真；属性分别保存，避免强调/删除线并发整包覆盖。
export function atomFields(marks = []) {
  if (marks.some((m) => !['strong', 'em', 'code', 'strike', 'link'].includes(m.type)))
    throw invalid();
  return {
    atomStrong: marks.some((m) => m.type === 'strong'),
    atomEm: marks.some((m) => m.type === 'em'),
    atomCode: marks.some((m) => m.type === 'code'),
    atomStrike: marks.some((m) => m.type === 'strike'),
    atomLink: structuredClone(marks.find((m) => m.type === 'link')?.attrs ?? null),
  };
}
export function toPM(value) {
  const n = structuredClone(value);
  if (n.type === 'image') n.attrs.src = `/controlled/${n.attrs.resourceId}`;
  if (n.content) n.content = n.content.map(toPM);
  return n;
}
export function fromPM(value) {
  const n = structuredClone(value);
  // PM默认值只负责编辑器渲染；不写回旧正文，否则单纯打开/刷新也会改变固定版本哈希。
  if (['paragraph', 'heading'].includes(n.type) && n.attrs) {
    if (n.attrs.align === 'left') delete n.attrs.align;
    if (n.attrs.firstLineIndent === false) delete n.attrs.firstLineIndent;
  }
  // PM/Yjs 省略空可编辑块的 content；恢复规范业务形式，避免未编辑正文产生新的哈希。
  if (!n.content && ['paragraph', 'heading', 'code_block'].includes(n.type)) n.content = [];
  // 绑定层为无属性mark补空对象；业务schema不接受这项传输冗余，非空未知属性仍交给校验拒绝。
  for (const mark of n.marks ?? [])
    if (mark.attrs && Object.keys(mark.attrs).length === 0) delete mark.attrs;
  if (atoms.has(n.type)) {
    if (n.marks?.length) {
      const fields = atomFields(n.marks);
      for (const [k, v] of Object.entries(fields)) if (v !== false && v !== null) n.attrs[k] = v;
      delete n.marks;
    }
    if (n.type === 'image') delete n.attrs.src;
  }
  if (n.content) n.content = n.content.map(fromPM);
  return n;
}
function inspectValue(value, depth = 0) {
  if (depth > 128) throw invalid();
  if (typeof value === 'string') {
    for (let i = 0; i < value.length; i++) {
      const c = value.charCodeAt(i);
      if (c >= 0xd800 && c <= 0xdbff) {
        const next = value.charCodeAt(++i);
        if (!(next >= 0xdc00 && next <= 0xdfff)) throw invalid();
      } else if (c >= 0xdc00 && c <= 0xdfff) throw invalid();
    }
  } else if (typeof value === 'number') {
    if (!Number.isInteger(value) || Math.abs(value) > 2147483647) throw invalid();
  } else if (value !== null && typeof value === 'object') {
    for (const [key, child] of Object.entries(value)) {
      inspectValue(key, depth + 1);
      inspectValue(child, depth + 1);
    }
  } else if (value !== null && typeof value !== 'boolean') throw invalid();
}
export function canonical(value) {
  inspectValue(value);
  // 文本空白、数组顺序和Unicode原样保留；固定转义两个分隔符，Node/Go共用字节向量。
  return JSON.stringify(value, (_, v) =>
    v && typeof v === 'object' && !Array.isArray(v)
      ? Object.fromEntries(
          Object.keys(v)
            .sort()
            .map((k) => [k, v[k]]),
        )
      : v,
  )
    .replaceAll('\u2028', '\\u2028')
    .replaceAll('\u2029', '\\u2029');
}
export function normalizeContent(value, { workspaceKind = 'BRANCH' } = {}) {
  try {
    inspectValue(value);
    const body = structuredClone(value);
    let count = 0;
    const ids = new Set();
    function inspect(n, depth = 0) {
      if (
        depth > 64 ||
        ++count > 100000 ||
        !n ||
        typeof n !== 'object' ||
        typeof n.type !== 'string'
      )
        throw invalid();
      if (workspaceKind === 'BRANCH' && (n.marks ?? []).some((m) => m.type === 'font'))
        throw invalid();
      if (atoms.has(n.type))
        n.attrs = {
          ...Object.fromEntries(Object.entries(atomAttributes).map(([k, v]) => [k, v.default])),
          ...n.attrs,
        };
      if (n.type === 'code_block') n.attrs = { language: null, ...n.attrs };
      if (n.type === 'table_cell' || n.type === 'table_header')
        n.attrs = { colwidth: null, ...n.attrs };
      if (n.type === 'image') {
        n.attrs = { alt: null, title: null, width: null, height: null, ...n.attrs };
        if (
          !/^[1-9][0-9]{0,19}$/.test(n.attrs.resourceId) ||
          BigInt(n.attrs.resourceId) > 18446744073709551615n
        )
          throw invalid();
      }
      const spec = schema.nodes[n.type];
      if (!spec) throw invalid();
      if (n.type !== 'doc' && n.type !== 'text' && !spec.isInline) {
        const id = n.attrs?.id;
        if (typeof id !== 'string' || ids.has(id)) throw invalid();
        ids.add(id);
      }
      for (const c of n.content ?? []) inspect(c, depth + 1);
    }
    inspect(body);
    if (!validateSchema(body)) throw invalid();
    const parsed = schema.nodeFromJSON(toPM(body));
    parsed.check();
    parsed.descendants((n) => {
      if (n.type.name === 'table' && TableMap.get(n).problems?.length) throw invalid();
    });
    if (new TextEncoder().encode(canonical(body)).length > 16 * 1024 * 1024) throw invalid();
    return body;
  } catch {
    throw invalid();
  }
}
export function resourceIds(value) {
  const ids = new Set();
  const walk = (n) => {
    if (n.type === 'image') ids.add(n.attrs.resourceId);
    for (const child of n.content ?? []) walk(child);
  };
  walk(value);
  return [...ids].sort((a, b) => (BigInt(a) < BigInt(b) ? -1 : BigInt(a) > BigInt(b) ? 1 : 0));
}
// 只读检查已恢复的私有文档；复用候选的全部规则，不重复编码/克隆Yjs状态。
// 文档生命周期仍归调用方所有，校验成功或失败都不安装更新、不销毁传入文档。
export function inspectDocument(doc, options) {
  try {
    if (doc.store.pendingStructs || doc.store.pendingDs)
      throw Object.assign(new Error('DEPENDENCIES_MISSING'), { code: 'DEPENDENCIES_MISSING' });
    const body = normalizeContent(fromPM(yDocToProsemirrorJSON(doc)), options);
    return { body, resources: resourceIds(body), canonical: canonical(body) };
  } catch (err) {
    if (err.code === 'DEPENDENCIES_MISSING') throw err;
    throw invalid();
  }
}
export function prepareCandidate(host, update, options) {
  // 推测候选独立于公共房间；校验、资源保护、事务成功之前不能向host applyUpdate。
  const doc = new Y.Doc();
  try {
    Y.applyUpdate(doc, Y.encodeStateAsUpdate(host));
    Y.applyUpdate(doc, update);
    return { doc, ...inspectDocument(doc, options) };
  } catch (err) {
    doc.destroy();
    if (err.code === 'DEPENDENCIES_MISSING') throw err;
    throw invalid();
  }
}
````

UTF-8/LF全文SHA-256：`c2add5e3757e0dcd5e17f063ef77843cc66f715d19cf251597f603e57bad6fba`。[参考实现](../../../../packages/content/index.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
