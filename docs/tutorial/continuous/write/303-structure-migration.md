# 分步编写 `services/collaboration/src/domain/structure_migration.mjs`

先按 [C05 · 结构拆并与每个在线编辑器保存握手](../lessons/collaboration/05-structure-online.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/303-structure-migration.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

从同一个固定全篇预览构造拆章、合章、删除的候选与来源范围映射。

结构迁移需要完整范围证明，不能只移动屏幕上的标题文字，也不能把草稿当已接纳MAIN。

写之前先按这条执行路线理解各段的连接：

1. chapterGroups核目录层级、unit唯一性及排序
2. decodeBody核每个固定内容目标、字节hash、规范字节和资源集合
3. 拆章先computePull虚拟对齐草稿再严格比H2；合章用原boundary.heading恢复标题
4. 输出structure-migration-v1 changes，每个target同时含main_body/draft_body和source_ranges

本文件需要守住的边界：

- 缺lineage必须失败，不能拿main当base消掉真实冲突
- 同章节重叠范围操作拒绝STRUCTURE_RANGE_OVERLAP
- 纯计算不写活动room，不读时钟或数据库

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/structure_migration.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/structure_migration.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/structure_migration.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/structure_migration.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:290 -->
````javascript
import { canonical, normalizeContent, resourceIds } from '../../../../packages/content/index.mjs';
import { hash } from './shared_protocol.mjs';
import { computePull, draftBody } from './draft_protocol.mjs';
import { splitChapterDrafts, joinChapterBodies } from './structure_content.mjs';

````

## 第 2 步：failure

插入位置：第 1 步代码之后，紧接追加；完成稿约第 6 行。

本步声明：`const failure = (code) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>code</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(new Error(code), { code })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>Object.assign(new Error(code), { code })</code> |

<!-- write-step: 2 bytes=290:358 -->
````javascript
const failure = (code) => Object.assign(new Error(code), { code });
````

## 第 3 步：rangeActions

插入位置：第 2 步代码之后，紧接追加；完成稿约第 7 行。

本步声明：`const rangeActions = new Set(['SPLIT_LEVEL2', 'JOIN_CHAPTER', 'REMOVE']);`。

<!-- write-step: 3 bytes=358:433 -->
````javascript
const rangeActions = new Set(['SPLIT_LEVEL2', 'JOIN_CHAPTER', 'REMOVE']);

````

## 第 4 步：decodeBody

插入位置：第 3 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`function decodeBody(content, fixed, unit, branch, epoch) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>content</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>!content &#124;&#124;     content.document_id !== target.document_id</code>；<code>content.document_id</code>；<code>content.unit_id</code>；<code>content.structure_id</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>fixed.binding</code> |
| <code>unit</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>content.unit_id !== unit</code> |
| <code>branch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>content.branch_id !== branch</code>；<code>branch === &#x27;0&#x27;</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>epoch !== undefined</code>；<code>content.workspace_epoch !== epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>body</code> |

**跟着本段代码走一遍：**

- 检查 <code>!content &#124;&#124;     content.document_id !== target.document_id &#124;&#124;     content.unit_id !== unit &#124;&#124;     content.structure_id !== target.structure_id &#124;&#124;     content.branch_id !== branch &#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_CONTENT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_CONTENT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>bytes.length &gt; 16 &lt;&lt; 20 &#124;&#124;       String(bytes.length) !== content.byte_size &#124;&#124;       hash(bytes).toString(&#x27;base64&#x27;) !== content.body_hash &#124;&#124;       canonical(body) !== bytes.toStrin …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_CONTENT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_CONTENT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return body;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>throw failure(&#x27;STRUCTURE_CONTENT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 4 bytes=433:1520 -->
````javascript
function decodeBody(content, fixed, unit, branch, epoch) {
  const target = fixed.binding.context.target;
  if (
    !content ||
    content.document_id !== target.document_id ||
    content.unit_id !== unit ||
    content.structure_id !== target.structure_id ||
    content.branch_id !== branch ||
    content.operation_id !== fixed.binding.context.operation_id ||
    (epoch !== undefined && content.workspace_epoch !== epoch)
  )
    throw failure('STRUCTURE_CONTENT_UNKNOWN');
  try {
    const bytes = Buffer.from(content.body_json_utf8, 'base64');
    const body = normalizeContent(JSON.parse(bytes), {
      workspaceKind: branch === '0' ? 'MAIN' : 'BRANCH',
    });
    if (
      bytes.length > 16 << 20 ||
      String(bytes.length) !== content.byte_size ||
      hash(bytes).toString('base64') !== content.body_hash ||
      canonical(body) !== bytes.toString() ||
      canonical(resourceIds(body)) !== canonical(content.resource_ids)
    )
      throw failure('STRUCTURE_CONTENT_UNKNOWN');
    return body;
  } catch {
    throw failure('STRUCTURE_CONTENT_UNKNOWN');
  }
}

````

## 第 5 步：indexExact

插入位置：第 4 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`function indexExact(items, units) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>items</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Array.isArray(items)</code>；<code>items.length</code> |
| <code>units</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>units.size</code>；<code>units.has</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>index</code> |

**跟着本段代码走一遍：**

- 检查 <code>!Array.isArray(items) &#124;&#124; items.length !== units.size</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!units.has(item.unit_id) &#124;&#124; index.has(item.unit_id)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return index;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1520:1886 -->
````javascript
function indexExact(items, units) {
  if (!Array.isArray(items) || items.length !== units.size)
    throw failure('STRUCTURE_SCOPE_UNKNOWN');
  const index = new Map();
  for (const item of items) {
    if (!units.has(item.unit_id) || index.has(item.unit_id))
      throw failure('STRUCTURE_SCOPE_UNKNOWN');
    index.set(item.unit_id, item);
  }
  return index;
}

````

## 第 6 步：chapterGroups

插入位置：第 5 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`function chapterGroups(structure) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>structure</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>structure.entries</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ chapters, units }</code> |

