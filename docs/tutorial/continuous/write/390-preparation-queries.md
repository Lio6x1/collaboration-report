# 分步编写 `services/search/internal/repository/rebuild/preparation_queries.go`

先按 [S04 · 先封存L，再导入签名快照并种子回填](../lessons/search/04-rebuild-snapshot.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/390-preparation-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久化重建准备绑定、三分区封存前沿、任务创建及分页验证进度。

这些记录共同证明同一次重建用了哪份快照和哪段日志。统一 generation→binding→job 的锁顺序，避免准备、推进、切换互相倒锁。

写之前先按这条执行路线理解各段的连接：

1. Find 以原接受摘要还原Prepare，再逐字段与数据库列核对；CheckAvailable防止同别名并行准备。
2. Insert 保存PREPARED封装；AcceptJob 在同一事务建立job、progress和三个从原L开始的专用检查点，再将binding置ACCEPTED。
3. EncodeFrontier/DecodeFrontier用规范十进制字符串保存H；SealFrontier将H与SWITCHING一起提交，不重新捕获覆盖。
4. LoadValidation/SaveValidation保存SNAPSHOT和APPLICATION两阶段的分页位置、累计数量与摘要链；ValidationApplications每页最多32个物理应用记录。

本文件需要守住的边界：

- SQL内JSON可能重排键，比较签名与摘要时使用保存的原封装或重新规范化的业务载荷。
- 缺失准备绑定、检查点或部分导入字段是损坏/冲突，不能当成新任务补一份。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/repository/rebuild' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/repository/rebuild/preparation_queries.go') -and (Get-Item -LiteralPath 'services/search/internal/repository/rebuild/preparation_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/repository/rebuild/preparation_queries.go')) { New-Item -ItemType File -Path 'services/search/internal/repository/rebuild/preparation_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:478 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	"bytes"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	events "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
)

// binding.go：保留原实现，按业务合并文件。

````

## 第 2 步：Binding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`type Binding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Prepare</code> | <code>rebuildmeta.Prepare</code> | 已验签并恢复的原准备业务载荷。 |
| <code>Envelope</code> | <code>&#91;&#93;byte</code> | 原准备签名封装，不能从数据库JSON重新拼接签名字节。 |
| <code>State</code> | <code>string</code> | 绑定阶段PREPARED/IMPORTED/ACCEPTED等。 |

<!-- write-step: 2 bytes=478:567 -->
````go
type Binding struct {
	Prepare  rebuildmeta.Prepare
	Envelope []byte
	State    string
}

````

## 第 3 步：Find

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

Find由已持有代次锁的用例调用。恢复核数据库原接受摘要，不按当前证书期限重新签名。

本步声明：`func Find(ctx context.Context, tx *sql.Tx, operation string) (Binding, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT prepare_envelope,prepare_digest,prepare_signer_digest,state,alias_name,source_index,target_index, 		CAST(source_generation AS CHAR),CAST(source_revi …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 原重建幂等操作号。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT prepare_envelope,prepare_digest,prepare_signer_digest,state,alias_name,source_index,target_index, 		CAST(source_generation AS CHAR),CAST(source_revi …</code>（长表达式，按本步完整代码继续）；<code>p.Operation != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Binding</code> | 实际返回表达式：<code>row</code>；<code>Binding{}</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;SEARCH_REBUILD_BINDING_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_BINDING_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT prepare_envelope,prepare_digest,prepare_signer_digest,state,alias_name,source_index,target_index, 		CAST(source_generation AS CHAR),CAST(source_revi …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return row, false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return row, false, errors.New(&quot;SEARCH_REBUILD_BINDING_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(digest) != 32 &#124;&#124; len(signer) != 32</code>。条件成立时执行 <code>return row, false, errors.New(&quot;SEARCH_REBUILD_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return row, false, errors.New(&quot;SEARCH_REBUILD_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; p.Operation != operation &#124;&#124; p.Alias != alias &#124;&#124; p.Source != source &#124;&#124; p.Target != target &#124;&#124; p.SourceGeneration != sourceGeneration &#124;&#124; p.SourceRevision != sourceRevisi …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return Binding{}, false, errors.New(&quot;SEARCH_REBUILD_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return row, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=567:2306 -->
````go
// Find由已持有代次锁的用例调用。恢复核数据库原接受摘要，不按当前证书期限重新签名。
func Find(ctx context.Context, tx *sql.Tx, operation string) (Binding, bool, error) {
	var row Binding
	var digest, signer []byte
	var alias, source, target, sourceGeneration, sourceRevision, targetGeneration string
	err := tx.QueryRowContext(ctx, `SELECT prepare_envelope,prepare_digest,prepare_signer_digest,state,alias_name,source_index,target_index,
		CAST(source_generation AS CHAR),CAST(source_revision AS CHAR),CAST(target_generation AS CHAR)
		FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation).
		Scan(&row.Envelope, &digest, &signer, &row.State, &alias, &source, &target, &sourceGeneration, &sourceRevision, &targetGeneration)
	if errors.Is(err, sql.ErrNoRows) {
		return row, false, nil
	}
	if err != nil {
		return row, false, errors.New("SEARCH_REBUILD_BINDING_READ_FAILED")
	}
	if len(digest) != 32 || len(signer) != 32 {
		return row, false, errors.New("SEARCH_REBUILD_BINDING_INVALID")
	}
	var expected, identity [32]byte
	copy(expected[:], digest)
	copy(identity[:], signer)
	accepted, err := rebuildartifact.RestoreAccepted(row.Envelope, rebuildartifact.PrepareV1, expected, identity)
	if err != nil {
		return row, false, errors.New("SEARCH_REBUILD_BINDING_INVALID")
	}
	row.Prepare, err = rebuildmeta.ParsePrepare(accepted.Payload)
	p := row.Prepare
	if err != nil || p.Operation != operation || p.Alias != alias || p.Source != source || p.Target != target || p.SourceGeneration != sourceGeneration || p.SourceRevision != sourceRevision || p.TargetGeneration != targetGeneration {
		return Binding{}, false, errors.New("SEARCH_REBUILD_BINDING_INVALID")
	}
	return row, true, nil
}

