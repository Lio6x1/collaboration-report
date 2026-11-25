# 分步编写 `services/document/internal/logic/document/public_readings_service.go`

先按 [D07 · 正式发布与当前受众](../lessons/document/07-publication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/219-public-readings-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按当前正式指针与当前受众输出公开文档、受控历史和搜索快照。

已发布版本固定内容，当前受众决定能否看；这两条轴必须分别核验。

写之前先按这条执行路线理解各段的连接：

1. Current取得facts/audience后锁来源再复核
2. fixed只读取release自有manifest
3. History/Historical要求内部资格
4. Snapshot给搜索当前正式事实并核水位。

本文件需要守住的边界：

- 公开阅读不调用Node活跃MAIN、不创建版本
- 旧release/seq只是事件提示，不能据此选择旧正文。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/public_readings_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/public_readings_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/public_readings_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/public_readings_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:582 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"sort"
	"strconv"
	"strings"
)

// public_reading.go：保留原实现，按业务合并文件。

````

## 第 2 步：PublicReading

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`type PublicReading struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。 |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |

<!-- write-step: 2 bytes=582:749 -->
````go
type PublicReading struct {
	db         *sql.DB
	identity   *Identity
	access     *Access
	outputs    *Outputs
	pagination *Pagination
	validator  *codec.Validator
}

````

## 第 3 步：NewPublicReading

插入位置：第 2 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func NewPublicReading(db *sql.DB, identity *Identity, access *Access, outputs *Outputs, pagination *Pagination, validator *codec.Validator) *PublicReading`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs: outputs</code> |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>pagination: pagination</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*PublicReading</code> | 实际返回表达式：<code>&amp;PublicReading{db: db, identity: identity, access: access, outputs: outputs, pagination: pagination, validator: validator}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;PublicReading{db: db, identity: identity, access: access, outputs: outputs, pagination: pagination, validator: validator}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=749:1040 -->
````go
func NewPublicReading(db *sql.DB, identity *Identity, access *Access, outputs *Outputs, pagination *Pagination, validator *codec.Validator) *PublicReading {
	return &PublicReading{db: db, identity: identity, access: access, outputs: outputs, pagination: pagination, validator: validator}
}

````

## 第 4 步：PublicReading.facts

插入位置：第 3 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`func (s *PublicReading) facts(ctx context.Context, id uint64) (domain.PublicationFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{ReadOnly: true})</code>；<code>repository.PublicationFactsRow(ctx, tx, id, false)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.PublicationFactsRow(ctx, tx, id, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.PublicationFacts</code> | 使用 <code>domain.PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。<br>实际返回表达式：<code>domain.PublicationFacts{}</code>；<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{ReadOnly: true})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.PublicationFacts{}, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return facts, err</code>。当前函数在这里结束。
- 此处通过 <code>return facts, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 4 bytes=1040:1530 -->
````go
func (s *PublicReading) facts(ctx context.Context, id uint64) (domain.PublicationFacts, error) {
	// 乐观快照只决定需要哪些真实证明。最终输出仍按来源→Doc→KB→Space当前读复核。
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return domain.PublicationFacts{}, err
	}
	defer tx.Rollback()
	facts, err := repository.PublicationFactsRow(ctx, tx, id, false)
	if err != nil {
		return facts, err
	}
	return facts, tx.Commit()
}

````

## 第 5 步：publicationSources

插入位置：第 4 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`func publicationSources(facts domain.PublicationFacts) ([]*pb.SourceEpoch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>domain.PublicationFacts</code> | 使用 <code>domain.PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。<br>在此声明中的实际使用：<code>facts.Document</code>；<code>facts.KnowledgeBase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>facts.KnowledgeBase != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1530:2249 -->
````go
func publicationSources(facts domain.PublicationFacts) ([]*pb.SourceEpoch, error) {
	document := facts.Document
	acl, err := domain.CanonicalID(document.AclRevision)
	if err != nil {
		return nil, err
	}
	epoch, err := domain.CanonicalID(document.PublicationEpoch)
	if err != nil {
		return nil, err
	}
	sources := []*pb.SourceEpoch{{ScopeKey: "doc-acl:" + document.Id, Epoch: acl}, {ScopeKey: "publication:" + document.Id, Epoch: epoch}}
	if facts.KnowledgeBase != nil {
		revision, err := domain.CanonicalID(facts.KnowledgeBase.AclRevision)
		if err != nil {
			return nil, err
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + facts.KnowledgeBase.Id, Epoch: revision})
	}
	return sources, nil
}

