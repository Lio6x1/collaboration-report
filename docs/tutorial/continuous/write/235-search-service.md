# 分步编写 `services/document/internal/logic/document/search_service.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/235-search-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

收集搜索候选、生成固定正式版摘要并以整页事务重新鉴权输出。

ES候选和乐观摘要都不是许可，最终任何切版、撤下、撤权都应阻止整页旧内容泄露。

写之前先按这条执行路线理解各段的连接：

1. collectSearchPage逐批评估并保留消费前缀
2. fixedSearchSummaryRow同事务读固定清单
3. commitSearchPage锁联合来源再核全部事实
4. lockedOutputSources把锁证明绑定原tx后登记输出。

本文件需要守住的边界：

- 不能用bool冒称已锁来源
- lookahead共享正文预算与来源
- 候选错误未知时不能静默当过滤项。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/search_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/search_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/search_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/search_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:615 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"google.golang.org/protobuf/proto"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// search_audience.go：保留原实现，按业务合并文件。

````

## 第 2 步：searchAudience

插入位置：第 1 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`type searchAudience struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。 |
| <code>spaces</code> | <code>map&#91;uint64&#93;*pb.IdentityAdmission</code> | 本次搜索已核验的空间准入缓存，键为空间编号。 |
| <code>proofs</code> | <code>map&#91;string&#93;*pb.IdentityAdmission</code> | 本次搜索收集的身份准入证明，按引用去重并供最终交付复核。 |

<!-- write-step: 2 bytes=615:788 -->
````go
type searchAudience struct {
	identity *Identity
	access   *Access
	actor    *Actor
	spaces   map[uint64]*pb.IdentityAdmission
	proofs   map[string]*pb.IdentityAdmission
}

````

## 第 3 步：newSearchAudience

插入位置：第 2 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`func newSearchAudience(identity *Identity, access *Access, actor *Actor) *searchAudience`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor: actor</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*searchAudience</code> | 实际返回表达式：<code>&amp;searchAudience{identity: identity, access: access, actor: actor, spaces: make(map&#91;uint64&#93;*pb.IdentityAdmission), proofs: make(map&#91;string&#93;*pb.IdentityAdmission)}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;searchAudience{identity: identity, access: access, actor: actor, spaces: make(map&#91;uint64&#93;*pb.IdentityAdmission), proofs: make(map&#91;string&#93;*pb.IdentityAdmission)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=788:1052 -->
````go
func newSearchAudience(identity *Identity, access *Access, actor *Actor) *searchAudience {
	return &searchAudience{identity: identity, access: access, actor: actor, spaces: make(map[uint64]*pb.IdentityAdmission), proofs: make(map[string]*pb.IdentityAdmission)}
}

````

## 第 4 步：searchAudience.authorize

插入位置：第 3 步代码之后，紧接追加；完成稿约第 37 行。

本步声明：`func (a *searchAudience) authorize(ctx context.Context, facts domain.PublicationFacts, policy string) (*DocumentAccess, []*pb.SourceEpoch, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>a.access.SpaceIdentity(ctx, *a.actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, &quot;READ_PUBLIC_DOCUMENT&quot;, 0)</code>；<code>a.denied(ctx, err)</code>；<code>a.access.Authorize(ctx, *a.actor, id, &quot;READ_PUBLIC_DOCUMENT&quot;)</code>；<code>a.identity.Revalidate(ctx, proof)</code> |
| <code>facts</code> | <code>domain.PublicationFacts</code> | 使用 <code>domain.PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。<br>在此声明中的实际使用：<code>facts.Space</code>；<code>facts.Document</code>；<code>publicationSources(facts)</code> |
| <code>policy</code> | <code>string</code> | 在此声明中的实际使用：<code>policy != &quot;PUBLIC&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>由 <code>a.denied(ctx, err)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>access</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 由 <code>a.denied(ctx, err)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>a.denied(ctx, err)</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>false</code>；<code>err == nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>a.denied(ctx, err)</code> 的第 4 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>a.actor == nil &amp;&amp; policy != &quot;PUBLIC&quot;</code>。条件成立时执行 <code>return nil, nil, false, nil</code>。当前函数在这里结束。
- 检查 <code>a.actor != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, false, err</code>。当前函数在这里结束。
- 检查 <code>proof == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return a.denied(ctx, err)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, false, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return a.denied(ctx, err)</code>。当前函数在这里结束。
- 检查 <code>value.Document.AclRevision != facts.Document.AclRevision &#124;&#124; value.Document.SpaceId != facts.Document.SpaceId &#124;&#124; optionalText(value.Document.KnowledgeBaseId) != optionalText(facts.Document.KnowledgeBaseId)</code>。条件成立时执行 <code>return nil, nil, false, domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, false, err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=1052:2764 -->
````go
func (a *searchAudience) authorize(ctx context.Context, facts domain.PublicationFacts, policy string) (*DocumentAccess, []*pb.SourceEpoch, bool, error) {
	if a.actor == nil && policy != "PUBLIC" {
		return nil, nil, false, nil
	}
	proofs := []*pb.IdentityAdmission{}
	var access *DocumentAccess
	if a.actor != nil {
		proofs = append(proofs, a.actor.Proof)
		switch policy {
		case "SPACE_MEMBERS":
			space, err := domain.CanonicalID(facts.Space.Id)
			if err != nil {
				return nil, nil, false, err
			}
			proof := a.spaces[space]
			if proof == nil {
				_, proof, err = a.access.SpaceIdentity(ctx, *a.actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, "READ_PUBLIC_DOCUMENT", 0)
				if err != nil {
					return a.denied(ctx, err)
				}
				a.spaces[space] = proof
			}
			proofs = append(proofs, proof)
		case "COLLABORATORS":
			id, err := domain.CanonicalID(facts.Document.Id)
			if err != nil {
				return nil, nil, false, err
			}
			value, err := a.access.Authorize(ctx, *a.actor, id, "READ_PUBLIC_DOCUMENT")
			if err != nil {
				return a.denied(ctx, err)
			}
			// 内部授权读的是当前主线资格；若与候选一致快照已有冲突，中止整页而非输出旧摘要。
			if value.Document.AclRevision != facts.Document.AclRevision || value.Document.SpaceId != facts.Document.SpaceId || optionalText(value.Document.KnowledgeBaseId) != optionalText(facts.Document.KnowledgeBaseId) {
				return nil, nil, false, domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
			}
			access = &value
			proofs = append(proofs, value.Proofs...)
		}
	}
	identitySources, err := MergeSources(proofs...)
	if err != nil {
		return nil, nil, false, err
	}
	for _, proof := range proofs {
````

## 第 5 步：searchAudience.authorize（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 82 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, false, err</code>。当前函数在这里结束。
- 检查 <code>access != nil</code>。
- 检查 <code>err == nil</code>。
- 此处通过 <code>return access, sources, err == nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2764:3270 -->
````go
		if err = a.identity.Revalidate(ctx, proof); err != nil {
			return nil, nil, false, err
		}
	}
	publication, err := publicationSources(facts)
	if err != nil {
		return nil, nil, false, err
	}
	groups := [][]*pb.SourceEpoch{identitySources, publication}
	if access != nil {
		groups = append(groups, access.Sources)
	}
	sources, err := searchSourceUnion(groups...)
	if err == nil {
		for _, proof := range proofs {
			a.proofs[proof.AdmissionId] = proof
		}
	}
	return access, sources, err == nil, err
}

````

## 第 6 步：searchAudience.finalProofs

插入位置：第 5 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`func (a *searchAudience) finalProofs() []*pb.IdentityAdmission`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>result</code> |

**跟着本段代码走一遍：**

- 检查 <code>a.actor != nil</code>。
- 检查 <code>a.actor == nil &#124;&#124; proof.AdmissionId != a.actor.Proof.AdmissionId</code>。
- 此处通过 <code>return result</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3270:3626 -->
````go
func (a *searchAudience) finalProofs() []*pb.IdentityAdmission {
	result := make([]*pb.IdentityAdmission, 0, len(a.proofs)+1)
	if a.actor != nil {
		result = append(result, a.actor.Proof)
	}
	for _, proof := range a.proofs {
		if a.actor == nil || proof.AdmissionId != a.actor.Proof.AdmissionId {
			result = append(result, proof)
		}
	}
	return result
}

````

## 第 7 步：searchAudience.denied

插入位置：第 6 步代码之后，紧接追加；完成稿约第 116 行。

本步声明：`func (a *searchAudience) denied(ctx context.Context, err error) (*DocumentAccess, []*pb.SourceEpoch, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>a.identity.Revalidate(ctx, a.actor.Proof)</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;failure)</code>；<code>err = a.identity.Revalidate(ctx, a.actor.Proof)</code>；<code>err != nil</code>；<code>return nil, nil, false, err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;failure) &amp;&amp; failure.Kind == domain.PermissionDenied</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, false, err</code>。当前函数在这里结束。
- 此处通过 <code>return nil, nil, false, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>errors.As(err, &amp;failure) &amp;&amp; failure.Kind == domain.EntityNotFound</code>。条件成立时执行 <code>return nil, nil, false, domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil, nil, false, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3626:4457 -->
````go
func (a *searchAudience) denied(ctx context.Context, err error) (*DocumentAccess, []*pb.SourceEpoch, bool, error) {
	var failure domain.RuleViolation
	if errors.As(err, &failure) && failure.Kind == domain.PermissionDenied {
		// 同一个PermissionDenied可能来自账户注销或目标资格不足。再次确认真实账户，
		// 只有账户仍有效的确定目标拒绝才能略过候选，不能匿名降级失效会话。
		if err = a.identity.Revalidate(ctx, a.actor.Proof); err != nil {
			return nil, nil, false, err
		}
		return nil, nil, false, nil
	}
	if errors.As(err, &failure) && failure.Kind == domain.EntityNotFound {
		return nil, nil, false, domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
	}
	return nil, nil, false, err
}

// search_candidates.go：保留原实现，按业务合并文件。

````

## 第 8 步：SearchCandidatesPeer

插入位置：第 7 步代码之后，紧接追加；完成稿约第 134 行。

本步声明：`type SearchCandidatesPeer interface {`。

**这一段为什么这样写：** Search仅提供候选；Doc负责最终过滤、片段构建与输出登记。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>SearchCandidates</code> | <code>(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)</code> | ctx与搜索候选请求输入，返回索引候选及游标；候选不等于获准交付结果，Doc还需逐条查权威正文和权限。 |
| <code>CloseCursor</code> | <code>(context.Context, *pb.CloseCursorRequest) (*pb.OperationResult, error)</code> | ctx与原游标关闭请求输入，释放对应搜索上下文，返回操作结果。 |

<!-- write-step: 8 bytes=4457:4680 -->
````go
type SearchCandidatesPeer interface {
	SearchCandidates(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)
	CloseCursor(context.Context, *pb.CloseCursorRequest) (*pb.OperationResult, error)
}

````

## 第 9 步：searchCandidatePeer

插入位置：第 8 步代码之后，紧接追加；完成稿约第 139 行。

扫描器只依赖候选查询，权限/固定内容通过逐批一致读取的评估器接入。
它不能授予阅读权限，也不能把组件测试当真实SQL、RPC或输出验收。

本步声明：`type searchCandidatePeer interface {`。

**这一段为什么这样写：** Search仅提供候选；Doc负责最终过滤、片段构建与输出登记。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>SearchCandidates</code> | <code>(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)</code> | ctx与搜索候选请求输入，返回索引候选及游标；候选不等于获准交付结果，Doc还需逐条查权威正文和权限。 |

<!-- write-step: 9 bytes=4680:5014 -->
````go
// 扫描器只依赖候选查询，权限/固定内容通过逐批一致读取的评估器接入。
// 它不能授予阅读权限，也不能把组件测试当真实SQL、RPC或输出验收。
type searchCandidatePeer interface {
	SearchCandidates(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)
}

````

## 第 10 步：searchPageEntry

插入位置：第 9 步代码之后，紧接追加；完成稿约第 145 行。

本步声明：`type searchPageEntry struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Summary</code> | <code>dto.SearchResult</code> | 从权威固定正文构建的搜索结果标题/片段。 |
| <code>Facts</code> | <code>domain.PublicationFacts</code> | 使用 <code>domain.PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。 |
| <code>Access</code> | <code>*DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。 |
| <code>Sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 此条结果实际依赖的权限来源，最终整页输出时取联合来源。 |

<!-- write-step: 10 bytes=5014:5158 -->
````go
type searchPageEntry struct {
	Summary dto.SearchResult
	Facts   domain.PublicationFacts
	Access  *DocumentAccess
	Sources []*pb.SourceEpoch
}

````

## 第 11 步：searchCollection

插入位置：第 10 步代码之后，紧接追加；完成稿约第 152 行。

本步声明：`type searchCollection struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Entries</code> | <code>&#91;&#93;searchPageEntry</code> | 至多limit+1条；最后一条lookahead也参加最终权限和联合输出复核。 |
| <code>Next</code> | <code>*domain.SearchPageCursor</code> | 使用 <code>domain.SearchPageCursor</code> 类型；其定义处的职责：SearchPageCursor只携带Search的opaque批锚点及Doc已消费前缀；不复制候选/正文。 Until是初次建立链时固定的UnixMillis绝对期限，Seal和Open都不续期。 |
| <code>Until</code> | <code>int64</code> | 当前搜索集合可交付的最晚时间戳，取各准入/游标约束，不能续期。 |

<!-- write-step: 11 bytes=5158:5358 -->
````go
type searchCollection struct {
	// 至多limit+1条；最后一条lookahead也参加最终权限和联合输出复核。
	Entries []searchPageEntry
	Next    *domain.SearchPageCursor
	Until   int64
}

````

## 第 12 步：searchCandidateEvaluator

插入位置：第 11 步代码之后，紧接追加；完成稿约第 159 行。

本步声明：`type searchCandidateEvaluator func(context.Context, *pb.SearchCandidate) (*searchPageEntry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>(匿名/未命名)</code> | <code>*pb.SearchCandidate</code> | 一条Search候选，仍需从Doc权威正文与权限重验。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*searchPageEntry</code> | 可交付的结果条目；nil表示此候选被过滤，不代表已经搜完所有候选。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。 |

<!-- write-step: 12 bytes=5358:5457 -->
````go
type searchCandidateEvaluator func(context.Context, *pb.SearchCandidate) (*searchPageEntry, error)
````

## 第 13 步：searchBatchEvaluator

插入位置：第 12 步代码之后，紧接追加；完成稿约第 160 行。

本步声明：`type searchBatchEvaluator func(context.Context, []*pb.SearchCandidate) (searchCandidateEvaluator, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SearchCandidate</code> | 本批索引候选列表，先批量准备复核所需事实。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>searchCandidateEvaluator</code> | 本批可用的逐条候选复核函数，复用批量准备结果而不省略每条权限检查。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。 |

<!-- write-step: 13 bytes=5457:5563 -->
````go
type searchBatchEvaluator func(context.Context, []*pb.SearchCandidate) (searchCandidateEvaluator, error)

````

## 第 14 步：searchIncomplete

插入位置：第 13 步代码之后，紧接追加；完成稿约第 162 行。

本步声明：`func searchIncomplete(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;failure)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>failure</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;failure)</code>。条件成立时执行 <code>return failure</code>。当前函数在这里结束。
- 此处通过 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=5563:5769 -->
````go
func searchIncomplete(err error) error {
	var failure domain.RuleViolation
	if errors.As(err, &failure) {
		return failure
	}
	return domain.Violation(domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE")
}

````

## 第 15 步：collectSearchPage

插入位置：第 14 步代码之后，紧接追加；完成稿约第 170 行。

本步声明：`func collectSearchPage(ctx context.Context, peer searchCandidatePeer, query domain.SearchQuery, scope *pb.SearchScope, binding [32]byte, start *domain.SearchPageCursor, evaluate searchBatchEvaluator) (searchCollection, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Err</code>；<code>peer.SearchCandidates(ctx, request)</code>；<code>evaluate(ctx, batch.Candidates)</code>；<code>assess(ctx, batch.Candidates&#91;index&#93;)</code> |
| <code>peer</code> | <code>searchCandidatePeer</code> | 在此声明中的实际使用：<code>peer == nil</code>；<code>peer.SearchCandidates</code> |
| <code>query</code> | <code>domain.SearchQuery</code> | 使用 <code>domain.SearchQuery</code> 类型；其定义处的职责：SearchQuery是参数解析后的domain值，不是另一份HTTP/RPC DTO；SpaceID=0表示未限定空间。 HTTP limit属于Doc整页，不能拿它替换Search固定候选批大小。<br>在此声明中的实际使用：<code>query.Limit</code>；<code>query.Text</code> |
| <code>scope</code> | <code>*pb.SearchScope</code> | 在此声明中的实际使用：<code>scope == nil</code>；<code>Scope: scope</code> |
| <code>binding</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>binding == &#91;32&#93;byte{}</code>；<code>start.Binding != binding</code>；<code>Binding: binding</code> |
| <code>start</code> | <code>*domain.SearchPageCursor</code> | 使用 <code>domain.SearchPageCursor</code> 类型；其定义处的职责：SearchPageCursor只携带Search的opaque批锚点及Doc已消费前缀；不复制候选/正文。 Until是初次建立链时固定的UnixMillis绝对期限，Seal和Open都不续期。<br>在此声明中的实际使用：<code>start != nil</code>；<code>start.Binding</code>；<code>start.Until</code>；<code>start.Anchor</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>evaluate</code> | <code>searchBatchEvaluator</code> | 在此声明中的实际使用：<code>evaluate == nil</code>；<code>evaluate(ctx, batch.Candidates)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>searchCollection</code> | 由 <code>fail(nil)</code> 的第 1 个返回项原样转交。<br>由 <code>fail(domain.Violation(domain.StateConflict, &quot;CURSOR_STALE&quot;))</code> 的第 1 个返回项原样转交。<br>由 <code>fail(domain.Violation(domain.StateConflict, &quot;CURSOR_EXPIRED&quot;))</code> 的第 1 个返回项原样转交。<br>由 <code>fail(err)</code> 的第 1 个返回项原样转交。<br>由 <code>fail(domain.Violation(domain.InvalidRequest, &quot;CURSOR_INVALID&quot;))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>page</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>fail(nil)</code> 的第 2 个返回项原样转交。<br>由 <code>fail(domain.Violation(domain.StateConflict, &quot;CURSOR_STALE&quot;))</code> 的第 2 个返回项原样转交。<br>由 <code>fail(domain.Violation(domain.StateConflict, &quot;CURSOR_EXPIRED&quot;))</code> 的第 2 个返回项原样转交。<br>由 <code>fail(err)</code> 的第 2 个返回项原样转交。<br>由 <code>fail(domain.Violation(domain.InvalidRequest, &quot;CURSOR_INVALID&quot;))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return searchCollection{}, searchIncomplete(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>query.Limit &lt; 1 &#124;&#124; query.Limit &gt; 100 &#124;&#124; peer == nil &#124;&#124; evaluate == nil &#124;&#124; scope == nil &#124;&#124; binding == &#91;32&#93;byte{}</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。
- 检查 <code>start != nil</code>。
- 检查 <code>start.Binding != binding</code>。条件成立时执行 <code>return fail(domain.Violation(domain.StateConflict, &quot;CURSOR_STALE&quot;))</code>。当前函数在这里结束。
- 检查 <code>start.Until &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return fail(domain.Violation(domain.StateConflict, &quot;CURSOR_EXPIRED&quot;))</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>until != 0 &amp;&amp; until &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;CURSOR_EXPIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>batch == nil &#124;&#124; !batch.Complete &#124;&#124; len(batch.Candidates) &gt; 100 &#124;&#124; (batch.HasMore &amp;&amp; len(batch.Candidates) != 100) &#124;&#124; batch.ExpiresUnixMs &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=5769:7588 -->
````go
func collectSearchPage(ctx context.Context, peer searchCandidatePeer, query domain.SearchQuery, scope *pb.SearchScope, binding [32]byte, start *domain.SearchPageCursor, evaluate searchBatchEvaluator) (searchCollection, error) {
	fail := func(err error) (searchCollection, error) { return searchCollection{}, searchIncomplete(err) }
	if query.Limit < 1 || query.Limit > 100 || peer == nil || evaluate == nil || scope == nil || binding == [32]byte{} {
		return fail(nil)
	}
	digest := sha256.Sum256([]byte(query.Text))
	request := &pb.SearchCandidatesRequest{Query: query.Text, QueryDigest: hex.EncodeToString(digest[:]), Scope: scope, PageSize: 100}
	var until int64
	prefix := 0
	if start != nil {
		if start.Binding != binding {
			return fail(domain.Violation(domain.StateConflict, "CURSOR_STALE"))
		}
		if start.Until <= time.Now().UnixMilli() {
			return fail(domain.Violation(domain.StateConflict, "CURSOR_EXPIRED"))
		}
		var err error
		request.Cursor, err = domain.EnvelopeSearchAnchor(start.Anchor, true)
		if err != nil {
			return fail(err)
		}
		until, prefix = start.Until, int(start.Prefix)
	}
	page := searchCollection{Entries: make([]searchPageEntry, 0, query.Limit+1)}
	var physical string
	var generation uint64
	checkDeadline := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if until != 0 && until <= time.Now().UnixMilli() {
			return domain.Violation(domain.StateConflict, "CURSOR_EXPIRED")
		}
		return nil
	}
	for {
		if err := checkDeadline(); err != nil {
			return fail(err)
		}
		batch, err := peer.SearchCandidates(ctx, request)
		if err != nil {
			return fail(err)
		}
		if batch == nil || !batch.Complete || len(batch.Candidates) > 100 || (batch.HasMore && len(batch.Candidates) != 100) || batch.ExpiresUnixMs <= time.Now().UnixMilli() {
			return fail(nil)
		}
````

## 第 16 步：collectSearchPage（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 216 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>until == 0</code>。
- 检查 <code>physical == &quot;&quot;</code>。
- 检查 <code>until != batch.ExpiresUnixMs &#124;&#124; physical != batch.PhysicalIndex &#124;&#124; generation != batch.IndexGeneration</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。
- 检查 <code>prefix &gt; len(batch.Candidates) &#124;&#124; (start != nil &amp;&amp; prefix == len(batch.Candidates))</code>。条件成立时执行 <code>return fail(domain.Violation(domain.InvalidRequest, &quot;CURSOR_INVALID&quot;))</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; assess == nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>entry == nil</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>len(page.Entries) == query.Limit+1</code>。
- 此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!batch.HasMore</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。

<!-- write-step: 16 bytes=7588:9335 -->
````go
		if until == 0 {
			until = batch.ExpiresUnixMs
			physical, generation = batch.PhysicalIndex, batch.IndexGeneration
		}
		if physical == "" {
			physical, generation = batch.PhysicalIndex, batch.IndexGeneration
		}
		page.Until = until
		// 同链的绝对期限、物理索引和代次不可随批次变化；原HTTP期限也不被延长。
		if until != batch.ExpiresUnixMs || physical != batch.PhysicalIndex || generation != batch.IndexGeneration {
			return fail(nil)
		}
		if prefix > len(batch.Candidates) || (start != nil && prefix == len(batch.Candidates)) {
			return fail(domain.Violation(domain.InvalidRequest, "CURSOR_INVALID"))
		}
		assess, err := evaluate(ctx, batch.Candidates)
		if err != nil || assess == nil {
			return fail(err)
		}
		for index := prefix; index < len(batch.Candidates); index++ {
			if err = ctx.Err(); err != nil {
				return fail(err)
			}
			entry, err := assess(ctx, batch.Candidates[index])
			if err != nil {
				return fail(err)
			}
			// 正文与身份检查可能耗完原期限；最后一项也须复核，不能靠下一轮循环兜底。
			if err = checkDeadline(); err != nil {
				return fail(err)
			}
			if entry == nil {
				continue
			}
			page.Entries = append(page.Entries, *entry)
			if len(page.Entries) == query.Limit+1 {
				// Prefix停在lookahead之前，而不是批尾。终批HasMore=false仍须保留锚点。
				// 下一页REPLAY并重新核验这个候选，不能复用本页旧权限或跳过它。
				page.Next = &domain.SearchPageCursor{Anchor: append([]byte(nil), batch.Continuation...), Prefix: uint8(index), Binding: binding, Until: until}
				return page, nil
			}
		}
		if !batch.HasMore {
			if err = checkDeadline(); err != nil {
				return fail(err)
			}
````

## 第 17 步：collectSearchPage（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 262 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。

<!-- write-step: 17 bytes=9335:9766 -->
````go
			return page, nil
		}
		request.Cursor, err = domain.EnvelopeSearchAnchor(batch.Continuation, false)
		if err != nil {
			return fail(err)
		}
		prefix = 0
		start = nil
		// 不设1000候选的静默截断。只有找到lookahead或真实EOF才能成功，
		// 其余情况由原请求期限失败；返回的空结构保证未知结果不泄露半页。
	}
}

// search_commit.go：保留原实现，按业务合并文件。

````

## 第 18 步：SearchPages.commitSearchPage

插入位置：第 17 步代码之后，紧接追加；完成稿约第 277 行。

本步声明：`func (s *SearchPages) commitSearchPage(ctx context.Context, actor *Actor, entries []searchPageEntry, proofs []*pb.IdentityAdmission, until int64, handler string, bind BindOutput) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Revalidate(ctx, proof)</code>；<code>ctx.Err</code>；<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor != nil</code>；<code>actor.SessionExpiresUnixMs</code>；<code>actor.Proof</code> |
| <code>entries</code> | <code>&#91;&#93;searchPageEntry</code> | 在此声明中的实际使用：<code>len(entries)</code>；<code>append(&#91;&#93;searchPageEntry(nil), entries...)</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 本页全部候选及lookahead使用的身份准入证明，提交输出前在原事务中统一复核。 |
| <code>until</code> | <code>int64</code> | 在此声明中的实际使用：<code>until &lt;= now</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>handler == &quot;&quot;</code>；<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.outputs.registerLockedSearch(ctx, locked, handler, edge)</code> |
| <code>bind</code> | <code>BindOutput</code> | 使用 <code>BindOutput</code> 类型；其定义处的职责：BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。<br>在此声明中的实际使用：<code>bind == nil</code>；<code>bind(registration, locked.copySources())</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_REQUIRED&quot;)</code>；<code>searchIncomplete(nil)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_OUTPUT_SOURCES_REQUIRED&quot;)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>handler == &quot;&quot; &#124;&#124; bind == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ExpiresUnixMs &lt;= 0</code>。条件成立时执行 <code>return searchIncomplete(nil)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>proofUntil == 0 &#124;&#124; proof.ExpiresUnixMs &lt; proofUntil</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return searchIncomplete(err)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>until &lt;= now</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;CURSOR_EXPIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>actor != nil &amp;&amp; actor.SessionExpiresUnixMs &lt;= now</code>。条件成立时执行 <code>return domain.Violation(domain.SessionRequired, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>proofUntil != 0 &amp;&amp; proofUntil &lt;= now</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>actor != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=9766:11211 -->
````go
func (s *SearchPages) commitSearchPage(ctx context.Context, actor *Actor, entries []searchPageEntry, proofs []*pb.IdentityAdmission, until int64, handler string, bind BindOutput) error {
	if handler == "" || bind == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_REQUIRED")
	}
	groups := make([][]*pb.SourceEpoch, 0, len(entries)+1)
	var proofUntil int64
	for _, proof := range proofs {
		if proof == nil || proof.ExpiresUnixMs <= 0 {
			return searchIncomplete(nil)
		}
		if err := s.identity.Revalidate(ctx, proof); err != nil {
			return err
		}
		if proofUntil == 0 || proof.ExpiresUnixMs < proofUntil {
			proofUntil = proof.ExpiresUnixMs
		}
	}
	checkDeadline := func() error {
		if err := ctx.Err(); err != nil {
			return searchIncomplete(err)
		}
		now := time.Now().UnixMilli()
		if until <= now {
			return domain.Violation(domain.StateConflict, "CURSOR_EXPIRED")
		}
		if actor != nil && actor.SessionExpiresUnixMs <= now {
			return domain.Violation(domain.SessionRequired, "SESSION_EXPIRED")
		}
		if proofUntil != 0 && proofUntil <= now {
			return domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
		}
		return nil
	}
	if err := checkDeadline(); err != nil {
		return err
	}
	if actor != nil {
		sources, err := MergeSources(actor.Proof)
		if err != nil {
			return err
		}
		groups = append(groups, sources)
	}
	for _, entry := range entries {
		groups = append(groups, entry.Sources)
	}
````

## 第 19 步：SearchPages.commitSearchPage（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 323 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(sources) == 0</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_OUTPUT_SOURCES_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; edge == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return searchFinalChange(err)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return left &lt; right</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return searchFinalChange(err)</code>。当前函数在这里结束。
- 检查 <code>publicFactsBinding(current) != publicFactsBinding(entry.Facts)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code>。当前函数在这里结束。
- 检查 <code>entry.Access != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return searchFinalChange(err)</code>。当前函数在这里结束。

<!-- write-step: 19 bytes=11211:12867 -->
````go
	sources, err := searchSourceUnion(groups...)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return domain.Violation(domain.TemporarilyUnavailable, "SEARCH_OUTPUT_SOURCES_REQUIRED")
	}
	// 所有跨进程RPC/Edge检查在SQL锁外。两个进程登记各自真实handler/instance，
	// 使用相同完整来源并集，而非将多文档页伪装成某个Doc的准入。
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	locked, err := lockSearchOutputSources(ctx, tx, sources)
	if err != nil {
		return searchFinalChange(err)
	}
	ordered := append([]searchPageEntry(nil), entries...)
	sort.Slice(ordered, func(i, j int) bool {
		left, _ := domain.CanonicalID(ordered[i].Facts.Document.Id)
		right, _ := domain.CanonicalID(ordered[j].Facts.Document.Id)
		return left < right
	})
	for _, entry := range ordered {
		id, err := domain.CanonicalID(entry.Facts.Document.Id)
		if err != nil {
			return searchFinalChange(err)
		}
		current, err := repository.PublicationFactsRow(ctx, tx, id, true)
		if err != nil {
			return searchFinalChange(err)
		}
		if publicFactsBinding(current) != publicFactsBinding(entry.Facts) {
			return domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
		}
		if entry.Access != nil {
			if _, err = ValidateAccess(ctx, tx, *entry.Access); err != nil {
				return searchFinalChange(err)
			}
			// responsible字段不是只有nil/non-nil之分；保留确切资格绑定，不靠旧快照推断。
````

## 第 20 步：SearchPages.commitSearchPage（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 368 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>optionalText(current.Document.ResponsibleUserId) != optionalText(entry.Access.Document.ResponsibleUserId)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>output == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_OUTPUT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;OUTPUT_COMMIT_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=12867:13760 -->
````go
			if optionalText(current.Document.ResponsibleUserId) != optionalText(entry.Access.Document.ResponsibleUserId) {
				return domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
			}
		}
	}
	registration, err := s.outputs.registerLockedSearch(ctx, locked, handler, edge)
	if err != nil {
		return err
	}
	output := bind(registration, locked.copySources())
	if output == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "SEARCH_OUTPUT_BINDING_INVALID")
	}
	// 锁等待、Edge及输出绑定也在原期限内；SQL内不发RPC，只核验已验证证明的绝对期限。
	if err = checkDeadline(); err != nil {
		output.Abort()
		return err
	}
	if err = tx.Commit(); err != nil {
		output.Abort()
		return domain.Violation(domain.TemporarilyUnavailable, "OUTPUT_COMMIT_UNKNOWN")
	}
	if err = checkDeadline(); err != nil {
		output.Abort()
		return err
	}
	return nil
}

