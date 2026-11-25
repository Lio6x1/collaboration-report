# 分步编写 `services/collaboration/src/domain/structure_content.mjs`

先按 [C05 · 结构拆并与每个在线编辑器保存握手](../lessons/collaboration/05-structure-online.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/302-structure-content.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按顶层H2将整章拆为引言/小节，并能保留原标题节点合章。

拆章不能把列表/表格内部标题也当边界；管理标题的块ID和完整格式必须能还原。

写之前先按这条执行路线理解各段的连接：

1. splitChapterBody只遇顶层level2标题才切段，收集含嵌套块的blockIds
2. 空节用标题ID哈希派生确定性空段落并防碰撞
3. joinChapterBodies恢复原heading，仅消除完全未改占位段落
4. splitChapterDrafts分别迁移MAIN/草稿，要求去字体后的H2边界完全一致

本文件需要守住的边界：

- 没有H2、标题空白或超过240字符明确拒绝
- 草稿自己改了H2不能通过结构管理绕过审核接纳

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/structure_content.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/structure_content.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/structure_content.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/structure_content.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:179 -->
````javascript
import { createHash } from 'node:crypto';
import { canonical, normalizeContent } from '../../../../packages/content/index.mjs';
import { draftBody } from './draft_protocol.mjs';

````

## 第 2 步：failure

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

本步声明：`const failure = (code) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>code</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(new Error(code), { code })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>Object.assign(new Error(code), { code })</code> |

<!-- write-step: 2 bytes=179:248 -->
````javascript
const failure = (code) => Object.assign(new Error(code), { code });

````

## 第 3 步：headingTitle

插入位置：第 2 步代码之后，紧接追加；完成稿约第 7 行。

本步声明：`function headingTitle(heading) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>heading</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>text(heading)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>title</code> |

**跟着本段代码走一遍：**

- 检查 <code>!title &#124;&#124; &#91;...title&#93;.length &gt; 240</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_TITLE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_TITLE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return title;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=248:632 -->
````javascript
function headingTitle(heading) {
  const text = (node) =>
    node.text ??
    (node.type === 'formula'
      ? node.attrs.latex
      : node.type === 'image'
        ? (node.attrs.alt ?? '')
        : (node.content ?? []).map(text).join(''));
  const title = text(heading).trim();
  if (!title || [...title].length > 240) throw failure('STRUCTURE_TITLE_INVALID');
  return title;
}

````

## 第 4 步：blockIds

插入位置：第 3 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`function blockIds(nodes) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>nodes</code> | <code>JavaScript 参数</code> | 需要收集块编号的节点列表；递归遍历各节点content，保留原遍历顺序。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>result</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>node.attrs?.id</code>。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=632:884 -->
````javascript
function blockIds(nodes) {
  const result = [];
  const visit = (node) => {
    if (node.attrs?.id) result.push(node.attrs.id);
    for (const child of node.content ?? []) visit(child);
  };
  for (const node of nodes) visit(node);
  return result;
}

````

## 第 5 步：emptyParagraph

插入位置：第 4 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`function emptyParagraph(id) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>id</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>return { type: &#x27;paragraph&#x27;, attrs: { id }, content: &#91;&#93; };</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ type: &#x27;paragraph&#x27;, attrs: { id }, content: &#91;&#93; }</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return { type: &#x27;paragraph&#x27;, attrs: { id }, content: &#91;&#93; };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=884:977 -->
````javascript
function emptyParagraph(id) {
  return { type: 'paragraph', attrs: { id }, content: [] };
}

````

## 第 6 步：splitChapterBody

插入位置：第 5 步代码之后，紧接追加；完成稿约第 34 行。

只按顶层H2分界；表格、列表、H3及以下子树原样迁移。管理标题的完整原节点随映射保存供合章还原。

本步声明：`export function splitChapterBody(input, { workspaceKind = 'BRANCH' } = {}) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>normalizeContent(input, { workspaceKind })</code> |
| <code>{ workspaceKind = &#x27;BRANCH&#x27; } = {}</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>normalizeContent(input, { workspaceKind })</code>；<code>normalizeContent({ type: &#x27;doc&#x27;, content: nodes }, { workspaceKind })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>pieces.map(({ nodes, ...piece }) =&gt; {     const part = {       ...piece,       blockIds: blockIds(piece.heading ? &#91;piece.heading, ...nodes&#93; : nodes),     };     if (!nodes.length)  …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>node.type === &#x27;heading&#x27; &amp;&amp; node.attrs.level === 2</code>。
- 检查 <code>current.kind === &#x27;SECTION&#x27; &#124;&#124; current.nodes.length</code>。
- 检查 <code>!pieces.some((piece) =&gt; piece.kind === &#x27;SECTION&#x27;)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_LEVEL2_MISSING&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_LEVEL2_MISSING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return pieces.map(({ nodes, ...piece }) =&gt; {     const part = {       ...piece,       blockIds: blockIds(piece.heading ? &#91;piece.heading, ...nodes&#93; : nodes),     };     if (!nodes.l …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>!nodes.length</code>。
- 在本段的内部函数内：检查 <code>originalIds.has(part.emptyBlockId)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_BLOCK_ID_COLLISION&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw failure(&#x27;STRUCTURE_BLOCK_ID_COLLISION&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return part;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=977:2623 -->
````javascript
// 只按顶层H2分界；表格、列表、H3及以下子树原样迁移。管理标题的完整原节点随映射保存供合章还原。
export function splitChapterBody(input, { workspaceKind = 'BRANCH' } = {}) {
  const body = normalizeContent(input, { workspaceKind });
  const pieces = [];
  let current = { kind: 'INTRODUCTION', title: '', nodes: [] };
  for (const node of body.content) {
    if (node.type === 'heading' && node.attrs.level === 2) {
      if (current.kind === 'SECTION' || current.nodes.length) pieces.push(current);
      current = { kind: 'SECTION', title: headingTitle(node), heading: node, nodes: [] };
    } else current.nodes.push(node);
  }
  if (current.kind === 'SECTION' || current.nodes.length) pieces.push(current);
  if (!pieces.some((piece) => piece.kind === 'SECTION')) throw failure('STRUCTURE_LEVEL2_MISSING');
  const originalIds = new Set(blockIds(body.content));
  return pieces.map(({ nodes, ...piece }) => {
    const part = {
      ...piece,
      blockIds: blockIds(piece.heading ? [piece.heading, ...nodes] : nodes),
    };
    if (!nodes.length) {
      // ProseMirror根节点要求至少一个块。确定性占位不冒充原来源；只有未改动的空占位可在合章时消除。
      part.emptyBlockId =
        'structure-empty-' +
        createHash('sha256').update(piece.heading.attrs.id).digest('hex').slice(0, 32);
      if (originalIds.has(part.emptyBlockId)) throw failure('STRUCTURE_BLOCK_ID_COLLISION');
      nodes = [emptyParagraph(part.emptyBlockId)];
    }
    part.body = normalizeContent({ type: 'doc', content: nodes }, { workspaceKind });
    return part;
  });
}

````

## 第 7 步：joinChapterBodies

插入位置：第 6 步代码之后，紧接追加；完成稿约第 66 行。

本步声明：`export function joinChapterBodies(parts, { workspaceKind = 'BRANCH' } = {}) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parts</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Array.isArray(parts)</code>；<code>parts.length</code>；<code>parts.entries</code> |
| <code>{ workspaceKind = &#x27;BRANCH&#x27; } = {}</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>normalizeContent(part.body, { workspaceKind })</code>；<code>normalizeContent({ type: &#x27;doc&#x27;, content }, { workspaceKind })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>normalizeContent({ type: &#x27;doc&#x27;, content }, { workspaceKind })</code> |

**跟着本段代码走一遍：**

- 检查 <code>!Array.isArray(parts) &#124;&#124; !parts.length</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_PARTS_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_PARTS_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>part.kind === &#x27;INTRODUCTION&#x27;</code>。
- 检查 <code>index !== 0 &#124;&#124; part.heading</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_PARTS_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_PARTS_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>part.kind === &#x27;SECTION&#x27; &amp;&amp;       part.heading?.type === &#x27;heading&#x27; &amp;&amp;       part.heading.attrs?.level === 2</code>。
- 检查 <code>headingTitle(part.heading) !== part.title</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_TITLE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_TITLE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 执行 <code>throw failure(&#x27;STRUCTURE_PARTS_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!untouchedEmpty</code>。
- 检查 <code>!sections</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_LEVEL2_MISSING&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_LEVEL2_MISSING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return normalizeContent({ type: &#x27;doc&#x27;, content }, { workspaceKind });</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2623:3787 -->
````javascript
export function joinChapterBodies(parts, { workspaceKind = 'BRANCH' } = {}) {
  if (!Array.isArray(parts) || !parts.length) throw failure('STRUCTURE_PARTS_INVALID');
  const content = [];
  let sections = 0;
  for (const [index, part] of parts.entries()) {
    const body = normalizeContent(part.body, { workspaceKind });
    if (part.kind === 'INTRODUCTION') {
      if (index !== 0 || part.heading) throw failure('STRUCTURE_PARTS_INVALID');
    } else if (
      part.kind === 'SECTION' &&
      part.heading?.type === 'heading' &&
      part.heading.attrs?.level === 2
    ) {
      if (headingTitle(part.heading) !== part.title) throw failure('STRUCTURE_TITLE_CHANGED');
      content.push(structuredClone(part.heading));
      sections++;
    } else throw failure('STRUCTURE_PARTS_INVALID');
    const untouchedEmpty =
      part.emptyBlockId &&
      body.content.length === 1 &&
      canonical(body.content[0]) === canonical(emptyParagraph(part.emptyBlockId));
    if (!untouchedEmpty) content.push(...body.content);
  }
  if (!sections) throw failure('STRUCTURE_LEVEL2_MISSING');
  return normalizeContent({ type: 'doc', content }, { workspaceKind });
}

````

## 第 8 步：splitChapterDrafts

插入位置：第 7 步代码之后，紧接追加；完成稿约第 93 行。

MAIN与已对齐的可靠草稿分别迁移，绝不把S2当成已接纳主线。固定H2边界必须一致，支线自改边界须先正常送审。

本步声明：`export function splitChapterDrafts(mainInput, draftInput) `。

**这一段为什么这样写：** 让MAIN和可靠草稿使用同一边界但各自保留自己的正文。

1. 分别拆两份正文
2. 比较每个H2身份和内容
3. 对缺引言侧补确定性空正文
4. 返回主线/草稿配对及来源块集合


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>mainInput</code> | <code>JavaScript 参数</code> | 固定MAIN正文，可能有font<br>在此声明中的实际使用：<code>splitChapterBody(mainInput, { workspaceKind: &#x27;MAIN&#x27; })</code>；<code>mainInput.content</code> |
| <code>draftInput</code> | <code>JavaScript 参数</code> | 已三方对齐的BRANCH正文<br>在此声明中的实际使用：<code>splitChapterBody(draftInput)</code>；<code>draftInput.content</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>output</code> |

**跟着本段代码走一遍：**

- 检查 <code>mainSections.length !== draftSections.length &#124;&#124;     mainSections.some(       (part, i) =&gt;         canonical(draftBody({ type: &#x27;doc&#x27;, content: &#91;part.heading&#93; }).content&#91;0&#93;) !==      …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw failure(&#x27;DRAFT_STRUCTURE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;DRAFT_STRUCTURE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>mainIntro &#124;&#124; draftIntro</code>。
- 检查 <code>blockIds(mainInput.content).includes(emptyId) &#124;&#124;       blockIds(draftInput.content).includes(emptyId)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_BLOCK_ID_COLLISION&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_BLOCK_ID_COLLISION&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 8 bytes=3787:5798 -->
````javascript
// MAIN与已对齐的可靠草稿分别迁移，绝不把S2当成已接纳主线。固定H2边界必须一致，支线自改边界须先正常送审。
export function splitChapterDrafts(mainInput, draftInput) {
  const main = splitChapterBody(mainInput, { workspaceKind: 'MAIN' });
  const draft = splitChapterBody(draftInput);
  const mainSections = main.filter((p) => p.kind === 'SECTION'),
    draftSections = draft.filter((p) => p.kind === 'SECTION');
  if (
    mainSections.length !== draftSections.length ||
    mainSections.some(
      (part, i) =>
        canonical(draftBody({ type: 'doc', content: [part.heading] }).content[0]) !==
        canonical(draftSections[i].heading),
    )
  ) {
    throw failure('DRAFT_STRUCTURE_CHANGED');
  }
  const output = [];
  const mainIntro = main.find((p) => p.kind === 'INTRODUCTION'),
    draftIntro = draft.find((p) => p.kind === 'INTRODUCTION');
  if (mainIntro || draftIntro) {
    const emptyId =
      'structure-intro-' +
      createHash('sha256').update(mainSections[0].heading.attrs.id).digest('hex').slice(0, 32);
    if (
      blockIds(mainInput.content).includes(emptyId) ||
      blockIds(draftInput.content).includes(emptyId)
    )
      throw failure('STRUCTURE_BLOCK_ID_COLLISION');
    const empty = { type: 'doc', content: [emptyParagraph(emptyId)] };
    output.push({
      kind: 'INTRODUCTION',
      title: '',
      main: mainIntro?.body ?? empty,
      draft: draftIntro?.body ?? structuredClone(empty),
      mainBlockIds: mainIntro?.blockIds ?? [],
      draftBlockIds: draftIntro?.blockIds ?? [],
      emptyBlockId: emptyId,
    });
  }
  for (const [index, part] of mainSections.entries()) {
    const pending = draftSections[index];
    output.push({
      kind: 'SECTION',
      title: part.title,
      heading: part.heading,
      main: part.body,
      draft: pending.body,
      mainBlockIds: part.blockIds,
      draftBlockIds: pending.blockIds,
      emptyBlockId: part.emptyBlockId ?? pending.emptyBlockId,
    });
  }
````

## 第 9 步：splitChapterDrafts（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 145 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return output;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5798:5817 -->
````javascript
  return output;
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/structure_content.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/05-structure-online.md)。
