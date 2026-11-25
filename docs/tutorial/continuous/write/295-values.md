# 分步编写 `services/collaboration/src/domain/merge/values.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/295-values.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供三方合并中的值相等、语义相等、属性和标记合并规则。

支线不拥有正式字体控制；文字语义合并要忽略font差异，同时保留MAIN字体。

写之前先按这条执行路线理解各段的连接：

1. semantic递归剔除font标记，same用规范JSON比较语义
2. MergeValues.scalar按base/submitted/main判断单侧修改或同改，双侧不同修改记ATTRIBUTE_CONFLICT
3. marks按标记类型合并，font始终取main
4. report收集block_id/node_path/kind/reason交上层统一拒绝候选

本文件需要守住的边界：

- 冲突时内部暂取MAIN只是方便继续收集冲突，不能把临时正文作为成功合并
- undefined代表字段缺失，不能与null混为一谈

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain/merge' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/merge/values.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/merge/values.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/merge/values.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/merge/values.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:72 -->
````javascript
import { canonical } from '../../../../../packages/content/index.mjs';

````

## 第 2 步：clone

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`export const clone = (value) => (`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>value === undefined</code>；<code>structuredClone(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>value === undefined ? undefined : structuredClone(value)</code> |

<!-- write-step: 2 bytes=72:164 -->
````javascript
export const clone = (value) => (value === undefined ? undefined : structuredClone(value));
````

## 第 3 步：equal

插入位置：第 2 步代码之后，紧接追加；完成稿约第 4 行。

本步声明：`export const equal = (left, right) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>left</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>left === undefined</code>；<code>left === right</code>；<code>canonical(left)</code> |
| <code>right</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>right === undefined</code>；<code>left === right</code>；<code>canonical(right)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>left === undefined &#124;&#124; right === undefined ? left === right : canonical(left) === canonical(right)</code> |

<!-- write-step: 3 bytes=164:304 -->
````javascript
export const equal = (left, right) =>
  left === undefined || right === undefined ? left === right : canonical(left) === canonical(right);

````

## 第 4 步：semantic

插入位置：第 3 步代码之后，紧接追加；完成稿约第 7 行。

分支没有正文font权限；主线font不构成送审语义差异，但最终结果必须保留它。

本步声明：`export function semantic(value) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Array.isArray(value)</code>；<code>value.map</code>；<code>!value &#124;&#124; typeof value !== &#x27;object&#x27;</code>；<code>typeof value !== &#x27;object&#x27;</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>value.map(semantic)</code>；<code>value</code>；<code>result</code> |

**跟着本段代码走一遍：**

- 检查 <code>Array.isArray(value)</code>。条件成立时执行 <code>return value.map(semantic);</code>。当前函数在这里结束。
- 检查 <code>!value &#124;&#124; typeof value !== &#x27;object&#x27;</code>。条件成立时执行 <code>return value;</code>。当前函数在这里结束。
- 检查 <code>key !== &#x27;marks&#x27; &#124;&#124; next.length</code>。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=304:852 -->
````javascript
// 分支没有正文font权限；主线font不构成送审语义差异，但最终结果必须保留它。
export function semantic(value) {
  if (Array.isArray(value)) return value.map(semantic);
  if (!value || typeof value !== 'object') return value;
  const result = {};
  for (const [key, child] of Object.entries(value)) {
    const next =
      key === 'marks'
        ? child.filter((mark) => mark.type !== 'font').map(semantic)
        : semantic(child);
    if (key !== 'marks' || next.length) result[key] = next;
  }
  return result;
}
````

## 第 5 步：same

插入位置：第 4 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`export const same = (left, right) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>left</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>semantic(left)</code> |
| <code>right</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>semantic(right)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>equal(semantic(left), semantic(right))</code> |

<!-- write-step: 5 bytes=852:929 -->
````javascript
export const same = (left, right) => equal(semantic(left), semantic(right));
````

## 第 6 步：hasFont

插入位置：第 5 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`export function hasFont(node) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>node</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>node.marks</code>；<code>node.content</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>(node.marks ?? &#91;&#93;).some((mark) =&gt; mark.type === &#x27;font&#x27;) &#124;&#124; (node.content ?? &#91;&#93;).some(hasFont)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return (     (node.marks ?? &#91;&#93;).some((mark) =&gt; mark.type === &#x27;font&#x27;) &#124;&#124; (node.content ?? &#91;&#93;).some(hasFont)   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=929:1078 -->
````javascript
export function hasFont(node) {
  return (
    (node.marks ?? []).some((mark) => mark.type === 'font') || (node.content ?? []).some(hasFont)
  );
}

````

## 第 7 步：MergeValues

插入位置：第 6 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`export class MergeValues `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>conflicts</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.conflicts = &#91;&#93;</code> |

**本段进入 MergeValues.constructor：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 MergeValues.report：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>kind</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.conflicts.push({ block_id: block, node_path: path, kind, reason })</code> |
| <code>block</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.conflicts.push({ block_id: block, node_path: path, kind, reason })</code> |
| <code>path</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.conflicts.push({ block_id: block, node_path: path, kind, reason })</code> |
| <code>reason</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.conflicts.push({ block_id: block, node_path: path, kind, reason })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。 |

**本段进入 MergeValues.scalar：**

**这一段为什么这样写：** 识别同一字段到底是单侧修改还是冲突。

1. main与submitted相同或submitted等于base时保留main
2. main等于base时接受submitted
3. 否则报告冲突并临时保留main


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>JavaScript 参数</code> | 共同基线字段值<br>在此声明中的实际使用：<code>equal(base, submitted)</code>；<code>equal(base, main)</code> |
| <code>submitted</code> | <code>JavaScript 参数</code> | 固定送审字段值<br>在此声明中的实际使用：<code>equal(main, submitted)</code>；<code>equal(base, submitted)</code>；<code>clone(submitted)</code> |
| <code>main</code> | <code>JavaScript 参数</code> | 可靠主线字段值<br>在此声明中的实际使用：<code>equal(main, submitted)</code>；<code>clone(main)</code>；<code>equal(base, main)</code> |
| <code>block</code> | <code>JavaScript 参数</code> | 所属块ID<br>在此声明中的实际使用：<code>this.report(&#x27;ATTRIBUTE_CONFLICT&#x27;, block, path, &#x27;同一字段在主线和送审中发生了不同修改。&#x27;)</code> |
| <code>path</code> | <code>JavaScript 参数</code> | 冲突字段的定位路径<br>在此声明中的实际使用：<code>this.report(&#x27;ATTRIBUTE_CONFLICT&#x27;, block, path, &#x27;同一字段在主线和送审中发生了不同修改。&#x27;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>clone(main)</code>；<code>clone(submitted)</code> |

**本段进入 MergeValues.attributes：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base = {}</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>Object.keys(base)</code>；<code>base&#91;key&#93;</code> |
| <code>submitted = {}</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>Object.keys(submitted)</code>；<code>submitted&#91;key&#93;</code> |
| <code>main = {}</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>Object.keys(main)</code>；<code>main&#91;key&#93;</code> |
| <code>block</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.scalar(base&#91;key&#93;, submitted&#91;key&#93;, main&#91;key&#93;, block, `${path}.${key}`)</code> |
| <code>path</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.scalar(base&#91;key&#93;, submitted&#91;key&#93;, main&#91;key&#93;, block, `${path}.${key}`)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>result</code> |

**本段进入 MergeValues.marks：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>map(base)</code> |
| <code>submitted</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>map(submitted)</code> |
| <code>main</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>map(main)</code> |
| <code>block</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.attributes(             b&#91;kind&#93;.attrs,             s&#91;kind&#93;.attrs,             m&#91;kind&#93;.attrs,             block,             `${path}.marks.${kind}.attrs`,           )</code>；<code>this.scalar(b&#91;kind&#93;, s&#91;kind&#93;, m&#91;kind&#93;, block, `${path}.marks.${kind}`)</code> |
| <code>path</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.attributes(             b&#91;kind&#93;.attrs,             s&#91;kind&#93;.attrs,             m&#91;kind&#93;.attrs,             block,             `${path}.marks.${kind}.attrs`,           )</code>；<code>this.scalar(b&#91;kind&#93;, s&#91;kind&#93;, m&#91;kind&#93;, block, `${path}.marks.${kind}`)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>result</code> |

**跟着本段代码走一遍：**

- 检查 <code>equal(main, submitted) &#124;&#124; equal(base, submitted)</code>。条件成立时执行 <code>return clone(main);</code>。当前函数在这里结束。
- 检查 <code>equal(base, main)</code>。条件成立时执行 <code>return clone(submitted);</code>。当前函数在这里结束。
- 此处通过 <code>return clone(main);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>value !== undefined</code>。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>kind === &#x27;font&#x27;</code>。
- 检查 <code>b&#91;kind&#93;?.attrs &amp;&amp; s&#91;kind&#93;?.attrs &amp;&amp; m&#91;kind&#93;?.attrs</code>。

<!-- write-step: 7 bytes=1078:2815 -->
````javascript
export class MergeValues {
  constructor() {
    this.conflicts = [];
  }

  report(kind, block, path, reason) {
    this.conflicts.push({ block_id: block, node_path: path, kind, reason });
  }

  scalar(base, submitted, main, block, path) {
    if (equal(main, submitted) || equal(base, submitted)) return clone(main);
    if (equal(base, main)) return clone(submitted);
    this.report('ATTRIBUTE_CONFLICT', block, path, '同一字段在主线和送审中发生了不同修改。');
    return clone(main);
  }

  attributes(base = {}, submitted = {}, main = {}, block, path) {
    const result = {};
    for (const key of new Set([
      ...Object.keys(base),
      ...Object.keys(submitted),
      ...Object.keys(main),
    ])) {
      const value = this.scalar(base[key], submitted[key], main[key], block, `${path}.${key}`);
      if (value !== undefined) result[key] = value;
    }
    return result;
  }

  marks(base, submitted, main, block, path) {
    const map = (token) =>
      Object.fromEntries((token?.marks ?? []).map((mark) => [mark.type, mark]));
    const b = map(base),
      s = map(submitted),
      m = map(main),
      result = [];
    for (const kind of [
      ...new Set([...Object.keys(b), ...Object.keys(s), ...Object.keys(m)]),
    ].sort()) {
      let mark;
      if (kind === 'font') mark = clone(m.font);
      else if (b[kind]?.attrs && s[kind]?.attrs && m[kind]?.attrs)
        mark = {
          type: kind,
          attrs: this.attributes(
            b[kind].attrs,
            s[kind].attrs,
            m[kind].attrs,
            block,
            `${path}.marks.${kind}.attrs`,
          ),
        };
      else mark = this.scalar(b[kind], s[kind], m[kind], block, `${path}.marks.${kind}`);
````

## 第 8 步：MergeValues（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 81 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 MergeValues.token：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.marks(base, submitted, main, block, path)</code>；<code>base?.attrs</code> |
| <code>submitted</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.marks(base, submitted, main, block, path)</code>；<code>submitted.attrs</code> |
| <code>main</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>main.type</code>；<code>clone(main)</code>；<code>this.marks(base, submitted, main, block, path)</code>；<code>main.attrs</code> |
| <code>block</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.marks(base, submitted, main, block, path)</code>；<code>this.attributes(base?.attrs, submitted.attrs, main.attrs, block, `${path}.attrs`)</code> |
| <code>path</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.marks(base, submitted, main, block, path)</code>；<code>this.attributes(base?.attrs, submitted.attrs, main.attrs, block, `${path}.attrs`)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ ...clone(main), marks: this.marks(base, submitted, main, block, path) }</code>；<code>clone(main)</code>；<code>{       ...clone(main),       attrs: this.attributes(base?.attrs, submitted.attrs, main.attrs, block, `${path}.attrs`),     }</code> |

**跟着本段代码走一遍：**

- 检查 <code>mark</code>。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>main.type === &#x27;text&#x27;</code>。条件成立时执行 <code>return { ...clone(main), marks: this.marks(base, submitted, main, block, path) };</code>。当前函数在这里结束。
- 检查 <code>!base?.attrs &amp;&amp; !submitted.attrs &amp;&amp; !main.attrs</code>。条件成立时执行 <code>return clone(main);</code>。当前函数在这里结束。
- 此处通过 <code>return {       ...clone(main),       attrs: this.attributes(base?.attrs, submitted.attrs, main.attrs, block, `${path}.attrs`),     };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2815:3372 -->
````javascript
      if (mark) result.push(mark);
    }
    return result;
  }

  token(base, submitted, main, block, path) {
    if (main.type === 'text')
      return { ...clone(main), marks: this.marks(base, submitted, main, block, path) };
    // hard_break没有attrs；不能凭合并层的通用对象形状添加schema不允许的空字段。
    if (!base?.attrs && !submitted.attrs && !main.attrs) return clone(main);
    return {
      ...clone(main),
      attrs: this.attributes(base?.attrs, submitted.attrs, main.attrs, block, `${path}.attrs`),
    };
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/merge/values.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