````

## 第 4 步：CheckAvailable

插入位置：第 3 步代码之后，紧接追加；完成稿约第 60 行。

本步声明：`func CheckAvailable(ctx context.Context, tx *sql.Tx, alias string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b LEFT JOIN rebuild_job j ON j.id=b.job_id 		WHERE b.alias_name=? AND (b.state IN (&#x27;PREPARED&#x27;,&#x27;IMPORTED&#x27;) OR 		( …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>alias</code> | <code>string</code> | 被重建的稳定搜索别名。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b LEFT JOIN rebuild_job j ON j.id=b.job_id 		WHERE b.alias_name=? AND (b.state IN (&#x27;PREPARED&#x27;,&#x27;IMPORTED&#x27;) OR 		( …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;SEARCH_REBUILD_BINDING_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_ALREADY_PENDING&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b LEFT JOIN rebuild_job j ON j.id=b.job_id 		WHERE b.alias_name=? AND (b.state IN (&#x27;PREPARED&#x27;,&#x27;IMPORTED&#x27;) OR 		( …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b LEFT JOIN rebuild_job j ON j.id=b.job_id 		WHERE b.alias_name=? AND (b.state IN (&#x27;PREPARED&#x27;,&#x27;IMPORTED&#x27;) OR 		( …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_BINDING_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;SEARCH_REBUILD_ALREADY_PENDING&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2306:2905 -->
````go
func CheckAvailable(ctx context.Context, tx *sql.Tx, alias string) error {
	var operation string
	err := tx.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b LEFT JOIN rebuild_job j ON j.id=b.job_id
		WHERE b.alias_name=? AND (b.state IN ('PREPARED','IMPORTED') OR
		(b.state='ACCEPTED' AND (j.id IS NULL OR j.state NOT IN ('SUCCEEDED','REJECTED')))) LIMIT 1 FOR UPDATE`, alias).Scan(&operation)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_REBUILD_BINDING_READ_FAILED")
	}
	return errors.New("SEARCH_REBUILD_ALREADY_PENDING")
}

````

## 第 5 步：Insert

