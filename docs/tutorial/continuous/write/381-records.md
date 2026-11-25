# 分步编写 `services/search/internal/client/logsource/records.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/381-records.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

严格解码Kafka RecordBatch，验证CRC、连续offset并限制解压和头数量。

损坏尾部不能让合法前缀先推进持久checkpoint；压缩包必须在解压过程中限内存。

写之前先按这条执行路线理解各段的连接：

1. decodeRecords核magic2、CRC32C、批长度、连续offset与record数量，按[next,until)交付
2. 先boundedRecordHeaders再kmsg解码，阻断短包声明巨量header分配
3. 支持无压缩/gzip/snappy/lz4/zstd，累计最多8MiB
4. 全部批/记录验证完成才返回；任何失败next还原原值

本文件需要守住的边界：

- H/until是排他上界，不能借批尾越过
- 非压实主题要求连续，不可跳缺口
- 先无界解压再检查长度已经来不及

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/client/logsource' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/client/logsource/records.go') -and (Get-Item -LiteralPath 'services/search/internal/client/logsource/records.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/client/logsource/records.go')) { New-Item -ItemType File -Path 'services/search/internal/client/logsource/records.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:301 -->
````go
package logsource

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"math"

	"collaboration/search/internal/domain/event"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
	"github.com/twmb/franz-go/pkg/kmsg"
)

````

## 第 2 步：maxRecordFrameBytes、maxDecodedBytes、maxRecordCount、maxRecordHeaders

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`const (`。

<!-- write-step: 2 bytes=301:456 -->
````go
const (
	maxRecordFrameBytes = event.MaxRecordBytes + (64 << 10)
	maxDecodedBytes     = 8 << 20
	maxRecordCount      = 10000
	maxRecordHeaders    = 128
)

````

## 第 3 步：invalidRecords

插入位置：第 2 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`var invalidRecords = errors.New("SEARCH_LOG_RECORDS_INVALID")`。

<!-- write-step: 3 bytes=456:518 -->
````go
var invalidRecords = errors.New("SEARCH_LOG_RECORDS_INVALID")
````

## 第 4 步：recordCRC

插入位置：第 3 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`var recordCRC = crc32.MakeTable(crc32.Castagnoli)`。

<!-- write-step: 4 bytes=518:569 -->
````go
var recordCRC = crc32.MakeTable(crc32.Castagnoli)

````

## 第 5 步：decodeRecords

插入位置：第 4 步代码之后，紧接追加；完成稿约第 29 行。

decodeRecords只在整个响应验证通过后交付，防止损坏尾部仍使持久checkpoint前移。
非压实主题必须连续；起点允许位于批中，H是排他上界，不能借批尾跨过它。

本步声明：`func decodeRecords(raw []byte, topic string, partition int, next, until int64) ([]event.Record, int64, error)`。

**这一段为什么这样写：** 把原始Fetch字节转换为一个整体可接受的有限记录批。

