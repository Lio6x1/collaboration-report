# 分步编写 `services/collaboration/src/domain/merge/blocks.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/293-blocks.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按稳定块ID和章节父子关系合并块内容、删除和排序。

块id是结构身份，标题层级变化会改变章节归属；不能只把全文当文本diff。

写之前先按这条执行路线理解各段的连接：

1. headingTree将标题层级转成树，parentMap发现跨父移动即报告CROSS_PARENT_STRUCTURE
2. entity处理新增ID碰撞、双侧删除及DELETE_EDIT
3. node按类型选择行内合并或列表/表格子块递归
4. children比较旧块次序，给新增块寻找前后锚点；顺序不唯一时报ORDER/INSERT冲突

本文件需要守住的边界：

- 表格/列表的嵌套块也必须有唯一稳定ID
- 一侧删块另一侧修改不能静默删除；同新ID不同内容也不能任选一个

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain/merge' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/merge/blocks.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/merge/blocks.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/merge/blocks.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/merge/blocks.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:115 -->
````javascript
import { clone, equal, same, semantic, hasFont } from './values.mjs';
import { mergeInline } from './inline.mjs';

````

## 第 2 步：headingTree

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

编辑正文以平铺heading保存；计算时临时成树，删除标题才会检查整个子树，而不是遗漏子段落。

