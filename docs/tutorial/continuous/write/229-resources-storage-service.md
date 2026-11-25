# 分步编写 `services/document/internal/logic/document/resources_storage_service.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/229-resources-storage-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

编排上传预留、流式写入、完成结算和未引用对象回收。

SQL状态与文件IO不能单一事务完成，使用同一资源跨进程锁覆盖IO到最终确认，未知结果按原号恢复。

写之前先按这条执行路线理解各段的连接：

1. PrepareUpload预留额度与暂存
2. HoldUpload取排他锁
3. WriteUpload验证发起人及权限
4. PrepareComplete核实际文件后返回事务闭包
5. RecoverUpload/RecoverReady分开处理未完成与共享对象GC。

本文件需要守住的边界：

- 准备完成不是READY
- 取消请求不等于写入结束
- 已READY对象不能套上传TTL直接删除。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/resources_storage_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/resources_storage_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/resources_storage_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/resources_storage_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:499 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/resource"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"strings"
	"time"
)

// resource_bytes.go：保留原实现，按业务合并文件。

````

## 第 2 步：Resources.HoldUpload

插入位置：第 1 步代码之后，紧接追加；完成稿约第 22 行。

HoldUpload在读取文件前取得进程间互斥，直到完成确认的事务和响应结束才释放。
清理也必须持有同一把锁；取消请求不等于写入结束，不能仅凭TTL回收配额。

