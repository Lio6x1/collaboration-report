# 分步编写 `services/search/internal/domain/index/projection_models.go`

先按 [S01 · Canal事件与Doc当前正式投影](../lessons/search/01-event-authority.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/371-projection-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

校验Doc当前正式快照，提取可检索纯文本并定义单调应用规则。

旧事件只能提示“至少看到某序号”，不能决定读旧release；隐藏投影也需保留高水位防旧消息复活。

写之前先按这条执行路线理解各段的连接：

1. FromSnapshot核document/minimum、policy和固定正文/provenance字节hash，隐藏态严格无正文
2. ExtractChapter提取文本、公式和图片说明，资源集合必须恰好匹配，链接URL和内部ID不入摘要
3. ShouldApply拒绝同序不同权威摘要，旧序忽略，新序应用
4. Watermark将uint64格式为20位keyword供ES词典序比较

本文件需要守住的边界：

- Visible表示可索引正式快照，不等于当前匿名浏览器有权限
- 不能物理删隐藏文档而丢水位
- 原事件RawDigest与Authority.Digest必须分别保存

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/domain/index' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/domain/index/projection_models.go') -and (Get-Item -LiteralPath 'services/search/internal/domain/index/projection_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/domain/index/projection_models.go')) { New-Item -ItemType File -Path 'services/search/internal/domain/index/projection_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:384 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package index

import (
	"bytes"
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"strconv"
	"strings"
	"unicode/utf8"
)

// snapshot.go：保留原实现，按业务合并文件。

````

## 第 2 步：FromSnapshot

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

FromSnapshot只消费Doc认证的当前固定版本，不读Yjs，不把旧事件release当正文选择器。
权威摘要覆盖完整protobuf固定事实；它与原Canal行摘要独立保存。

本步声明：`func FromSnapshot(validator *codec.Validator, snapshot *pb.PublicSnapshot, document, minimum uint64) (Authority, error)`。

**这一段为什么这样写：** 只从已校验固定正式内容建立Authority。

1. 核快照身份和水位
2. 验证每单元正文/provenance
3. 提纯文字并核资源
4. 确定性protobuf序列化后散列


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>validator</code> | <code>*codec.Validator</code> | 共享content-v1/provenance-v1校验器<br>在此声明中的实际使用：<code>validator == nil</code>；<code>validator.Body</code>；<code>validator.Provenance</code> |
| <code>snapshot</code> | <code>*pb.PublicSnapshot</code> | Doc经mTLS提供的当前快照<br>在此声明中的实际使用：<code>snapshot == nil</code>；<code>snapshot.DocumentId</code>；<code>snapshot.AggregateSeq</code>；<code>snapshot.PublicationEpoch</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 期望文档ID<br>在此声明中的实际使用：<code>document == 0</code>；<code>snapshot.DocumentId != document</code>；<code>Document: document</code>；<code>fixed.DocumentId != document</code> |
| <code>minimum</code> | <code>uint64</code> | 触发事件要求至少达到的聚合序号<br>在此声明中的实际使用：<code>snapshot.AggregateSeq &lt; minimum</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Authority</code> | 可送入ES的当前Authority<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 非nil时不得造隐藏墓碑<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURRENT_SNAPSHOT_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_CURRENT_POLICY_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_HIDDEN_SNAPSHOT_HAS_BODY&quot;)</code>；<code>errors.New(&quot;SEARCH_VISIBLE_SNAPSHOT_INVALID&quot;)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>validator == nil &#124;&#124; snapshot == nil &#124;&#124; document == 0 &#124;&#124; snapshot.DocumentId != document &#124;&#124; snapshot.AggregateSeq &lt; minimum &#124;&#124; 		snapshot.PublicationEpoch == 0 &#124;&#124; snapshot.Visibilit …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_CURRENT_SNAPSHOT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>result.Policy != &quot;PUBLIC&quot; &amp;&amp; result.Policy != &quot;SPACE_MEMBERS&quot; &amp;&amp; result.Policy != &quot;COLLABORATORS&quot;</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_CURRENT_POLICY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!result.Visible</code>。
- 检查 <code>result.Release != 0 &#124;&#124; result.Structure != 0 &#124;&#124; result.Title != &quot;&quot; &#124;&#124; len(snapshot.Units) != 0</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_HIDDEN_SNAPSHOT_HAS_BODY&quot;)</code>。当前函数在这里结束。
- 检查 <code>result.Release == 0 &#124;&#124; result.Structure == 0 &#124;&#124; result.Sequence == 0 &#124;&#124; !utf8.ValidString(result.Title) &#124;&#124; len(result.Title) &gt; 1024</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_VISIBLE_SNAPSHOT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>unit == nil &#124;&#124; unit.UnitId == 0 &#124;&#124; unit.ContentVersionId == 0 &#124;&#124; seen&#91;unit.UnitId&#93;</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_FIXED_UNIT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>fixed == nil &#124;&#124; fixed.DocumentId != document &#124;&#124; fixed.UnitId != unit.UnitId &#124;&#124; fixed.StructureId != result.Structure &#124;&#124; 				fixed.SchemaVersion != &quot;content-v1&quot; &#124;&#124; fixed.Canonicaliz …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_FIXED_CONTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validator.Body(body) != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_FIXED_BODY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(canonical, fixed.BodyJsonUtf8) &#124;&#124; !bytes.Equal(digest&#91;:&#93;, fixed.BodyHash)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_FIXED_BODY_DIGEST_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=384:3213 -->
````go
// FromSnapshot只消费Doc认证的当前固定版本，不读Yjs，不把旧事件release当正文选择器。
// 权威摘要覆盖完整protobuf固定事实；它与原Canal行摘要独立保存。
func FromSnapshot(validator *codec.Validator, snapshot *pb.PublicSnapshot, document, minimum uint64) (Authority, error) {
	var result Authority
	if validator == nil || snapshot == nil || document == 0 || snapshot.DocumentId != document || snapshot.AggregateSeq < minimum ||
		snapshot.PublicationEpoch == 0 || snapshot.VisibilityRevision == 0 || proto.Size(snapshot) > 32<<20 ||
		len(snapshot.ProtoReflect().GetUnknown()) != 0 || len(snapshot.Units) > 100000 {
		return result, errors.New("SEARCH_CURRENT_SNAPSHOT_INVALID")
	}
	result = Authority{Document: document, Sequence: snapshot.AggregateSeq, Release: snapshot.ReleaseId, PublicationEpoch: snapshot.PublicationEpoch,
		Structure: snapshot.StructureId, Visible: snapshot.Visible, Policy: snapshot.VisibilityPolicy, Title: snapshot.Title, Chapters: []Chapter{}}
	if result.Policy != "PUBLIC" && result.Policy != "SPACE_MEMBERS" && result.Policy != "COLLABORATORS" {
		return result, errors.New("SEARCH_CURRENT_POLICY_INVALID")
	}
	if !result.Visible {
		if result.Release != 0 || result.Structure != 0 || result.Title != "" || len(snapshot.Units) != 0 {
			return result, errors.New("SEARCH_HIDDEN_SNAPSHOT_HAS_BODY")
		}
	} else {
		if result.Release == 0 || result.Structure == 0 || result.Sequence == 0 || !utf8.ValidString(result.Title) || len(result.Title) > 1024 {
			return result, errors.New("SEARCH_VISIBLE_SNAPSHOT_INVALID")
		}
		seen := make(map[uint64]bool)
		var text strings.Builder
		var total uint64
		for _, unit := range snapshot.Units {
			if unit == nil || unit.UnitId == 0 || unit.ContentVersionId == 0 || seen[unit.UnitId] {
				return result, errors.New("SEARCH_FIXED_UNIT_INVALID")
			}
			seen[unit.UnitId] = true
			fixed := unit.Content
			if fixed == nil || fixed.DocumentId != document || fixed.UnitId != unit.UnitId || fixed.StructureId != result.Structure ||
				fixed.SchemaVersion != "content-v1" || fixed.CanonicalizationVersion != "canonical-v1" || fixed.HashAlgorithm != "SHA-256" ||
				fixed.ProvenanceSchema != "provenance-v1" || len(fixed.BodyJsonUtf8) > 16<<20 || fixed.ByteSize != uint64(len(fixed.BodyJsonUtf8)) ||
				fixed.ByteSize > 24<<20 || total > (24<<20)-fixed.ByteSize {
				return result, errors.New("SEARCH_FIXED_CONTENT_INVALID")
			}
			total += fixed.ByteSize
			body, err := codec.Decode(fixed.BodyJsonUtf8)
			if err != nil || validator.Body(body) != nil {
				return result, errors.New("SEARCH_FIXED_BODY_INVALID")
			}
			canonical, err := codec.Canonical(body)
			digest := sha256.Sum256(canonical)
			if err != nil || !bytes.Equal(canonical, fixed.BodyJsonUtf8) || !bytes.Equal(digest[:], fixed.BodyHash) {
````

## 第 3 步：FromSnapshot（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 64 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return result, errors.New(&quot;SEARCH_FIXED_BODY_DIGEST_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; validator.Provenance(provenance) != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_FIXED_PROVENANCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(canonical, fixed.ProvenanceJsonUtf8)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_FIXED_PROVENANCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>text.Len()+len(chapter.Text)+1 &gt; 24&lt;&lt;20</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_FIXED_TEXT_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_AUTHORITY_DIGEST_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=3213:4318 -->
````go
				return result, errors.New("SEARCH_FIXED_BODY_DIGEST_INVALID")
			}
			provenance, err := codec.Decode(fixed.ProvenanceJsonUtf8)
			if err != nil || validator.Provenance(provenance) != nil {
				return result, errors.New("SEARCH_FIXED_PROVENANCE_INVALID")
			}
			canonical, err = codec.Canonical(provenance)
			if err != nil || !bytes.Equal(canonical, fixed.ProvenanceJsonUtf8) {
				return result, errors.New("SEARCH_FIXED_PROVENANCE_INVALID")
			}
			chapter, err := ExtractChapter(body, unit.UnitId, fixed.ResourceIds)
			if err != nil {
				return result, err
			}
			if text.Len()+len(chapter.Text)+1 > 24<<20 {
				return result, errors.New("SEARCH_FIXED_TEXT_LIMIT")
			}
			text.WriteString(chapter.Text)
			text.WriteByte('\n')
			result.Chapters = append(result.Chapters, chapter)
		}
		result.Text = text.String()
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(snapshot)
	if err != nil {
		return result, errors.New("SEARCH_AUTHORITY_DIGEST_INVALID")
	}
	result.Digest = sha256.Sum256(raw)
	return result, nil
}

// state.go：保留原实现，按业务合并文件。

````

## 第 4 步：Chapter

插入位置：第 3 步代码之后，紧接追加；完成稿约第 97 行。

本步声明：`type Chapter struct {`。

**这一段为什么这样写：** 搜索摘要的章节定位信息。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Unit</code> | <code>uint64</code> | 正文单元ID，结果导航定位键<br>序列化/校验标记：<code>`json:&quot;unit&quot;`</code> |
| <code>Title</code> | <code>string</code> | 正文中首个标题提取文字<br>序列化/校验标记：<code>`json:&quot;title&quot;`</code> |
| <code>Text</code> | <code>string</code> | 该单元可检索语义文本<br>序列化/校验标记：<code>`json:&quot;text&quot;`</code> |

<!-- write-step: 4 bytes=4318:4428 -->
````go
type Chapter struct {
	Unit  uint64 `json:"unit"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

````

## 第 5 步：Authority

插入位置：第 4 步代码之后，紧接追加；完成稿约第 103 行。

Authority来自Doc当前正式快照；Visible是可索引状态，不代表当前浏览器匿名可读。

本步声明：`type Authority struct {`。

**这一段为什么这样写：** Doc当前完整固定快照生成的索引权威。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>uint64</code> | 文档ID |
| <code>Sequence</code> | <code>uint64</code> | 当前Doc聚合水位，可能高于触发事件 |
| <code>Release</code> | <code>uint64</code> | 当前正式发布ID；隐藏时0 |
| <code>PublicationEpoch</code> | <code>uint64</code> | 当前发布权限代次 |
| <code>Structure</code> | <code>uint64</code> | 正式正文结构版本ID |
| <code>Visible</code> | <code>bool</code> | 当前是否具可索引正式正文，不代表匿名可读 |
| <code>Policy</code> | <code>string</code> | PUBLIC/SPACE_MEMBERS/COLLABORATORS策略 |
| <code>Title</code> | <code>string</code> | 正式文档标题 |
| <code>Text</code> | <code>string</code> | 从合法正文提取的全篇纯文本 |
| <code>Chapters</code> | <code>&#91;&#93;Chapter</code> | 按固定清单顺序保留单元定位文本 |
| <code>Digest</code> | <code>&#91;32&#93;byte</code> | 完整确定性protobuf快照摘要，和原事件摘要独立 |

<!-- write-step: 5 bytes=4428:4843 -->
````go
// Authority来自Doc当前正式快照；Visible是可索引状态，不代表当前浏览器匿名可读。
type Authority struct {
	Document         uint64
	Sequence         uint64
	Release          uint64
	PublicationEpoch uint64
	Structure        uint64
	Visible          bool
	Policy           string
	Title            string
	Text             string
	Chapters         []Chapter
	Digest           [32]byte
}

````

## 第 6 步：State

插入位置：第 5 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`type State struct {`。

**这一段为什么这样写：** 某物理索引对一个文档实际应用的水位。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Sequence</code> | <code>uint64</code> | 实际已应用聚合序号 |
| <code>Release</code> | <code>uint64</code> | 实际正式发布ID或隐藏0 |
| <code>Visible</code> | <code>bool</code> | 实际投影可见状态 |
| <code>Policy</code> | <code>string</code> | 实际可见性策略 |
| <code>Digest</code> | <code>&#91;32&#93;byte</code> | 对应权威快照摘要 |
| <code>Revision</code> | <code>uint64</code> | SQL index_application乐观修订号 |

<!-- write-step: 6 bytes=4843:4968 -->
````go
type State struct {
	Sequence uint64
	Release  uint64
	Visible  bool
	Policy   string
	Digest   [32]byte
	Revision uint64
}

````

## 第 7 步：Watermark

插入位置：第 6 步代码之后，紧接追加；完成稿约第 127 行。

ES版本long不能承载全uint64。固定20位十进制keyword可在原子脚本中比较，
包括最大水位的隐藏墓碑；不能通过物理删除丢失水位后让旧事件复活正文。

本步声明：`func Watermark(value uint64) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>uint64</code> | 在此声明中的实际使用：<code>fmt.Sprintf(&quot;%020d&quot;, value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>fmt.Sprintf(&quot;%020d&quot;, value)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return fmt.Sprintf(&quot;%020d&quot;, value)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4968:5247 -->
````go
// ES版本long不能承载全uint64。固定20位十进制keyword可在原子脚本中比较，
// 包括最大水位的隐藏墓碑；不能通过物理删除丢失水位后让旧事件复活正文。
func Watermark(value uint64) string { return fmt.Sprintf("%020d", value) }

````

## 第 8 步：ShouldApply

插入位置：第 7 步代码之后，紧接追加；完成稿约第 131 行。

本步声明：`func ShouldApply(current State, next Authority) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>current</code> | <code>State</code> | 使用 <code>State</code> 类型；其定义处的职责：某物理索引对一个文档实际应用的水位。<br>在此声明中的实际使用：<code>current.Digest</code>；<code>current.Sequence</code>；<code>current.Visible</code>；<code>current.Release</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>next</code> | <code>Authority</code> | 使用 <code>Authority</code> 类型；其定义处的职责：Doc当前完整固定快照生成的索引权威。<br>在此声明中的实际使用：<code>next.Digest</code>；<code>next.Policy</code>；<code>next.Visible</code>；<code>next.Release</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_AUTHORITY_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_INDEX_STATE_INVALID&quot;)</code>；<code>nil</code>；<code>errors.New(&quot;SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>next.Digest == (&#91;32&#93;byte{}) &#124;&#124; 		(next.Policy != &quot;PUBLIC&quot; &amp;&amp; next.Policy != &quot;SPACE_MEMBERS&quot; &amp;&amp; next.Policy != &quot;COLLABORATORS&quot;) &#124;&#124; 		(next.Visible &amp;&amp; next.Release == 0) &#124;&#124; (!next.Visible &amp;&amp; next.Release != 0)</code>。条件成立时执行 <code>return false, errors.New(&quot;SEARCH_AUTHORITY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.Digest == (&#91;32&#93;byte{})</code>。
- 检查 <code>current.Sequence != 0 &#124;&#124; current.Visible &#124;&#124; current.Release != 0</code>。条件成立时执行 <code>return false, errors.New(&quot;SEARCH_INDEX_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>next.Sequence &lt; current.Sequence</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>next.Sequence == current.Sequence</code>。
- 检查 <code>next.Digest != current.Digest &#124;&#124; next.Visible != current.Visible &#124;&#124; 			next.Release != current.Release &#124;&#124; next.Policy != current.Policy</code>。条件成立时执行 <code>return false, errors.New(&quot;SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return false, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=5247:6197 -->
````go
func ShouldApply(current State, next Authority) (bool, error) {
	if next.Digest == ([32]byte{}) ||
		(next.Policy != "PUBLIC" && next.Policy != "SPACE_MEMBERS" && next.Policy != "COLLABORATORS") ||
		(next.Visible && next.Release == 0) || (!next.Visible && next.Release != 0) {
		return false, errors.New("SEARCH_AUTHORITY_INVALID")
	}
	if current.Digest == ([32]byte{}) {
		if current.Sequence != 0 || current.Visible || current.Release != 0 {
			return false, errors.New("SEARCH_INDEX_STATE_INVALID")
		}
		return true, nil
	}
	if next.Sequence < current.Sequence {
		return false, nil
	}
	if next.Sequence == current.Sequence {
		if next.Digest != current.Digest || next.Visible != current.Visible ||
			next.Release != current.Release || next.Policy != current.Policy {
			return false, errors.New("SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT")
		}
		return false, nil
	}
	return true, nil
}

// text.go：保留原实现，按业务合并文件。

````

## 第 9 步：ExtractChapter

插入位置：第 8 步代码之后，紧接追加；完成稿约第 158 行。

ExtractChapter从已通过共享content-v1验证的语义树提取纯文本。
正文、表格/代码、公式和图片说明可检索；链接目标、资源ID和provenance内部标识不进入摘要。

本步声明：`func ExtractChapter(body any, unit uint64, resourceIDs []uint64) (Chapter, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>body</code> | <code>any</code> | 在此声明中的实际使用：<code>extract(body, &amp;text, &amp;result.Title, resources)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Unit: unit</code> |
| <code>resourceIDs</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(resourceIDs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Chapter</code> | 使用 <code>Chapter</code> 类型；其定义处的职责：搜索摘要的章节定位信息。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_FIXED_RESOURCES_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>len(resources) != len(resourceIDs)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_FIXED_RESOURCES_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>resource == 0 &#124;&#124; seen&#91;resource&#93; &#124;&#124; !resources&#91;resource&#93;</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_FIXED_RESOURCES_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=6197:7085 -->
````go
// ExtractChapter从已通过共享content-v1验证的语义树提取纯文本。
// 正文、表格/代码、公式和图片说明可检索；链接目标、资源ID和provenance内部标识不进入摘要。
func ExtractChapter(body any, unit uint64, resourceIDs []uint64) (Chapter, error) {
	result := Chapter{Unit: unit}
	resources := make(map[uint64]bool)
	var text strings.Builder
	if err := extract(body, &text, &result.Title, resources); err != nil {
		return result, err
	}
	if len(resources) != len(resourceIDs) {
		return result, errors.New("SEARCH_FIXED_RESOURCES_INVALID")
	}
	seen := make(map[uint64]bool)
	for _, resource := range resourceIDs {
		if resource == 0 || seen[resource] || !resources[resource] {
			return result, errors.New("SEARCH_FIXED_RESOURCES_INVALID")
		}
		seen[resource] = true
	}
	result.Text = strings.TrimSpace(text.String())
	return result, nil
}

````

## 第 10 步：extract

插入位置：第 9 步代码之后，紧接追加；完成稿约第 181 行。

本步声明：`func extract(value any, text *strings.Builder, heading *string, resources map[uint64]bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>node, valid := value.(map&#91;string&#93;any)</code> |
| <code>text</code> | <code>*strings.Builder</code> | 在此声明中的实际使用：<code>text.WriteString</code>；<code>text.WriteByte</code>；<code>text.Len</code>；<code>extract(child, text, heading, resources)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>heading</code> | <code>*string</code> | 在此声明中的实际使用：<code>extract(child, text, heading, resources)</code>；<code>*heading == &quot;&quot;</code>；<code>*heading = strings.TrimSpace(text.String()&#91;start:&#93;)</code> |
| <code>resources</code> | <code>map&#91;uint64&#93;bool</code> | 在此声明中的实际使用：<code>resources&#91;resource&#93; = true</code>；<code>extract(child, text, heading, resources)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_FIXED_NODE_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_FIXED_RESOURCES_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_FIXED_TEXT_LIMIT&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!valid</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_FIXED_NODE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; resource == 0 &#124;&#124; strconv.FormatUint(resource, 10) != id</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_FIXED_RESOURCES_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>ok</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>kind == &quot;heading&quot; &amp;&amp; *heading == &quot;&quot;</code>。
- 检查 <code>text.Len() &gt; 24&lt;&lt;20</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_FIXED_TEXT_LIMIT&quot;)</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=7085:8424 -->
````go
func extract(value any, text *strings.Builder, heading *string, resources map[uint64]bool) error {
	node, valid := value.(map[string]any)
	if !valid {
		return errors.New("SEARCH_FIXED_NODE_INVALID")
	}
	kind, _ := node["type"].(string)
	attrs, _ := node["attrs"].(map[string]any)
	switch kind {
	case "text":
		value, _ := node["text"].(string)
		text.WriteString(value)
	case "formula", "formula_block":
		value, _ := attrs["latex"].(string)
		text.WriteByte(' ')
		text.WriteString(value)
		text.WriteByte(' ')
	case "image":
		id, _ := attrs["resourceId"].(string)
		resource, err := strconv.ParseUint(id, 10, 64)
		if err != nil || resource == 0 || strconv.FormatUint(resource, 10) != id {
			return errors.New("SEARCH_FIXED_RESOURCES_INVALID")
		}
		resources[resource] = true
		for _, field := range []string{"alt", "title"} {
			if value, ok := attrs[field].(string); ok {
				text.WriteByte(' ')
				text.WriteString(value)
			}
		}
	case "hard_break":
		text.WriteByte('\n')
	default:
		start := text.Len()
		children, _ := node["content"].([]any)
		for _, child := range children {
			if err := extract(child, text, heading, resources); err != nil {
				return err
			}
		}
		if kind == "heading" && *heading == "" {
			*heading = strings.TrimSpace(text.String()[start:])
		}
		text.WriteByte('\n')
	}
	if text.Len() > 24<<20 {
````

## 第 11 步：extract（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 226 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return errors.New(&quot;SEARCH_FIXED_TEXT_LIMIT&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=8424:8488 -->
````go
		return errors.New("SEARCH_FIXED_TEXT_LIMIT")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/domain/index/projection_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/01-event-authority.md)。
