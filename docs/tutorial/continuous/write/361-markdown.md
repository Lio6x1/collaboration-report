# 分步编写 `packages/content/markdown.mjs`

先按 [C01 · 正文契约、隔离候选与连续日志恢复](../lessons/collaboration/01-content-replay.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/361-markdown.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把Markdown导入为受约束的正文树，同时列出格式损失和未转存外链图片。

导入必须沿同一正文schema；不能让Markdown HTML或图片地址绕过资源保护。

写之前先按这条执行路线理解各段的连接：

1. markdown-it关闭HTML、linkify与typographer，注册行内/块公式token
2. importMarkdown用stack构造块树，newId为每个块分配稳定ID，marks描述行内格式
3. resources将已转存URL映射到resourceId；未映射图片变为提示文字并进入externalImages
4. 最终经normalizeContent统一校验，返回正文和losses

本文件需要守住的边界：

- 输入超过2MiB或节点过多应失败，不能直接吞掉超限部分
- 公式作为原子属性保存，不能当HTML执行；导入图片前须取得同空间READY资源

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/content' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/content/markdown.mjs') -and (Get-Item -LiteralPath 'packages/content/markdown.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/content/markdown.mjs')) { New-Item -ItemType File -Path 'packages/content/markdown.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:98 -->
````javascript
import MarkdownIt from 'markdown-it';
import { normalizeContent, atomFields } from './index.mjs';
````

## 第 2 步：markdown

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`const markdown = new MarkdownIt({ html: false, linkify: false, typographer: false, breaks: false });`。

<!-- write-step: 2 bytes=98:199 -->
````javascript
const markdown = new MarkdownIt({ html: false, linkify: false, typographer: false, breaks: false });
````

## 第 3 步：statement-245

插入位置：第 2 步代码之后，紧接追加；完成稿约第 4 行。

数学只产生受控原子节点；LaTeX本身不是HTML，也不能让导入器执行宏或外部链接。

本步声明：`markdown.inline.ruler.before('escape', 'collab_math', (state, silent) => {`。

<!-- write-step: 3 bytes=199:874 -->
````javascript
// 数学只产生受控原子节点；LaTeX本身不是HTML，也不能让导入器执行宏或外部链接。
markdown.inline.ruler.before('escape', 'collab_math', (state, silent) => {
  const start = state.pos;
  if (state.src[start] !== '$' || state.src[start + 1] === '$') return false;
  let end = start + 1;
  while ((end = state.src.indexOf('$', end)) >= 0 && state.src[end - 1] === '\\') end++;
  if (end < 0 || end === start + 1 || state.src.slice(start + 1, end).includes('\n')) return false;
  if (!silent) {
    const token = state.push('collab_formula', '', 0);
    token.content = state.src.slice(start + 1, end);
  }
  state.pos = end + 1;
  return true;
});
````

## 第 4 步：statement-808

插入位置：第 3 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`markdown.block.ruler.before(`。

<!-- write-step: 4 bytes=874:1911 -->
````javascript
markdown.block.ruler.before(
  'fence',
  'collab_math_block',
  (state, start, end, silent) => {
    const first = state.src
      .slice(state.bMarks[start] + state.tShift[start], state.eMarks[start])
      .trim();
    if (!first.startsWith('$$')) return false;
    let stop = start,
      latex;
    if (first.length > 4 && first.endsWith('$$')) latex = first.slice(2, -2);
    else {
      if (first !== '$$') return false;
      stop = start + 1;
      while (
        stop < end &&
        state.src.slice(state.bMarks[stop] + state.tShift[stop], state.eMarks[stop]).trim() !== '$$'
      )
        stop++;
      if (stop >= end) return false;
      latex = state.getLines(start + 1, stop, state.blkIndent, false).replace(/\n$/, '');
    }
    if (silent) return true;
    const token = state.push('collab_formula_block', '', 0);
    token.block = true;
    token.content = latex;
    token.map = [start, stop + 1];
    state.line = stop + 1;
    return true;
  },
  { alt: ['paragraph', 'reference', 'blockquote', 'list'] },
);

````

## 第 5 步：importMarkdown

插入位置：第 4 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`export function importMarkdown(`。

**这一段为什么这样写：** 把导入成功与有损转换同时告诉调用方。

1. 先限制输入体积
2. 逐token维护块栈和行内marks
3. 外链图片保留待转存信息
4. 校验完整正文后输出


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>text</code> | <code>JavaScript 参数</code> | 用户粘贴或上传的Markdown字符串<br>在此声明中的实际使用：<code>typeof text !== &#x27;string&#x27;</code>；<code>new TextEncoder().encode(text)</code>；<code>markdown.parse(text, {})</code> |
| <code>{ newId = () =&gt; crypto.randomUUID(), resources = new Map(), workspaceKind = &#x27;BRANCH&#x27; } = {}</code> | <code>带默认值的参数</code> | newId分配块ID；resources映射已转存图片；workspaceKind决定可用格式<br>在此声明中的实际使用：<code>newId()</code>；<code>resources.get</code>；<code>normalizeContent(body, { workspaceKind })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     body: normalizeContent(body, { workspaceKind }),     losses: &#91;...new Set(losses)&#93;,     externalImages,   }</code> |

**跟着本段代码走一遍：**

- 检查 <code>typeof text !== &#x27;string&#x27; &#124;&#124; new TextEncoder().encode(text).length &gt; 2 * 1024 * 1024</code>。条件成立时执行 <code>throw Error(&#x27;MARKDOWN_TOO_LARGE&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;MARKDOWN_TOO_LARGE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>++count &gt; 100000</code>。条件成立时执行 <code>throw Error(&#x27;MARKDOWN_TOO_LARGE&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw Error(&#x27;MARKDOWN_TOO_LARGE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>token.type.endsWith(&#x27;_open&#x27;)</code>。
- 在本段的内部函数内：检查 <code>type</code>。
- 在本段的内部函数内：检查 <code>token.type.endsWith(&#x27;_close&#x27;)</code>。

<!-- write-step: 5 bytes=1911:3387 -->
````javascript
export function importMarkdown(
  text,
  { newId = () => crypto.randomUUID(), resources = new Map(), workspaceKind = 'BRANCH' } = {},
) {
  if (typeof text !== 'string' || new TextEncoder().encode(text).length > 2 * 1024 * 1024)
    throw Error('MARKDOWN_TOO_LARGE');
  const body = { type: 'doc', content: [] },
    stack = [body],
    losses = [],
    externalImages = [];
  let count = 0;
  const current = () => stack.at(-1),
    push = (n) => {
      if (++count > 100000) throw Error('MARKDOWN_TOO_LARGE');
      current().content ??= [];
      current().content.push(n);
    },
    open = (type, attrs = {}) => {
      const n = { type, attrs: { id: newId(), ...attrs }, content: [] };
      push(n);
      stack.push(n);
    };
  function inline(tokens) {
    const marks = [],
      content = [];
    for (const token of tokens ?? []) {
      if (token.type.endsWith('_open')) {
        const type = { strong_open: 'strong', em_open: 'em', s_open: 'strike', link_open: 'link' }[
          token.type
        ];
        if (type)
          marks.push(
            type === 'link'
              ? {
                  type,
                  attrs: { href: token.attrGet('href'), title: token.attrGet('title') ?? null },
                }
              : { type },
          );
        else losses.push('无法表示的内联格式已保留为文本');
        continue;
      }
      if (token.type.endsWith('_close')) {
        marks.pop();
        continue;
      }
````

## 第 6 步：importMarkdown（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 97 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>token.type === &#x27;image&#x27;</code>。
- 在本段的内部函数内：检查 <code>resource</code>。
- 在本段的内部函数内：检查 <code>token.type === &#x27;collab_formula&#x27;</code>。
- 在本段的内部函数内：检查 <code>token.type === &#x27;hardbreak&#x27;</code>。
- 在本段的内部函数内：检查 <code>!value</code>。条件成立时执行 <code>continue;</code>。跳过当前循环本轮剩余语句。
- 在本段的内部函数内：此处通过 <code>return content;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3387:5207 -->
````javascript
      if (token.type === 'image') {
        const url = token.attrGet('src'),
          resource = resources.get(url);
        if (resource) {
          content.push({
            type: 'image',
            attrs: {
              resourceId: resource,
              alt: token.content || null,
              title: token.attrGet('title') ?? null,
              ...atomFields(marks),
            },
          });
        } else {
          externalImages.push({ url, alt: token.content });
          losses.push('外部图片尚未转存为同空间READY资源');
          content.push({
            type: 'text',
            text: '[图片待转存：' + (token.content || url) + ']',
            ...(marks.length ? { marks: structuredClone(marks) } : {}),
          });
        }
        continue;
      }
      if (token.type === 'collab_formula') {
        content.push({ type: 'formula', attrs: { latex: token.content, ...atomFields(marks) } });
        continue;
      }
      if (token.type === 'hardbreak') {
        content.push({ type: 'hard_break' });
        continue;
      }
      const value = token.type === 'softbreak' ? '\n' : token.content;
      if (!value) continue;
      const ownMarks = token.type === 'code_inline' ? [...marks, { type: 'code' }] : marks;
      content.push({
        type: 'text',
        text: value,
        ...(ownMarks.length ? { marks: structuredClone(ownMarks) } : {}),
      });
    }
    return content;
  }
  for (const token of markdown.parse(text, {})) {
    const block = {
      paragraph_open: 'paragraph',
      heading_open: 'heading',
      blockquote_open: 'blockquote',
      bullet_list_open: 'bullet_list',
      ordered_list_open: 'ordered_list',
      list_item_open: 'list_item',
      table_open: 'table',
      tr_open: 'table_row',
    }[token.type];
````

## 第 7 步：importMarkdown（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 151 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>block</code>。
- 检查 <code>&#91;&#x27;thead_open&#x27;, &#x27;thead_close&#x27;, &#x27;tbody_open&#x27;, &#x27;tbody_close&#x27;&#93;.includes(token.type)</code>。条件成立时执行 <code>continue;</code>。跳过当前循环本轮剩余语句。
- 检查 <code>token.type === &#x27;th_open&#x27; &#124;&#124; token.type === &#x27;td_open&#x27;</code>。
- 检查 <code>token.attrGet(&#x27;style&#x27;)</code>。
- 检查 <code>token.type === &#x27;th_close&#x27; &#124;&#124; token.type === &#x27;td_close&#x27;</code>。
- 检查 <code>token.nesting === -1</code>。
- 检查 <code>token.type === &#x27;inline&#x27;</code>。
- 检查 <code>token.type === &#x27;fence&#x27; &#124;&#124; token.type === &#x27;code_block&#x27;</code>。
- 检查 <code>token.type === &#x27;hr&#x27;</code>。

<!-- write-step: 7 bytes=5207:6657 -->
````javascript
    if (block) {
      const attrs =
        block === 'heading'
          ? { level: Number(token.tag.slice(1)) }
          : block === 'ordered_list'
            ? { order: Number(token.attrGet('start') ?? 1) }
            : {};
      open(block, attrs);
      continue;
    }
    if (['thead_open', 'thead_close', 'tbody_open', 'tbody_close'].includes(token.type)) continue;
    if (token.type === 'th_open' || token.type === 'td_open') {
      if (token.attrGet('style')) losses.push('Markdown表格对齐已规范化');
      open(token.type === 'th_open' ? 'table_header' : 'table_cell', {
        colspan: 1,
        rowspan: 1,
        colwidth: null,
      });
      open('paragraph');
      continue;
    }
    if (token.type === 'th_close' || token.type === 'td_close') {
      stack.pop();
      stack.pop();
      continue;
    }
    if (token.nesting === -1) {
      stack.pop();
      continue;
    }
    if (token.type === 'inline') {
      current().content.push(...inline(token.children));
      continue;
    }
    if (token.type === 'fence' || token.type === 'code_block') {
      push({
        type: 'code_block',
        attrs: { id: newId(), language: token.info.trim().split(/\s+/)[0] || null },
        content: token.content ? [{ type: 'text', text: token.content }] : [],
      });
      continue;
    }
    if (token.type === 'hr') {
      push({ type: 'horizontal_rule', attrs: { id: newId() } });
      continue;
    }
````

## 第 8 步：importMarkdown（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 197 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>token.type === &#x27;collab_formula_block&#x27;</code>。
- 检查 <code>token.content</code>。
- 检查 <code>!body.content.length</code>。
- 此处通过 <code>return {     body: normalizeContent(body, { workspaceKind }),     losses: &#91;...new Set(losses)&#93;,     externalImages,   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=6657:7335 -->
````javascript
    if (token.type === 'collab_formula_block') {
      push({
        type: 'formula_block',
        attrs: { id: newId(), latex: token.content, ...atomFields() },
      });
      continue;
    }
    if (token.content) {
      losses.push('无法表示的块格式已保留为普通文本');
      push({
        type: 'paragraph',
        attrs: { id: newId() },
        content: [{ type: 'text', text: token.content }],
      });
    }
  }
  if (!body.content.length)
    body.content.push({ type: 'paragraph', attrs: { id: newId() }, content: [] });
  return {
    body: normalizeContent(body, { workspaceKind }),
    losses: [...new Set(losses)],
    externalImages,
  };
}
````

## 文件完成后的检查

```powershell
node --check 'packages/content/markdown.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/01-content-replay.md)。