**跟着本段代码走一遍：**

- 检查 <code>nodes.has(entry.node_id)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>entry.kind === &#x27;CHAPTER&#x27; &amp;&amp; !entry.parent_node_id</code>。
- 检查 <code>chapters.has(entry.chapter_id)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!chapter</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>entry !== chapter.root</code>。
- 检查 <code>!chapter.root.split_level2 &#124;&#124;         entry.parent_node_id !== chapter.root.node_id &#124;&#124;         !&#91;&#x27;INTRODUCTION&#x27;, &#x27;SECTION&#x27;&#93;.includes(entry.kind)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>entry.unit_id</code>。
- 检查 <code>units.has(entry.unit_id)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>chapter.root.split_level2</code>。
- 检查 <code>chapter.root.unit_id &#124;&#124;         !chapter.children.length &#124;&#124;         chapter.children.some((v) =&gt; !v.unit_id)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>new Set(chapter.children.map((v) =&gt; v.sort_order)).size !== chapter.children.length &#124;&#124;         chapter.children.filter((v) =&gt; v.kind === &#x27;INTRODUCTION&#x27;).length &gt; 1 &#124;&#124;         chapt …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 6 bytes=1886:3690 -->
````javascript
function chapterGroups(structure) {
  const chapters = new Map(),
    units = new Set(),
    nodes = new Set();
  for (const entry of structure.entries) {
    if (nodes.has(entry.node_id)) throw failure('STRUCTURE_SCOPE_UNKNOWN');
    nodes.add(entry.node_id);
    if (entry.kind === 'CHAPTER' && !entry.parent_node_id) {
      if (chapters.has(entry.chapter_id)) throw failure('STRUCTURE_SCOPE_UNKNOWN');
      chapters.set(entry.chapter_id, { root: entry, children: [] });
    }
  }
  for (const entry of structure.entries) {
    const chapter = chapters.get(entry.chapter_id);
    if (!chapter) throw failure('STRUCTURE_SCOPE_UNKNOWN');
    if (entry !== chapter.root) {
      if (
        !chapter.root.split_level2 ||
        entry.parent_node_id !== chapter.root.node_id ||
        !['INTRODUCTION', 'SECTION'].includes(entry.kind)
      )
        throw failure('STRUCTURE_SCOPE_UNKNOWN');
      chapter.children.push(entry);
    }
    if (entry.unit_id) {
      if (units.has(entry.unit_id)) throw failure('STRUCTURE_SCOPE_UNKNOWN');
      units.add(entry.unit_id);
    }
  }
  for (const chapter of chapters.values()) {
    if (chapter.root.split_level2) {
      if (
        chapter.root.unit_id ||
        !chapter.children.length ||
        chapter.children.some((v) => !v.unit_id)
      )
        throw failure('STRUCTURE_SCOPE_UNKNOWN');
      chapter.children.sort((a, b) => a.sort_order - b.sort_order);
      if (
        new Set(chapter.children.map((v) => v.sort_order)).size !== chapter.children.length ||
        chapter.children.filter((v) => v.kind === 'INTRODUCTION').length > 1 ||
        chapter.children.some((v, i) => v.kind === 'INTRODUCTION' && i !== 0) ||
        !chapter.children.some((v) => v.kind === 'SECTION')
      )
        throw failure('STRUCTURE_SCOPE_UNKNOWN');
````

## 第 7 步：chapterGroups（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 97 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!chapter.root.unit_id &#124;&#124; chapter.children.length</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return { chapters, units };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3690:3934 -->
````javascript
      chapter.units = chapter.children;
    } else {
      if (!chapter.root.unit_id || chapter.children.length)
        throw failure('STRUCTURE_SCOPE_UNKNOWN');
      chapter.units = [chapter.root];
    }
  }
  return { chapters, units };
}