插入位置：第 4 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`func Insert(ctx context.Context, tx *sql.Tx, p rebuildmeta.Prepare, envelope []byte, signer string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO rebuild_binding(operation_id,alias_name,source_index,source_generation,source_revision,target_index,target_generation, 		prepare_digest,prepare_sig …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>p</code> | <code>rebuildmeta.Prepare</code> | 使用 <code>rebuildmeta.Prepare</code> 类型；其定义处的职责：Prepare的整数采用字符串。原L与源/目标代次一起签名，不能靠两台机器的时间戳证明先后。<br>在此声明中的实际使用：<code>p.Bytes</code>；<code>p.Operation</code>；<code>p.Alias</code>；<code>p.Source</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>envelope</code> | <code>&#91;&#93;byte</code> | 原签名准备封装字节。<br>在此声明中的实际使用：<code>len(envelope)</code>；<code>sha256.Sum256(envelope)</code>；<code>tx.ExecContext(ctx, `INSERT INTO rebuild_binding(operation_id,alias_name,source_index,source_generation,source_revision,target_index,target_generation, 		prepare_digest,prepare_sig …</code>（长表达式，按本步完整代码继续） |
| <code>signer</code> | <code>string</code> | 已验证签发者摘要的十六进制字符串。<br>在此声明中的实际使用：<code>hex.DecodeString(signer)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_BINDING_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_BINDING_WRITE_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(identity) != 32 &#124;&#124; len(envelope) == 0 &#124;&#124; len(envelope) &gt; rebuildartifact.MaxEnvelopeBytes</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO rebuild_binding(operation_id,alias_name,source_index,source_generation,source_revision,target_index,target_generation, 		prepare_digest,prepare_sig …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_BINDING_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2905:3870 -->
````go
func Insert(ctx context.Context, tx *sql.Tx, p rebuildmeta.Prepare, envelope []byte, signer string) error {
	if _, err := p.Bytes(); err != nil {
		return err
	}
	identity, err := hex.DecodeString(signer)
	if err != nil || len(identity) != 32 || len(envelope) == 0 || len(envelope) > rebuildartifact.MaxEnvelopeBytes {
		return errors.New("SEARCH_REBUILD_BINDING_INVALID")
	}
	digest := sha256.Sum256(envelope)
	_, err = tx.ExecContext(ctx, `INSERT INTO rebuild_binding(operation_id,alias_name,source_index,source_generation,source_revision,target_index,target_generation,
		prepare_digest,prepare_signer_digest,prepare_envelope,state) VALUES(?,?,?,?,?,?,?,?,?,?,'PREPARED')`,
		p.Operation, p.Alias, p.Source, p.SourceGeneration, p.SourceRevision, p.Target, p.TargetGeneration, digest[:], identity, envelope)
	if err != nil {
		return errors.New("SEARCH_REBUILD_BINDING_WRITE_FAILED")
	}
	return nil
}

// frontier.go：保留原实现，按业务合并文件。

````

## 第 6 步：Frontier

插入位置：第 5 步代码之后，紧接追加；完成稿约第 94 行。

Frontier是切换前沿的不可变版本化载荷。offset使用十进制字符串，
避免JSON数字经过其他语言时损失精度；L仍只从原prepare读取。

本步声明：`type Frontier struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>int</code> | 前沿载荷格式版本，当前仅接受1。<br>序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>Ends</code> | <code>&#91;events.PartitionCount&#93;string</code> | 三个分区不可变终点H，字符串保留64位偏移量精度。<br>序列化/校验标记：<code>`json:&quot;ends&quot;`</code> |

<!-- write-step: 6 bytes=3870:4180 -->
````go
// Frontier是切换前沿的不可变版本化载荷。offset使用十进制字符串，
// 避免JSON数字经过其他语言时损失精度；L仍只从原prepare读取。
type Frontier struct {
	Version int                           `json:"version"`
	Ends    [events.PartitionCount]string `json:"ends"`
}

````

## 第 7 步：EncodeFrontier

插入位置：第 6 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`func EncodeFrontier(ends [events.PartitionCount]int64) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ends</code> | <code>&#91;events.PartitionCount&#93;int64</code> | 准备封存的三个分区终点H。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 由 <code>json.Marshal(f)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>json.Marshal(f)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_FRONTIER_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>end &lt; 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_FRONTIER_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return json.Marshal(f)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4180:4459 -->
````go
func EncodeFrontier(ends [events.PartitionCount]int64) ([]byte, error) {
	f := Frontier{Version: 1}
	for p, end := range ends {
		if end < 0 {
			return nil, errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
		}
		f.Ends[p] = strconv.FormatInt(end, 10)
	}
	return json.Marshal(f)
}

````

## 第 8 步：DecodeFrontier

插入位置：第 7 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`func DecodeFrontier(raw []byte) ([events.PartitionCount]int64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>json.Unmarshal(raw, &amp;f)</code>；<code>bytes.Equal(canonical, raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;events.PartitionCount&#93;int64</code> | 实际返回表达式：<code>ends</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_FRONTIER_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(raw, &amp;f) != nil &#124;&#124; f.Version != 1</code>。条件成立时执行 <code>return ends, errors.New(&quot;SEARCH_REBUILD_FRONTIER_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; end &lt; 0 &#124;&#124; strconv.FormatInt(end, 10) != value</code>。条件成立时执行 <code>return ends, errors.New(&quot;SEARCH_REBUILD_FRONTIER_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(canonical, raw)</code>。条件成立时执行 <code>return ends, errors.New(&quot;SEARCH_REBUILD_FRONTIER_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return ends, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4459:5094 -->
````go
func DecodeFrontier(raw []byte) ([events.PartitionCount]int64, error) {
	var ends [events.PartitionCount]int64
	var f Frontier
	if json.Unmarshal(raw, &f) != nil || f.Version != 1 {
		return ends, errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
	}
	for p, value := range f.Ends {
		end, err := strconv.ParseInt(value, 10, 64)
		if err != nil || end < 0 || strconv.FormatInt(end, 10) != value {
			return ends, errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
		}
		ends[p] = end
	}
	canonical, _ := EncodeFrontier(ends)
	if !bytes.Equal(canonical, raw) {
		return ends, errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
	}
	return ends, nil
}

````

## 第 9 步：SealFrontier

插入位置：第 8 步代码之后，紧接追加；完成稿约第 132 行。

SealFrontier由持有generation独占锁→原binding/job锁的业务事务调用。
H与SWITCHING必须一起提交；未知结果只能重读原载荷，不能重新捕获H覆盖它。

本步声明：`func SealFrontier(ctx context.Context, tx *sql.Tx, id uint64, ends [events.PartitionCount]int64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_progress SET frontier_payload=?,revision=revision+1 		WHERE job_id=? AND frontier_payload IS NULL AND validation_digest IS NULL AND revision&lt;18446744073709551615`, payload, id)</code>；<code>tx.ExecContext(ctx, `UPDATE rebuild_job SET state=&#x27;SWITCHING&#x27;,last_error=NULL WHERE id=? AND state=&#x27;REPLAYING&#x27;`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 重建job主键；ConsumerName以它隔离专用消费账本。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_progress SET frontier_payload=?,revision=revision+1 		WHERE job_id=? AND frontier_payload IS NULL AND validation_digest IS NULL AND revision&lt;18446744073709551615`, payload, id)</code>；<code>tx.ExecContext(ctx, `UPDATE rebuild_job SET state=&#x27;SWITCHING&#x27;,last_error=NULL WHERE id=? AND state=&#x27;REPLAYING&#x27;`, id)</code> |
| <code>ends</code> | <code>&#91;events.PartitionCount&#93;int64</code> | 准备封存的三个分区终点H。<br>在此声明中的实际使用：<code>EncodeFrontier(ends)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_FRONTIER_WRITE_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE rebuild_progress SET frontier_payload=?,revision=revision+1 		WHERE job_id=? AND frontier_payload IS NULL AND validation_digest IS NULL AND revision&lt;18446744073709551615`, payload, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_FRONTIER_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE rebuild_job SET state=&#x27;SWITCHING&#x27;,last_error=NULL WHERE id=? AND state=&#x27;REPLAYING&#x27;`, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_FRONTIER_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5094:6237 -->
````go
// SealFrontier由持有generation独占锁→原binding/job锁的业务事务调用。
// H与SWITCHING必须一起提交；未知结果只能重读原载荷，不能重新捕获H覆盖它。
func SealFrontier(ctx context.Context, tx *sql.Tx, id uint64, ends [events.PartitionCount]int64) error {
	payload, err := EncodeFrontier(ends)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE rebuild_progress SET frontier_payload=?,revision=revision+1
		WHERE job_id=? AND frontier_payload IS NULL AND validation_digest IS NULL AND revision<18446744073709551615`, payload, id)
	if err != nil {
		return errors.New("SEARCH_REBUILD_FRONTIER_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	result, err = tx.ExecContext(ctx, `UPDATE rebuild_job SET state='SWITCHING',last_error=NULL WHERE id=? AND state='REPLAYING'`, id)
	if err != nil {
		return errors.New("SEARCH_REBUILD_FRONTIER_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	return nil
}

````

## 第 10 步：LoadFrontier

插入位置：第 9 步代码之后，紧接追加；完成稿约第 157 行。

本步声明：`func LoadFrontier(ctx context.Context, tx *sql.Tx, id uint64) ([events.PartitionCount]int64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT frontier_payload FROM rebuild_progress WHERE job_id=? FOR UPDATE`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 重建job主键；ConsumerName以它隔离专用消费账本。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT frontier_payload FROM rebuild_progress WHERE job_id=? FOR UPDATE`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;events.PartitionCount&#93;int64</code> | 由 <code>DecodeFrontier(raw)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>&#91;events.PartitionCount&#93;int64{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>DecodeFrontier(raw)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_FRONTIER_READ_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &#91;events.PartitionCount&#93;int64{}, errors.New(&quot;SEARCH_REBUILD_FRONTIER_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT frontier_payload FROM rebuild_progress WHERE job_id=? FOR UPDATE`, id).Scan(&amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT frontier_payload FROM rebuild_progress WHERE job_id=? FOR UPDATE`, id)</code>。
- 此处通过 <code>return DecodeFrontier(raw)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6237:6676 -->
````go
func LoadFrontier(ctx context.Context, tx *sql.Tx, id uint64) ([events.PartitionCount]int64, error) {
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT frontier_payload FROM rebuild_progress WHERE job_id=? FOR UPDATE`, id).Scan(&raw); err != nil {
		return [events.PartitionCount]int64{}, errors.New("SEARCH_REBUILD_FRONTIER_READ_FAILED")
	}
	return DecodeFrontier(raw)
}

// jobs.go：保留原实现，按业务合并文件。

````

## 第 11 步：JobBinding

插入位置：第 10 步代码之后，紧接追加；完成稿约第 167 行。

JobBinding只承载Search私有库事实；调用者先取得generation及binding锁。

本步声明：`type JobBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 已接受StartRebuild请求的摘要。 |
| <code>Revision</code> | <code>uint64</code> | 绑定行修订号。 |
| <code>JobID</code> | <code>sql.NullString</code> | 可能尚未创建的job主键；Valid区分NULL与已有任务。 |

<!-- write-step: 11 bytes=6676:6863 -->
````go
// JobBinding只承载Search私有库事实；调用者先取得generation及binding锁。
type JobBinding struct {
	Fingerprint []byte
	Revision    uint64
	JobID       sql.NullString
}

````

## 第 12 步：LoadJobBinding

插入位置：第 11 步代码之后，紧接追加；完成稿约第 174 行。

本步声明：`func LoadJobBinding(ctx context.Context, tx *sql.Tx, operation string) (JobBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT request_fingerprint,revision,CAST(job_id AS CHAR) FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 原重建幂等操作号。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT request_fingerprint,revision,CAST(job_id AS CHAR) FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>JobBinding</code> | 使用 <code>JobBinding</code> 类型；其定义处的职责：JobBinding只承载Search私有库事实；调用者先取得generation及binding锁。<br>实际返回表达式：<code>row</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_BINDING_READ_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT request_fingerprint,revision,CAST(job_id AS CHAR) FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return row, errors.New(&quot;SEARCH_REBUILD_BINDING_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return row, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6863:7288 -->
````go
func LoadJobBinding(ctx context.Context, tx *sql.Tx, operation string) (JobBinding, error) {
	var row JobBinding
	err := tx.QueryRowContext(ctx, `SELECT request_fingerprint,revision,CAST(job_id AS CHAR) FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation).
		Scan(&row.Fingerprint, &row.Revision, &row.JobID)
	if err != nil {
		return row, errors.New("SEARCH_REBUILD_BINDING_READ_FAILED")
	}
	return row, nil
}

````

## 第 13 步：Job

插入位置：第 12 步代码之后，紧接追加；完成稿约第 184 行。

本步声明：`type Job struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 重建任务主键。 |
| <code>Generation</code> | <code>uint64</code> | 目标索引代次，应为原来源代次加1。 |
| <code>SnapshotAfter</code> | <code>uint64</code> | 快照播种已完成到的文档编号。 |
| <code>State</code> | <code>string</code> | SNAPSHOT/REPLAYING/SWITCHING/SUCCEEDED等处理阶段。 |
| <code>SnapshotRef</code> | <code>string</code> | 已导入Doc快照的内容地址引用。 |
| <code>Target</code> | <code>string</code> | 原准备的目标物理索引。 |
| <code>Source</code> | <code>string</code> | 原准备的来源物理索引。 |
| <code>Watermark</code> | <code>&#91;&#93;byte</code> | 重建描述符JSON，绑定Prepare/Snapshot及原日志窗口。 |

<!-- write-step: 13 bytes=7288:7438 -->
````go
type Job struct {
	ID, Generation, SnapshotAfter      uint64
	State, SnapshotRef, Target, Source string
	Watermark                          []byte
}

````

## 第 14 步：LoadJob

插入位置：第 13 步代码之后，紧接追加；完成稿约第 190 行。

本步声明：`func LoadJob(ctx context.Context, tx *sql.Tx, id uint64) (Job, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT j.id,j.state,j.snapshot_ref,j.watermark,j.target_index,j.source_index,j.generation,p.snapshot_after 		FROM rebuild_job j JOIN rebuild_progress p ON  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 重建job主键；ConsumerName以它隔离专用消费账本。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT j.id,j.state,j.snapshot_ref,j.watermark,j.target_index,j.source_index,j.generation,p.snapshot_after 		FROM rebuild_job j JOIN rebuild_progress p ON  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Job</code> | 实际返回表达式：<code>j</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_JOB_READ_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT j.id,j.state,j.snapshot_ref,j.watermark,j.target_index,j.source_index,j.generation,p.snapshot_after 		FROM rebuild_job j JOIN rebuild_progress p ON  …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return j, errors.New(&quot;SEARCH_REBUILD_JOB_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return j, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=7438:7958 -->
````go
func LoadJob(ctx context.Context, tx *sql.Tx, id uint64) (Job, error) {
	var j Job
	err := tx.QueryRowContext(ctx, `SELECT j.id,j.state,j.snapshot_ref,j.watermark,j.target_index,j.source_index,j.generation,p.snapshot_after
		FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id).
		Scan(&j.ID, &j.State, &j.SnapshotRef, &j.Watermark, &j.Target, &j.Source, &j.Generation, &j.SnapshotAfter)
	if err != nil {
		return j, errors.New("SEARCH_REBUILD_JOB_READ_FAILED")
	}
	return j, nil
}

````

## 第 15 步：ConsumerName

插入位置：第 14 步代码之后，紧接追加；完成稿约第 201 行。

ConsumerName以数据库任务主键隔离重放账本，最长28字节，满足003的40字节限制。

本步声明：`func ConsumerName(id uint64) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>id</code> | <code>uint64</code> | 重建job主键；ConsumerName以它隔离专用消费账本。<br>在此声明中的实际使用：<code>strconv.FormatUint(id, 10)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;rebuild-&quot; + strconv.FormatUint(id, 10)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;rebuild-&quot; + strconv.FormatUint(id, 10)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7958:8153 -->
````go
// ConsumerName以数据库任务主键隔离重放账本，最长28字节，满足003的40字节限制。
func ConsumerName(id uint64) string { return "rebuild-" + strconv.FormatUint(id, 10) }

````

## 第 16 步：AcceptJob

插入位置：第 15 步代码之后，紧接追加；完成稿约第 204 行。

AcceptJob的任务、三分区原L及请求绑定必须在同一个业务事务提交。
不创建Kafka group，也绝不借用普通消费者的SQL检查点。

本步声明：`func AcceptJob(ctx context.Context, tx *sql.Tx, operation string, fingerprint []byte, j Job, topic string, starts [events.PartitionCount]string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO rebuild_job(state,snapshot_ref,watermark,target_index,source_index,generation) VALUES(&#x27;SNAPSHOT&#x27;,?,?,?,?,?)`, j.SnapshotRef, j.Watermark, j.Target, j.Source, j.Generation)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT LAST_INSERT_ID()&quot;)</code>；<code>tx.ExecContext(ctx, `INSERT INTO rebuild_progress(job_id) VALUES(?)`, id)</code>；<code>tx.ExecContext(ctx, `INSERT INTO consumer_checkpoint(consumer_name,kafka_topic,partition_id,next_offset) VALUES(?,?,?,?)`, ConsumerName(id), topic, partition, start)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 原重建幂等操作号。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_binding SET state=&#x27;ACCEPTED&#x27;,request_fingerprint=?,job_id=?,revision=revision+1 WHERE operation_id=? AND state=&#x27;IMPORTED&#x27; AND revision&lt;184467440 …</code>（长表达式，按本步完整代码继续） |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_binding SET state=&#x27;ACCEPTED&#x27;,request_fingerprint=?,job_id=?,revision=revision+1 WHERE operation_id=? AND state=&#x27;IMPORTED&#x27; AND revision&lt;184467440 …</code>（长表达式，按本步完整代码继续） |
| <code>j</code> | <code>Job</code> | 在此声明中的实际使用：<code>j.SnapshotRef</code>；<code>j.Watermark</code>；<code>j.Target</code>；<code>j.Source</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>topic</code> | <code>string</code> | 原准备中固定的Kafka主题。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO consumer_checkpoint(consumer_name,kafka_topic,partition_id,next_offset) VALUES(?,?,?,?)`, ConsumerName(id), topic, partition, start)</code> |
| <code>starts</code> | <code>&#91;events.PartitionCount&#93;string</code> | 原准备固定的三个分区起点L，以十进制字符串传入。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_JOB_WRITE_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_BINDING_WRITE_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO rebuild_job(state,snapshot_ref,watermark,target_index,source_index,generation) VALUES(&#x27;SNAPSHOT&#x27;,?,?,?,?,?)`, j.SnapshotRef, j.Watermark, j.Target, j.Source, j.Generation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_REBUILD_JOB_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; id == 0</code>。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_REBUILD_JOB_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT LAST_INSERT_ID()&quot;).Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT LAST_INSERT_ID()&quot;)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_REBUILD_JOB_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO rebuild_progress(job_id) VALUES(?)`, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_REBUILD_JOB_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO consumer_checkpoint(consumer_name,kafka_topic,partition_id,next_offset) VALUES(?,?,?,?)`, ConsumerName(id), topic, partition, start)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, `UPDATE rebuild_binding SET state=&#x27;ACCEPTED&#x27;,request_fingerprint=?,job_id=?,revision=revision+1 WHERE operation_id=? AND state=&#x27;IMPORTED&#x27; AND revision&lt;184467440 …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_REBUILD_BINDING_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_REBUILD_BINDING_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return id, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=8153:10042 -->
````go
// AcceptJob的任务、三分区原L及请求绑定必须在同一个业务事务提交。
// 不创建Kafka group，也绝不借用普通消费者的SQL检查点。
func AcceptJob(ctx context.Context, tx *sql.Tx, operation string, fingerprint []byte, j Job, topic string, starts [events.PartitionCount]string) (uint64, error) {
	result, err := tx.ExecContext(ctx, `INSERT INTO rebuild_job(state,snapshot_ref,watermark,target_index,source_index,generation) VALUES('SNAPSHOT',?,?,?,?,?)`, j.SnapshotRef, j.Watermark, j.Target, j.Source, j.Generation)
	if err != nil {
		return 0, errors.New("SEARCH_REBUILD_JOB_WRITE_FAILED")
	}
	// LAST_INSERT_ID绑定当前事务连接，直接扫描uint64避免经过driver的int64接口。
	var id uint64
	if err := tx.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&id); err != nil || id == 0 {
		return 0, errors.New("SEARCH_REBUILD_JOB_WRITE_FAILED")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO rebuild_progress(job_id) VALUES(?)`, id); err != nil {
		return 0, errors.New("SEARCH_REBUILD_JOB_WRITE_FAILED")
	}
	for partition, start := range starts {
		if _, err = tx.ExecContext(ctx, `INSERT INTO consumer_checkpoint(consumer_name,kafka_topic,partition_id,next_offset) VALUES(?,?,?,?)`, ConsumerName(id), topic, partition, start); err != nil {
			return 0, errors.New("SEARCH_REBUILD_JOB_WRITE_FAILED")
		}
	}
	result, err = tx.ExecContext(ctx, `UPDATE rebuild_binding SET state='ACCEPTED',request_fingerprint=?,job_id=?,revision=revision+1 WHERE operation_id=? AND state='IMPORTED' AND revision<18446744073709551615`, fingerprint, id, operation)
	if err != nil {
		return 0, errors.New("SEARCH_REBUILD_BINDING_WRITE_FAILED")
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return 0, errors.New("SEARCH_REBUILD_BINDING_WRITE_FAILED")
	}
	return id, nil
}

