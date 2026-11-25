# 分步编写 `services/document/internal/logic/document/exports_service.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/205-exports-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

编排MAIN/PUBLIC导出准入、持久排队、后台转换及下载和生命周期排空。

导出既要固定内容，也要在领取、查询和下载时遵守当前来源；后台任务不能成为撤权后的下载旁路。

写之前先按这条执行路线理解各段的连接：

1. Create固定计划并登记响应
2. Recover按文档共享门及job排他锁处理
3. exportDocument组装规范版本和资源
4. convertJob转换后复核authority
5. Download核当前资格并登记输出
6. DrainDocument只等本篇真实转换。

本文件需要守住的边界：

- 不靠TTL猜子进程结束
- 删除实际制品先于归还预算
- 其他文档持续导出不能饿死本篇排空。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/exports_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/exports_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/exports_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/exports_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:689 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"collaboration/document/internal/storage/local"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"time"
)

// export.go：保留原实现，按业务合并文件。

````

## 第 2 步：ExportRenderer

插入位置：第 1 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`type ExportRenderer interface {`。

**这一段为什么这样写：** 文档转换适配接口，独立于SQL排队和权限业务，便于替换转换器而不改变导出状态机。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Render</code> | <code>(context.Context, domain.ExportRecord, domain.ExportDocument) (string, uint64, &#91;32&#93;byte, error)</code> | 顺序输入ctx、固定导出任务、固定正文，返回存储键、字节数、SHA-256、错误；转换成功后还要由业务提交任务结果。 |
| <code>Cleanup</code> | <code>(context.Context, domain.ExportRecord) error</code> | 输入ctx和原任务，回收该任务转换临时产物；不能删除其他任务目录。 |
| <code>Files</code> | <code>() ResourceFiles</code> | 返回导出产物的受控存储接口，下载仍须走权限和容量校验。 |

<!-- write-step: 2 bytes=689:903 -->
````go
type ExportRenderer interface {
	Render(context.Context, domain.ExportRecord, domain.ExportDocument) (string, uint64, [32]byte, error)
	Cleanup(context.Context, domain.ExportRecord) error
	Files() ResourceFiles
}

````

## 第 3 步：Exports

插入位置：第 2 步代码之后，紧接追加；完成稿约第 33 行。

Exports编排权威准入与持久作业；格式转换/文件存储通过构造注入，HTTP与SQL分别留在适配层。

本步声明：`type Exports struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>public</code> | <code>*PublicReading</code> | 使用 <code>PublicReading</code> 类型；其定义处的职责：按当前正式指针与当前受众输出公开文档、受控历史和搜索快照。 |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。 |
| <code>resources</code> | <code>*Resources</code> | 使用 <code>Resources</code> 类型；其定义处的职责：编排上传预留、流式写入、完成结算和未引用对象回收。 |
| <code>renderer</code> | <code>ExportRenderer</code> | 使用 <code>ExportRenderer</code> 类型；其定义处的职责：文档转换适配接口，独立于SQL排队和权限业务，便于替换转换器而不改变导出状态机。 |
| <code>workers</code> | <code>int</code> | 受控导出worker数量，限制同时启动的转换进程；排队不等于无限并发。 |
| <code>temporaryLimit</code> | <code>uint64</code> | 导出临时容量预算（字节），和worker数量共同限制服务器资源。 |

<!-- write-step: 3 bytes=903:1286 -->
````go
// Exports编排权威准入与持久作业；格式转换/文件存储通过构造注入，HTTP与SQL分别留在适配层。
type Exports struct {
	db             *sql.DB
	identity       *Identity
	access         *Access
	public         *PublicReading
	outputs        *Outputs
	resources      *Resources
	renderer       ExportRenderer
	workers        int
	temporaryLimit uint64
}

````

## 第 4 步：NewExports

插入位置：第 3 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`func NewExports(db *sql.DB, identity *Identity, access *Access, public *PublicReading, outputs *Outputs, resources *Resources, renderer ExportRenderer, workers int) (*Exports, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>public</code> | <code>*PublicReading</code> | 使用 <code>PublicReading</code> 类型；其定义处的职责：按当前正式指针与当前受众输出公开文档、受控历史和搜索快照。<br>在此声明中的实际使用：<code>public: public</code> |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs: outputs</code> |
| <code>resources</code> | <code>*Resources</code> | 使用 <code>Resources</code> 类型；其定义处的职责：编排上传预留、流式写入、完成结算和未引用对象回收。<br>在此声明中的实际使用：<code>resources: resources</code> |
| <code>renderer</code> | <code>ExportRenderer</code> | 使用 <code>ExportRenderer</code> 类型；其定义处的职责：文档转换适配接口，独立于SQL排队和权限业务，便于替换转换器而不改变导出状态机。<br>在此声明中的实际使用：<code>renderer: renderer</code> |
| <code>workers</code> | <code>int</code> | 在此声明中的实际使用：<code>workers == 0</code>；<code>workers = 1</code>；<code>workers &lt; 1</code>；<code>workers &gt; 8</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Exports</code> | 使用 <code>Exports</code> 类型；其定义处的职责：Exports编排权威准入与持久作业；格式转换/文件存储通过构造注入，HTTP与SQL分别留在适配层。<br>实际返回表达式：<code>nil</code>；<code>&amp;Exports{db: db, identity: identity, access: access, public: public, outputs: outputs, resources: resources, renderer: renderer, 		workers: workers, temporaryLimit: uint64(workers) * repository.ExportTemporaryLimit}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;EXPORT_WORKERS_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>workers == 0</code>。
- 检查 <code>workers &lt; 1 &#124;&#124; workers &gt; 8</code>。条件成立时执行 <code>return nil, errors.New(&quot;EXPORT_WORKERS_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Exports{db: db, identity: identity, access: access, public: public, outputs: outputs, resources: resources, renderer: renderer, 		workers: workers, temporaryLimit: uint64(w …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1286:1979 -->
````go
func NewExports(db *sql.DB, identity *Identity, access *Access, public *PublicReading, outputs *Outputs, resources *Resources, renderer ExportRenderer, workers int) (*Exports, error) {
	if workers == 0 {
		workers = 1
	}
	if workers < 1 || workers > 8 {
		return nil, errors.New("EXPORT_WORKERS_INVALID")
	}
	// 这是持久预留的磁盘预算，不代表转换器的内存上限。
	// 每worker留256MiB，沿用单任务192MiB预留和32MiB制品限制。
	return &Exports{db: db, identity: identity, access: access, public: public, outputs: outputs, resources: resources, renderer: renderer,
		workers: workers, temporaryLimit: uint64(workers) * repository.ExportTemporaryLimit}, nil
}

````

## 第 5 步：Exports.WorkerCount

插入位置：第 4 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func (s *Exports) WorkerCount() int`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>s.workers</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.workers</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1979:2037 -->
````go
func (s *Exports) WorkerCount() int { return s.workers }

````

## 第 6 步：exportAdmission

插入位置：第 5 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`type exportAdmission struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 本次导出读取依赖的权限来源列表。 |
| <code>check</code> | <code>func(context.Context, *sql.Tx) (domain.ExportSnapshot, error)</code> | 接收ctx和原事务，再核验导出快照、返回固定ExportSnapshot；不能在另一个连接先读取再提交。 |

<!-- write-step: 6 bytes=2037:2168 -->
````go
type exportAdmission struct {
	sources []*pb.SourceEpoch
	check   func(context.Context, *sql.Tx) (domain.ExportSnapshot, error)
}

````

## 第 7 步：Exports.prepare

插入位置：第 6 步代码之后，紧接追加；完成稿约第 66 行。

本步声明：`func (s *Exports) prepare(ctx context.Context, actor *Actor, document uint64, input domain.ExportInput, creating bool) (exportAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, *actor, document, &quot;EXPORT_MAIN&quot;)</code>；<code>s.public.facts(ctx, document)</code>；<code>s.public.audience(ctx, actor, facts, policy)</code> |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor == nil</code>；<code>s.access.Authorize(ctx, *actor, document, &quot;EXPORT_MAIN&quot;)</code>；<code>s.mainExportView(ctx, tx, document, manifest, *actor)</code>；<code>s.public.audience(ctx, actor, facts, policy)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, *actor, document, &quot;EXPORT_MAIN&quot;)</code>；<code>s.mainExportView(ctx, tx, document, manifest, *actor)</code>；<code>repository.ExportManifestFacts(ctx, tx, document, manifest)</code>；<code>s.public.facts(ctx, document)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>domain.ExportInput</code> | 使用 <code>domain.ExportInput</code> 类型；其定义处的职责：ExportInput 是冻结 union 的服务端判别结果，不替代生成 DTO；解码仍拒绝未知字段。<br>在此声明中的实际使用：<code>input.Target</code>；<code>input.Manifest</code>；<code>input.Format</code>；<code>input.Release</code> |
| <code>creating</code> | <code>bool</code> | 在此声明中的实际使用：<code>admitted.check = func(ctx context.Context, tx *sql.Tx) (domain.ExportSnapshot, error) { 			var fixed domain.ExportSnapshot 			current, err := ValidateAccess(ctx, tx, access) 			if  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>exportAdmission</code> | 实际返回表达式：<code>admitted</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.SessionRequired, &quot;SESSION_REQUIRED&quot;)</code>；<code>err</code>；<code>nil</code>；<code>domain.Violation(domain.EntityNotFound, &quot;EXPORT_PUBLIC_NOT_CURRENT&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>input.Target == &quot;MAIN&quot;</code>。
- 检查 <code>actor == nil</code>。条件成立时执行 <code>return admitted, domain.Violation(domain.SessionRequired, &quot;SESSION_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return admitted, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>creating</code>。
- 在本段的匿名函数内：检查 <code>!allowed</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!allowed</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.StateConflict, &quot;EXPORT_MAIN_VIEW_EXPIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2168:3680 -->
````go
func (s *Exports) prepare(ctx context.Context, actor *Actor, document uint64, input domain.ExportInput, creating bool) (exportAdmission, error) {
	var admitted exportAdmission
	if input.Target == "MAIN" {
		if actor == nil {
			return admitted, domain.Violation(domain.SessionRequired, "SESSION_REQUIRED")
		}
		access, err := s.access.Authorize(ctx, *actor, document, "EXPORT_MAIN")
		if err != nil {
			return admitted, err
		}
		admitted.sources = access.Sources
		admitted.check = func(ctx context.Context, tx *sql.Tx) (domain.ExportSnapshot, error) {
			var fixed domain.ExportSnapshot
			current, err := ValidateAccess(ctx, tx, access)
			if err != nil {
				return fixed, err
			}
			manifest, err := domain.CanonicalID(input.Manifest)
			if err != nil {
				return fixed, err
			}
			if creating {
				allowed := current.MainLocked && current.CurrentManifestId == input.Manifest
				if !allowed {
					allowed, err = s.mainExportView(ctx, tx, document, manifest, *actor)
				}
				if err != nil {
					return fixed, err
				}
				if !allowed {
					return fixed, domain.Violation(domain.StateConflict, "EXPORT_MAIN_VIEW_EXPIRED")
				}
			}
			fixed, err = repository.ExportManifestFacts(ctx, tx, document, manifest)
			if err != nil {
				return fixed, err
			}
			fixed.Title = current.Title
			fixed.Space, _ = domain.CanonicalID(current.SpaceId)
			fixed.Target = "MAIN"
			fixed.Format = input.Format
			fixed.Captured = time.Now().UTC()
			fixed.Sources = admitted.sources
			return fixed, nil
		}
````

## 第 8 步：Exports.prepare（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 111 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return admitted, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return admitted, err</code>。当前函数在这里结束。
- 检查 <code>!domain.HasCurrentPublication(facts, policy) &#124;&#124; facts.Document.CurrentReleaseId == nil &#124;&#124; *facts.Document.CurrentReleaseId != input.Release</code>。条件成立时执行 <code>return admitted, domain.Violation(domain.EntityNotFound, &quot;EXPORT_PUBLIC_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return admitted, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>publicFactsBinding(current) != publicFactsBinding(facts) &#124;&#124; !domain.HasCurrentPublication(current, policy)</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.StateConflict, &quot;EXPORT_PUBLIC_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3680:5355 -->
````go
		return admitted, nil
	}
	facts, err := s.public.facts(ctx, document)
	if err != nil {
		return admitted, err
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return admitted, err
	}
	if !domain.HasCurrentPublication(facts, policy) || facts.Document.CurrentReleaseId == nil || *facts.Document.CurrentReleaseId != input.Release {
		return admitted, domain.Violation(domain.EntityNotFound, "EXPORT_PUBLIC_NOT_CURRENT")
	}
	_, admitted.sources, err = s.public.audience(ctx, actor, facts, policy)
	if err != nil {
		return admitted, err
	}
	admitted.check = func(ctx context.Context, tx *sql.Tx) (domain.ExportSnapshot, error) {
		var fixed domain.ExportSnapshot
		current, err := repository.PublicationFactsRow(ctx, tx, document, true)
		if err != nil {
			return fixed, err
		}
		if publicFactsBinding(current) != publicFactsBinding(facts) || !domain.HasCurrentPublication(current, policy) {
			return fixed, domain.Violation(domain.StateConflict, "EXPORT_PUBLIC_CHANGED")
		}
		release, _ := domain.CanonicalID(input.Release)
		value, err := repository.FixedReleaseRow(ctx, tx, document, release)
		if err != nil {
			return fixed, err
		}
		manifest, _ := domain.CanonicalID(value.Release.ManifestId)
		fixed, err = repository.ExportManifestFacts(ctx, tx, document, manifest)
		if err != nil {
			return fixed, err
		}
		fixed.Space, _ = domain.CanonicalID(current.Space.Id)
		fixed.Publication, _ = domain.CanonicalID(current.Document.PublicationEpoch)
		fixed.Release = release
		fixed.Title = value.Title
		fixed.Target = "PUBLIC"
		fixed.Format = input.Format
		fixed.Captured = time.Now().UTC()
		fixed.Sources = admitted.sources
		return fixed, nil
	}
````

## 第 9 步：Exports.prepare（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 157 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return admitted, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5355:5380 -->
````go
	return admitted, nil
}

````

## 第 10 步：Exports.mainExportView

插入位置：第 9 步代码之后，紧接追加；完成稿约第 160 行。

本步声明：`func (s *Exports) mainExportView(ctx context.Context, tx *sql.Tx, doc, manifest uint64, actor Actor) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ExportMainView(ctx, tx, doc, manifest, actor.ID, actor.Proof.SessionRef)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.ExportMainView(ctx, tx, doc, manifest, actor.ID, actor.Proof.SessionRef)</code> |
| <code>doc</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ExportMainView(ctx, tx, doc, manifest, actor.ID, actor.Proof.SessionRef)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ExportMainView(ctx, tx, doc, manifest, actor.ID, actor.Proof.SessionRef)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>repository.ExportMainView(ctx, tx, doc, manifest, actor.ID, actor.Proof.SessionRef)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>repository.ExportMainView(ctx, tx, doc, manifest, actor.ID, actor.Proof.SessionRef)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return repository.ExportMainView(ctx, tx, doc, manifest, actor.ID, actor.Proof.SessionRef)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5380:5592 -->
````go
func (s *Exports) mainExportView(ctx context.Context, tx *sql.Tx, doc, manifest uint64, actor Actor) (bool, error) {
	return repository.ExportMainView(ctx, tx, doc, manifest, actor.ID, actor.Proof.SessionRef)
}

````

## 第 11 步：Exports.register

插入位置：第 10 步代码之后，紧接追加；完成稿约第 164 行。

本步声明：`func (s *Exports) register(ctx context.Context, tx *sql.Tx, actor *Actor, fixed domain.ExportSnapshot, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.outputs.RegisterPublication(ctx, tx, fixed.Sources, fixed.Document, fixed.Publication, fixed.Release, handler, edge)</code>；<code>InsertAdmission(ctx, tx, *actor, fixed.Sources, fixed.Document, &quot;EXPORT_MAIN&quot;, admission)</code>；<code>s.outputs.Register(ctx, tx, fixed.Sources, fixed.Document, admission, handler, edge)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.outputs.RegisterPublication(ctx, tx, fixed.Sources, fixed.Document, fixed.Publication, fixed.Release, handler, edge)</code>；<code>InsertAdmission(ctx, tx, *actor, fixed.Sources, fixed.Document, &quot;EXPORT_MAIN&quot;, admission)</code>；<code>s.outputs.Register(ctx, tx, fixed.Sources, fixed.Document, admission, handler, edge)</code> |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>InsertAdmission(ctx, tx, *actor, fixed.Sources, fixed.Document, &quot;EXPORT_MAIN&quot;, admission)</code> |
| <code>fixed</code> | <code>domain.ExportSnapshot</code> | 使用 <code>domain.ExportSnapshot</code> 类型；其定义处的职责：固定计划只存依赖及权威来源；正文从不可变manifest装配，RUNNING任务阻止提前清理历史。<br>在此声明中的实际使用：<code>fixed.Target</code>；<code>fixed.Sources</code>；<code>fixed.Document</code>；<code>fixed.Publication</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>s.outputs.RegisterPublication(ctx, tx, fixed.Sources, fixed.Document, fixed.Publication, fixed.Release, handler, edge)</code>；<code>s.outputs.Register(ctx, tx, fixed.Sources, fixed.Document, admission, handler, edge)</code> |
| <code>edge</code> | <code>*domain.OutputInstance</code> | 使用 <code>domain.OutputInstance</code> 类型；其定义处的职责：输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。<br>在此声明中的实际使用：<code>s.outputs.RegisterPublication(ctx, tx, fixed.Sources, fixed.Document, fixed.Publication, fixed.Release, handler, edge)</code>；<code>s.outputs.Register(ctx, tx, fixed.Sources, fixed.Document, admission, handler, edge)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResponseRegistration</code> | 使用 <code>ResponseRegistration</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。<br>由 <code>s.outputs.RegisterPublication(ctx, tx, fixed.Sources, fixed.Document, fixed.Publication, fixed.Release, handler, edge)</code> 的第 1 个返回项原样转交。<br>由 <code>s.outputs.Register(ctx, tx, fixed.Sources, fixed.Document, admission, handler, edge)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.outputs.RegisterPublication(ctx, tx, fixed.Sources, fixed.Document, fixed.Publication, fixed.Release, handler, edge)</code> 的第 2 个返回项原样转交。<br>由 <code>s.outputs.Register(ctx, tx, fixed.Sources, fixed.Document, admission, handler, edge)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Target == &quot;PUBLIC&quot;</code>。条件成立时执行 <code>return s.outputs.RegisterPublication(ctx, tx, fixed.Sources, fixed.Document, fixed.Publication, fixed.Release, handler, edge)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return s.outputs.Register(ctx, tx, fixed.Sources, fixed.Document, admission, handler, edge)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5592:6203 -->
````go
func (s *Exports) register(ctx context.Context, tx *sql.Tx, actor *Actor, fixed domain.ExportSnapshot, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	if fixed.Target == "PUBLIC" {
		return s.outputs.RegisterPublication(ctx, tx, fixed.Sources, fixed.Document, fixed.Publication, fixed.Release, handler, edge)
	}
	admission := platform.NewTraceID()
	if err := InsertAdmission(ctx, tx, *actor, fixed.Sources, fixed.Document, "EXPORT_MAIN", admission); err != nil {
		return nil, err
	}
	return s.outputs.Register(ctx, tx, fixed.Sources, fixed.Document, admission, handler, edge)
}

````

## 第 12 步：exportActorID

插入位置：第 11 步代码之后，紧接追加；完成稿约第 175 行。

本步声明：`func exportActorID(actor *Actor) uint64`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor == nil</code>；<code>actor.ID</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>actor.ID</code> |

**跟着本段代码走一遍：**

- 检查 <code>actor == nil</code>。条件成立时执行 <code>return 0</code>。当前函数在这里结束。
- 此处通过 <code>return actor.ID</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6203:6298 -->
````go
func exportActorID(actor *Actor) uint64 {
	if actor == nil {
		return 0
	}
	return actor.ID
}

````

## 第 13 步：exportStoredInput

插入位置：第 12 步代码之后，紧接追加；完成稿约第 182 行。

本步声明：`func exportStoredInput(value domain.ExportRecord) domain.ExportInput`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>在此声明中的实际使用：<code>value.Operation</code>；<code>value.Snapshot</code>；<code>value.Job</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ExportInput</code> | 使用 <code>domain.ExportInput</code> 类型；其定义处的职责：ExportInput 是冻结 union 的服务端判别结果，不替代生成 DTO；解码仍拒绝未知字段。<br>实际返回表达式：<code>input</code> |

**跟着本段代码走一遍：**

- 检查 <code>input.Target == &quot;MAIN&quot;</code>。
- 检查 <code>value.Job.ReleaseId != nil</code>。
- 此处通过 <code>return input</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6298:6717 -->
````go
func exportStoredInput(value domain.ExportRecord) domain.ExportInput {
	input := domain.ExportInput{Operation: value.Operation, Target: value.Snapshot.Target, Format: value.Job.Format}
	if input.Target == "MAIN" {
		input.Manifest = value.Job.ManifestId
	} else if value.Job.ReleaseId != nil {
		input.Release = *value.Job.ReleaseId
	}
	return input
}

// export_create.go：保留原实现，按业务合并文件。

````

## 第 14 步：Exports.Create

插入位置：第 13 步代码之后，紧接追加；完成稿约第 194 行。

本步声明：`func (s *Exports) Create(ctx context.Context, actor *Actor, id uint64, input domain.ExportInput, raw dto.ExportCreate, path, handler string, bind BindOutput) (dto.ExportJob, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ExportOperation(ctx, s.db, input.Operation)</code>；<code>s.prepare(ctx, actor, id, input, lookupErr != nil)</code>；<code>s.identity.Revalidate(ctx, actor.Proof)</code>；<code>s.outputs.EdgeInstance(ctx, handler)</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>exportActorID(actor)</code>；<code>s.prepare(ctx, actor, id, input, lookupErr != nil)</code>；<code>actor != nil</code>；<code>actor.Proof</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(id)</code>；<code>s.prepare(ctx, actor, id, input, lookupErr != nil)</code> |
| <code>input</code> | <code>domain.ExportInput</code> | 使用 <code>domain.ExportInput</code> 类型；其定义处的职责：ExportInput 是冻结 union 的服务端判别结果，不替代生成 DTO；解码仍拒绝未知字段。<br>在此声明中的实际使用：<code>input.Validate</code>；<code>input.Operation</code>；<code>s.prepare(ctx, actor, id, input, lookupErr != nil)</code>；<code>input.Format</code> |
| <code>raw</code> | <code>dto.ExportCreate</code> | 在此声明中的实际使用：<code>json.RawMessage(raw)</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, json.RawMessage(raw))</code>；<code>repository.InsertExportReceipt(ctx, tx, exportActorID(actor), identityRef, input.Operation, path, fingerprint, jobID)</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.register(ctx, tx, actor, fixed, handler, edge)</code> |
| <code>bind</code> | <code>BindOutput</code> | 使用 <code>BindOutput</code> 类型；其定义处的职责：BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。<br>在此声明中的实际使用：<code>bind(registration, fixed.Sources)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.ExportJob</code> | 实际返回表达式：<code>empty</code>；<code>job</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>lookupErr</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>lookupErr != nil &amp;&amp; !errors.Is(lookupErr, sql.ErrNoRows)</code>。条件成立时执行 <code>return empty, lookupErr</code>。当前函数在这里结束。
- 检查 <code>lookupErr == nil &amp;&amp; (observed.Requester != exportActorID(actor) &#124;&#124; observed.Job.DocumentId != domain.TextID(id))</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>actor != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; edge == nil</code>。条件成立时执行 <code>return empty, domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>priorErr == nil</code>。
- 检查 <code>prior.Requester != exportActorID(actor) &#124;&#124; prior.Job.DocumentId != domain.TextID(id) &#124;&#124; !bytes.Equal(prior.Fingerprint&#91;:&#93;, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>!errors.Is(priorErr, sql.ErrNoRows)</code>。条件成立时执行 <code>return empty, priorErr</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=6717:8709 -->
````go
func (s *Exports) Create(ctx context.Context, actor *Actor, id uint64, input domain.ExportInput, raw dto.ExportCreate, path, handler string, bind BindOutput) (dto.ExportJob, error) {
	var empty dto.ExportJob
	if err := input.Validate(); err != nil {
		return empty, err
	}
	observed, lookupErr := repository.ExportOperation(ctx, s.db, input.Operation)
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return empty, lookupErr
	}
	if lookupErr == nil && (observed.Requester != exportActorID(actor) || observed.Job.DocumentId != domain.TextID(id)) {
		return empty, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	admitted, err := s.prepare(ctx, actor, id, input, lookupErr != nil)
	if err != nil {
		return empty, err
	}
	if actor != nil {
		if err = s.identity.Revalidate(ctx, actor.Proof); err != nil {
			return empty, err
		}
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	fingerprint, err := domain.OperationFingerprint(path, json.RawMessage(raw))
	if err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, input.Operation); err != nil {
		return empty, err
	}
	if err = LockSources(ctx, tx, admitted.sources); err != nil {
		return empty, err
	}
	// 原job先复核公开/私有资格，但不再要求已经到期的原阅读视图；只重放已登记的固定版本。
	prior, priorErr := repository.ExportOperation(ctx, tx, input.Operation)
	if priorErr == nil {
		if prior.Requester != exportActorID(actor) || prior.Job.DocumentId != domain.TextID(id) || !bytes.Equal(prior.Fingerprint[:], fingerprint[:]) {
			return empty, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
	} else if !errors.Is(priorErr, sql.ErrNoRows) {
````

## 第 15 步：Exports.Create（接着实现）

插入位置：第 14 步代码之后，紧接追加；完成稿约第 241 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return empty, priorErr</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>priorErr == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>conflict</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>actor != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>fixed.Release != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=8709:10171 -->
````go
		return empty, priorErr
	}
	fixed, err := admitted.check(ctx, tx)
	if err != nil {
		return empty, err
	}
	var job dto.ExportJob
	if priorErr == nil {
		job = prior.Job
	} else {
		// 不与其他业务种类共用同号，匿名导出也必须参与全局原号gate。
		conflict, err := repository.OperationExists(ctx, tx, input.Operation)
		if err != nil {
			return empty, err
		}
		if conflict {
			return empty, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		if err = repository.CheckExportQueue(ctx, tx); err != nil {
			return empty, err
		}
		jobID, err := repository.InsertExport(ctx, tx, exportActorID(actor), input.Operation, fingerprint, fixed)
		if err != nil {
			return empty, err
		}
		if err = repository.PinExportResources(ctx, tx, jobID, fixed); err != nil {
			return empty, err
		}
		var identityRef string
		if actor != nil {
			identityRef = actor.Proof.AdmissionId
		}
		if err = repository.InsertExportReceipt(ctx, tx, exportActorID(actor), identityRef, input.Operation, path, fingerprint, jobID); err != nil {
			return empty, err
		}
		job = dto.ExportJob{Id: domain.TextID(jobID), DocumentId: domain.TextID(id), ManifestId: domain.TextID(fixed.Manifest), Format: input.Format, State: "QUEUED"}
		if fixed.Release != 0 {
			release := domain.TextID(fixed.Release)
			job.ReleaseId = &release
		}
	}
	registration, err := s.register(ctx, tx, actor, fixed, handler, edge)
	if err != nil {
		return empty, err
	}
````

## 第 16 步：Exports.Create（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 286 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return empty, errors.New(&quot;EXPORT_REGISTRATION_COMMIT_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return job, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=10171:10422 -->
````go
	output := bind(registration, fixed.Sources)
	if err = tx.Commit(); err != nil {
		output.Abort()
		return empty, errors.New("EXPORT_REGISTRATION_COMMIT_UNKNOWN")
	}
	return job, nil
}

// export_drain.go：保留原实现，按业务合并文件。

````

## 第 17 步：Exports.DrainDocument

插入位置：第 16 步代码之后，紧接追加；完成稿约第 296 行。

来源真正关闭后，生命周期恢复器可以排空本篇已终态的导出副本。
只等待本篇实际转换；其他文档不断导出也不能饿死本篇治理排空。
旧全局键仍取共享门以排除旧版本串行worker，删除文件先于归还预算。

本步声明：`func (s *Exports) DrainDocument(ctx context.Context, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>files.AcquireRead(ctx, exportGuardKey(&quot;collaboration-report-export-worker-v1&quot;))</code>；<code>files.Acquire(ctx, exportDocumentGuard(document))</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.ExpireClosedDocumentExports(ctx, tx, document)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>exportDocumentGuard(document)</code>；<code>repository.ExpireClosedDocumentExports(ctx, tx, document)</code>；<code>repository.DocumentExportControlsPending(ctx, s.db, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_DOCUMENT_DRAIN_PENDING&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>release()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_DOCUMENT_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=10422:11710 -->
````go
// 来源真正关闭后，生命周期恢复器可以排空本篇已终态的导出副本。
// 只等待本篇实际转换；其他文档不断导出也不能饿死本篇治理排空。
// 旧全局键仍取共享门以排除旧版本串行worker，删除文件先于归还预算。
func (s *Exports) DrainDocument(ctx context.Context, document uint64) error {
	files := s.renderer.Files()
	release, err := files.AcquireRead(ctx, exportGuardKey("collaboration-report-export-worker-v1"))
	if err != nil {
		return err
	}
	defer release()
	unlock, err := files.Acquire(ctx, exportDocumentGuard(document))
	if err != nil {
		return err
	}
	defer unlock()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	err = repository.ExpireClosedDocumentExports(ctx, tx, document)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if err = s.expireArtifacts(ctx); err != nil {
		return err
	}
	pending, err := repository.DocumentExportControlsPending(ctx, s.db, document)
	if err != nil {
		return err
	}
	if pending {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCUMENT_DRAIN_PENDING")
	}
	return nil
}

// export_expiry.go：保留原实现，按业务合并文件。

````

## 第 18 步：Exports.expireArtifacts

插入位置：第 17 步代码之后，紧接追加；完成稿约第 338 行。

本步声明：`func (s *Exports) expireArtifacts(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.renderer.Files().Acquire(ctx, exportGuardKey(&quot;collaboration-report-export-maintenance-v1&quot;))</code>；<code>repository.ExpiredExports(ctx, s.db)</code>；<code>files.Acquire(ctx, item.Key)</code>；<code>files.Delete(ctx, item.Key)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, local.ErrBusy)</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>item.Key != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=11710:12854 -->
````go
func (s *Exports) expireArtifacts(ctx context.Context) error {
	// 多worker可并行转换，但回收必须串行，否则可能重复归还同一制品预算。
	unlock, err := s.renderer.Files().Acquire(ctx, exportGuardKey("collaboration-report-export-maintenance-v1"))
	if errors.Is(err, local.ErrBusy) {
		return nil
	}
	if err != nil {
		return err
	}
	defer unlock()
	items, err := repository.ExpiredExports(ctx, s.db)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Key != "" {
			files := s.renderer.Files()
			unlock, err := files.Acquire(ctx, item.Key)
			if err != nil {
				continue
			} // 正在实际发送的共享guard不能被临时TTL越过。
			err = files.Delete(ctx, item.Key)
			unlock()
			if err != nil {
				return err
			}
		}
		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			return err
		}
		err = repository.RemoveExpiredExport(ctx, tx, item)
		if err == nil {
			err = tx.Commit()
		} else {
			tx.Rollback()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// export_fixed.go：保留原实现，按业务合并文件。

````

## 第 19 步：Exports.exportDocument

插入位置：第 18 步代码之后，紧接追加；完成稿约第 384 行。

本步声明：`func (s *Exports) exportDocument(ctx context.Context, value domain.ExportRecord) (domain.ExportDocument, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>LockSources(ctx, tx, fixed.Sources)</code>；<code>s.workerAuthority(ctx, tx, value)</code>；<code>repository.StructureRow(ctx, tx, domain.TextID(fixed.Structure), domain.TextID(fixed.Document))</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>value</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>在此声明中的实际使用：<code>value.Snapshot</code>；<code>s.workerAuthority(ctx, tx, value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ExportDocument</code> | 使用 <code>domain.ExportDocument</code> 类型；其定义处的职责：固定输入交给受控转换器；每种输出共享相同正文、资源与版本说明。<br>实际返回表达式：<code>document</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_FIXED_CONTENT_INVALID&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_LIMIT&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_ASSET_LIMIT&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Release != 0</code>。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return document, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return document, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return document, domain.Violation(domain.StateConflict, &quot;EXPORT_FIXED_CONTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>content.ByteSize &gt; 24&lt;&lt;20 &#124;&#124; bytes &gt; (24&lt;&lt;20)-content.ByteSize</code>。条件成立时执行 <code>return document, domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_LIMIT&quot;)</code>。当前函数在这里结束。

<!-- write-step: 19 bytes=12854:14697 -->
````go
func (s *Exports) exportDocument(ctx context.Context, value domain.ExportRecord) (domain.ExportDocument, error) {
	fixed := value.Snapshot
	document := domain.ExportDocument{Title: fixed.Title, Target: fixed.Target, Format: fixed.Format, Manifest: domain.TextID(fixed.Manifest), Captured: fixed.Captured}
	if fixed.Release != 0 {
		document.Release = domain.TextID(fixed.Release)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return document, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, fixed.Sources); err != nil {
		return document, err
	}
	if err = s.workerAuthority(ctx, tx, value); err != nil {
		return document, err
	}
	document.Structure, err = repository.StructureRow(ctx, tx, domain.TextID(fixed.Structure), domain.TextID(fixed.Document))
	if err != nil {
		return document, err
	}
	document.Style, err = repository.StyleRow(ctx, tx, domain.TextID(fixed.Style), domain.TextID(fixed.Document))
	if err != nil {
		return document, err
	}
	units, err := repository.ManifestUnits(ctx, tx, domain.TextID(fixed.Manifest), fixed.Document)
	if err != nil {
		return document, err
	}
	ids := map[uint64]bool{}
	var bytes uint64
	for _, unit := range units {
		version, _ := domain.CanonicalID(unit["content_version_id"])
		unitID, _ := domain.CanonicalID(unit["unit_id"])
		content, err := repository.VersionContent(ctx, tx, version, fixed.Document)
		if err != nil {
			return document, err
		}
		if err = canonicalVersion(content, version); err != nil {
			return document, err
		}
		if err = validateFixedMergeContent(s.public.validator, content, fixed.Document, unitID, fixed.Structure); err != nil {
			return document, domain.Violation(domain.StateConflict, "EXPORT_FIXED_CONTENT_INVALID")
		}
		if content.ByteSize > 24<<20 || bytes > (24<<20)-content.ByteSize {
````

## 第 20 步：Exports.exportDocument（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 429 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return document, domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_LIMIT&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return document, err</code>。当前函数在这里结束。
- 检查 <code>hex.EncodeToString(digest&#91;:&#93;) != body.BodyHash</code>。条件成立时执行 <code>return document, domain.Violation(domain.StateConflict, &quot;EXPORT_FIXED_CONTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return document, domain.Violation(domain.StateConflict, &quot;EXPORT_FIXED_CONTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ordered&#91;i&#93; &lt; ordered&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return document, err</code>。当前函数在这里结束。

<!-- write-step: 20 bytes=14697:16232 -->
````go
			return document, domain.Violation(domain.StateConflict, "EXPORT_CONTENT_LIMIT")
		}
		bytes += content.ByteSize
		body, err := repository.ContentRow(ctx, tx, unit["content_version_id"], domain.TextID(fixed.Document))
		if err != nil {
			return document, err
		}
		// 防止JSON列和参与hash的canonical列漂移；不能把副本损坏静默导出。
		raw, err := json.Marshal(body.BodyJson)
		if err != nil {
			return document, err
		}
		decoded, err := codec.Decode(raw)
		if err != nil {
			return document, err
		}
		canonical, err := codec.Canonical(decoded)
		if err != nil {
			return document, err
		}
		digest := sha256.Sum256(canonical)
		if hex.EncodeToString(digest[:]) != body.BodyHash {
			return document, domain.Violation(domain.StateConflict, "EXPORT_FIXED_CONTENT_INVALID")
		}
		tree, ok := decoded.(map[string]any)
		if !ok {
			return document, domain.Violation(domain.StateConflict, "EXPORT_FIXED_CONTENT_INVALID")
		}
		document.Units = append(document.Units, domain.ExportUnit{ID: body.UnitId, Body: tree})
		for _, id := range content.ResourceIds {
			ids[id] = true
		}
	}
	ordered := make([]uint64, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	var assetBytes uint64
	for _, id := range ordered {
		object, err := resources.ObjectRow(ctx, tx, id, true)
		if err != nil {
			return document, err
		}
		allowed, err := resources.ResourceAllowedForDocument(ctx, tx, id, fixed.Document, fixed.Space)
````

## 第 21 步：Exports.exportDocument（接着实现）

插入位置：第 20 步代码之后，紧接追加；完成稿约第 474 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return document, err</code>。当前函数在这里结束。
- 检查 <code>!allowed &#124;&#124; object.State != &quot;READY&quot; &#124;&#124; object.Size &gt; 32&lt;&lt;20 &#124;&#124; assetBytes &gt; (32&lt;&lt;20)-object.Size</code>。条件成立时执行 <code>return document, domain.Violation(domain.StateConflict, &quot;EXPORT_ASSET_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return document, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return document, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return document, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return document, err</code>。当前函数在这里结束。
- 检查 <code>uint64(len(asset.Data)) != asset.Size</code>。条件成立时执行 <code>return document, domain.Violation(domain.StateConflict, &quot;EXPORT_RESOURCE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return document, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=16232:17543 -->
````go
		if err != nil {
			return document, err
		}
		if !allowed || object.State != "READY" || object.Size > 32<<20 || assetBytes > (32<<20)-object.Size {
			return document, domain.Violation(domain.StateConflict, "EXPORT_ASSET_LIMIT")
		}
		assetBytes += object.Size
		document.Assets = append(document.Assets, domain.ExportAsset{ID: domain.TextID(id), Key: object.Key, MediaType: object.MediaType, Size: object.Size, Checksum: object.Checksum})
	}
	if err = tx.Commit(); err != nil {
		return document, err
	}
	// 资源EXPORT pin已与原job提交；OS共享guard与完整checksum保护这里的数据库外读取。
	for index := range document.Assets {
		asset := &document.Assets[index]
		unlock, err := s.resources.files.AcquireRead(ctx, asset.Key)
		if err != nil {
			return document, err
		}
		file, err := s.resources.files.Open(ctx, asset.Key, asset.Size, asset.Checksum)
		if err != nil {
			unlock()
			return document, err
		}
		asset.Data, err = io.ReadAll(io.LimitReader(file, int64(asset.Size)+1))
		file.Close()
		unlock()
		if err != nil {
			return document, err
		}
		if uint64(len(asset.Data)) != asset.Size {
			return document, domain.Violation(domain.StateConflict, "EXPORT_RESOURCE_INVALID")
		}
	}
	return document, nil
}

// export_query.go：保留原实现，按业务合并文件。

````

## 第 22 步：Exports.lookup

插入位置：第 21 步代码之后，紧接追加；完成稿约第 513 行。

本步声明：`func (s *Exports) lookup(ctx context.Context, actor *Actor, id uint64) (domain.ExportRecord, exportAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ExportRow(ctx, s.db, id, false)</code>；<code>s.prepare(ctx, actor, value.Snapshot.Document, exportStoredInput(value), false)</code> |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>exportActorID(actor)</code>；<code>s.prepare(ctx, actor, value.Snapshot.Document, exportStoredInput(value), false)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ExportRow(ctx, s.db, id, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>exportAdmission</code> | 实际返回表达式：<code>exportAdmission{}</code>；<code>admitted</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;EXPORT_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;EXPORT_EXPIRED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, exportAdmission{}, err</code>。当前函数在这里结束。
- 检查 <code>value.Requester != 0 &amp;&amp; value.Requester != exportActorID(actor)</code>。条件成立时执行 <code>return value, exportAdmission{}, domain.Violation(domain.EntityNotFound, &quot;EXPORT_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>!value.Expires.After(time.Now().UTC())</code>。条件成立时执行 <code>return value, exportAdmission{}, domain.Violation(domain.EntityNotFound, &quot;EXPORT_EXPIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, admitted, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=17543:18325 -->
````go
func (s *Exports) lookup(ctx context.Context, actor *Actor, id uint64) (domain.ExportRecord, exportAdmission, error) {
	value, err := repository.ExportRow(ctx, s.db, id, false)
	if errors.Is(err, sql.ErrNoRows) {
		err = domain.Violation(domain.EntityNotFound, "EXPORT_NOT_FOUND")
	}
	if err != nil {
		return value, exportAdmission{}, err
	}
	if value.Requester != 0 && value.Requester != exportActorID(actor) {
		return value, exportAdmission{}, domain.Violation(domain.EntityNotFound, "EXPORT_NOT_FOUND")
	}
	if !value.Expires.After(time.Now().UTC()) {
		return value, exportAdmission{}, domain.Violation(domain.EntityNotFound, "EXPORT_EXPIRED")
	}
	admitted, err := s.prepare(ctx, actor, value.Snapshot.Document, exportStoredInput(value), false)
	return value, admitted, err
}

````

## 第 23 步：Exports.Job

插入位置：第 22 步代码之后，紧接追加；完成稿约第 531 行。

本步声明：`func (s *Exports) Job(ctx context.Context, actor *Actor, id uint64, handler string, bind BindOutput) (dto.ExportJob, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.lookup(ctx, actor, id)</code>；<code>s.identity.Revalidate(ctx, actor.Proof)</code>；<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.lookup(ctx, actor, id)</code>；<code>actor != nil</code>；<code>actor.Proof</code>；<code>s.register(ctx, tx, actor, fixed, handler, edge)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.lookup(ctx, actor, id)</code>；<code>repository.ExportRow(ctx, tx, id, true)</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.register(ctx, tx, actor, fixed, handler, edge)</code> |
| <code>bind</code> | <code>BindOutput</code> | 使用 <code>BindOutput</code> 类型；其定义处的职责：BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。<br>在此声明中的实际使用：<code>bind(registration, fixed.Sources)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.ExportJob</code> | 实际返回表达式：<code>empty</code>；<code>current.Job</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_JOB_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>actor != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; edge == nil</code>。条件成立时执行 <code>return empty, domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>current.Operation != value.Operation &#124;&#124; current.Requester != value.Requester &#124;&#124; !current.Expires.After(time.Now().UTC())</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;EXPORT_JOB_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return empty, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return current.Job, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=18325:19726 -->
````go
func (s *Exports) Job(ctx context.Context, actor *Actor, id uint64, handler string, bind BindOutput) (dto.ExportJob, error) {
	var empty dto.ExportJob
	value, admitted, err := s.lookup(ctx, actor, id)
	if err != nil {
		return empty, err
	}
	if actor != nil {
		if err = s.identity.Revalidate(ctx, actor.Proof); err != nil {
			return empty, err
		}
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, admitted.sources); err != nil {
		return empty, err
	}
	fixed, err := admitted.check(ctx, tx)
	if err != nil {
		return empty, err
	}
	current, err := repository.ExportRow(ctx, tx, id, true)
	if err != nil {
		return empty, err
	}
	if current.Operation != value.Operation || current.Requester != value.Requester || !current.Expires.After(time.Now().UTC()) {
		return empty, domain.Violation(domain.StateConflict, "EXPORT_JOB_CHANGED")
	}
	registration, err := s.register(ctx, tx, actor, fixed, handler, edge)
	if err != nil {
		return empty, err
	}
	output := bind(registration, fixed.Sources)
	if err = tx.Commit(); err != nil {
		output.Abort()
		return empty, err
	}
	return current.Job, nil
}

````

## 第 24 步：Exports.Download

插入位置：第 23 步代码之后，紧接追加；完成稿约第 577 行。

本步声明：`func (s *Exports) Download(ctx context.Context, actor *Actor, id uint64, handler string, bind BindOutput) (*ResourceDownload, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.lookup(ctx, actor, id)</code>；<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>files.AcquireRead(ctx, value.ArtifactKey)</code>；<code>files.Open(ctx, value.ArtifactKey, value.Size, value.Checksum)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.lookup(ctx, actor, id)</code>；<code>actor != nil</code>；<code>actor.Proof</code>；<code>s.register(ctx, tx, actor, fixed, handler, edge)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.lookup(ctx, actor, id)</code>；<code>ID: id</code>；<code>repository.ExportRow(ctx, tx, id, true)</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.register(ctx, tx, actor, fixed, handler, edge)</code> |
| <code>bind</code> | <code>BindOutput</code> | 使用 <code>BindOutput</code> 类型；其定义处的职责：BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。<br>在此声明中的实际使用：<code>bind(registration, fixed.Sources)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResourceDownload</code> | 使用 <code>ResourceDownload</code> 类型；其定义处的职责：按PUBLIC、MAIN、REVIEW或BRANCH实际引用打开资源，并提供空间配额读取。<br>实际返回表达式：<code>nil</code>；<code>download</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>&quot;document-&quot; + value.Job.DocumentId + &quot;-export-&quot; + value.Job.Id + &quot;.&quot; + extension</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_NOT_READY&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_JOB_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>value.Job.State != &quot;SUCCEEDED&quot; &#124;&#124; value.ArtifactKey == &quot;&quot; &#124;&#124; value.Size == 0 &#124;&#124; value.Size &gt; 32&lt;&lt;20</code>。条件成立时执行 <code>return nil, &quot;&quot;, domain.Violation(domain.StateConflict, &quot;EXPORT_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; edge == nil</code>。条件成立时执行 <code>return nil, &quot;&quot;, domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return nil, &quot;&quot;, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 登记 <code>func() { 		if !accepted { 			download.Close() 		} 	}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>!accepted</code>。
- 检查 <code>actor != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, &quot;&quot;, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, &quot;&quot;, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, &quot;&quot;, err</code>。当前函数在这里结束。

<!-- write-step: 24 bytes=19726:21273 -->
````go
func (s *Exports) Download(ctx context.Context, actor *Actor, id uint64, handler string, bind BindOutput) (*ResourceDownload, string, error) {
	value, admitted, err := s.lookup(ctx, actor, id)
	if err != nil {
		return nil, "", err
	}
	if value.Job.State != "SUCCEEDED" || value.ArtifactKey == "" || value.Size == 0 || value.Size > 32<<20 {
		return nil, "", domain.Violation(domain.StateConflict, "EXPORT_NOT_READY")
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return nil, "", domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	files := s.renderer.Files()
	// 与资源读取相同：共享文件guard覆盖完整校验和实际发送；数据库外IO后再次准入。
	release, err := files.AcquireRead(ctx, value.ArtifactKey)
	if err != nil {
		return nil, "", err
	}
	file, err := files.Open(ctx, value.ArtifactKey, value.Size, value.Checksum)
	if err != nil {
		release()
		return nil, "", err
	}
	download := &ResourceDownload{ID: id, Size: value.Size, Reader: file, release: release}
	accepted := false
	defer func() {
		if !accepted {
			download.Close()
		}
	}()
	if actor != nil {
		if err = s.identity.Revalidate(ctx, actor.Proof); err != nil {
			return nil, "", err
		}
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, admitted.sources); err != nil {
		return nil, "", err
	}
	fixed, err := admitted.check(ctx, tx)
	if err != nil {
````

## 第 25 步：Exports.Download（接着实现）

插入位置：第 24 步代码之后，紧接追加；完成稿约第 622 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, &quot;&quot;, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>current.Job.State != &quot;SUCCEEDED&quot; &#124;&#124; current.Generation != value.Generation &#124;&#124; current.ArtifactKey != value.ArtifactKey &#124;&#124; current.Checksum != value.Checksum &#124;&#124; current.Size != valu …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, &quot;&quot;, domain.Violation(domain.StateConflict, &quot;EXPORT_JOB_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil, &quot;&quot;, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>value.Job.Format == &quot;DOCX&quot;</code>。
- 检查 <code>value.Job.Format == &quot;PDF&quot;</code>。
- 此处通过 <code>return download, &quot;document-&quot; + value.Job.DocumentId + &quot;-export-&quot; + value.Job.Id + &quot;.&quot; + extension, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=21273:22452 -->
````go
		return nil, "", err
	}
	current, err := repository.ExportRow(ctx, tx, id, true)
	if err != nil {
		return nil, "", err
	}
	if current.Job.State != "SUCCEEDED" || current.Generation != value.Generation || current.ArtifactKey != value.ArtifactKey || current.Checksum != value.Checksum || current.Size != value.Size || !current.Expires.After(time.Now().UTC()) {
		return nil, "", domain.Violation(domain.StateConflict, "EXPORT_JOB_CHANGED")
	}
	registration, err := s.register(ctx, tx, actor, fixed, handler, edge)
	if err != nil {
		return nil, "", err
	}
	output := bind(registration, fixed.Sources)
	if err = tx.Commit(); err != nil {
		output.Abort()
		return nil, "", err
	}
	extension := "zip"
	download.MediaType = "application/zip"
	if value.Job.Format == "DOCX" {
		extension = "docx"
		download.MediaType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	if value.Job.Format == "PDF" {
		extension = "pdf"
		download.MediaType = "application/pdf"
	}
	accepted = true
	return download, "document-" + value.Job.DocumentId + "-export-" + value.Job.Id + "." + extension, nil
}

// export_worker.go：保留原实现，按业务合并文件。

````

## 第 26 步：exportGuardKey

插入位置：第 25 步代码之后，紧接追加；完成稿约第 656 行。

本步声明：`func exportGuardKey(value string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>hex.EncodeToString(key&#91;:&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return hex.EncodeToString(key&#91;:&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=22452:22570 -->
````go
func exportGuardKey(value string) string {
	key := sha256.Sum256([]byte(value))
	return hex.EncodeToString(key[:])
}

````

## 第 27 步：exportDocumentGuard

插入位置：第 26 步代码之后，紧接追加；完成稿约第 661 行。

本步声明：`func exportDocumentGuard(document uint64) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>exportGuardKey(&quot;collaboration-report-export-document-v1\n&quot; + domain.TextID(document))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return exportGuardKey(&quot;collaboration-report-export-document-v1\n&quot; + domain.TextID(document))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=22570:22718 -->
````go
func exportDocumentGuard(document uint64) string {
	return exportGuardKey("collaboration-report-export-document-v1\n" + domain.TextID(document))
}

````

## 第 28 步：Exports.Recover

插入位置：第 27 步代码之后，紧接追加；完成稿约第 665 行。

沿用旧全局键共享门，排除旧串行worker；新转换与排空在对应文档门上互斥。
每job另取跨进程排他锁，覆盖实际子进程、临时文件和最终SQL，不靠TTL猜进程退出。

本步声明：`func (s *Exports) Recover(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>files.AcquireRead(ctx, exportGuardKey(&quot;collaboration-report-export-worker-v1&quot;))</code>；<code>s.expireArtifacts(ctx)</code>；<code>repository.NextExports(ctx, s.db)</code>；<code>files.Acquire(ctx, exportGuardKey(&quot;collaboration-report-export-job-v1\n&quot;+domain.TextID(id)))</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, local.ErrBusy)</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>release()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, local.ErrBusy)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>handled &#124;&#124; err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=22718:23774 -->
````go
// 沿用旧全局键共享门，排除旧串行worker；新转换与排空在对应文档门上互斥。
// 每job另取跨进程排他锁，覆盖实际子进程、临时文件和最终SQL，不靠TTL猜进程退出。
func (s *Exports) Recover(ctx context.Context) error {
	files := s.renderer.Files()
	release, err := files.AcquireRead(ctx, exportGuardKey("collaboration-report-export-worker-v1"))
	if errors.Is(err, local.ErrBusy) {
		return nil
	}
	if err != nil {
		return err
	}
	defer release()
	if err = s.expireArtifacts(ctx); err != nil {
		return err
	}
	ids, err := repository.NextExports(ctx, s.db)
	if err != nil {
		return err
	}
	for _, id := range ids {
		unlock, err := files.Acquire(ctx, exportGuardKey("collaboration-report-export-job-v1\n"+domain.TextID(id)))
		if errors.Is(err, local.ErrBusy) {
			continue // 最早的RUNNING仍在转换；后续任务不能被它挡住。
		}
		if err != nil {
			return err
		}
		handled, err := s.recoverJob(ctx, id)
		unlock()
		if handled || err != nil {
			return err
		}
	}
	return nil
}

````

## 第 29 步：Exports.recoverJob

插入位置：第 28 步代码之后，紧接追加；完成稿约第 701 行。

候选查询和取得锁之间可能已有worker完成任务，持锁重读后只处理非终态。

本步声明：`func (s *Exports) recoverJob(ctx context.Context, id uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ExportRow(ctx, s.db, id, false)</code>；<code>s.renderer.Files().AcquireRead(ctx, exportDocumentGuard(document))</code>；<code>s.convertJob(ctx, id, value)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ExportRow(ctx, s.db, id, false)</code>；<code>s.convertJob(ctx, id, value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>s.convertJob(ctx, id, value)</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>value.Job.State != &quot;QUEUED&quot; &amp;&amp; value.Job.State != &quot;RUNNING&quot;</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, local.ErrBusy)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 登记 <code>unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return true, s.convertJob(ctx, id, value)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=23774:24543 -->
````go
// 候选查询和取得锁之间可能已有worker完成任务，持锁重读后只处理非终态。
func (s *Exports) recoverJob(ctx context.Context, id uint64) (bool, error) {
	value, err := repository.ExportRow(ctx, s.db, id, false)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if value.Job.State != "QUEUED" && value.Job.State != "RUNNING" {
		return false, nil
	}
	document, err := domain.CanonicalID(value.Job.DocumentId)
	if err != nil {
		return false, err
	}
	unlock, err := s.renderer.Files().AcquireRead(ctx, exportDocumentGuard(document))
	if errors.Is(err, local.ErrBusy) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer unlock()
	return true, s.convertJob(ctx, id, value)
}

````

## 第 30 步：Exports.convertJob

插入位置：第 29 步代码之后，紧接追加；完成稿约第 728 行。

本步声明：`func (s *Exports) convertJob(ctx context.Context, id uint64, value domain.ExportRecord) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>LockSources(ctx, tx, value.Snapshot.Sources)</code>；<code>s.finishExport(ctx, value, &quot;CANCELLED&quot;, &quot;EXPORT_SOURCE_CLOSED&quot;, &quot;&quot;, 0, &#91;32&#93;byte{})</code>；<code>s.workerAuthority(ctx, tx, value)</code><br>另有 12 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ClaimExport(ctx, tx, id, s.temporaryLimit)</code>；<code>repository.DeferExport(ctx, s.db, id)</code>；<code>repository.FinishExport(ctx, check, id, claimed.Generation, &quot;SUCCEEDED&quot;, &quot;&quot;, artifact, size, checksum)</code> |
| <code>value</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>在此声明中的实际使用：<code>value.Snapshot</code>；<code>s.finishExport(ctx, value, &quot;CANCELLED&quot;, &quot;EXPORT_SOURCE_CLOSED&quot;, &quot;&quot;, 0, &#91;32&#93;byte{})</code>；<code>s.workerAuthority(ctx, tx, value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>s.finishExport(ctx, value, &quot;CANCELLED&quot;, &quot;EXPORT_SOURCE_CLOSED&quot;, &quot;&quot;, 0, &#91;32&#93;byte{})</code>；<code>errors.New(&quot;EXPORT_CLAIM_COMMIT_UNKNOWN&quot;)</code>；<code>s.finishExport(ctx, claimed, &quot;CANCELLED&quot;, &quot;EXPORT_SOURCE_CLOSED&quot;, &quot;&quot;, 0, &#91;32&#93;byte{})</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>exportSourceClosed(err)</code>。条件成立时执行 <code>return s.finishExport(ctx, value, &quot;CANCELLED&quot;, &quot;EXPORT_SOURCE_CLOSED&quot;, &quot;&quot;, 0, &#91;32&#93;byte{})</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>exportSourceClosed(err)</code>。条件成立时执行 <code>return s.finishExport(ctx, value, &quot;CANCELLED&quot;, &quot;EXPORT_SOURCE_CLOSED&quot;, &quot;&quot;, 0, &#91;32&#93;byte{})</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;EXPORT_CLAIM_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>exportSourceClosed(err)</code>。条件成立时执行 <code>return s.finishExport(ctx, claimed, &quot;CANCELLED&quot;, &quot;EXPORT_SOURCE_CLOSED&quot;, &quot;&quot;, 0, &#91;32&#93;byte{})</code>。当前函数在这里结束。

<!-- write-step: 30 bytes=24543:26014 -->
````go
func (s *Exports) convertJob(ctx context.Context, id uint64, value domain.ExportRecord) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	err = LockSources(ctx, tx, value.Snapshot.Sources)
	if err != nil {
		tx.Rollback()
		if exportSourceClosed(err) {
			return s.finishExport(ctx, value, "CANCELLED", "EXPORT_SOURCE_CLOSED", "", 0, [32]byte{})
		}
		return err
	}
	if err = s.workerAuthority(ctx, tx, value); err != nil {
		tx.Rollback()
		if exportSourceClosed(err) {
			return s.finishExport(ctx, value, "CANCELLED", "EXPORT_SOURCE_CLOSED", "", 0, [32]byte{})
		}
		return err
	}
	claimed, err := repository.ClaimExport(ctx, tx, id, s.temporaryLimit)
	if err != nil {
		tx.Rollback()
		_ = repository.DeferExport(ctx, s.db, id)
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("EXPORT_CLAIM_COMMIT_UNKNOWN")
	}
	// 提交后只持文件guard，不跨格式转换持数据库锁。
	document, err := s.exportDocument(ctx, claimed)
	var artifact string
	var size uint64
	var checksum [32]byte
	if err == nil {
		artifact, size, checksum, err = s.renderer.Render(ctx, claimed, document)
	}
	if err != nil {
		if ctx.Err() != nil {
			return err
		} // 进程/父任务退出仍保留RUNNING，让原job恢复。
		if exportSourceClosed(err) {
			return s.finishExport(ctx, claimed, "CANCELLED", "EXPORT_SOURCE_CLOSED", "", 0, [32]byte{})
		}
		var rule domain.RuleViolation
````

## 第 31 步：Exports.convertJob（接着实现）

插入位置：第 30 步代码之后，紧接追加；完成稿约第 773 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;rule)</code>。条件成立时执行 <code>return s.finishExport(ctx, claimed, &quot;REJECTED&quot;, rule.Code, &quot;&quot;, 0, &#91;32&#93;byte{})</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 执行 <code>check.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 执行 <code>check.Rollback()</code>。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>exportSourceClosed(err)</code>。
- 检查 <code>lockErr != nil</code>。条件成立时执行 <code>return lockErr</code>。当前函数在这里结束。
- 检查 <code>removeErr != nil</code>。条件成立时执行 <code>return removeErr</code>。当前函数在这里结束。
- 此处通过 <code>return s.finishExport(ctx, claimed, &quot;CANCELLED&quot;, &quot;EXPORT_SOURCE_CLOSED&quot;, &quot;&quot;, 0, &#91;32&#93;byte{})</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=26014:27259 -->
````go
		if errors.As(err, &rule) {
			return s.finishExport(ctx, claimed, "REJECTED", rule.Code, "", 0, [32]byte{})
		}
		_ = repository.DeferExport(ctx, s.db, id)
		return err
	}
	// 最后重查原来源与当前正式指针。转换时撤权先等待实际进程结束，再删制品并取消。
	check, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	err = LockSources(ctx, check, claimed.Snapshot.Sources)
	if err == nil {
		err = s.workerAuthority(ctx, check, claimed)
	}
	if err == nil {
		err = repository.FinishExport(ctx, check, id, claimed.Generation, "SUCCEEDED", "", artifact, size, checksum)
	}
	if err == nil {
		err = check.Commit()
	} else {
		check.Rollback()
	}
	if err == nil {
		return nil
	}
	if exportSourceClosed(err) {
		files := s.renderer.Files()
		unlock, lockErr := files.Acquire(ctx, artifact)
		if lockErr != nil {
			return lockErr
		}
		removeErr := files.Delete(ctx, artifact)
		unlock()
		if removeErr != nil {
			return removeErr
		}
		return s.finishExport(ctx, claimed, "CANCELLED", "EXPORT_SOURCE_CLOSED", "", 0, [32]byte{})
	}
	// SQL提交未知不能删除可能已SUCCEEDED的原制品或释放预留，下一轮读取原状态。
	return err
}

````

## 第 32 步：exportSourceClosed

插入位置：第 31 步代码之后，紧接追加；完成稿约第 816 行。

本步声明：`func exportSourceClosed(err error) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;rule)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>errors.As(err, &amp;rule) &amp;&amp; (rule.Code == &quot;SOURCE_CLOSED&quot; &#124;&#124; rule.Code == &quot;EXPORT_PUBLIC_NOT_CURRENT&quot; &#124;&#124; rule.Code == &quot;EXPORT_DOCUMENT_CLOSED&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return errors.As(err, &amp;rule) &amp;&amp; (rule.Code == &quot;SOURCE_CLOSED&quot; &#124;&#124; rule.Code == &quot;EXPORT_PUBLIC_NOT_CURRENT&quot; &#124;&#124; rule.Code == &quot;EXPORT_DOCUMENT_CLOSED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=27259:27484 -->
````go
func exportSourceClosed(err error) bool {
	var rule domain.RuleViolation
	return errors.As(err, &rule) && (rule.Code == "SOURCE_CLOSED" || rule.Code == "EXPORT_PUBLIC_NOT_CURRENT" || rule.Code == "EXPORT_DOCUMENT_CLOSED")
}

````

## 第 33 步：Exports.workerAuthority

插入位置：第 32 步代码之后，紧接追加；完成稿约第 821 行。

本步声明：`func (s *Exports) workerAuthority(ctx context.Context, tx *sql.Tx, value domain.ExportRecord) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PublicationFactsRow(ctx, tx, fixed.Document, true)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.PublicationFactsRow(ctx, tx, fixed.Document, true)</code> |
| <code>value</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>在此声明中的实际使用：<code>value.Snapshot</code>；<code>value.Expires</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;EXPORT_DOCUMENT_CLOSED&quot;)</code>；<code>err</code>；<code>nil</code>；<code>domain.Violation(domain.PermissionDenied, &quot;EXPORT_PUBLIC_NOT_CURRENT&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!value.Expires.After(time.Now().UTC())</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;EXPORT_DOCUMENT_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fixed.Target == &quot;MAIN&quot;</code>。
- 检查 <code>facts.Document.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;EXPORT_DOCUMENT_CLOSED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!domain.HasCurrentPublication(facts, policy) &#124;&#124; facts.Document.CurrentReleaseId == nil &#124;&#124; *facts.Document.CurrentReleaseId != domain.TextID(fixed.Release) &#124;&#124; facts.Document.Publica …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;EXPORT_PUBLIC_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=27484:28423 -->
````go
func (s *Exports) workerAuthority(ctx context.Context, tx *sql.Tx, value domain.ExportRecord) error {
	fixed := value.Snapshot
	if !value.Expires.After(time.Now().UTC()) {
		return domain.Violation(domain.PermissionDenied, "EXPORT_DOCUMENT_CLOSED")
	}
	facts, err := repository.PublicationFactsRow(ctx, tx, fixed.Document, true)
	if err != nil {
		return err
	}
	if fixed.Target == "MAIN" {
		if facts.Document.Lifecycle != "ACTIVE" {
			return domain.Violation(domain.PermissionDenied, "EXPORT_DOCUMENT_CLOSED")
		}
		return nil
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return err
	}
	if !domain.HasCurrentPublication(facts, policy) || facts.Document.CurrentReleaseId == nil || *facts.Document.CurrentReleaseId != domain.TextID(fixed.Release) || facts.Document.PublicationEpoch != domain.TextID(fixed.Publication) {
		return domain.Violation(domain.PermissionDenied, "EXPORT_PUBLIC_NOT_CURRENT")
	}
	return nil
}

````

## 第 34 步：Exports.finishExport

插入位置：第 33 步代码之后，紧接追加；完成稿约第 846 行。

本步声明：`func (s *Exports) finishExport(ctx context.Context, value domain.ExportRecord, state, code, key string, size uint64, checksum [32]byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.renderer.Cleanup(ctx, value)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.FinishExport(ctx, tx, id, value.Generation, state, code, key, size, checksum)</code> |
| <code>value</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>在此声明中的实际使用：<code>s.renderer.Cleanup(ctx, value)</code>；<code>value.Job</code>；<code>value.Generation</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>state != &quot;SUCCEEDED&quot;</code>；<code>repository.FinishExport(ctx, tx, id, value.Generation, state, code, key, size, checksum)</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.FinishExport(ctx, tx, id, value.Generation, state, code, key, size, checksum)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.FinishExport(ctx, tx, id, value.Generation, state, code, key, size, checksum)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.FinishExport(ctx, tx, id, value.Generation, state, code, key, size, checksum)</code> |
| <code>checksum</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>repository.FinishExport(ctx, tx, id, value.Generation, state, code, key, size, checksum)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>state != &quot;SUCCEEDED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 34 bytes=28423:29001 -->
````go
func (s *Exports) finishExport(ctx context.Context, value domain.ExportRecord, state, code, key string, size uint64, checksum [32]byte) error {
	if state != "SUCCEEDED" {
		if err := s.renderer.Cleanup(ctx, value); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id, _ := domain.CanonicalID(value.Job.Id)
	if err = repository.FinishExport(ctx, tx, id, value.Generation, state, code, key, size, checksum); err != nil {
		return err
	}
	return tx.Commit()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/exports_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
