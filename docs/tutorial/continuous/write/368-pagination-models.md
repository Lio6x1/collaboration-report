# 分步编写 `services/search/internal/domain/cursor/pagination_models.go`

先按 [S03 · 签名批锚点、PIT与当前权限二次过滤](../lessons/search/03-candidates-cursor.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/368-pagination-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义游标链在途控制、查询范围绑定和原始ES sort三元组校验。

MySQL JSON和Go float64都可能改写高位数字词法，分页位置必须保留原字节；scope仅绑定Doc请求，不能当用户当前权限。

写之前先按这条执行路线理解各段的连接：

1. Control区分OPENING/IDLE/INFLIGHT，nonce/deadline约束在途确认
2. EncodeControl把Tail作为UTF8字符串入JSON，Decode拒绝重复根键/未知字段并恢复RawMessage
3. ValidateQuery核query摘要、500字符、page_size和scope，复制可变protobuf切片
4. ValidateSort只允许score、十进制文档字符串、uint64 shard三元组，拒绝舍入/指数/转义ID

本文件需要守住的边界：

- 不能直接json.Marshal(Control)入库，否则tail内数字会被MySQL规范化
- ScopeHash不是浏览器准入凭证，最终权限仍由Doc逐候选核验

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/domain/cursor' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/domain/cursor/pagination_models.go') -and (Get-Item -LiteralPath 'services/search/internal/domain/cursor/pagination_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/domain/cursor/pagination_models.go')) { New-Item -ItemType File -Path 'services/search/internal/domain/cursor/pagination_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:347 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package cursor

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// control.go：保留原实现，按业务合并文件。

````

## 第 2 步：PhaseOpening、PhaseIdle、PhaseInflight、MaxControlBytes

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`const (`。

<!-- write-step: 2 bytes=347:467 -->
````go
const (
	PhaseOpening    = "OPENING"
	PhaseIdle       = "IDLE"
	PhaseInflight   = "INFLIGHT"
	MaxControlBytes = 2048
)

````

## 第 3 步：Control

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

Control只保存单链最新确认位置与未知调用事实，不存批次历史。Deadline/RetryAt
均为UnixMillis；是否到期与SQL OPEN/CLOSING/CLOSED状态由持同一事务的用例决定。

本步声明：`type Control struct {`。

**这一段为什么这样写：** SQL游标链最新确认位置和一次未知调用的恢复状态。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>uint8</code> | 持久格式版本1<br>序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>Phase</code> | <code>string</code> | OPENING为申请PIT，INFLIGHT为查询/关闭在途，IDLE无在途<br>序列化/校验标记：<code>`json:&quot;phase&quot;`</code> |
| <code>Tail</code> | <code>json.RawMessage</code> | 最新完整确认批尾的原始sort JSON；不是批次历史<br>序列化/校验标记：<code>`json:&quot;tail&quot;`</code> |
| <code>Nonce</code> | <code>string</code> | 64位十六进制随机领取标记，拒绝迟到确认<br>序列化/校验标记：<code>`json:&quot;nonce&quot;`</code> |
| <code>Deadline</code> | <code>int64</code> | 当前调用Unix毫秒期限，不能超过原Expires<br>序列化/校验标记：<code>`json:&quot;deadline&quot;`</code> |
| <code>RetryAt</code> | <code>int64</code> | 下一关闭重试最早Unix毫秒时刻<br>序列化/校验标记：<code>`json:&quot;retry_at&quot;`</code> |
| <code>Uncertain</code> | <code>bool</code> | 可能存在未观测轮换PIT，已知句柄关闭也不能提前释放配额<br>序列化/校验标记：<code>`json:&quot;uncertain&quot;`</code> |

<!-- write-step: 3 bytes=467:1005 -->
````go
// Control只保存单链最新确认位置与未知调用事实，不存批次历史。Deadline/RetryAt
// 均为UnixMillis；是否到期与SQL OPEN/CLOSING/CLOSED状态由持同一事务的用例决定。
type Control struct {
	Version   uint8           `json:"version"`
	Phase     string          `json:"phase"`
	Tail      json.RawMessage `json:"tail"`
	Nonce     string          `json:"nonce"`
	Deadline  int64           `json:"deadline"`
	RetryAt   int64           `json:"retry_at"`
	Uncertain bool            `json:"uncertain"`
}

````

## 第 4 步：persistedControl

插入位置：第 3 步代码之后，紧接追加；完成稿约第 39 行。

持久格式与Go内存字段刻意区分：MySQL JSON会正规化数组中的数字，
tail必须作为UTF8字符串保存；读取时再校验内部sort，不能直接Marshal Control入库。

本步声明：`type persistedControl struct {`。

**这一段为什么这样写：** SQL JSON的安全存储形状。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>uint8</code> | 控制格式版本1<br>序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>Phase</code> | <code>string</code> | 当前阶段<br>序列化/校验标记：<code>`json:&quot;phase&quot;`</code> |
| <code>Tail</code> | <code>*string</code> | 包含原sort JSON词法的字符串指针；nil代表首批前<br>序列化/校验标记：<code>`json:&quot;tail&quot;`</code> |
| <code>Nonce</code> | <code>string</code> | 当前领取身份<br>序列化/校验标记：<code>`json:&quot;nonce&quot;`</code> |
| <code>Deadline</code> | <code>int64</code> | 当前调用绝对毫秒期限<br>序列化/校验标记：<code>`json:&quot;deadline&quot;`</code> |
| <code>RetryAt</code> | <code>int64</code> | 恢复重试时刻<br>序列化/校验标记：<code>`json:&quot;retry_at&quot;`</code> |
| <code>Uncertain</code> | <code>bool</code> | 未知PIT轮换标记<br>序列化/校验标记：<code>`json:&quot;uncertain&quot;`</code> |

<!-- write-step: 4 bytes=1005:1488 -->
````go
// 持久格式与Go内存字段刻意区分：MySQL JSON会正规化数组中的数字，
// tail必须作为UTF8字符串保存；读取时再校验内部sort，不能直接Marshal Control入库。
type persistedControl struct {
	Version   uint8   `json:"version"`
	Phase     string  `json:"phase"`
	Tail      *string `json:"tail"`
	Nonce     string  `json:"nonce"`
	Deadline  int64   `json:"deadline"`
	RetryAt   int64   `json:"retry_at"`
	Uncertain bool    `json:"uncertain"`
}

````

## 第 5 步：validControlNonce

插入位置：第 4 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`func validControlNonce(nonce string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>nonce</code> | <code>string</code> | 在此声明中的实际使用：<code>len(nonce)</code>；<code>&#91;&#93;byte(nonce)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>nonzero</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(nonce) != 64</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!((digit &gt;= &#x27;0&#x27; &amp;&amp; digit &lt;= &#x27;9&#x27;) &#124;&#124; (digit &gt;= &#x27;a&#x27; &amp;&amp; digit &lt;= &#x27;f&#x27;))</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return nonzero</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1488:1783 -->
````go
func validControlNonce(nonce string) bool {
	if len(nonce) != 64 {
		return false
	}
	nonzero := false
	for _, digit := range []byte(nonce) {
		if !((digit >= '0' && digit <= '9') || (digit >= 'a' && digit <= 'f')) {
			return false
		}
		nonzero = nonzero || digit != '0'
	}
	return nonzero
}

````

## 第 6 步：validateControl

插入位置：第 5 步代码之后，紧接追加；完成稿约第 65 行。

本步声明：`func validateControl(control Control) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>control</code> | <code>Control</code> | 使用 <code>Control</code> 类型；其定义处的职责：SQL游标链最新确认位置和一次未知调用的恢复状态。<br>在此声明中的实际使用：<code>control.Version</code>；<code>control.RetryAt</code>；<code>control.Tail</code>；<code>control.Phase</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>control.Version != 1 &#124;&#124; control.RetryAt &lt; 0 &#124;&#124; (len(control.Tail) != 0 &amp;&amp; ValidateSort(control.Tail) != nil)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>len(control.Tail) != 0</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>!validControlNonce(control.Nonce) &#124;&#124; control.Deadline &lt;= 0</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>control.Nonce != &quot;&quot; &#124;&#124; control.Deadline != 0</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1783:2375 -->
````go
func validateControl(control Control) error {
	invalid := errors.New("SEARCH_CURSOR_CONTROL_INVALID")
	if control.Version != 1 || control.RetryAt < 0 || (len(control.Tail) != 0 && ValidateSort(control.Tail) != nil) {
		return invalid
	}
	switch control.Phase {
	case PhaseOpening:
		if len(control.Tail) != 0 {
			return invalid
		}
		fallthrough
	case PhaseInflight:
		if !validControlNonce(control.Nonce) || control.Deadline <= 0 {
			return invalid
		}
	case PhaseIdle:
		if control.Nonce != "" || control.Deadline != 0 {
			return invalid
		}
	default:
		return invalid
	}
	return nil
}

````

## 第 7 步：EncodeControl

插入位置：第 6 步代码之后，紧接追加；完成稿约第 90 行。

本步声明：`func EncodeControl(control Control) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>control</code> | <code>Control</code> | 使用 <code>Control</code> 类型；其定义处的职责：SQL游标链最新确认位置和一次未知调用的恢复状态。<br>在此声明中的实际使用：<code>validateControl(control)</code>；<code>control.Version</code>；<code>control.Phase</code>；<code>control.Nonce</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>data</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_CURSOR_CONTROL_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>len(control.Tail) != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_CONTROL_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(data) &gt; MaxControlBytes</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_CONTROL_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return data, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2375:2992 -->
````go
func EncodeControl(control Control) ([]byte, error) {
	if err := validateControl(control); err != nil {
		return nil, err
	}
	wire := persistedControl{Version: control.Version, Phase: control.Phase, Nonce: control.Nonce,
		Deadline: control.Deadline, RetryAt: control.RetryAt, Uncertain: control.Uncertain}
	if len(control.Tail) != 0 {
		tail := string(control.Tail)
		wire.Tail = &tail
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, errors.New("SEARCH_CURSOR_CONTROL_INVALID")
	}
	if len(data) > MaxControlBytes {
		return nil, errors.New("SEARCH_CURSOR_CONTROL_INVALID")
	}
	return data, nil
}

````

## 第 8 步：strictControlRoot

插入位置：第 7 步代码之后，紧接追加；完成稿约第 110 行。

本步声明：`func strictControlRoot(data []byte) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>data</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(data)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>decoder.Decode(&amp;extra) == io.EOF</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; first != json.Delim(&#x27;{&#x27;)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !isKey &#124;&#124; seen&#91;key&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>decoder.Decode(&amp;value) != nil &#124;&#124; (key != &quot;tail&quot; &amp;&amp; bytes.Equal(bytes.TrimSpace(value), &#91;&#93;byte(&quot;null&quot;)))</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; last != json.Delim(&#x27;}&#x27;) &#124;&#124; !seen&#91;&quot;version&quot;&#93; &#124;&#124; !seen&#91;&quot;phase&quot;&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return decoder.Decode(&amp;extra) == io.EOF</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2992:3850 -->
````go
func strictControlRoot(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		key, isKey := token.(string)
		if err != nil || !isKey || seen[key] {
			return false
		}
		switch key {
		case "version", "phase", "tail", "nonce", "deadline", "retry_at", "uncertain":
		default:
			return false
		}
		seen[key] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil || (key != "tail" && bytes.Equal(bytes.TrimSpace(value), []byte("null"))) {
			return false
		}
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim('}') || !seen["version"] || !seen["phase"] {
		return false
	}
	var extra any
	return decoder.Decode(&extra) == io.EOF
}

````

## 第 9 步：DecodeControl

插入位置：第 8 步代码之后，紧接追加；完成稿约第 142 行。

本步声明：`func DecodeControl(data []byte) (Control, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>data</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(data)</code>；<code>utf8.Valid(data)</code>；<code>strictControlRoot(data)</code>；<code>bytes.NewReader(data)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Control</code> | 使用 <code>Control</code> 类型；其定义处的职责：SQL游标链最新确认位置和一次未知调用的恢复状态。<br>实际返回表达式：<code>control</code>；<code>Control{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(data) == 0 &#124;&#124; len(data) &gt; MaxControlBytes &#124;&#124; !utf8.Valid(data) &#124;&#124; !strictControlRoot(data)</code>。条件成立时执行 <code>return control, invalid</code>。当前函数在这里结束。
- 检查 <code>decoder.Decode(&amp;wire) != nil</code>。条件成立时执行 <code>return Control{}, invalid</code>。当前函数在这里结束。
- 检查 <code>wire.Tail != nil</code>。
- 检查 <code>*wire.Tail == &quot;&quot;</code>。条件成立时执行 <code>return Control{}, invalid</code>。当前函数在这里结束。
- 检查 <code>validateControl(control) != nil</code>。条件成立时执行 <code>return Control{}, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return control, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3850:4889 -->
````go
func DecodeControl(data []byte) (Control, error) {
	var control Control
	invalid := errors.New("SEARCH_CURSOR_CONTROL_INVALID")
	// 标准Decoder对重复键采用最后值；先检查根键，避免SQL JSON与Go恢复规则分歧。
	if len(data) == 0 || len(data) > MaxControlBytes || !utf8.Valid(data) || !strictControlRoot(data) {
		return control, invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var wire persistedControl
	if decoder.Decode(&wire) != nil {
		return Control{}, invalid
	}
	control = Control{Version: wire.Version, Phase: wire.Phase, Nonce: wire.Nonce,
		Deadline: wire.Deadline, RetryAt: wire.RetryAt, Uncertain: wire.Uncertain}
	if wire.Tail != nil {
		if *wire.Tail == "" {
			return Control{}, invalid
		}
		control.Tail = json.RawMessage(*wire.Tail)
	}
	if validateControl(control) != nil {
		return Control{}, invalid
	}
	control.Tail = append(json.RawMessage(nil), control.Tail...)
	return control, nil
}

// query.go：保留原实现，按业务合并文件。

````

## 第 10 步：Query

插入位置：第 9 步代码之后，紧接追加；完成稿约第 172 行。

Query只保存校验后的RPC输入，不代表Doc当前读取资格；实际权限由Doc最终复核。
ScopeHash是受mTLS保护的Doc绑定，不能把它当作浏览器自报身份或准入证明。

本步声明：`type Query struct {`。

**这一段为什么这样写：** 已校验且自有副本的Doc候选请求。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Query</code> | <code>string</code> | 原查询文字，不擅自trim改写摘要 |
| <code>QueryHash</code> | <code>string</code> | 原查询SHA-256十六进制 |
| <code>PageSize</code> | <code>uint32</code> | Search固定候选批大小，上层要求100 |
| <code>Scope</code> | <code>*pb.SearchScope</code> | Doc认证的scope/session/space绑定副本 |

<!-- write-step: 10 bytes=4889:5202 -->
````go
// Query只保存校验后的RPC输入，不代表Doc当前读取资格；实际权限由Doc最终复核。
// ScopeHash是受mTLS保护的Doc绑定，不能把它当作浏览器自报身份或准入证明。
type Query struct {
	Query     string
	QueryHash string
	PageSize  uint32
	Scope     *pb.SearchScope
}

````

## 第 11 步：QueryDigest

插入位置：第 10 步代码之后，紧接追加；完成稿约第 181 行。

本步声明：`func QueryDigest(query string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>query</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(query)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>hex.EncodeToString(digest&#91;:&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return hex.EncodeToString(digest&#91;:&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5202:5323 -->
````go
func QueryDigest(query string) string {
	digest := sha256.Sum256([]byte(query))
	return hex.EncodeToString(digest[:])
}

````

## 第 12 步：canonicalDigest

插入位置：第 11 步代码之后，紧接追加；完成稿约第 186 行。

本步声明：`func canonicalDigest(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>hex.DecodeString(value)</code>；<code>hex.EncodeToString(decoded) == value</code>；<code>value != strings.Repeat(&quot;0&quot;, sha256.Size*2)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>err == nil &amp;&amp; len(decoded) == sha256.Size &amp;&amp; hex.EncodeToString(decoded) == value &amp;&amp; 		value != strings.Repeat(&quot;0&quot;, sha256.Size*2)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return err == nil &amp;&amp; len(decoded) == sha256.Size &amp;&amp; hex.EncodeToString(decoded) == value &amp;&amp; 		value != strings.Repeat(&quot;0&quot;, sha256.Size*2)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5323:5548 -->
````go
func canonicalDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == value &&
		value != strings.Repeat("0", sha256.Size*2)
}

````

## 第 13 步：ValidateQuery

插入位置：第 12 步代码之后，紧接追加；完成稿约第 192 行。

本步声明：`func ValidateQuery(request *pb.SearchCandidatesRequest) (Query, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>request</code> | <code>*pb.SearchCandidatesRequest</code> | 在此声明中的实际使用：<code>request == nil</code>；<code>request.Query</code>；<code>request.QueryDigest</code>；<code>request.PageSize</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Query</code> | 使用 <code>Query</code> 类型；其定义处的职责：已校验且自有副本的Doc候选请求。<br>实际返回表达式：<code>Query{}</code>；<code>Query{Query: request.Query, QueryHash: request.QueryDigest, PageSize: request.PageSize, Scope: scope}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>request == nil &#124;&#124; len(request.Query) &gt; 500*utf8.UTFMax &#124;&#124; !utf8.ValidString(request.Query) &#124;&#124; utf8.RuneCountInString(request.Query) &gt; 500 &#124;&#124; 		strings.TrimSpace(request.Query) == &quot; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return Query{}, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return Query{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return Query{Query: request.Query, QueryHash: request.QueryDigest, PageSize: request.PageSize, Scope: scope}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5548:6574 -->
````go
func ValidateQuery(request *pb.SearchCandidatesRequest) (Query, error) {
	invalid := errors.New("SEARCH_QUERY_BINDING_INVALID")
	if request == nil || len(request.Query) > 500*utf8.UTFMax || !utf8.ValidString(request.Query) || utf8.RuneCountInString(request.Query) > 500 ||
		strings.TrimSpace(request.Query) == "" || request.QueryDigest != QueryDigest(request.Query) ||
		request.PageSize == 0 || request.PageSize > 100 || len(request.Cursor) > MaxEnvelopeBytes {
		return Query{}, invalid
	}
	if err := ValidateScope(request.Scope); err != nil {
		return Query{}, err
	}
	// 自有副本阻止调用者在SQL绑定后改变space/session；禁止依赖共享protobuf可变数组。
	scope := &pb.SearchScope{ScopeHash: request.Scope.ScopeHash, Anonymous: request.Scope.Anonymous,
		SessionRef: append([]byte(nil), request.Scope.SessionRef...),
		SpaceIds:   append([]uint64(nil), request.Scope.SpaceIds...)}
	return Query{Query: request.Query, QueryHash: request.QueryDigest, PageSize: request.PageSize, Scope: scope}, nil
}

````

## 第 14 步：ValidateScope

插入位置：第 13 步代码之后，紧接追加；完成稿约第 209 行。

CloseCursor与查询共用同一个scope边界。位置验签及SQL中的原scope/session仍由用例核对。

本步声明：`func ValidateScope(scope *pb.SearchScope) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>scope</code> | <code>*pb.SearchScope</code> | 在此声明中的实际使用：<code>scope == nil</code>；<code>scope.ScopeHash</code>；<code>scope.SpaceIds</code>；<code>scope.Anonymous</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_SCOPE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>scope == nil &#124;&#124; !canonicalDigest(scope.ScopeHash) &#124;&#124; len(scope.SpaceIds) &gt; 1024</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_SCOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>scope.Anonymous</code>。
- 检查 <code>len(scope.SessionRef) != 0</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_SCOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(scope.SessionRef) != sha256.Size</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_SCOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>string(scope.SessionRef) == string(zero&#91;:&#93;)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_SCOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>space == 0 &#124;&#124; (i &gt; 0 &amp;&amp; space &lt;= scope.SpaceIds&#91;i-1&#93;)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_SCOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6574:7436 -->
````go
// CloseCursor与查询共用同一个scope边界。位置验签及SQL中的原scope/session仍由用例核对。
func ValidateScope(scope *pb.SearchScope) error {
	if scope == nil || !canonicalDigest(scope.ScopeHash) || len(scope.SpaceIds) > 1024 {
		return errors.New("SEARCH_SCOPE_INVALID")
	}
	if scope.Anonymous {
		if len(scope.SessionRef) != 0 {
			return errors.New("SEARCH_SCOPE_INVALID")
		}
	} else {
		if len(scope.SessionRef) != sha256.Size {
			return errors.New("SEARCH_SCOPE_INVALID")
		}
		var zero [sha256.Size]byte
		if string(scope.SessionRef) == string(zero[:]) {
			return errors.New("SEARCH_SCOPE_INVALID")
		}
	}
	for i, space := range scope.SpaceIds {
		if space == 0 || (i > 0 && space <= scope.SpaceIds[i-1]) {
			return errors.New("SEARCH_SCOPE_INVALID")
		}
	}
	return nil
}

// sort.go：保留原实现，按业务合并文件。

````

## 第 15 步：ValidateSort

插入位置：第 14 步代码之后，紧接追加；完成稿约第 237 行。

ValidateSort只检查固定score/Doc/shard三元组，原JSON始终由锚点原样保存。
uint64不能解成float64再输出，否则2^53以上位置会移动、导致分页漏项或重复。

本步声明：`func ValidateSort(raw json.RawMessage) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>在此声明中的实际使用：<code>len(raw)</code>；<code>utf8.Valid(raw)</code>；<code>json.Unmarshal(raw, &amp;tuple)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) == 0 &#124;&#124; len(raw) &gt; MaxSortBytes &#124;&#124; !utf8.Valid(raw)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;tuple) != nil &#124;&#124; len(tuple) != 3</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>decoder.Decode(&amp;scoreValue) != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>!number</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; math.IsNaN(finite) &#124;&#124; math.IsInf(finite, 0)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>len(document) &lt; 3 &#124;&#124; document&#91;0&#93; != &#x27;&quot;&#x27; &#124;&#124; document&#91;len(document)-1&#93; != &#x27;&quot;&#x27; &#124;&#124; 		!canonicalUnsigned(document&#91;1:len(document)-1&#93;, false)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>!canonicalUnsigned(bytes.TrimSpace(tuple&#91;2&#93;), true)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7436:8680 -->
````go
// ValidateSort只检查固定score/Doc/shard三元组，原JSON始终由锚点原样保存。
// uint64不能解成float64再输出，否则2^53以上位置会移动、导致分页漏项或重复。
func ValidateSort(raw json.RawMessage) error {
	invalid := errors.New("SEARCH_CURSOR_SORT_INVALID")
	if len(raw) == 0 || len(raw) > MaxSortBytes || !utf8.Valid(raw) {
		return invalid
	}
	var tuple []json.RawMessage
	if json.Unmarshal(raw, &tuple) != nil || len(tuple) != 3 {
		return invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(tuple[0]))
	decoder.UseNumber()
	var scoreValue any
	if decoder.Decode(&scoreValue) != nil {
		return invalid
	}
	score, number := scoreValue.(json.Number)
	if !number {
		return invalid
	}
	// score转数值只为排除不可表示的无穷值，绝不据此重写已绑定的原sort词法。
	finite, err := score.Float64()
	if err != nil || math.IsNaN(finite) || math.IsInf(finite, 0) {
		return invalid
	}
	document := bytes.TrimSpace(tuple[1])
	if len(document) < 3 || document[0] != '"' || document[len(document)-1] != '"' ||
		!canonicalUnsigned(document[1:len(document)-1], false) {
		return invalid
	}
	if !canonicalUnsigned(bytes.TrimSpace(tuple[2]), true) {
		return invalid
	}
	return nil
}

````

## 第 16 步：canonicalUnsigned

插入位置：第 15 步代码之后，紧接追加；完成稿约第 274 行。

直接核对十进制词法与完整范围，拒绝转义Doc字符串、指数/小数shard、前导零及回绕。

本步声明：`func canonicalUnsigned(raw []byte, allowZero bool) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>raw&#91;0&#93; == &#x27;0&#x27;</code>；<code>string(raw)</code> |
| <code>allowZero</code> | <code>bool</code> | 在此声明中的实际使用：<code>!allowZero</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) == 0 &#124;&#124; len(raw) &gt; 20 &#124;&#124; (raw&#91;0&#93; == &#x27;0&#x27; &amp;&amp; (len(raw) != 1 &#124;&#124; !allowZero))</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>digit &lt; &#x27;0&#x27; &#124;&#124; digit &gt; &#x27;9&#x27;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=8680:9120 -->
````go
// 直接核对十进制词法与完整范围，拒绝转义Doc字符串、指数/小数shard、前导零及回绕。
func canonicalUnsigned(raw []byte, allowZero bool) bool {
	if len(raw) == 0 || len(raw) > 20 || (raw[0] == '0' && (len(raw) != 1 || !allowZero)) {
		return false
	}
	for _, digit := range raw {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(string(raw), 10, 64)
	return err == nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/domain/cursor/pagination_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/03-candidates-cursor.md)。
