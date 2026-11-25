# 分步编写 `services/document/internal/logic/document/candidate_cleanup.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/195-candidate-cleanup.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

为Node清理送审/合并临时候选签发绑定原工件的许可。

候选身份不能随后台重领而变化，capture用guard_generation，merge用不可复用attempt_id。

写之前先按这条执行路线理解各段的连接：

1. parseCleanupReference解析精确引用
2. Authorize锁原事实并检查业务/安装/副本，保存candidateCleanupProof
3. cleanupPermitReply返回原摘要许可。

本文件需要守住的边界：

- worker租约generation不是候选generation
- 没有权威副本时不得销毁唯一恢复正文。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/candidate_cleanup.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/candidate_cleanup.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/candidate_cleanup.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/candidate_cleanup.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:334 -->
````go
package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"google.golang.org/protobuf/proto"
)

````

## 第 2 步：CandidateCleanup

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type CandidateCleanup struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 2 bytes=334:378 -->
````go
type CandidateCleanup struct{ db *sql.DB }

````

## 第 3 步：NewCandidateCleanup

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func NewCandidateCleanup(db *sql.DB) *CandidateCleanup`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*CandidateCleanup</code> | 实际返回表达式：<code>&amp;CandidateCleanup{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;CandidateCleanup{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=378:471 -->
````go
func NewCandidateCleanup(db *sql.DB) *CandidateCleanup { return &CandidateCleanup{db: db} }

````

## 第 4 步：candidateCleanupProof

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`type candidateCleanupProof struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>*pb.CleanupAuthorization</code> | 序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |
| <code>Version</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;digest&quot;`</code> |

<!-- write-step: 4 bytes=471:662 -->
````go
type candidateCleanupProof struct {
	Binding *pb.CleanupAuthorization `json:"binding"`
	Version uint64                   `json:"version"`
	Digest  []byte                   `json:"digest"`
}

````

## 第 5 步：parseCleanupReference

插入位置：第 4 步代码之后，紧接追加；完成稿约第 29 行。

引用格式明确绑定原操作；capture代次为原guard_generation，merge代次为不可复用的attempt_id。
它们均不是后台worker租约代次，清理重试不能随着重领任务改变候选身份。

本步声明：`func parseCleanupReference(request *pb.CleanupAuthorization) (string, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>request</code> | <code>*pb.CleanupAuthorization</code> | 在此声明中的实际使用：<code>request == nil</code>；<code>request.OperationId</code>；<code>request.Generation</code>；<code>request.CandidateDigest</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>&quot;capture&quot;</code>；<code>&quot;merge&quot;</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>attempt</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;CANDIDATE_BINDING_INVALID&quot;)</code>；<code>nil</code>；<code>domain.Violation(domain.InvalidRequest, &quot;CANDIDATE_REFERENCE_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>request == nil &#124;&#124; !domain.ValidOperationID(request.OperationId) &#124;&#124; request.Generation == 0 &#124;&#124; len(request.CandidateDigest) != sha256.Size</code>。条件成立时执行 <code>return &quot;&quot;, 0, domain.Violation(domain.InvalidRequest, &quot;CANDIDATE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(parts) == 2 &amp;&amp; parts&#91;0&#93; == &quot;capture&quot; &amp;&amp; parts&#91;1&#93; == request.OperationId</code>。条件成立时执行 <code>return &quot;capture&quot;, 0, nil</code>。当前函数在这里结束。
- 检查 <code>len(parts) == 3 &amp;&amp; parts&#91;0&#93; == &quot;merge&quot; &amp;&amp; parts&#91;1&#93; == request.OperationId</code>。
- 检查 <code>err == nil &amp;&amp; attempt &gt; 0 &amp;&amp; strconv.FormatUint(attempt, 10) == parts&#91;2&#93; &amp;&amp; attempt == request.Generation</code>。条件成立时执行 <code>return &quot;merge&quot;, attempt, nil</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;&quot;, 0, domain.Violation(domain.InvalidRequest, &quot;CANDIDATE_REFERENCE_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=662:1742 -->
````go
// 引用格式明确绑定原操作；capture代次为原guard_generation，merge代次为不可复用的attempt_id。
// 它们均不是后台worker租约代次，清理重试不能随着重领任务改变候选身份。
func parseCleanupReference(request *pb.CleanupAuthorization) (string, uint64, error) {
	if request == nil || !domain.ValidOperationID(request.OperationId) || request.Generation == 0 || len(request.CandidateDigest) != sha256.Size {
		return "", 0, domain.Violation(domain.InvalidRequest, "CANDIDATE_BINDING_INVALID")
	}
	parts := strings.Split(request.CandidateRef, ":")
	if len(parts) == 2 && parts[0] == "capture" && parts[1] == request.OperationId {
		return "capture", 0, nil
	}
	if len(parts) == 3 && parts[0] == "merge" && parts[1] == request.OperationId {
		attempt, err := strconv.ParseUint(parts[2], 10, 64)
		if err == nil && attempt > 0 && strconv.FormatUint(attempt, 10) == parts[2] && attempt == request.Generation {
			return "merge", attempt, nil
		}
	}
	return "", 0, domain.Violation(domain.InvalidRequest, "CANDIDATE_REFERENCE_INVALID")
}

````

## 第 6 步：cleanupPermitReply

插入位置：第 5 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func cleanupPermitReply(request *pb.CleanupAuthorization, state pb.CleanupPermit_State, raw []byte) *pb.CleanupPermit`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>request</code> | <code>*pb.CleanupAuthorization</code> | 在此声明中的实际使用：<code>request.OperationId</code>；<code>request.Generation</code> |
| <code>state</code> | <code>pb.CleanupPermit_State</code> | 在此声明中的实际使用：<code>State: state</code>；<code>state == pb.CleanupPermit_CLEANABLE</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>sha256.Sum256(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.CleanupPermit</code> | 实际返回表达式：<code>reply</code> |

**跟着本段代码走一遍：**

- 检查 <code>state == pb.CleanupPermit_CLEANABLE</code>。
- 此处通过 <code>return reply</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1742:2096 -->
````go
func cleanupPermitReply(request *pb.CleanupAuthorization, state pb.CleanupPermit_State, raw []byte) *pb.CleanupPermit {
	reply := &pb.CleanupPermit{OperationId: request.OperationId, Generation: request.Generation, State: state}
	if state == pb.CleanupPermit_CLEANABLE {
		digest := sha256.Sum256(raw)
		reply.PermitDigest = digest[:]
	}
	return reply
}

````

## 第 7 步：CandidateCleanup.Authorize

插入位置：第 6 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func (s *CandidateCleanup) Authorize(ctx context.Context, request *pb.CleanupAuthorization) (*pb.CleanupPermit, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.LockOperationGate(ctx, tx, request.OperationId)</code>；<code>repository.CandidateCleanupPermit(ctx, tx, request.CandidateRef)</code>；<code>repository.SubmissionCleanupCopy(ctx, tx, request.OperationId)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>*pb.CleanupAuthorization</code> | 在此声明中的实际使用：<code>parseCleanupReference(request)</code>；<code>request.OperationId</code>；<code>request.CandidateRef</code>；<code>proto.Equal(proof.Binding, request)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.CleanupPermit</code> | 实际返回表达式：<code>nil</code>；<code>cleanupPermitReply(request, pb.CleanupPermit_CLEANABLE, raw)</code>；<code>cleanupPermitReply(request, pb.CleanupPermit_UNKNOWN, nil)</code>；<code>cleanupPermitReply(request, pb.CleanupPermit_RETAIN, nil)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;CANDIDATE_PERMIT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;CANDIDATE_BINDING_CHANGED&quot;)</code>；<code>nil</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>json.Unmarshal(raw, &amp;proof) != nil &#124;&#124; proof.Binding == nil &#124;&#124; proof.Version == 0 &#124;&#124; len(proof.Digest) != sha256.Size</code>。条件成立时执行 <code>return nil, errors.New(&quot;CANDIDATE_PERMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(proof.Binding, request) &#124;&#124; !bytes.Equal(proof.Digest, request.CandidateDigest)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;CANDIDATE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return cleanupPermitReply(request, pb.CleanupPermit_CLEANABLE, raw), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>kind == &quot;capture&quot;</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return cleanupPermitReply(request, pb.CleanupPermit_UNKNOWN, nil), nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>generation != request.Generation</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;CANDIDATE_GENERATION_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=2096:3889 -->
````go
func (s *CandidateCleanup) Authorize(ctx context.Context, request *pb.CleanupAuthorization) (*pb.CleanupPermit, error) {
	kind, attempt, err := parseCleanupReference(request)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, request.OperationId); err != nil {
		return nil, err
	}
	// 已签出的许可保存原证明。后来正文按合法生命周期删除，不会破坏同号清理回执重放。
	raw, err := repository.CandidateCleanupPermit(ctx, tx, request.CandidateRef)
	if err == nil {
		var proof candidateCleanupProof
		if json.Unmarshal(raw, &proof) != nil || proof.Binding == nil || proof.Version == 0 || len(proof.Digest) != sha256.Size {
			return nil, errors.New("CANDIDATE_PERMIT_UNKNOWN")
		}
		if !proto.Equal(proof.Binding, request) || !bytes.Equal(proof.Digest, request.CandidateDigest) {
			return nil, domain.Violation(domain.StateConflict, "CANDIDATE_BINDING_CHANGED")
		}
		return cleanupPermitReply(request, pb.CleanupPermit_CLEANABLE, raw), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var generation, version uint64
	var state string
	var digest, actual []byte
	var copied bool
	if kind == "capture" {
		generation, state, version, digest, actual, copied, err = repository.SubmissionCleanupCopy(ctx, tx, request.OperationId)
	} else {
		generation = attempt
		state, version, digest, actual, copied, err = repository.MergeCleanupCopy(ctx, tx, request.OperationId, attempt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return cleanupPermitReply(request, pb.CleanupPermit_UNKNOWN, nil), nil
	}
	if err != nil {
		return nil, err
	}
	if generation != request.Generation {
````

## 第 8 步：CandidateCleanup.Authorize（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 102 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, domain.Violation(domain.StateConflict, &quot;CANDIDATE_GENERATION_CHANGED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>kind == &quot;merge&quot;</code>。
- 检查 <code>state != expectedState &#124;&#124; version == 0 &#124;&#124; !copied</code>。条件成立时执行 <code>return cleanupPermitReply(request, pb.CleanupPermit_RETAIN, nil), nil</code>。当前函数在这里结束。
- 检查 <code>len(digest) != sha256.Size &#124;&#124; !bytes.Equal(digest, actual)</code>。条件成立时执行 <code>return cleanupPermitReply(request, pb.CleanupPermit_UNKNOWN, nil), nil</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(digest, request.CandidateDigest)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;CANDIDATE_DIGEST_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!complete</code>。条件成立时执行 <code>return cleanupPermitReply(request, pb.CleanupPermit_RETAIN, nil), nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;CANDIDATE_PERMIT_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;CANDIDATE_PERMIT_READ_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return cleanupPermitReply(request, pb.CleanupPermit_CLEANABLE, raw), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3889:5530 -->
````go
		return nil, domain.Violation(domain.StateConflict, "CANDIDATE_GENERATION_CHANGED")
	}
	expectedState := "SUCCEEDED"
	if kind == "merge" {
		expectedState = "COMMITTED"
	}
	if state != expectedState || version == 0 || !copied {
		return cleanupPermitReply(request, pb.CleanupPermit_RETAIN, nil), nil
	}
	if len(digest) != sha256.Size || !bytes.Equal(digest, actual) {
		return cleanupPermitReply(request, pb.CleanupPermit_UNKNOWN, nil), nil
	}
	if !bytes.Equal(digest, request.CandidateDigest) {
		return nil, domain.Violation(domain.StateConflict, "CANDIDATE_DIGEST_CHANGED")
	}
	complete, err := repository.CandidateCleanupTerminal(ctx, tx, request.OperationId)
	if err != nil {
		return nil, err
	}
	if !complete {
		return cleanupPermitReply(request, pb.CleanupPermit_RETAIN, nil), nil
	}
	raw, err = json.Marshal(candidateCleanupProof{Binding: proto.Clone(request).(*pb.CleanupAuthorization), Version: version, Digest: digest})
	if err != nil {
		return nil, err
	}
	if err = repository.SaveCandidateCleanupPermit(ctx, tx, request.CandidateRef, request.OperationId, raw); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, errors.New("CANDIDATE_PERMIT_COMMIT_UNKNOWN")
	}
	// MySQL JSON列会规范化空白/键序，首次回复也必须读取真正持久的字节。
	// 不能以写入前Marshal的摘要签许可，否则Node重查原号必然得到不同摘要。
	raw, err = repository.CandidateCleanupPermit(ctx, s.db, request.CandidateRef)
	if err != nil {
		return nil, errors.New("CANDIDATE_PERMIT_READ_UNKNOWN")
	}
	return cleanupPermitReply(request, pb.CleanupPermit_CLEANABLE, raw), nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/candidate_cleanup.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
