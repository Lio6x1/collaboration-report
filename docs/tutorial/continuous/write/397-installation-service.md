# 分步编写 `services/search/internal/logic/rebuild/installation_service.go`

先按 [S04 · 先封存L，再导入签名快照并种子回填](../lessons/search/04-rebuild-snapshot.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/397-installation-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在受控目录接受Doc快照，再按固定文件分批播种目标索引。

先验证完整文件才能让任何条目产生可见副作用；文件先发布后落SQL，避免IMPORTED指向半份副本。

写之前先按这条执行路线理解各段的连接：

1. Importer.Import先恢复原号；新导入验证签名与PrepareRef，流式扫描数量/摘要后将完整entries和封口发布到accepted。
2. 短事务重新核原绑定后记录IMPORTED；已接受副本缺失时只能用相同inbox和原摘要恢复。
3. Seeder.Step核原Kafka保留窗口仍覆盖L，snapshotPage整流校验后只交最多32项。
4. Seeder.apply对每篇文档用短事务重检job与after，写物理应用并推进游标；空页才进入REPLAYING。

本文件需要守住的边界：

- 不能在Scan回调中直接写ES，否则文件尾部损坏可能在摘要失败前污染索引。
- 长文件IO放在代次锁外；未知提交保留原after继续，不换快照或任务号。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/logic/rebuild' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/logic/rebuild/installation_service.go') -and (Get-Item -LiteralPath 'services/search/internal/logic/rebuild/installation_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/logic/rebuild/installation_service.go')) { New-Item -ItemType File -Path 'services/search/internal/logic/rebuild/installation_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:582 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	"bytes"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/client/logsource"
	generation "collaboration/search/internal/repository/generation"
	repository "collaboration/search/internal/repository/rebuild"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// import.go：保留原实现，按业务合并文件。

````

## 第 2 步：Importer

插入位置：第 1 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`type Importer struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>DB</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>Verifier</code> | <code>*rebuildartifact.Verifier</code> | 本服务CA信任下的签名制品验证器。 |
| <code>Owner</code> | <code>string</code> | 本实例owner标识，防止串用其他环境准备。 |
| <code>Alias</code> | <code>string</code> | 本服务要重建的搜索别名。 |
| <code>Inbox</code> | <code>string</code> | 接收Doc制品的受控绝对目录。 |
| <code>Accepted</code> | <code>string</code> | 发布完整校验副本的受控绝对目录。 |

<!-- write-step: 2 bytes=582:742 -->
````go
type Importer struct {
	DB                            *sql.DB
	Verifier                      *rebuildartifact.Verifier
	Owner, Alias, Inbox, Accepted string
}

````

## 第 3 步：Importer.Import

插入位置：第 2 步代码之后，紧接追加；完成稿约第 32 行。

Import先恢复原绑定，再在事务外验证并发布完整流，最后短事务复核原绑定后接受。
文件先于SQL发布：崩溃最多留下内容地址孤儿，不能留下指向半份文件的IMPORTED。

本步声明：`func (s *Importer) Import(ctx context.Context, operation, reference string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.inspect(ctx, operation)</code>；<code>ctx: ctx</code>；<code>context.WithTimeout(ctx, 5*time.Second)</code> |
| <code>operation</code> | <code>string</code> | 原准备/导入/重建共用的操作号。<br>在此声明中的实际使用：<code>rebuildmeta.OperationValid(operation)</code>；<code>s.inspect(ctx, operation)</code>；<code>s.lock(short, tx, operation)</code>；<code>repository.SaveImport(short, tx, operation, row)</code> |
| <code>reference</code> | <code>string</code> | 原Doc签名快照的内容地址引用，仅接受.snapshot.json格式。<br>在此声明中的实际使用：<code>rebuildartifact.FileName(reference)</code>；<code>actual != reference</code>；<code>rebuildartifact.ReadFile(s.Inbox, reference)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>accepted.Envelope</code>；<code>row.Envelope</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_IMPORT_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>；<code>rebuildartifact.ErrConfig</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>s == nil &#124;&#124; s.DB == nil &#124;&#124; !rebuildmeta.OperationValid(operation) &#124;&#124; err != nil &#124;&#124; !strings.HasSuffix(name, &quot;.snapshot.json&quot;) &#124;&#124; s.Inbox == &quot;&quot; &#124;&#124; s.Accepted == &quot;&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_IMPORT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>found</code>。
- 检查 <code>actual != reference</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>s.Verifier == nil</code>。条件成立时执行 <code>return nil, rebuildartifact.ErrConfig</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(signer) != 32</code>。条件成立时执行 <code>return nil, rebuildartifact.ErrIdentity</code>。当前函数在这里结束。
- 检查 <code>row.Seal.PrepareRef != prepareRef</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=742:2506 -->
````go
// Import先恢复原绑定，再在事务外验证并发布完整流，最后短事务复核原绑定后接受。
// 文件先于SQL发布：崩溃最多留下内容地址孤儿，不能留下指向半份文件的IMPORTED。
func (s *Importer) Import(ctx context.Context, operation, reference string) ([]byte, error) {
	name, err := rebuildartifact.FileName(reference)
	if s == nil || s.DB == nil || !rebuildmeta.OperationValid(operation) || err != nil || !strings.HasSuffix(name, ".snapshot.json") || s.Inbox == "" || s.Accepted == "" {
		return nil, errors.New("SEARCH_REBUILD_IMPORT_INVALID")
	}
	binding, original, found, err := s.inspect(ctx, operation)
	if err != nil {
		return nil, err
	}
	var row repository.Imported
	if found {
		actual, _ := rebuildartifact.Reference(original.Envelope, rebuildartifact.SnapshotV1)
		if actual != reference {
			return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
		}
		row = original
	} else {
		raw, err := rebuildartifact.ReadFile(s.Inbox, reference)
		if err != nil {
			return nil, err
		}
		if s.Verifier == nil {
			return nil, rebuildartifact.ErrConfig
		}
		verified, err := s.Verifier.Verify(raw, rebuildartifact.SnapshotV1)
		if err != nil {
			return nil, err
		}
		row.Seal, err = rebuildmeta.ParseSeal(verified.Payload)
		if err != nil {
			return nil, err
		}
		row.Envelope = raw
		row.Digest = sha256.Sum256(raw)
		signer, err := hex.DecodeString(verified.SignerFingerprint)
		if err != nil || len(signer) != 32 {
			return nil, rebuildartifact.ErrIdentity
		}
		copy(row.SignerDigest[:], signer)
	}
	prepareRef, _ := rebuildartifact.Reference(binding.Envelope, rebuildartifact.PrepareV1)
	if row.Seal.PrepareRef != prepareRef {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
````

## 第 4 步：Importer.Import（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 78 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>found</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>!errors.Is(err, os.ErrNotExist)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_INPUT_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; !info.Mode().IsRegular()</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_INPUT_FAILED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_INPUT_FAILED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：登记 <code>f.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：执行 <code>rebuildmeta.Scan(io.TeeReader(&amp;importReader{ctx: ctx, reader: f}, w), nil)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>strconv.FormatUint(count, 10) != row.Seal.Count &#124;&#124; hex.EncodeToString(digest&#91;:&#93;) != row.Seal.EntriesDigest</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_ENTRIES_MISMATCH&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, 5*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.DB.BeginTx(short, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=2506:4351 -->
````go
	entriesName := strings.TrimSuffix(name, ".snapshot.json") + ".entries.ndjson"
	// 已接受副本优先恢复；丢失时允许从相同inbox重建，但仍核完整原摘要，绝不重签。
	inputDir := s.Inbox
	if found {
		if _, err := os.Lstat(filepath.Join(s.Accepted, entriesName)); err == nil {
			inputDir = s.Accepted
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("SEARCH_REBUILD_INPUT_FAILED")
		}
	}
	if err := rebuildartifact.PublishFile(s.Accepted, entriesName, func(w io.Writer) error {
		info, err := os.Lstat(filepath.Join(inputDir, entriesName))
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("SEARCH_REBUILD_INPUT_FAILED")
		}
		f, err := os.Open(filepath.Join(inputDir, entriesName))
		if err != nil {
			return errors.New("SEARCH_REBUILD_INPUT_FAILED")
		}
		defer f.Close()
		count, digest, err := rebuildmeta.Scan(io.TeeReader(&importReader{ctx: ctx, reader: f}, w), nil)
		if err != nil {
			return err
		}
		if strconv.FormatUint(count, 10) != row.Seal.Count || hex.EncodeToString(digest[:]) != row.Seal.EntriesDigest {
			return errors.New("SEARCH_REBUILD_ENTRIES_MISMATCH")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := rebuildartifact.PublishFile(s.Accepted, name, func(w io.Writer) error { _, err := io.Copy(w, bytes.NewReader(row.Envelope)); return err }); err != nil {
		return nil, err
	}
	// 全流IO已退出；此处只复核不可变绑定，不在代次锁内重扫文件。
	short, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(short, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	current, accepted, has, err := s.lock(short, tx, operation)
	if err != nil {
		return nil, err
	}
````

## 第 5 步：Importer.Import（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 124 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!bytes.Equal(binding.Envelope, current.Envelope)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>has</code>。
- 检查 <code>!bytes.Equal(accepted.Envelope, row.Envelope)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return accepted.Envelope, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_IMPORT_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return row.Envelope, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=4351:4853 -->
````go
	if !bytes.Equal(binding.Envelope, current.Envelope) {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	if has {
		if !bytes.Equal(accepted.Envelope, row.Envelope) {
			return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
		}
		return accepted.Envelope, nil
	}
	if err := repository.SaveImport(short, tx, operation, row); err != nil {
		return nil, err
	}
	if tx.Commit() != nil {
		return nil, errors.New("SEARCH_REBUILD_IMPORT_COMMIT_UNKNOWN")
	}
	return row.Envelope, nil
}

````

## 第 6 步：Importer.inspect

插入位置：第 5 步代码之后，紧接追加；完成稿约第 142 行。

本步声明：`func (s *Importer) inspect(ctx context.Context, operation string) (repository.Binding, repository.Imported, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(ctx, 5*time.Second)</code> |
| <code>operation</code> | <code>string</code> | 原准备/导入/重建共用的操作号。<br>在此声明中的实际使用：<code>s.lock(short, tx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.Binding</code> | 使用 <code>repository.Binding</code> 类型；其定义处的职责：持久化重建准备绑定、三分区封存前沿、任务创建及分页验证进度。<br>由 <code>s.lock(short, tx, operation)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>repository.Binding{}</code> |
| <code>(匿名/未命名)</code> | <code>repository.Imported</code> | 使用 <code>repository.Imported</code> 类型；其定义处的职责：保存已验证的导入封口，并维护快照播种进度。<br>由 <code>s.lock(short, tx, operation)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>repository.Imported{}</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>s.lock(short, tx, operation)</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>false</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.lock(short, tx, operation)</code> 的第 4 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 5*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.DB.BeginTx(short, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return repository.Binding{}, repository.Imported{}, false, errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 此处通过 <code>return s.lock(short, tx, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=4853:5328 -->
````go
func (s *Importer) inspect(ctx context.Context, operation string) (repository.Binding, repository.Imported, bool, error) {
	short, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(short, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return repository.Binding{}, repository.Imported{}, false, errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	return s.lock(short, tx, operation)
}

````

## 第 7 步：Importer.lock

插入位置：第 6 步代码之后，紧接追加；完成稿约第 153 行。

本步声明：`func (s *Importer) lock(ctx context.Context, tx *sql.Tx, operation string) (repository.Binding, repository.Imported, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>generation.Find(ctx, tx, s.Alias, true)</code>；<code>repository.Find(ctx, tx, operation)</code>；<code>repository.FindImport(ctx, tx, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>generation.Find(ctx, tx, s.Alias, true)</code>；<code>repository.Find(ctx, tx, operation)</code>；<code>repository.FindImport(ctx, tx, operation)</code> |
| <code>operation</code> | <code>string</code> | 原准备/导入/重建共用的操作号。<br>在此声明中的实际使用：<code>repository.Find(ctx, tx, operation)</code>；<code>repository.FindImport(ctx, tx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.Binding</code> | 使用 <code>repository.Binding</code> 类型；其定义处的职责：持久化重建准备绑定、三分区封存前沿、任务创建及分页验证进度。<br>由 <code>fail(err)</code> 的第 1 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_GENERATION_MISSING&quot;))</code> 的第 1 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_PREPARE_MISSING&quot;))</code> 的第 1 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;))</code> 的第 1 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_IMPORT_INVALID&quot;))</code> 的第 1 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_GENERATION_INVALID&quot;))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>binding</code> |
| <code>(匿名/未命名)</code> | <code>repository.Imported</code> | 使用 <code>repository.Imported</code> 类型；其定义处的职责：保存已验证的导入封口，并维护快照播种进度。<br>由 <code>fail(err)</code> 的第 2 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_GENERATION_MISSING&quot;))</code> 的第 2 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_PREPARE_MISSING&quot;))</code> 的第 2 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;))</code> 的第 2 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_IMPORT_INVALID&quot;))</code> 的第 2 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_GENERATION_INVALID&quot;))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>row</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>fail(err)</code> 的第 3 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_GENERATION_MISSING&quot;))</code> 的第 3 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_PREPARE_MISSING&quot;))</code> 的第 3 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;))</code> 的第 3 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_IMPORT_INVALID&quot;))</code> 的第 3 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_GENERATION_INVALID&quot;))</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>found</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>fail(err)</code> 的第 4 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_GENERATION_MISSING&quot;))</code> 的第 4 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_PREPARE_MISSING&quot;))</code> 的第 4 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;))</code> 的第 4 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_IMPORT_INVALID&quot;))</code> 的第 4 个返回项原样转交。<br>由 <code>fail(errors.New(&quot;SEARCH_REBUILD_GENERATION_INVALID&quot;))</code> 的第 4 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return repository.Binding{}, repository.Imported{}, false, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>!exists</code>。条件成立时执行 <code>return fail(errors.New(&quot;SEARCH_GENERATION_MISSING&quot;))</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>!exists</code>。条件成立时执行 <code>return fail(errors.New(&quot;SEARCH_REBUILD_PREPARE_MISSING&quot;))</code>。当前函数在这里结束。
- 检查 <code>p.Owner != s.Owner &#124;&#124; p.Alias != s.Alias &#124;&#124; binding.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return fail(errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;))</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>found</code>。
- 检查 <code>row.Seal.PrepareRef != ref &#124;&#124; (binding.State != &quot;IMPORTED&quot; &amp;&amp; binding.State != &quot;ACCEPTED&quot;)</code>。条件成立时执行 <code>return fail(errors.New(&quot;SEARCH_REBUILD_IMPORT_INVALID&quot;))</code>。当前函数在这里结束。
- 检查 <code>binding.State != &quot;PREPARED&quot; &#124;&#124; source.Physical != p.Source &#124;&#124; strconv.FormatUint(source.Generation, 10) != p.SourceGeneration &#124;&#124; strconv.FormatUint(source.Revision, 10) != p.SourceRevision</code>。条件成立时执行 <code>return fail(errors.New(&quot;SEARCH_REBUILD_GENERATION_INVALID&quot;))</code>。当前函数在这里结束。
- 此处通过 <code>return binding, row, found, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=5328:6790 -->
````go
func (s *Importer) lock(ctx context.Context, tx *sql.Tx, operation string) (repository.Binding, repository.Imported, bool, error) {
	fail := func(err error) (repository.Binding, repository.Imported, bool, error) {
		return repository.Binding{}, repository.Imported{}, false, err
	}
	source, exists, err := generation.Find(ctx, tx, s.Alias, true)
	if err != nil {
		return fail(err)
	}
	if !exists {
		return fail(errors.New("SEARCH_GENERATION_MISSING"))
	}
	binding, exists, err := repository.Find(ctx, tx, operation)
	if err != nil {
		return fail(err)
	}
	if !exists {
		return fail(errors.New("SEARCH_REBUILD_PREPARE_MISSING"))
	}
	p := binding.Prepare
	if p.Owner != s.Owner || p.Alias != s.Alias || binding.State == "REJECTED" {
		return fail(errors.New("SEARCH_REBUILD_OPERATION_CONFLICT"))
	}
	row, found, err := repository.FindImport(ctx, tx, operation)
	if err != nil {
		return fail(err)
	}
	if found {
		ref, _ := rebuildartifact.Reference(binding.Envelope, rebuildartifact.PrepareV1)
		if row.Seal.PrepareRef != ref || (binding.State != "IMPORTED" && binding.State != "ACCEPTED") {
			return fail(errors.New("SEARCH_REBUILD_IMPORT_INVALID"))
		}
	} else if binding.State != "PREPARED" || source.Physical != p.Source || strconv.FormatUint(source.Generation, 10) != p.SourceGeneration || strconv.FormatUint(source.Revision, 10) != p.SourceRevision {
		return fail(errors.New("SEARCH_REBUILD_GENERATION_INVALID"))
	}
	return binding, row, found, nil
}