// validation.go：保留原实现，按业务合并文件。

````

## 第 17 步：Validation

插入位置：第 16 步代码之后，紧接追加；完成稿约第 237 行。

本步声明：`type Validation struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>int</code> | 验证进度载荷格式版本。<br>序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>Phase</code> | <code>string</code> | 先SNAPSHOT覆盖检查，再APPLICATION全物理记录检查。<br>序列化/校验标记：<code>`json:&quot;phase&quot;`</code> |
| <code>Count</code> | <code>string</code> | 已检查物理记录数量，规范十进制字符串。<br>序列化/校验标记：<code>`json:&quot;count&quot;`</code> |
| <code>Digest</code> | <code>string</code> | 迄今物理记录内容形成的SHA-256摘要链。<br>序列化/校验标记：<code>`json:&quot;digest&quot;`</code> |
| <code>After</code> | <code>uint64</code> | 当前阶段分页文档游标，单独存SQL，不序列化进JSON。<br>序列化/校验标记：<code>`json:&quot;-&quot;`</code> |
| <code>Complete</code> | <code>bool</code> | 数据库已有最终验证摘要，表示验证已封存。<br>序列化/校验标记：<code>`json:&quot;-&quot;`</code> |

<!-- write-step: 17 bytes=10042:10257 -->
````go
type Validation struct {
	Version  int    `json:"version"`
	Phase    string `json:"phase"`
	Count    string `json:"count"`
	Digest   string `json:"digest"`
	After    uint64 `json:"-"`
	Complete bool   `json:"-"`
}

````

## 第 18 步：LoadValidation

插入位置：第 17 步代码之后，紧接追加；完成稿约第 246 行。

本步声明：`func LoadValidation(ctx context.Context, tx *sql.Tx, id uint64) (Validation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.validation_after,p.validation_digest FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 重建job主键；ConsumerName以它隔离专用消费账本。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.validation_after,p.validation_digest FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Validation</code> | 实际返回表达式：<code>v</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_VALIDATION_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_VALIDATION_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.validation_after,p.validation_digest FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id).Scan(&amp;raw, &amp;after, &amp;digest) != nil</code>。条件成立时执行 <code>return v, errors.New(&quot;SEARCH_REBUILD_VALIDATION_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.validation_after,p.validation_digest FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id).Scan(&amp;raw, &amp;after, &amp;digest)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.validation_after,p.validation_digest FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id)</code>。
- 检查 <code>len(raw) == 0 &#124;&#124; string(raw) == &quot;null&quot;</code>。
- 检查 <code>json.Unmarshal(raw, &amp;v) != nil</code>。条件成立时执行 <code>return v, errors.New(&quot;SEARCH_REBUILD_VALIDATION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>v.Version != 1 &#124;&#124; (v.Phase != &quot;SNAPSHOT&quot; &amp;&amp; v.Phase != &quot;APPLICATION&quot;) &#124;&#124; err != nil &#124;&#124; hashErr != nil &#124;&#124; len(hash) != 32</code>。条件成立时执行 <code>return v, errors.New(&quot;SEARCH_REBUILD_VALIDATION_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return v, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=10257:11291 -->
````go
func LoadValidation(ctx context.Context, tx *sql.Tx, id uint64) (Validation, error) {
	var v Validation
	var raw, digest []byte
	var after uint64
	if tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.validation_after,p.validation_digest FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id).Scan(&raw, &after, &digest) != nil {
		return v, errors.New("SEARCH_REBUILD_VALIDATION_READ_FAILED")
	}
	if len(raw) == 0 || string(raw) == "null" {
		v = Validation{Version: 1, Phase: "SNAPSHOT", Count: "0", Digest: hex.EncodeToString(make([]byte, 32))}
	} else if json.Unmarshal(raw, &v) != nil {
		return v, errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
	}
	_, err := strconv.ParseUint(v.Count, 10, 64)
	hash, hashErr := hex.DecodeString(v.Digest)
	if v.Version != 1 || (v.Phase != "SNAPSHOT" && v.Phase != "APPLICATION") || err != nil || hashErr != nil || len(hash) != 32 {
		return v, errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
	}
	v.After, v.Complete = after, len(digest) == 32
	return v, nil
}

