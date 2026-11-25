# 分步编写 `services/document/internal/logic/snapshot/capture.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/248-capture.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在Document私库一次RR事务中捕获全量重建元数据并签名封存。

条目和封存签名必须来自同一个一致时点，Search私库不应参与Doc快照。

写之前先按这条执行路线理解各段的连接：

1. Service.Capture验Search prepare与operation
2. Reserve复用原输入
3. 逐页CurrentPage并InsertEntries
4. 签名后Seal
5. COMMIT确认才返回制品。

本文件需要守住的边界：

- 提交回复未知不导出内存结果，必须同号同输入重试
- 已SEALED恢复不需要当前Signer重新签名。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/snapshot' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/snapshot/capture.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/snapshot/capture.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/snapshot/capture.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/snapshot/capture.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:354 -->
````go
// Package snapshot 编排 Document 自有的一致快照，不读取 Search 的私有库。
package snapshot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"

	repo "collaboration/document/internal/repository/snapshot"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
)

````

## 第 2 步：ErrInput、ErrConflict、ErrCommitUnknown、ErrIncomplete

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`var (`。

<!-- write-step: 2 bytes=354:638 -->
````go
var (
	ErrInput         = errors.New("REBUILD_SNAPSHOT_INPUT_INVALID")
	ErrConflict      = errors.New("REBUILD_OPERATION_CONFLICT")
	ErrCommitUnknown = errors.New("REBUILD_SNAPSHOT_COMMIT_UNKNOWN_RETRY_SAME_OPERATION")
	ErrIncomplete    = errors.New("REBUILD_SNAPSHOT_INCOMPLETE")
)

````

## 第 3 步：Service

插入位置：第 2 步代码之后，紧接追加；完成稿约第 25 行。

Service 的证书与数据库只能由本服务配置装配。恢复已封存任务不使用 Signer，
因而 CLI 可先恢复，再在确需新封存时加载当前有效的本服务签名身份。