````

## 第 21 步：searchFinalChange

插入位置：第 20 步代码之后，紧接追加；完成稿约第 397 行。

本步声明：`func searchFinalChange(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;failure)</code>；<code>searchIncomplete(err)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code>；<code>searchIncomplete(err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;failure) &amp;&amp; (failure.Kind == domain.StateConflict &#124;&#124; failure.Kind == domain.EntityNotFound &#124;&#124; failure.Kind == domain.PermissionDenied)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return searchIncomplete(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=13760:14164 -->
````go
func searchFinalChange(err error) error {
	var failure domain.RuleViolation
	if errors.As(err, &failure) && (failure.Kind == domain.StateConflict || failure.Kind == domain.EntityNotFound || failure.Kind == domain.PermissionDenied) {
		return domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
	}
	return searchIncomplete(err)
}

// search_fixed.go：保留原实现，按业务合并文件。

````

## 第 22 步：SearchPages.fixedSearchSummary

插入位置：第 21 步代码之后，紧接追加；完成稿约第 407 行。

本步声明：`func (s *SearchPages) fixedSearchSummary(ctx context.Context, facts domain.PublicationFacts, release uint64, query string, used uint64) (dto.SearchResult, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})</code>；<code>s.fixedSearchSummaryRow(ctx, tx, facts, release, query, used)</code> |
| <code>facts</code> | <code>domain.PublicationFacts</code> | 使用 <code>domain.PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。<br>在此声明中的实际使用：<code>s.fixedSearchSummaryRow(ctx, tx, facts, release, query, used)</code> |
| <code>release</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.fixedSearchSummaryRow(ctx, tx, facts, release, query, used)</code> |
| <code>query</code> | <code>string</code> | 在此声明中的实际使用：<code>s.fixedSearchSummaryRow(ctx, tx, facts, release, query, used)</code> |
| <code>used</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.fixedSearchSummaryRow(ctx, tx, facts, release, query, used)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.SearchResult</code> | 实际返回表达式：<code>dto.SearchResult{}</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>next</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return dto.SearchResult{}, 0, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return dto.SearchResult{}, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return value, next, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=14164:14753 -->
````go
func (s *SearchPages) fixedSearchSummary(ctx context.Context, facts domain.PublicationFacts, release uint64, query string, used uint64) (dto.SearchResult, uint64, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return dto.SearchResult{}, 0, err
	}
	defer tx.Rollback()
	value, next, err := s.fixedSearchSummaryRow(ctx, tx, facts, release, query, used)
	if err != nil {
		return dto.SearchResult{}, 0, err
	}
	if err = tx.Commit(); err != nil {
		return dto.SearchResult{}, 0, err
	}
	return value, next, nil
}

