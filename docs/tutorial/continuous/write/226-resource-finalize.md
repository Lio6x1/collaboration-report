# 分步编写 `services/document/internal/logic/document/resource_finalize.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/226-resource-finalize.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

根据Node持久UPDATE结果确认或释放草稿资源保护。

确认请求本身只是声明，Doc必须查询真实提交事实才能把临时保护转换为草稿引用或释放预留。

写之前先按这条执行路线理解各段的连接：

1. Finalize核operation/hash/generation
2. 读取NodeUpdateProof
3. 锁原保护、来源及资源
4. COMMITTED时建立实际DRAFT引用并结算正文
5. 释放路径核安全证据。

本文件需要守住的边界：

- Node结果未知返回NODE_OUTCOME_NOT_PROVEN
- 不能用seq自报或准入过期证明可以释放。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/resource_finalize.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/resource_finalize.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/resource_finalize.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/resource_finalize.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:304 -->
````go
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"encoding/hex"
	"sort"
)

````

## 第 2 步：DraftResources.Finalize

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func (s *DraftResources) Finalize(ctx context.Context, in *pb.DraftResourceFinalize, release bool) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.queryProtection(ctx, in.Query)</code>；<code>s.outcomes.UpdateOutcome(ctx, req)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>validateProtectionGrant(ctx, tx, req)</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>in</code> | <code>*pb.DraftResourceFinalize</code> | 在此声明中的实际使用：<code>in == nil</code>；<code>in.Query</code>；<code>in.UpdateHash</code>；<code>in.DurableSeq</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>release</code> | <code>bool</code> | 在此声明中的实际使用：<code>release &amp;&amp; state != &quot;CONFIRMED&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 实际返回表达式：<code>nil</code>；<code>protectionReceipt(req, state)</code>；<code>protectionReceipt(req, next)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RejectProtection(domain.ProtectionInvalid, &quot;PROTECTION_FINALIZE_INVALID&quot;)</code>；<code>err</code>；<code>domain.RejectProtection(domain.ProtectionConflict, &quot;UPDATE_HASH_CONFLICT&quot;)</code>；<code>domain.RejectProtection(domain.ProtectionPrecondition, &quot;NODE_OUTCOME_NOT_PROVEN&quot;)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>in == nil</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionInvalid, &quot;PROTECTION_FINALIZE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(req.UpdateHash, in.UpdateHash)</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionConflict, &quot;UPDATE_HASH_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!committed &#124;&#124; in.ProvenNodeOutcome != &quot;COMMITTED&quot;</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionPrecondition, &quot;NODE_OUTCOME_NOT_PROVEN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionForbidden, &quot;DOCUMENT_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionUnknown, &quot;PROTECTION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>state == &quot;RELEASED&quot;</code>。
- 检查 <code>release</code>。条件成立时执行 <code>return protectionReceipt(req, state), nil</code>。当前函数在这里结束。
- 此处通过 <code>return nil, domain.RejectProtection(domain.ProtectionPrecondition, &quot;PROTECTION_RELEASED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=304:2363 -->
````go
func (s *DraftResources) Finalize(ctx context.Context, in *pb.DraftResourceFinalize, release bool) (*pb.DraftResourceReceipt, error) {
	if in == nil {
		return nil, domain.RejectProtection(domain.ProtectionInvalid, "PROTECTION_FINALIZE_INVALID")
	}
	req, err := s.queryProtection(ctx, in.Query)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(req.UpdateHash, in.UpdateHash) {
		return nil, domain.RejectProtection(domain.ProtectionConflict, "UPDATE_HASH_CONFLICT")
	}
	proof, err := s.outcomes.UpdateOutcome(ctx, req)
	if err != nil {
		return nil, err
	}
	// 自报COMMITTED或seq不构成保存证据，必须逐项匹配实际Node日志的原保护、grant和hash。
	committed := proof.State == "COMMITTED" && proof.Protection == req.Context.OperationId && proof.Grant == req.GrantId && proof.Hash == hex.EncodeToString(req.UpdateHash) && proof.Seq == domain.TextID(in.DurableSeq) && in.NodeReceiptRef == domain.TextID(req.RoomId)+":"+domain.TextID(req.WorkspaceEpoch)+":"+proof.Seq
	if !committed || in.ProvenNodeOutcome != "COMMITTED" {
		return nil, domain.RejectProtection(domain.ProtectionPrecondition, "NODE_OUTCOME_NOT_PROVEN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = validateProtectionGrant(ctx, tx, req); err != nil {
		return nil, err
	}
	space, lifecycle, err := repository.LockedResourceDocument(ctx, tx, req.Context.Target.DocumentId)
	if err != nil || lifecycle != "ACTIVE" {
		return nil, domain.RejectProtection(domain.ProtectionForbidden, "DOCUMENT_NOT_CURRENT")
	}
	if err = repository.LockDocumentCapacity(ctx, tx, req.Context.Target.DocumentId); err != nil {
		return nil, err
	}
	state, err := repository.LockedResourceProtectionState(ctx, tx, req.Context.OperationId)
	if err != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_UNKNOWN")
	}
	if state == "RELEASED" {
		if release {
			return protectionReceipt(req, state), nil
		}
		return nil, domain.RejectProtection(domain.ProtectionPrecondition, "PROTECTION_RELEASED")
	}
````

## 第 3 步：DraftResources.Finalize（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 59 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>release &amp;&amp; state != &quot;CONFIRMED&quot;</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionPrecondition, &quot;SAFE_TRANSFER_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; current != &quot;READY&quot;</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionPrecondition, &quot;RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>release</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionUnknown, &quot;PROTECTION_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return protectionReceipt(req, next), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2363:4010 -->
````go
	if release && state != "CONFIRMED" {
		return nil, domain.RejectProtection(domain.ProtectionPrecondition, "SAFE_TRANSFER_REQUIRED")
	}
	limit, err := resources.QuotaLimit(ctx, tx, space)
	if err != nil {
		return nil, err
	}
	q, err := resources.LockQuota(ctx, tx, space, limit)
	if err != nil {
		return nil, err
	}
	if err = resources.SettleDraftBody(ctx, tx, q, req.Context.OperationId, proof.BodyBytes, proof.BodyHash); err != nil {
		return nil, err
	}
	ids := append([]uint64{}, req.ResourceIds...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		current, err := repository.LockedResourceState(ctx, tx, id)
		if err != nil || current != "READY" {
			return nil, domain.RejectProtection(domain.ProtectionPrecondition, "RESOURCE_NOT_READY")
		}
		owner := domain.TextID(req.RoomId) + ":" + domain.TextID(req.WorkspaceEpoch)
		if err = repository.InsertDraftResourceRef(ctx, tx, id, req.Context.Target.DocumentId, owner, req.Generation); err != nil {
			return nil, err
		}
	}
	// 同一事务先让DRAFT引用接住，再删临时pin；未知结果只多保留，不造成资源悬空。
	if err = repository.DeleteOperationResourceRefs(ctx, tx, req.Context.OperationId); err != nil {
		return nil, err
	}
	next := "CONFIRMED"
	if release {
		next = "RELEASED"
	}
	if err = repository.FinalizeResourceProtection(ctx, tx, req.Context.OperationId, next, in.NodeReceiptRef, in.DurableSeq); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_COMMIT_UNKNOWN")
	}
	return protectionReceipt(req, next), nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/resource_finalize.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