本步声明：`type Service struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>DB</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>Verifier</code> | <code>*rebuildartifact.Verifier</code> | 使用 <code>rebuildartifact.Verifier</code> 类型；其定义处的职责：为离线重建制品签名、首次验签及恢复已接受原字节。 |
| <code>Signer</code> | <code>*rebuildartifact.Signer</code> | 使用 <code>rebuildartifact.Signer</code> 类型；其定义处的职责：为离线重建制品签名、首次验签及恢复已接受原字节。 |
| <code>LoadSigner</code> | <code>func() (*rebuildartifact.Signer, error)</code> | 在需要时加载快照签名器的依赖，返回签名器或错误；恢复仍需与原任务SignerDigest一致，不能换签名者重新接受原号。 |

<!-- write-step: 3 bytes=638:1014 -->
````go
// Service 的证书与数据库只能由本服务配置装配。恢复已封存任务不使用 Signer，
// 因而 CLI 可先恢复，再在确需新封存时加载当前有效的本服务签名身份。
type Service struct {
	DB         *sql.DB
	Verifier   *rebuildartifact.Verifier
	Signer     *rebuildartifact.Signer
	LoadSigner func() (*rebuildartifact.Signer, error)
}

````

## 第 4 步：Service.Capture

插入位置：第 3 步代码之后，紧接追加；完成稿约第 34 行。

Capture 在一次 RR 事务内同时保存完整条目和签名。任何扫描/签名错误均回滚；
commit 回执未知时不导出本次内存结果，调用者只能带完全相同的原号和输入重试。

本步声明：`func (s *Service) Capture(ctx context.Context, operation string, prepareEnvelope []byte) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelRepeatableRead})</code>；<code>repo.Lock(ctx, tx, operation)</code>；<code>repo.Reserve(ctx, tx, operation, prepareEnvelope, signerDigest)</code>；<code>repo.CurrentPage(ctx, tx, after, 500)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>rebuildmeta.OperationValid(operation)</code>；<code>repo.Lock(ctx, tx, operation)</code>；<code>checkOperation(accepted, operation)</code>；<code>repo.Reserve(ctx, tx, operation, prepareEnvelope, signerDigest)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>prepareEnvelope</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(prepareEnvelope)</code>；<code>s.Verifier.Verify(prepareEnvelope, rebuildartifact.PrepareV1)</code>；<code>repo.Reserve(ctx, tx, operation, prepareEnvelope, signerDigest)</code>；<code>bytes.Equal(job.Prepare, prepareEnvelope)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>append(&#91;&#93;byte(nil), job.Seal...)</code>；<code>envelope</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrInput</code>；<code>errors.New(&quot;REBUILD_SNAPSHOT_BEGIN_FAILED&quot;)</code>；<code>rebuildartifact.ErrConfig</code>；<code>err</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>s == nil &#124;&#124; s.DB == nil &#124;&#124; !rebuildmeta.OperationValid(operation) &#124;&#124; len(prepareEnvelope) == 0 &#124;&#124; len(prepareEnvelope) &gt; rebuildartifact.MaxEnvelopeBytes</code>。条件成立时执行 <code>return nil, ErrInput</code>。当前函数在这里结束。
- 执行 <code>s.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelRepeatableRead})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_SNAPSHOT_BEGIN_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 执行 <code>repo.Lock(ctx, tx, operation)</code>。
- 检查 <code>errors.Is(err, repo.ErrNotFound)</code>。
- 检查 <code>s.Verifier == nil</code>。条件成立时执行 <code>return nil, rebuildartifact.ErrConfig</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(fingerprint) != 32</code>。条件成立时执行 <code>return nil, rebuildartifact.ErrIdentity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>job.Operation != operation &#124;&#124; !bytes.Equal(job.Prepare, prepareEnvelope) &#124;&#124; sha256.Sum256(prepareEnvelope) != job.PrepareDigest</code>。条件成立时执行 <code>return nil, ErrConflict</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=1014:3024 -->
````go
// Capture 在一次 RR 事务内同时保存完整条目和签名。任何扫描/签名错误均回滚；
// commit 回执未知时不导出本次内存结果，调用者只能带完全相同的原号和输入重试。
func (s *Service) Capture(ctx context.Context, operation string, prepareEnvelope []byte) ([]byte, error) {
	if s == nil || s.DB == nil || !rebuildmeta.OperationValid(operation) || len(prepareEnvelope) == 0 || len(prepareEnvelope) > rebuildartifact.MaxEnvelopeBytes {
		return nil, ErrInput
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, errors.New("REBUILD_SNAPSHOT_BEGIN_FAILED")
	}
	defer tx.Rollback()

	// 锁定读取不建立业务表的一致视图；先检查持久接受事实，证书到期不能使
	// 已接受的原操作失去恢复能力。只有明确不存在才允许按当前信任链首次准入。
	job, err := repo.Lock(ctx, tx, operation)
	var accepted rebuildartifact.Verified
	if errors.Is(err, repo.ErrNotFound) {
		if s.Verifier == nil {
			return nil, rebuildartifact.ErrConfig
		}
		accepted, err = s.Verifier.Verify(prepareEnvelope, rebuildartifact.PrepareV1)
		if err != nil {
			return nil, err
		}
		if err := checkOperation(accepted, operation); err != nil {
			return nil, err
		}
		fingerprint, err := hex.DecodeString(accepted.SignerFingerprint)
		if err != nil || len(fingerprint) != 32 {
			return nil, rebuildartifact.ErrIdentity
		}
		var signerDigest [32]byte
		copy(signerDigest[:], fingerprint)
		job, err = repo.Reserve(ctx, tx, operation, prepareEnvelope, signerDigest)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		if job.Operation != operation || !bytes.Equal(job.Prepare, prepareEnvelope) || sha256.Sum256(prepareEnvelope) != job.PrepareDigest {
			return nil, ErrConflict
		}
		accepted, err = rebuildartifact.RestoreAccepted(job.Prepare, rebuildartifact.PrepareV1, job.PrepareDigest, job.SignerDigest)
		if err != nil {
````

## 第 5 步：Service.Capture（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 79 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>job.State == &quot;SEALED&quot;</code>。条件成立时执行 <code>return append(&#91;&#93;byte(nil), job.Seal...), nil</code>。当前函数在这里结束。
- 检查 <code>signer == nil &amp;&amp; s.LoadSigner != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>signer == nil</code>。条件成立时执行 <code>return nil, rebuildartifact.ErrConfig</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>len(entries) == 0</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>entry.Document &lt;= after &#124;&#124; count == ^uint64(0)</code>。条件成立时执行 <code>return nil, ErrIncomplete</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=3024:4278 -->
````go
			return nil, err
		}
		if err := checkOperation(accepted, operation); err != nil {
			return nil, err
		}
	}
	if job.State == "SEALED" {
		// 只返回已经提交的原签名；回滚释放读取锁即可，不重签也不重扫业务表。
		return append([]byte(nil), job.Seal...), nil
	}
	// 当前签名身份仅用于新封存。延迟加载让证书轮换/过期与原制品恢复分离，
	// 同时在整库扫描前报告配置错误，避免已知无法签名仍持有长事务。
	signer := s.Signer
	if signer == nil && s.LoadSigner != nil {
		signer, err = s.LoadSigner()
		if err != nil {
			return nil, err
		}
	}
	if signer == nil {
		return nil, rebuildartifact.ErrConfig
	}

	hash := sha256.New()
	var count, after uint64
	for {
		// 首次普通 SELECT 在 prepare 已验签并绑定后建立 S，后续所有页仍在同一事务。
		entries, err := repo.CurrentPage(ctx, tx, after, 500)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			break
		}
		for _, entry := range entries {
			if entry.Document <= after || count == ^uint64(0) {
				return nil, ErrIncomplete
			}
			line, err := entry.Line()
			if err != nil {
				return nil, err
			}
			hash.Write(line)
			after, count = entry.Document, count+1
		}
````

## 第 6 步：Service.Capture（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 124 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrCommitUnknown</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return envelope, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=4278:4945 -->
````go
		if err := repo.InsertEntries(ctx, tx, operation, entries); err != nil {
			return nil, err
		}
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	payload, err := (rebuildmeta.Seal{Version: 1, PrepareRef: accepted.Reference,
		Count: strconv.FormatUint(count, 10), EntriesDigest: hex.EncodeToString(digest[:])}).Bytes()
	if err != nil {
		return nil, err
	}
	envelope, err := signer.Sign(rebuildartifact.SnapshotV1, payload)
	if err != nil {
		return nil, err
	}
	if err := repo.Seal(ctx, tx, operation, count, digest, envelope); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, ErrCommitUnknown
	}
	return envelope, nil
}

````

## 第 7 步：checkOperation

插入位置：第 6 步代码之后，紧接追加；完成稿约第 148 行。

本步声明：`func checkOperation(accepted rebuildartifact.Verified, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>accepted</code> | <code>rebuildartifact.Verified</code> | 使用 <code>rebuildartifact.Verified</code> 类型；其定义处的职责：为离线重建制品签名、首次验签及恢复已接受原字节。<br>在此声明中的实际使用：<code>accepted.Payload</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>prepare.Operation != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ErrConflict</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>prepare.Operation != operation</code>。条件成立时执行 <code>return ErrConflict</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4945:5194 -->
````go
func checkOperation(accepted rebuildartifact.Verified, operation string) error {
	prepare, err := rebuildmeta.ParsePrepare(accepted.Payload)
	if err != nil {
		return err
	}
	if prepare.Operation != operation {
		return ErrConflict
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/snapshot/capture.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