````

## 第 6 步：PublicReading.audience

插入位置：第 5 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func (s *PublicReading) audience(ctx context.Context, actor *Actor, facts domain.PublicationFacts, policy string) ([]*pb.IdentityAdmission, []*pb.SourceEpoch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, *actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, &quot;READ_PUBLIC_DOCUMENT&quot;, 0)</code>；<code>s.access.Authorize(ctx, *actor, id, &quot;READ_PUBLIC_DOCUMENT&quot;)</code>；<code>s.identity.Revalidate(ctx, proof)</code> |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor != nil</code>；<code>actor.Proof</code>；<code>s.access.SpaceIdentity(ctx, *actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, &quot;READ_PUBLIC_DOCUMENT&quot;, 0)</code>；<code>s.access.Authorize(ctx, *actor, id, &quot;READ_PUBLIC_DOCUMENT&quot;)</code> |
| <code>facts</code> | <code>domain.PublicationFacts</code> | 使用 <code>domain.PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。<br>在此声明中的实际使用：<code>facts.Space</code>；<code>facts.Document</code>；<code>publicationSources(facts)</code> |
| <code>policy</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidatePublicAudience(policy, actor != nil, member, internal)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proofs</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>append(identitySources, sources...)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>actor != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return proofs, append(identitySources, sources...), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2249:3694 -->
````go
func (s *PublicReading) audience(ctx context.Context, actor *Actor, facts domain.PublicationFacts, policy string) ([]*pb.IdentityAdmission, []*pb.SourceEpoch, error) {
	proofs := []*pb.IdentityAdmission{}
	member, internal := false, false
	if actor != nil {
		proofs = append(proofs, actor.Proof)
		switch policy {
		case "SPACE_MEMBERS":
			space, err := domain.CanonicalID(facts.Space.Id)
			if err != nil {
				return nil, nil, err
			}
			_, proof, err := s.access.SpaceIdentity(ctx, *actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, "READ_PUBLIC_DOCUMENT", 0)
			if err != nil {
				return nil, nil, err
			}
			proofs = append(proofs, proof)
			member = true
		case "COLLABORATORS":
			id, _ := domain.CanonicalID(facts.Document.Id)
			access, err := s.access.Authorize(ctx, *actor, id, "READ_PUBLIC_DOCUMENT")
			if err != nil {
				return nil, nil, err
			}
			proofs = append(proofs, access.Proofs...)
			internal = access.Permissions.Read
		}
	}
	if err := domain.ValidatePublicAudience(policy, actor != nil, member, internal); err != nil {
		return nil, nil, err
	}
	identitySources, err := MergeSources(proofs...)
	if err != nil {
		return nil, nil, err
	}
	sources, err := publicationSources(facts)
	if err != nil {
		return nil, nil, err
	}
	for _, proof := range proofs {
		if err = s.identity.Revalidate(ctx, proof); err != nil {
			return nil, nil, err
		}
	}
	return proofs, append(identitySources, sources...), nil
}

````

## 第 7 步：PublicReading.Current

插入位置：第 6 步代码之后，紧接追加；完成稿约第 117 行。

