# 分步编写 `services/collaboration/src/domain/fence_protocol.mjs`

先按 [C03 · 撤权门闩、真实输出排空与进程恢复](../lessons/collaboration/03-fence-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/290-fence-protocol.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

校验撤权目标绑定并产生确定性的排空ACK摘要。

同一revocation_id重试必须关闭原scope/epoch，不能被另一次权限变更劫用。

写之前先按这条执行路线理解各段的连接：

1. binding限制一个closed_scope、generation=1及合法uint64代次
2. 按SESSION_LOGOUT或AUTHORIZATION_CHANGE重算target_digest并常量时间比较
3. assertSameBinding比较历史scope/epoch/digest/generation
4. ack将服务名、撤权ID、来源和代次合入结果摘要

本文件需要守住的边界：

- epoch禁止已被Number舍入的值
- ack只是确定性响应构造器，必须由用例在真实停止输出之后调用并持久化

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/fence_protocol.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/fence_protocol.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/fence_protocol.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/fence_protocol.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:112 -->
````javascript
import { createHash, timingSafeEqual } from 'node:crypto';
import { RPCFailure } from './shared_protocol.mjs';

````

## 第 2 步：binding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

固定目标摘要绑定撤权号、来源和代次；重试必须消费原计划，不能替换成当前可变权限。

本步声明：`export function binding(req) `。

**这一段为什么这样写：** 核对来自权威服务的固定撤权计划。

1. 验证scope种类、ID范围和代次
2. 重算目标摘要
3. 返回十六进制摘要与generation用于落库


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>req</code> | <code>JavaScript 参数</code> | 含revocation_id、closed_scopes、generation和target_digest的撤权请求<br>在此声明中的实际使用：<code>req.closed_scopes</code>；<code>req.revocation_id</code>；<code>req.generation</code>；<code>req.target_digest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ bootstrap_target_digest: actual.toString(&#x27;hex&#x27;), generation: &#x27;1&#x27; }</code> |

**跟着本段代码走一遍：**

- 检查 <code>!/^&#91;A-Za-z0-9&#93;&#91;A-Za-z0-9_-&#93;{15,95}$/.test(req.revocation_id ?? &#x27;&#x27;) &#124;&#124;     String(req.generation) !== &#x27;1&#x27; &#124;&#124;     req.closed_scopes?.length !== 1 &#124;&#124;     (!isSession &amp;&amp; !isAuthorizati …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(3, &#x27;BOOTSTRAP_FENCE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(3, &#x27;BOOTSTRAP_FENCE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>actual.length !== 32 &#124;&#124; !timingSafeEqual(actual, expected)</code>。条件成立时执行 <code>throw new RPCFailure(3, &#x27;BOOTSTRAP_FENCE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(3, &#x27;BOOTSTRAP_FENCE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return { bootstrap_target_digest: actual.toString(&#x27;hex&#x27;), generation: &#x27;1&#x27; };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=112:1817 -->
````javascript
// 固定目标摘要绑定撤权号、来源和代次；重试必须消费原计划，不能替换成当前可变权限。
export function binding(req) {
  const scope = req.closed_scopes?.[0],
    epoch = scope?.epoch;
  const key = scope?.scope_key ?? '',
    isSession = /^session:[a-f0-9]{64}$/.test(key),
    isAuthorizationSource =
      /^(?:(?:identity|credential|platform-admin|doc-acl|kb-acl|publication):[1-9][0-9]{0,19}|(?:org-member|org-admin|org-owner|org-create):[1-9][0-9]{0,19}:[1-9][0-9]{0,19})$/.test(
        key,
      );
  if (
    !/^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$/.test(req.revocation_id ?? '') ||
    String(req.generation) !== '1' ||
    req.closed_scopes?.length !== 1 ||
    (!isSession && !isAuthorizationSource) ||
    (isAuthorizationSource &&
      key
        .split(':')
        .slice(1)
        .some((x) => BigInt(x) > 18446744073709551615n)) ||
    // protobuf固定以十进制字符串传uint64；不能接纳已被Number舍入的代次或多种绑定拼法。
    (typeof epoch === 'number' && !Number.isSafeInteger(epoch)) ||
    !/^[1-9][0-9]{0,19}$/.test(String(epoch)) ||
    BigInt(epoch) < 1n ||
    BigInt(epoch) > 18446744073709551615n
  )
    throw new RPCFailure(3, 'BOOTSTRAP_FENCE_INVALID');
  const expected = createHash('sha256')
    .update(
      isSession
        ? 'SESSION_LOGOUT\n' + key.slice(8)
        : 'AUTHORIZATION_CHANGE\n' + key + '\n' + String(epoch),
    )
    .digest();
  const actual = Buffer.from(req.target_digest ?? []);
  if (actual.length !== 32 || !timingSafeEqual(actual, expected))
    throw new RPCFailure(3, 'BOOTSTRAP_FENCE_INVALID');
  return { bootstrap_target_digest: actual.toString('hex'), generation: '1' };
}
````

## 第 3 步：ack

插入位置：第 2 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`export function ack(req) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>req</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>req.closed_scopes</code>；<code>req.revocation_id</code>；<code>req.target_digest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     state: &#x27;ACKNOWLEDGED&#x27;,     revocation_id: req.revocation_id,     closed_scopes: req.closed_scopes,     generation: &#x27;1&#x27;,     result_digest: digest,     missing_instances: &#91;&#93;,   }</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return {     state: &#x27;ACKNOWLEDGED&#x27;,     revocation_id: req.revocation_id,     closed_scopes: req.closed_scopes,     generation: &#x27;1&#x27;,     result_digest: digest,     missing_instances: &#91;&#93;,   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1817:2404 -->
````javascript
export function ack(req) {
  const scope = req.closed_scopes[0],
    digest = createHash('sha256')
      .update(
        [
          'bootstrap-fence-v1',
          'collaboration',
          req.revocation_id,
          scope.scope_key,
          String(scope.epoch),
          '1',
          Buffer.from(req.target_digest).toString('hex'),
        ].join('\n'),
      )
      .digest();
  return {
    state: 'ACKNOWLEDGED',
    revocation_id: req.revocation_id,
    closed_scopes: req.closed_scopes,
    generation: '1',
    result_digest: digest,
    missing_instances: [],
  };
}

````

## 第 4 步：assertSameBinding

插入位置：第 3 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`export function assertSameBinding(row, scope, current) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>row.target_permission</code>；<code>row.scope_key</code>；<code>row.epoch</code> |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>scope.scope_key</code>；<code>scope.epoch</code> |
| <code>current</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>current.bootstrap_target_digest</code>；<code>current.generation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。 |

**跟着本段代码走一遍：**

- 检查 <code>row.scope_key !== scope.scope_key &#124;&#124;     String(row.epoch) !== String(scope.epoch) &#124;&#124;     p.bootstrap_target_digest !== current.bootstrap_target_digest &#124;&#124;     p.generation !== current.generation</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;FENCE_ID_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;FENCE_ID_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 4 bytes=2404:2854 -->
````javascript
export function assertSameBinding(row, scope, current) {
  const p =
    typeof row.target_permission === 'string'
      ? JSON.parse(row.target_permission)
      : row.target_permission;
  if (
    row.scope_key !== scope.scope_key ||
    String(row.epoch) !== String(scope.epoch) ||
    p.bootstrap_target_digest !== current.bootstrap_target_digest ||
    p.generation !== current.generation
  )
    throw new RPCFailure(6, 'FENCE_ID_CONFLICT');
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/fence_protocol.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/03-fence-recovery.md)。
