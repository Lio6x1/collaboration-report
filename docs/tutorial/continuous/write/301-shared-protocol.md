# 分步编写 `services/collaboration/src/domain/shared_protocol.mjs`

先按 [C01 · 正文契约、隔离候选与连续日志恢复](../lessons/collaboration/01-content-replay.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/301-shared-protocol.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

集中稳定业务错误、SHA-256、JSON兼容读取和操作ID格式。

底层SQL/TLS错误可能含正文或配置；传输层只能暴露受限业务码。

写之前先按这条执行路线理解各段的连接：

1. RPCFailure只接受大写稳定码，否则降为SERVICE_UNAVAILABLE与14
2. hash返回32字节摘要；json兼容mysql JSON对象/字符串
3. validID限制16至96字符，deny/bad分别构造权限/参数错误

本文件需要守住的边界：

- 业务ID、十进制实体ID和哈希不是同一种格式，不要互换
- 不要把驱动message直接交给RPCFailure期待原样返回

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/shared_protocol.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/shared_protocol.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/shared_protocol.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/shared_protocol.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:103 -->
````javascript
import { createHash } from 'node:crypto';

// 来源 errors.mjs：原业务声明与函数体保留。
````

## 第 2 步：RPCFailure

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

只传播稳定业务码；驱动原文不能进入HTTP/WSS/RPC错误或日志。

本步声明：`export class RPCFailure extends Error `。

**这一段为什么这样写：** 隔离内部异常文本和公开错误码。

1. 校验details正则
2. 非法details替换为SERVICE_UNAVAILABLE
3. 保存安全code/details


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>code</code> | <code>实例字段</code> | RPC状态码，非法详情统一14<br>在此声明中的实际使用：<code>this.code = safe === details ? code : 14</code> |
| <code>details</code> | <code>实例字段</code> | 可公开的稳定大写业务码<br>在此声明中的实际使用：<code>this.details = safe</code> |

**本段进入 RPCFailure.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>code</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.code = safe === details ? code : 14</code> |
| <code>details</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>typeof details === &#x27;string&#x27;</code>；<code>/^&#91;A-Z&#93;&#91;A-Z0-9_&#93;{2,63}$/.test(details)</code>；<code>safe === details</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

<!-- write-step: 2 bytes=103:498 -->
````javascript
// 只传播稳定业务码；驱动原文不能进入HTTP/WSS/RPC错误或日志。
export class RPCFailure extends Error {
  constructor(code, details) {
    const safe =
      typeof details === 'string' && /^[A-Z][A-Z0-9_]{2,63}$/.test(details)
        ? details
        : 'SERVICE_UNAVAILABLE';
    super(safe);
    this.code = safe === details ? code : 14;
    this.details = safe;
  }
}

````

## 第 3 步：hash

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

来源 bytes.mjs：原业务声明与函数体保留。

本步声明：`export const hash = (value) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>createHash(&#x27;sha256&#x27;).update(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>createHash(&#x27;sha256&#x27;).update(value).digest()</code> |

<!-- write-step: 3 bytes=498:635 -->
````javascript
// 来源 bytes.mjs：原业务声明与函数体保留。

export const hash = (value) => createHash('sha256').update(value).digest();

````

## 第 4 步：json

插入位置：第 3 步代码之后，紧接追加；完成稿约第 21 行。

来源 protocol.mjs：原业务声明与函数体保留。

本步声明：`export const json = (value) => (`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>typeof value === &#x27;string&#x27;</code>；<code>JSON.parse(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>typeof value === &#x27;string&#x27; ? JSON.parse(value) : value</code> |

<!-- write-step: 4 bytes=635:786 -->
````javascript
// 来源 protocol.mjs：原业务声明与函数体保留。

export const json = (value) => (typeof value === 'string' ? JSON.parse(value) : value);
````

## 第 5 步：validID

插入位置：第 4 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`export const validID = (value) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>typeof value === &#x27;string&#x27;</code>；<code>/^&#91;A-Za-z0-9&#93;&#91;A-Za-z0-9_-&#93;{15,95}$/.test(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>typeof value === &#x27;string&#x27; &amp;&amp; /^&#91;A-Za-z0-9&#93;&#91;A-Za-z0-9_-&#93;{15,95}$/.test(value)</code> |

<!-- write-step: 5 bytes=786:900 -->
````javascript
export const validID = (value) =>
  typeof value === 'string' && /^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$/.test(value);
````

## 第 6 步：deny

插入位置：第 5 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`export const deny = (code) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>code</code> | <code>JavaScript 参数</code> | 稳定业务错误标识；包装为gRPC PERMISSION_DENIED(7)，调用者仍需throw或返回错误。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>new RPCFailure(7, code)</code> |

<!-- write-step: 6 bytes=900:955 -->
````javascript
export const deny = (code) => new RPCFailure(7, code);
````

## 第 7 步：bad

插入位置：第 6 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`export const bad = (code) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>code</code> | <code>JavaScript 参数</code> | 稳定业务错误标识；包装为gRPC INVALID_ARGUMENT(3)，不附加敏感底层堆栈。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>new RPCFailure(3, code)</code> |

<!-- write-step: 7 bytes=955:1009 -->
````javascript
export const bad = (code) => new RPCFailure(3, code);
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/shared_protocol.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/01-content-replay.md)。
