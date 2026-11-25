# 分步编写 `services/document/internal/domain/document/search_models.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/102-search-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义正式索引事件、搜索查询、请求正文预算和不续期的分页游标。

Search只给候选，当前权限与固定正文仍由Doc负责；索引水位独立于发布epoch。

写之前先按这条执行路线理解各段的连接：

1. EncodeIndexEvent输出八键规范消息
2. NextIndexEventSequence防溢出
3. SearchQuery.Binding绑定真实会话与查询
4. SearchCursorCodec封装opaque批锚点
5. ReserveSearchBody在读正文前占预算。

本文件需要守住的边界：

- release只是事件提示非历史授权
- Until首次固定不可续期
- 元数据预算后仍要验len/hash/schema。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/search_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/search_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/search_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/search_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:430 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// index_event.go：保留原实现，按业务合并文件。

````

## 第 2 步：IndexEvent

插入位置：第 1 步代码之后，紧接追加；完成稿约第 26 行。

IndexEvent只描述正式引用及索引水位，不携带正文、策略或身份证明。
事件触发Search读取Doc当前权威快照；Release不是读取旧历史正文的授权。

本步声明：`type IndexEvent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 唯一outbox事件号 |
| <code>Type</code> | <code>string</code> | 发布/撤下/可见性变化类型 |
| <code>Document</code> | <code>uint64</code> | 权威文档ID |
| <code>Sequence</code> | <code>uint64</code> | 单调索引事件水位 |
| <code>Release</code> | <code>uint64</code> | 当前正式版ID，零编码null |
| <code>PublicationEpoch</code> | <code>uint64</code> | 当前正式发布代次 |
| <code>OccurredAt</code> | <code>time.Time</code> | 事件发生时间 |

<!-- write-step: 2 bytes=430:816 -->
````go
// IndexEvent只描述正式引用及索引水位，不携带正文、策略或身份证明。
// 事件触发Search读取Doc当前权威快照；Release不是读取旧历史正文的授权。
type IndexEvent struct {
	ID, Type                                      string
	Document, Sequence, Release, PublicationEpoch uint64
	OccurredAt                                    time.Time
}

````

## 第 3 步：NextIndexEventSequence

插入位置：第 2 步代码之后，紧接追加；完成稿约第 34 行。

可见性变更不属于再次发布，所以索引水位独立推进，不能顺带改变publication_epoch。
调用方必须在业务事务内先取得当前水位；耗尽拒绝整个动作，不能回绕复活旧事件。

本步声明：`func NextIndexEventSequence(current uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>current</code> | <code>uint64</code> | 在此声明中的实际使用：<code>current == math.MaxUint64</code>；<code>current + 1</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>current + 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;EVENT_SEQUENCE_EXHAUSTED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>current == math.MaxUint64</code>。条件成立时执行 <code>return 0, Violation(StateConflict, &quot;EVENT_SEQUENCE_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return current + 1, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=816:1238 -->
````go
// 可见性变更不属于再次发布，所以索引水位独立推进，不能顺带改变publication_epoch。
// 调用方必须在业务事务内先取得当前水位；耗尽拒绝整个动作，不能回绕复活旧事件。
func NextIndexEventSequence(current uint64) (uint64, error) {
	if current == math.MaxUint64 {
		return 0, Violation(StateConflict, "EVENT_SEQUENCE_EXHAUSTED")
	}
	return current + 1, nil
}

````

## 第 4 步：VisibilityIndexEventID

插入位置：第 3 步代码之后，紧接追加；完成稿约第 43 行。

一个KB原操作会为多个文档登记事件，不能复用原操作作为每条outbox的主键。
固定类型前缀及完整摘要绑定原操作和文档，重试稳定，最大长度输入也不截断摘要。

本步声明：`func VisibilityIndexEventID(operation string, document uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>ValidOperationID(operation)</code>；<code>operation + &quot;\n&quot;</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>document == 0</code>；<code>TextID(document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>&quot;visibility_index_&quot; + hex.EncodeToString(digest&#91;:&#93;)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;INDEX_EVENT_BINDING_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(operation) &#124;&#124; document == 0</code>。条件成立时执行 <code>return &quot;&quot;, Violation(InvalidRequest, &quot;INDEX_EVENT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;visibility_index_&quot; + hex.EncodeToString(digest&#91;:&#93;), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1238:1916 -->
````go
// 一个KB原操作会为多个文档登记事件，不能复用原操作作为每条outbox的主键。
// 固定类型前缀及完整摘要绑定原操作和文档，重试稳定，最大长度输入也不截断摘要。
func VisibilityIndexEventID(operation string, document uint64) (string, error) {
	if !ValidOperationID(operation) || document == 0 {
		return "", Violation(InvalidRequest, "INDEX_EVENT_BINDING_INVALID")
	}
	// 合法操作编号不含换行，因此此分隔不会让不同原操作/文档组合产生相同输入。
	digest := sha256.Sum256([]byte(operation + "\n" + TextID(document)))
	return "visibility_index_" + hex.EncodeToString(digest[:]), nil
}

````

## 第 5 步：EncodeIndexEvent

插入位置：第 4 步代码之后，紧接追加；完成稿约第 54 行。

EncodeIndexEvent维持既有八键消息形状。所有业务uint64使用规范十进制字符串，
release_id缺失显式为null；不可见策略不会改变这里的正式指针事实。

本步声明：`func EncodeIndexEvent(event IndexEvent) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>event</code> | <code>IndexEvent</code> | 使用 <code>IndexEvent</code> 类型；其定义处的职责：IndexEvent只描述正式引用及索引水位，不携带正文、策略或身份证明。 事件触发Search读取Doc当前权威快照；Release不是读取旧历史正文的授权。<br>在此声明中的实际使用：<code>event.ID</code>；<code>event.Document</code>；<code>event.Sequence</code>；<code>event.PublicationEpoch</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 由 <code>json.Marshal(payload)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>json.Marshal(payload)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>invalid</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(event.ID) &#124;&#124; event.Document == 0 &#124;&#124; event.Sequence == 0 &#124;&#124; 		event.PublicationEpoch == 0 &#124;&#124; event.OccurredAt.IsZero()</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>event.Release == 0</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>event.Release != 0</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>event.Release != 0</code>。

<!-- write-step: 5 bytes=1916:3613 -->
````go
// EncodeIndexEvent维持既有八键消息形状。所有业务uint64使用规范十进制字符串，
// release_id缺失显式为null；不可见策略不会改变这里的正式指针事实。
func EncodeIndexEvent(event IndexEvent) ([]byte, error) {
	invalid := Violation(InvalidRequest, "INDEX_EVENT_INVALID")
	if !ValidOperationID(event.ID) || event.Document == 0 || event.Sequence == 0 ||
		event.PublicationEpoch == 0 || event.OccurredAt.IsZero() {
		return nil, invalid
	}
	switch event.Type {
	case "DOCUMENT_PUBLISHED":
		if event.Release == 0 {
			return nil, invalid
		}
	case "DOCUMENT_UNPUBLISHED":
		if event.Release != 0 {
			return nil, invalid
		}
	case "DOCUMENT_VISIBILITY_CHANGED":
		// 未发布及已有当前正式版都可变更策略；此动作不创建或撤下正式版。
	default:
		return nil, invalid
	}
	// MarshalText在输出RFC3339Nano前校验可表示年份；不能把无效时间写成毒消息。
	occurred, err := event.OccurredAt.UTC().MarshalText()
	if err != nil {
		return nil, invalid
	}
	var release *string
	if event.Release != 0 {
		value := TextID(event.Release)
		release = &value
	}
	payload := struct {
		ID       string  `json:"event_id"`
		Type     string  `json:"event_type"`
		Schema   string  `json:"schema_version"`
		Document string  `json:"document_id"`
		Sequence string  `json:"aggregate_seq"`
		Release  *string `json:"release_id"`
		Epoch    string  `json:"publication_epoch"`
		Occurred string  `json:"occurred_at"`
	}{
		ID: event.ID, Type: event.Type, Schema: "1", Document: TextID(event.Document),
		Sequence: TextID(event.Sequence), Release: release,
		Epoch: TextID(event.PublicationEpoch), Occurred: string(occurred),
	}
````

## 第 6 步：EncodeIndexEvent（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 100 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return json.Marshal(payload)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3613:3712 -->
````go
	return json.Marshal(payload)
}

// search_budget.go：保留原实现，按业务合并文件。

````

## 第 7 步：SearchDocumentBodyLimit、SearchPageBodyLimit

插入位置：第 6 步代码之后，紧接追加；完成稿约第 105 行。

本步声明：`const (`。

<!-- write-step: 7 bytes=3712:3809 -->
````go
const (
	SearchDocumentBodyLimit uint64 = 24 << 20
	SearchPageBodyLimit     uint64 = 64 << 20
)

````

## 第 8 步：ReserveSearchBody

插入位置：第 7 步代码之后，紧接追加；完成稿约第 110 行。

ReserveSearchBody只处理权威manifest的字节元数据，不读取正文，也不提供权限判断。
调用方须先核验所有unit/version归属；整篇占额后才读正文，lookahead使用同一请求预算。
随后的正文必须再次核验实际len==byte_size及完整hash/schema，不能把元数据当完整验证。

本步声明：`func ReserveSearchBody(used uint64, units []uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>used</code> | <code>uint64</code> | 在此声明中的实际使用：<code>used &gt; SearchPageBodyLimit</code>；<code>SearchPageBodyLimit-used</code>；<code>used + document</code> |
| <code>units</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(units)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>used + document</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>used &gt; SearchPageBodyLimit &#124;&#124; len(units) == 0</code>。条件成立时执行 <code>return 0, Violation(TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>size == 0 &#124;&#124; size &gt; SearchDocumentBodyLimit &#124;&#124; document &gt; SearchDocumentBodyLimit-size</code>。条件成立时执行 <code>return 0, Violation(TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>document &gt; SearchPageBodyLimit-used</code>。条件成立时执行 <code>return 0, Violation(TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return used + document, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3809:4905 -->
````go
// ReserveSearchBody只处理权威manifest的字节元数据，不读取正文，也不提供权限判断。
// 调用方须先核验所有unit/version归属；整篇占额后才读正文，lookahead使用同一请求预算。
// 随后的正文必须再次核验实际len==byte_size及完整hash/schema，不能把元数据当完整验证。
func ReserveSearchBody(used uint64, units []uint64) (uint64, error) {
	if used > SearchPageBodyLimit || len(units) == 0 {
		return 0, Violation(TemporarilyUnavailable, "SEARCH_INCOMPLETE")
	}
	var document uint64
	for _, size := range units {
		// 先检查减法边界再加，最大uint64、整篇超限及错误零字节都不能回绕成小正文。
		if size == 0 || size > SearchDocumentBodyLimit || document > SearchDocumentBodyLimit-size {
			return 0, Violation(TemporarilyUnavailable, "SEARCH_INCOMPLETE")
		}
		document += size
	}
	if document > SearchPageBodyLimit-used {
		return 0, Violation(TemporarilyUnavailable, "SEARCH_INCOMPLETE")
	}
	return used + document, nil
}

// search_cursor.go：保留原实现，按业务合并文件。

````

## 第 9 步：searchCursorVersion、searchCursorMaxAnchor、searchCursorMaxEncoded、searchCursorPlainHeader、searchCursorPurpose

插入位置：第 8 步代码之后，紧接追加；完成稿约第 133 行。

本步声明：`const (`。

<!-- write-step: 9 bytes=4905:5177 -->
````go
const (
	searchCursorVersion     = byte(1)
	searchCursorMaxAnchor   = 768
	searchCursorMaxEncoded  = 2048
	searchCursorPlainHeader = 43 // Until(8)、Binding(32)、Prefix(1)、anchor长度(2)。
	searchCursorPurpose     = "collaboration/document/search-page-cursor/v1"
)

````

## 第 10 步：SearchPageCursor

插入位置：第 9 步代码之后，紧接追加；完成稿约第 141 行。

SearchPageCursor只携带Search的opaque批锚点及Doc已消费前缀；不复制候选/正文。
Until是初次建立链时固定的UnixMillis绝对期限，Seal和Open都不续期。

本步声明：`type SearchPageCursor struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Anchor</code> | <code>&#91;&#93;byte</code> | Search签出的opaque批锚点原字节 |
| <code>Prefix</code> | <code>uint8</code> | 该批已消费候选前缀长度 |
| <code>Binding</code> | <code>&#91;32&#93;byte</code> | 原查询和真实会话摘要 |
| <code>Until</code> | <code>int64</code> | 首次游标链绝对截止毫秒，翻页不延期 |

<!-- write-step: 10 bytes=5177:5467 -->
````go
// SearchPageCursor只携带Search的opaque批锚点及Doc已消费前缀；不复制候选/正文。
// Until是初次建立链时固定的UnixMillis绝对期限，Seal和Open都不续期。
type SearchPageCursor struct {
	Anchor  []byte
	Prefix  uint8
	Binding [32]byte
	Until   int64
}

````

## 第 11 步：SearchCursorCodec

插入位置：第 10 步代码之后，紧接追加；完成稿约第 150 行。

本步声明：`type SearchCursorCodec struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>aead</code> | <code>cipher.AEAD</code> | 独立派生密钥的认证加密器，保护HTTP游标 |
| <code>authentication</code> | <code>&#91;32&#93;byte</code> | 另行派生的外层签名密钥，与加密用途隔离 |

<!-- write-step: 11 bytes=5467:5555 -->
````go
type SearchCursorCodec struct {
	aead           cipher.AEAD
	authentication [32]byte
}

````

## 第 12 步：NewSearchCursorCodec

插入位置：第 11 步代码之后，紧接追加；完成稿约第 155 行。

本步声明：`func NewSearchCursorCodec(key []byte) (*SearchCursorCodec, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(key)</code>；<code>subtle.ConstantTimeCompare(key, make(&#91;&#93;byte, 32))</code>；<code>deriveSearchCursorKey(key, searchCursorPurpose+&quot;/encryption&quot;)</code>；<code>deriveSearchCursorKey(key, searchCursorPurpose+&quot;/authentication&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*SearchCursorCodec</code> | 实际返回表达式：<code>nil</code>；<code>&amp;SearchCursorCodec{aead: aead, authentication: authentication}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>；<code>Violation(TemporarilyUnavailable, &quot;CURSOR_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(key) != 32 &#124;&#124; subtle.ConstantTimeCompare(key, make(&#91;&#93;byte, 32)) == 1</code>。条件成立时执行 <code>return nil, Violation(InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, Violation(TemporarilyUnavailable, &quot;CURSOR_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;SearchCursorCodec{aead: aead, authentication: authentication}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5555:6407 -->
````go
func NewSearchCursorCodec(key []byte) (*SearchCursorCodec, error) {
	if len(key) != 32 || subtle.ConstantTimeCompare(key, make([]byte, 32)) == 1 {
		return nil, Violation(InvalidRequest, "CURSOR_INVALID")
	}
	// 同一master仅派生两个独立用途密钥；不持有调用方可变key切片，也不复用其他游标签名。
	encryption := deriveSearchCursorKey(key, searchCursorPurpose+"/encryption")
	authentication := deriveSearchCursorKey(key, searchCursorPurpose+"/authentication")
	block, err := aes.NewCipher(encryption[:])
	clear(encryption[:])
	if err != nil {
		return nil, Violation(TemporarilyUnavailable, "CURSOR_UNAVAILABLE")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, Violation(TemporarilyUnavailable, "CURSOR_UNAVAILABLE")
	}
	return &SearchCursorCodec{aead: aead, authentication: authentication}, nil
}

````

## 第 13 步：deriveSearchCursorKey

插入位置：第 12 步代码之后，紧接追加；完成稿约第 174 行。

本步声明：`func deriveSearchCursorKey(master []byte, purpose string) [32]byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>master</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>hmac.New(sha256.New, master)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(purpose)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 实际返回表达式：<code>key</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return key</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6407:6609 -->
````go
func deriveSearchCursorKey(master []byte, purpose string) [32]byte {
	mac := hmac.New(sha256.New, master)
	_, _ = mac.Write([]byte(purpose))
	var key [32]byte
	copy(key[:], mac.Sum(nil))
	return key
}

````

## 第 14 步：validSearchPageCursor

插入位置：第 13 步代码之后，紧接追加；完成稿约第 182 行。

本步声明：`func validSearchPageCursor(cursor SearchPageCursor) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cursor</code> | <code>SearchPageCursor</code> | 使用 <code>SearchPageCursor</code> 类型；其定义处的职责：SearchPageCursor只携带Search的opaque批锚点及Doc已消费前缀；不复制候选/正文。 Until是初次建立链时固定的UnixMillis绝对期限，Seal和Open都不续期。<br>在此声明中的实际使用：<code>cursor.Anchor</code>；<code>cursor.Prefix</code>；<code>cursor.Binding</code>；<code>cursor.Until</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>len(cursor.Anchor) &gt;= 1 &amp;&amp; len(cursor.Anchor) &lt;= searchCursorMaxAnchor &amp;&amp; 		cursor.Prefix &lt;= 99 &amp;&amp; cursor.Binding != &#91;32&#93;byte{} &amp;&amp; cursor.Until &gt; 0</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return len(cursor.Anchor) &gt;= 1 &amp;&amp; len(cursor.Anchor) &lt;= searchCursorMaxAnchor &amp;&amp; 		cursor.Prefix &lt;= 99 &amp;&amp; cursor.Binding != &#91;32&#93;byte{} &amp;&amp; cursor.Until &gt; 0</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6609:6827 -->
````go
func validSearchPageCursor(cursor SearchPageCursor) bool {
	return len(cursor.Anchor) >= 1 && len(cursor.Anchor) <= searchCursorMaxAnchor &&
		cursor.Prefix <= 99 && cursor.Binding != [32]byte{} && cursor.Until > 0
}

````

## 第 15 步：SearchCursorCodec.valid

插入位置：第 14 步代码之后，紧接追加；完成稿约第 187 行。

本步声明：`func (codec *SearchCursorCodec) valid() bool`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>codec != nil &amp;&amp; codec.aead != nil &amp;&amp; codec.authentication != &#91;32&#93;byte{}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return codec != nil &amp;&amp; codec.aead != nil &amp;&amp; codec.authentication != &#91;32&#93;byte{}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6827:6957 -->
````go
func (codec *SearchCursorCodec) valid() bool {
	return codec != nil && codec.aead != nil && codec.authentication != [32]byte{}
}

````

## 第 16 步：SearchCursorCodec.signature

插入位置：第 15 步代码之后，紧接追加；完成稿约第 191 行。

本步声明：`func (codec *SearchCursorCodec) signature(envelope []byte) []byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>envelope</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>mac.Write(envelope)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>mac.Sum(nil)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return mac.Sum(nil)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=6957:7192 -->
````go
func (codec *SearchCursorCodec) signature(envelope []byte) []byte {
	mac := hmac.New(sha256.New, codec.authentication[:])
	_, _ = mac.Write([]byte(searchCursorPurpose + "/envelope"))
	_, _ = mac.Write(envelope)
	return mac.Sum(nil)
}

````

## 第 17 步：SearchCursorCodec.Seal

插入位置：第 16 步代码之后，紧接追加；完成稿约第 198 行。

本步声明：`func (codec *SearchCursorCodec) Seal(cursor SearchPageCursor) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cursor</code> | <code>SearchPageCursor</code> | 使用 <code>SearchPageCursor</code> 类型；其定义处的职责：SearchPageCursor只携带Search的opaque批锚点及Doc已消费前缀；不复制候选/正文。 Until是初次建立链时固定的UnixMillis绝对期限，Seal和Open都不续期。<br>在此声明中的实际使用：<code>validSearchPageCursor(cursor)</code>；<code>cursor.Anchor</code>；<code>cursor.Until</code>；<code>cursor.Binding</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>encoded</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>；<code>Violation(TemporarilyUnavailable, &quot;CURSOR_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!codec.valid() &#124;&#124; !validSearchPageCursor(cursor)</code>。条件成立时执行 <code>return &quot;&quot;, Violation(InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 登记 <code>clear(plain)</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, Violation(TemporarilyUnavailable, &quot;CURSOR_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(encoded) &gt; searchCursorMaxEncoded</code>。条件成立时执行 <code>return &quot;&quot;, Violation(InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return encoded, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=7192:8473 -->
````go
func (codec *SearchCursorCodec) Seal(cursor SearchPageCursor) (string, error) {
	if !codec.valid() || !validSearchPageCursor(cursor) {
		return "", Violation(InvalidRequest, "CURSOR_INVALID")
	}
	// 显式二进制字段避免JSON数值精度和另一套字段拼法；Anchor原字节只复制，不解析sort。
	plain := make([]byte, searchCursorPlainHeader+len(cursor.Anchor))
	defer clear(plain)
	binary.BigEndian.PutUint64(plain[:8], uint64(cursor.Until))
	copy(plain[8:40], cursor.Binding[:])
	plain[40] = cursor.Prefix
	binary.BigEndian.PutUint16(plain[41:43], uint16(len(cursor.Anchor)))
	copy(plain[43:], cursor.Anchor)
	nonce := make([]byte, codec.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", Violation(TemporarilyUnavailable, "CURSOR_UNAVAILABLE")
	}
	envelope := make([]byte, 1+len(nonce), 1+len(nonce)+len(plain)+codec.aead.Overhead()+sha256.Size)
	envelope[0] = searchCursorVersion
	copy(envelope[1:], nonce)
	envelope = codec.aead.Seal(envelope, nonce, plain, []byte(searchCursorPurpose))
	envelope = append(envelope, codec.signature(envelope)...)
	encoded := base64.RawURLEncoding.EncodeToString(envelope)
	if len(encoded) > searchCursorMaxEncoded {
		return "", Violation(InvalidRequest, "CURSOR_INVALID")
	}
	return encoded, nil
}

````

## 第 18 步：SearchCursorCodec.Open

插入位置：第 17 步代码之后，紧接追加；完成稿约第 226 行。

本步声明：`func (codec *SearchCursorCodec) Open(encoded string, expectedBinding [32]byte, now time.Time) (SearchPageCursor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>encoded</code> | <code>string</code> | 在此声明中的实际使用：<code>len(encoded)</code>；<code>base64.RawURLEncoding.Strict().DecodeString(encoded)</code>；<code>base64.RawURLEncoding.EncodeToString(envelope) != encoded</code> |
| <code>expectedBinding</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>expectedBinding == &#91;32&#93;byte{}</code>；<code>subtle.ConstantTimeCompare(cursor.Binding&#91;:&#93;, expectedBinding&#91;:&#93;)</code> |
| <code>now</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>now.UnixMilli</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>SearchPageCursor</code> | 使用 <code>SearchPageCursor</code> 类型；其定义处的职责：SearchPageCursor只携带Search的opaque批锚点及Doc已消费前缀；不复制候选/正文。 Until是初次建立链时固定的UnixMillis绝对期限，Seal和Open都不续期。<br>由 <code>invalid()</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>SearchPageCursor{}</code>；<code>cursor</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>invalid()</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>Violation(StateConflict, &quot;CURSOR_STALE&quot;)</code>；<code>Violation(StateConflict, &quot;CURSOR_EXPIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return SearchPageCursor{}, Violation(InvalidRequest, &quot;CURSOR_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!codec.valid() &#124;&#124; expectedBinding == &#91;32&#93;byte{} &#124;&#124; len(encoded) == 0 &#124;&#124; len(encoded) &gt; searchCursorMaxEncoded</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; base64.RawURLEncoding.EncodeToString(envelope) != encoded</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>len(envelope) &lt; minimum &#124;&#124; len(envelope) &gt; maximum &#124;&#124; envelope&#91;0&#93; != searchCursorVersion</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>!hmac.Equal(signature, codec.signature(sealed))</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 登记 <code>clear(plain)</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>len(plain) &lt; searchCursorPlainHeader &#124;&#124; int(binary.BigEndian.Uint16(plain&#91;41:43&#93;)) != len(plain)-searchCursorPlainHeader</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>!validSearchPageCursor(cursor)</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>subtle.ConstantTimeCompare(cursor.Binding&#91;:&#93;, expectedBinding&#91;:&#93;) != 1</code>。条件成立时执行 <code>return SearchPageCursor{}, Violation(StateConflict, &quot;CURSOR_STALE&quot;)</code>。当前函数在这里结束。
- 检查 <code>cursor.Until &lt;= now.UnixMilli()</code>。条件成立时执行 <code>return SearchPageCursor{}, Violation(StateConflict, &quot;CURSOR_EXPIRED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=8473:10721 -->
````go
func (codec *SearchCursorCodec) Open(encoded string, expectedBinding [32]byte, now time.Time) (SearchPageCursor, error) {
	invalid := func() (SearchPageCursor, error) {
		return SearchPageCursor{}, Violation(InvalidRequest, "CURSOR_INVALID")
	}
	if !codec.valid() || expectedBinding == [32]byte{} || len(encoded) == 0 || len(encoded) > searchCursorMaxEncoded {
		return invalid()
	}
	envelope, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	// Go的base64解码器会忽略CR/LF；重新编码核对还拒绝这些非规范表示。
	if err != nil || base64.RawURLEncoding.EncodeToString(envelope) != encoded {
		return invalid()
	}
	header := 1 + codec.aead.NonceSize()
	minimum := header + searchCursorPlainHeader + 1 + codec.aead.Overhead() + sha256.Size
	maximum := header + searchCursorPlainHeader + searchCursorMaxAnchor + codec.aead.Overhead() + sha256.Size
	if len(envelope) < minimum || len(envelope) > maximum || envelope[0] != searchCursorVersion {
		return invalid()
	}
	sealed, signature := envelope[:len(envelope)-sha256.Size], envelope[len(envelope)-sha256.Size:]
	// 外层先验证版本/nonce/密文整体，再进入GCM；所有认证错误只返回固定码。
	if !hmac.Equal(signature, codec.signature(sealed)) {
		return invalid()
	}
	plain, err := codec.aead.Open(nil, sealed[1:header], sealed[header:], []byte(searchCursorPurpose))
	if err != nil {
		return invalid()
	}
	defer clear(plain)
	if len(plain) < searchCursorPlainHeader || int(binary.BigEndian.Uint16(plain[41:43])) != len(plain)-searchCursorPlainHeader {
		return invalid()
	}
	cursor := SearchPageCursor{Prefix: plain[40], Until: int64(binary.BigEndian.Uint64(plain[:8]))}
	copy(cursor.Binding[:], plain[8:40])
	cursor.Anchor = plain[searchCursorPlainHeader:]
	if !validSearchPageCursor(cursor) {
		return invalid()
	}
	if subtle.ConstantTimeCompare(cursor.Binding[:], expectedBinding[:]) != 1 {
		return SearchPageCursor{}, Violation(StateConflict, "CURSOR_STALE")
	}
	if cursor.Until <= now.UnixMilli() {
		return SearchPageCursor{}, Violation(StateConflict, "CURSOR_EXPIRED")
	}
	// 解密临时缓冲退出时清除；返回独立Anchor，连续Open或调用方改写均互不影响。
	cursor.Anchor = append([]byte(nil), cursor.Anchor...)
````

## 第 19 步：SearchCursorCodec.Open（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 271 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return cursor, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=10721:10812 -->
````go
	return cursor, nil
}

// search_manifest.go：保留原实现，按业务合并文件。

````

## 第 20 步：SearchManifestMaxUnits

插入位置：第 19 步代码之后，紧接追加；完成稿约第 276 行。

SearchManifestMaxUnits沿冻结HTTP MainView.units/StructureRevision.entries的10000上限，
只限制异常持久清单读取规模，不另设搜索业务文档数或正文预算。

本步声明：`const SearchManifestMaxUnits = 10000`。

<!-- write-step: 20 bytes=10812:11041 -->
````go
// SearchManifestMaxUnits沿冻结HTTP MainView.units/StructureRevision.entries的10000上限，
// 只限制异常持久清单读取规模，不另设搜索业务文档数或正文预算。
const SearchManifestMaxUnits = 10000

````

## 第 21 步：SearchManifestUnit

插入位置：第 20 步代码之后，紧接追加；完成稿约第 280 行。

SearchManifestUnit是Doc私有固定清单的元数据；ByteSize用于读取正文之前预留预算。
它不包含正文/摘要或RPC DTO，读取后仍须验证实际字节、hash、schema与完整单位绑定。

本步声明：`type SearchManifestUnit struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Unit</code> | <code>uint64</code> | 清单内真实单元 |
| <code>Version</code> | <code>uint64</code> | 此单元固定正文版本 |
| <code>ByteSize</code> | <code>uint64</code> | 读正文前预留预算的元数据，随后仍核实际长度 |

<!-- write-step: 21 bytes=11041:11416 -->
````go
// SearchManifestUnit是Doc私有固定清单的元数据；ByteSize用于读取正文之前预留预算。
// 它不包含正文/摘要或RPC DTO，读取后仍须验证实际字节、hash、schema与完整单位绑定。
type SearchManifestUnit struct {
	Unit     uint64
	Version  uint64
	ByteSize uint64
}

// search_query.go：保留原实现，按业务合并文件。

````

## 第 22 步：SearchQuery

插入位置：第 21 步代码之后，紧接追加；完成稿约第 290 行。

SearchQuery是参数解析后的domain值，不是另一份HTTP/RPC DTO；SpaceID=0表示未限定空间。
HTTP limit属于Doc整页，不能拿它替换Search固定候选批大小。

本步声明：`type SearchQuery struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Text</code> | <code>string</code> | 规范搜索文本 |
| <code>Limit</code> | <code>int</code> | Doc最终页大小，不是Search候选批大小 |
| <code>SpaceID</code> | <code>uint64</code> | 限制空间ID，零为不限制 |
| <code>Cursor</code> | <code>string</code> | 客户端携带的外层加密分页令牌 |

<!-- write-step: 22 bytes=11416:11697 -->
````go
// SearchQuery是参数解析后的domain值，不是另一份HTTP/RPC DTO；SpaceID=0表示未限定空间。
// HTTP limit属于Doc整页，不能拿它替换Search固定候选批大小。
type SearchQuery struct {
	Text    string
	Limit   int
	SpaceID uint64
	Cursor  string
}

````

## 第 23 步：ParseSearchQuery

插入位置：第 22 步代码之后，紧接追加；完成稿约第 299 行。

本步声明：`func ParseSearchQuery(values url.Values) (SearchQuery, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>values</code> | <code>url.Values</code> | 在此声明中的实际使用：<code>values.Get</code>；<code>items, present := values&#91;&quot;limit&quot;&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>SearchQuery</code> | 使用 <code>SearchQuery</code> 类型；其定义处的职责：SearchQuery是参数解析后的domain值，不是另一份HTTP/RPC DTO；SpaceID=0表示未限定空间。 HTTP limit属于Doc整页，不能拿它替换Search固定候选批大小。<br>实际返回表达式：<code>SearchQuery{}</code>；<code>query</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;SEARCH_PARAMETERS_INVALID&quot;)</code>；<code>Violation(InvalidRequest, &quot;LIMIT_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(items) != 1</code>。条件成立时执行 <code>return SearchQuery{}, Violation(InvalidRequest, &quot;SEARCH_PARAMETERS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>present</code>。
- 检查 <code>len(value) &lt; 1 &#124;&#124; len(value) &gt; 3</code>。条件成立时执行 <code>return SearchQuery{}, Violation(InvalidRequest, &quot;LIMIT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; limit &lt; 1 &#124;&#124; limit &gt; 100 &#124;&#124; strconv.Itoa(limit) != value</code>。条件成立时执行 <code>return SearchQuery{}, Violation(InvalidRequest, &quot;LIMIT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>space != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return SearchQuery{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return query, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=11697:13022 -->
````go
func ParseSearchQuery(values url.Values) (SearchQuery, error) {
	// 必须先检查完整参数集合，不能用Get悄悄取重复参数的第一项或丢掉伪造身份参数。
	// handler须用url.ParseQuery并核对其error；URL.Query会先隐藏错误，domain无法恢复原字节。
	for name, items := range values {
		switch name {
		case "q", "limit", "space_id", "cursor":
			if len(items) != 1 {
				return SearchQuery{}, Violation(InvalidRequest, "SEARCH_PARAMETERS_INVALID")
			}
		default:
			return SearchQuery{}, Violation(InvalidRequest, "SEARCH_PARAMETERS_INVALID")
		}
	}
	query := SearchQuery{Text: values.Get("q"), Limit: 20, Cursor: values.Get("cursor")}
	if items, present := values["limit"]; present {
		value := items[0]
		if len(value) < 1 || len(value) > 3 {
			return SearchQuery{}, Violation(InvalidRequest, "LIMIT_INVALID")
		}
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 || strconv.Itoa(limit) != value {
			return SearchQuery{}, Violation(InvalidRequest, "LIMIT_INVALID")
		}
		query.Limit = limit
	}
	if space := values.Get("space_id"); space != "" {
		id, err := CanonicalID(space)
		if err != nil {
			return SearchQuery{}, err
		}
		query.SpaceID = id
	}
	if err := query.validate(); err != nil {
		return SearchQuery{}, err
	}
	return query, nil
}

````

## 第 24 步：SearchQuery.validate

插入位置：第 23 步代码之后，紧接追加；完成稿约第 337 行。

本步声明：`func (query SearchQuery) validate() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;SEARCH_QUERY_INVALID&quot;)</code>；<code>Violation(InvalidRequest, &quot;LIMIT_INVALID&quot;)</code>；<code>nil</code>；<code>Violation(InvalidRequest, &quot;CURSOR_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(query.Text) &gt; 500*utf8.UTFMax &#124;&#124; !utf8.ValidString(query.Text) &#124;&#124; 		utf8.RuneCountInString(query.Text) &lt; 1 &#124;&#124; utf8.RuneCountInString(query.Text) &gt; 500 &#124;&#124; 		strings.TrimSpace(query.Text) == &quot;&quot;</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;SEARCH_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>query.Limit &lt; 1 &#124;&#124; query.Limit &gt; 100</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;LIMIT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>query.Cursor == &quot;&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>len(query.Cursor) &gt; searchCursorMaxEncoded</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; base64.RawURLEncoding.EncodeToString(decoded) != query.Cursor</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=13022:14019 -->
````go
func (query SearchQuery) validate() error {
	// 先按UTF8最大字节数截住无界输入，再计Unicode码点；保留原词法，不trim/NFC改写。
	if len(query.Text) > 500*utf8.UTFMax || !utf8.ValidString(query.Text) ||
		utf8.RuneCountInString(query.Text) < 1 || utf8.RuneCountInString(query.Text) > 500 ||
		strings.TrimSpace(query.Text) == "" {
		return Violation(InvalidRequest, "SEARCH_QUERY_INVALID")
	}
	if query.Limit < 1 || query.Limit > 100 {
		return Violation(InvalidRequest, "LIMIT_INVALID")
	}
	if query.Cursor == "" {
		return nil
	}
	if len(query.Cursor) > searchCursorMaxEncoded {
		return Violation(InvalidRequest, "CURSOR_INVALID")
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(query.Cursor)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != query.Cursor {
		return Violation(InvalidRequest, "CURSOR_INVALID")
	}
	// 规范base64仍可是不可信内容；MAC、版本、字段和绝对期限留给SearchCursorCodec。
	return nil
}

````

## 第 25 步：SearchQuery.Binding

插入位置：第 24 步代码之后，紧接追加；完成稿约第 361 行。

Binding只标记已核验输入，不授予任何权限；session必须来自OptionalActor真实Proof.SessionRef，
不能传Cookie token、用户ID或HTTP自报scope。匿名与登录不能互相降级。

本步声明：`func (query SearchQuery) Binding(anonymous bool, session []byte) ([32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>anonymous</code> | <code>bool</code> | 在此声明中的实际使用：<code>anonymous &amp;&amp; len(session) != 0</code>；<code>!anonymous</code> |
| <code>session</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(session)</code>；<code>subtle.ConstantTimeCompare(session, make(&#91;&#93;byte, 32))</code>；<code>writeSearchQueryField(digest, session)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 实际返回表达式：<code>&#91;32&#93;byte{}</code>；<code>binding</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>Violation(InvalidRequest, &quot;SEARCH_BINDING_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, err</code>。当前函数在这里结束。
- 检查 <code>(anonymous &amp;&amp; len(session) != 0) &#124;&#124; (!anonymous &amp;&amp; (len(session) != 32 &#124;&#124; subtle.ConstantTimeCompare(session, make(&#91;&#93;byte, 32)) == 1))</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, Violation(InvalidRequest, &quot;SEARCH_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>anonymous</code>。
- 此处通过 <code>return binding, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=14019:15401 -->
````go
// Binding只标记已核验输入，不授予任何权限；session必须来自OptionalActor真实Proof.SessionRef，
// 不能传Cookie token、用户ID或HTTP自报scope。匿名与登录不能互相降级。
func (query SearchQuery) Binding(anonymous bool, session []byte) ([32]byte, error) {
	if err := query.validate(); err != nil {
		return [32]byte{}, err
	}
	if (anonymous && len(session) != 0) || (!anonymous && (len(session) != 32 || subtle.ConstantTimeCompare(session, make([]byte, 32)) == 1)) {
		return [32]byte{}, Violation(InvalidRequest, "SEARCH_BINDING_INVALID")
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte("collaboration/document/search-query-binding/v1\x00"))
	writeSearchQueryField(digest, []byte(query.Text))
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], uint64(query.Limit))
	writeSearchQueryField(digest, number[:])
	binary.BigEndian.PutUint64(number[:], query.SpaceID)
	writeSearchQueryField(digest, number[:])
	audience := byte(0)
	if anonymous {
		audience = 1
	}
	writeSearchQueryField(digest, []byte{audience})
	writeSearchQueryField(digest, session)
	// 显式长度前缀与版本用途标签固定各字段边界；不拼接可能撞界的分隔文本。
	// Cursor故意不入绑定，否则同一查询的下一页无法用原Binding解密。
	var binding [32]byte
	copy(binding[:], digest.Sum(nil))
	return binding, nil
}

````

## 第 26 步：writeSearchQueryField

插入位置：第 25 步代码之后，紧接追加；完成稿约第 391 行。

本步声明：`func writeSearchQueryField(digest hash.Hash, value []byte)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>digest</code> | <code>hash.Hash</code> | 在此声明中的实际使用：<code>digest.Write</code> |
| <code>value</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(value)</code>；<code>digest.Write(value)</code> |

<!-- write-step: 26 bytes=15401:15667 -->
````go
func writeSearchQueryField(digest hash.Hash, value []byte) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write(value)
}

// search_rpc.go：保留原实现，按业务合并文件。

````

## 第 27 步：EnvelopeSearchAnchor

插入位置：第 26 步代码之后，紧接追加；完成稿约第 400 行。

EnvelopeSearchAnchor只为冻结RPC选择下一批或重放，绝不解析SCB内部或验证Search的MAC。
Doc的HTTP外层另用AEAD保护；这里保留opaque原字节，位置及签名权威仍在Search。

本步声明：`func EnvelopeSearchAnchor(anchor []byte, replay bool) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>anchor</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(anchor)</code>；<code>copy(envelope&#91;header:&#93;, anchor)</code> |
| <code>replay</code> | <code>bool</code> | 是否标记为当前页重放锚点：true写类型2，false写类型1；不能把重放游标误用为继续向后翻页。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>envelope</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(anchor) == 0 &#124;&#124; len(anchor) &gt; searchCursorMaxAnchor-header</code>。条件成立时执行 <code>return nil, Violation(InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>replay</code>。
- 此处通过 <code>return envelope, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=15667:16363 -->
````go
// EnvelopeSearchAnchor只为冻结RPC选择下一批或重放，绝不解析SCB内部或验证Search的MAC。
// Doc的HTTP外层另用AEAD保护；这里保留opaque原字节，位置及签名权威仍在Search。
func EnvelopeSearchAnchor(anchor []byte, replay bool) ([]byte, error) {
	const header = 7
	if len(anchor) == 0 || len(anchor) > searchCursorMaxAnchor-header {
		return nil, Violation(InvalidRequest, "CURSOR_INVALID")
	}
	envelope := make([]byte, header+len(anchor))
	copy(envelope, []byte{'S', 'C', 'E', 1})
	envelope[4] = 1
	if replay {
		envelope[4] = 2
	}
	binary.BigEndian.PutUint16(envelope[5:7], uint16(len(anchor)))
	copy(envelope[header:], anchor)
	return envelope, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/search_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
