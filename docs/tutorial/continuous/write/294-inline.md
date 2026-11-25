# 分步编写 `services/collaboration/src/domain/merge/inline.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/294-inline.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按字素及原子节点合并段落内部的双侧修改。

按UTF-16字符切分会拆开emoji/组合字符；只比较文本又会丢掉并发格式和图片/公式属性修改。

写之前先按这条执行路线理解各段的连接：

1. Intl.Segmenter把文字切成grapheme，原子节点保持整体
2. changes构造基线到两侧的映射与diff区间
3. 相同插入合并格式，重叠编辑或删除另一侧格式修改时报冲突
4. 将MAIN和SUBMITTED hunks按位置合并，新送审文字继承附近MAIN字体，再拼回相同marks文字

本文件需要守住的边界：

- 超过200000估计字符预算返回MERGE_COMPLEXITY_LIMIT
- 同位置不同插入不应随意按时间排序，应交给用户处理

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain/merge' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/merge/inline.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/merge/inline.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/merge/inline.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/merge/inline.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:105 -->
````javascript
import { diff } from 'lib0/diff/patience';
import { clone, equal, semantic, same } from './values.mjs';

````

## 第 2 步：segmenter

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

本步声明：`const segmenter = new Intl.Segmenter('zh', { granularity: 'grapheme' });`。

<!-- write-step: 2 bytes=105:178 -->
````javascript
const segmenter = new Intl.Segmenter('zh', { granularity: 'grapheme' });
````

## 第 3 步：tokenKey

插入位置：第 2 步代码之后，紧接追加；完成稿约第 5 行。

本步声明：`const tokenKey = (token) => (`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>token</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>token.type</code>；<code>token.text</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>token.type === &#x27;text&#x27; ? &#x27;text:&#x27; + token.text : &#x27;atom:&#x27; + token.type</code> |

<!-- write-step: 3 bytes=178:277 -->
````javascript
const tokenKey = (token) => (token.type === 'text' ? 'text:' + token.text : 'atom:' + token.type);
````

## 第 4 步：tokens

插入位置：第 3 步代码之后，紧接追加；完成稿约第 6 行。

本步声明：`function tokens(node) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>node</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>node.content</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>(node.content ?? &#91;&#93;).flatMap((child) =&gt;     child.type === &#x27;text&#x27;       ? &#91;...segmenter.segment(child.text)&#93;.map(({ segment }) =&gt; ({           ...clone(child),           text: segm …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return (node.content ?? &#91;&#93;).flatMap((child) =&gt;     child.type === &#x27;text&#x27;       ? &#91;...segmenter.segment(child.text)&#93;.map(({ segment }) =&gt; ({           ...clone(child),           tex …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=277:538 -->
````javascript
function tokens(node) {
  return (node.content ?? []).flatMap((child) =>
    child.type === 'text'
      ? [...segmenter.segment(child.text)].map(({ segment }) => ({
          ...clone(child),
          text: segment,
        }))
      : [clone(child)],
  );
}
````

## 第 5 步：changes

插入位置：第 4 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`function changes(base, side) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>base.map</code>；<code>base.length</code> |
| <code>side</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>side.map</code>；<code>side.slice</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ mapping, hunks }</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return { mapping, hunks };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=538:1058 -->
````javascript
function changes(base, side) {
  const mapping = new Map();
  let bi = 0,
    si = 0;
  const hunks = diff(base.map(tokenKey), side.map(tokenKey)).map((hunk) => {
    while (bi < hunk.index) mapping.set(bi++, si++);
    const result = {
      start: bi,
      end: bi + hunk.remove.length,
      insert: side.slice(si, si + hunk.insert.length),
    };
    bi += hunk.remove.length;
    si += hunk.insert.length;
    return result;
  });
  while (bi < base.length) mapping.set(bi++, si++);
  return { mapping, hunks };
}
````

## 第 6 步：overlap

插入位置：第 5 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`function overlap(left, right) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>left</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>left.start</code>；<code>left.end</code> |
| <code>right</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>right.start</code>；<code>right.end</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>left.start === right.start</code>；<code>left.start &gt;= right.start &amp;&amp; left.start &lt;= right.end</code>；<code>right.start &gt;= left.start &amp;&amp; right.start &lt;= left.end</code>；<code>left.start &lt; right.end &amp;&amp; right.start &lt; left.end</code> |

**跟着本段代码走一遍：**

- 检查 <code>left.start === left.end &amp;&amp; right.start === right.end</code>。条件成立时执行 <code>return left.start === right.start;</code>。当前函数在这里结束。
- 检查 <code>left.start === left.end</code>。条件成立时执行 <code>return left.start &gt;= right.start &amp;&amp; left.start &lt;= right.end;</code>。当前函数在这里结束。
- 检查 <code>right.start === right.end</code>。条件成立时执行 <code>return right.start &gt;= left.start &amp;&amp; right.start &lt;= left.end;</code>。当前函数在这里结束。
- 此处通过 <code>return left.start &lt; right.end &amp;&amp; right.start &lt; left.end;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1058:1432 -->
````javascript
function overlap(left, right) {
  if (left.start === left.end && right.start === right.end) return left.start === right.start;
  if (left.start === left.end) return left.start >= right.start && left.start <= right.end;
  if (right.start === right.end) return right.start >= left.start && right.start <= left.end;
  return left.start < right.end && right.start < left.end;
}
````

## 第 7 步：untokenize

插入位置：第 6 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`function untokenize(list) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>list</code> | <code>JavaScript 参数</code> | 合并计算后的行内token序列；逐个复制并把相邻且marks相同的文本合并回正文节点。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>result</code> |

