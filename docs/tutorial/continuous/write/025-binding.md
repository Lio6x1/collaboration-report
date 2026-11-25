# 分步编写 `packages/platform/rebuildmeta/binding.go`

先按 [F05 · 重建元数据、签名与原子制品](../lessons/foundation/05-rebuild-artifacts.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/025-binding.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义搜索重建原准备元数据和封存摘要，生成唯一规范字节。

源代次、目标代次和三分区起点L一起固定，重试不能重新读取当前位置覆盖原输入。

写之前先按这条执行路线理解各段的连接：

1. Prepare.Bytes核验版本1、owner/物理索引名互异、非零topic UUID、随机nonce及target=source+1。
2. ParsePrepare/ParseSeal重新编码逐字节比较，拒绝重复键、重排、未知字段和非规范数字。
3. Seal通过PrepareRef内容地址绑定完整准备，不复制易漂移字段。

本文件需要守住的边界：

- uint64/offset用字符串；offset不可超过MaxInt64，source/revision最大值拒绝，防递增溢出。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform/rebuildmeta' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/rebuildmeta/binding.go') -and (Get-Item -LiteralPath 'packages/platform/rebuildmeta/binding.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/rebuildmeta/binding.go')) { New-Item -ItemType File -Path 'packages/platform/rebuildmeta/binding.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:141 -->
````go
package rebuildmeta

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

````

## 第 2 步：Prepare

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

Prepare的整数采用字符串。原L与源/目标代次一起签名，不能靠两台机器的时间戳证明先后。

本步声明：`type Prepare struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>int</code> | 元数据格式版本，当前必须1。<br>序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>Operation</code> | <code>string</code> | 原搜索重建操作号。<br>序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>Owner</code> | <code>string</code> | 部署归属标识，限制制品与索引所属实例。<br>序列化/校验标记：<code>`json:&quot;owner&quot;`</code> |
| <code>Alias</code> | <code>string</code> | 将来原子切换的查询别名，不得等于物理索引名。<br>序列化/校验标记：<code>`json:&quot;alias&quot;`</code> |
| <code>Source</code> | <code>string</code> | 准备时当前活跃物理索引。<br>序列化/校验标记：<code>`json:&quot;source_index&quot;`</code> |
| <code>SourceGeneration</code> | <code>string</code> | 源索引物理代次，正uint64字符串且可递增。<br>序列化/校验标记：<code>`json:&quot;source_generation&quot;`</code> |
| <code>SourceRevision</code> | <code>string</code> | 源索引控制修订，用于拒绝准备后并发切换。<br>序列化/校验标记：<code>`json:&quot;source_revision&quot;`</code> |
| <code>Target</code> | <code>string</code> | 本次独占的新物理索引，不能复用源或别名。<br>序列化/校验标记：<code>`json:&quot;target_index&quot;`</code> |
| <code>TargetGeneration</code> | <code>string</code> | 严格等于SourceGeneration+1。<br>序列化/校验标记：<code>`json:&quot;target_generation&quot;`</code> |
| <code>Cluster</code> | <code>string</code> | Kafka真实cluster ID，不能仅靠broker地址代替。<br>序列化/校验标记：<code>`json:&quot;cluster_id&quot;`</code> |
| <code>Topic</code> | <code>string</code> | 事件主题名。<br>序列化/校验标记：<code>`json:&quot;topic&quot;`</code> |
| <code>TopicID</code> | <code>string</code> | 16字节非零Kafka主题UUID的hex，识别同名重建。<br>序列化/校验标记：<code>`json:&quot;topic_id&quot;`</code> |
| <code>Starts</code> | <code>&#91;3&#93;string</code> | 按分区0/1/2排序的初始L offset，非负十进制字符串。<br>序列化/校验标记：<code>`json:&quot;starts&quot;`</code> |
| <code>Nonce</code> | <code>string</code> | 32字节随机nonce的hex，让原准备身份不可混用。<br>序列化/校验标记：<code>`json:&quot;nonce&quot;`</code> |

<!-- write-step: 2 bytes=141:958 -->
````go
// Prepare的整数采用字符串。原L与源/目标代次一起签名，不能靠两台机器的时间戳证明先后。
type Prepare struct {
	Version          int       `json:"version"`
	Operation        string    `json:"operation_id"`
	Owner            string    `json:"owner"`
	Alias            string    `json:"alias"`
	Source           string    `json:"source_index"`
	SourceGeneration string    `json:"source_generation"`
	SourceRevision   string    `json:"source_revision"`
	Target           string    `json:"target_index"`
	TargetGeneration string    `json:"target_generation"`
	Cluster          string    `json:"cluster_id"`
	Topic            string    `json:"topic"`
	TopicID          string    `json:"topic_id"`
	Starts           [3]string `json:"starts"`
	Nonce            string    `json:"nonce"`
}

````

## 第 3 步：Seal

插入位置：第 2 步代码之后，紧接追加；完成稿约第 33 行。

Seal通过完整原prepare内容地址绑定owner、operation、代次和L，避免复制两份易漂移事实。

本步声明：`type Seal struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>int</code> | 封存格式版本，必须1。<br>序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>PrepareRef</code> | <code>string</code> | 原prepare envelope内容地址，隐含其完整owner/operation/代次/L。<br>序列化/校验标记：<code>`json:&quot;prepare_ref&quot;`</code> |
| <code>Count</code> | <code>string</code> | 全部规范NDJSON条目数的十进制字符串。<br>序列化/校验标记：<code>`json:&quot;entry_count&quot;`</code> |
| <code>EntriesDigest</code> | <code>string</code> | 完整entries.ndjson原字节SHA256 hex。<br>序列化/校验标记：<code>`json:&quot;entries_digest&quot;`</code> |

<!-- write-step: 3 bytes=958:1268 -->
````go
// Seal通过完整原prepare内容地址绑定owner、operation、代次和L，避免复制两份易漂移事实。
type Seal struct {
	Version       int    `json:"version"`
	PrepareRef    string `json:"prepare_ref"`
	Count         string `json:"entry_count"`
	EntriesDigest string `json:"entries_digest"`
}

````

## 第 4 步：operationName

插入位置：第 3 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`var operationName = regexp.MustCompile(\`^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$\`)`。

<!-- write-step: 4 bytes=1268:1345 -->
````go
var operationName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$`)
````

## 第 5 步：physicalName

插入位置：第 4 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`var physicalName = regexp.MustCompile(\`^[a-z0-9][a-z0-9._-]{0,159}$\`)`。

<!-- write-step: 5 bytes=1345:1415 -->
````go
var physicalName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)
````

## 第 6 步：ownerName

插入位置：第 5 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`var ownerName = regexp.MustCompile(\`^[A-Za-z0-9_.-]{1,64}$\`)`。

<!-- write-step: 6 bytes=1415:1476 -->
````go
var ownerName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
````

## 第 7 步：topicName

插入位置：第 6 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`var topicName = regexp.MustCompile(\`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$\`)`。

<!-- write-step: 7 bytes=1476:1550 -->
````go
var topicName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$`)

````

## 第 8 步：OperationValid

插入位置：第 7 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`func OperationValid(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>operationName.MatchString(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>operationName.MatchString(value)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return operationName.MatchString(value)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1550:1634 -->
````go
func OperationValid(value string) bool { return operationName.MatchString(value) }

````

## 第 9 步：Decimal

插入位置：第 8 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func Decimal(value string, positive bool) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>strconv.ParseUint(value, 10, 64)</code>；<code>strconv.FormatUint(n, 10) != value</code> |
| <code>positive</code> | <code>bool</code> | 在此声明中的实际使用：<code>positive &amp;&amp; n == 0</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>n</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;REBUILD_NUMBER_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; strconv.FormatUint(n, 10) != value &#124;&#124; (positive &amp;&amp; n == 0)</code>。条件成立时执行 <code>return 0, errors.New(&quot;REBUILD_NUMBER_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return n, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=1634:1887 -->
````go
func Decimal(value string, positive bool) (uint64, error) {
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil || strconv.FormatUint(n, 10) != value || (positive && n == 0) {
		return 0, errors.New("REBUILD_NUMBER_INVALID")
	}
	return n, nil
}

````

## 第 10 步：Hex

插入位置：第 9 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`func Hex(value string, size int) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>hex.DecodeString(value)</code>；<code>hex.EncodeToString(decoded) == value</code> |
| <code>size</code> | <code>int</code> | 在此声明中的实际使用：<code>len(decoded) == size</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>err == nil &amp;&amp; len(decoded) == size &amp;&amp; hex.EncodeToString(decoded) == value</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return err == nil &amp;&amp; len(decoded) == size &amp;&amp; hex.EncodeToString(decoded) == value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=1887:2054 -->
````go
func Hex(value string, size int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == size && hex.EncodeToString(decoded) == value
}

````

## 第 11 步：Prepare.Bytes

插入位置：第 10 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`func (p Prepare) Bytes() ([]byte, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 由 <code>json.Marshal(p)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>json.Marshal(p)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>bad</code> |

**跟着本段代码走一遍：**

- 检查 <code>p.Version != 1 &#124;&#124; !OperationValid(p.Operation) &#124;&#124; !ownerName.MatchString(p.Owner) &#124;&#124; !physicalName.MatchString(p.Alias) &#124;&#124; !physicalName.MatchString(p.Source) &#124;&#124; !physicalName.Matc …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, bad</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; source == math.MaxUint64</code>。条件成立时执行 <code>return nil, bad</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; revision == math.MaxUint64</code>。条件成立时执行 <code>return nil, bad</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; target != source+1</code>。条件成立时执行 <code>return nil, bad</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n &gt; math.MaxInt64</code>。条件成立时执行 <code>return nil, bad</code>。当前函数在这里结束。
- 此处通过 <code>return json.Marshal(p)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=2054:3194 -->
````go
func (p Prepare) Bytes() ([]byte, error) {
	bad := errors.New("REBUILD_PREPARE_INVALID")
	if p.Version != 1 || !OperationValid(p.Operation) || !ownerName.MatchString(p.Owner) || !physicalName.MatchString(p.Alias) || !physicalName.MatchString(p.Source) || !physicalName.MatchString(p.Target) ||
		p.Alias == p.Source || p.Alias == p.Target || p.Source == p.Target || !topicName.MatchString(p.Topic) || !Hex(p.TopicID, 16) || p.TopicID == strings.Repeat("0", 32) || !Hex(p.Nonce, 32) ||
		len(p.Cluster) == 0 || len(p.Cluster) > 256 || !utf8.ValidString(p.Cluster) || strings.ContainsAny(p.Cluster, "\x00\r\n\t") {
		return nil, bad
	}
	source, err := Decimal(p.SourceGeneration, true)
	if err != nil || source == math.MaxUint64 {
		return nil, bad
	}
	revision, err := Decimal(p.SourceRevision, true)
	if err != nil || revision == math.MaxUint64 {
		return nil, bad
	}
	target, err := Decimal(p.TargetGeneration, true)
	if err != nil || target != source+1 {
		return nil, bad
	}
	for _, offset := range p.Starts {
		n, err := Decimal(offset, false)
		if err != nil || n > math.MaxInt64 {
			return nil, bad
		}
	}
	return json.Marshal(p)
}

````

## 第 12 步：ParsePrepare

插入位置：第 11 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`func ParsePrepare(raw []byte) (Prepare, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>json.Unmarshal(raw, &amp;p)</code>；<code>bytes.Equal(canonical, raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Prepare</code> | 使用 <code>Prepare</code> 类型；其定义处的职责：Prepare的整数采用字符串。原L与源/目标代次一起签名，不能靠两台机器的时间戳证明先后。<br>实际返回表达式：<code>p</code>；<code>Prepare{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;REBUILD_PREPARE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) &gt; 16384 &#124;&#124; json.Unmarshal(raw, &amp;p) != nil</code>。条件成立时执行 <code>return p, errors.New(&quot;REBUILD_PREPARE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(canonical, raw)</code>。条件成立时执行 <code>return Prepare{}, errors.New(&quot;REBUILD_PREPARE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return p, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3194:3525 -->
````go
func ParsePrepare(raw []byte) (Prepare, error) {
	var p Prepare
	if len(raw) > 16384 || json.Unmarshal(raw, &p) != nil {
		return p, errors.New("REBUILD_PREPARE_INVALID")
	}
	canonical, err := p.Bytes()
	if err != nil || !bytes.Equal(canonical, raw) {
		return Prepare{}, errors.New("REBUILD_PREPARE_INVALID")
	}
	return p, nil
}

````

## 第 13 步：Seal.Bytes

插入位置：第 12 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`func (s Seal) Bytes() ([]byte, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 由 <code>json.Marshal(s)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>json.Marshal(s)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;REBUILD_SEAL_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.Version != 1 &#124;&#124; !strings.HasPrefix(s.PrepareRef, &quot;rebuild-prepare-v1:&quot;) &#124;&#124; !Hex(strings.TrimPrefix(s.PrepareRef, &quot;rebuild-prepare-v1:&quot;), 32) &#124;&#124; !Hex(s.EntriesDigest, 32)</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_SEAL_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_SEAL_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return json.Marshal(s)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=3525:3926 -->
````go
func (s Seal) Bytes() ([]byte, error) {
	if s.Version != 1 || !strings.HasPrefix(s.PrepareRef, "rebuild-prepare-v1:") || !Hex(strings.TrimPrefix(s.PrepareRef, "rebuild-prepare-v1:"), 32) || !Hex(s.EntriesDigest, 32) {
		return nil, errors.New("REBUILD_SEAL_INVALID")
	}
	if _, err := Decimal(s.Count, false); err != nil {
		return nil, errors.New("REBUILD_SEAL_INVALID")
	}
	return json.Marshal(s)
}

````

## 第 14 步：ParseSeal

插入位置：第 13 步代码之后，紧接追加；完成稿约第 111 行。

本步声明：`func ParseSeal(raw []byte) (Seal, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>json.Unmarshal(raw, &amp;s)</code>；<code>bytes.Equal(canonical, raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Seal</code> | 使用 <code>Seal</code> 类型；其定义处的职责：Seal通过完整原prepare内容地址绑定owner、operation、代次和L，避免复制两份易漂移事实。<br>实际返回表达式：<code>s</code>；<code>Seal{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;REBUILD_SEAL_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) &gt; 4096 &#124;&#124; json.Unmarshal(raw, &amp;s) != nil</code>。条件成立时执行 <code>return s, errors.New(&quot;REBUILD_SEAL_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(canonical, raw)</code>。条件成立时执行 <code>return Seal{}, errors.New(&quot;REBUILD_SEAL_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=3926:4237 -->
````go
func ParseSeal(raw []byte) (Seal, error) {
	var s Seal
	if len(raw) > 4096 || json.Unmarshal(raw, &s) != nil {
		return s, errors.New("REBUILD_SEAL_INVALID")
	}
	canonical, err := s.Bytes()
	if err != nil || !bytes.Equal(canonical, raw) {
		return Seal{}, errors.New("REBUILD_SEAL_INVALID")
	}
	return s, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/rebuildmeta/binding.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/05-rebuild-artifacts.md)。