本步声明：`func (s *PublicReading) Current(ctx context.Context, actor *Actor, id uint64, handler string, bind BindOutput) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.facts(ctx, id)</code>；<code>s.audience(ctx, actor, facts, policy)</code>；<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.audience(ctx, actor, facts, policy)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.facts(ctx, id)</code>；<code>repository.PublicationFactsRow(ctx, tx, id, true)</code>；<code>s.outputs.RegisterPublication(ctx, tx, sources, id, epoch, release, handler, edge)</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>handler == &quot;&quot;</code>；<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.outputs.RegisterPublication(ctx, tx, sources, id, epoch, release, handler, edge)</code> |
| <code>bind</code> | <code>BindOutput</code> | 使用 <code>BindOutput</code> 类型；其定义处的职责：BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。<br>在此声明中的实际使用：<code>bind(registration, sources)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_REQUIRED&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;PUBLIC_DOCUMENT_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>handler == &quot;&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!domain.HasCurrentPublication(facts, policy)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;PUBLIC_DOCUMENT_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; edge == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>publicFactsBinding(current) != publicFactsBinding(facts)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;PUBLIC_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!domain.HasCurrentPublication(current, policy)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;PUBLIC_DOCUMENT_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=3694:5266 -->
````go
func (s *PublicReading) Current(ctx context.Context, actor *Actor, id uint64, handler string, bind BindOutput) (any, error) {
	if handler == "" {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_REQUIRED")
	}
	facts, err := s.facts(ctx, id)
	if err != nil {
		return nil, err
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return nil, err
	}
	if !domain.HasCurrentPublication(facts, policy) {
		return nil, domain.Violation(domain.EntityNotFound, "PUBLIC_DOCUMENT_NOT_FOUND")
	}
	_, sources, err := s.audience(ctx, actor, facts, policy)
	if err != nil {
		return nil, err
	}
	// Edge实例查询是跨进程调用，必须位于SQL锁外；缺失/变化不能用匿名路径绕过。
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	current, err := repository.PublicationFactsRow(ctx, tx, id, true)
	if err != nil {
		return nil, err
	}
	if publicFactsBinding(current) != publicFactsBinding(facts) {
		return nil, domain.Violation(domain.StateConflict, "PUBLIC_DOCUMENT_CHANGED")
	}
	if !domain.HasCurrentPublication(current, policy) {
		return nil, domain.Violation(domain.EntityNotFound, "PUBLIC_DOCUMENT_NOT_FOUND")
	}
	release, err := domain.CanonicalID(*current.Document.CurrentReleaseId)
	if err != nil {
		return nil, err
	}
````

## 第 8 步：PublicReading.Current（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 163 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=5266:5826 -->
````go
	epoch, err := domain.CanonicalID(current.Document.PublicationEpoch)
	if err != nil {
		return nil, err
	}
	value, _, err := s.fixed(ctx, tx, current.Document, release)
	if err != nil {
		return nil, err
	}
	registration, err := s.outputs.RegisterPublication(ctx, tx, sources, id, epoch, release, handler, edge)
	if err != nil {
		return nil, err
	}
	output := bind(registration, sources)
	if err = tx.Commit(); err != nil {
		output.Abort()
		return nil, err
	}
	return value, nil
}

// public_reading_binding.go：保留原实现，按业务合并文件。

````

## 第 9 步：optionalText

插入位置：第 8 步代码之后，紧接追加；完成稿约第 185 行。

本步声明：`func optionalText(value *string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>*string</code> | 在此声明中的实际使用：<code>value == nil</code>；<code>return *value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>*value</code> |

**跟着本段代码走一遍：**

- 检查 <code>value == nil</code>。条件成立时执行 <code>return &quot;&quot;</code>。当前函数在这里结束。
- 此处通过 <code>return *value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5826:5920 -->
````go
func optionalText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

````

## 第 10 步：publicFactsBinding

插入位置：第 9 步代码之后，紧接追加；完成稿约第 192 行。

只绑定正式指针与当前授权事实。管理标题、MAIN/结构/control随后变化不改写固定正式版。

本步声明：`func publicFactsBinding(facts domain.PublicationFacts) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>domain.PublicationFacts</code> | 使用 <code>domain.PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。<br>在此声明中的实际使用：<code>facts.Document</code>；<code>facts.Sequence</code>；<code>facts.Space</code>；<code>facts.KnowledgeBase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>hex.EncodeToString(digest&#91;:&#93;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>facts.KnowledgeBase != nil</code>。
- 此处通过 <code>return hex.EncodeToString(digest&#91;:&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5920:6848 -->
````go
// 只绑定正式指针与当前授权事实。管理标题、MAIN/结构/control随后变化不改写固定正式版。
func publicFactsBinding(facts domain.PublicationFacts) string {
	d := facts.Document
	parts := []string{d.Id, d.SpaceId, optionalText(d.KnowledgeBaseId), d.Lifecycle, d.AclRevision, d.VisibilityPolicy, d.VisibilityRevision, d.PublicationEpoch, d.PublicationState, optionalText(d.CurrentReleaseId), domain.TextID(facts.Sequence), facts.Space.Lifecycle, facts.Space.Kind, optionalText(facts.Space.OwnerUserId), optionalText(facts.Space.OrganizationId)}
	if facts.KnowledgeBase != nil {
		kb := facts.KnowledgeBase
		parts = append(parts, kb.Id, kb.SpaceId, kb.Lifecycle, kb.AclRevision, kb.VisibilityPolicy, kb.VisibilityRevision)
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(digest[:])
}

// public_reading_fixed.go：保留原实现，按业务合并文件。

````

## 第 11 步：PublicReading.fixed

插入位置：第 10 步代码之后，紧接追加；完成稿约第 206 行。

所有元数据来自release自己的manifest。阅读不创建新版本，也不调用Node读取活跃MAIN。

本步声明：`func (s *PublicReading) fixed(ctx context.Context, tx *sql.Tx, document dto.Document, release uint64) (dto.PublicDocument, []*pb.ManifestUnit, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.FixedReleaseRow(ctx, tx, id, release)</code>；<code>repository.StructureRow(ctx, tx, fixed.Structure, document.Id)</code>；<code>repository.StyleRow(ctx, tx, fixed.Style, document.Id)</code>；<code>repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.FixedReleaseRow(ctx, tx, id, release)</code>；<code>repository.StructureRow(ctx, tx, fixed.Structure, document.Id)</code>；<code>repository.StyleRow(ctx, tx, fixed.Style, document.Id)</code>；<code>repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>dto.Document</code> | 在此声明中的实际使用：<code>document.Id</code>；<code>document.PublicationEpoch</code> |
| <code>release</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.FixedReleaseRow(ctx, tx, id, release)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.PublicDocument</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.ManifestUnit</code> | 实际返回表达式：<code>nil</code>；<code>wire</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_FIXED_CONTENT_INVALID&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_FIXED_CONTENT_LIMIT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, nil, err</code>。当前函数在这里结束。

<!-- write-step: 11 bytes=6848:8568 -->
````go
// 所有元数据来自release自己的manifest。阅读不创建新版本，也不调用Node读取活跃MAIN。
func (s *PublicReading) fixed(ctx context.Context, tx *sql.Tx, document dto.Document, release uint64) (dto.PublicDocument, []*pb.ManifestUnit, error) {
	value := dto.PublicDocument{DocumentId: document.Id, PublicationEpoch: document.PublicationEpoch, Units: []dto.FixedContent{}}
	id, err := domain.CanonicalID(document.Id)
	if err != nil {
		return value, nil, err
	}
	fixed, err := repository.FixedReleaseRow(ctx, tx, id, release)
	if err != nil {
		return value, nil, err
	}
	value.ReleaseId, value.Title, value.ManifestId = fixed.Release.Id, fixed.Title, fixed.Release.ManifestId
	value.Structure, err = repository.StructureRow(ctx, tx, fixed.Structure, document.Id)
	if err != nil {
		return value, nil, err
	}
	value.Style, err = repository.StyleRow(ctx, tx, fixed.Style, document.Id)
	if err != nil {
		return value, nil, err
	}
	units, err := repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)
	if err != nil {
		return value, nil, err
	}
	if err = validateReadingUnits(value.Structure, units); err != nil {
		return value, nil, err
	}
	wire := []*pb.ManifestUnit{}
	var total uint64
	structure, _ := domain.CanonicalID(fixed.Structure)
	for _, unit := range units {
		unitID, err := domain.CanonicalID(unit["unit_id"])
		if err != nil {
			return value, nil, err
		}
		version, err := domain.CanonicalID(unit["content_version_id"])
		if err != nil {
			return value, nil, err
		}
		content, err := repository.VersionContent(ctx, tx, version, id)
		if err != nil {
			return value, nil, err
		}
		if err = canonicalVersion(content, version); err != nil {
			return value, nil, err
		}
````

## 第 12 步：PublicReading.fixed（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 252 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_FIXED_CONTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>content.ByteSize &gt; 24&lt;&lt;20 &#124;&#124; total &gt; (24&lt;&lt;20)-content.ByteSize</code>。条件成立时执行 <code>return value, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_FIXED_CONTENT_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, wire, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=8568:10056 -->
````go
		if err = validateFixedMergeContent(s.validator, content, id, unitID, structure); err != nil {
			return value, nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_FIXED_CONTENT_INVALID")
		}
		// 总预算在加法前检查；即使未来历史引用异常，也不能整数回绕或无限装配正文。
		if content.ByteSize > 24<<20 || total > (24<<20)-content.ByteSize {
			return value, nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_FIXED_CONTENT_LIMIT")
		}
		total += content.ByteSize
		body := dto.FixedContent{Id: domain.TextID(version), DocumentId: document.Id, UnitId: domain.TextID(unitID), StructureId: domain.TextID(content.StructureId), SchemaVersion: content.SchemaVersion, CanonicalizationVersion: content.CanonicalizationVersion, BodyHash: hex.EncodeToString(content.BodyHash), ByteSize: domain.TextID(content.ByteSize), ResourceIds: []string{}}
		if err = json.Unmarshal(content.BodyJsonUtf8, &body.BodyJson); err != nil {
			return value, nil, err
		}
		if err = json.Unmarshal(content.ProvenanceJsonUtf8, &body.Provenance); err != nil {
			return value, nil, err
		}
		for _, resource := range content.ResourceIds {
			body.ResourceIds = append(body.ResourceIds, domain.TextID(resource))
		}
		value.Units = append(value.Units, body)
		wire = append(wire, &pb.ManifestUnit{UnitId: unitID, ContentVersionId: version, Content: content})
	}
	return value, wire, nil
}

// public_reading_history.go：保留原实现，按业务合并文件。

````

## 第 13 步：PublicReading.History

插入位置：第 12 步代码之后，紧接追加；完成稿约第 278 行。

本步声明：`func (s *PublicReading) History(ctx context.Context, actor Actor, id uint64, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_RELEASE_HISTORY&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_RELEASE_HISTORY&quot;)</code>；<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_RELEASE_HISTORY&quot;)</code>；<code>domain.TextID(id)</code>；<code>repository.ReleaseRows(ctx, tx, id, before, limit+1)</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, &quot;&quot;, &quot;&quot;)</code>；<code>s.pagination.Parse(limitText, cursor, binding)</code> |
| <code>cursor</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return sources&#91;i&#93;.Key &lt; sources&#91;j&#93;.Key</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.PublicationEpoch != access.Document.PublicationEpoch</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;RELEASE_HISTORY_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>page.HasMore</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=10056:11817 -->
````go
func (s *PublicReading) History(ctx context.Context, actor Actor, id uint64, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_RELEASE_HISTORY")
	if err != nil {
		return nil, nil, err
	}
	limit, _, err := s.pagination.Parse(limitText, "", "")
	if err != nil {
		return nil, nil, err
	}
	sources := SourcesWire(access.Sources)
	sort.Slice(sources, func(i, j int) bool { return sources[i].Key < sources[j].Key })
	// 每次新证明的随机admission号不属于游标；真实会话/来源、页大小和发布代次必须属于绑定。
	raw, _ := json.Marshal([]any{domain.TextID(actor.ID), hex.EncodeToString(actor.Proof.SessionRef), domain.TextID(id), strconv.Itoa(limit), access.Document.PublicationEpoch, sources})
	digest := sha256.Sum256(raw)
	binding := "release-history:" + hex.EncodeToString(digest[:])
	_, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	var before uint64
	if last != "" {
		before, err = domain.CanonicalID(last)
		if err != nil {
			return nil, nil, err
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if current.PublicationEpoch != access.Document.PublicationEpoch {
			return nil, domain.Violation(domain.StateConflict, "RELEASE_HISTORY_CHANGED")
		}
		items, err := repository.ReleaseRows(ctx, tx, id, before, limit+1)
		if err != nil {
			return nil, err
		}
		page := dto.ReleasePage{Items: items, HasMore: len(items) > limit}
		if page.HasMore {
			page.Items = items[:limit]
			page.NextCursor = s.pagination.Next(binding, page.Items[limit-1].Id)
		}
		return page, nil
	}
````

## 第 14 步：PublicReading.History（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 323 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=11817:11853 -->
````go
	return access.Sources, run, nil
}

````

## 第 15 步：PublicReading.Historical

插入位置：第 14 步代码之后，紧接追加；完成稿约第 326 行。

本步声明：`func (s *PublicReading) Historical(ctx context.Context, actor Actor, id, release uint64) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_RELEASE_HISTORY&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_RELEASE_HISTORY&quot;)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_RELEASE_HISTORY&quot;)</code> |
| <code>release</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.fixed(ctx, tx, current, release)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=11853:12442 -->
````go
func (s *PublicReading) Historical(ctx context.Context, actor Actor, id, release uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_RELEASE_HISTORY")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		value, _, err := s.fixed(ctx, tx, current, release)
		return value, err
	}
	return access.Sources, run, nil
}