**跟着本段代码走一遍：**

- 检查 <code>!node.marks?.length</code>。
- 检查 <code>node.type === &#x27;text&#x27; &amp;&amp; previous?.type === &#x27;text&#x27; &amp;&amp; equal(previous.marks, node.marks)</code>。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1432:1808 -->
````javascript
function untokenize(list) {
  const result = [];
  for (const token of list) {
    const node = clone(token);
    if (!node.marks?.length) delete node.marks;
    const previous = result.at(-1);
    if (node.type === 'text' && previous?.type === 'text' && equal(previous.marks, node.marks))
      previous.text += node.text;
    else result.push(node);
  }
  return result;
}

````

## 第 8 步：mergeInline

插入位置：第 7 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`export function mergeInline(values, baseNode, submittedNode, mainNode, block, path) `。

**这一段为什么这样写：** 在共享基线位置比较文本编辑并保留主线样式。

1. 先处理无需合并和复杂度上限
2. 检查重叠与删除格式冲突
3. 按基线顺序组合不冲突区间
4. untokenize恢复正文节点


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>values</code> | <code>JavaScript 参数</code> | 收集冲突并处理属性/标记的MergeValues<br>在此声明中的实际使用：<code>values.report</code>；<code>values.token</code> |
| <code>baseNode</code> | <code>JavaScript 参数</code> | 共同基线段落<br>在此声明中的实际使用：<code>baseNode.content</code>；<code>&#91;baseNode, submittedNode, mainNode&#93;.reduce</code>；<code>tokens(baseNode)</code> |
| <code>submittedNode</code> | <code>JavaScript 参数</code> | 固定送审段落<br>在此声明中的实际使用：<code>submittedNode.content</code>；<code>&#91;baseNode, submittedNode, mainNode&#93;.reduce</code>；<code>tokens(submittedNode)</code> |
| <code>mainNode</code> | <code>JavaScript 参数</code> | 当前固定主线段落<br>在此声明中的实际使用：<code>mainNode.content</code>；<code>&#91;baseNode, submittedNode, mainNode&#93;.reduce</code>；<code>tokens(mainNode)</code> |
| <code>block</code> | <code>JavaScript 参数</code> | 冲突归属块ID<br>在此声明中的实际使用：<code>values.report(       &#x27;MERGE_COMPLEXITY_LIMIT&#x27;,       block,       path,       &#x27;本段双侧改动超出自动合并计算预算，请分段处理。&#x27;,     )</code>；<code>values.token(undefined, sh.insert&#91;i&#93;, token, block, `${path}@${sh.start}+${i}`)</code>；<code>values.report(&#x27;TEXT_OVERLAP&#x27;, block, path, &#x27;同一字素范围被不同修改或同位置插入占用。&#x27;)</code>；<code>values.report(         &#x27;DELETE_FORMAT&#x27;,         block,         `${path}@${i}`,         &#x27;删除或替换会丢弃另一侧保留内容上的格式或原子字段修改。&#x27;,       )</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>path</code> | <code>JavaScript 参数</code> | 正文节点路径<br>在此声明中的实际使用：<code>values.report(       &#x27;MERGE_COMPLEXITY_LIMIT&#x27;,       block,       path,       &#x27;本段双侧改动超出自动合并计算预算，请分段处理。&#x27;,     )</code>；<code>values.token(undefined, sh.insert&#91;i&#93;, token, block, `${path}@${sh.start}+${i}`)</code>；<code>values.report(&#x27;TEXT_OVERLAP&#x27;, block, path, &#x27;同一字素范围被不同修改或同位置插入占用。&#x27;)</code>；<code>values.report(         &#x27;DELETE_FORMAT&#x27;,         block,         `${path}@${i}`,         &#x27;删除或替换会丢弃另一侧保留内容上的格式或原子字段修改。&#x27;,       )</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>clone(mainNode.content ?? &#91;&#93;)</code>；<code>&#91;&#93;</code>；<code>untokenize(result)</code> |

**跟着本段代码走一遍：**

- 检查 <code>same(baseNode.content, submittedNode.content)</code>。条件成立时执行 <code>return clone(mainNode.content ?? &#91;&#93;);</code>。当前函数在这里结束。
- 检查 <code>estimate &gt; 200000</code>。
- 此处通过 <code>return &#91;&#93;;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>sh.start === mh.start &amp;&amp;         sh.end === mh.end &amp;&amp;         equal(sh.insert.map(tokenKey), mh.insert.map(tokenKey))</code>。
- 检查 <code>overlap(sh, mh)</code>。
- 此处通过 <code>return &#91;&#93;;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1808:3457 -->
````javascript
export function mergeInline(values, baseNode, submittedNode, mainNode, block, path) {
  if (same(baseNode.content, submittedNode.content)) return clone(mainNode.content ?? []);
  // 先按UTF-16长度保守估算，再分字素。否则仅为检查预算先分词也会阻塞事件循环。
  const estimate = [baseNode, submittedNode, mainNode].reduce(
    (total, node) =>
      total +
      (node.content ?? []).reduce(
        (size, child) => size + (child.type === 'text' ? child.text.length : 1),
        0,
      ),
    0,
  );
  if (estimate > 200000) {
    values.report(
      'MERGE_COMPLEXITY_LIMIT',
      block,
      path,
      '本段双侧改动超出自动合并计算预算，请分段处理。',
    );
    return [];
  }
  // diff API是锁定版本的实验接口；以字素作差异单位，不切断emoji或组合字符。
  const base = tokens(baseNode),
    submitted = tokens(submittedNode),
    main = tokens(mainNode);
  const sc = changes(base, submitted),
    mc = changes(base, main),
    duplicates = new Set();
  for (const [index, sh] of sc.hunks.entries()) {
    for (const mh of mc.hunks) {
      if (
        sh.start === mh.start &&
        sh.end === mh.end &&
        equal(sh.insert.map(tokenKey), mh.insert.map(tokenKey))
      ) {
        duplicates.add(index);
        mh.insert = mh.insert.map((token, i) =>
          values.token(undefined, sh.insert[i], token, block, `${path}@${sh.start}+${i}`),
        );
      } else if (overlap(sh, mh)) {
        values.report('TEXT_OVERLAP', block, path, '同一字素范围被不同修改或同位置插入占用。');
        return [];
      }
    }
  }
````

## 第 9 步：mergeInline（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 98 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>(si === undefined &amp;&amp; mi !== undefined &amp;&amp; !same(base&#91;i&#93;, main&#91;mi&#93;)) &#124;&#124;       (mi === undefined &amp;&amp; si !== undefined &amp;&amp; !same(base&#91;i&#93;, submitted&#91;si&#93;))</code>。
- 此处通过 <code>return &#91;&#93;;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>mc.mapping.has(index)</code>。条件成立时执行 <code>return markAt(index);</code>。当前函数在这里结束。
- 在本段的内部函数内：检查 <code>mc.mapping.has(i)</code>。条件成立时执行 <code>return markAt(i);</code>。当前函数在这里结束。
- 在本段的内部函数内：此处通过 <code>return undefined;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>hunk.side === &#x27;MAIN&#x27; &#124;&#124; token.type !== &#x27;text&#x27;</code>。条件成立时执行 <code>return clone(token);</code>。当前函数在这里结束。
- 在本段的内部函数内：此处通过 <code>return {           ...clone(token),           marks: &#91;...(semantic(token).marks ?? &#91;&#93;), ...(font ? &#91;clone(font)&#93; : &#91;&#93;)&#93;,         };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3457:5163 -->
````javascript
  for (let i = 0; i < base.length; i++) {
    const si = sc.mapping.get(i),
      mi = mc.mapping.get(i);
    if (
      (si === undefined && mi !== undefined && !same(base[i], main[mi])) ||
      (mi === undefined && si !== undefined && !same(base[i], submitted[si]))
    ) {
      values.report(
        'DELETE_FORMAT',
        block,
        `${path}@${i}`,
        '删除或替换会丢弃另一侧保留内容上的格式或原子字段修改。',
      );
      return [];
    }
  }
  const fontAt = (index) => {
    const markAt = (i) => main[mc.mapping.get(i)]?.marks?.find((mark) => mark.type === 'font');
    if (mc.mapping.has(index)) return markAt(index);
    for (let i = index - 1; i >= 0; i--) if (mc.mapping.has(i)) return markAt(i);
    for (let i = index + 1; i < base.length; i++) if (mc.mapping.has(i)) return markAt(i);
    return undefined;
  };
  const hunks = [
    ...mc.hunks.map((h) => ({ ...h, side: 'MAIN' })),
    ...sc.hunks.filter((_, i) => !duplicates.has(i)).map((h) => ({ ...h, side: 'SUBMITTED' })),
  ].sort((a, b) => a.start - b.start);
  const result = [];
  let position = 0;
  const keep = (i) =>
    values.token(
      base[i],
      submitted[sc.mapping.get(i)],
      main[mc.mapping.get(i)],
      block,
      `${path}@${i}`,
    );
  for (const hunk of hunks) {
    while (position < hunk.start) result.push(keep(position++));
    const font = fontAt(hunk.start);
    result.push(
      ...hunk.insert.map((token) => {
        if (hunk.side === 'MAIN' || token.type !== 'text') return clone(token);
        return {
          ...clone(token),
          marks: [...(semantic(token).marks ?? []), ...(font ? [clone(font)] : [])],
        };
      }),
    );
````

## 第 10 步：mergeInline（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 147 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return untokenize(result);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5163:5287 -->
````javascript
    position = hunk.end;
  }
  while (position < base.length) result.push(keep(position++));
  return untokenize(result);
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/merge/inline.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
