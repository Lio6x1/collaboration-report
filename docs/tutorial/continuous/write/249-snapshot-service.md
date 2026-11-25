# 分步编写 `services/document/internal/logic/snapshot/snapshot_service.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/249-snapshot-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

导出已封存原条目，并由原记录重建Search重建请求。

文件丢失不应重新采样数据库；Start响应未知也不能让操作者重新填写代次或L。

写之前先按这条执行路线理解各段的连接：

1. Service.Export从StoredPage写流并返回原seal
2. RebuildRequest按operation/snapshotRef还原原请求与指纹。

本文件需要守住的边界：

- 调用者须Flush/Sync临时文件后再发布
- 部分写入不是可导入制品。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/snapshot' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/snapshot/snapshot_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/snapshot/snapshot_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/snapshot/snapshot_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/snapshot/snapshot_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:427 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package snapshot

import (
	pb "collaboration/contracts/internalv1"
	repo "collaboration/document/internal/repository/snapshot"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"io"
)

// export.go：保留原实现，按业务合并文件。

````

## 第 2 步：Service.Export

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

Export 只把原封存条目写入调用者提供的流。调用者须写受控临时文件，等本方法
成功及文件 Flush/Sync 完成后再发布；流的部分写入不能视为一个可导入制品。
返回原 seal 字节供最后发布，文件丢失后仍可从相同 operation 精确重生。