````

## 第 19 步：SaveValidation

插入位置：第 18 步代码之后，紧接追加；完成稿约第 267 行。

SaveValidation与调用者持有的原job锁同事务提交分页进度；完成摘要只封存一次。

本步声明：`func SaveValidation(ctx context.Context, tx *sql.Tx, id uint64, v Validation, complete bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_progress SET validation_after=?,validation_digest=?,revision=revision+1 WHERE job_id=? AND frontier_payload IS NOT NULL AND validation_digest IS …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE rebuild_job SET cursor_json=? WHERE id=? AND state=&#x27;SWITCHING&#x27;`, raw, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 重建job主键；ConsumerName以它隔离专用消费账本。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_progress SET validation_after=?,validation_digest=?,revision=revision+1 WHERE job_id=? AND frontier_payload IS NOT NULL AND validation_digest IS …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE rebuild_job SET cursor_json=? WHERE id=? AND state=&#x27;SWITCHING&#x27;`, raw, id)</code> |
| <code>v</code> | <code>Validation</code> | 在此声明中的实际使用：<code>json.Marshal(v)</code>；<code>v.Digest</code>；<code>v.After</code> |
| <code>complete</code> | <code>bool</code> | 是否已完成全部验证，仅最后一页可封存摘要。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_VALIDATION_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_VALIDATION_WRITE_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_VALIDATION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>complete</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_VALIDATION_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE rebuild_progress SET validation_after=?,validation_digest=?,revision=revision+1 WHERE job_id=? AND frontier_payload IS NOT NULL AND validation_digest IS …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_VALIDATION_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>e != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_VALIDATION_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE rebuild_job SET cursor_json=? WHERE id=? AND state=&#x27;SWITCHING&#x27;`, raw, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=11291:12428 -->
````go
// SaveValidation与调用者持有的原job锁同事务提交分页进度；完成摘要只封存一次。
func SaveValidation(ctx context.Context, tx *sql.Tx, id uint64, v Validation, complete bool) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
	}
	var digest any
	if complete {
		digest, err = hex.DecodeString(v.Digest)
		if err != nil {
			return errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE rebuild_progress SET validation_after=?,validation_digest=?,revision=revision+1 WHERE job_id=? AND frontier_payload IS NOT NULL AND validation_digest IS NULL AND revision<18446744073709551615`, v.After, digest, id)
	if err != nil {
		return errors.New("SEARCH_REBUILD_VALIDATION_WRITE_FAILED")
	}
	if n, e := result.RowsAffected(); e != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE rebuild_job SET cursor_json=? WHERE id=? AND state='SWITCHING'`, raw, id); err != nil {
		return errors.New("SEARCH_REBUILD_VALIDATION_WRITE_FAILED")
	}
	return nil
}