// public_snapshot.go：保留原实现，按业务合并文件。

````

## 第 16 步：PublicReading.Snapshot

插入位置：第 15 步代码之后，紧接追加；完成稿约第 344 行。

Search是受信任的候选索引者，visible表示当前正式版可索引，COLLABORATORS仍须最后由Doc授权。
请求release/seq只是旧事件提示；绝不能用它选择旧历史正文或把未知Doc当成删除。

本步声明：`func (s *PublicReading) Snapshot(ctx context.Context, input *pb.PublicSnapshotRequest) (*pb.PublicSnapshot, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.facts(ctx, input.DocumentId)</code>；<code>s.db.BeginTx(ctx, options)</code>；<code>LockSources(ctx, tx, sources)</code>；<code>repository.PublicationFactsRow(ctx, tx, input.DocumentId, visible)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>*pb.PublicSnapshotRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.DocumentId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.PublicSnapshot</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;PUBLIC_SNAPSHOT_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PUBLIC_SNAPSHOT_CHANGED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_SNAPSHOT_TRANSFER_LIMIT&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.DocumentId == 0</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;PUBLIC_SNAPSHOT_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!visible</code>。
- 执行 <code>s.db.BeginTx(ctx, options)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>visible</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 16 bytes=12442:14214 -->
````go
// Search是受信任的候选索引者，visible表示当前正式版可索引，COLLABORATORS仍须最后由Doc授权。
// 请求release/seq只是旧事件提示；绝不能用它选择旧历史正文或把未知Doc当成删除。
func (s *PublicReading) Snapshot(ctx context.Context, input *pb.PublicSnapshotRequest) (*pb.PublicSnapshot, error) {
	if input == nil || input.DocumentId == 0 {
		return nil, domain.Violation(domain.InvalidRequest, "PUBLIC_SNAPSHOT_QUERY_INVALID")
	}
	facts, err := s.facts(ctx, input.DocumentId)
	if err != nil {
		return nil, err
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return nil, err
	}
	visible := domain.HasCurrentPublication(facts, policy)
	var options *sql.TxOptions
	if !visible {
		// 隐藏回复没有正文，也不登记用户输出；同一只读一致快照足以绑定已提交事实。
		// 不抢Doc/KB行锁，避免与持有KB→Doc锁序的政策fanout形成循环等待。
		// 旧快照若低于消息最低水位，Search仍拒绝并重读；刚变可见则沿下面绑定检查重试。
		options = &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}
	}
	tx, err := s.db.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if visible {
		sources, err := publicationSources(facts)
		if err != nil {
			return nil, err
		}
		if err = LockSources(ctx, tx, sources); err != nil {
			return nil, err
		}
	}
	current, err := repository.PublicationFactsRow(ctx, tx, input.DocumentId, visible)
	if err != nil {
		return nil, err
	}
	currentPolicy, err := domain.PublicPolicy(current)
	if err != nil {
		return nil, err
	}
	// 无正文的不可见事实可在关闭中返回。若刚转成可见，须重试完整来源路径，不能倒置锁序。
````

## 第 17 步：PublicReading.Snapshot（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 389 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>publicFactsBinding(current) != publicFactsBinding(facts) &#124;&#124; (!visible &amp;&amp; domain.HasCurrentPublication(current, currentPolicy))</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;PUBLIC_SNAPSHOT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>current.KnowledgeBase != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>visible</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>proto.Size(result) &gt; 32&lt;&lt;20</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_SNAPSHOT_TRANSFER_LIMIT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 17 bytes=14214:15682 -->
````go
	if publicFactsBinding(current) != publicFactsBinding(facts) || (!visible && domain.HasCurrentPublication(current, currentPolicy)) {
		return nil, domain.Violation(domain.StateConflict, "PUBLIC_SNAPSHOT_CHANGED")
	}
	epoch, err := domain.CanonicalID(current.Document.PublicationEpoch)
	if err != nil {
		return nil, err
	}
	visibility, err := domain.CanonicalID(current.Document.VisibilityRevision)
	if err != nil {
		return nil, err
	}
	result := &pb.PublicSnapshot{DocumentId: input.DocumentId, AggregateSeq: current.Sequence, PublicationEpoch: epoch, VisibilityPolicy: currentPolicy, VisibilityRevision: visibility, Units: []*pb.ManifestUnit{}}
	if current.KnowledgeBase != nil {
		result.KnowledgeBaseVisibilityRevision, err = domain.CanonicalID(current.KnowledgeBase.VisibilityRevision)
		if err != nil {
			return nil, err
		}
	}
	if visible {
		release, err := domain.CanonicalID(*current.Document.CurrentReleaseId)
		if err != nil {
			return nil, err
		}
		fixed, units, err := s.fixed(ctx, tx, current.Document, release)
		if err != nil {
			return nil, err
		}
		structure, err := domain.CanonicalID(fixed.Structure.Id)
		if err != nil {
			return nil, err
		}
		result.Visible, result.ReleaseId, result.Title, result.StructureId, result.Units = true, release, fixed.Title, structure, units
	}
	if proto.Size(result) > 32<<20 {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_SNAPSHOT_TRANSFER_LIMIT")
	}
	return result, tx.Commit()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/public_readings_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/07-publication.md)。