````

## 第 8 步：blockIds

插入位置：第 7 步代码之后，紧接追加；完成稿约第 107 行。

本步声明：`function blockIds(body) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>body</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>visit(body)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>ids</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>node.attrs?.id</code>。
- 此处通过 <code>return ids;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3934:4150 -->
````javascript
function blockIds(body) {
  const ids = [];
  const visit = (node) => {
    if (node.attrs?.id) ids.push(node.attrs.id);
    for (const child of node.content ?? []) visit(child);
  };
  visit(body);
  return ids;
}

````

## 第 9 步：sourceRange

插入位置：第 8 步代码之后，紧接追加；完成稿约第 117 行。

本步声明：`function sourceRange(unit, main, draft, start = null, end = null) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>unit</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>return {     source_unit_id: unit,     source_scope: &#x27;WHOLE_UNIT&#x27;,     start_boundary: start,     end_boundary: end,     main_block_ids: main,     draft_block_ids: draft,   };</code> |
| <code>main</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>return {     source_unit_id: unit,     source_scope: &#x27;WHOLE_UNIT&#x27;,     start_boundary: start,     end_boundary: end,     main_block_ids: main,     draft_block_ids: draft,   };</code> |
| <code>draft</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>return {     source_unit_id: unit,     source_scope: &#x27;WHOLE_UNIT&#x27;,     start_boundary: start,     end_boundary: end,     main_block_ids: main,     draft_block_ids: draft,   };</code> |
| <code>start = null</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>return {     source_unit_id: unit,     source_scope: &#x27;WHOLE_UNIT&#x27;,     start_boundary: start,     end_boundary: end,     main_block_ids: main,     draft_block_ids: draft,   };</code> |
| <code>end = null</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>return {     source_unit_id: unit,     source_scope: &#x27;WHOLE_UNIT&#x27;,     start_boundary: start,     end_boundary: end,     main_block_ids: main,     draft_block_ids: draft,   };</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     source_unit_id: unit,     source_scope: &#x27;WHOLE_UNIT&#x27;,     start_boundary: start,     end_boundary: end,     main_block_ids: main,     draft_block_ids: draft,   }</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return {     source_unit_id: unit,     source_scope: &#x27;WHOLE_UNIT&#x27;,     start_boundary: start,     end_boundary: end,     main_block_ids: main,     draft_block_ids: draft,   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4150:4399 -->
````javascript
function sourceRange(unit, main, draft, start = null, end = null) {
  return {
    source_unit_id: unit,
    source_scope: 'WHOLE_UNIT',
    start_boundary: start,
    end_boundary: end,
    main_block_ids: main,
    draft_block_ids: draft,
  };
}

````

## 第 10 步：splitTargets

插入位置：第 9 步代码之后，紧接追加；完成稿约第 128 行。

本步声明：`function splitTargets(change, main, draft, lineage) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>change</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>change.chapter_id</code>；<code>change.source_units</code> |
| <code>main</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>computePull(lineage.body, draft, main)</code>；<code>splitChapterDrafts(main, aligned.body)</code> |
| <code>draft</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>computePull(lineage.body, draft, main)</code> |
| <code>lineage</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>lineage?.body</code>；<code>lineage.body</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>pieces.map((part, i) =&gt; {     const section = sections.indexOf(part);     const start = part.kind === &#x27;INTRODUCTION&#x27; ? null : part.heading.attrs.id;     const end =       part.kind …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>!lineage?.body</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_LINEAGE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_LINEAGE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>aligned.conflicts.length</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_DRAFT_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_DRAFT_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return pieces.map((part, i) =&gt; {     const section = sections.indexOf(part);     const start = part.kind === &#x27;INTRODUCTION&#x27; ? null : part.heading.attrs.id;     const end =       pa …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：此处通过 <code>return {       key: `${change.chapter_id}:SPLIT:${i}`,       kind: part.kind,       title: part.title,       main_body: part.main,       draft_body: part.draft,       boundary: {   …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4399:5918 -->
````javascript
function splitTargets(change, main, draft, lineage) {
  // 预览内部虚拟拉取同一固定MAIN：旧基线、原草稿与MAIN都来自原固定事实，不写回房间或原草稿。
  // 缺失谱系不能拿MAIN冒充base；否则会吞掉支线与主线对同一块的真实冲突。
  if (!lineage?.body) throw failure('STRUCTURE_LINEAGE_UNKNOWN');
  const aligned = computePull(lineage.body, draft, main);
  if (aligned.conflicts.length) throw failure('STRUCTURE_DRAFT_CONFLICT');
  // 三方对齐只补主线变化。支线独自改过的H2仍由严格分界校验拒绝，不能借拆分接纳未审核目录。
  const pieces = splitChapterDrafts(main, aligned.body);
  const sections = pieces.filter((v) => v.kind === 'SECTION');
  return pieces.map((part, i) => {
    const section = sections.indexOf(part);
    const start = part.kind === 'INTRODUCTION' ? null : part.heading.attrs.id;
    const end =
      part.kind === 'INTRODUCTION'
        ? sections[0].heading.attrs.id
        : (sections[section + 1]?.heading.attrs.id ?? null);
    return {
      key: `${change.chapter_id}:SPLIT:${i}`,
      kind: part.kind,
      title: part.title,
      main_body: part.main,
      draft_body: part.draft,
      boundary: {
        ...(part.heading && { heading: part.heading }),
        ...(part.emptyBlockId && { empty_block_id: part.emptyBlockId }),
      },
      source_ranges: [
        sourceRange(change.source_units[0], part.mainBlockIds, part.draftBlockIds, start, end),
      ],
    };
  });
}

````

## 第 11 步：joinTarget

插入位置：第 10 步代码之后，紧接追加；完成稿约第 161 行。

本步声明：`function joinTarget(change, chapter, bodies, boundaries) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>change</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>change.chapter_id</code> |
| <code>chapter</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>chapter.units</code>；<code>chapter.root</code> |
| <code>bodies</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>bodies.get</code> |
| <code>boundaries</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>boundaries.get</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     key: `${change.chapter_id}:JOIN`,     kind: &#x27;CHAPTER&#x27;,     title: chapter.root.title,     main_body: main,     draft_body: draft,     boundary: {},     source_ranges: chapter …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>!boundary &#124;&#124; (entry.kind === &#x27;SECTION&#x27; &amp;&amp; !boundary.heading)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_BOUNDARY_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw failure(&#x27;STRUCTURE_BOUNDARY_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return {       kind: entry.kind,       title: entry.title,       heading: boundary.heading,       emptyBlockId: boundary.empty_block_id,     };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return {     key: `${change.chapter_id}:JOIN`,     kind: &#x27;CHAPTER&#x27;,     title: chapter.root.title,     main_body: main,     draft_body: draft,     boundary: {},     source_ranges:  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：此处通过 <code>return {         ...sourceRange(entry.unit_id, blockIds(pair.main), blockIds(pair.draft)),         target_scope: &#x27;INTERVAL&#x27;,         target_start_boundary: parts&#91;i&#93;.heading?.attrs. …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5918:7439 -->
````javascript
function joinTarget(change, chapter, bodies, boundaries) {
  const parts = chapter.units.map((entry) => {
    const boundary = boundaries.get(entry.unit_id);
    // SECTION管理标题的原节点必须来自已固定的完整谱系，不能由标题文字重新生成一个身份。
    if (!boundary || (entry.kind === 'SECTION' && !boundary.heading))
      throw failure('STRUCTURE_BOUNDARY_UNKNOWN');
    return {
      kind: entry.kind,
      title: entry.title,
      heading: boundary.heading,
      emptyBlockId: boundary.empty_block_id,
    };
  });
  const main = joinChapterBodies(
    parts.map((part, i) => ({ ...part, body: bodies.get(chapter.units[i].unit_id).main })),
    { workspaceKind: 'MAIN' },
  );
  const draft = joinChapterBodies(
    parts.map((part, i) => ({
      ...part,
      heading: part.heading && draftBody({ type: 'doc', content: [part.heading] }).content[0],
      body: bodies.get(chapter.units[i].unit_id).draft,
    })),
  );
  return {
    key: `${change.chapter_id}:JOIN`,
    kind: 'CHAPTER',
    title: chapter.root.title,
    main_body: main,
    draft_body: draft,
    boundary: {},
    source_ranges: chapter.units.map((entry, i) => {
      const pair = bodies.get(entry.unit_id);
      return {
        ...sourceRange(entry.unit_id, blockIds(pair.main), blockIds(pair.draft)),
        target_scope: 'INTERVAL',
        target_start_boundary: parts[i].heading?.attrs.id ?? null,
        target_end_boundary: parts[i + 1]?.heading?.attrs.id ?? null,
      };
    }),
  };
}

````

## 第 12 步：buildStructureMigration

插入位置：第 11 步代码之后，紧接追加；完成稿约第 204 行。

只消费同一个固定预览；不访问时钟、数据库或活动房间，候选与范围声明一起受原摘要保护。

本步声明：`export function buildStructureMigration(fixed) `。

**这一段为什么这样写：** 把结构提案变成可哈希的完整迁移事实。

1. 验证结构和MAIN/草稿范围恰好对应
2. 按change类型生成targets
3. 限制每个候选正文16MiB
4. 返回候选与块范围一体的迁移对象


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>JavaScript 参数</code> | 冻结的binding/source/main_units/drafts/observed_cut预览<br>在此声明中的实际使用：<code>fixed.source</code>；<code>fixed.binding</code>；<code>fixed.main_units</code>；<code>fixed.drafts</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ schema_version: &#x27;structure-migration-v1&#x27;, changes }</code> |

**跟着本段代码走一遍：**

- 检查 <code>!target &#124;&#124;     !source?.structure &#124;&#124;     source.structure.id !== target.structure_id &#124;&#124;     source.structure.document_id !== target.document_id &#124;&#124;     !Array.isArray(source.structu …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!units.has(boundary.unit_id) &#124;&#124; boundaries.has(boundary.unit_id)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_BOUNDARY_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_BOUNDARY_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!rangeActions.has(input.type)</code>。条件成立时执行 <code>continue;</code>。跳过当前循环本轮剩余语句。
- 检查 <code>!chapter</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>affected.has(input.chapter_id)</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_RANGE_OVERLAP&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_RANGE_OVERLAP&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 12 bytes=7439:9178 -->
````javascript
// 只消费同一个固定预览；不访问时钟、数据库或活动房间，候选与范围声明一起受原摘要保护。
export function buildStructureMigration(fixed) {
  const source = fixed.source,
    target = fixed.binding?.context?.target;
  if (
    !target ||
    !source?.structure ||
    source.structure.id !== target.structure_id ||
    source.structure.document_id !== target.document_id ||
    !Array.isArray(source.structure.entries) ||
    !Array.isArray(source.changes)
  )
    throw failure('STRUCTURE_SCOPE_UNKNOWN');
  const { chapters, units } = chapterGroups(source.structure);
  const main = indexExact(fixed.main_units, units),
    drafts = indexExact(fixed.drafts, units);
  const workspaces = indexExact(source.drafts, units),
    bodies = new Map();
  for (const unit of units) {
    const workspace = workspaces.get(unit);
    bodies.set(unit, {
      main: decodeBody(main.get(unit), fixed, unit, '0'),
      draft: decodeBody(
        drafts.get(unit),
        fixed,
        unit,
        workspace.branch_id,
        workspace.workspace_epoch,
      ),
    });
  }
  const boundaries = new Map();
  for (const boundary of source.unit_boundaries ?? []) {
    if (!units.has(boundary.unit_id) || boundaries.has(boundary.unit_id))
      throw failure('STRUCTURE_BOUNDARY_UNKNOWN');
    boundaries.set(boundary.unit_id, boundary);
  }
  const changes = [],
    affected = new Set();
  for (const input of source.changes) {
    if (!rangeActions.has(input.type)) continue;
    const chapter = chapters.get(input.chapter_id);
    if (!chapter) throw failure('STRUCTURE_SCOPE_UNKNOWN');
    if (affected.has(input.chapter_id)) throw failure('STRUCTURE_RANGE_OVERLAP');
    affected.add(input.chapter_id);
````

## 第 13 步：buildStructureMigration（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 249 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>input.type === &#x27;SPLIT_LEVEL2&#x27;</code>。
- 检查 <code>chapter.root.split_level2</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_MODE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_MODE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>input.type === &#x27;JOIN_CHAPTER&#x27;</code>。
- 检查 <code>!chapter.root.split_level2</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_MODE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_MODE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>Buffer.byteLength(canonical(candidate.main_body)) &gt; 16 &lt;&lt; 20 &#124;&#124;         Buffer.byteLength(canonical(candidate.draft_body)) &gt; 16 &lt;&lt; 20</code>。条件成立时执行 <code>throw failure(&#x27;STRUCTURE_CONTENT_LIMIT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw failure(&#x27;STRUCTURE_CONTENT_LIMIT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return { schema_version: &#x27;structure-migration-v1&#x27;, changes };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=9178:10227 -->
````javascript
    const change = {
      type: input.type,
      chapter_id: input.chapter_id,
      source_units: chapter.units.map((v) => v.unit_id),
      targets: [],
    };
    if (input.type === 'SPLIT_LEVEL2') {
      if (chapter.root.split_level2) throw failure('STRUCTURE_MODE_CHANGED');
      const pair = bodies.get(chapter.root.unit_id);
      change.targets = splitTargets(
        change,
        pair.main,
        pair.draft,
        workspaces.get(chapter.root.unit_id).lineage,
      );
    } else if (input.type === 'JOIN_CHAPTER') {
      if (!chapter.root.split_level2) throw failure('STRUCTURE_MODE_CHANGED');
      change.targets = [joinTarget(change, chapter, bodies, boundaries)];
    }
    for (const candidate of change.targets)
      if (
        Buffer.byteLength(canonical(candidate.main_body)) > 16 << 20 ||
        Buffer.byteLength(canonical(candidate.draft_body)) > 16 << 20
      )
        throw failure('STRUCTURE_CONTENT_LIMIT');
    changes.push(change);
  }
  return { schema_version: 'structure-migration-v1', changes };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/structure_migration.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/05-structure-online.md)。
