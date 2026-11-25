# 分步编写 `services/document/internal/repository/document/publication.go`

先按 [D07 · 正式发布与当前受众](../lessons/document/07-publication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/166-publication.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存发布上下文、正式版本、发布指针与搜索outbox水位。

业务正式指针和唯一索引事件必须同事务推进，否则搜索会永久落后于已成功发布。

写之前先按这条执行路线理解各段的连接：

1. RegisterPublication固定动作
2. InsertRelease绑定manifest
3. PublicationEventSequence取得水位
4. SetPublication写release/epoch/sequence及事件
5. ClaimPublicationTask恢复捕获。

本文件需要守住的边界：

- 不能先发Kafka再提交
- Switch只能指向此文档实际历史release。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/publication.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/publication.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/publication.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/publication.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:123 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"time"
)

````

## 第 2 步：PublicationReceipt

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`type PublicationReceipt struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 原回执绑定的操作者账号。 |
| <code>Generation</code> | <code>uint64</code> | 回执代次，恢复提交时防止旧工作覆盖新进度。 |
| <code>Kind</code> | <code>string</code> | 回执业务种类，避免同号跨业务复用。 |
| <code>State</code> | <code>string</code> | 原操作回执状态，决定返回结果或等待恢复。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>Result</code> | <code>&#91;&#93;byte</code> | 已经保存的结果JSON，不重新执行以制造新结果。 |
| <code>Expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。 |

<!-- write-step: 2 bytes=123:302 -->
````go
type PublicationReceipt struct {
	Actor               uint64
	Generation          uint64
	Kind, State         string
	Fingerprint, Result []byte
	Expires             time.Time
}

````

## 第 3 步：PublicationResult

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func PublicationResult(ctx context.Context, q Queryer, operation string) (PublicationReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,generation,kind,state,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind IN (&#x27;RELEASE_CREATE&#x27;,&#x27;SWITCH&#x27;,&#x27;UNPUBLISH&#x27;)&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,generation,kind,state,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind IN (&#x27;RELEASE_CREATE&#x27;,&#x27;SWITCH&#x27;,&#x27;UNPUBLISH&#x27;)&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PublicationReceipt</code> | 实际返回表达式：<code>receipt</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,generation,kind,state,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind IN (&#x27;RELEASE_CREATE&#x27;,&#x27;SWITCH&#x27;,&#x27;UNPUBLISH&#x27;)&quot;, operation)</code>。
- 此处通过 <code>return receipt, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=302:816 -->
````go
func PublicationResult(ctx context.Context, q Queryer, operation string) (PublicationReceipt, error) {
	var receipt PublicationReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,generation,kind,state,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind IN ('RELEASE_CREATE','SWITCH','UNPUBLISH')", operation).
		Scan(&receipt.Actor, &receipt.Generation, &receipt.Kind, &receipt.State, &receipt.Fingerprint, &receipt.Result, &receipt.Expires)
	return receipt, err
}

````

## 第 4 步：RegisterPublication

插入位置：第 3 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func RegisterPublication(ctx context.Context, tx *sql.Tx, operation string, original []byte, capture bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;PUBLICATION_CONTEXT&quot;, operation, operation, original, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PUBLICATION_RECOVER&quot;, operation, operation, original, &quot;QUEUED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;PUBLICATION_CONTEXT&quot;, operation, operation, original, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PUBLICATION_RECOVER&quot;, operation, operation, original, &quot;QUEUED&quot;)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;PUBLICATION_CONTEXT&quot;, operation, operation, original, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PUBLICATION_RECOVER&quot;, operation, operation, original, &quot;QUEUED&quot;)</code> |
| <code>original</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;PUBLICATION_CONTEXT&quot;, operation, operation, original, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PUBLICATION_RECOVER&quot;, operation, operation, original, &quot;QUEUED&quot;)</code> |
| <code>capture</code> | <code>bool</code> | 是否需要新采样：true登记PUBLICATION_RECOVER待执行任务；无论取值都保存原PUBLICATION_CONTEXT。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PUBLICATION_RECOVER&quot;, operation, operation, original, &quot;QUEUED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>capture</code>。条件成立时执行 <code>return InsertSubmissionTask(ctx, tx, &quot;PUBLICATION_RECOVER&quot;, operation, operation, original, &quot;QUEUED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=816:1206 -->
````go
func RegisterPublication(ctx context.Context, tx *sql.Tx, operation string, original []byte, capture bool) error {
	if err := InsertSubmissionTask(ctx, tx, "PUBLICATION_CONTEXT", operation, operation, original, "SUCCEEDED"); err != nil {
		return err
	}
	if capture {
		return InsertSubmissionTask(ctx, tx, "PUBLICATION_RECOVER", operation, operation, original, "QUEUED")
	}
	return nil
}

````

## 第 5 步：ClaimPublicationTask

插入位置：第 4 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func ClaimPublicationTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;PUBLICATION_RECOVER&quot;, &quot;PUBLICATION_RECOVER&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;PUBLICATION_RECOVER&quot;, &quot;PUBLICATION_RECOVER&quot;)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;PUBLICATION_RECOVER&quot;, &quot;PUBLICATION_RECOVER&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;PUBLICATION_RECOVER&quot;, &quot;PUBLICATION_RECOVER&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;PUBLICATION_RECOVER&quot;, &quot;PUBLICATION_RECOVER&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return claimOperationTask(ctx, tx, instance, &quot;PUBLICATION_RECOVER&quot;, &quot;PUBLICATION_RECOVER&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1206:1410 -->
````go
func ClaimPublicationTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "PUBLICATION_RECOVER", "PUBLICATION_RECOVER")
}

````

## 第 6 步：InsertRelease

插入位置：第 5 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func InsertRelease(ctx context.Context, tx *sql.Tx, document, manifest, actor uint64, title, operation string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)&quot;, document, manifest, title, actor, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)&quot;, document, manifest, title, actor, operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)&quot;, document, manifest, title, actor, operation)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)&quot;, document, manifest, title, actor, operation)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)&quot;, document, manifest, title, actor, operation)</code> |
| <code>title</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)&quot;, document, manifest, title, actor, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)&quot;, document, manifest, title, actor, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)&quot;, document, manifest, title, actor, operation)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)&quot;, document, manifest, title, actor, operation)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)&quot;, document, manifest, title, actor, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1410:1713 -->
````go
func InsertRelease(ctx context.Context, tx *sql.Tx, document, manifest, actor uint64, title, operation string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)", document, manifest, title, actor, operation)
}

````

## 第 7 步：ReleaseManifest

插入位置：第 6 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`func ReleaseManifest(ctx context.Context, q Queryer, document, release uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT manifest_id FROM `release` WHERE document_id=? AND id=?&quot;, document, release)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT manifest_id FROM `release` WHERE document_id=? AND id=?&quot;, document, release)</code> |
| <code>release</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT manifest_id FROM `release` WHERE document_id=? AND id=?&quot;, document, release)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>manifest</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT manifest_id FROM `release` WHERE document_id=? AND id=?&quot;, document, release).Scan(&amp;manifest)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT manifest_id FROM `release` WHERE document_id=? AND id=?&quot;, document, release)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。
- 此处通过 <code>return manifest, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1713:2087 -->
````go
func ReleaseManifest(ctx context.Context, q Queryer, document, release uint64) (uint64, error) {
	var manifest uint64
	err := q.QueryRowContext(ctx, "SELECT manifest_id FROM `release` WHERE document_id=? AND id=?", document, release).Scan(&manifest)
	if err == sql.ErrNoRows {
		err = domain.Violation(domain.EntityNotFound, "RELEASE_NOT_FOUND")
	}
	return manifest, err
}

