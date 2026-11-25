# 分步编写 `services/search/internal/domain/event/events_models.go`

先按 [S01 · Canal事件与Doc当前正式投影](../lessons/search/01-event-authority.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/370-events-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义不可变outbox事件、Kafka位置/记录/保留窗口，并提供拒绝重复键的JSON解码。

事件身份来自Doc原行，投递批次和当前Doc快照都不能改写它；Kafka offset与业务uint64序号也不能混用。

写之前先按这条执行路线理解各段的连接：

1. decodeStrict先递归检查JSON重复键、最大16层和完整结束，再DisallowUnknownFields解码
2. Event保存规范原行及RawDigest
3. Position描述传输topic/partition/offset，Record携带Value
4. PartitionCount固定3属于日志身份协议

本文件需要守住的边界：

- 重复JSON键不能采用后值覆盖
- 扩分区需要协议与checkpoint迁移，不是改常量扩容

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/domain/event' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/domain/event/events_models.go') -and (Get-Item -LiteralPath 'services/search/internal/domain/event/events_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/domain/event/events_models.go')) { New-Item -ItemType File -Path 'services/search/internal/domain/event/events_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:174 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package event

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"
)

````

## 第 2 步：PartitionCount

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

PartitionCount 属于当前日志身份和重建前沿 v1 协议，不能当作可随意修改的容量参数。
扩分区必须先设计前沿版本/检查点迁移，不能只调整 broker 分区数或这个常量。

本步声明：`const PartitionCount = 3`。

<!-- write-step: 2 bytes=174:483 -->
````go
// PartitionCount 属于当前日志身份和重建前沿 v1 协议，不能当作可随意修改的容量参数。
// 扩分区必须先设计前沿版本/检查点迁移，不能只调整 broker 分区数或这个常量。
const PartitionCount = 3

// event.go：保留原实现，按业务合并文件。

````

## 第 3 步：Event

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

Event只保存outbox公开引用及原内容摘要。消息批次/投递时间不属于业务身份，
Doc后来返回的当前权威摘要必须另存，不能覆盖RawDigest来伪造原事件一致性。

本步声明：`type Event struct {`。

**这一段为什么这样写：** 保存能够重新核验的原业务事件，而不是ES当前投影。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | outbox event_id，消费者幂等键 |
| <code>Type</code> | <code>string</code> | 发布/撤下/可见性改变的事件类型 |
| <code>Document</code> | <code>uint64</code> | 受影响文档ID |
| <code>Sequence</code> | <code>uint64</code> | 原事件文档聚合序号，Doc当前快照至少达到它 |
| <code>Release</code> | <code>uint64</code> | 原事件发布引用，撤下事件为0 |
| <code>PublicationEpoch</code> | <code>uint64</code> | 原事件公开发布代次 |
| <code>OccurredAt</code> | <code>time.Time</code> | payload中的UTC业务发生时间 |
| <code>RawDigest</code> | <code>&#91;32&#93;byte</code> | 规范原outbox行SHA-256，不是当前正文摘要 |
| <code>Original</code> | <code>&#91;&#93;byte</code> | 可由RestoreEvent重新解析的规范原行字节 |

<!-- write-step: 3 bytes=483:1005 -->
````go
// Event只保存outbox公开引用及原内容摘要。消息批次/投递时间不属于业务身份，
// Doc后来返回的当前权威摘要必须另存，不能覆盖RawDigest来伪造原事件一致性。
type Event struct {
	ID               string
	Type             string
	Document         uint64
	Sequence         uint64
	Release          uint64
	PublicationEpoch uint64
	OccurredAt       time.Time
	RawDigest        [32]byte
	Original         []byte
}

// json.go：保留原实现，按业务合并文件。

````

## 第 4 步：decodeStrict

插入位置：第 3 步代码之后，紧接追加；完成稿约第 34 行。

encoding/json默认接受重复键、忽略未知字段。事件身份不能依赖“后一个值覆盖前一个”，
所以先检查完整JSON及重复键，再按冻结字段解码；所有错误只返回固定码。

