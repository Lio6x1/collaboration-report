# 分步编写 `services/search/internal/domain/cursor/anchor.go`

先按 [S03 · 签名批锚点、PIT与当前权限二次过滤](../lessons/search/03-candidates-cursor.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/366-anchor.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现带用途隔离HMAC的完整候选批锚点和NEXT/REPLAY信封。

续接只能引用已经完整取得的批次，不能让客户端自由填写任意search_after或直接持有PIT。

写之前先按这条执行路线理解各段的连接：

1. Batch绑定随机chain、before/after原sort、count/more和完整候选摘要
2. Codec.Seal编码固定magic/长度并HMAC；Open验签后复制sort字节
3. Envelope只携动作和已签锚点，ParseEnvelope后仍必须Open验签
4. query/scope/session/physical/期限保存在chain对应SQL行

本文件需要守住的边界：

- More=true必须满100候选，0条不能移动after
- 全零签名key拒绝；token不存PIT
- 信封动作未签名但只允许受信Doc在有效锚点上选择

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/domain/cursor' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/domain/cursor/anchor.go') -and (Get-Item -LiteralPath 'services/search/internal/domain/cursor/anchor.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/domain/cursor/anchor.go')) { New-Item -ItemType File -Path 'services/search/internal/domain/cursor/anchor.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:132 -->
````go
package cursor

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
)

````

## 第 2 步：MaxEnvelopeBytes、MaxSortBytes、anchorHeader、anchorMinimum、envelopeHeader、anchorPurpose

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`const (`。

<!-- write-step: 2 bytes=132:347 -->
````go
const (
	MaxEnvelopeBytes = 768
	MaxSortBytes     = 128
	anchorHeader     = 70
	anchorMinimum    = anchorHeader + 2 + sha256.Size
	envelopeHeader   = 7
	anchorPurpose    = "search-candidates-batch-anchor-v1\x00"
)

````

## 第 3 步：anchorMagic、envelopeMagic

插入位置：第 2 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`var (`。

<!-- write-step: 3 bytes=347:442 -->
````go
var (
	anchorMagic   = [4]byte{'S', 'C', 'B', 1}
	envelopeMagic = [4]byte{'S', 'C', 'E', 1}
)

````

## 第 4 步：Action

插入位置：第 3 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`type Action uint8`。

<!-- write-step: 4 bytes=442:461 -->
````go
type Action uint8

````

## 第 5 步：ActionNext、ActionReplay

插入位置：第 4 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`const (`。

<!-- write-step: 5 bytes=461:522 -->
````go
const (
	ActionNext   Action = 1
	ActionReplay Action = 2
)

````

## 第 6 步：Batch

插入位置：第 5 步代码之后，紧接追加；完成稿约第 34 行。

Batch绑定一个已经完整取得的批次，而非可随意指定的ES位置。nil位置表示首批起点。
Query/scope/session/物理代次/绝对期限由Chain对应SQL行绑定，不重复塞进签名token。
Digest由上层对完整有序候选计算；此层仅检查非零并签名，不能代替ES完整性验证。

本步声明：`type Batch struct {`。

**这一段为什么这样写：** 不可随意改写的一个完整候选批证据。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Chain</code> | <code>&#91;32&#93;byte</code> | 随机32字节链秘密标识，SQL只存其hash |
| <code>Before</code> | <code>json.RawMessage</code> | 本批搜索前的位置；nil为首批 |
| <code>After</code> | <code>json.RawMessage</code> | 本批末候选原sort；空批与Before相等 |
| <code>Count</code> | <code>uint8</code> | 本批条数0..100 |
| <code>More</code> | <code>bool</code> | 是否存在经验证的第101个探测候选 |
| <code>Digest</code> | <code>&#91;32&#93;byte</code> | 完整有序候选及边界的SHA-256 |

<!-- write-step: 6 bytes=522:991 -->
````go
// Batch绑定一个已经完整取得的批次，而非可随意指定的ES位置。nil位置表示首批起点。
// Query/scope/session/物理代次/绝对期限由Chain对应SQL行绑定，不重复塞进签名token。
// Digest由上层对完整有序候选计算；此层仅检查非零并签名，不能代替ES完整性验证。
type Batch struct {
	Chain  [32]byte
	Before json.RawMessage
	After  json.RawMessage
	Count  uint8
	More   bool
	Digest [32]byte
}

````

## 第 7 步：Codec

插入位置：第 6 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`type Codec struct{ key []byte }`。