````

## 第 8 步：PublicationEventSequence

插入位置：第 7 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func PublicationEventSequence(ctx context.Context, tx *sql.Tx, document uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT event_seq FROM document WHERE id=? FOR UPDATE&quot;, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT event_seq FROM document WHERE id=? FOR UPDATE&quot;, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>sequence</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT event_seq FROM document WHERE id=? FOR UPDATE&quot;, document).Scan(&amp;sequence)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT event_seq FROM document WHERE id=? FOR UPDATE&quot;, document)</code>。
- 此处通过 <code>return sequence, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2087:2345 -->
````go
func PublicationEventSequence(ctx context.Context, tx *sql.Tx, document uint64) (uint64, error) {
	var sequence uint64
	err := tx.QueryRowContext(ctx, "SELECT event_seq FROM document WHERE id=? FOR UPDATE", document).Scan(&sequence)
	return sequence, err
}

````

## 第 9 步：SetPublication

插入位置：第 8 步代码之后，紧接追加；完成稿约第 58 行。

业务指针、单调水位和唯一outbox在调用方同一事务写入；这里不自行提交或发送Kafka。

本步声明：`func SetPublication(ctx context.Context, tx *sql.Tx, document, release, epoch, sequence uint64, operation, eventType string, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_release_id=?,publication_state=?,publication_epoch=?,event_seq=? WHERE id=?&quot;, current, state, epoch, sequence, document)</code>；<code>InsertIndexEvent(ctx, tx, document, sequence, operation, eventType, payload)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>InsertIndexEvent(ctx, tx, document, sequence, operation, eventType, payload)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_release_id=?,publication_state=?,publication_epoch=?,event_seq=? WHERE id=?&quot;, current, state, epoch, sequence, document)</code>；<code>InsertIndexEvent(ctx, tx, document, sequence, operation, eventType, payload)</code> |
| <code>release</code> | <code>uint64</code> | 在此声明中的实际使用：<code>release != 0</code>；<code>current, state = release, &quot;PUBLISHED&quot;</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_release_id=?,publication_state=?,publication_epoch=?,event_seq=? WHERE id=?&quot;, current, state, epoch, sequence, document)</code> |
| <code>sequence</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_release_id=?,publication_state=?,publication_epoch=?,event_seq=? WHERE id=?&quot;, current, state, epoch, sequence, document)</code>；<code>InsertIndexEvent(ctx, tx, document, sequence, operation, eventType, payload)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertIndexEvent(ctx, tx, document, sequence, operation, eventType, payload)</code> |
| <code>eventType</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertIndexEvent(ctx, tx, document, sequence, operation, eventType, payload)</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertIndexEvent(ctx, tx, document, sequence, operation, eventType, payload)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertIndexEvent(ctx, tx, document, sequence, operation, eventType, payload)</code> |

**跟着本段代码走一遍：**

- 检查 <code>release != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET current_release_id=?,publication_state=?,publication_epoch=?,event_seq=? WHERE id=?&quot;, current, state, epoch, sequence, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return InsertIndexEvent(ctx, tx, document, sequence, operation, eventType, payload)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2345:3016 -->
````go
// 业务指针、单调水位和唯一outbox在调用方同一事务写入；这里不自行提交或发送Kafka。
func SetPublication(ctx context.Context, tx *sql.Tx, document, release, epoch, sequence uint64, operation, eventType string, payload []byte) error {
	var current any
	state := "UNPUBLISHED"
	if release != 0 {
		current, state = release, "PUBLISHED"
	}
	if _, err := tx.ExecContext(ctx, "UPDATE document SET current_release_id=?,publication_state=?,publication_epoch=?,event_seq=? WHERE id=?", current, state, epoch, sequence, document); err != nil {
		return err
	}
	return InsertIndexEvent(ctx, tx, document, sequence, operation, eventType, payload)
}

````

## 第 10 步：PendingPublicationBinding

插入位置：第 9 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`type PendingPublicationBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 原业务操作号，恢复继续同一号。 |
| <code>State</code> | <code>string</code> | 与此操作关联的回执状态。 |
| <code>Kind</code> | <code>string</code> | 持久恢复任务种类，区分计算、安装与核验阶段。 |
| <code>Actor</code> | <code>uint64</code> | 原回执绑定的操作者。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 当前恢复任务的载荷，与原上下文互相核对。 |
| <code>Original</code> | <code>&#91;&#93;byte</code> | 原上下文任务中保存的载荷；不能由当前请求替换。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>Target</code> | <code>&#91;&#93;byte</code> | 原回执的目标JSON。 |

