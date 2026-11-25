# 分步编写 `services/collaboration/src/transport/rpc_authorization.mjs`

先按 [C07 · 协作完整依赖装配、传输与就绪](../lessons/collaboration/07-wiring-readiness.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/351-rpc-authorization.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

从mTLS证书提取唯一服务身份并按私有方法白名单授权。

有可信证书不等于能调用所有私有接口；RPC还必须有有限且短的deadline。

写之前先按这条执行路线理解各段的连接：

1. authorizeCall要求ssl和peer certificate
2. URI SAN筛出四个已知服务，必须恰好一个角色
3. deadline必须未来且不超过5秒
4. caller-policy按完整method路径允许角色，否则SERVICE_METHOD_FORBIDDEN

本文件需要守住的边界：

- 多个服务URI不能取第一个默认通过
- 没有deadline或无限deadline都拒绝

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/transport' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/transport/rpc_authorization.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/transport/rpc_authorization.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/transport/rpc_authorization.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/transport/rpc_authorization.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:40 -->
````javascript
import { readFileSync } from 'node:fs';
````

## 第 2 步：policy

插入位置：第 1 步代码之后，紧接追加；完成稿约第 2 行。

本步声明：`const policy = JSON.parse(`。

<!-- write-step: 2 bytes=40:162 -->
````javascript
const policy = JSON.parse(
  readFileSync(new URL('../../../../contracts/proto/caller-policy.json', import.meta.url)),
);
````

## 第 3 步：failure

插入位置：第 2 步代码之后，紧接追加；完成稿约第 5 行。

本步声明：`const failure = (code, details) => (`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>code</code> | <code>JavaScript 参数</code> | gRPC数值状态码：未认证16、参数错误3或无权限7等。 |
| <code>details</code> | <code>JavaScript 参数</code> | 对应稳定拒绝原因字符串，交给RPC传输层报告。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>{ code, details }</code> |

<!-- write-step: 3 bytes=162:218 -->
````javascript
const failure = (code, details) => ({ code, details });
````

## 第 4 步：authorizeCall

插入位置：第 3 步代码之后，紧接追加；完成稿约第 6 行。

本步声明：`export function authorizeCall(auth, method, deadline) `。

**这一段为什么这样写：** 在进入任何业务用例之前验证机器身份与方法权限。

1. 核TLS上下文
2. 唯一提取角色
3. 核有限deadline
4. 查静态方法政策


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>auth</code> | <code>JavaScript 参数</code> | grpc-js握手后的认证上下文<br>在此声明中的实际使用：<code>auth?.transportSecurityType</code>；<code>auth.sslPeerCertificate</code> |
| <code>method</code> | <code>JavaScript 参数</code> | 完整RPC方法路径<br>在此声明中的实际使用：<code>policy&#91;method&#93;</code> |
| <code>deadline</code> | <code>JavaScript 参数</code> | gRPC调用的Date或毫秒终点<br>在此声明中的实际使用：<code>deadline instanceof Date</code>；<code>deadline.getTime</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>failure(16, &#x27;SERVICE_IDENTITY_REQUIRED&#x27;)</code>；<code>failure(3, &#x27;FINITE_DEADLINE_REQUIRED&#x27;)</code>；<code>failure(7, &#x27;SERVICE_METHOD_FORBIDDEN&#x27;)</code>；<code>null</code> |

**跟着本段代码走一遍：**

- 检查 <code>auth?.transportSecurityType !== &#x27;ssl&#x27; &#124;&#124; !auth.sslPeerCertificate</code>。条件成立时执行 <code>return failure(16, &#x27;SERVICE_IDENTITY_REQUIRED&#x27;);</code>。当前函数在这里结束。
- 检查 <code>roles.length !== 1</code>。条件成立时执行 <code>return failure(16, &#x27;SERVICE_IDENTITY_REQUIRED&#x27;);</code>。当前函数在这里结束。
- 检查 <code>!Number.isFinite(end) &#124;&#124; end &lt;= Date.now() &#124;&#124; end - Date.now() &gt; 5000</code>。条件成立时执行 <code>return failure(3, &#x27;FINITE_DEADLINE_REQUIRED&#x27;);</code>。当前函数在这里结束。
- 检查 <code>!policy&#91;method&#93;?.includes(roles&#91;0&#93;)</code>。条件成立时执行 <code>return failure(7, &#x27;SERVICE_METHOD_FORBIDDEN&#x27;);</code>。当前函数在这里结束。
- 此处通过 <code>return null;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=218:1255 -->
````javascript
export function authorizeCall(auth, method, deadline) {
  // grpc-js在mTLS握手成功后提供证书；应用继续检查URI身份及具体方法权限。
  if (auth?.transportSecurityType !== 'ssl' || !auth.sslPeerCertificate)
    return failure(16, 'SERVICE_IDENTITY_REQUIRED');
  const known = ['account', 'document', 'search', 'collaboration'];
  const identities = (auth.sslPeerCertificate.subjectaltname ?? '')
    .split(/,\s*/)
    .filter((x) => x.startsWith('URI:spiffe://collaboration-report/'))
    .map((x) => x.slice('URI:spiffe://collaboration-report/'.length));
  const roles = [...new Set(identities.filter((x) => known.includes(x)))];
  if (roles.length !== 1) return failure(16, 'SERVICE_IDENTITY_REQUIRED');
  const end = deadline instanceof Date ? deadline.getTime() : deadline;
  if (!Number.isFinite(end) || end <= Date.now() || end - Date.now() > 5000)
    return failure(3, 'FINITE_DEADLINE_REQUIRED');
  if (!policy[method]?.includes(roles[0])) return failure(7, 'SERVICE_METHOD_FORBIDDEN');
  return null;
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/transport/rpc_authorization.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/07-wiring-readiness.md)。