本步声明：`func decodeStrict(raw []byte, target any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code> |
| <code>target</code> | <code>any</code> | 在此声明中的实际使用：<code>decoder.Decode(target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;CANAL_JSON_INVALID&quot;)</code>；<code>errors.New(&quot;CANAL_FIELDS_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;CANAL_JSON_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != io.EOF</code>。条件成立时执行 <code>return errors.New(&quot;CANAL_JSON_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;CANAL_FIELDS_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1005:1750 -->
````go
// encoding/json默认接受重复键、忽略未知字段。事件身份不能依赖“后一个值覆盖前一个”，
// 所以先检查完整JSON及重复键，再按冻结字段解码；所有错误只返回固定码。
func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := checkValue(decoder, 0); err != nil {
		return errors.New("CANAL_JSON_INVALID")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("CANAL_JSON_INVALID")
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return errors.New("CANAL_FIELDS_INVALID")
	}
	return nil
}

````

## 第 5 步：checkValue

插入位置：第 4 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`func checkValue(decoder *json.Decoder, depth int) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>decoder</code> | <code>*json.Decoder</code> | 在此声明中的实际使用：<code>decoder.Token</code>；<code>decoder.More</code>；<code>checkValue(decoder, depth+1)</code> |
| <code>depth</code> | <code>int</code> | 在此声明中的实际使用：<code>depth &gt; 16</code>；<code>depth+1</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;CANAL_JSON_DEPTH_EXCEEDED&quot;)</code>；<code>err</code>；<code>nil</code>；<code>errors.New(&quot;CANAL_JSON_DUPLICATE_KEY&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>depth &gt; 16</code>。条件成立时执行 <code>return errors.New(&quot;CANAL_JSON_DEPTH_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!compound</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !valid &#124;&#124; seen&#91;name&#93;</code>。条件成立时执行 <code>return errors.New(&quot;CANAL_JSON_DUPLICATE_KEY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;CANAL_JSON_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1750:2630 -->
````go
func checkValue(decoder *json.Decoder, depth int) error {
	if depth > 16 {
		return errors.New("CANAL_JSON_DEPTH_EXCEEDED")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			name, valid := key.(string)
			if err != nil || !valid || seen[name] {
				return errors.New("CANAL_JSON_DUPLICATE_KEY")
			}
			seen[name] = true
			if err = checkValue(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err = checkValue(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("CANAL_JSON_INVALID")
	}
	_, err = decoder.Token()
	return err
}

// record.go：保留原实现，按业务合并文件。

````

## 第 6 步：Position

插入位置：第 5 步代码之后，紧接追加；完成稿约第 95 行。

Kafka位置属于有符号协议offset，业务文档/发布/序号仍使用Event中的全uint64。
传输记录模型隔离第三方Kafka客户端，业务用例不持有其Reader或Message。

本步声明：`type Position struct {`。

**这一段为什么这样写：** 一条Kafka记录的物理位置。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Topic</code> | <code>string</code> | 冻结消费主题名称 |
| <code>Partition</code> | <code>int</code> | 固定0/1/2分区编号 |
| <code>Offset</code> | <code>int64</code> | 有符号非负记录offset，不是下一位置 |

<!-- write-step: 6 bytes=2630:2906 -->
````go
// Kafka位置属于有符号协议offset，业务文档/发布/序号仍使用Event中的全uint64。
// 传输记录模型隔离第三方Kafka客户端，业务用例不持有其Reader或Message。
type Position struct {
	Topic     string
	Partition int
	Offset    int64
}

````

## 第 7 步：Record

插入位置：第 6 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`type Record struct {`。

**这一段为什么这样写：** 把Kafka库对象转换为业务能校验的记录。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Position</code> | <code>Position</code> | 完整主题/分区/offset |
| <code>Value</code> | <code>&#91;&#93;byte</code> | 未经更改的Canal消息字节 |

<!-- write-step: 7 bytes=2906:2954 -->
````go
type Record struct {
	Position
	Value []byte
}

````

## 第 8 步：PartitionWindow

插入位置：第 7 步代码之后，紧接追加；完成稿约第 108 行。

本步声明：`type PartitionWindow struct {`。

**这一段为什么这样写：** 比较保留窗口、SQL进度和broker组位置。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>First</code> | <code>int64</code> | 仍可读取的最早offset，包含端点 |
| <code>Last</code> | <code>int64</code> | 日志末尾下一offset，排他上界 |
| <code>Committed</code> | <code>int64</code> | broker组已提交的下一offset；-1表示尚无提交 |

<!-- write-step: 8 bytes=2954:3037 -->
````go
type PartitionWindow struct {
	First     int64
	Last      int64
	Committed int64
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/domain/event/events_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/01-event-authority.md)。