本步声明：`func (s *Resources) HoldUpload(ctx context.Context, actor Actor, id uint64) (func(), error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.ownedUpload(ctx, actor, id)</code>；<code>s.files.Acquire(ctx, u.Key)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.ownedUpload(ctx, actor, id)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.ownedUpload(ctx, actor, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>func()</code> | 由 <code>s.files.Acquire(ctx, u.Key)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.files.Acquire(ctx, u.Key)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOURCE_STORAGE_UNAVAILABLE&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>s.files == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOURCE_STORAGE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.files.Acquire(ctx, u.Key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=499:1048 -->
````go
// HoldUpload在读取文件前取得进程间互斥，直到完成确认的事务和响应结束才释放。
// 清理也必须持有同一把锁；取消请求不等于写入结束，不能仅凭TTL回收配额。
func (s *Resources) HoldUpload(ctx context.Context, actor Actor, id uint64) (func(), error) {
	u, _, err := s.ownedUpload(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if s.files == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOURCE_STORAGE_UNAVAILABLE")
	}
	return s.files.Acquire(ctx, u.Key)
}

````

## 第 3 步：resourceDTO

插入位置：第 2 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func resourceDTO(u repository.Upload) dto.Resource`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>u</code> | <code>repository.Upload</code> | 使用 <code>repository.Upload</code> 类型；其定义处的职责：实现上传、引用、跨空间分配、正文高水位及配额结算的SQL规则。<br>在此声明中的实际使用：<code>u.Checksum</code>；<code>u.ID</code>；<code>u.State</code>；<code>u.Size</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Resource</code> | 实际返回表达式：<code>dto.Resource{Id: domain.TextID(u.ID), State: u.State, ByteSize: domain.TextID(u.Size), Checksum: &amp;digest, MediaType: u.MediaType, Revision: domain.TextID(u.Revision)}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return dto.Resource{Id: domain.TextID(u.ID), State: u.State, ByteSize: domain.TextID(u.Size), Checksum: &amp;digest, MediaType: u.MediaType, Revision: domain.TextID(u.Revision)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1048:1324 -->
````go
func resourceDTO(u repository.Upload) dto.Resource {
	digest := hex.EncodeToString(u.Checksum[:])
	return dto.Resource{Id: domain.TextID(u.ID), State: u.State, ByteSize: domain.TextID(u.Size), Checksum: &digest, MediaType: u.MediaType, Revision: domain.TextID(u.Revision)}
}

````

## 第 4 步：Resources.WriteUpload

插入位置：第 3 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`func (s *Resources) WriteUpload(ctx context.Context, actor Actor, id uint64, body io.Reader) (Actor, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.HoldUpload(ctx, actor, id)</code>；<code>s.ownedUpload(ctx, actor, id)</code>；<code>s.validateUpload(ctx, actor, access, u)</code>；<code>s.files.CleanupUpload(ctx, u.Key)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.HoldUpload(ctx, actor, id)</code>；<code>return actor, nil, err</code>；<code>s.ownedUpload(ctx, actor, id)</code>；<code>return actor, nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_CLOSED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.HoldUpload(ctx, actor, id)</code>；<code>s.ownedUpload(ctx, actor, id)</code> |
| <code>body</code> | <code>io.Reader</code> | 按需读取字节的输入流，不保证一次Read就拿到完整内容；调用者负责期限、体积限制和最终完整性核验。<br>在此声明中的实际使用：<code>s.files.Put(ctx, u.Key, body, u.Size, u.Checksum)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_CLOSED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 登记 <code>release()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>u.State != &quot;UPLOADING&quot; &#124;&#124; !u.Expires.After(time.Now())</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return resourceDTO(u), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1324:2724 -->
````go
func (s *Resources) WriteUpload(ctx context.Context, actor Actor, id uint64, body io.Reader) (Actor, ReadBody, error) {
	release, err := s.HoldUpload(ctx, actor, id)
	if err != nil {
		return actor, nil, err
	}
	defer release()
	// 等锁之前取得的状态不能用于继续写入；清理可能已经将原号关闭。
	u, access, err := s.ownedUpload(ctx, actor, id)
	if err != nil {
		return actor, nil, err
	}
	if u.State != "UPLOADING" || !u.Expires.After(time.Now()) {
		return actor, nil, domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_CLOSED")
	}
	if err := s.validateUpload(ctx, actor, access, u); err != nil {
		return actor, nil, err
	}
	// 持锁后可以清除前一进程遗留的半文件；预算在创建上传时已持久预留。
	if err := s.files.CleanupUpload(ctx, u.Key); err != nil {
		return actor, nil, err
	}
	if err := s.files.Put(ctx, u.Key, body, u.Size, u.Checksum); err != nil {
		return actor, nil, err
	}
	// 文件落盘不授予READY，且写入期间的撤权必须在确认响应前重新核验。
	_, access, err = s.ownedUpload(ctx, actor, id)
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if err := validateUploadInTransaction(ctx, tx, access, u); err != nil {
			return nil, err
		}
		return resourceDTO(u), nil
	}
	return actor, run, nil
}

````

## 第 5 步：Resources.validateUpload

插入位置：第 4 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func (s *Resources) validateUpload(ctx context.Context, actor Actor, access DocumentAccess, u repository.Upload) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.identity.Revalidate(ctx, actor.Proof)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, access.Sources)</code>；<code>validateUploadInTransaction(ctx, tx, access, u)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Proof</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Sources</code>；<code>validateUploadInTransaction(ctx, tx, access, u)</code> |
| <code>u</code> | <code>repository.Upload</code> | 使用 <code>repository.Upload</code> 类型；其定义处的职责：实现上传、引用、跨空间分配、正文高水位及配额结算的SQL规则。<br>在此声明中的实际使用：<code>validateUploadInTransaction(ctx, tx, access, u)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 5 bytes=2724:3212 -->
````go
func (s *Resources) validateUpload(ctx context.Context, actor Actor, access DocumentAccess, u repository.Upload) error {
	if err := s.access.identity.Revalidate(ctx, actor.Proof); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := LockSources(ctx, tx, access.Sources); err != nil {
		return err
	}
	if err := validateUploadInTransaction(ctx, tx, access, u); err != nil {
		return err
	}
	return tx.Commit()
}

````

## 第 6 步：validateUploadInTransaction

插入位置：第 5 步代码之后，紧接追加；完成稿约第 97 行。

本步声明：`func validateUploadInTransaction(ctx context.Context, tx *sql.Tx, access DocumentAccess, u repository.Upload) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.UploadRow(ctx, tx, u.ID, false)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.UploadRow(ctx, tx, u.ID, false)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code> |
| <code>u</code> | <code>repository.Upload</code> | 使用 <code>repository.Upload</code> 类型；其定义处的职责：实现上传、引用、跨空间分配、正文高水位及配额结算的SQL规则。<br>在此声明中的实际使用：<code>u.ID</code>；<code>u.Revision</code>；<code>u.Key</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_CLOSED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>doc.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.State != &quot;UPLOADING&quot; &#124;&#124; current.Revision != u.Revision &#124;&#124; current.Key != u.Key &#124;&#124; !current.Expires.After(time.Now())</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_CLOSED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3212:3899 -->
````go
func validateUploadInTransaction(ctx context.Context, tx *sql.Tx, access DocumentAccess, u repository.Upload) error {
	doc, err := ValidateAccess(ctx, tx, access)
	if err != nil {
		return err
	}
	if doc.Lifecycle != "ACTIVE" {
		return domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
	}
	current, err := repository.UploadRow(ctx, tx, u.ID, false)
	if err != nil {
		return err
	}
	if current.State != "UPLOADING" || current.Revision != u.Revision || current.Key != u.Key || !current.Expires.After(time.Now()) {
		return domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_CLOSED")
	}
	return nil
}

// resource_cleanup.go：保留原实现，按业务合并文件。

````

## 第 7 步：Resources.RecoverUpload

插入位置：第 6 步代码之后，紧接追加；完成稿约第 117 行。

RecoverUpload只处理未完成上传。READY的共享引用回收另走引用/保护核验，
不能把上传过期条件套用于已被草稿、版本或发布引用的资源。

本步声明：`func (s *Resources) RecoverUpload(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.CleanupCandidates(ctx, s.db, 32)</code>；<code>repository.UploadRow(ctx, s.db, id, false)</code>；<code>s.files.Acquire(ctx, u.Key)</code>；<code>s.cleanupUpload(ctx, u)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; violation.Code == &quot;RESOURCE_UPLOAD_BUSY&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3899:4647 -->
````go
// RecoverUpload只处理未完成上传。READY的共享引用回收另走引用/保护核验，
// 不能把上传过期条件套用于已被草稿、版本或发布引用的资源。
func (s *Resources) RecoverUpload(ctx context.Context) error {
	ids, err := repository.CleanupCandidates(ctx, s.db, 32)
	if err != nil {
		return err
	}
	for _, id := range ids {
		u, err := repository.UploadRow(ctx, s.db, id, false)
		if err != nil {
			return err
		}
		release, err := s.files.Acquire(ctx, u.Key)
		if err != nil {
			var violation domain.RuleViolation
			if errors.As(err, &violation) && violation.Code == "RESOURCE_UPLOAD_BUSY" {
				continue
			}
			return err
		}
		err = s.cleanupUpload(ctx, u)
		release()
		return err
	}
	return nil
}

````

## 第 8 步：Resources.cleanupUpload

插入位置：第 7 步代码之后，紧接追加；完成稿约第 144 行。

本步声明：`func (s *Resources) cleanupUpload(ctx context.Context, expected repository.Upload) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockQuota(ctx, tx, expected.Space, s.limits.SpaceBytes)</code>；<code>repository.UploadRow(ctx, tx, expected.ID, true)</code>；<code>repository.MarkDeleting(ctx, tx, u)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>expected</code> | <code>repository.Upload</code> | 使用 <code>repository.Upload</code> 类型；其定义处的职责：实现上传、引用、跨空间分配、正文高水位及配额结算的SQL规则。<br>在此声明中的实际使用：<code>expected.Space</code>；<code>expected.ID</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>u.State == &quot;READY&quot; &#124;&#124; u.State == &quot;DELETED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>u.State == &quot;UPLOADING&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>u.State != &quot;DELETING&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=4647:5914 -->
````go
func (s *Resources) cleanupUpload(ctx context.Context, expected repository.Upload) error {
	// 先持久关闭上传，再删文件；此提交未知时停止，不凭客户端错误推断回滚。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := repository.LockQuota(ctx, tx, expected.Space, s.limits.SpaceBytes); err != nil {
		return err
	}
	u, err := repository.UploadRow(ctx, tx, expected.ID, true)
	if err != nil {
		return err
	}
	if u.State == "READY" || u.State == "DELETED" {
		return nil
	}
	if u.State == "UPLOADING" {
		if err := repository.MarkDeleting(ctx, tx, u); err != nil {
			return err
		}
	} else if u.State != "DELETING" {
		return nil
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// guard覆盖整个IO与结算，进程退出由OS释放；原号恢复从DELETING继续。
	if err := s.files.CleanupUpload(ctx, u.Key); err != nil {
		return err
	}
	if err := s.files.Delete(ctx, u.Key); err != nil {
		return err
	}
	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q, err := repository.LockQuota(ctx, tx, u.Space, s.limits.SpaceBytes)
	if err != nil {
		return err
	}
	u, err = repository.UploadRow(ctx, tx, u.ID, true)
	if err != nil {
````

## 第 9 步：Resources.cleanupUpload（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 189 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>u.State == &quot;DELETED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 9 bytes=5914:6324 -->
````go
		return err
	}
	if u.State == "DELETED" {
		return nil
	}
	if err := repository.MarkDeleted(ctx, tx, u); err != nil {
		return err
	}
	if err := repository.SettleQuota(ctx, tx, q, u.Operation, false); err != nil {
		return err
	}
	if err := repository.ReleaseTemporary(ctx, tx, u.Size); err != nil {
		return err
	}
	return tx.Commit()
}

// resource_complete.go：保留原实现，按业务合并文件。

````

## 第 10 步：Resources.ownedUpload

插入位置：第 9 步代码之后，紧接追加；完成稿约第 208 行。

ownedUpload要求当前操作者既是上传发起人，也仍拥有文档准入来源。
仅知道资源id不能替他人完成上传；返回的来源还需在最终事务中重检。

本步声明：`func (s *Resources) ownedUpload(ctx context.Context, actor Actor, id uint64) (repository.Upload, DocumentAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.UploadRow(ctx, s.db, id, false)</code>；<code>s.uploadAccess(ctx, actor, u.Document)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.uploadAccess(ctx, actor, u.Document)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.UploadRow(ctx, s.db, id, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.Upload</code> | 使用 <code>repository.Upload</code> 类型；其定义处的职责：实现上传、引用、跨空间分配、正文高水位及配额结算的SQL规则。<br>实际返回表达式：<code>u</code> |
| <code>(匿名/未命名)</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>实际返回表达式：<code>DocumentAccess{}</code>；<code>a</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;RESOURCE_UPLOAD_NOT_OWNED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return u, DocumentAccess{}, err</code>。当前函数在这里结束。
- 检查 <code>u.Uploader != actor.ID</code>。条件成立时执行 <code>return u, DocumentAccess{}, domain.Violation(domain.PermissionDenied, &quot;RESOURCE_UPLOAD_NOT_OWNED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return u, a, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6324:7050 -->
````go
// ownedUpload要求当前操作者既是上传发起人，也仍拥有文档准入来源。
// 仅知道资源id不能替他人完成上传；返回的来源还需在最终事务中重检。
func (s *Resources) ownedUpload(ctx context.Context, actor Actor, id uint64) (repository.Upload, DocumentAccess, error) {
	u, err := repository.UploadRow(ctx, s.db, id, false)
	if err == sql.ErrNoRows {
		err = domain.Violation(domain.EntityNotFound, "RESOURCE_NOT_FOUND")
	}
	if err != nil {
		return u, DocumentAccess{}, err
	}
	if u.Uploader != actor.ID {
		return u, DocumentAccess{}, domain.Violation(domain.PermissionDenied, "RESOURCE_UPLOAD_NOT_OWNED")
	}
	a, err := s.uploadAccess(ctx, actor, u.Document)
	return u, a, err
}

````

## 第 11 步：Resources.PrepareComplete

插入位置：第 10 步代码之后，紧接追加；完成稿约第 225 行。

PrepareComplete先在锁外验证真实文件，再返回交给统一Mutation执行的事务体与结果呈现函数。
调用方不能把“准备成功”当作READY：容量结算、状态变更和原operation回执必须一起落定。

本步声明：`func (s *Resources) PrepareComplete(ctx context.Context, actor Actor, id uint64, input dto.UploadComplete) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.ownedUpload(ctx, actor, id)</code>；<code>s.files.Open(ctx, u.Key, u.Size, u.Checksum)</code>；<code>s.files.CleanupUpload(ctx, u.Key)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.ownedUpload(ctx, actor, id)</code>；<code>return actor, nil, nil, err</code>；<code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_CHECKSUM_MISMATCH&quot;)</code>；<code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_CLOSED&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.ownedUpload(ctx, actor, id)</code>；<code>repository.UploadRow(ctx, tx, id, true)</code>；<code>repository.RecordCommittedAllocation(ctx, tx, id)</code>；<code>OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.UploadComplete</code> | 在此声明中的实际使用：<code>input.ByteSize</code>；<code>input.Checksum</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOURCE_CHECKSUM_MISMATCH&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_CLOSED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOURCE_STORAGE_UNAVAILABLE&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>input.ByteSize != domain.TextID(u.Size) &#124;&#124; input.Checksum != hex.EncodeToString(u.Checksum&#91;:&#93;)</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_CHECKSUM_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>(u.State != &quot;UPLOADING&quot; &amp;&amp; u.State != &quot;READY&quot;) &#124;&#124; (u.State == &quot;UPLOADING&quot; &amp;&amp; !u.Expires.After(time.Now()))</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>s.files == nil</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOURCE_STORAGE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOURCE_STORAGE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>document.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 11 bytes=7050:9171 -->
````go
// PrepareComplete先在锁外验证真实文件，再返回交给统一Mutation执行的事务体与结果呈现函数。
// 调用方不能把“准备成功”当作READY：容量结算、状态变更和原operation回执必须一起落定。
func (s *Resources) PrepareComplete(ctx context.Context, actor Actor, id uint64, input dto.UploadComplete) (Actor, MutationBody, MutationPresentation, error) {
	u, access, err := s.ownedUpload(ctx, actor, id)
	if err != nil {
		return actor, nil, nil, err
	}
	if input.ByteSize != domain.TextID(u.Size) || input.Checksum != hex.EncodeToString(u.Checksum[:]) {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "RESOURCE_CHECKSUM_MISMATCH")
	}
	if (u.State != "UPLOADING" && u.State != "READY") || (u.State == "UPLOADING" && !u.Expires.After(time.Now())) {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_CLOSED")
	}
	if s.files == nil {
		return actor, nil, nil, domain.Violation(domain.TemporarilyUnavailable, "RESOURCE_STORAGE_UNAVAILABLE")
	}
	// 完整文件读取在SQL锁外。存储key不可覆盖，最终事务复核同一key/摘要/代次；
	// 真实文件不存在、只写半份或摘要不符时，绝不结算READY和used容量。
	file, err := s.files.Open(ctx, u.Key, u.Size, u.Checksum)
	if err != nil {
		return actor, nil, nil, err
	}
	if err := file.Close(); err != nil {
		return actor, nil, nil, domain.Violation(domain.TemporarilyUnavailable, "RESOURCE_STORAGE_UNAVAILABLE")
	}
	if err := s.files.CleanupUpload(ctx, u.Key); err != nil {
		return actor, nil, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if document.Lifecycle != "ACTIVE" {
			return nil, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
		}
		quota, err := repository.LockQuota(ctx, tx, u.Space, s.limits.SpaceBytes)
		if err != nil {
			return nil, err
		}
		current, err := repository.UploadRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
````

## 第 12 步：Resources.PrepareComplete（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 270 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>current.Key != u.Key &#124;&#124; current.Checksum != u.Checksum &#124;&#124; current.Size != u.Size &#124;&#124; current.Document != u.Document &#124;&#124; current.Uploader != actor.ID</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.State == &quot;READY&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>current.State != &quot;UPLOADING&quot; &#124;&#124; current.Revision != u.Revision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>document.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.State != &quot;READY&quot;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return dto.Resource{Id: domain.TextID(id), State: current.State, ByteSize: domain.TextID(current.Size), Checksum: &amp;digest, MediaType: current.MediaType, Revision: domain.TextID(current.Revision)}, access.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=9171:11267 -->
````go
		if current.Key != u.Key || current.Checksum != u.Checksum || current.Size != u.Size || current.Document != u.Document || current.Uploader != actor.ID {
			return nil, domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_CHANGED")
		}
		if current.State == "READY" {
			// 完成请求可以重放，但已结算的容量不能再次扣除；只补记同一资源的提交事实。
			if err = repository.RecordCommittedAllocation(ctx, tx, id); err != nil {
				return nil, err
			}
			return OperationResult(input.OperationId, "SUCCEEDED", id), nil
		}
		if current.State != "UPLOADING" || current.Revision != u.Revision {
			return nil, domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_CHANGED")
		}
		if err := repository.SettleQuota(ctx, tx, quota, u.Operation, true); err != nil {
			return nil, err
		}
		if err := repository.ReleaseTemporary(ctx, tx, u.Size); err != nil {
			return nil, err
		}
		if err := repository.MarkReady(ctx, tx, current); err != nil {
			return nil, err
		}
		if err = repository.RecordCommittedAllocation(ctx, tx, id); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		// 原operation已成功仍不代表当前请求有权看结果；呈现前重新核验文档与来源。
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, nil, err
		}
		if document.Lifecycle != "ACTIVE" {
			return nil, nil, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
		}
		current, err := repository.UploadRow(ctx, tx, id, false)
		if err != nil {
			return nil, nil, err
		}
		if current.State != "READY" {
			return nil, nil, domain.Violation(domain.StateConflict, "RESOURCE_NOT_READY")
		}
		digest := hex.EncodeToString(current.Checksum[:])
		return dto.Resource{Id: domain.TextID(id), State: current.State, ByteSize: domain.TextID(current.Size), Checksum: &digest, MediaType: current.MediaType, Revision: domain.TextID(current.Revision)}, access.Sources, nil
	}