````

## 第 8 步：importReader

插入位置：第 7 步代码之后，紧接追加；完成稿约第 190 行。

本步声明：`type importReader struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>reader</code> | <code>io.Reader</code> | 底层文件Reader，每次读前先检查ctx取消。 |

<!-- write-step: 8 bytes=6790:6862 -->
````go
type importReader struct {
	ctx    context.Context
	reader io.Reader
}

````

## 第 9 步：importReader.Read

插入位置：第 8 步代码之后，紧接追加；完成稿约第 195 行。

本步声明：`func (r *importReader) Read(p []byte) (int, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>p</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>r.reader.Read(p)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 由 <code>r.reader.Read(p)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>0</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>r.reader.Read(p)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return r.reader.Read(p)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=6862:7056 -->
````go
func (r *importReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// seed.go：保留原实现，按业务合并文件。

````

## 第 10 步：SeedWriter

插入位置：第 9 步代码之后，紧接追加；完成稿约第 204 行。

本步声明：`type SeedWriter interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>SeedInTransaction</code> | <code>(context.Context, *sql.Tx, string, rebuildmeta.Entry) error</code> | ctx和调用者原tx、固定目标物理索引、Doc快照Entry；在同一事务记录物理应用，不写重放事件账本。 |

<!-- write-step: 10 bytes=7056:7164 -->
````go
type SeedWriter interface {
	SeedInTransaction(context.Context, *sql.Tx, string, rebuildmeta.Entry) error
}
````

## 第 11 步：SeedIndex

插入位置：第 10 步代码之后，紧接追加；完成稿约第 207 行。

本步声明：`type SeedIndex interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>EnsureIndex</code> | <code>(context.Context, string) error</code> | ctx与原准备目标物理索引名；创建或核验本owner/schema索引，错误则不播种。 |

<!-- write-step: 11 bytes=7164:7238 -->
````go
type SeedIndex interface {
	EnsureIndex(context.Context, string) error
}

````

## 第 12 步：Seeder

插入位置：第 11 步代码之后，紧接追加；完成稿约第 211 行。

本步声明：`type Seeder struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>DB</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>Owner</code> | <code>string</code> | 原准备必须匹配的实例owner。 |
| <code>Alias</code> | <code>string</code> | 原准备绑定的稳定搜索别名。 |
| <code>AcceptedDir</code> | <code>string</code> | 保存已接受封口和完整entries的绝对目录。 |
| <code>Writer</code> | <code>SeedWriter</code> | 逐篇向固定目标应用权威正文的用例能力。 |
| <code>Indexes</code> | <code>SeedIndex</code> | 创建或验证目标物理索引的能力。 |
| <code>Logs</code> | <code>Logs</code> | 核对原Kafka身份及保留窗口是否仍覆盖L。 |

<!-- write-step: 12 bytes=7238:7438 -->
````go
type Seeder struct {
	DB                        *sql.DB
	Owner, Alias, AcceptedDir string
	Writer                    SeedWriter
	Indexes                   SeedIndex
	Logs                      Logs
}

````

## 第 13 步：seedJob

插入位置：第 12 步代码之后，紧接追加；完成稿约第 219 行。

本步声明：`type seedJob struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>prepare</code> | <code>rebuildmeta.Prepare</code> | 原签名Prepare业务载荷。 |
| <code>imported</code> | <code>repository.Imported</code> | 已接受Doc封口及原签名摘要。 |
| <code>job</code> | <code>repository.Job</code> | SQL原任务状态、目标和播种游标。 |

<!-- write-step: 13 bytes=7438:7548 -->
````go
type seedJob struct {
	prepare  rebuildmeta.Prepare
	imported repository.Imported
	job      repository.Job
}

````

## 第 14 步：Seeder.Step

插入位置：第 13 步代码之后，紧接追加；完成稿约第 225 行。

Step只取一个已验证批次。整库文件校验在事务外，逐文档事务串行保护状态，
不使用到期租约推断另一个写步骤已停止，也不持有整库级长事务。

本步声明：`func (s *Seeder) Step(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 30*time.Second)</code>；<code>context.WithTimeout(ctx, 30*time.Second)</code>；<code>repository.PendingSnapshot(ctx, s.DB)</code>；<code>s.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>nil</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>s == nil &#124;&#124; s.DB == nil &#124;&#124; s.Writer == nil &#124;&#124; s.Indexes == nil &#124;&#124; s.Logs == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 30*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; operation == &quot;&quot;</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>input.job.State != &quot;SNAPSHOT&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>window.Partitions&#91;partition&#93;.First &gt; start &#124;&#124; window.Partitions&#91;partition&#93;.Last &lt; start</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_LOG_WINDOW_LOST&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=7548:9218 -->
````go
// Step只取一个已验证批次。整库文件校验在事务外，逐文档事务串行保护状态，
// 不使用到期租约推断另一个写步骤已停止，也不持有整库级长事务。
func (s *Seeder) Step(ctx context.Context) error {
	if s == nil || s.DB == nil || s.Writer == nil || s.Indexes == nil || s.Logs == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	operation, err := repository.PendingSnapshot(ctx, s.DB)
	if err != nil || operation == "" {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	input, err := s.lock(ctx, tx, operation)
	tx.Rollback()
	if err != nil {
		return err
	}
	if input.job.State != "SNAPSHOT" {
		return nil
	}
	identity := logsource.Identity{Cluster: input.prepare.Cluster, Topic: input.prepare.Topic}
	uuid, _ := hex.DecodeString(input.prepare.TopicID)
	copy(identity.TopicID[:], uuid)
	window, err := s.Logs.Capture(ctx, &identity)
	if err != nil {
		return err
	}
	for partition, raw := range input.prepare.Starts {
		start, _ := strconv.ParseInt(raw, 10, 64)
		if window.Partitions[partition].First > start || window.Partitions[partition].Last < start {
			return errors.New("SEARCH_REBUILD_LOG_WINDOW_LOST")
		}
	}
	page, err := snapshotPage(ctx, s.AcceptedDir, input.job.SnapshotRef, input.imported.Seal, input.job.SnapshotAfter)
	if err != nil {
		return err
	}
	if err := s.Indexes.EnsureIndex(ctx, input.job.Target); err != nil {
		return err
	}
	after := input.job.SnapshotAfter
````

## 第 15 步：Seeder.Step（接着实现）

插入位置：第 14 步代码之后，紧接追加；完成稿约第 270 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(page) == 0</code>。条件成立时执行 <code>return s.apply(ctx, operation, input.job.ID, after, nil)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=9218:9471 -->
````go
	for _, entry := range page {
		if err := s.apply(ctx, operation, input.job.ID, after, &entry); err != nil {
			return err
		}
		after = entry.Document
	}
	if len(page) == 0 {
		return s.apply(ctx, operation, input.job.ID, after, nil)
	}
	return nil
}

````

## 第 16 步：Seeder.lock

插入位置：第 15 步代码之后，紧接追加；完成稿约第 282 行。

本步声明：`func (s *Seeder) lock(ctx context.Context, tx *sql.Tx, operation string) (seedJob, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>generation.Find(ctx, tx, s.Alias, false)</code>；<code>repository.Find(ctx, tx, operation)</code>；<code>repository.LoadJobBinding(ctx, tx, operation)</code>；<code>repository.LoadJob(ctx, tx, id)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>generation.Find(ctx, tx, s.Alias, false)</code>；<code>repository.Find(ctx, tx, operation)</code>；<code>repository.LoadJobBinding(ctx, tx, operation)</code>；<code>repository.LoadJob(ctx, tx, id)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 原准备/导入/重建共用的操作号。<br>在此声明中的实际使用：<code>repository.Find(ctx, tx, operation)</code>；<code>repository.LoadJobBinding(ctx, tx, operation)</code>；<code>repository.FindImport(ctx, tx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>seedJob</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_JOB_INVALID&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!found &#124;&#124; binding.State != &quot;ACCEPTED&quot; &#124;&#124; binding.Prepare.Owner != s.Owner &#124;&#124; binding.Prepare.Alias != s.Alias</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bound.JobID.Valid</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_REBUILD_JOB_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>result.job.State != &quot;SNAPSHOT&quot; &amp;&amp; result.job.State != &quot;REPLAYING&quot; &amp;&amp; result.job.State != &quot;SWITCHING&quot;</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>current.Physical != p.Source &#124;&#124; strconv.FormatUint(current.Generation, 10) != p.SourceGeneration &#124;&#124; strconv.FormatUint(current.Revision, 10) != p.SourceRevision &#124;&#124; result.job.Target != p.Target</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_REBUILD_GENERATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_REBUILD_NOT_IMPORTED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=9471:11065 -->
````go
func (s *Seeder) lock(ctx context.Context, tx *sql.Tx, operation string) (seedJob, error) {
	var result seedJob
	current, found, err := generation.Find(ctx, tx, s.Alias, false)
	if err != nil {
		return result, err
	}
	if !found {
		return result, errors.New("SEARCH_GENERATION_MISSING")
	}
	binding, found, err := repository.Find(ctx, tx, operation)
	if err != nil {
		return result, err
	}
	if !found || binding.State != "ACCEPTED" || binding.Prepare.Owner != s.Owner || binding.Prepare.Alias != s.Alias {
		return result, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	bound, err := repository.LoadJobBinding(ctx, tx, operation)
	if err != nil {
		return result, err
	}
	id, err := rebuildmeta.Decimal(bound.JobID.String, true)
	if err != nil || !bound.JobID.Valid {
		return result, errors.New("SEARCH_REBUILD_JOB_INVALID")
	}
	result.job, err = repository.LoadJob(ctx, tx, id)
	if err != nil {
		return result, err
	}
	result.prepare = binding.Prepare
	if result.job.State != "SNAPSHOT" && result.job.State != "REPLAYING" && result.job.State != "SWITCHING" {
		return result, nil
	}
	p := result.prepare
	if current.Physical != p.Source || strconv.FormatUint(current.Generation, 10) != p.SourceGeneration || strconv.FormatUint(current.Revision, 10) != p.SourceRevision || result.job.Target != p.Target {
		return result, errors.New("SEARCH_REBUILD_GENERATION_CHANGED")
	}
	result.imported, found, err = repository.FindImport(ctx, tx, operation)
	if err != nil {
		return result, err
	}
	if !found {
		return result, errors.New("SEARCH_REBUILD_NOT_IMPORTED")
	}
	return result, nil
}

````

## 第 17 步：Seeder.apply

插入位置：第 16 步代码之后，紧接追加；完成稿约第 328 行。

本步声明：`func (s *Seeder) apply(parent context.Context, operation string, id, after uint64, entry *rebuildmeta.Entry) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 5*time.Second)</code> |
| <code>operation</code> | <code>string</code> | 原准备/导入/重建共用的操作号。<br>在此声明中的实际使用：<code>s.lock(ctx, tx, operation)</code> |
| <code>id</code> | <code>uint64</code> | 当前原job主键。<br>在此声明中的实际使用：<code>current.job.ID != id</code>；<code>repository.FinishSnapshot(ctx, tx, id)</code>；<code>repository.AdvanceSnapshot(ctx, tx, id, after, entry.Document)</code> |
| <code>after</code> | <code>uint64</code> | 已经播种完成的最后文档编号。<br>在此声明中的实际使用：<code>current.job.SnapshotAfter != after</code>；<code>entry.Document &lt;= after</code>；<code>repository.AdvanceSnapshot(ctx, tx, id, after, entry.Document)</code> |
| <code>entry</code> | <code>*rebuildmeta.Entry</code> | 本篇快照条目；nil表示已验证的空尾页，应结束播种。<br>在此声明中的实际使用：<code>entry == nil</code>；<code>entry.Document</code>；<code>s.Writer.SeedInTransaction(ctx, tx, current.job.Target, *entry)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_COMMIT_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 5*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.job.ID != id &#124;&#124; current.job.State != &quot;SNAPSHOT&quot; &#124;&#124; current.job.SnapshotAfter != after</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>entry == nil</code>。
- 检查 <code>entry.Document &lt;= after</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=11065:12208 -->
````go
func (s *Seeder) apply(parent context.Context, operation string, id, after uint64, entry *rebuildmeta.Entry) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	current, err := s.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if current.job.ID != id || current.job.State != "SNAPSHOT" || current.job.SnapshotAfter != after {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	if entry == nil {
		err = repository.FinishSnapshot(ctx, tx, id)
	} else {
		if entry.Document <= after {
			return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
		}
		if err = s.Writer.SeedInTransaction(ctx, tx, current.job.Target, *entry); err == nil {
			err = repository.AdvanceSnapshot(ctx, tx, id, after, entry.Document)
		}
	}
	if err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return nil
}

// snapshot_page.go：保留原实现，按业务合并文件。

````

## 第 18 步：snapshotPage

插入位置：第 17 步代码之后，紧接追加；完成稿约第 364 行。

snapshotPage先核完整原副本，再交付一个有界批次。不能在Scan回调中直接写ES，
否则文件尾部损坏会在摘要失败前产生未经封存证明的可见副作用。

本步声明：`func snapshotPage(ctx context.Context, dir, reference string, seal rebuildmeta.Seal, after uint64) ([]rebuildmeta.Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx: ctx</code> |
| <code>dir</code> | <code>string</code> | 本服务绝对路径accepted目录。<br>在此声明中的实际使用：<code>filepath.IsAbs(dir)</code>；<code>filepath.Join(dir, strings.TrimSuffix(name, &quot;.snapshot.json&quot;)+&quot;.entries.ndjson&quot;)</code> |
| <code>reference</code> | <code>string</code> | 原Doc签名快照的内容地址引用，仅接受.snapshot.json格式。<br>在此声明中的实际使用：<code>rebuildartifact.FileName(reference)</code> |
| <code>seal</code> | <code>rebuildmeta.Seal</code> | 已经验签接受的Doc封口，提供总数和entries摘要。<br>在此声明中的实际使用：<code>seal.Count</code>；<code>seal.EntriesDigest</code> |
| <code>after</code> | <code>uint64</code> | 已经播种完成的最后文档编号。<br>在此声明中的实际使用：<code>entry.Document &gt; after</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;rebuildmeta.Entry</code> | 使用 <code>rebuildmeta.Entry</code> 类型；其定义处的职责：流式编码和校验Doc交给Search的重建元数据行。<br>实际返回表达式：<code>nil</code>；<code>page</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_SNAPSHOT_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; !filepath.IsAbs(dir) &#124;&#124; !strings.HasSuffix(name, &quot;.snapshot.json&quot;)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !info.Mode().IsRegular()</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>f.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>rebuildmeta.Scan(&amp;importReader{ctx: ctx, reader: f}, func(entry rebuildmeta.Entry) error { 		if entry.Document &gt; after &amp;&amp; len(page) &lt; 32 { 			page = append(page, entry) 		} 		return nil 	})</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 在本段的匿名函数内：检查 <code>entry.Document &gt; after &amp;&amp; len(page) &lt; 32</code>。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; strconv.FormatUint(count, 10) != seal.Count &#124;&#124; hex.EncodeToString(digest&#91;:&#93;) != seal.EntriesDigest</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_SNAPSHOT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=12208:13556 -->
````go
// snapshotPage先核完整原副本，再交付一个有界批次。不能在Scan回调中直接写ES，
// 否则文件尾部损坏会在摘要失败前产生未经封存证明的可见副作用。
func snapshotPage(ctx context.Context, dir, reference string, seal rebuildmeta.Seal, after uint64) ([]rebuildmeta.Entry, error) {
	name, err := rebuildartifact.FileName(reference)
	if err != nil || !filepath.IsAbs(dir) || !strings.HasSuffix(name, ".snapshot.json") {
		return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE")
	}
	path := filepath.Join(dir, strings.TrimSuffix(name, ".snapshot.json")+".entries.ndjson")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE")
	}
	defer f.Close()
	page := make([]rebuildmeta.Entry, 0, 32)
	count, digest, err := rebuildmeta.Scan(&importReader{ctx: ctx, reader: f}, func(entry rebuildmeta.Entry) error {
		if entry.Document > after && len(page) < 32 {
			page = append(page, entry)
		}
		return nil
	})
	if err != nil || strconv.FormatUint(count, 10) != seal.Count || hex.EncodeToString(digest[:]) != seal.EntriesDigest {
		return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_INVALID")
	}
	return page, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/logic/rebuild/installation_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/04-rebuild-snapshot.md)。