**这一段为什么这样写：** 持有HMAC主密钥的签名器。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>&#91;&#93;byte</code> | 32字节非全零密钥的自有副本，不写token或日志 |

<!-- write-step: 7 bytes=991:1024 -->
````go
type Codec struct{ key []byte }

````

## 第 8 步：NewCodec

插入位置：第 7 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func NewCodec(key []byte) (*Codec, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(key)</code>；<code>subtle.ConstantTimeCompare(key, zero&#91;:&#93;)</code>；<code>key: append(&#91;&#93;byte(nil), key...)</code>；<code>append(&#91;&#93;byte(nil), key...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Codec</code> | 使用 <code>Codec</code> 类型；其定义处的职责：持有HMAC主密钥的签名器。<br>实际返回表达式：<code>nil</code>；<code>&amp;Codec{key: append(&#91;&#93;byte(nil), key...)}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_KEY_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(key) != sha256.Size</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_KEY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>subtle.ConstantTimeCompare(key, zero&#91;:&#93;) == 1</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_KEY_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Codec{key: append(&#91;&#93;byte(nil), key...)}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1024:1450 -->
````go
func NewCodec(key []byte) (*Codec, error) {
	if len(key) != sha256.Size {
		return nil, errors.New("SEARCH_CURSOR_KEY_INVALID")
	}
	// 全零master是公开已知密钥；固定时间比较避免按首个非零字节提前返回。
	var zero [sha256.Size]byte
	if subtle.ConstantTimeCompare(key, zero[:]) == 1 {
		return nil, errors.New("SEARCH_CURSOR_KEY_INVALID")
	}
	return &Codec{key: append([]byte(nil), key...)}, nil
}

````

## 第 9 步：validateBatch

插入位置：第 8 步代码之后，紧接追加；完成稿约第 60 行。

本步声明：`func validateBatch(batch Batch) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>batch</code> | <code>Batch</code> | 使用 <code>Batch</code> 类型；其定义处的职责：不可随意改写的一个完整候选批证据。<br>在此声明中的实际使用：<code>batch.Chain</code>；<code>batch.Digest</code>；<code>batch.Count</code>；<code>batch.More</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_BATCH_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>batch.Chain == (&#91;32&#93;byte{}) &#124;&#124; batch.Digest == (&#91;32&#93;byte{}) &#124;&#124; batch.Count &gt; 100 &#124;&#124; 		(batch.More &amp;&amp; batch.Count != 100)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_BATCH_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(position) != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>(batch.Count == 0 &amp;&amp; (!bytes.Equal(batch.Before, batch.After) &#124;&#124; batch.More)) &#124;&#124; 		(batch.Count != 0 &amp;&amp; len(batch.After) == 0)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_BATCH_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=1450:2049 -->
````go
func validateBatch(batch Batch) error {
	if batch.Chain == ([32]byte{}) || batch.Digest == ([32]byte{}) || batch.Count > 100 ||
		(batch.More && batch.Count != 100) {
		return errors.New("SEARCH_CURSOR_BATCH_INVALID")
	}
	for _, position := range []json.RawMessage{batch.Before, batch.After} {
		if len(position) != 0 {
			if err := ValidateSort(position); err != nil {
				return err
			}
		}
	}
	if (batch.Count == 0 && (!bytes.Equal(batch.Before, batch.After) || batch.More)) ||
		(batch.Count != 0 && len(batch.After) == 0) {
		return errors.New("SEARCH_CURSOR_BATCH_INVALID")
	}
	return nil
}

````

## 第 10 步：Codec.signature

插入位置：第 9 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func (c *Codec) signature(data []byte) []byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>data</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>mac.Write(data)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>mac.Sum(nil)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return mac.Sum(nil)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2049:2208 -->
````go
func (c *Codec) signature(data []byte) []byte {
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(anchorPurpose))
	mac.Write(data)
	return mac.Sum(nil)
}

````

## 第 11 步：Codec.Seal

插入位置：第 10 步代码之后，紧接追加；完成稿约第 86 行。

本步声明：`func (c *Codec) Seal(batch Batch) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>batch</code> | <code>Batch</code> | 使用 <code>Batch</code> 类型；其定义处的职责：不可随意改写的一个完整候选批证据。<br>在此声明中的实际使用：<code>validateBatch(batch)</code>；<code>batch.Before</code>；<code>batch.After</code>；<code>batch.Chain</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>append(data, c.signature(data)...)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_KEY_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>c == nil &#124;&#124; len(c.key) != sha256.Size</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_KEY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>batch.More</code>。
- 此处通过 <code>return append(data, c.signature(data)...), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=2208:2917 -->
````go
func (c *Codec) Seal(batch Batch) ([]byte, error) {
	if c == nil || len(c.key) != sha256.Size {
		return nil, errors.New("SEARCH_CURSOR_KEY_INVALID")
	}
	if err := validateBatch(batch); err != nil {
		return nil, err
	}
	data := make([]byte, 0, anchorMinimum+len(batch.Before)+len(batch.After))
	data = append(data, anchorMagic[:]...)
	data = append(data, batch.Chain[:]...)
	more := byte(0)
	if batch.More {
		more = 1
	}
	data = append(data, batch.Count, more)
	data = append(data, batch.Digest[:]...)
	for _, position := range []json.RawMessage{batch.Before, batch.After} {
		data = append(data, byte(len(position)))
		data = append(data, position...)
	}
	return append(data, c.signature(data)...), nil
}

````

## 第 12 步：Codec.Open

插入位置：第 11 步代码之后，紧接追加；完成稿约第 109 行。

本步声明：`func (c *Codec) Open(raw []byte) (Batch, error)`。

**这一段为什么这样写：** 只接受合法签名与精确二进制布局的原批次。

1. 核magic/长度
2. 常量时间验HMAC
3. 读取并复制before/after
4. 核批语义及无多余字节


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 不含信封的已签锚点字节<br>在此声明中的实际使用：<code>len(raw)</code>；<code>bytes.Equal(raw&#91;:4&#93;, anchorMagic&#91;:&#93;)</code>；<code>data, signature := raw&#91;:len(raw)-sha256.Size&#93;, raw&#91;len(raw)-sha256.Size:&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Batch</code> | 验证过的Batch<br>实际返回表达式：<code>Batch{}</code>；<code>batch</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 任何修改/格式错误均拒绝<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>c == nil &#124;&#124; len(c.key) != sha256.Size &#124;&#124; len(raw) &lt; anchorMinimum &#124;&#124; 		len(raw) &gt; anchorMinimum+2*MaxSortBytes &#124;&#124; !bytes.Equal(raw&#91;:4&#93;, anchorMagic&#91;:&#93;)</code>。条件成立时执行 <code>return Batch{}, invalid</code>。当前函数在这里结束。
- 检查 <code>!hmac.Equal(signature, c.signature(data)) &#124;&#124; data&#91;37&#93; &gt; 1</code>。条件成立时执行 <code>return Batch{}, invalid</code>。当前函数在这里结束。
- 检查 <code>offset &gt;= len(data)</code>。条件成立时执行 <code>return Batch{}, invalid</code>。当前函数在这里结束。
- 检查 <code>length &gt; MaxSortBytes &#124;&#124; length &gt; len(data)-offset</code>。条件成立时执行 <code>return Batch{}, invalid</code>。当前函数在这里结束。
- 检查 <code>offset != len(data) &#124;&#124; validateBatch(batch) != nil</code>。条件成立时执行 <code>return Batch{}, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return batch, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=2917:4117 -->
````go
func (c *Codec) Open(raw []byte) (Batch, error) {
	invalid := errors.New("SEARCH_CURSOR_ANCHOR_INVALID")
	if c == nil || len(c.key) != sha256.Size || len(raw) < anchorMinimum ||
		len(raw) > anchorMinimum+2*MaxSortBytes || !bytes.Equal(raw[:4], anchorMagic[:]) {
		return Batch{}, invalid
	}
	data, signature := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	if !hmac.Equal(signature, c.signature(data)) || data[37] > 1 {
		return Batch{}, invalid
	}
	var batch Batch
	copy(batch.Chain[:], data[4:36])
	batch.Count, batch.More = data[36], data[37] == 1
	copy(batch.Digest[:], data[38:anchorHeader])
	offset := anchorHeader
	for _, position := range []*json.RawMessage{&batch.Before, &batch.After} {
		if offset >= len(data) {
			return Batch{}, invalid
		}
		length := int(data[offset])
		offset++
		if length > MaxSortBytes || length > len(data)-offset {
			return Batch{}, invalid
		}
		// 返回值属于调用方；不能让后续修改sort改变验签原输入或另一次回放。
		*position = append(json.RawMessage(nil), data[offset:offset+length]...)
		offset += length
	}
	if offset != len(data) || validateBatch(batch) != nil {
		return Batch{}, invalid
	}
	return batch, nil
}

````

## 第 13 步：validAction

插入位置：第 12 步代码之后，紧接追加；完成稿约第 143 行。

本步声明：`func validAction(action Action) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>action</code> | <code>Action</code> | 在此声明中的实际使用：<code>action == ActionNext</code>；<code>action == ActionReplay</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>action == ActionNext &#124;&#124; action == ActionReplay</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return action == ActionNext &#124;&#124; action == ActionReplay</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=4117:4213 -->
````go
func validAction(action Action) bool { return action == ActionNext || action == ActionReplay }

````

## 第 14 步：Envelope

插入位置：第 13 步代码之后，紧接追加；完成稿约第 145 行。

动作由受信Doc在已签锚点上选择；这里只检查有界framing，调用方随后必须Codec.Open。
空RPC cursor的首次建链不经过本函数，不能把无锚点请求解释为合法续接。

本步声明：`func Envelope(action Action, anchor []byte) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>action</code> | <code>Action</code> | 在此声明中的实际使用：<code>validAction(action)</code>；<code>byte(action)</code> |
| <code>anchor</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(anchor)</code>；<code>append(raw, anchor...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>append(raw, anchor...)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_ENVELOPE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!validAction(action) &#124;&#124; len(anchor) == 0 &#124;&#124; len(anchor) &gt; MaxEnvelopeBytes-envelopeHeader</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_ENVELOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return append(raw, anchor...), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=4213:4863 -->
````go
// 动作由受信Doc在已签锚点上选择；这里只检查有界framing，调用方随后必须Codec.Open。
// 空RPC cursor的首次建链不经过本函数，不能把无锚点请求解释为合法续接。
func Envelope(action Action, anchor []byte) ([]byte, error) {
	if !validAction(action) || len(anchor) == 0 || len(anchor) > MaxEnvelopeBytes-envelopeHeader {
		return nil, errors.New("SEARCH_CURSOR_ENVELOPE_INVALID")
	}
	raw := make([]byte, envelopeHeader, envelopeHeader+len(anchor))
	copy(raw, envelopeMagic[:])
	raw[4] = byte(action)
	binary.BigEndian.PutUint16(raw[5:7], uint16(len(anchor)))
	return append(raw, anchor...), nil
}

````

## 第 15 步：ParseEnvelope

插入位置：第 14 步代码之后，紧接追加；完成稿约第 158 行。

本步声明：`func ParseEnvelope(raw []byte) (Action, []byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>bytes.Equal(raw&#91;:4&#93;, envelopeMagic&#91;:&#93;)</code>；<code>Action(raw&#91;4&#93;)</code>；<code>binary.BigEndian.Uint16(raw&#91;5:7&#93;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Action</code> | 实际返回表达式：<code>0</code>；<code>Action(raw&#91;4&#93;)</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>append(&#91;&#93;byte(nil), raw&#91;envelopeHeader:&#93;...)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_ENVELOPE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) &lt;= envelopeHeader &#124;&#124; len(raw) &gt; MaxEnvelopeBytes &#124;&#124; 		!bytes.Equal(raw&#91;:4&#93;, envelopeMagic&#91;:&#93;) &#124;&#124; !validAction(Action(raw&#91;4&#93;)) &#124;&#124; 		int(binary.BigEndian.Uint16(raw&#91;5:7&#93;)) != len(raw)-envelopeHeader</code>。条件成立时执行 <code>return 0, nil, errors.New(&quot;SEARCH_CURSOR_ENVELOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return Action(raw&#91;4&#93;), append(&#91;&#93;byte(nil), raw&#91;envelopeHeader:&#93;...), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=4863:5273 -->
````go
func ParseEnvelope(raw []byte) (Action, []byte, error) {
	if len(raw) <= envelopeHeader || len(raw) > MaxEnvelopeBytes ||
		!bytes.Equal(raw[:4], envelopeMagic[:]) || !validAction(Action(raw[4])) ||
		int(binary.BigEndian.Uint16(raw[5:7])) != len(raw)-envelopeHeader {
		return 0, nil, errors.New("SEARCH_CURSOR_ENVELOPE_INVALID")
	}
	return Action(raw[4]), append([]byte(nil), raw[envelopeHeader:]...), nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/domain/cursor/anchor.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/03-candidates-cursor.md)。