本步声明：`func (s *Service) Export(ctx context.Context, operation string, output io.Writer) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelRepeatableRead})</code>；<code>repo.Lock(ctx, tx, operation)</code>；<code>repo.StoredPage(ctx, tx, operation, after, 500)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>rebuildmeta.OperationValid(operation)</code>；<code>repo.Lock(ctx, tx, operation)</code>；<code>job.Operation != operation</code>；<code>checkOperation(accepted, operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>output</code> | <code>io.Writer</code> | 接收输出字节的流，写入可能失败或短写；不能只看业务计算完成就宣称文件已输出。<br>在此声明中的实际使用：<code>output == nil</code>；<code>output.Write</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>append(&#91;&#93;byte(nil), job.Seal...)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrInput</code>；<code>errors.New(&quot;REBUILD_SNAPSHOT_BEGIN_FAILED&quot;)</code>；<code>err</code>；<code>ErrIncomplete</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>s == nil &#124;&#124; s.DB == nil &#124;&#124; output == nil &#124;&#124; !rebuildmeta.OperationValid(operation)</code>。条件成立时执行 <code>return nil, ErrInput</code>。当前函数在这里结束。
- 执行 <code>s.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelRepeatableRead})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_SNAPSHOT_BEGIN_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 执行 <code>repo.Lock(ctx, tx, operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>job.State != &quot;SEALED&quot; &#124;&#124; job.Operation != operation</code>。条件成立时执行 <code>return nil, ErrIncomplete</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>len(entries) == 0</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>entry.Document &lt;= after &#124;&#124; count == ^uint64(0)</code>。条件成立时执行 <code>return nil, ErrIncomplete</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=427:2048 -->
````go
// Export 只把原封存条目写入调用者提供的流。调用者须写受控临时文件，等本方法
// 成功及文件 Flush/Sync 完成后再发布；流的部分写入不能视为一个可导入制品。
// 返回原 seal 字节供最后发布，文件丢失后仍可从相同 operation 精确重生。
func (s *Service) Export(ctx context.Context, operation string, output io.Writer) ([]byte, error) {
	if s == nil || s.DB == nil || output == nil || !rebuildmeta.OperationValid(operation) {
		return nil, ErrInput
	}
	// Lock 使用 FOR UPDATE，因此不用数据库只读事务；此事务没有数据写入。
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, errors.New("REBUILD_SNAPSHOT_BEGIN_FAILED")
	}
	defer tx.Rollback()
	job, err := repo.Lock(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if job.State != "SEALED" || job.Operation != operation {
		return nil, ErrIncomplete
	}
	accepted, err := rebuildartifact.RestoreAccepted(job.Prepare, rebuildartifact.PrepareV1, job.PrepareDigest, job.SignerDigest)
	if err != nil {
		return nil, err
	}
	if err := checkOperation(accepted, operation); err != nil {
		return nil, err
	}

	hash := sha256.New()
	var after, count uint64
	for {
		entries, err := repo.StoredPage(ctx, tx, operation, after, 500)
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
````

## 第 3 步：Service.Export（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 64 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; n != len(line)</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_SNAPSHOT_OUTPUT_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>count != job.Count &#124;&#124; digest != job.EntriesDigest</code>。条件成立时执行 <code>return nil, ErrIncomplete</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_SNAPSHOT_EXPORT_FINISH_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return append(&#91;&#93;byte(nil), job.Seal...), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2048:2701 -->
````go
			n, err := output.Write(line)
			if err != nil || n != len(line) {
				// 不传播底层文件路径，也不把短写或磁盘错误吞掉后返回原 seal。
				return nil, errors.New("REBUILD_SNAPSHOT_OUTPUT_FAILED")
			}
			hash.Write(line)
			after, count = entry.Document, count+1
		}
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	if count != job.Count || digest != job.EntriesDigest {
		return nil, ErrIncomplete
	}
	if err := tx.Commit(); err != nil {
		return nil, errors.New("REBUILD_SNAPSHOT_EXPORT_FINISH_FAILED")
	}
	return append([]byte(nil), job.Seal...), nil
}

// request.go：保留原实现，按业务合并文件。

````

## 第 4 步：RebuildRequest

插入位置：第 3 步代码之后，紧接追加；完成稿约第 86 行。

RebuildRequest只从Doc已提交的封存记录还原原请求。CLI不让操作者重填代次或L，
Start回执未知时，后续Start/Get仍使用完全相同的输入与指纹。

本步声明：`func RebuildRequest(ctx context.Context, db *sql.DB, operation, snapshotRef string) (*pb.RebuildRequest, [32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.BeginTx(ctx, nil)</code>；<code>repo.Lock(ctx, tx, operation)</code> |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db == nil</code>；<code>db.BeginTx</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>rebuildmeta.OperationValid(operation)</code>；<code>repo.Lock(ctx, tx, operation)</code>；<code>p.Operation != operation</code>；<code>rebuildmeta.RequestFingerprint(operation, generation, ref, watermark)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>snapshotRef</code> | <code>string</code> | 在此声明中的实际使用：<code>ref != snapshotRef</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.RebuildRequest</code> | 实际返回表达式：<code>nil</code>；<code>&amp;pb.RebuildRequest{OperationId: operation, Generation: generation, SnapshotRef: ref, WatermarkJsonUtf8: watermark}</code> |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 实际返回表达式：<code>empty</code>；<code>fingerprint</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrInput</code>；<code>errors.New(&quot;REBUILD_SNAPSHOT_BEGIN_FAILED&quot;)</code>；<code>err</code>；<code>ErrIncomplete</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>db == nil &#124;&#124; !rebuildmeta.OperationValid(operation)</code>。条件成立时执行 <code>return nil, empty, ErrInput</code>。当前函数在这里结束。
- 执行 <code>db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, empty, errors.New(&quot;REBUILD_SNAPSHOT_BEGIN_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 执行 <code>repo.Lock(ctx, tx, operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, empty, err</code>。当前函数在这里结束。
- 检查 <code>job.State != &quot;SEALED&quot;</code>。条件成立时执行 <code>return nil, empty, ErrIncomplete</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; ref != snapshotRef</code>。条件成立时执行 <code>return nil, empty, ErrConflict</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; p.Operation != operation</code>。条件成立时执行 <code>return nil, empty, ErrConflict</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, empty, err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=2701:4281 -->
````go
// RebuildRequest只从Doc已提交的封存记录还原原请求。CLI不让操作者重填代次或L，
// Start回执未知时，后续Start/Get仍使用完全相同的输入与指纹。
func RebuildRequest(ctx context.Context, db *sql.DB, operation, snapshotRef string) (*pb.RebuildRequest, [32]byte, error) {
	var empty [32]byte
	if db == nil || !rebuildmeta.OperationValid(operation) {
		return nil, empty, ErrInput
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, empty, errors.New("REBUILD_SNAPSHOT_BEGIN_FAILED")
	}
	defer tx.Rollback()
	job, err := repo.Lock(ctx, tx, operation)
	if err != nil {
		return nil, empty, err
	}
	if job.State != "SEALED" {
		return nil, empty, ErrIncomplete
	}
	ref, err := rebuildartifact.Reference(job.Seal, rebuildartifact.SnapshotV1)
	if err != nil || ref != snapshotRef {
		return nil, empty, ErrConflict
	}
	accepted, err := rebuildartifact.RestoreAccepted(job.Prepare, rebuildartifact.PrepareV1, job.PrepareDigest, job.SignerDigest)
	if err != nil {
		return nil, empty, err
	}
	p, err := rebuildmeta.ParsePrepare(accepted.Payload)
	if err != nil || p.Operation != operation {
		return nil, empty, ErrConflict
	}
	d, err := rebuildmeta.DescriptorFor(p, accepted.Reference, ref)
	if err != nil {
		return nil, empty, err
	}
	watermark, err := d.Bytes()
	if err != nil {
		return nil, empty, err
	}
	generation, err := rebuildmeta.Decimal(p.TargetGeneration, true)
	if err != nil {
		return nil, empty, err
	}
	fingerprint, err := rebuildmeta.RequestFingerprint(operation, generation, ref, watermark)
	if err != nil {
````

## 第 5 步：RebuildRequest（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 131 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, empty, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, empty, errors.New(&quot;REBUILD_REQUEST_RELEASE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Rollback()</code>。
- 此处通过 <code>return &amp;pb.RebuildRequest{OperationId: operation, Generation: generation, SnapshotRef: ref, WatermarkJsonUtf8: watermark}, fingerprint, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=4281:4649 -->
````go
		return nil, empty, err
	}
	// 释放本地读取锁后才允许调用Search，避免跨服务RPC占有Doc事务。
	if err := tx.Rollback(); err != nil {
		return nil, empty, errors.New("REBUILD_REQUEST_RELEASE_FAILED")
	}
	return &pb.RebuildRequest{OperationId: operation, Generation: generation, SnapshotRef: ref, WatermarkJsonUtf8: watermark}, fingerprint, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/snapshot/snapshot_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