````

## 第 23 步：SearchPages.fixedSearchSummaryRow

插入位置：第 22 步代码之后，紧接追加；完成稿约第 423 行。

固定清单与正文来自同一只读事务；它不持有权限锁。最终整页事务会重新核验
当前正式指针与联合来源，期间任何切版/撤下都不能输出这个乐观摘要。

本步声明：`func (s *SearchPages) fixedSearchSummaryRow(ctx context.Context, tx *sql.Tx, facts domain.PublicationFacts, release uint64, query string, used uint64) (dto.SearchResult, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.FixedReleaseRow(ctx, tx, id, release)</code>；<code>repository.StructureRow(ctx, tx, fixed.Structure, facts.Document.Id)</code>；<code>repository.StyleRow(ctx, tx, fixed.Style, facts.Document.Id)</code>；<code>repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.FixedReleaseRow(ctx, tx, id, release)</code>；<code>repository.StructureRow(ctx, tx, fixed.Structure, facts.Document.Id)</code>；<code>repository.StyleRow(ctx, tx, fixed.Style, facts.Document.Id)</code>；<code>repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>facts</code> | <code>domain.PublicationFacts</code> | 使用 <code>domain.PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。<br>在此声明中的实际使用：<code>facts.Document</code> |
| <code>release</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.FixedReleaseRow(ctx, tx, id, release)</code>；<code>domain.TextID(release)</code> |
| <code>query</code> | <code>string</code> | 在此声明中的实际使用：<code>strings.ToLower(query)</code> |
| <code>used</code> | <code>uint64</code> | 在此声明中的实际使用：<code>next := used</code>；<code>domain.ReserveSearchBody(used, sizes)</code>；<code>used &gt; domain.SearchPageBodyLimit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.SearchResult</code> | 由 <code>fail(err)</code> 的第 1 个返回项原样转交。<br>由 <code>fail(nil)</code> 的第 1 个返回项原样转交。<br>由 <code>fail(domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>fail(err)</code> 的第 2 个返回项原样转交。<br>由 <code>fail(nil)</code> 的第 2 个返回项原样转交。<br>由 <code>fail(domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>next</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>fail(err)</code> 的第 3 个返回项原样转交。<br>由 <code>fail(nil)</code> 的第 3 个返回项原样转交。<br>由 <code>fail(domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;))</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err == nil</code>。
- 在本段的匿名函数内：此处通过 <code>return dto.SearchResult{}, 0, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>fixed.Release.DocumentId != facts.Document.Id &#124;&#124; fixed.Release.Id != domain.TextID(release) &#124;&#124; !utf8.ValidString(fixed.Title) &#124;&#124; utf8.RuneCountInString(fixed.Title) &gt; 240</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>structure.SchemaVersion != &quot;structure-v1&quot; &#124;&#124; style.SchemaVersion != &quot;style-v1&quot; &#124;&#124; len(structure.Entries) &gt; domain.SearchManifestMaxUnits</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。

<!-- write-step: 23 bytes=14753:16588 -->
````go
// 固定清单与正文来自同一只读事务；它不持有权限锁。最终整页事务会重新核验
// 当前正式指针与联合来源，期间任何切版/撤下都不能输出这个乐观摘要。
func (s *SearchPages) fixedSearchSummaryRow(ctx context.Context, tx *sql.Tx, facts domain.PublicationFacts, release uint64, query string, used uint64) (dto.SearchResult, uint64, error) {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "SEARCH_FIXED_CONTENT_INVALID")
	fail := func(err error) (dto.SearchResult, uint64, error) {
		if err == nil {
			err = invalid
		}
		return dto.SearchResult{}, 0, err
	}
	id, err := domain.CanonicalID(facts.Document.Id)
	if err != nil {
		return fail(err)
	}
	fixed, err := repository.FixedReleaseRow(ctx, tx, id, release)
	if err != nil {
		return fail(err)
	}
	if fixed.Release.DocumentId != facts.Document.Id || fixed.Release.Id != domain.TextID(release) || !utf8.ValidString(fixed.Title) || utf8.RuneCountInString(fixed.Title) > 240 {
		return fail(nil)
	}
	structure, err := repository.StructureRow(ctx, tx, fixed.Structure, facts.Document.Id)
	if err != nil {
		return fail(err)
	}
	style, err := repository.StyleRow(ctx, tx, fixed.Style, facts.Document.Id)
	if err != nil {
		return fail(err)
	}
	if structure.SchemaVersion != "structure-v1" || style.SchemaVersion != "style-v1" || len(structure.Entries) > domain.SearchManifestMaxUnits {
		return fail(nil)
	}
	units, err := repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)
	if err != nil {
		return fail(err)
	}
	if err = validateReadingUnits(structure, units); err != nil {
		return fail(nil)
	}
	manifest, err := domain.CanonicalID(fixed.Release.ManifestId)
	if err != nil {
		return fail(err)
	}
	metadata, err := repository.SearchManifestMetadata(ctx, tx, manifest, id, len(units))
	if err != nil {
````

## 第 24 步：SearchPages.fixedSearchSummaryRow（接着实现）

插入位置：第 23 步代码之后，紧接追加；完成稿约第 468 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return fail(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!present &#124;&#124; domain.TextID(meta.Version) != unit&#91;&quot;content_version_id&quot;&#93;</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。
- 检查 <code>len(metadata) &gt; 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>used &gt; domain.SearchPageBodyLimit</code>。条件成立时执行 <code>return fail(domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;))</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>entry.UnitId == nil</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>seen&#91;*entry.UnitId&#93;</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。
- 检查 <code>!present</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。

<!-- write-step: 24 bytes=16588:18072 -->
````go
		return fail(err)
	}
	sizes := make([]uint64, 0, len(metadata))
	byUnit := make(map[string]domain.SearchManifestUnit, len(metadata))
	for _, unit := range metadata {
		sizes = append(sizes, unit.ByteSize)
		byUnit[domain.TextID(unit.Unit)] = unit
	}
	for _, unit := range units {
		meta, present := byUnit[unit["unit_id"]]
		if !present || domain.TextID(meta.Version) != unit["content_version_id"] {
			return fail(nil)
		}
	}
	next := used
	if len(metadata) > 0 {
		// 整篇元数据占额发生在任何VersionContent正文读取前；lookahead共用此累计预算。
		// 空正式结构经上面的完整归属检查，没有正文可占额，不伪造零字节版本。
		next, err = domain.ReserveSearchBody(used, sizes)
		if err != nil {
			return fail(err)
		}
	} else if used > domain.SearchPageBodyLimit {
		return fail(domain.Violation(domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE"))
	}
	structureID, err := domain.CanonicalID(fixed.Structure)
	if err != nil {
		return fail(err)
	}
	value := dto.SearchResult{DocumentId: facts.Document.Id, ReleaseId: fixed.Release.Id, Title: fixed.Title}
	matched := false
	seen := make(map[string]bool, len(metadata))
	for _, entry := range structure.Entries {
		if entry.UnitId == nil {
			continue
		}
		if seen[*entry.UnitId] {
			return fail(nil)
		}
		seen[*entry.UnitId] = true
		if _, err = domain.CanonicalID(entry.ChapterId); err != nil {
			return fail(nil)
		}
		unit, present := byUnit[*entry.UnitId]
		if !present {
````

## 第 25 步：SearchPages.fixedSearchSummaryRow（接着实现）

插入位置：第 24 步代码之后，紧接追加；完成稿约第 513 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return fail(nil)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>(!matched &amp;&amp; match) &#124;&#124; (value.Snippet == &quot;&quot; &amp;&amp; !matched &amp;&amp; text != &quot;&quot;)</code>。
- 检查 <code>len(seen) != len(metadata)</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。
- 此处通过 <code>return value, next, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=18072:18722 -->
````go
			return fail(nil)
		}
		text, err := s.searchUnitText(ctx, tx, id, structureID, unit)
		if err != nil {
			return fail(err)
		}
		match := strings.Contains(strings.ToLower(text), strings.ToLower(query)) || strings.Contains(strings.ToLower(entry.Title), strings.ToLower(query))
		if (!matched && match) || (value.Snippet == "" && !matched && text != "") {
			value.Snippet = text
			chapter := entry.ChapterId
			value.ChapterId = &chapter
			matched = match
		}
		// 达到摘要2000字符后仍验证全部剩余单位；损坏后缀不能成为部分成功。
	}
	if len(seen) != len(metadata) {
		return fail(nil)
	}
	return value, next, nil
}

````

## 第 26 步：SearchPages.searchUnitText

插入位置：第 25 步代码之后，紧接追加；完成稿约第 534 行。

本步声明：`func (s *SearchPages) searchUnitText(ctx context.Context, tx *sql.Tx, document, structure uint64, unit domain.SearchManifestUnit) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, unit.Version, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, unit.Version, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, unit.Version, document)</code>；<code>validateFixedMergeContent(s.validator, content, document, unit.Unit, structure)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>validateFixedMergeContent(s.validator, content, document, unit.Unit, structure)</code> |
| <code>unit</code> | <code>domain.SearchManifestUnit</code> | 使用 <code>domain.SearchManifestUnit</code> 类型；其定义处的职责：SearchManifestUnit是Doc私有固定清单的元数据；ByteSize用于读取正文之前预留预算。 它不包含正文/摘要或RPC DTO，读取后仍须验证实际字节、hash、schema与完整单位绑定。<br>在此声明中的实际使用：<code>unit.Version</code>；<code>unit.ByteSize</code>；<code>unit.Unit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>text</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 登记 <code>func() { clear(content.BodyJsonUtf8); clear(content.ProvenanceJsonUtf8) }()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>content.ByteSize != unit.ByteSize &#124;&#124; uint64(len(content.BodyJsonUtf8)) != unit.ByteSize</code>。条件成立时执行 <code>return &quot;&quot;, invalid</code>。当前函数在这里结束。
- 检查 <code>canonicalVersion(content, unit.Version) != nil &#124;&#124; validateFixedMergeContent(s.validator, content, document, unit.Unit, structure) != nil</code>。条件成立时执行 <code>return &quot;&quot;, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return text, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=18722:20122 -->
````go
func (s *SearchPages) searchUnitText(ctx context.Context, tx *sql.Tx, document, structure uint64, unit domain.SearchManifestUnit) (string, error) {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "SEARCH_FIXED_CONTENT_INVALID")
	content, err := repository.VersionContent(ctx, tx, unit.Version, document)
	if err != nil {
		return "", err
	}
	// 单个固定单元只在此函数持有，完成后清除原字节并释放树，不装配PublicDocument.units。
	defer func() { clear(content.BodyJsonUtf8); clear(content.ProvenanceJsonUtf8) }()
	if content.ByteSize != unit.ByteSize || uint64(len(content.BodyJsonUtf8)) != unit.ByteSize {
		return "", invalid
	}
	// canonical_body是BLOB，必须保留并验证原规范字节/hash；provenance_json是MySQL
	// JSON列，只保存语义，会重排键并添加空白。先沿既有读取规则规范化它，再核完整
	// schema/资源/绑定，不能把MySQL正常的JSON词法改写误判成正文损坏。
	if canonicalVersion(content, unit.Version) != nil || validateFixedMergeContent(s.validator, content, document, unit.Unit, structure) != nil {
		return "", invalid
	}
	body, err := codec.Decode(content.BodyJsonUtf8)
	if err != nil {
		return "", invalid
	}
	text, err := searchSnippet(body)
	if err != nil {
		return "", invalid
	}
	return text, nil
}

