# 分步编写 `packages/content/index.mjs`

先按 [C01 · 正文契约、隔离候选与连续日志恢复](../lessons/collaboration/01-content-replay.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/360-index.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义前后端共用的正文 schema，并从真实 Y.Doc 产生已校验的隔离候选。

浏览器能渲染并不代表正文合法；服务端必须限制节点、表格、资源ID、Unicode和Yjs依赖，再计算跨Go/Node一致的哈希。

写之前先按这条执行路线理解各段的连接：

1. schema扩展列表、表格、公式和受控图片，块id参与后续合并
2. normalizeContent补齐允许的默认属性，检查唯一块id、结构深度、节点数、schema和TableMap
3. canonical按对象键排序保留数组/文字顺序，统一U+2028/U+2029转义
4. prepareCandidate复制host状态后应用update；inspectDocument拒绝pendingStructs/pendingDs再提取正文和资源

本文件需要守住的边界：

- 不要对共享host先applyUpdate再校验；失败候选可能污染后续用户
- 图片只接受受控resourceId，src是编辑器展示字段；大整数ID必须保持字符串
- 调用方必须销毁成功返回的candidate.doc，函数只销毁失败候选

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/content' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/content/index.mjs') -and (Get-Item -LiteralPath 'packages/content/index.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/content/index.mjs')) { New-Item -ItemType File -Path 'packages/content/index.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:346 -->
````javascript
import { Schema } from 'prosemirror-model';
import { schema as basic } from 'prosemirror-schema-basic';
import { addListNodes } from 'prosemirror-schema-list';
import { tableNodes, TableMap } from 'prosemirror-tables';
import * as Y from 'yjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import validateSchema from './validate.mjs';

````

## 第 2 步：atoms

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`const atoms = new Set(['formula', 'formula_block', 'image']);`。

<!-- write-step: 2 bytes=346:408 -->
````javascript
const atoms = new Set(['formula', 'formula_block', 'image']);
````

## 第 3 步：atomAttributes

插入位置：第 2 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`const atomAttributes = {`。

<!-- write-step: 3 bytes=408:597 -->
````javascript
const atomAttributes = {
  atomStrong: { default: false },
  atomEm: { default: false },
  atomCode: { default: false },
  atomStrike: { default: false },
  atomLink: { default: null },
};
````

## 第 4 步：invalid

插入位置：第 3 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`const invalid = () => `。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>Object.assign(new Error(&#x27;INVALID_CONTENT&#x27;), { code: &#x27;INVALID_CONTENT&#x27; })</code> |

<!-- write-step: 4 bytes=597:693 -->
````javascript
const invalid = () => Object.assign(new Error('INVALID_CONTENT'), { code: 'INVALID_CONTENT' });
````

## 第 5 步：nodes

插入位置：第 4 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`let nodes = addListNodes(basic.spec.nodes, 'paragraph block*', 'block').append(`。

<!-- write-step: 5 bytes=693:838 -->
````javascript
let nodes = addListNodes(basic.spec.nodes, 'paragraph block*', 'block').append(
  tableNodes({ tableGroup: 'block', cellContent: 'block+' }),
);
````

## 第 6 步：paragraphDOM

插入位置：第 5 步代码之后，紧接追加；完成稿约第 21 行。

段落格式存为独立属性，协作时无需用正文空格伪造缩进；剪贴板只接收固定枚举。

本步声明：`const paragraphDOM = (n) => (`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>n</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>n.attrs</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>{   style: `text-align:${n.attrs.align};text-indent:${n.attrs.firstLineIndent ? &#x27;2em&#x27; : &#x27;0&#x27;}`, }</code> |

<!-- write-step: 6 bytes=838:1081 -->
````javascript
// 段落格式存为独立属性，协作时无需用正文空格伪造缩进；剪贴板只接收固定枚举。
const paragraphDOM = (n) => ({
  style: `text-align:${n.attrs.align};text-indent:${n.attrs.firstLineIndent ? '2em' : '0'}`,
});
````

## 第 7 步：paragraphFromDOM

插入位置：第 6 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`const paragraphFromDOM = (element) => (`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>element</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>element.style</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>{   align: &#91;&#x27;left&#x27;, &#x27;center&#x27;, &#x27;right&#x27;&#93;.includes(element.style.textAlign)     ? element.style.textAlign     : &#x27;left&#x27;,   firstLineIndent: element.style.textIndent === &#x27;2em&#x27;, }</code> |

<!-- write-step: 7 bytes=1081:1296 -->
````javascript
const paragraphFromDOM = (element) => ({
  align: ['left', 'center', 'right'].includes(element.style.textAlign)
    ? element.style.textAlign
    : 'left',
  firstLineIndent: element.style.textIndent === '2em',
});
````

## 第 8 步：statement-1222

插入位置：第 7 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`for (const name of ['paragraph', 'heading']) `。

<!-- write-step: 8 bytes=1296:1759 -->
````javascript
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
````

## 第 9 步：atomStyle

插入位置：第 8 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`const atomStyle = (n) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>n</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>n.attrs</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>&#91;     n.attrs.atomStrong ? &#x27;font-weight:700&#x27; : &#x27;&#x27;,     n.attrs.atomEm ? &#x27;font-style:italic&#x27; : &#x27;&#x27;,     n.attrs.atomStrike ? &#x27;text-decoration:line-through&#x27; : &#x27;&#x27;,   &#93;     .filter(Boolean)     .join(&#x27;;&#x27;)</code> |

<!-- write-step: 9 bytes=1759:1987 -->
````javascript
const atomStyle = (n) =>
  [
    n.attrs.atomStrong ? 'font-weight:700' : '',
    n.attrs.atomEm ? 'font-style:italic' : '',
    n.attrs.atomStrike ? 'text-decoration:line-through' : '',
  ]
    .filter(Boolean)
    .join(';');
````

## 第 10 步：atomDOM

插入位置：第 9 步代码之后，紧接追加；完成稿约第 51 行。

浏览器剪贴板走DOM而非JSON，必须显式携带每项原子属性，CSS不足以表达code/link。

本步声明：`const atomDOM = (n) => (`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>n</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>n.attrs</code>；<code>atomStyle(n)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>{   &#x27;data-atom-strong&#x27;: String(n.attrs.atomStrong),   &#x27;data-atom-em&#x27;: String(n.attrs.atomEm),   &#x27;data-atom-code&#x27;: String(n.attrs.atomCode),   &#x27;data-atom-strike&#x27;: String(n.attrs.ato …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 10 bytes=1987:2449 -->
````javascript
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
````

## 第 11 步：atomFromDOM

插入位置：第 10 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`const atomFromDOM = (e) => (`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>e</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>e.getAttribute</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>{   atomStrong: e.getAttribute(&#x27;data-atom-strong&#x27;) === &#x27;true&#x27;,   atomEm: e.getAttribute(&#x27;data-atom-em&#x27;) === &#x27;true&#x27;,   atomCode: e.getAttribute(&#x27;data-atom-code&#x27;) === &#x27;true&#x27;,   atomS …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 11 bytes=2449:2905 -->
````javascript
const atomFromDOM = (e) => ({
  atomStrong: e.getAttribute('data-atom-strong') === 'true',
  atomEm: e.getAttribute('data-atom-em') === 'true',
  atomCode: e.getAttribute('data-atom-code') === 'true',
  atomStrike: e.getAttribute('data-atom-strike') === 'true',
  atomLink: /^(https?:\/\/|mailto:)/.test(e.getAttribute('data-atom-link') ?? '')
    ? { href: e.getAttribute('data-atom-link'), title: e.getAttribute('data-atom-link-title') }
    : null,
});
````

## 第 12 步：statement-2773

插入位置：第 11 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`nodes = nodes.addToEnd('formula', {`。

<!-- write-step: 12 bytes=2905:3306 -->
````javascript
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
````

## 第 13 步：statement-3174

插入位置：第 12 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`nodes = nodes.addToEnd('formula_block', {`。

<!-- write-step: 13 bytes=3306:3712 -->
````javascript
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
````

## 第 14 步：statement-3580

插入位置：第 13 步代码之后，紧接追加；完成稿约第 97 行。

本步声明：`nodes = nodes.update('image', {`。

<!-- write-step: 14 bytes=3712:4747 -->
````javascript
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
````

## 第 15 步：statement-4615

插入位置：第 14 步代码之后，紧接追加；完成稿约第 136 行。

本步声明：`nodes = nodes.update('code_block', {`。

<!-- write-step: 15 bytes=4747:5103 -->
````javascript
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
````

## 第 16 步：statement-5015

插入位置：第 15 步代码之后，紧接追加；完成稿约第 148 行。

块身份用于来源、复制和三方合并。这里不自动修复缺失ID，恶意增量必须明确被拒绝。

本步声明：`nodes.forEach((name, spec) => {`。

<!-- write-step: 16 bytes=5103:5410 -->
````javascript
// 块身份用于来源、复制和三方合并。这里不自动修复缺失ID，恶意增量必须明确被拒绝。
nodes.forEach((name, spec) => {
  if (name !== 'doc' && name !== 'text' && !spec.inline)
    nodes = nodes.update(name, { ...spec, attrs: { ...spec.attrs, id: { default: null } } });
});
````

## 第 17 步：schema

插入位置：第 16 步代码之后，紧接追加；完成稿约第 153 行。

本步声明：`export const schema = new Schema({`。

<!-- write-step: 17 bytes=5410:5716 -->
````javascript
export const schema = new Schema({
  nodes,
  marks: basic.spec.marks
    .addToEnd('strike', { toDOM: () => ['s', 0], parseDOM: [{ tag: 's' }, { tag: 'del' }] })
    .addToEnd('font', {
      attrs: { family: {} },
      toDOM: (m) => ['span', { style: `font-family:${m.attrs.family}` }, 0],
    }),
});

````

## 第 18 步：atomFields

插入位置：第 17 步代码之后，紧接追加；完成稿约第 163 行。

原版y-prosemirror对原子mark不保证保真；属性分别保存，避免强调/删除线并发整包覆盖。

本步声明：`export function atomFields(marks = []) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>marks = &#91;&#93;</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>marks.some</code>；<code>marks.find</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     atomStrong: marks.some((m) =&gt; m.type === &#x27;strong&#x27;),     atomEm: marks.some((m) =&gt; m.type === &#x27;em&#x27;),     atomCode: marks.some((m) =&gt; m.type === &#x27;code&#x27;),     atomStrike: marks. …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>marks.some((m) =&gt; !&#91;&#x27;strong&#x27;, &#x27;em&#x27;, &#x27;code&#x27;, &#x27;strike&#x27;, &#x27;link&#x27;&#93;.includes(m.type))</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {     atomStrong: marks.some((m) =&gt; m.type === &#x27;strong&#x27;),     atomEm: marks.some((m) =&gt; m.type === &#x27;em&#x27;),     atomCode: marks.some((m) =&gt; m.type === &#x27;code&#x27;),     atomStrike: …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=5716:6297 -->
````javascript
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
````

## 第 19 步：toPM

插入位置：第 18 步代码之后，紧接追加；完成稿约第 175 行。

本步声明：`export function toPM(value) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>structuredClone(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>n</code> |

**跟着本段代码走一遍：**

- 检查 <code>n.type === &#x27;image&#x27;</code>。
- 检查 <code>n.content</code>。
- 此处通过 <code>return n;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=6297:6504 -->
````javascript
export function toPM(value) {
  const n = structuredClone(value);
  if (n.type === 'image') n.attrs.src = `/controlled/${n.attrs.resourceId}`;
  if (n.content) n.content = n.content.map(toPM);
  return n;
}
````

## 第 20 步：fromPM

插入位置：第 19 步代码之后，紧接追加；完成稿约第 181 行。

本步声明：`export function fromPM(value) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>structuredClone(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>n</code> |

**跟着本段代码走一遍：**

- 检查 <code>&#91;&#x27;paragraph&#x27;, &#x27;heading&#x27;&#93;.includes(n.type) &amp;&amp; n.attrs</code>。
- 检查 <code>n.attrs.align === &#x27;left&#x27;</code>。
- 检查 <code>n.attrs.firstLineIndent === false</code>。
- 检查 <code>!n.content &amp;&amp; &#91;&#x27;paragraph&#x27;, &#x27;heading&#x27;, &#x27;code_block&#x27;&#93;.includes(n.type)</code>。
- 检查 <code>mark.attrs &amp;&amp; Object.keys(mark.attrs).length === 0</code>。
- 检查 <code>atoms.has(n.type)</code>。
- 检查 <code>n.marks?.length</code>。
- 检查 <code>v !== false &amp;&amp; v !== null</code>。
- 检查 <code>n.type === &#x27;image&#x27;</code>。
- 检查 <code>n.content</code>。
- 此处通过 <code>return n;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=6504:7687 -->
````javascript
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
````

## 第 21 步：inspectValue

插入位置：第 20 步代码之后，紧接追加；完成稿约第 204 行。

本步声明：`function inspectValue(value, depth = 0) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>typeof value === &#x27;string&#x27;</code>；<code>value.length</code>；<code>value.charCodeAt</code>；<code>typeof value === &#x27;number&#x27;</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>depth = 0</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>depth &gt; 128</code>；<code>depth + 1</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。 |

**跟着本段代码走一遍：**

- 检查 <code>depth &gt; 128</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>typeof value === &#x27;string&#x27;</code>。
- 检查 <code>c &gt;= 0xd800 &amp;&amp; c &lt;= 0xdbff</code>。
- 检查 <code>!(next &gt;= 0xdc00 &amp;&amp; next &lt;= 0xdfff)</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>c &gt;= 0xdc00 &amp;&amp; c &lt;= 0xdfff</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>typeof value === &#x27;number&#x27;</code>。
- 检查 <code>!Number.isInteger(value) &#124;&#124; Math.abs(value) &gt; 2147483647</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>value !== null &amp;&amp; typeof value === &#x27;object&#x27;</code>。
- 检查 <code>value !== null &amp;&amp; typeof value !== &#x27;boolean&#x27;</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 21 bytes=7687:8499 -->
````javascript
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
````

## 第 22 步：canonical

插入位置：第 21 步代码之后，紧接追加；完成稿约第 223 行。

本步声明：`export function canonical(value) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>inspectValue(value)</code>；<code>JSON.stringify(value, (_, v) =&gt;     v &amp;&amp; typeof v === &#x27;object&#x27; &amp;&amp; !Array.isArray(v)       ? Object.fromEntries(           Object.keys(v)             .sort()             .map((k) =&gt; &#91;k, v&#91;k&#93;&#93;),         )       : v,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>JSON.stringify(value, (_, v) =&gt;     v &amp;&amp; typeof v === &#x27;object&#x27; &amp;&amp; !Array.isArray(v)       ? Object.fromEntries(           Object.keys(v)             .sort()             .map((k) =&gt; …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return JSON.stringify(value, (_, v) =&gt;     v &amp;&amp; typeof v === &#x27;object&#x27; &amp;&amp; !Array.isArray(v)       ? Object.fromEntries(           Object.keys(v)             .sort()             .map …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=8499:8977 -->
````javascript
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
````

## 第 23 步：normalizeContent

插入位置：第 22 步代码之后，紧接追加；完成稿约第 238 行。

本步声明：`export function normalizeContent(value, { workspaceKind = 'BRANCH' } = {}) `。

**这一段为什么这样写：** 把允许的正文规范化成可稳定散列的业务JSON。

1. 先检查值类型/Unicode，再复制正文
2. 补默认属性后校验唯一块id、表格布局和16MiB规范字节限制
3. 任一失败统一为INVALID_CONTENT


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 待校验的正文JSON，根节点必须符合doc schema<br>在此声明中的实际使用：<code>inspectValue(value)</code>；<code>structuredClone(value)</code> |
| <code>{ workspaceKind = &#x27;BRANCH&#x27; } = {}</code> | <code>带默认值的参数</code> | BRANCH禁止font标记；MAIN可以保留正式正文的字体设置<br>在此声明中的实际使用：<code>workspaceKind === &#x27;BRANCH&#x27;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>body</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>depth &gt; 64 &#124;&#124;         ++count &gt; 100000 &#124;&#124;         !n &#124;&#124;         typeof n !== &#x27;object&#x27; &#124;&#124;         typeof n.type !== &#x27;string&#x27;</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>workspaceKind === &#x27;BRANCH&#x27; &amp;&amp; (n.marks ?? &#91;&#93;).some((m) =&gt; m.type === &#x27;font&#x27;)</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>atoms.has(n.type)</code>。
- 在本段的内部函数内：检查 <code>n.type === &#x27;code_block&#x27;</code>。
- 在本段的内部函数内：检查 <code>n.type === &#x27;table_cell&#x27; &#124;&#124; n.type === &#x27;table_header&#x27;</code>。
- 在本段的内部函数内：检查 <code>n.type === &#x27;image&#x27;</code>。
- 在本段的内部函数内：检查 <code>!/^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(n.attrs.resourceId) &#124;&#124;           BigInt(n.attrs.resourceId) &gt; 18446744073709551615n</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>!spec</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>n.type !== &#x27;doc&#x27; &amp;&amp; n.type !== &#x27;text&#x27; &amp;&amp; !spec.isInline</code>。
- 在本段的内部函数内：检查 <code>typeof id !== &#x27;string&#x27; &#124;&#124; ids.has(id)</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!validateSchema(body)</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 23 bytes=8977:10626 -->
````javascript
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
````

## 第 24 步：normalizeContent（接着实现）

插入位置：第 23 步代码之后，紧接追加；完成稿约第 283 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>n.type.name === &#x27;table&#x27; &amp;&amp; TableMap.get(n).problems?.length</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>new TextEncoder().encode(canonical(body)).length &gt; 16 * 1024 * 1024</code>。条件成立时执行 <code>throw invalid();</code>。转入外层异常处理路径。
- 执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return body;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 24 bytes=10626:10924 -->
````javascript
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
````

## 第 25 步：resourceIds

插入位置：第 24 步代码之后，紧接追加；完成稿约第 293 行。

本步声明：`export function resourceIds(value) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>walk(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>&#91;...ids&#93;.sort((a, b) =&gt; (BigInt(a) &lt; BigInt(b) ? -1 : BigInt(a) &gt; BigInt(b) ? 1 : 0))</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>n.type === &#x27;image&#x27;</code>。
- 此处通过 <code>return &#91;...ids&#93;.sort((a, b) =&gt; (BigInt(a) &lt; BigInt(b) ? -1 : BigInt(a) &gt; BigInt(b) ? 1 : 0));</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=10924:11346 -->
````javascript
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
````

## 第 26 步：inspectDocument

插入位置：第 25 步代码之后，紧接追加；完成稿约第 303 行。

文档生命周期仍归调用方所有，校验成功或失败都不安装更新、不销毁传入文档。

本步声明：`export function inspectDocument(doc, options) `。

**这一段为什么这样写：** 确认增量依赖完整后，统一读取业务正文、资源集合与规范字节。

1. 先拒绝Yjs尚未满足的结构/删除依赖
2. yDocToProsemirrorJSON→fromPM→normalizeContent
3. 返回body/resources/canonical供计量、资源保护和哈希共同使用


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>doc</code> | <code>JavaScript 参数</code> | 调用方独占的Y.Doc；此函数只读取，不安装、不销毁<br>在此声明中的实际使用：<code>doc.store</code>；<code>yDocToProsemirrorJSON(doc)</code> |
| <code>options</code> | <code>JavaScript 参数</code> | 正文规范化选项<br>在此声明中的实际使用：<code>normalizeContent(fromPM(yDocToProsemirrorJSON(doc)), options)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ body, resources: resourceIds(body), canonical: canonical(body) }</code> |

**跟着本段代码走一遍：**

- 检查 <code>doc.store.pendingStructs &#124;&#124; doc.store.pendingDs</code>。条件成立时执行 <code>throw Object.assign(new Error(&#x27;DEPENDENCIES_MISSING&#x27;), { code: &#x27;DEPENDENCIES_MISSING&#x27; });</code>。转入外层异常处理路径。
- 执行 <code>throw Object.assign(new Error(&#x27;DEPENDENCIES_MISSING&#x27;), { code: &#x27;DEPENDENCIES_MISSING&#x27; });</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return { body, resources: resourceIds(body), canonical: canonical(body) };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err.code === &#x27;DEPENDENCIES_MISSING&#x27;</code>。条件成立时执行 <code>throw err;</code>。转入外层异常处理路径。
- 执行 <code>throw err;</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 26 bytes=11346:11927 -->
````javascript
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
````

## 第 27 步：prepareCandidate

插入位置：第 26 步代码之后，紧接追加；完成稿约第 315 行。

本步声明：`export function prepareCandidate(host, update, options) `。

**这一段为什么这样写：** 把未可信Yjs增量隔离在独立文档里，避免验证失败损坏已保存状态。

1. 复制host的完整Yjs状态
2. 只对新doc应用update
3. inspectDocument读取真实合并结果；失败立即destroy，成功交给调用方释放


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>host</code> | <code>JavaScript 参数</code> | 由持久检查点和连续日志恢复的Y.Doc<br>在此声明中的实际使用：<code>Y.encodeStateAsUpdate(host)</code> |
| <code>update</code> | <code>JavaScript 参数</code> | 本次解码后的Yjs增量字节<br>在此声明中的实际使用：<code>Y.applyUpdate(doc, update)</code> |
| <code>options</code> | <code>JavaScript 参数</code> | 正文校验选项，包含workspaceKind<br>在此声明中的实际使用：<code>inspectDocument(doc, options)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ doc, ...inspectDocument(doc, options) }</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return { doc, ...inspectDocument(doc, options) };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err.code === &#x27;DEPENDENCIES_MISSING&#x27;</code>。条件成立时执行 <code>throw err;</code>。转入外层异常处理路径。
- 执行 <code>throw err;</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 执行 <code>throw invalid();</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 27 bytes=11927:12391 -->
````javascript
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

## 文件完成后的检查

```powershell
node --check 'packages/content/index.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/01-content-replay.md)。
