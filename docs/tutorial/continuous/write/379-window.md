# 分步编写 `services/search/internal/client/kafka/window.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/379-window.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

同时读取真实保留窗口和broker消费组已提交位置。

SQL的下一offset必须仍可重读，broker位置不能领先SQL；连接成功不提供这两个证明。

写之前先按这条执行路线理解各段的连接：

1. Window限3秒，Metadata要求恰好分区0/1/2及有效leader
2. ListOffsets读取每分区First/Last
3. OffsetFetch读取group的Committed，核无重复/缺分区及不超过Last
4. 返回完整partition→window映射

本文件需要守住的边界：

- Last是排他末尾，Committed是下一位置
- 分区数量变化应拒绝，不可自动扩展数组继续旧协议

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/client/kafka' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/client/kafka/window.go') -and (Get-Item -LiteralPath 'services/search/internal/client/kafka/window.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/client/kafka/window.go')) { New-Item -ItemType File -Path 'services/search/internal/client/kafka/window.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:149 -->
````go
package kafka

import (
	domain "collaboration/search/internal/domain/event"
	"context"
	"errors"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

````

## 第 2 步：PartitionWindow

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`type PartitionWindow = domain.PartitionWindow`。

<!-- write-step: 2 bytes=149:196 -->
````go
type PartitionWindow = domain.PartitionWindow

````

## 第 3 步：Client.Window

插入位置：第 2 步代码之后，紧接追加；完成稿约第 14 行。

Window同时读取真实保留窗口和真实组位置。SQL下一offset须在窗口内，
broker位置不能领先SQL；调用方据此拒绝缺失历史，而不是接受自动跳转到最新位置。

本步声明：`func (c *Client) Window(ctx context.Context) (map[int]PartitionWindow, error)`。

**这一段为什么这样写：** 向消费者提供可核验的日志保留和提交前沿。

1. 验证固定分区集合
2. 读取最早/末尾offset
3. 读取组提交位置并比较范围


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 调用预算<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 3*time.Second)</code>；<code>context.WithTimeout(ctx, 3*time.Second)</code>；<code>c.api.Metadata(ctx, &amp;kafka.MetadataRequest{Topics: &#91;&#93;string{topic}})</code>；<code>c.api.ListOffsets(ctx, &amp;kafka.ListOffsetsRequest{Topics: map&#91;string&#93;&#91;&#93;kafka.OffsetRequest{topic: requests}})</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;int&#93;PartitionWindow</code> | 每个分区First/Last/Committed<br>实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 任一事实未知即失败<br>实际返回表达式：<code>errors.New(&quot;SEARCH_KAFKA_PARTITIONS_UNBOUND&quot;)</code>；<code>errors.New(&quot;SEARCH_KAFKA_WINDOW_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_KAFKA_WINDOW_UNBOUND&quot;)</code>；<code>errors.New(&quot;SEARCH_KAFKA_GROUP_UNAVAILABLE&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 3*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; metadata == nil &#124;&#124; len(metadata.Topics) != 1 &#124;&#124; metadata.Topics&#91;0&#93;.Name != topic &#124;&#124; 		metadata.Topics&#91;0&#93;.Error != nil &#124;&#124; len(metadata.Topics&#91;0&#93;.Partitions) != domain.PartitionCount</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_KAFKA_PARTITIONS_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>partition.ID &lt; 0 &#124;&#124; partition.ID &gt;= domain.PartitionCount &#124;&#124; seen&#91;partition.ID&#93; &#124;&#124; partition.Error != nil &#124;&#124; partition.Leader.Host == &quot;&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_KAFKA_PARTITIONS_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; offsets == nil &#124;&#124; len(offsets.Topics) != 1 &#124;&#124; len(offsets.Topics&#91;topic&#93;) != domain.PartitionCount</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_KAFKA_WINDOW_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>exists &#124;&#124; value.Partition &lt; 0 &#124;&#124; value.Partition &gt;= domain.PartitionCount &#124;&#124; value.Error != nil &#124;&#124; 			value.FirstOffset &lt; 0 &#124;&#124; value.LastOffset &lt; value.FirstOffset</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_KAFKA_WINDOW_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; committed == nil &#124;&#124; committed.Error != nil &#124;&#124; len(committed.Topics) != 1 &#124;&#124; len(committed.Topics&#91;topic&#93;) != domain.PartitionCount</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_KAFKA_GROUP_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>!exists &#124;&#124; seen&#91;value.Partition&#93; &#124;&#124; value.Error != nil &#124;&#124; value.CommittedOffset &lt; -1 &#124;&#124; value.CommittedOffset &gt; window.Last</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_KAFKA_GROUP_UNBOUND&quot;)</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=196:2951 -->
````go
// Window同时读取真实保留窗口和真实组位置。SQL下一offset须在窗口内，
// broker位置不能领先SQL；调用方据此拒绝缺失历史，而不是接受自动跳转到最新位置。
func (c *Client) Window(ctx context.Context) (map[int]PartitionWindow, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	topic := c.configuration.Topic
	metadata, err := c.api.Metadata(ctx, &kafka.MetadataRequest{Topics: []string{topic}})
	if err != nil || metadata == nil || len(metadata.Topics) != 1 || metadata.Topics[0].Name != topic ||
		metadata.Topics[0].Error != nil || len(metadata.Topics[0].Partitions) != domain.PartitionCount {
		return nil, errors.New("SEARCH_KAFKA_PARTITIONS_UNBOUND")
	}
	seen := map[int]bool{}
	for _, partition := range metadata.Topics[0].Partitions {
		if partition.ID < 0 || partition.ID >= domain.PartitionCount || seen[partition.ID] || partition.Error != nil || partition.Leader.Host == "" {
			return nil, errors.New("SEARCH_KAFKA_PARTITIONS_UNBOUND")
		}
		seen[partition.ID] = true
	}
	requests := make([]kafka.OffsetRequest, 0, 6)
	for partition := 0; partition < domain.PartitionCount; partition++ {
		requests = append(requests, kafka.FirstOffsetOf(partition), kafka.LastOffsetOf(partition))
	}
	offsets, err := c.api.ListOffsets(ctx, &kafka.ListOffsetsRequest{Topics: map[string][]kafka.OffsetRequest{topic: requests}})
	if err != nil || offsets == nil || len(offsets.Topics) != 1 || len(offsets.Topics[topic]) != domain.PartitionCount {
		return nil, errors.New("SEARCH_KAFKA_WINDOW_UNAVAILABLE")
	}
	result := make(map[int]PartitionWindow, 3)
	for _, value := range offsets.Topics[topic] {
		if _, exists := result[value.Partition]; exists || value.Partition < 0 || value.Partition >= domain.PartitionCount || value.Error != nil ||
			value.FirstOffset < 0 || value.LastOffset < value.FirstOffset {
			return nil, errors.New("SEARCH_KAFKA_WINDOW_UNBOUND")
		}
		result[value.Partition] = PartitionWindow{First: value.FirstOffset, Last: value.LastOffset, Committed: -1}
	}
	committed, err := c.api.OffsetFetch(ctx, &kafka.OffsetFetchRequest{GroupID: c.configuration.Group, Topics: map[string][]int{topic: {0, 1, 2}}})
	if err != nil || committed == nil || committed.Error != nil || len(committed.Topics) != 1 || len(committed.Topics[topic]) != domain.PartitionCount {
		return nil, errors.New("SEARCH_KAFKA_GROUP_UNAVAILABLE")
	}
	seen = map[int]bool{}
	for _, value := range committed.Topics[topic] {
		window, exists := result[value.Partition]
		if !exists || seen[value.Partition] || value.Error != nil || value.CommittedOffset < -1 || value.CommittedOffset > window.Last {
			return nil, errors.New("SEARCH_KAFKA_GROUP_UNBOUND")
		}
		seen[value.Partition] = true
````

## 第 4 步：Client.Window（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 59 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2951:3054 -->
````go
		window.Committed = value.CommittedOffset
		result[value.Partition] = window
	}
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/client/kafka/window.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