````

## 第 13 步：Resources.PrepareComplete（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 316 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=11267:11378 -->
````go
	return actor, run, present, nil
}

// resource_ready_cleanup.go：保留原实现，按业务合并文件。

````

## 第 14 步：Resources.RecoverReady

插入位置：第 13 步代码之后，紧接追加；完成稿约第 321 行。

RecoverReady回收已完成但不再被引用的对象。候选查询不提供删除许可；
真正关闭新增引用的边界是资源行锁下提交READY→DELETING。

本步声明：`func (s *Resources) RecoverReady(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ReadyCleanupCandidates(ctx, s.db, 32)</code>；<code>repository.UploadRow(ctx, s.db, id, false)</code>；<code>s.files.Acquire(ctx, u.Key)</code>；<code>s.cleanupReady(ctx, u)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; violation.Code == &quot;RESOURCE_UPLOAD_BUSY&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=11378:12143 -->
````go
// RecoverReady回收已完成但不再被引用的对象。候选查询不提供删除许可；
// 真正关闭新增引用的边界是资源行锁下提交READY→DELETING。
func (s *Resources) RecoverReady(ctx context.Context) error {
	ids, err := repository.ReadyCleanupCandidates(ctx, s.db, 32)
	if err != nil {
		return err
	}
	for _, id := range ids {
		u, err := repository.UploadRow(ctx, s.db, id, false)
		if err != nil {
			return err
		}
		release, err := s.files.Acquire(ctx, u.Key)
		if err != nil {
			var violation domain.RuleViolation
			if errors.As(err, &violation) && violation.Code == "RESOURCE_UPLOAD_BUSY" {
				continue
			}
			return err
		}
		err = s.cleanupReady(ctx, u)
		release()
		if err != nil {
			return err
		}
	}
	return nil
}

````

## 第 15 步：Resources.cleanupReady

插入位置：第 14 步代码之后，紧接追加；完成稿约第 350 行。

本步声明：`func (s *Resources) cleanupReady(ctx context.Context, expected repository.Upload) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockResourceAllocationQuotas(ctx, tx, expected.ID, expected.Space, s.limits.SpaceBytes)</code>；<code>repository.UploadRow(ctx, tx, expected.ID, true)</code>；<code>repository.RecordCommittedAllocation(ctx, tx, u.ID)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |
| <code>expected</code> | <code>repository.Upload</code> | 使用 <code>repository.Upload</code> 类型；其定义处的职责：实现上传、引用、跨空间分配、正文高水位及配额结算的SQL规则。<br>在此声明中的实际使用：<code>expected.ID</code>；<code>expected.Space</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!committed</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>u.State == &quot;READY&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!changed</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>u.State != &quot;DELETING&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=12143:13322 -->
````go
func (s *Resources) cleanupReady(ctx context.Context, expected repository.Upload) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	quotas, err := repository.LockResourceAllocationQuotas(ctx, tx, expected.ID, expected.Space, s.limits.SpaceBytes)
	if err != nil {
		return err
	}
	u, err := repository.UploadRow(ctx, tx, expected.ID, true)
	if err != nil {
		return err
	}
	if err = repository.RecordCommittedAllocation(ctx, tx, u.ID); err != nil {
		return err
	}
	if err = repository.ValidateResourceAllocationQuotas(ctx, tx, u.ID, quotas); err != nil {
		return err
	}
	committed, err := repository.ReadyCleanupCommitted(ctx, tx, u)
	if err != nil {
		return err
	}
	if !committed {
		return nil
	}
	if u.State == "READY" {
		changed, err := repository.MarkReadyDeleting(ctx, tx, u)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
	} else if u.State != "DELETING" {
		return nil
	}
	// 提交结果未知时不执行外部删除，后续根据持久DELETING继续同一对象。
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := s.files.CleanupUpload(ctx, u.Key); err != nil {
		return err
	}