// search_output_sources.go：保留原实现，按业务合并文件。

````

## 第 27 步：lockedOutputSources

插入位置：第 26 步代码之后，紧接追加；完成稿约第 564 行。

仅搜索最终事务内部拥有此证明；不能以bool声明已锁定，不能换tx或来源。
持证明后的事实复核只读，不修改gate/transition；原tx结束即失效。

本步声明：`type lockedOutputSources struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。 |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在当前事务已锁定核验的整页来源集合。 |
| <code>consumed</code> | <code>bool</code> | 标记此锁定结果是否已被一次输出消费，防止在不同事务或二次输出中复用。 |

<!-- write-step: 27 bytes=20122:20413 -->
````go
// 仅搜索最终事务内部拥有此证明；不能以bool声明已锁定，不能换tx或来源。
// 持证明后的事实复核只读，不修改gate/transition；原tx结束即失效。
type lockedOutputSources struct {
	tx       *sql.Tx
	sources  []*pb.SourceEpoch
	consumed bool
}

````

## 第 28 步：lockedOutputSources.copySources

插入位置：第 27 步代码之后，紧接追加；完成稿约第 572 行。

本步声明：`func (p *lockedOutputSources) copySources() []*pb.SourceEpoch`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>values</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return values</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=20413:20637 -->
````go
func (p *lockedOutputSources) copySources() []*pb.SourceEpoch {
	values := make([]*pb.SourceEpoch, len(p.sources))
	for i, source := range p.sources {
		values[i] = proto.Clone(source).(*pb.SourceEpoch)
	}
	return values
}

````

## 第 29 步：lockSearchOutputSources

插入位置：第 28 步代码之后，紧接追加；完成稿约第 580 行。

本步声明：`func lockSearchOutputSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch) (*lockedOutputSources, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockSourceGatesBatch(ctx, tx, SourcesWire(proof.sources))</code>；<code>repository.PendingSourceTransitionsBatch(ctx, tx, documentKeys)</code>；<code>repository.ReopenSourceGate(ctx, tx, source.ScopeKey, source.Epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx == nil</code>；<code>tx: tx</code>；<code>repository.LockSourceGatesBatch(ctx, tx, SourcesWire(proof.sources))</code>；<code>repository.PendingSourceTransitionsBatch(ctx, tx, documentKeys)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>len(sources)</code>；<code>sources: make(&#91;&#93;*pb.SourceEpoch, len(sources))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*lockedOutputSources</code> | 实际返回表达式：<code>nil</code>；<code>proof</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SOURCE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;SCOPE_TRANSITION_PENDING&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;SOURCE_CLOSED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>tx == nil &#124;&#124; len(sources) == 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;SOURCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>source == nil &#124;&#124; source.Epoch == 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;SOURCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return proof.sources&#91;i&#93;.ScopeKey &lt; proof.sources&#91;j&#93;.ScopeKey</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>platform.ValidDocumentScope(source.ScopeKey)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;SCOPE_TRANSITION_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>gate.Epoch &gt; source.Epoch &#124;&#124; (gate.Epoch == source.Epoch &amp;&amp; gate.State == &quot;BLOCKED&quot;)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;SOURCE_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>gate.Epoch &lt; source.Epoch</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return proof, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=20637:22217 -->
````go
func lockSearchOutputSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch) (*lockedOutputSources, error) {
	if tx == nil || len(sources) == 0 {
		return nil, errors.New("SOURCE_BINDING_INVALID")
	}
	proof := &lockedOutputSources{tx: tx, sources: make([]*pb.SourceEpoch, len(sources))}
	for i, source := range sources {
		if source == nil || source.Epoch == 0 {
			return nil, errors.New("SOURCE_BINDING_INVALID")
		}
		proof.sources[i] = proto.Clone(source).(*pb.SourceEpoch)
	}
	sort.Slice(proof.sources, func(i, j int) bool { return proof.sources[i].ScopeKey < proof.sources[j].ScopeKey })
	gates, err := repository.LockSourceGatesBatch(ctx, tx, SourcesWire(proof.sources))
	if err != nil {
		return nil, err
	}
	documentKeys := make([]string, 0, len(proof.sources))
	for _, source := range proof.sources {
		if platform.ValidDocumentScope(source.ScopeKey) {
			documentKeys = append(documentKeys, source.ScopeKey)
		}
	}
	pending, err := repository.PendingSourceTransitionsBatch(ctx, tx, documentKeys)
	if err != nil {
		return nil, err
	}
	if pending {
		return nil, domain.Violation(domain.StateConflict, "SCOPE_TRANSITION_PENDING")
	}
	for _, source := range proof.sources {
		gate := gates[source.ScopeKey]
		if gate.Epoch > source.Epoch || (gate.Epoch == source.Epoch && gate.State == "BLOCKED") {
			return nil, domain.Violation(domain.PermissionDenied, "SOURCE_CLOSED")
		}
		if gate.Epoch < source.Epoch {
			if err := repository.ReopenSourceGate(ctx, tx, source.ScopeKey, source.Epoch); err != nil {
				return nil, err
			}
		}
	}
	return proof, nil
}

````

## 第 30 步：Outputs.registerLockedSearch

插入位置：第 29 步代码之后，紧接追加；完成稿约第 623 行。

本步声明：`func (m *Outputs) registerLockedSearch(ctx context.Context, proof *lockedOutputSources, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>m.writeResponseRegistration(ctx, proof.tx, proof.copySources(), domain.OutputLease{}, handler, edge)</code> |
| <code>proof</code> | <code>*lockedOutputSources</code> | 在此声明中的实际使用：<code>proof == nil</code>；<code>proof.tx</code>；<code>proof.consumed</code>；<code>proof.sources</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>handler == &quot;&quot;</code>；<code>m.writeResponseRegistration(ctx, proof.tx, proof.copySources(), domain.OutputLease{}, handler, edge)</code> |
| <code>edge</code> | <code>*domain.OutputInstance</code> | 使用 <code>domain.OutputInstance</code> 类型；其定义处的职责：输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。<br>在此声明中的实际使用：<code>edge == nil</code>；<code>m.writeResponseRegistration(ctx, proof.tx, proof.copySources(), domain.OutputLease{}, handler, edge)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResponseRegistration</code> | 使用 <code>ResponseRegistration</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。<br>由 <code>m.writeResponseRegistration(ctx, proof.tx, proof.copySources(), domain.OutputLease{}, handler, edge)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>m.writeResponseRegistration(ctx, proof.tx, proof.copySources(), domain.OutputLease{}, handler, edge)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_OUTPUT_PROOF_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>proof == nil &#124;&#124; proof.tx == nil &#124;&#124; proof.consumed &#124;&#124; len(proof.sources) == 0 &#124;&#124; handler == &quot;&quot; &#124;&#124; edge == nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_OUTPUT_PROOF_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return m.writeResponseRegistration(ctx, proof.tx, proof.copySources(), domain.OutputLease{}, handler, edge)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=22217:22836 -->
````go
func (m *Outputs) registerLockedSearch(ctx context.Context, proof *lockedOutputSources, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	if proof == nil || proof.tx == nil || proof.consumed || len(proof.sources) == 0 || handler == "" || edge == nil {
		return nil, errors.New("SEARCH_OUTPUT_PROOF_INVALID")
	}
	proof.consumed = true
	// SQL仍在原tx执行；已回滚/提交的证明将被database/sql拒绝。
	return m.writeResponseRegistration(ctx, proof.tx, proof.copySources(), domain.OutputLease{}, handler, edge)
}

// search_page.go：保留原实现，按业务合并文件。

````

## 第 31 步：SearchPages

插入位置：第 30 步代码之后，紧接追加；完成稿约第 634 行。

本步声明：`type SearchPages struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。 |
| <code>peer</code> | <code>SearchCandidatesPeer</code> | 使用 <code>SearchCandidatesPeer</code> 类型；其定义处的职责：Search仅提供候选；Doc负责最终过滤、片段构建与输出登记。 |
| <code>cursors</code> | <code>*domain.SearchCursorCodec</code> | 使用 <code>domain.SearchCursorCodec</code> 类型；其定义处的职责：定义正式索引事件、搜索查询、请求正文预算和不续期的分页游标。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>admission</code> | <code>chan struct{}</code> | 有界并发准入信号量，限制同时执行搜索页装配，避免昂贵候选复核无限堆积。 |

<!-- write-step: 31 bytes=22836:23066 -->
````go
type SearchPages struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	outputs   *Outputs
	peer      SearchCandidatesPeer
	cursors   *domain.SearchCursorCodec
	validator *codec.Validator
	admission chan struct{}
}

````

## 第 32 步：NewSearchPages

插入位置：第 31 步代码之后，紧接追加；完成稿约第 645 行。

本步声明：`func NewSearchPages(db *sql.DB, identity *Identity, access *Access, outputs *Outputs, peer SearchCandidatesPeer, cursors *domain.SearchCursorCodec, validator *codec.Validator) *SearchPages`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs: outputs</code> |
| <code>peer</code> | <code>SearchCandidatesPeer</code> | 使用 <code>SearchCandidatesPeer</code> 类型；其定义处的职责：Search仅提供候选；Doc负责最终过滤、片段构建与输出登记。<br>在此声明中的实际使用：<code>peer: peer</code> |
| <code>cursors</code> | <code>*domain.SearchCursorCodec</code> | 使用 <code>domain.SearchCursorCodec</code> 类型；其定义处的职责：定义正式索引事件、搜索查询、请求正文预算和不续期的分页游标。<br>在此声明中的实际使用：<code>cursors: cursors</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*SearchPages</code> | 实际返回表达式：<code>&amp;SearchPages{db: db, identity: identity, access: access, outputs: outputs, peer: peer, cursors: cursors, validator: validator, admission: make(chan struct{}, 2)}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;SearchPages{db: db, identity: identity, access: access, outputs: outputs, peer: peer, cursors: cursors, validator: validator, admission: make(chan struct{}, 2)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=23066:23430 -->
````go
func NewSearchPages(db *sql.DB, identity *Identity, access *Access, outputs *Outputs, peer SearchCandidatesPeer, cursors *domain.SearchCursorCodec, validator *codec.Validator) *SearchPages {
	return &SearchPages{db: db, identity: identity, access: access, outputs: outputs, peer: peer, cursors: cursors, validator: validator, admission: make(chan struct{}, 2)}
}

````

## 第 33 步：SearchPages.Page

插入位置：第 32 步代码之后，紧接追加；完成稿约第 649 行。

本步声明：`func (s *SearchPages) Page(ctx context.Context, actor *Actor, query domain.SearchQuery, handler string, bind BindOutput) (dto.SearchResultPage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>ctx.Err</code>；<code>s.identity.Revalidate(ctx, actor.Proof)</code>；<code>collectSearchPage(ctx, s.peer, query, scope, binding, start, func(ctx context.Context, candidates &#91;&#93;*pb.SearchCandidate) (searchCandidateEvaluator, error) { 		facts, err := s.batch …</code>（长表达式，按本步完整代码继续）<br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor != nil</code>；<code>actor.Proof</code>；<code>actor.ID</code>；<code>actor == nil</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>query</code> | <code>domain.SearchQuery</code> | 使用 <code>domain.SearchQuery</code> 类型；其定义处的职责：SearchQuery是参数解析后的domain值，不是另一份HTTP/RPC DTO；SpaceID=0表示未限定空间。 HTTP limit属于Doc整页，不能拿它替换Search固定候选批大小。<br>在此声明中的实际使用：<code>query.Binding</code>；<code>query.SpaceID</code>；<code>query.Cursor</code>；<code>collectSearchPage(ctx, s.peer, query, scope, binding, start, func(ctx context.Context, candidates &#91;&#93;*pb.SearchCandidate) (searchCandidateEvaluator, error) { 		facts, err := s.batch …</code>（长表达式，按本步完整代码继续）<br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>s.commitSearchPage(ctx, actor, collection.Entries, audience.finalProofs(), collection.Until, handler, bind)</code> |
| <code>bind</code> | <code>BindOutput</code> | 使用 <code>BindOutput</code> 类型；其定义处的职责：BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。<br>在此声明中的实际使用：<code>s.commitSearchPage(ctx, actor, collection.Entries, audience.finalProofs(), collection.Until, handler, bind)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.SearchResultPage</code> | 由 <code>fail(nil)</code> 的第 1 个返回项原样转交。<br>由 <code>fail(ctx.Err())</code> 的第 1 个返回项原样转交。<br>由 <code>fail(err)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>page</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>fail(nil)</code> 的第 2 个返回项原样转交。<br>由 <code>fail(ctx.Err())</code> 的第 2 个返回项原样转交。<br>由 <code>fail(err)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return dto.SearchResultPage{}, searchIncomplete(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>s == nil &#124;&#124; s.db == nil &#124;&#124; s.peer == nil &#124;&#124; s.cursors == nil &#124;&#124; s.validator == nil &#124;&#124; s.outputs == nil &#124;&#124; s.identity == nil &#124;&#124; s.access == nil</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。
- 登记 <code>func() { &lt;-s.admission }()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return fail(ctx.Err())</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>actor != nil</code>。
- 检查 <code>actor.Proof == nil &#124;&#124; actor.ID == 0 &#124;&#124; actor.Proof.ActorId != actor.ID</code>。条件成立时执行 <code>return fail(nil)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>query.SpaceID != 0</code>。
- 检查 <code>query.Cursor != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 33 bytes=23430:25218 -->
````go
func (s *SearchPages) Page(ctx context.Context, actor *Actor, query domain.SearchQuery, handler string, bind BindOutput) (dto.SearchResultPage, error) {
	fail := func(err error) (dto.SearchResultPage, error) { return dto.SearchResultPage{}, searchIncomplete(err) }
	if s == nil || s.db == nil || s.peer == nil || s.cursors == nil || s.validator == nil || s.outputs == nil || s.identity == nil || s.access == nil {
		return fail(nil)
	}
	// 在任何SQL读取前准入，保留原8连接池给消费、关闭与其他业务；不等持锁后再排队。
	select {
	case s.admission <- struct{}{}:
		defer func() { <-s.admission }()
	case <-ctx.Done():
		return fail(ctx.Err())
	}
	var session []byte
	if actor != nil {
		if actor.Proof == nil || actor.ID == 0 || actor.Proof.ActorId != actor.ID {
			return fail(nil)
		}
		session = actor.Proof.SessionRef
		if err := s.identity.Revalidate(ctx, actor.Proof); err != nil {
			return fail(err)
		}
	}
	binding, err := query.Binding(actor == nil, session)
	if err != nil {
		return fail(err)
	}
	scope := &pb.SearchScope{ScopeHash: hex.EncodeToString(binding[:]), Anonymous: actor == nil, SessionRef: append([]byte(nil), session...)}
	if query.SpaceID != 0 {
		scope.SpaceIds = []uint64{query.SpaceID}
	}
	var start *domain.SearchPageCursor
	if query.Cursor != "" {
		cursor, err := s.cursors.Open(query.Cursor, binding, time.Now())
		if err != nil {
			return fail(err)
		}
		start = &cursor
	}
	var bodyBytes uint64
	audience := newSearchAudience(s.identity, s.access, actor)
	collection, err := collectSearchPage(ctx, s.peer, query, scope, binding, start, func(ctx context.Context, candidates []*pb.SearchCandidate) (searchCandidateEvaluator, error) {
		facts, err := s.batchFacts(ctx, candidates)
		if err != nil {
			return nil, err
		}
````

## 第 34 步：SearchPages.Page（接着实现）

插入位置：第 33 步代码之后，紧接追加；完成稿约第 694 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>actor != nil</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return func(ctx context.Context, candidate *pb.SearchCandidate) (*searchPageEntry, error) { 			current, present := facts&#91;candidate.DocumentId&#93; 			if !present { 				return nil, nil  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>!present</code>。条件成立时执行 <code>return nil, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; !visible</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; !allowed</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return &amp;searchPageEntry{Summary: summary, Facts: current, Access: access, Sources: sources}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>i &lt; query.Limit</code>。
- 检查 <code>collection.Next != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 检查 <code>actor == nil &amp;&amp; len(collection.Entries) == 0 &amp;&amp; collection.Next == nil</code>。条件成立时执行 <code>return page, nil</code>。当前函数在这里结束。

<!-- write-step: 34 bytes=25218:26713 -->
````go
		if actor != nil {
			if err = s.identity.Revalidate(ctx, actor.Proof); err != nil {
				return nil, err
			}
		}
		return func(ctx context.Context, candidate *pb.SearchCandidate) (*searchPageEntry, error) {
			current, present := facts[candidate.DocumentId]
			if !present {
				return nil, nil
			}
			policy, visible, err := qualifySearchCandidate(candidate, current, query.SpaceID)
			if err != nil || !visible {
				return nil, err
			}
			access, sources, allowed, err := audience.authorize(ctx, current, policy)
			if err != nil || !allowed {
				return nil, err
			}
			summary, nextBytes, err := s.fixedSearchSummary(ctx, current, candidate.ReleaseId, query.Text, bodyBytes)
			if err != nil {
				return nil, err
			}
			bodyBytes = nextBytes
			return &searchPageEntry{Summary: summary, Facts: current, Access: access, Sources: sources}, nil
		}, nil
	})
	if err != nil {
		return fail(err)
	}
	page := dto.SearchResultPage{Items: make([]dto.SearchResult, 0, query.Limit), HasMore: collection.Next != nil}
	for i, entry := range collection.Entries {
		if i < query.Limit {
			page.Items = append(page.Items, entry.Summary)
		}
	}
	if collection.Next != nil {
		cursor, err := s.cursors.Seal(*collection.Next)
		if err != nil {
			return fail(err)
		}
		page.NextCursor = &cursor
	}
	// 完全耗尽的匿名空页没有标题、正文或游标，不捏造账户/单Doc输出准入。
	if actor == nil && len(collection.Entries) == 0 && collection.Next == nil {
		return page, nil
	}
````

## 第 35 步：SearchPages.Page（接着实现）

插入位置：第 34 步代码之后，紧接追加；完成稿约第 740 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail(err)</code>。当前函数在这里结束。
- 此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=26713:27072 -->
````go
	if err = s.commitSearchPage(ctx, actor, collection.Entries, audience.finalProofs(), collection.Until, handler, bind); err != nil {
		return fail(err)
	}
	// 正常EOF保留Search原链至原期限，支持HTTP回复丢失后的同锚点重试。
	// 不在最后一批已输出时提前Close，从而破坏终批后缀或前页重试。
	return page, nil
}

````

## 第 36 步：SearchPages.batchFacts

插入位置：第 35 步代码之后，紧接追加；完成稿约第 748 行。

本步声明：`func (s *SearchPages) batchFacts(ctx context.Context, candidates []*pb.SearchCandidate) (map[uint64]domain.PublicationFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})</code>；<code>repository.SearchPublicationFacts(ctx, tx, ids)</code> |
| <code>candidates</code> | <code>&#91;&#93;*pb.SearchCandidate</code> | 在此声明中的实际使用：<code>len(candidates)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;uint64&#93;domain.PublicationFacts</code> | 实际返回表达式：<code>nil</code>；<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>searchIncomplete(nil)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>candidate == nil</code>。条件成立时执行 <code>return nil, searchIncomplete(nil)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return facts, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=27072:27809 -->
````go
func (s *SearchPages) batchFacts(ctx context.Context, candidates []*pb.SearchCandidate) (map[uint64]domain.PublicationFacts, error) {
	ids := make([]uint64, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			return nil, searchIncomplete(nil)
		}
		ids = append(ids, candidate.DocumentId)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	facts, err := repository.SearchPublicationFacts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return facts, nil
}

// search_qualification.go：保留原实现，按业务合并文件。

````

## 第 37 步：qualifySearchCandidate

插入位置：第 36 步代码之后，紧接追加；完成稿约第 773 行。

本步声明：`func qualifySearchCandidate(candidate *pb.SearchCandidate, facts domain.PublicationFacts, space uint64) (string, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>candidate</code> | <code>*pb.SearchCandidate</code> | 在此声明中的实际使用：<code>candidate == nil</code>；<code>candidate.DocumentId</code>；<code>candidate.AggregateSeq</code>；<code>candidate.ReleaseId</code> |
| <code>facts</code> | <code>domain.PublicationFacts</code> | 使用 <code>domain.PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。<br>在此声明中的实际使用：<code>facts.Document</code>；<code>facts.Sequence</code>；<code>domain.PublicPolicy(facts)</code>；<code>domain.HasCurrentPublication(facts, policy)</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>space != 0</code>；<code>currentSpace != space</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>policy</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_FACTS_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>candidate == nil &#124;&#124; facts.Document.Id != domain.TextID(candidate.DocumentId)</code>。条件成立时执行 <code>return &quot;&quot;, false, domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_FACTS_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, false, err</code>。当前函数在这里结束。
- 检查 <code>(space != 0 &amp;&amp; currentSpace != space) &#124;&#124; facts.Sequence != candidate.AggregateSeq &#124;&#124; facts.Document.CurrentReleaseId == nil &#124;&#124; *facts.Document.CurrentReleaseId != domain.TextID(candidate.ReleaseId)</code>。条件成立时执行 <code>return &quot;&quot;, false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, false, err</code>。当前函数在这里结束。
- 检查 <code>!domain.HasCurrentPublication(facts, policy)</code>。条件成立时执行 <code>return &quot;&quot;, false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return policy, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=27809:28828 -->
````go
func qualifySearchCandidate(candidate *pb.SearchCandidate, facts domain.PublicationFacts, space uint64) (string, bool, error) {
	if candidate == nil || facts.Document.Id != domain.TextID(candidate.DocumentId) {
		return "", false, domain.Violation(domain.TemporarilyUnavailable, "SEARCH_FACTS_BINDING_INVALID")
	}
	currentSpace, err := domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return "", false, err
	}
	// ES可以落后，但不拥有正式指针。明确旧版本/水位/空间的候选先排除，
	// 不读旧正文，也不拿管理标题或MAIN回退填充页面。
	if (space != 0 && currentSpace != space) || facts.Sequence != candidate.AggregateSeq || facts.Document.CurrentReleaseId == nil || *facts.Document.CurrentReleaseId != domain.TextID(candidate.ReleaseId) {
		return "", false, nil
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return "", false, err
	}
	if !domain.HasCurrentPublication(facts, policy) {
		return "", false, nil
	}
	return policy, true, nil
}

````

## 第 38 步：searchSourceUnion

插入位置：第 37 步代码之后，紧接追加；完成稿约第 796 行。

本步声明：`func searchSourceUnion(groups ...[]*pb.SourceEpoch) ([]*pb.SourceEpoch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>groups</code> | <code>...&#91;&#93;*pb.SourceEpoch</code> | 多条结果各自的权限来源组；合并成整页输出依赖，相同来源的不同代次不能被静默覆盖。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>source == nil &#124;&#124; source.ScopeKey == &quot;&quot; &#124;&#124; source.Epoch == 0</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code>。当前函数在这里结束。
- 检查 <code>present &amp;&amp; epoch != source.Epoch</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;PAGE_CHANGED_RETRY&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=28828:29667 -->
````go
func searchSourceUnion(groups ...[]*pb.SourceEpoch) ([]*pb.SourceEpoch, error) {
	values := map[string]uint64{}
	for _, group := range groups {
		for _, source := range group {
			if source == nil || source.ScopeKey == "" || source.Epoch == 0 {
				return nil, domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
			}
			if epoch, present := values[source.ScopeKey]; present && epoch != source.Epoch {
				return nil, domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
			}
			values[source.ScopeKey] = source.Epoch
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]*pb.SourceEpoch, 0, len(keys))
	for _, key := range keys {
		result = append(result, &pb.SourceEpoch{ScopeKey: key, Epoch: values[key]})
	}
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/search_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