1. 验证批CRC和连续性
2. 有界解压
3. 验证记录offset及体积
4. 裁剪固定半开区间
5. 完整通过后返回


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | Kafka Fetch返回的RecordBatches字节<br>在此声明中的实际使用：<code>len(raw)</code>；<code>binary.BigEndian.Uint32(raw&#91;8:12&#93;)</code>；<code>encoded := raw&#91;:int(length)&#93;</code>；<code>raw = raw&#91;int(length):&#93;</code> |
| <code>topic</code> | <code>string</code> | 已绑定主题名<br>在此声明中的实际使用：<code>topicName.MatchString(topic)</code>；<code>Topic: topic</code> |
| <code>partition</code> | <code>int</code> | 固定分区编号<br>在此声明中的实际使用：<code>partition &lt; 0</code>；<code>partition &gt; 2</code>；<code>Partition: partition</code> |
| <code>next</code> | <code>int64</code> | 期望首个未重放offset<br>在此声明中的实际使用：<code>original := next</code>；<code>next &lt; 0</code>；<code>until &lt; next</code>；<code>offset != next</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>until</code> | <code>int64</code> | 本轮冻结排他前沿<br>在此声明中的实际使用：<code>until &lt; next</code>；<code>offset &lt; until</code>；<code>offset &gt;= until</code>；<code>next &lt; until</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;event.Record</code> | 全部已验证且在区间内的Record<br>由 <code>fail()</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>int64</code> | 验证成功的下一offset；失败保留输入next<br>由 <code>fail()</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>next</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 记录不完整/损坏/超限错误<br>由 <code>fail()</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, original, invalidRecords</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>next &lt; 0 &#124;&#124; until &lt; next &#124;&#124; partition &lt; 0 &#124;&#124; partition &gt; 2 &#124;&#124; !topicName.MatchString(topic) &#124;&#124; len(raw) &gt; maxDecodedBytes</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>len(raw) &lt; 61</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>length &lt; 61 &#124;&#124; length &gt; int64(len(raw))</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>batch.ReadFrom(encoded) != nil &#124;&#124; batch.Magic != 2 &#124;&#124; uint32(batch.CRC) != crc32.Checksum(encoded&#91;21:&#93;, recordCRC) &#124;&#124; batch.Attributes &amp; ^int16(15) != 0 &#124;&#124; batch.Attributes&amp;7 &gt; 4 &#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>previousEnd &gt;= 0 &amp;&amp; batch.FirstOffset != previousEnd</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>n &lt;= 0 &#124;&#124; n &gt; 5 &#124;&#124; size &lt; 0 &#124;&#124; size &gt; maxRecordFrameBytes &#124;&#124; size &gt; int64(len(decoded)-n)</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>!boundedRecordHeaders(recordBytes&#91;n:&#93;)</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=569:2690 -->
````go
// decodeRecords只在整个响应验证通过后交付，防止损坏尾部仍使持久checkpoint前移。
// 非压实主题必须连续；起点允许位于批中，H是排他上界，不能借批尾跨过它。
func decodeRecords(raw []byte, topic string, partition int, next, until int64) ([]event.Record, int64, error) {
	original := next
	fail := func() ([]event.Record, int64, error) { return nil, original, invalidRecords }
	if next < 0 || until < next || partition < 0 || partition > 2 || !topicName.MatchString(topic) || len(raw) > maxDecodedBytes {
		return fail()
	}
	var result []event.Record
	remaining, count := maxDecodedBytes, 0
	previousEnd := int64(-1)
	for len(raw) > 0 {
		if len(raw) < 61 {
			return fail()
		}
		length := int64(int32(binary.BigEndian.Uint32(raw[8:12]))) + 12
		if length < 61 || length > int64(len(raw)) {
			return fail()
		}
		encoded := raw[:int(length)]
		raw = raw[int(length):]
		var batch kmsg.RecordBatch
		if batch.ReadFrom(encoded) != nil || batch.Magic != 2 || uint32(batch.CRC) != crc32.Checksum(encoded[21:], recordCRC) || batch.Attributes & ^int16(15) != 0 || batch.Attributes&7 > 4 || batch.NumRecords <= 0 || int64(batch.NumRecords) > int64(maxRecordCount-count) || batch.LastOffsetDelta != batch.NumRecords-1 || batch.FirstOffset < 0 || batch.FirstOffset > math.MaxInt64-int64(batch.NumRecords) {
			return fail()
		}
		if previousEnd >= 0 && batch.FirstOffset != previousEnd {
			return fail()
		}
		previousEnd = batch.FirstOffset + int64(batch.NumRecords)
		decoded, err := decompressRecords(batch.Records, batch.Attributes&7, remaining)
		if err != nil {
			return fail()
		}
		remaining -= len(decoded)
		count += int(batch.NumRecords)
		for i := int32(0); i < batch.NumRecords; i++ {
			size, n := binary.Varint(decoded)
			if n <= 0 || n > 5 || size < 0 || size > maxRecordFrameBytes || size > int64(len(decoded)-n) {
				return fail()
			}
			recordBytes := decoded[:n+int(size)]
			// kmsg会按header数量分配数组；先检查长度与数量，阻断恶意短包巨量分配。
			if !boundedRecordHeaders(recordBytes[n:]) {
				return fail()
			}
````

## 第 6 步：decodeRecords（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 74 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>record.ReadFrom(recordBytes) != nil &#124;&#124; record.OffsetDelta != i &#124;&#124; record.Attributes != 0 &#124;&#124; len(record.Value) == 0 &#124;&#124; len(record.Value) &gt; event.MaxRecordBytes</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>offset &gt;= original &amp;&amp; offset &lt; until</code>。
- 检查 <code>offset != next</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>offset &gt;= until &amp;&amp; next &lt; until</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>len(decoded) != 0</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 此处通过 <code>return result, next, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2690:3399 -->
````go
			var record kmsg.Record
			if record.ReadFrom(recordBytes) != nil || record.OffsetDelta != i || record.Attributes != 0 || len(record.Value) == 0 || len(record.Value) > event.MaxRecordBytes {
				return fail()
			}
			offset := batch.FirstOffset + int64(i)
			if offset >= original && offset < until {
				if offset != next {
					return fail()
				}
				result = append(result, event.Record{Position: event.Position{Topic: topic, Partition: partition, Offset: offset}, Value: bytes.Clone(record.Value)})
				next++
			} else if offset >= until && next < until {
				return fail()
			}
			decoded = decoded[len(recordBytes):]
		}
		if len(decoded) != 0 {
			return fail()
		}
	}
	return result, next, nil
}

````

## 第 7 步：boundedRecordHeaders

插入位置：第 6 步代码之后，紧接追加；完成稿约第 97 行。

只预检会触发数组分配的前缀，字段的完整语义仍由固定版本kmsg解码。

本步声明：`func boundedRecordHeaders(src []byte) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>src</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(src)</code>；<code>src = src&#91;1:&#93;</code>；<code>binary.Varint(src)</code>；<code>src = src&#91;n:&#93;</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>ok &amp;&amp; headers &gt;= 0 &amp;&amp; headers &lt;= maxRecordHeaders &amp;&amp; headers &lt;= int64(len(src)/2)</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(src) &lt; 1</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>n &lt;= 0</code>。条件成立时执行 <code>return 0, false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; size &lt; -1 &#124;&#124; size &gt; int64(len(src))</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>size &gt;= 0</code>。
- 此处通过 <code>return ok &amp;&amp; headers &gt;= 0 &amp;&amp; headers &lt;= maxRecordHeaders &amp;&amp; headers &lt;= int64(len(src)/2)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3399:4108 -->
````go
// 只预检会触发数组分配的前缀，字段的完整语义仍由固定版本kmsg解码。
func boundedRecordHeaders(src []byte) bool {
	if len(src) < 1 {
		return false
	}
	src = src[1:]
	take := func() (int64, bool) {
		value, n := binary.Varint(src)
		if n <= 0 {
			return 0, false
		}
		src = src[n:]
		return value, true
	}
	if _, ok := take(); !ok {
		return false
	}
	if _, ok := take(); !ok {
		return false
	}
	for i := 0; i < 2; i++ {
		size, ok := take()
		if !ok || size < -1 || size > int64(len(src)) {
			return false
		}
		if size >= 0 {
			src = src[int(size):]
		}
	}
	headers, ok := take()
	return ok && headers >= 0 && headers <= maxRecordHeaders && headers <= int64(len(src)/2)
}

````

## 第 8 步：decompressRecords

插入位置：第 7 步代码之后，紧接追加；完成稿约第 130 行。

本步声明：`func decompressRecords(src []byte, codec int16, limit int) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>src</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(src)</code>；<code>return src, nil</code>；<code>bytes.NewReader(src)</code>；<code>decodeSnappy(src, limit)</code> |
| <code>codec</code> | <code>int16</code> | Kafka记录批属性中的压缩算法编号；switch选择解压器，未知编号拒绝，所有分支都受解压后长度上限约束。 |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>limit &lt; 0</code>；<code>len(src) &gt; limit</code>；<code>decodeSnappy(src, limit)</code>；<code>int64(limit)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 由 <code>decodeSnappy(src, limit)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>src</code>；<code>decoded</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>decodeSnappy(src, limit)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>invalidRecords</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>limit &lt; 0</code>。条件成立时执行 <code>return nil, invalidRecords</code>。当前函数在这里结束。
- 检查 <code>len(src) &gt; limit</code>。条件成立时执行 <code>return nil, invalidRecords</code>。当前函数在这里结束。
- 此处通过 <code>return src, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, invalidRecords</code>。当前函数在这里结束。
- 登记 <code>value.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return decodeSnappy(src, limit)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, invalidRecords</code>。当前函数在这里结束。
- 登记 <code>value.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; len(decoded) &gt; limit</code>。条件成立时执行 <code>return nil, invalidRecords</code>。当前函数在这里结束。
- 此处通过 <code>return decoded, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4108:5171 -->
````go
func decompressRecords(src []byte, codec int16, limit int) ([]byte, error) {
	if limit < 0 {
		return nil, invalidRecords
	}
	var reader io.Reader
	switch codec {
	case 0:
		if len(src) > limit {
			return nil, invalidRecords
		}
		return src, nil
	case 1:
		value, err := gzip.NewReader(bytes.NewReader(src))
		if err != nil {
			return nil, invalidRecords
		}
		defer value.Close()
		reader = value
	case 2:
		return decodeSnappy(src, limit)
	case 3:
		reader = lz4.NewReader(bytes.NewReader(src))
	case 4:
		value, err := zstd.NewReader(bytes.NewReader(src), zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(maxDecodedBytes), zstd.WithDecoderMaxWindow(maxDecodedBytes))
		if err != nil {
			return nil, invalidRecords
		}
		defer value.Close()
		reader = value
	default:
		return nil, invalidRecords
	}
	// 多读一个字节辨别超限，不能先无界解压再检查结果大小。
	decoded, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil || len(decoded) > limit {
		return nil, invalidRecords
	}
	return decoded, nil
}

````

## 第 9 步：decodeSnappy

插入位置：第 8 步代码之后，紧接追加；完成稿约第 170 行。

本步声明：`func decodeSnappy(src []byte, limit int) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>src</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.HasPrefix(src, prefix)</code>；<code>chunk(src, limit)</code>；<code>len(src)</code>；<code>binary.BigEndian.Uint32(src&#91;8:12&#93;)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>chunk(src, limit)</code>；<code>limit-len(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 由 <code>chunk(src, limit)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>chunk(src, limit)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>invalidRecords</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; size &gt; available</code>。条件成立时执行 <code>return nil, invalidRecords</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, invalidRecords</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return decoded, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!bytes.HasPrefix(src, prefix)</code>。条件成立时执行 <code>return chunk(src, limit)</code>。当前函数在这里结束。
- 检查 <code>len(src) &lt; 16 &#124;&#124; binary.BigEndian.Uint32(src&#91;8:12&#93;) != 1 &#124;&#124; binary.BigEndian.Uint32(src&#91;12:16&#93;) != 1</code>。条件成立时执行 <code>return nil, invalidRecords</code>。当前函数在这里结束。
- 检查 <code>len(src) &lt; 4</code>。条件成立时执行 <code>return nil, invalidRecords</code>。当前函数在这里结束。
- 检查 <code>size &lt;= 0 &#124;&#124; size &gt; int64(len(src))</code>。条件成立时执行 <code>return nil, invalidRecords</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5171:6232 -->
````go
func decodeSnappy(src []byte, limit int) ([]byte, error) {
	prefix := []byte{130, 83, 78, 65, 80, 80, 89, 0}
	chunk := func(encoded []byte, available int) ([]byte, error) {
		size, err := s2.DecodedLen(encoded)
		if err != nil || size > available {
			return nil, invalidRecords
		}
		decoded, err := s2.Decode(make([]byte, 0, size), encoded)
		if err != nil {
			return nil, invalidRecords
		}
		return decoded, nil
	}
	if !bytes.HasPrefix(src, prefix) {
		return chunk(src, limit)
	}
	if len(src) < 16 || binary.BigEndian.Uint32(src[8:12]) != 1 || binary.BigEndian.Uint32(src[12:16]) != 1 {
		return nil, invalidRecords
	}
	src = src[16:]
	var result []byte
	for len(src) > 0 {
		if len(src) < 4 {
			return nil, invalidRecords
		}
		size := int64(binary.BigEndian.Uint32(src))
		src = src[4:]
		if size <= 0 || size > int64(len(src)) {
			return nil, invalidRecords
		}
		decoded, err := chunk(src[:int(size)], limit-len(result))
		if err != nil {
			return nil, err
		}
		result = append(result, decoded...)
		src = src[int(size):]
	}
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/client/logsource/records.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
