# 分步编写 `packages/platform/rebuildmeta/descriptor.go`

先按 [F05 · 重建元数据、签名与原子制品](../lessons/foundation/05-rebuild-artifacts.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/026-descriptor.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将重建制品引用与Kafka物理日志起点编码进冻结RPC watermark，并计算完整请求指纹。

同名topic或只凭operation号不能证明原任务；watermark需携带cluster/topic UUID和3个原offset。

写之前先按这条执行路线理解各段的连接：

1. DescriptorFor从Prepare复制日志身份与starts并绑定prepare/snapshot引用。
2. ParseDescriptor严格重编码验证；RequestFingerprint对用途标签及长度前缀字段作SHA256，再写入generation。

本文件需要守住的边界：

- 不能简单字符串拼接造成边界歧义；snapshotRef必须等于descriptor中的引用。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform/rebuildmeta' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/rebuildmeta/descriptor.go') -and (Get-Item -LiteralPath 'packages/platform/rebuildmeta/descriptor.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/rebuildmeta/descriptor.go')) { New-Item -ItemType File -Path 'packages/platform/rebuildmeta/descriptor.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:140 -->
````go
package rebuildmeta

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

````

## 第 2 步：Descriptor

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

Descriptor是冻结RPC内watermark字段的版本化内容，不携带私钥、证书或整库元数据。
starts依次对应分区0、1、2，使用字符串避免JSON数字转换损失原offset。

本步声明：`type Descriptor struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>int</code> | RPC watermark版本，固定1。<br>序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>PrepareRef</code> | <code>string</code> | 已签名原prepare制品内容地址。<br>序列化/校验标记：<code>`json:&quot;prepare_ref&quot;`</code> |
| <code>SnapshotRef</code> | <code>string</code> | 已签名Doc封存snapshot制品内容地址。<br>序列化/校验标记：<code>`json:&quot;snapshot_ref&quot;`</code> |
| <code>Cluster</code> | <code>string</code> | 原Kafka物理cluster ID。<br>序列化/校验标记：<code>`json:&quot;cluster_id&quot;`</code> |
| <code>Topic</code> | <code>string</code> | 原事件主题名。<br>序列化/校验标记：<code>`json:&quot;topic&quot;`</code> |
| <code>TopicID</code> | <code>string</code> | 原主题16字节UUID hex，拒绝同名换日志。<br>序列化/校验标记：<code>`json:&quot;topic_id&quot;`</code> |
| <code>Starts</code> | <code>&#91;3&#93;string</code> | 原分区0/1/2起点L，必须与Prepare保持相同。<br>序列化/校验标记：<code>`json:&quot;starts&quot;`</code> |

<!-- write-step: 2 bytes=140:663 -->
````go
// Descriptor是冻结RPC内watermark字段的版本化内容，不携带私钥、证书或整库元数据。
// starts依次对应分区0、1、2，使用字符串避免JSON数字转换损失原offset。
type Descriptor struct {
	Version     int       `json:"version"`
	PrepareRef  string    `json:"prepare_ref"`
	SnapshotRef string    `json:"snapshot_ref"`
	Cluster     string    `json:"cluster_id"`
	Topic       string    `json:"topic"`
	TopicID     string    `json:"topic_id"`
	Starts      [3]string `json:"starts"`
}

````

## 第 3 步：ErrDescriptor

插入位置：第 2 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`var ErrDescriptor = errors.New("SEARCH_REBUILD_DESCRIPTOR_INVALID")`。

<!-- write-step: 3 bytes=663:732 -->
````go
var ErrDescriptor = errors.New("SEARCH_REBUILD_DESCRIPTOR_INVALID")

````

## 第 4 步：referenceValid

插入位置：第 3 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func referenceValid(value, kind string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>strings.HasPrefix(value, prefix)</code>；<code>strings.TrimPrefix(value, prefix)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;rebuild-&quot; + kind</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>strings.HasPrefix(value, prefix) &amp;&amp; Hex(strings.TrimPrefix(value, prefix), 32)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return strings.HasPrefix(value, prefix) &amp;&amp; Hex(strings.TrimPrefix(value, prefix), 32)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=732:907 -->
````go
func referenceValid(value, kind string) bool {
	prefix := "rebuild-" + kind + "-v1:"
	return strings.HasPrefix(value, prefix) && Hex(strings.TrimPrefix(value, prefix), 32)
}

````

## 第 5 步：Descriptor.Bytes

插入位置：第 4 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`func (d Descriptor) Bytes() ([]byte, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 由 <code>json.Marshal(d)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>json.Marshal(d)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>ErrDescriptor</code> |

**跟着本段代码走一遍：**

- 检查 <code>d.Version != 1 &#124;&#124; !referenceValid(d.PrepareRef, &quot;prepare&quot;) &#124;&#124; !referenceValid(d.SnapshotRef, &quot;snapshot&quot;) &#124;&#124; 		len(d.Cluster) == 0 &#124;&#124; len(d.Cluster) &gt; 256 &#124;&#124; !utf8.ValidString(d.Clu …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, ErrDescriptor</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; value &gt; math.MaxInt64</code>。条件成立时执行 <code>return nil, ErrDescriptor</code>。当前函数在这里结束。
- 此处通过 <code>return json.Marshal(d)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=907:1504 -->
````go
func (d Descriptor) Bytes() ([]byte, error) {
	if d.Version != 1 || !referenceValid(d.PrepareRef, "prepare") || !referenceValid(d.SnapshotRef, "snapshot") ||
		len(d.Cluster) == 0 || len(d.Cluster) > 256 || !utf8.ValidString(d.Cluster) || strings.ContainsAny(d.Cluster, "\x00\r\n\t") ||
		!topicName.MatchString(d.Topic) || !Hex(d.TopicID, 16) || d.TopicID == strings.Repeat("0", 32) {
		return nil, ErrDescriptor
	}
	for _, offset := range d.Starts {
		value, err := Decimal(offset, false)
		if err != nil || value > math.MaxInt64 {
			return nil, ErrDescriptor
		}
	}
	return json.Marshal(d)
}

````

## 第 6 步：ParseDescriptor

插入位置：第 5 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func ParseDescriptor(raw []byte) (Descriptor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>json.Unmarshal(raw, &amp;d)</code>；<code>bytes.Equal(canonical, raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Descriptor</code> | 使用 <code>Descriptor</code> 类型；其定义处的职责：Descriptor是冻结RPC内watermark字段的版本化内容，不携带私钥、证书或整库元数据。 starts依次对应分区0、1、2，使用字符串避免JSON数字转换损失原offset。<br>实际返回表达式：<code>d</code>；<code>Descriptor{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrDescriptor</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) &gt; 4096 &#124;&#124; json.Unmarshal(raw, &amp;d) != nil</code>。条件成立时执行 <code>return d, ErrDescriptor</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(canonical, raw)</code>。条件成立时执行 <code>return Descriptor{}, ErrDescriptor</code>。当前函数在这里结束。
- 此处通过 <code>return d, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1504:1798 -->
````go
func ParseDescriptor(raw []byte) (Descriptor, error) {
	var d Descriptor
	if len(raw) > 4096 || json.Unmarshal(raw, &d) != nil {
		return d, ErrDescriptor
	}
	canonical, err := d.Bytes()
	if err != nil || !bytes.Equal(canonical, raw) {
		return Descriptor{}, ErrDescriptor
	}
	return d, nil
}

````

## 第 7 步：DescriptorFor

插入位置：第 6 步代码之后，紧接追加；完成稿约第 60 行。

本步声明：`func DescriptorFor(p Prepare, prepareRef, snapshotRef string) (Descriptor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>p</code> | <code>Prepare</code> | 使用 <code>Prepare</code> 类型；其定义处的职责：Prepare的整数采用字符串。原L与源/目标代次一起签名，不能靠两台机器的时间戳证明先后。<br>在此声明中的实际使用：<code>p.Bytes</code>；<code>p.Cluster</code>；<code>p.Topic</code>；<code>p.TopicID</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>prepareRef</code> | <code>string</code> | 在此声明中的实际使用：<code>PrepareRef: prepareRef</code> |
| <code>snapshotRef</code> | <code>string</code> | 在此声明中的实际使用：<code>SnapshotRef: snapshotRef</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Descriptor</code> | 使用 <code>Descriptor</code> 类型；其定义处的职责：Descriptor是冻结RPC内watermark字段的版本化内容，不携带私钥、证书或整库元数据。 starts依次对应分区0、1、2，使用字符串避免JSON数字转换损失原offset。<br>实际返回表达式：<code>Descriptor{}</code>；<code>d</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return Descriptor{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return d, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1798:2144 -->
````go
func DescriptorFor(p Prepare, prepareRef, snapshotRef string) (Descriptor, error) {
	if _, err := p.Bytes(); err != nil {
		return Descriptor{}, err
	}
	d := Descriptor{Version: 1, PrepareRef: prepareRef, SnapshotRef: snapshotRef,
		Cluster: p.Cluster, Topic: p.Topic, TopicID: p.TopicID, Starts: p.Starts}
	_, err := d.Bytes()
	return d, err
}

````

## 第 8 步：RequestFingerprint

插入位置：第 7 步代码之后，紧接追加；完成稿约第 70 行。

RequestFingerprint由Doc和Search共用。长度前缀与用途标签消除字段拼接歧义；
Get只接受原完整请求摘要，不能仅凭可猜的operation号查询另一个重建任务。

本步声明：`func RequestFingerprint(operation string, generation uint64, snapshotRef string, watermark []byte) ([32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>OperationValid(operation)</code>；<code>&#91;&#93;byte(operation)</code> |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>generation == 0</code>；<code>binary.BigEndian.PutUint64(number&#91;:&#93;, generation)</code> |
| <code>snapshotRef</code> | <code>string</code> | 在此声明中的实际使用：<code>d.SnapshotRef != snapshotRef</code>；<code>&#91;&#93;byte(snapshotRef)</code> |
| <code>watermark</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>ParseDescriptor(watermark)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 实际返回表达式：<code>empty</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrDescriptor</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; !OperationValid(operation) &#124;&#124; generation == 0 &#124;&#124; d.SnapshotRef != snapshotRef</code>。条件成立时执行 <code>return empty, ErrDescriptor</code>。当前函数在这里结束。
- 此处通过 <code>return empty, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2144:3065 -->
````go
// RequestFingerprint由Doc和Search共用。长度前缀与用途标签消除字段拼接歧义；
// Get只接受原完整请求摘要，不能仅凭可猜的operation号查询另一个重建任务。
func RequestFingerprint(operation string, generation uint64, snapshotRef string, watermark []byte) ([32]byte, error) {
	var empty [32]byte
	d, err := ParseDescriptor(watermark)
	if err != nil || !OperationValid(operation) || generation == 0 || d.SnapshotRef != snapshotRef {
		return empty, ErrDescriptor
	}
	h := sha256.New()
	h.Write([]byte("search-rebuild-request-v1\x00"))
	for _, value := range [][]byte{[]byte(operation), []byte(snapshotRef), watermark} {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		h.Write(size[:])
		h.Write(value)
	}
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], generation)
	h.Write(number[:])
	copy(empty[:], h.Sum(nil))
	return empty, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/rebuildmeta/descriptor.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/05-rebuild-artifacts.md)。