本步声明：`export function headingTree(document) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>document.content</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>root</code> |

**跟着本段代码走一遍：**

- 检查 <code>value.type === &#x27;heading&#x27;</code>。
- 此处通过 <code>return root;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=115:743 -->
````javascript
// 编辑正文以平铺heading保存；计算时临时成树，删除标题才会检查整个子树，而不是遗漏子段落。
export function headingTree(document) {
  const root = { id: '$root', children: [] },
    stack = [root];
  for (const value of document.content) {
    const entity = { id: value.attrs.id, value };
    if (value.type === 'heading') {
      entity.children = [];
      while (stack.length > 1 && stack.at(-1).value.attrs.level >= value.attrs.level) stack.pop();
      stack.at(-1).children.push(entity);
      stack.push(entity);
    } else stack.at(-1).children.push(entity);
  }
  return root;
}
````

## 第 3 步：flattenHeadings

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`export const flattenHeadings = (entities) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>entities</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>entities.flatMap</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>entities.flatMap((entity) =&gt; &#91;entity.value, ...flattenHeadings(entity.children ?? &#91;&#93;)&#93;)</code> |

<!-- write-step: 3 bytes=743:879 -->
````javascript
export const flattenHeadings = (entities) =>
  entities.flatMap((entity) => [entity.value, ...flattenHeadings(entity.children ?? [])]);
````

## 第 4 步：parentMap

插入位置：第 3 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`function parentMap(entity, parent = '$root', result = new Map()) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>entity</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>entity.value</code>；<code>entity.id</code>；<code>entity.children</code> |
| <code>parent = &#x27;$root&#x27;</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>result.set(entity.id, parent)</code> |
| <code>result = new Map()</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>result.set</code>；<code>parentMap(child, entity.id, result)</code>；<code>return result;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>result</code> |

**跟着本段代码走一遍：**

- 检查 <code>entity.value</code>。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=879:1099 -->
````javascript
function parentMap(entity, parent = '$root', result = new Map()) {
  if (entity.value) result.set(entity.id, parent);
  for (const child of entity.children ?? []) parentMap(child, entity.id, result);
  return result;
}

````

## 第 5 步：MergeBlocks

插入位置：第 4 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`export class MergeBlocks `。

**这一段为什么这样写：** 把正文树的身份、内容和顺序三个维度分开处理。

1. roots先核章节父归属
2. entity判断新增/删除
3. node合并字段与子树
4. children重组顺序并收集歧义


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>values</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.values = values</code> |

**本段进入 MergeBlocks.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>values</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.values = values</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 MergeBlocks.roots：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>headingTree(base)</code> |
| <code>submitted</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>headingTree(submitted)</code> |
| <code>main</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>headingTree(main)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>flattenHeadings(this.children(b.children, s.children, m.children, &#x27;$root&#x27;))</code> |

**本段进入 MergeBlocks.node：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>base.attrs</code>；<code>same(base, submitted)</code>；<code>equal(base, main)</code>；<code>base.type</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>submitted</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>submitted.attrs</code>；<code>same(base, submitted)</code>；<code>same(main, submitted)</code>；<code>semantic(submitted)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>main</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>main.attrs</code>；<code>clone(main)</code>；<code>same(main, submitted)</code>；<code>equal(base, main)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>path</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.values.report(&#x27;NODE_TYPE&#x27;, block, path, &#x27;同一块发生了不能安全组合的节点类型修改。&#x27;)</code>；<code>this.values.attributes(       base.attrs,       submitted.attrs,       main.attrs,       block,       `${path}.attrs`,     )</code>；<code>mergeInline(this.values, base, submitted, main, block, path)</code>；<code>this.children(           wrap(base.content),           wrap(submitted.content),           wrap(main.content),           path,         )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>clone(main)</code>；<code>semantic(submitted)</code>；<code>{         type: base.type,         attrs,         content: mergeInline(this.values, base, submitted, main, block, path),       }</code>；<code>{         type: base.type,         attrs,         content: this.children(           wrap(base.content),           wrap(submitted.content),           wrap(main.content),           p …</code>（长表达式，按本步完整代码继续）<br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>(sp.has(id) &amp;&amp; sp.get(id) !== parent) &#124;&#124; (mp.has(id) &amp;&amp; mp.get(id) !== parent)</code>。
- 此处通过 <code>return flattenHeadings(this.children(b.children, s.children, m.children, &#x27;$root&#x27;));</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>same(base, submitted)</code>。条件成立时执行 <code>return clone(main);</code>。当前函数在这里结束。
- 检查 <code>same(main, submitted)</code>。条件成立时执行 <code>return clone(main);</code>。当前函数在这里结束。
- 检查 <code>equal(base, main) &amp;&amp; !hasFont(main)</code>。条件成立时执行 <code>return semantic(submitted);</code>。当前函数在这里结束。
- 检查 <code>base.type !== main.type &#124;&#124; base.type !== submitted.type</code>。
- 此处通过 <code>return clone(main);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>&#91;&#x27;paragraph&#x27;, &#x27;heading&#x27;, &#x27;code_block&#x27;&#93;.includes(base.type)</code>。条件成立时执行 <code>return {         type: base.type,         attrs,         content: mergeInline(this.values, base, submitted, main, block, path),       };</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=1099:2648 -->
````javascript
export class MergeBlocks {
  constructor(values) {
    this.values = values;
  }

  roots(base, submitted, main) {
    const b = headingTree(base),
      s = headingTree(submitted),
      m = headingTree(main);
    const bp = parentMap(b),
      sp = parentMap(s),
      mp = parentMap(m);
    for (const [id, parent] of bp)
      if ((sp.has(id) && sp.get(id) !== parent) || (mp.has(id) && mp.get(id) !== parent))
        this.values.report(
          'CROSS_PARENT_STRUCTURE',
          id,
          id,
          '章节层级或跨父范围移动需要明确结构处理。',
        );
    return flattenHeadings(this.children(b.children, s.children, m.children, '$root'));
  }

  node(base, submitted, main, path) {
    const block = base.attrs?.id ?? main.attrs?.id ?? submitted.attrs?.id ?? '';
    if (same(base, submitted)) return clone(main);
    if (same(main, submitted)) return clone(main);
    if (equal(base, main) && !hasFont(main)) return semantic(submitted);
    if (base.type !== main.type || base.type !== submitted.type) {
      this.values.report('NODE_TYPE', block, path, '同一块发生了不能安全组合的节点类型修改。');
      return clone(main);
    }
    const attrs = this.values.attributes(
      base.attrs,
      submitted.attrs,
      main.attrs,
      block,
      `${path}.attrs`,
    );
    if (['paragraph', 'heading', 'code_block'].includes(base.type))
      return {
        type: base.type,
        attrs,
        content: mergeInline(this.values, base, submitted, main, block, path),
      };
````

## 第 6 步：MergeBlocks（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 72 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 MergeBlocks.entity：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>same(base, main ?? submitted)</code>；<code>base.id</code>；<code>base.value</code>；<code>base.children</code> |
| <code>submitted</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>semantic(submitted)</code>；<code>!submitted &#124;&#124; same(main, submitted)</code>；<code>same(main, submitted)</code>；<code>!main &#124;&#124; !submitted</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>main</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>same(main, submitted)</code>；<code>clone(main)</code>；<code>main.id</code>；<code>!main &#124;&#124; !submitted</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>path</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.values.report(&#x27;NEW_ID_COLLISION&#x27;, main.id, path, &#x27;同一新块身份在两侧产生不同内容。&#x27;)</code>；<code>this.values.report(&#x27;DELETE_EDIT&#x27;, base.id, path, &#x27;已删除的块或章节子树在另一侧仍有修改。&#x27;)</code>；<code>this.node(base.value, submitted.value, main.value, path)</code>；<code>this.children(               base.children,               submitted.children ?? &#91;&#93;,               main.children ?? &#91;&#93;,               path,             )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>semantic(submitted)</code>；<code>clone(main)</code>；<code>null</code>；<code>main ? clone(main) : null</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**本段进入 MergeBlocks.children：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>map(base)</code>；<code>original(base)</code> |
| <code>submitted</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>map(submitted)</code>；<code>original(submitted)</code> |
| <code>main</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>map(main)</code>；<code>original(main)</code>；<code>main.map</code> |
| <code>path</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.entity(b.get(id), s.get(id), m.get(id), `${path}/${id}`)</code>；<code>this.values.report(         &#x27;ORDER_CONFLICT&#x27;,         b.get(bo&#91;0&#93;)?.id ?? &#x27;&#x27;,         path,         &#x27;双方对原有块提出不同排序。&#x27;,       )</code>；<code>this.values.report(&#x27;INSERT_ANCHOR&#x27;, id, path, &#x27;新增块的前后锚点因移序互相矛盾。&#x27;)</code>；<code>this.values.report(             &#x27;INSERT_ORDER&#x27;,             added.MAIN&#91;0&#93;,             `${path}@${i}`,             &#x27;同一结构位置的两组新增块没有可确认顺序。&#x27;,           )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>main.map((node) =&gt; merged.get(node.id)).filter(Boolean)</code>；<code>result</code> |

**跟着本段代码走一遍：**

- 检查 <code>&#91;         &#x27;table&#x27;,         &#x27;table_row&#x27;,         &#x27;table_cell&#x27;,         &#x27;table_header&#x27;,         &#x27;bullet_list&#x27;,         &#x27;ordered_list&#x27;,         &#x27;list_item&#x27;,         &#x27;blockquote&#x27;,       &#93;.includes(base.type)</code>。
- 此处通过 <code>return {         type: base.type,         attrs,         content: this.children(           wrap(base.content),           wrap(submitted.content),           wrap(main.content),      …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return { ...clone(main), attrs };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!base</code>。
- 检查 <code>!main</code>。条件成立时执行 <code>return semantic(submitted);</code>。当前函数在这里结束。
- 检查 <code>!submitted &#124;&#124; same(main, submitted)</code>。条件成立时执行 <code>return clone(main);</code>。当前函数在这里结束。
- 检查 <code>!main &#124;&#124; !submitted</code>。
- 检查 <code>!main &amp;&amp; !submitted</code>。条件成立时执行 <code>return null;</code>。当前函数在这里结束。
- 检查 <code>same(base, main ?? submitted)</code>。条件成立时执行 <code>return null;</code>。当前函数在这里结束。
- 此处通过 <code>return main ? clone(main) : null;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return {       id: base.id,       value: this.node(base.value, submitted.value, main.value, path),       ...(base.children         ? {             children: this.children(          …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2648:4267 -->
````javascript
    if (
      [
        'table',
        'table_row',
        'table_cell',
        'table_header',
        'bullet_list',
        'ordered_list',
        'list_item',
        'blockquote',
      ].includes(base.type)
    ) {
      const wrap = (nodes) => (nodes ?? []).map((value) => ({ id: value.attrs.id, value }));
      return {
        type: base.type,
        attrs,
        content: this.children(
          wrap(base.content),
          wrap(submitted.content),
          wrap(main.content),
          path,
        ).map((entity) => entity.value),
      };
    }
    return { ...clone(main), attrs };
  }

  entity(base, submitted, main, path) {
    if (!base) {
      if (!main) return semantic(submitted);
      if (!submitted || same(main, submitted)) return clone(main);
      this.values.report('NEW_ID_COLLISION', main.id, path, '同一新块身份在两侧产生不同内容。');
      return clone(main);
    }
    if (!main || !submitted) {
      if (!main && !submitted) return null;
      if (same(base, main ?? submitted)) return null;
      this.values.report('DELETE_EDIT', base.id, path, '已删除的块或章节子树在另一侧仍有修改。');
      return main ? clone(main) : null;
    }
    return {
      id: base.id,
      value: this.node(base.value, submitted.value, main.value, path),
      ...(base.children
        ? {
            children: this.children(
              base.children,
              submitted.children ?? [],
              main.children ?? [],
              path,
            ),
          }
        : {}),
    };
  }

  children(base, submitted, main, path) {
````

## 第 7 步：MergeBlocks（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 129 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>entity</code>。
- 检查 <code>bo.length !== mo.length &#124;&#124; bo.length !== so.length</code>。条件成立时执行 <code>return main.map((node) =&gt; merged.get(node.id)).filter(Boolean);</code>。当前函数在这里结束。
- 检查 <code>!equal(bo, mo) &amp;&amp; !equal(bo, so) &amp;&amp; !equal(mo, so)</code>。
- 检查 <code>b.has(nodes&#91;i&#93;.id) &amp;&amp; merged.has(nodes&#91;i&#93;.id)</code>。
- 检查 <code>b.has(id) &amp;&amp; merged.has(id)</code>。
- 检查 <code>b.has(id) &#124;&#124; !merged.has(id)</code>。条件成立时执行 <code>continue;</code>。跳过当前循环本轮剩余语句。
- 检查 <code>before &amp;&amp; next &amp;&amp; orderPosition.get(before) &gt;= orderPosition.get(next)</code>。

<!-- write-step: 7 bytes=4267:6113 -->
````javascript
    const map = (nodes) => new Map(nodes.map((node) => [node.id, node]));
    const b = map(base),
      s = map(submitted),
      m = map(main),
      merged = new Map();
    for (const id of new Set([...b.keys(), ...s.keys(), ...m.keys()])) {
      const entity = this.entity(b.get(id), s.get(id), m.get(id), `${path}/${id}`);
      if (entity) merged.set(id, entity);
    }
    const original = (nodes) =>
      nodes.map((node) => node.id).filter((id) => b.has(id) && merged.has(id));
    const bo = original(base),
      so = original(submitted),
      mo = original(main);
    if (bo.length !== mo.length || bo.length !== so.length)
      return main.map((node) => merged.get(node.id)).filter(Boolean);
    let order = equal(bo, mo) ? so : mo;
    if (!equal(bo, mo) && !equal(bo, so) && !equal(mo, so)) {
      this.values.report(
        'ORDER_CONFLICT',
        b.get(bo[0])?.id ?? '',
        path,
        '双方对原有块提出不同排序。',
      );
      order = mo;
    }
    const additions = new Map(),
      orderPosition = new Map(order.map((id, i) => [id, i]));
    for (const [side, nodes] of [
      ['MAIN', main],
      ['SUBMITTED', submitted],
    ]) {
      const nextAnchor = new Array(nodes.length);
      let after, before;
      for (let i = nodes.length - 1; i >= 0; i--) {
        nextAnchor[i] = after;
        if (b.has(nodes[i].id) && merged.has(nodes[i].id)) after = nodes[i].id;
      }
      for (let i = 0; i < nodes.length; i++) {
        const id = nodes[i].id;
        if (b.has(id) && merged.has(id)) before = id;
        if (b.has(id) || !merged.has(id)) continue;
        const next = nextAnchor[i];
        if (before && next && orderPosition.get(before) >= orderPosition.get(next))
          this.values.report('INSERT_ANCHOR', id, path, '新增块的前后锚点因移序互相矛盾。');
````

## 第 8 步：MergeBlocks（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 174 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!additions.has(slot)</code>。
- 在本段的内部函数内：检查 <code>!emitted.has(id)</code>。
- 检查 <code>added</code>。
- 检查 <code>added.MAIN.length &amp;&amp; added.SUBMITTED.length &amp;&amp; !equal(added.MAIN, added.SUBMITTED)</code>。
- 检查 <code>i &lt; order.length</code>。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=6113:7080 -->
````javascript
        const slot = before ? orderPosition.get(before) + 1 : next ? orderPosition.get(next) : 0;
        if (!additions.has(slot)) additions.set(slot, { MAIN: [], SUBMITTED: [] });
        additions.get(slot)[side].push(id);
      }
    }
    const result = [],
      emitted = new Set();
    const emit = (id) => {
      if (!emitted.has(id)) {
        emitted.add(id);
        result.push(merged.get(id));
      }
    };
    for (let i = 0; i <= order.length; i++) {
      const added = additions.get(i);
      if (added) {
        if (added.MAIN.length && added.SUBMITTED.length && !equal(added.MAIN, added.SUBMITTED))
          this.values.report(
            'INSERT_ORDER',
            added.MAIN[0],
            `${path}@${i}`,
            '同一结构位置的两组新增块没有可确认顺序。',
          );
        [...added.MAIN, ...added.SUBMITTED].forEach(emit);
      }
      if (i < order.length) emit(order[i]);
    }
    return result;
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/merge/blocks.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