````

## 第 16 步：Resources.cleanupReady（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 395 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>u.State == &quot;DELETED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 16 bytes=13322:14218 -->
````go
	if err := s.files.Delete(ctx, u.Key); err != nil {
		return err
	}
	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	quotas, err = repository.LockResourceAllocationQuotas(ctx, tx, u.ID, u.Space, s.limits.SpaceBytes)
	if err != nil {
		return err
	}
	u, err = repository.UploadRow(ctx, tx, u.ID, true)
	if err != nil {
		return err
	}
	if u.State == "DELETED" {
		return nil
	}
	if err = repository.ValidateResourceAllocationQuotas(ctx, tx, u.ID, quotas); err != nil {
		return err
	}
	// DELETED和used扣减同事务。文件删除幂等，提交丢回复也不会再次释放容量。
	if err := repository.MarkDeleted(ctx, tx, u); err != nil {
		return err
	}
	if err := repository.ReleaseResourceAllocations(ctx, tx, u.ID, quotas); err != nil {
		return err
	}
	return tx.Commit()
}

// resource_upload.go：保留原实现，按业务合并文件。

````

## 第 17 步：ResourceLimits

插入位置：第 16 步代码之后，紧接追加；完成稿约第 429 行。

本步声明：`type ResourceLimits struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>SpaceBytes</code> | <code>uint64</code> | 每个空间容量上限（字节）。 |
| <code>UploadBytes</code> | <code>uint64</code> | 单次上传最大字节数。 |
| <code>TemporaryBytes</code> | <code>uint64</code> | 全部上传临时存储的受控预算（字节）。 |
| <code>UploadTTL</code> | <code>time.Duration</code> | 一段持续时间，按 time.Second 等单位运算；与表示某个瞬间的 time.Time 区别开。 |

<!-- write-step: 17 bytes=14218:14353 -->
````go
type ResourceLimits struct {
	SpaceBytes, UploadBytes, TemporaryBytes uint64
	UploadTTL                               time.Duration
}

````

## 第 18 步：Resources

插入位置：第 17 步代码之后，紧接追加；完成稿约第 434 行。

本步声明：`type Resources struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>limits</code> | <code>ResourceLimits</code> | 空间、单次上传、临时容量及上传期限的统一限制；SQL预留与流式写入共同执行。 |
| <code>origin</code> | <code>string</code> | 用户站点的固定源地址，用于构建同源编辑/资源/邀请入口；不能信任任意客户端Host。 |
| <code>files</code> | <code>ResourceFiles</code> | 使用 <code>ResourceFiles</code> 类型；其定义处的职责：统一的资源存储接口，保持本地和OSS都具备锁定、完整性核验、释放与删除语义。 |

