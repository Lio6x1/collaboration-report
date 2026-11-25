# 分步编写 `services/search/internal/client/kafka/client.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/378-client.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

封装只允许一条在途记录的Kafka组消费及同步提交。

继续Fetch会越过失败记录，自动或批量提交可能把未落SQL的记录标成已消费。

写之前先按这条执行路线理解各段的连接：

1. New校验brokers/topic/group，reader用QueueCapacity1、CommitInterval0、FirstOffset
2. Fetch拒绝已有pending，核topic/partition/offset/大小，记录sha256身份
3. 非法已取记录置blocked直到重启从未提交位置重读
4. Commit核位置及原字节摘要，5秒内同步CommitMessages，成功才清pending

本文件需要守住的边界：

- 禁止ReadMessage自动提交
- 提交超时保留pending原号重试，不可跳下一条
- offset out of range明确WINDOW_LOST，不能自动跳最新

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/client/kafka' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/client/kafka/client.go') -and (Get-Item -LiteralPath 'services/search/internal/client/kafka/client.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/client/kafka/client.go')) { New-Item -ItemType File -Path 'services/search/internal/client/kafka/client.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:239 -->
````go
package kafka

import (
	"collaboration/platform"
	domain "collaboration/search/internal/domain/event"
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

````

## 第 2 步：recordIdentity

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type recordIdentity struct {`。

**这一段为什么这样写：** 约束Commit必须对应本次Fetch原记录。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>partition</code> | <code>int</code> | 已取分区 |
| <code>offset</code> | <code>int64</code> | 已取记录位置 |
| <code>digest</code> | <code>&#91;32&#93;byte</code> | 原Value SHA-256 |

<!-- write-step: 2 bytes=239:323 -->
````go
type recordIdentity struct {
	partition int
	offset    int64
	digest    [32]byte
}

````

## 第 3 步：Client

插入位置：第 2 步代码之后，紧接追加；完成稿约第 25 行。

Client只保留一条在途记录；失败时必须处理或重读原记录，不能继续Fetch后面的记录。
Kafka提交并不证明SQL成功，业务调用方仍须先完成Record，再调用同步Commit。

本步声明：`type Client struct {`。

**这一段为什么这样写：** 一个消费组客户端的在途与阻断状态。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>configuration</code> | <code>platform.SearchConfig</code> | 冻结的SearchConfig副本 |
| <code>api</code> | <code>*kafka.Client</code> | 读取元数据/窗口/组位置的Kafka客户端 |
| <code>transport</code> | <code>*kafka.Transport</code> | 元数据连接生命周期 |
| <code>mu</code> | <code>sync.Mutex</code> | 串行Fetch/Commit/Close |
| <code>reader</code> | <code>*kafka.Reader</code> | 惰性创建的组Reader |
| <code>pending</code> | <code>*recordIdentity</code> | 唯一未确认记录身份 |
| <code>blocked</code> | <code>error</code> | 已取非法记录后的固定失败，防越过 |
| <code>closed</code> | <code>bool</code> | 是否已关闭 |

<!-- write-step: 3 bytes=323:791 -->
````go
// Client只保留一条在途记录；失败时必须处理或重读原记录，不能继续Fetch后面的记录。
// Kafka提交并不证明SQL成功，业务调用方仍须先完成Record，再调用同步Commit。
type Client struct {
	configuration platform.SearchConfig
	api           *kafka.Client
	transport     *kafka.Transport
	mu            sync.Mutex
	reader        *kafka.Reader
	pending       *recordIdentity
	blocked       error
	closed        bool
}

````

## 第 4 步：groupName

插入位置：第 3 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`var groupName = regexp.MustCompile(\`^[A-Za-z0-9][A-Za-z0-9._:-]{0,39}$\`)`。

<!-- write-step: 4 bytes=791:864 -->
````go
var groupName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,39}$`)
````

## 第 5 步：topicName

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`var topicName = regexp.MustCompile(\`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$\`)`。

<!-- write-step: 5 bytes=864:938 -->
````go
var topicName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$`)

````

## 第 6 步：New

插入位置：第 5 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`func New(configuration *platform.SearchConfig) (*Client, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>configuration</code> | <code>*platform.SearchConfig</code> | 使用 <code>platform.SearchConfig</code> 类型；其定义处的职责：加载服务自身配置，创建有界数据库池，并以迁移和恢复事实判定就绪。<br>在此声明中的实际使用：<code>configuration == nil</code>；<code>configuration.Group</code>；<code>configuration.Topic</code>；<code>configuration.Brokers</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Client</code> | 使用 <code>Client</code> 类型；其定义处的职责：一个消费组客户端的在途与阻断状态。<br>实际返回表达式：<code>nil</code>；<code>&amp;Client{configuration: value, transport: transport, api: &amp;kafka.Client{Addr: kafka.TCP(value.Brokers...), Timeout: 3 * time.Second, Transport: transport}}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_KAFKA_CONFIG_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>configuration == nil &#124;&#124; !groupName.MatchString(configuration.Group) &#124;&#124; !topicName.MatchString(configuration.Topic) &#124;&#124; 		len(configuration.Brokers) == 0 &#124;&#124; len(configuration.Brokers) &gt; 8</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_KAFKA_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; parsed != nil &#124;&#124; host == &quot;&quot; &#124;&#124; strings.ContainsAny(host, &quot; /\\@?#\t\r\n&quot;) &#124;&#124; number == 0 &#124;&#124; strconv.FormatUint(number, 10) != port</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_KAFKA_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Client{configuration: value, transport: transport, api: &amp;kafka.Client{Addr: kafka.TCP(value.Brokers...), Timeout: 3 * time.Second, Transport: transport}}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=938:2035 -->
````go
func New(configuration *platform.SearchConfig) (*Client, error) {
	if configuration == nil || !groupName.MatchString(configuration.Group) || !topicName.MatchString(configuration.Topic) ||
		len(configuration.Brokers) == 0 || len(configuration.Brokers) > 8 {
		return nil, errors.New("SEARCH_KAFKA_CONFIG_INVALID")
	}
	for _, address := range configuration.Brokers {
		host, port, err := net.SplitHostPort(address)
		number, parsed := strconv.ParseUint(port, 10, 16)
		if err != nil || parsed != nil || host == "" || strings.ContainsAny(host, " /\\@?#\t\r\n") || number == 0 || strconv.FormatUint(number, 10) != port {
			return nil, errors.New("SEARCH_KAFKA_CONFIG_INVALID")
		}
	}
	value := *configuration
	value.Brokers = append([]string(nil), configuration.Brokers...)
	transport := &kafka.Transport{DialTimeout: 2 * time.Second, IdleTimeout: 10 * time.Second, MetadataTopics: []string{value.Topic}, ClientID: value.Group}
	return &Client{configuration: value, transport: transport, api: &kafka.Client{Addr: kafka.TCP(value.Brokers...), Timeout: 3 * time.Second, Transport: transport}}, nil
}

````

## 第 7 步：Client.newReader

插入位置：第 6 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func (c *Client) newReader() *kafka.Reader`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*kafka.Reader</code> | 实际返回表达式：<code>kafka.NewReader(kafka.ReaderConfig{ 		Brokers: c.configuration.Brokers, Topic: c.configuration.Topic, GroupID: c.configuration.Group, 		Dialer:        &amp;kafka.Dialer{Timeout: 2 * ti …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return kafka.NewReader(kafka.ReaderConfig{ 		Brokers: c.configuration.Brokers, Topic: c.configuration.Topic, GroupID: c.configuration.Group, 		Dialer:        &amp;kafka.Dialer{Timeout: …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2035:2993 -->
````go
func (c *Client) newReader() *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: c.configuration.Brokers, Topic: c.configuration.Topic, GroupID: c.configuration.Group,
		Dialer:        &kafka.Dialer{Timeout: 2 * time.Second, ClientID: c.configuration.Group},
		QueueCapacity: 1, MinBytes: 1, MaxBytes: domain.MaxRecordBytes + 64<<10, MaxWait: 250 * time.Millisecond, ReadBatchTimeout: time.Second,
		// 不使用ReadMessage，也不设置异步CommitInterval；初次没有组位置必须从最早记录开始。
		CommitInterval: 0, StartOffset: kafka.FirstOffset, ReadLagInterval: -1,
		HeartbeatInterval: 2 * time.Second, SessionTimeout: 10 * time.Second, RebalanceTimeout: 10 * time.Second, JoinGroupBackoff: 500 * time.Millisecond,
		ReadBackoffMin: 100 * time.Millisecond, ReadBackoffMax: 500 * time.Millisecond, MaxAttempts: 3,
		WatchPartitionChanges: true, PartitionWatchInterval: time.Second, OffsetOutOfRangeError: true,
	})
}

````

## 第 8 步：Client.Fetch

插入位置：第 7 步代码之后，紧接追加；完成稿约第 72 行。

本步声明：`func (c *Client) Fetch(ctx context.Context) (domain.Record, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.reader.FetchMessage(ctx)</code>；<code>ctx.Err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Record</code> | 使用 <code>domain.Record</code> 类型；其定义处的职责：把Kafka库对象转换为业务能校验的记录。<br>实际返回表达式：<code>domain.Record{}</code>；<code>domain.Record{Position: domain.Position{Topic: message.Topic, Partition: message.Partition, Offset: message.Offset}, Value: message.Value}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_KAFKA_CLIENT_CLOSED&quot;)</code>；<code>c.blocked</code>；<code>errors.New(&quot;SEARCH_KAFKA_RECORD_PENDING&quot;)</code>；<code>errors.New(&quot;SEARCH_KAFKA_FETCH_CANCELED&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>c.mu.Lock()</code>。
- 登记 <code>c.mu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>c.mu.Unlock()</code>。
- 检查 <code>c.closed</code>。条件成立时执行 <code>return domain.Record{}, errors.New(&quot;SEARCH_KAFKA_CLIENT_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>c.blocked != nil</code>。条件成立时执行 <code>return domain.Record{}, c.blocked</code>。当前函数在这里结束。
- 检查 <code>c.pending != nil</code>。条件成立时执行 <code>return domain.Record{}, errors.New(&quot;SEARCH_KAFKA_RECORD_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>c.reader == nil</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return domain.Record{}, errors.New(&quot;SEARCH_KAFKA_FETCH_CANCELED&quot;)</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, kafka.OffsetOutOfRange)</code>。条件成立时执行 <code>return domain.Record{}, errors.New(&quot;SEARCH_KAFKA_WINDOW_LOST&quot;)</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, kafka.UnknownTopicOrPartition)</code>。条件成立时执行 <code>return domain.Record{}, errors.New(&quot;SEARCH_KAFKA_TOPIC_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return domain.Record{}, errors.New(&quot;SEARCH_KAFKA_FETCH_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>message.Topic != c.configuration.Topic &#124;&#124; message.Partition &lt; 0 &#124;&#124; message.Partition &gt;= domain.PartitionCount &#124;&#124; message.Offset &lt; 0 &#124;&#124; 		len(message.Value) == 0 &#124;&#124; len(message.Value) &gt; domain.MaxRecordBytes</code>。
- 此处通过 <code>return domain.Record{}, c.blocked</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return domain.Record{Position: domain.Position{Topic: message.Topic, Partition: message.Partition, Offset: message.Offset}, Value: message.Value}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2993:4660 -->
````go
func (c *Client) Fetch(ctx context.Context) (domain.Record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return domain.Record{}, errors.New("SEARCH_KAFKA_CLIENT_CLOSED")
	}
	if c.blocked != nil {
		return domain.Record{}, c.blocked
	}
	if c.pending != nil {
		return domain.Record{}, errors.New("SEARCH_KAFKA_RECORD_PENDING")
	}
	if c.reader == nil {
		c.reader = c.newReader()
	}
	message, err := c.reader.FetchMessage(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return domain.Record{}, errors.New("SEARCH_KAFKA_FETCH_CANCELED")
		}
		if errors.Is(err, kafka.OffsetOutOfRange) {
			return domain.Record{}, errors.New("SEARCH_KAFKA_WINDOW_LOST")
		}
		if errors.Is(err, kafka.UnknownTopicOrPartition) {
			return domain.Record{}, errors.New("SEARCH_KAFKA_TOPIC_UNKNOWN")
		}
		return domain.Record{}, errors.New("SEARCH_KAFKA_FETCH_UNAVAILABLE")
	}
	if message.Topic != c.configuration.Topic || message.Partition < 0 || message.Partition >= domain.PartitionCount || message.Offset < 0 ||
		len(message.Value) == 0 || len(message.Value) > domain.MaxRecordBytes {
		// Reader已取走原记录，校验失败后不能再Fetch而越过它。只保存固定阻断状态，
		// 不保留超限正文；Close后由新实例沿broker尚未提交的原位置重读并再次校验。
		c.blocked = errors.New("SEARCH_KAFKA_RECORD_INVALID")
		return domain.Record{}, c.blocked
	}
	c.pending = &recordIdentity{partition: message.Partition, offset: message.Offset, digest: sha256.Sum256(message.Value)}
	return domain.Record{Position: domain.Position{Topic: message.Topic, Partition: message.Partition, Offset: message.Offset}, Value: message.Value}, nil
}

````

## 第 9 步：Client.Commit

插入位置：第 8 步代码之后，紧接追加；完成稿约第 111 行。

本步声明：`func (c *Client) Commit(ctx context.Context, message domain.Record) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 5*time.Second)</code>；<code>context.WithTimeout(ctx, 5*time.Second)</code>；<code>ctx.Err</code>；<code>c.reader.CommitMessages(ctx, kafka.Message{Topic: message.Topic, Partition: message.Partition, Offset: message.Offset})</code> |
| <code>message</code> | <code>domain.Record</code> | 使用 <code>domain.Record</code> 类型；其定义处的职责：把Kafka库对象转换为业务能校验的记录。<br>在此声明中的实际使用：<code>message.Topic</code>；<code>message.Partition</code>；<code>message.Offset</code>；<code>message.Value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_KAFKA_COMMIT_UNBOUND&quot;)</code>；<code>errors.New(&quot;SEARCH_KAFKA_COMMIT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>c.mu.Lock()</code>。
- 登记 <code>c.mu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>c.mu.Unlock()</code>。
- 检查 <code>c.closed &#124;&#124; c.blocked != nil &#124;&#124; c.reader == nil &#124;&#124; c.pending == nil &#124;&#124; message.Topic != c.configuration.Topic &#124;&#124; 		message.Partition != c.pending.partition &#124;&#124; message.Offset != c.p …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_KAFKA_COMMIT_UNBOUND&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 5*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>ctx.Err() != nil &#124;&#124; c.reader.CommitMessages(ctx, kafka.Message{Topic: message.Topic, Partition: message.Partition, Offset: message.Offset}) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_KAFKA_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4660:5509 -->
````go
func (c *Client) Commit(ctx context.Context, message domain.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.blocked != nil || c.reader == nil || c.pending == nil || message.Topic != c.configuration.Topic ||
		message.Partition != c.pending.partition || message.Offset != c.pending.offset || sha256.Sum256(message.Value) != c.pending.digest {
		return errors.New("SEARCH_KAFKA_COMMIT_UNBOUND")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// 已取消的调用不再入提交队列；真正发送后的超时仍是未知结果，保留原在途身份重试。
	if ctx.Err() != nil || c.reader.CommitMessages(ctx, kafka.Message{Topic: message.Topic, Partition: message.Partition, Offset: message.Offset}) != nil {
		return errors.New("SEARCH_KAFKA_COMMIT_UNKNOWN")
	}
	c.pending = nil
	return nil
}

````

## 第 10 步：Client.Close

插入位置：第 9 步代码之后，紧接追加；完成稿约第 128 行。

本步声明：`func (c *Client) Close() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;SEARCH_KAFKA_CLOSE_UNKNOWN&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>c.mu.Lock()</code>。
- 登记 <code>c.mu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>c.mu.Unlock()</code>。
- 检查 <code>c.closed</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 登记 <code>c.transport.CloseIdleConnections()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>c.reader != nil &amp;&amp; c.reader.Close() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_KAFKA_CLOSE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5509:5782 -->
````go
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	defer c.transport.CloseIdleConnections()
	if c.reader != nil && c.reader.Close() != nil {
		return errors.New("SEARCH_KAFKA_CLOSE_UNKNOWN")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/client/kafka/client.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