<!-- write-step: 10 bytes=3016:3200 -->
````go
type PendingPublicationBinding struct {
	Operation, State, Kind                 string
	Actor                                  uint64
	Payload, Original, Fingerprint, Target []byte
}

````

## 第 11 步：PendingPublicationBindings

插入位置：第 10 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`func PendingPublicationBindings(ctx context.Context, db *sql.DB) ([]PendingPublicationBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT t.operation_id,t.payload,original.payload, 		o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json 		FROM durable_task t 		LEFT JOIN durable_ta …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;PendingPublicationBinding</code> | 实际返回表达式：<code>nil</code>；<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryContext(ctx, `SELECT t.operation_id,t.payload,original.payload, 		o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json 		FROM durable_task t 		LEFT JOIN durable_ta …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;fact.Operation, &amp;fact.Payload, &amp;fact.Original, &amp;fact.Actor, &amp;fact.Kind, &amp;fact.State, &amp;fact.Fingerprint, &amp;fact.Target)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return facts, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3200:4215 -->
````go
func PendingPublicationBindings(ctx context.Context, db *sql.DB) ([]PendingPublicationBinding, error) {
	rows, err := db.QueryContext(ctx, `SELECT t.operation_id,t.payload,original.payload,
		o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json
		FROM durable_task t
		LEFT JOIN durable_task original ON original.kind='PUBLICATION_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id AND original.state='SUCCEEDED'
		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
		WHERE t.kind='PUBLICATION_RECOVER' AND t.state NOT IN ('SUCCEEDED','REJECTED') ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var facts []PendingPublicationBinding
	for rows.Next() {
		var fact PendingPublicationBinding
		if err = rows.Scan(&fact.Operation, &fact.Payload, &fact.Original, &fact.Actor, &fact.Kind, &fact.State, &fact.Fingerprint, &fact.Target); err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/publication.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/07-publication.md)。
