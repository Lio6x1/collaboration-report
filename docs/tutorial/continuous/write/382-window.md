# 分步编写 `services/search/internal/client/logsource/window.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/382-window.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

以Kafka ClusterID+TopicID捕获重建日志身份和固定分区窗口。

同名主题被删除重建后offset可重复，名称不足以证明仍是原日志。

写之前先按这条执行路线理解各段的连接：

1. New锁定Metadata v10、ListOffsets v5、Fetch v13，禁止无TopicID协议降级
2. Capture先取identity，再取first/last，再取identity比较
3. expected非空时必须匹配原准备身份
4. checkedIdentity核集群/主题UUID及固定分区0/1/2

本文件需要守住的边界：

- leader变化仅影响路由，不是持久身份变更
- 多个broker请求不是原子快照，因此明确前后身份校验，不宣称原子采样

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/client/logsource' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/client/logsource/window.go') -and (Get-Item -LiteralPath 'services/search/internal/client/logsource/window.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/client/logsource/window.go')) { New-Item -ItemType File -Path 'services/search/internal/client/logsource/window.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:381 -->
````go
// Package logsource提供重建专用的只读日志入口，不加入普通消费组，也不提交其offset。
package logsource

import (
	events "collaboration/search/internal/domain/event"
	"context"
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/twmb/franz-go/pkg/kversion"
)

````

## 第 2 步：Identity

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

Identity在准备阶段封存。名字相同但TopicID不同的日志不能续接原任务。
leader不属于持久身份，正常选举允许刷新路由；分区集合固定为0、1、2。

本步声明：`type Identity struct {`。

**这一段为什么这样写：** 不可替换的物理日志身份。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Cluster</code> | <code>string</code> | Kafka集群ID |
| <code>Topic</code> | <code>string</code> | 主题名称 |
| <code>TopicID</code> | <code>&#91;16&#93;byte</code> | Kafka主题16字节UUID，同名重建必须不同 |

<!-- write-step: 2 bytes=381:654 -->
````go
// Identity在准备阶段封存。名字相同但TopicID不同的日志不能续接原任务。
// leader不属于持久身份，正常选举允许刷新路由；分区集合固定为0、1、2。
type Identity struct {
	Cluster string
	Topic   string
	TopicID [16]byte
}

````

## 第 3 步：Bounds

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`type Bounds struct{ First, Last int64 }`。

**这一段为什么这样写：** 一个分区的保留区间。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>First</code> | <code>int64</code> | 当前最早保留offset，包含 |
| <code>Last</code> | <code>int64</code> | 当前末尾下一offset，排他 |

<!-- write-step: 3 bytes=654:694 -->
````go
type Bounds struct{ First, Last int64 }
````

## 第 4 步：Window

插入位置：第 3 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`type Window struct {`。

**这一段为什么这样写：** 同一次身份核验期间观察的完整三分区窗口。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Identity</code> | <code>Identity</code> | 已经前后比对的日志身份 |
| <code>Partitions</code> | <code>&#91;events.PartitionCount&#93;Bounds</code> | 固定三个分区的First/Last边界 |

<!-- write-step: 4 bytes=694:781 -->
````go
type Window struct {
	Identity   Identity
	Partitions [events.PartitionCount]Bounds
}

````

## 第 5 步：Client

插入位置：第 4 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`type Client struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>api</code> | <code>*kgo.Client</code> | 复用的franz-go Kafka客户端，负责请求broker的分区窗口。 |
| <code>topic</code> | <code>string</code> | 固定索引事件主题；查询窗口必须与消费/重建的主题一致。 |

<!-- write-step: 5 bytes=781:838 -->
````go
type Client struct {
	api   *kgo.Client
	topic string
}

````

## 第 6 步：topicName

插入位置：第 5 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`var topicName = regexp.MustCompile(\`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$\`)`。

<!-- write-step: 6 bytes=838:912 -->
````go
var topicName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$`)

````

## 第 7 步：New

插入位置：第 6 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`func New(brokers []string, topic string) (*Client, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>brokers</code> | <code>&#91;&#93;string</code> | 在此声明中的实际使用：<code>len(brokers)</code>；<code>kgo.SeedBrokers(brokers...)</code> |
| <code>topic</code> | <code>string</code> | 在此声明中的实际使用：<code>topicName.MatchString(topic)</code>；<code>topic: topic</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Client</code> | 实际返回表达式：<code>nil</code>；<code>&amp;Client{api: api, topic: topic}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_LOG_CONFIG_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(brokers) == 0 &#124;&#124; len(brokers) &gt; 8 &#124;&#124; !topicName.MatchString(topic)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_LOG_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; parseErr != nil &#124;&#124; value == 0 &#124;&#124; host == &quot;&quot; &#124;&#124; strings.ContainsAny(host, &quot; /\\@?#\t\r\n&quot;) &#124;&#124; strconv.FormatUint(value, 10) != port</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_LOG_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_LOG_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Client{api: api, topic: topic}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=912:2183 -->
````go
func New(brokers []string, topic string) (*Client, error) {
	if len(brokers) == 0 || len(brokers) > 8 || !topicName.MatchString(topic) {
		return nil, errors.New("SEARCH_LOG_CONFIG_INVALID")
	}
	for _, address := range brokers {
		host, port, err := net.SplitHostPort(address)
		value, parseErr := strconv.ParseUint(port, 10, 16)
		if err != nil || parseErr != nil || value == 0 || host == "" || strings.ContainsAny(host, " /\\@?#\t\r\n") || strconv.FormatUint(value, 10) != port {
			return nil, errors.New("SEARCH_LOG_CONFIG_INVALID")
		}
	}
	maximum := kversion.V3_7_0()
	minimum := new(kversion.Versions)
	// Metadata v10开始返回TopicID；Fetch v13才按TopicID读取。禁止协议降级。
	for key, version := range map[int16]int16{3: 10, 2: 5, 1: 13} {
		minimum.SetMaxKeyVersion(key, version)
		maximum.SetMaxKeyVersion(key, version)
	}
	api, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ClientID("collab-rebuild"),
		kgo.MinVersions(minimum), kgo.MaxVersions(maximum), kgo.DialTimeout(2*time.Second),
		kgo.RequestTimeoutOverhead(3*time.Second), kgo.BrokerMaxReadBytes(8<<20), kgo.FetchMaxBytes(4<<20), kgo.RequestRetries(0))
	if err != nil {
		return nil, errors.New("SEARCH_LOG_CONFIG_INVALID")
	}
	return &Client{api: api, topic: topic}, nil
}

````

## 第 8 步：Client.Close

插入位置：第 7 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`func (c *Client) Close()`。

<!-- write-step: 8 bytes=2183:2227 -->
````go
func (c *Client) Close() { c.api.Close() }

````

## 第 9 步：Client.Capture

插入位置：第 8 步代码之后，紧接追加；完成稿约第 69 行。

Capture先后核验身份，再读取窗口。它证明观察期间身份一致，不声称几个broker请求原子执行。
expected在恢复时必须传原准备身份；调用方不得把新捕获的身份覆盖原任务。

本步声明：`func (c *Client) Capture(ctx context.Context, expected *Identity) (Window, error)`。

**这一段为什么这样写：** 保持重建原日志身份，同时观察可用窗口。

1. 身份前读
2. 读取各分区First和Last
3. 身份后读并比较
4. 返回Window


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 父生命周期<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 3*time.Second)</code>；<code>context.WithTimeout(ctx, 3*time.Second)</code>；<code>c.identity(ctx)</code>；<code>c.offsets(ctx, -2)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>expected</code> | <code>*Identity</code> | 原准备身份；首次捕获可nil，恢复必须传原值<br>在此声明中的实际使用：<code>expected != nil</code>；<code>before != *expected</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Window</code> | 完整身份与窗口<br>实际返回表达式：<code>Window{}</code>；<code>window</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 身份变化/窗口不可用稳定错误<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code>；<code>errors.New(&quot;SEARCH_LOG_WINDOW_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 3*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return Window{}, err</code>。当前函数在这里结束。
- 检查 <code>expected != nil &amp;&amp; before != *expected</code>。条件成立时执行 <code>return Window{}, errors.New(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return Window{}, err</code>。当前函数在这里结束。
- 检查 <code>before != after</code>。条件成立时执行 <code>return Window{}, errors.New(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>last&#91;partition&#93; &lt; first&#91;partition&#93;</code>。条件成立时执行 <code>return Window{}, errors.New(&quot;SEARCH_LOG_WINDOW_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return window, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2227:3413 -->
````go
// Capture先后核验身份，再读取窗口。它证明观察期间身份一致，不声称几个broker请求原子执行。
// expected在恢复时必须传原准备身份；调用方不得把新捕获的身份覆盖原任务。
func (c *Client) Capture(ctx context.Context, expected *Identity) (Window, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	before, err := c.identity(ctx)
	if err != nil {
		return Window{}, err
	}
	if expected != nil && before != *expected {
		return Window{}, errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	first, err := c.offsets(ctx, -2)
	if err != nil {
		return Window{}, err
	}
	last, err := c.offsets(ctx, -1)
	if err != nil {
		return Window{}, err
	}
	after, err := c.identity(ctx)
	if err != nil {
		return Window{}, err
	}
	if before != after {
		return Window{}, errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	window := Window{Identity: before}
	for partition := range window.Partitions {
		if last[partition] < first[partition] {
			return Window{}, errors.New("SEARCH_LOG_WINDOW_INVALID")
		}
		window.Partitions[partition] = Bounds{First: first[partition], Last: last[partition]}
	}
	return window, nil
}

````

## 第 10 步：Client.identity

插入位置：第 9 步代码之后，紧接追加；完成稿约第 106 行。

本步声明：`func (c *Client) identity(ctx context.Context) (Identity, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>request.RequestWith(ctx, c.api)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：不可替换的物理日志身份。<br>由 <code>checkedIdentity(c.topic, response)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>Identity{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>checkedIdentity(c.topic, response)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_LOG_IDENTITY_UNAVAILABLE&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return Identity{}, errors.New(&quot;SEARCH_LOG_IDENTITY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return checkedIdentity(c.topic, response)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3413:3809 -->
````go
func (c *Client) identity(ctx context.Context) (Identity, error) {
	request := kmsg.NewPtrMetadataRequest()
	request.Topics = []kmsg.MetadataRequestTopic{{Topic: &c.topic}}
	request.AllowAutoTopicCreation = false
	response, err := request.RequestWith(ctx, c.api)
	if err != nil {
		return Identity{}, errors.New("SEARCH_LOG_IDENTITY_UNAVAILABLE")
	}
	return checkedIdentity(c.topic, response)
}

````

## 第 11 步：checkedIdentity

插入位置：第 10 步代码之后，紧接追加；完成稿约第 117 行。

本步声明：`func checkedIdentity(topic string, response *kmsg.MetadataResponse) (Identity, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>topic</code> | <code>string</code> | 在此声明中的实际使用：<code>*value.Topic != topic</code>；<code>Topic: topic</code> |
| <code>response</code> | <code>*kmsg.MetadataResponse</code> | 在此声明中的实际使用：<code>response == nil</code>；<code>response.Version</code>；<code>response.ClusterID</code>；<code>response.Topics</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：不可替换的物理日志身份。<br>实际返回表达式：<code>Identity{}</code>；<code>Identity{Cluster: *response.ClusterID, Topic: topic, TopicID: value.TopicID}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>response == nil &#124;&#124; response.Version &lt; 10 &#124;&#124; response.ClusterID == nil &#124;&#124; *response.ClusterID == &quot;&quot; &#124;&#124; len(response.Topics) != 1</code>。条件成立时执行 <code>return Identity{}, invalid</code>。当前函数在这里结束。
- 检查 <code>value.ErrorCode != 0 &#124;&#124; value.Topic == nil &#124;&#124; *value.Topic != topic &#124;&#124; value.TopicID == &#91;16&#93;byte{} &#124;&#124; len(value.Partitions) != events.PartitionCount</code>。条件成立时执行 <code>return Identity{}, invalid</code>。当前函数在这里结束。
- 检查 <code>partition.Partition &lt; 0 &#124;&#124; partition.Partition &gt;= events.PartitionCount &#124;&#124; seen&#91;partition.Partition&#93; &#124;&#124; partition.ErrorCode != 0 &#124;&#124; partition.Leader &lt; 0</code>。条件成立时执行 <code>return Identity{}, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return Identity{Cluster: *response.ClusterID, Topic: topic, TopicID: value.TopicID}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3809:4742 -->
````go
func checkedIdentity(topic string, response *kmsg.MetadataResponse) (Identity, error) {
	invalid := errors.New("SEARCH_LOG_IDENTITY_INVALID")
	if response == nil || response.Version < 10 || response.ClusterID == nil || *response.ClusterID == "" || len(response.Topics) != 1 {
		return Identity{}, invalid
	}
	value := response.Topics[0]
	if value.ErrorCode != 0 || value.Topic == nil || *value.Topic != topic || value.TopicID == [16]byte{} || len(value.Partitions) != events.PartitionCount {
		return Identity{}, invalid
	}
	var seen [events.PartitionCount]bool
	for _, partition := range value.Partitions {
		if partition.Partition < 0 || partition.Partition >= events.PartitionCount || seen[partition.Partition] || partition.ErrorCode != 0 || partition.Leader < 0 {
			return Identity{}, invalid
		}
		seen[partition.Partition] = true
	}
	return Identity{Cluster: *response.ClusterID, Topic: topic, TopicID: value.TopicID}, nil
}

````

## 第 12 步：Client.offsets

插入位置：第 11 步代码之后，紧接追加；完成稿约第 136 行。

本步声明：`func (c *Client) offsets(ctx context.Context, timestamp int64) ([events.PartitionCount]int64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>request.RequestWith(ctx, c.api)</code> |
| <code>timestamp</code> | <code>int64</code> | 在此声明中的实际使用：<code>partition.Partition, partition.Timestamp = i, timestamp</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;events.PartitionCount&#93;int64</code> | 实际返回表达式：<code>&#91;events.PartitionCount&#93;int64{}</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_LOG_WINDOW_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_LOG_WINDOW_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; response == nil &#124;&#124; len(response.Topics) != 1 &#124;&#124; response.Topics&#91;0&#93;.Topic != c.topic &#124;&#124; len(response.Topics&#91;0&#93;.Partitions) != events.PartitionCount</code>。条件成立时执行 <code>return &#91;events.PartitionCount&#93;int64{}, errors.New(&quot;SEARCH_LOG_WINDOW_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>partition.Partition &lt; 0 &#124;&#124; partition.Partition &gt;= events.PartitionCount &#124;&#124; seen&#91;partition.Partition&#93; &#124;&#124; partition.ErrorCode != 0 &#124;&#124; partition.Offset &lt; 0</code>。条件成立时执行 <code>return &#91;events.PartitionCount&#93;int64{}, errors.New(&quot;SEARCH_LOG_WINDOW_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4742:6039 -->
````go
func (c *Client) offsets(ctx context.Context, timestamp int64) ([events.PartitionCount]int64, error) {
	request := kmsg.NewPtrListOffsetsRequest()
	topic := kmsg.NewListOffsetsRequestTopic()
	topic.Topic = c.topic
	for i := int32(0); i < events.PartitionCount; i++ {
		partition := kmsg.NewListOffsetsRequestTopicPartition()
		partition.Partition, partition.Timestamp = i, timestamp
		topic.Partitions = append(topic.Partitions, partition)
	}
	request.Topics = []kmsg.ListOffsetsRequestTopic{topic}
	response, err := request.RequestWith(ctx, c.api)
	if err != nil || response == nil || len(response.Topics) != 1 || response.Topics[0].Topic != c.topic || len(response.Topics[0].Partitions) != events.PartitionCount {
		return [events.PartitionCount]int64{}, errors.New("SEARCH_LOG_WINDOW_UNAVAILABLE")
	}
	var result [events.PartitionCount]int64
	var seen [events.PartitionCount]bool
	for _, partition := range response.Topics[0].Partitions {
		if partition.Partition < 0 || partition.Partition >= events.PartitionCount || seen[partition.Partition] || partition.ErrorCode != 0 || partition.Offset < 0 {
			return [events.PartitionCount]int64{}, errors.New("SEARCH_LOG_WINDOW_INVALID")
		}
		seen[partition.Partition] = true
		result[partition.Partition] = partition.Offset
	}
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/client/logsource/window.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