````

## 第 20 步：Application

插入位置：第 19 步代码之后，紧接追加；完成稿约第 293 行。

本步声明：`type Application struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>uint64</code> | 物理索引应用记录对应的文档。 |
| <code>State</code> | <code>index.State</code> | 该文档已应用的权威序号、摘要、正式版和可见性。 |

<!-- write-step: 20 bytes=12428:12496 -->
````go
type Application struct {
	Document uint64
	State    index.State
}

````

## 第 21 步：ValidationApplications

插入位置：第 20 步代码之后，紧接追加；完成稿约第 298 行。

本步声明：`func ValidationApplications(ctx context.Context, tx *sql.Tx, physical string, after uint64) ([]Application, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT document_id,applied_seq,authority_digest,COALESCE(release_id,0),visibility,visibility_policy,revision FROM index_application WHERE physical_index=? AND …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>physical</code> | <code>string</code> | 本轮重建目标物理索引。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT document_id,applied_seq,authority_digest,COALESCE(release_id,0),visibility,visibility_policy,revision FROM index_application WHERE physical_index=? AND …</code>（长表达式，按本步完整代码继续） |
| <code>after</code> | <code>uint64</code> | 上页最后完成的文档编号，下一页严格大于它。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT document_id,applied_seq,authority_digest,COALESCE(release_id,0),visibility,visibility_policy,revision FROM index_application WHERE physical_index=? AND …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;Application</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_APPLICATION_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_APPLICATION_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT document_id,applied_seq,authority_digest,COALESCE(release_id,0),visibility,visibility_policy,revision FROM index_application WHERE physical_index=? AND …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_APPLICATION_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;a.Document, &amp;a.State.Sequence, &amp;digest, &amp;a.State.Release, &amp;visibility, &amp;a.State.Policy, &amp;a.State.Revision) != nil &#124;&#124; len(digest) != 32</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_APPLICATION_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;a.Document, &amp;a.State.Sequence, &amp;digest, &amp;a.State.Release, &amp;visibility, &amp;a.State.Policy, &amp;a.State.Revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>a.Document == 0 &#124;&#124; a.State.Revision == 0 &#124;&#124; (visibility != &quot;PUBLIC&quot; &amp;&amp; visibility != &quot;HIDDEN&quot;)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_APPLICATION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_APPLICATION_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=12496:13703 -->
````go
func ValidationApplications(ctx context.Context, tx *sql.Tx, physical string, after uint64) ([]Application, error) {
	rows, err := tx.QueryContext(ctx, `SELECT document_id,applied_seq,authority_digest,COALESCE(release_id,0),visibility,visibility_policy,revision FROM index_application WHERE physical_index=? AND document_id>? ORDER BY document_id LIMIT 32`, physical, after)
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_APPLICATION_READ_FAILED")
	}
	defer rows.Close()
	var result []Application
	for rows.Next() {
		var a Application
		var digest []byte
		var visibility string
		if rows.Scan(&a.Document, &a.State.Sequence, &digest, &a.State.Release, &visibility, &a.State.Policy, &a.State.Revision) != nil || len(digest) != 32 {
			return nil, errors.New("SEARCH_REBUILD_APPLICATION_INVALID")
		}
		copy(a.State.Digest[:], digest)
		a.State.Visible = visibility == "PUBLIC"
		if a.Document == 0 || a.State.Revision == 0 || (visibility != "PUBLIC" && visibility != "HIDDEN") {
			return nil, errors.New("SEARCH_REBUILD_APPLICATION_INVALID")
		}
		result = append(result, a)
	}
	if rows.Err() != nil {
		return nil, errors.New("SEARCH_REBUILD_APPLICATION_READ_FAILED")
	}
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/repository/rebuild/preparation_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/04-rebuild-snapshot.md)。
