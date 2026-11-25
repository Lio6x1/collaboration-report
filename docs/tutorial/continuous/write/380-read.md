# 分步编写 `services/search/internal/client/logsource/read.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/380-read.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按原TopicID只读指定重建区间，不参与普通消费组提交。

重建的L不能随着next推进被重写，否则原L丢失后会假称窗口仍完整。

写之前先按这条执行路线理解各段的连接：

1. Read核partition和next>=原L，Capture重验身份/保留窗口
2. 刷新leader但Fetch v13带原TopicID，禁增量fetch session
3. 核response TopicID、分区、HighWatermark>=until和无异常事务
4. decodeRecords后再Capture，任一身份或保留变化都不交付已解码前缀

本文件需要守住的边界：

- checkRange检查所有分区原L仍保留，而不只当前next
- 重建读取不能提交普通消费组offset

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/client/logsource' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/client/logsource/read.go') -and (Get-Item -LiteralPath 'services/search/internal/client/logsource/read.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/client/logsource/read.go')) { New-Item -ItemType File -Path 'services/search/internal/client/logsource/read.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:151 -->
````go
package logsource

import (
	domain "collaboration/search/internal/domain/event"
	"context"
	"errors"
	"time"

	"github.com/twmb/franz-go/pkg/kmsg"
)

````

## 第 2 步：Client.Read

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

Read从原准备窗口的某分区读取一个有界批次，until为本轮封存的排他前沿。
结果只有全部校验成功才交付；调用方应用整条记录后持久推进自己的next，不提交普通组。

本步声明：`func (c *Client) Read(ctx context.Context, original Window, partition int, next, until int64) ([]domain.Record, int64, error)`。

**这一段为什么这样写：** 从冻结日志身份与边界中读一个有界完整批次。

1. 前置身份/保留检查
2. 按UUID获取原分区字节
3. 验证完整批
4. 后置再次核验原L


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 本次读取父期限<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 3*time.Second)</code>；<code>context.WithTimeout(ctx, 3*time.Second)</code>；<code>c.Capture(ctx, &amp;original.Identity)</code>；<code>metadataRequest.RequestWith(ctx, c.api)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>original</code> | <code>Window</code> | prepare保存的原日志窗口，Last用作L<br>在此声明中的实际使用：<code>original.Identity</code>；<code>original.Partitions</code>；<code>checkRange(original, window, partition, until)</code> |
| <code>partition</code> | <code>int</code> | 本轮重放分区0/1/2<br>在此声明中的实际使用：<code>partition &lt; 0</code>；<code>partition &gt;= domain.PartitionCount</code>；<code>original.Partitions&#91;partition&#93;.Last</code>；<code>checkRange(original, window, partition, until)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>next</code> | <code>int64</code> | 任务持久下一读取位置<br>在此声明中的实际使用：<code>return nil, next, errors.New(code)</code>；<code>next &lt; original.Partitions&#91;partition&#93;.Last</code>；<code>until &lt; next</code>；<code>return nil, next, err</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>until</code> | <code>int64</code> | 冻结H或封口C排他前沿<br>在此声明中的实际使用：<code>until &lt; next</code>；<code>checkRange(original, window, partition, until)</code>；<code>next == until</code>；<code>value.HighWatermark &lt; until</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.Record</code> | 完整可应用记录批<br>由 <code>failed(&quot;SEARCH_LOG_RANGE_INVALID&quot;)</code> 的第 1 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_ROUTE_UNAVAILABLE&quot;)</code> 的第 1 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code> 的第 1 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_FETCH_UNAVAILABLE&quot;)</code> 的第 1 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_FETCH_UNBOUND&quot;)</code> 的第 1 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_WINDOW_LOST&quot;)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>records</code> |
| <code>(匿名/未命名)</code> | <code>int64</code> | 解码后的下一offset<br>由 <code>failed(&quot;SEARCH_LOG_RANGE_INVALID&quot;)</code> 的第 2 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_ROUTE_UNAVAILABLE&quot;)</code> 的第 2 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code> 的第 2 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_FETCH_UNAVAILABLE&quot;)</code> 的第 2 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_FETCH_UNBOUND&quot;)</code> 的第 2 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_WINDOW_LOST&quot;)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>next</code>；<code>after</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败时禁止推进checkpoint<br>由 <code>failed(&quot;SEARCH_LOG_RANGE_INVALID&quot;)</code> 的第 3 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_ROUTE_UNAVAILABLE&quot;)</code> 的第 3 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code> 的第 3 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_FETCH_UNAVAILABLE&quot;)</code> 的第 3 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_FETCH_UNBOUND&quot;)</code> 的第 3 个返回项原样转交。<br>由 <code>failed(&quot;SEARCH_LOG_WINDOW_LOST&quot;)</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, next, errors.New(code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>partition &lt; 0 &#124;&#124; partition &gt;= domain.PartitionCount &#124;&#124; original.Identity.Topic != c.topic &#124;&#124; 		original.Identity.TopicID == &#91;16&#93;byte{} &#124;&#124; original.Partitions&#91;partition&#93;.Last &lt; 0 &#124;&#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return failed(&quot;SEARCH_LOG_RANGE_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 3*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, next, err</code>。当前函数在这里结束。
- 检查 <code>next == until</code>。条件成立时执行 <code>return nil, next, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return failed(&quot;SEARCH_LOG_ROUTE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; identity != original.Identity</code>。条件成立时执行 <code>return failed(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>route.Partition == int32(partition)</code>。

<!-- write-step: 2 bytes=151:2316 -->
````go
// Read从原准备窗口的某分区读取一个有界批次，until为本轮封存的排他前沿。
// 结果只有全部校验成功才交付；调用方应用整条记录后持久推进自己的next，不提交普通组。
func (c *Client) Read(ctx context.Context, original Window, partition int, next, until int64) ([]domain.Record, int64, error) {
	failed := func(code string) ([]domain.Record, int64, error) { return nil, next, errors.New(code) }
	if partition < 0 || partition >= domain.PartitionCount || original.Identity.Topic != c.topic ||
		original.Identity.TopicID == [16]byte{} || original.Partitions[partition].Last < 0 ||
		next < original.Partitions[partition].Last || until < next {
		return failed("SEARCH_LOG_RANGE_INVALID")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	window, err := c.Capture(ctx, &original.Identity)
	if err != nil {
		return nil, next, err
	}
	if err := checkRange(original, window, partition, until); err != nil {
		return nil, next, err
	}
	if next == until {
		return nil, next, nil
	}

	// 只用当前元数据刷新leader路由，Fetch请求仍携带原TopicID，绝不跟随同名新日志。
	metadataRequest := kmsg.NewPtrMetadataRequest()
	metadataRequest.Topics = []kmsg.MetadataRequestTopic{{Topic: &c.topic}}
	metadata, err := metadataRequest.RequestWith(ctx, c.api)
	if err != nil {
		return failed("SEARCH_LOG_ROUTE_UNAVAILABLE")
	}
	identity, err := checkedIdentity(c.topic, metadata)
	if err != nil || identity != original.Identity {
		return failed("SEARCH_LOG_IDENTITY_CHANGED")
	}
	var leader, epoch int32
	for _, route := range metadata.Topics[0].Partitions {
		if route.Partition == int32(partition) {
			leader, epoch = route.Leader, route.LeaderEpoch
		}
	}
	request := kmsg.NewPtrFetchRequest()
	request.MaxWaitMillis, request.MinBytes, request.MaxBytes = 250, 1, 4<<20
	// 无增量fetch session，不产生需要跨重启保存的broker会话。
	request.SessionID, request.SessionEpoch = 0, -1
	part := kmsg.NewFetchRequestTopicPartition()
	part.Partition, part.CurrentLeaderEpoch, part.FetchOffset, part.PartitionMaxBytes = int32(partition), epoch, next, 4<<20
````

## 第 3 步：Client.Read（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 57 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return failed(&quot;SEARCH_LOG_FETCH_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>response == nil &#124;&#124; response.Version != 13 &#124;&#124; response.ErrorCode != 0 &#124;&#124; len(response.Topics) != 1 &#124;&#124; 		response.Topics&#91;0&#93;.TopicID != original.Identity.TopicID &#124;&#124; len(response.Topics&#91;0&#93;.Partitions) != 1</code>。条件成立时执行 <code>return failed(&quot;SEARCH_LOG_FETCH_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>value.ErrorCode == 1 &#124;&#124; value.LogStartOffset &gt; original.Partitions&#91;partition&#93;.Last</code>。条件成立时执行 <code>return failed(&quot;SEARCH_LOG_WINDOW_LOST&quot;)</code>。当前函数在这里结束。
- 检查 <code>value.Partition != int32(partition) &#124;&#124; value.ErrorCode != 0 &#124;&#124; value.LogStartOffset &lt; 0 &#124;&#124; 		value.HighWatermark &lt; until &#124;&#124; len(value.AbortedTransactions) != 0</code>。条件成立时执行 <code>return failed(&quot;SEARCH_LOG_FETCH_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, next, err</code>。当前函数在这里结束。
- 此处通过 <code>return records, after, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2316:3706 -->
````go
	request.Topics = []kmsg.FetchRequestTopic{{TopicID: original.Identity.TopicID, Partitions: []kmsg.FetchRequestTopicPartition{part}}}
	response, err := request.RequestWith(ctx, c.api.Broker(int(leader)))
	if err != nil {
		return failed("SEARCH_LOG_FETCH_UNAVAILABLE")
	}
	if response == nil || response.Version != 13 || response.ErrorCode != 0 || len(response.Topics) != 1 ||
		response.Topics[0].TopicID != original.Identity.TopicID || len(response.Topics[0].Partitions) != 1 {
		return failed("SEARCH_LOG_FETCH_UNBOUND")
	}
	value := response.Topics[0].Partitions[0]
	if value.ErrorCode == 1 || value.LogStartOffset > original.Partitions[partition].Last {
		return failed("SEARCH_LOG_WINDOW_LOST")
	}
	if value.Partition != int32(partition) || value.ErrorCode != 0 || value.LogStartOffset < 0 ||
		value.HighWatermark < until || len(value.AbortedTransactions) != 0 {
		return failed("SEARCH_LOG_FETCH_UNBOUND")
	}
	records, after, err := decodeRecords(value.RecordBatches, c.topic, partition, next, until)
	if err != nil {
		return nil, next, err
	}
	// 元数据/保留策略变化时也不交付已经解码的前缀；持久原L不能因next前进而改写。
	window, err = c.Capture(ctx, &original.Identity)
	if err != nil {
		return nil, next, err
	}
	if err := checkRange(original, window, partition, until); err != nil {
		return nil, next, err
	}
	return records, after, nil
}

````

## 第 4 步：checkRange

插入位置：第 3 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`func checkRange(original, current Window, partition int, until int64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>original</code> | <code>Window</code> | 使用 <code>Window</code> 类型；其定义处的职责：同一次身份核验期间观察的完整三分区窗口。<br>在此声明中的实际使用：<code>original.Partitions</code> |
| <code>current</code> | <code>Window</code> | 使用 <code>Window</code> 类型；其定义处的职责：同一次身份核验期间观察的完整三分区窗口。<br>在此声明中的实际使用：<code>current.Partitions</code> |
| <code>partition</code> | <code>int</code> | 在此声明中的实际使用：<code>current.Partitions&#91;partition&#93;.Last</code> |
| <code>until</code> | <code>int64</code> | 在此声明中的实际使用：<code>until &gt; current.Partitions&#91;partition&#93;.Last</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_LOG_WINDOW_LOST&quot;)</code>；<code>errors.New(&quot;SEARCH_LOG_FRONTIER_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>original.Partitions&#91;i&#93;.Last &lt; current.Partitions&#91;i&#93;.First</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_WINDOW_LOST&quot;)</code>。当前函数在这里结束。
- 检查 <code>until &gt; current.Partitions&#91;partition&#93;.Last</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_FRONTIER_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=3706:4062 -->
````go
func checkRange(original, current Window, partition int, until int64) error {
	for i := range original.Partitions {
		if original.Partitions[i].Last < current.Partitions[i].First {
			return errors.New("SEARCH_LOG_WINDOW_LOST")
		}
	}
	if until > current.Partitions[partition].Last {
		return errors.New("SEARCH_LOG_FRONTIER_UNAVAILABLE")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/client/logsource/read.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