<!-- write-step: 18 bytes=14353:14472 -->
````go
type Resources struct {
	db     *sql.DB
	access *Access
	limits ResourceLimits
	origin string
	files  ResourceFiles
}

````

## 第 19 步：ResourceFiles

插入位置：第 18 步代码之后，紧接追加；完成稿约第 442 行。

本步声明：`type ResourceFiles interface {`。

**这一段为什么这样写：** 统一的资源存储接口，保持本地和OSS都具备锁定、完整性核验、释放与删除语义。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Acquire</code> | <code>(context.Context, string) (func(), error)</code> | ctx和对象键获取写/清理互斥保护，返回释放函数及错误；使用完成必须释放。 |
| <code>AcquireRead</code> | <code>(context.Context, string) (func(), error)</code> | ctx和对象键获取读取保护，返回释放函数，防止清理与下载相撞。 |
| <code>CleanupUpload</code> | <code>(context.Context, string) error</code> | ctx和原对象键回收上传临时内容，只处理该键的上传产物。 |
| <code>Put</code> | <code>(context.Context, string, io.Reader, uint64, &#91;32&#93;byte) error</code> | ctx、对象键、输入流、预期字节数、预期SHA-256依次输入，写入过程中核验大小/摘要，返回错误。 |
| <code>Open</code> | <code>(context.Context, string, uint64, &#91;32&#93;byte) (io.ReadCloser, error)</code> | ctx、对象键、预期大小、预期摘要输入，返回需关闭的读流与错误；不能按任意路径读取。 |
| <code>Delete</code> | <code>(context.Context, string) error</code> | ctx与受控对象键输入，删除该对象；权限及业务引用关系由上层先核验。 |

<!-- write-step: 19 bytes=14472:14832 -->
````go
type ResourceFiles interface {
	Acquire(context.Context, string) (func(), error)
	AcquireRead(context.Context, string) (func(), error)
	CleanupUpload(context.Context, string) error
	Put(context.Context, string, io.Reader, uint64, [32]byte) error
	Open(context.Context, string, uint64, [32]byte) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

````

## 第 20 步：NewResources

插入位置：第 19 步代码之后，紧接追加；完成稿约第 451 行。

本步声明：`func NewResources(db *sql.DB, access *Access, limits ResourceLimits, origin string, files ResourceFiles) *Resources`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>limits</code> | <code>ResourceLimits</code> | 在此声明中的实际使用：<code>limits: limits</code> |
| <code>origin</code> | <code>string</code> | 在此声明中的实际使用：<code>origin: strings.TrimRight(origin, &quot;/&quot;)</code>；<code>strings.TrimRight(origin, &quot;/&quot;)</code> |
| <code>files</code> | <code>ResourceFiles</code> | 使用 <code>ResourceFiles</code> 类型；其定义处的职责：统一的资源存储接口，保持本地和OSS都具备锁定、完整性核验、释放与删除语义。<br>在此声明中的实际使用：<code>files: files</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Resources</code> | 实际返回表达式：<code>&amp;Resources{db: db, access: access, limits: limits, origin: strings.TrimRight(origin, &quot;/&quot;), files: files}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Resources{db: db, access: access, limits: limits, origin: strings.TrimRight(origin, &quot;/&quot;), files: files}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=14832:15066 -->
````go
func NewResources(db *sql.DB, access *Access, limits ResourceLimits, origin string, files ResourceFiles) *Resources {
	return &Resources{db: db, access: access, limits: limits, origin: strings.TrimRight(origin, "/"), files: files}
}

````

## 第 21 步：Resources.SpaceQuotaLimit

插入位置：第 20 步代码之后，紧接追加；完成稿约第 455 行。

本步声明：`func (s *Resources) SpaceQuotaLimit() uint64`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>s.limits.SpaceBytes</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.limits.SpaceBytes</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=15066:15143 -->
````go
func (s *Resources) SpaceQuotaLimit() uint64 { return s.limits.SpaceBytes }

````

## 第 22 步：Resources.uploadAccess

插入位置：第 21 步代码之后，紧接追加；完成稿约第 457 行。

本步声明：`func (s *Resources) uploadAccess(ctx context.Context, actor Actor, document uint64) (DocumentAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;RESOURCE_UPLOAD&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;RESOURCE_UPLOAD&quot;)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;RESOURCE_UPLOAD&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>实际返回表达式：<code>a</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;RESOURCE_EDIT_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return a, err</code>。当前函数在这里结束。
- 检查 <code>!a.Permissions.Manage &amp;&amp; len(a.Permissions.Edit) == 0</code>。条件成立时执行 <code>return a, domain.Violation(domain.PermissionDenied, &quot;RESOURCE_EDIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>a.Document.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return a, domain.Violation(domain.StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return a, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=15143:15637 -->
````go
func (s *Resources) uploadAccess(ctx context.Context, actor Actor, document uint64) (DocumentAccess, error) {
	a, err := s.access.Authorize(ctx, actor, document, "RESOURCE_UPLOAD")
	if err != nil {
		return a, err
	}
	if !a.Permissions.Manage && len(a.Permissions.Edit) == 0 {
		return a, domain.Violation(domain.PermissionDenied, "RESOURCE_EDIT_REQUIRED")
	}
	if a.Document.Lifecycle != "ACTIVE" {
		return a, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
	}
	return a, nil
}

````

## 第 23 步：Resources.PrepareUpload

插入位置：第 22 步代码之后，紧接追加；完成稿约第 471 行。

本步声明：`func (s *Resources) PrepareUpload(ctx context.Context, actor Actor, document uint64, input dto.ResourceUpload) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.uploadAccess(ctx, actor, document)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, nil, invalid</code>；<code>s.uploadAccess(ctx, actor, document)</code>；<code>return actor, nil, nil, err</code>；<code>actor.Sources</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.uploadAccess(ctx, actor, document)</code>；<code>Document: document</code>；<code>u.Document != document</code> |
| <code>input</code> | <code>dto.ResourceUpload</code> | 在此声明中的实际使用：<code>input.ByteSize</code>；<code>input.ExpectedQuotaRevision</code>；<code>input.Checksum</code>；<code>input.MediaType</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; size &gt; s.limits.UploadBytes &#124;&#124; s.limits.SpaceBytes == 0 &#124;&#124; s.limits.UploadTTL &lt;= 0</code>。条件成立时执行 <code>return actor, nil, nil, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(checksum) != 32 &#124;&#124; hex.EncodeToString(checksum) != input.Checksum</code>。条件成立时执行 <code>return actor, nil, nil, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; media != input.MediaType &#124;&#124; len(params) != 0 &#124;&#124; len(media) &gt; 100</code>。条件成立时执行 <code>return actor, nil, nil, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 23 bytes=15637:17512 -->
````go
func (s *Resources) PrepareUpload(ctx context.Context, actor Actor, document uint64, input dto.ResourceUpload) (Actor, MutationBody, MutationPresentation, error) {
	invalid := domain.Violation(domain.InvalidRequest, "RESOURCE_UPLOAD_INVALID")
	size, err := domain.CanonicalRevision(input.ByteSize)
	if err != nil || size > s.limits.UploadBytes || s.limits.SpaceBytes == 0 || s.limits.UploadTTL <= 0 {
		return actor, nil, nil, invalid
	}
	revision, err := domain.CanonicalRevision(input.ExpectedQuotaRevision)
	if err != nil {
		return actor, nil, nil, invalid
	}
	checksum, err := hex.DecodeString(input.Checksum)
	if err != nil || len(checksum) != 32 || hex.EncodeToString(checksum) != input.Checksum {
		return actor, nil, nil, invalid
	}
	media, params, err := mime.ParseMediaType(input.MediaType)
	if err != nil || media != input.MediaType || len(params) != 0 || len(media) > 100 {
		return actor, nil, nil, invalid
	}
	access, err := s.uploadAccess(ctx, actor, document)
	if err != nil {
		return actor, nil, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if current.Lifecycle != "ACTIVE" {
			return nil, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
		}
		space, _ := domain.CanonicalID(current.SpaceId)
		quota, err := repository.LockQuota(ctx, tx, space, s.limits.SpaceBytes)
		if err != nil {
			return nil, err
		}
		expires := time.Now().UTC().Add(s.limits.UploadTTL)
		if err := repository.ReserveQuota(ctx, tx, quota, input.OperationId, size, revision, expires); err != nil {
			return nil, err
		}
		if err := repository.ReserveTemporary(ctx, tx, s.limits.TemporaryBytes, size); err != nil {
			return nil, err
		}
		var key [32]byte
		if _, err := rand.Read(key[:]); err != nil {
````

## 第 24 步：Resources.PrepareUpload（接着实现）

插入位置：第 23 步代码之后，紧接追加；完成稿约第 516 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOURCE_RECEIPT_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>u.Document != document &#124;&#124; u.Uploader != actor.ID &#124;&#124; u.Operation != input.OperationId</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return dto.UploadHandle{ResourceId: rawID, UploadUrl: s.origin + &quot;/v1/resources/&quot; + rawID + &quot;/upload&quot;, ExpiresAt: u.Expires.UTC().Format(time.RFC3339Nano)}, access.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=17512:19112 -->
````go
			return nil, err
		}
		u := repository.Upload{Space: space, Document: document, Uploader: actor.ID, Operation: input.OperationId, Key: hex.EncodeToString(key[:]), Size: size, MediaType: media}
		copy(u.Checksum[:], checksum)
		id, err := repository.InsertUpload(ctx, tx, u)
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, nil, err
		}
		if current.Lifecycle != "ACTIVE" {
			return nil, nil, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
		}
		result, ok := value["result"].(map[string]any)
		var rawID string
		if ok {
			rawID, _ = result["entity_id"].(string)
		} else if initial, ok := value["result"].(map[string]string); ok {
			rawID = initial["entity_id"]
		}
		id, err := domain.CanonicalID(rawID)
		if err != nil {
			return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "RESOURCE_RECEIPT_INVALID")
		}
		u, err := repository.UploadRow(ctx, tx, id, false)
		if err != nil {
			return nil, nil, err
		}
		if u.Document != document || u.Uploader != actor.ID || u.Operation != input.OperationId {
			return nil, nil, domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_BINDING_CHANGED")
		}
		return dto.UploadHandle{ResourceId: rawID, UploadUrl: s.origin + "/v1/resources/" + rawID + "/upload", ExpiresAt: u.Expires.UTC().Format(time.RFC3339Nano)}, access.Sources, nil
	}
	return actor, run, present, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/resources_storage_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
