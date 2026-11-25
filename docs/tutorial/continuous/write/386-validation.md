# 分步编写 `services/search/internal/repository/event/validation.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/386-validation.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

重新解析并核验准备写账本的Event原行及传输位置。

即使业务层传入Event结构体，字段也可能被修改，仓储不能只信它附带RawDigest。

写之前先按这条执行路线理解各段的连接：

1. validatePosition限制consumer/topic及非负partition/offset
2. validateEvent调用RestoreEvent重建原字段
3. 比较原字节SHA-256、规范字节及全部ID/序号/时间字段

本文件需要守住的边界：

- 不能只比较event_id认为重复安全
- 当前Doc权威摘要不能替换原RawDigest

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/repository/event' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/repository/event/validation.go') -and (Get-Item -LiteralPath 'services/search/internal/repository/event/validation.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/repository/event/validation.go')) { New-Item -ItemType File -Path 'services/search/internal/repository/event/validation.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:126 -->
````go
package event

import (
	"bytes"
	domain "collaboration/search/internal/domain/event"
	"crypto/sha256"
	"errors"
	"regexp"
)

````

## 第 2 步：Position

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

本步声明：`type Position = domain.Position`。

<!-- write-step: 2 bytes=126:159 -->
````go
type Position = domain.Position

````

## 第 3 步：consumerName

插入位置：第 2 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`var consumerName = regexp.MustCompile(\`^[A-Za-z0-9][A-Za-z0-9._:-]{0,39}$\`)`。

<!-- write-step: 3 bytes=159:235 -->
````go
var consumerName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,39}$`)
````

## 第 4 步：topicName

插入位置：第 3 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`var topicName = regexp.MustCompile(\`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$\`)`。

<!-- write-step: 4 bytes=235:308 -->
````go
var topicName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$`)
````

## 第 5 步：physicalName

插入位置：第 4 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`var physicalName = regexp.MustCompile(\`^[a-z0-9][a-z0-9._-]{0,159}$\`)`。

<!-- write-step: 5 bytes=308:378 -->
````go
var physicalName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)
````

## 第 6 步：errorCode

插入位置：第 5 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`var errorCode = regexp.MustCompile(\`^[A-Z][A-Z0-9_]{1,63}$\`)`。

<!-- write-step: 6 bytes=378:440 -->
````go
var errorCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

````

## 第 7 步：validatePosition

插入位置：第 6 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func validatePosition(consumer string, position Position) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>consumer</code> | <code>string</code> | 在此声明中的实际使用：<code>consumerName.MatchString(consumer)</code> |
| <code>position</code> | <code>Position</code> | 在此声明中的实际使用：<code>position.Topic</code>；<code>position.Partition</code>；<code>position.Offset</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_MESSAGE_POSITION_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!consumerName.MatchString(consumer) &#124;&#124; !topicName.MatchString(position.Topic) &#124;&#124; position.Partition &lt; 0 &#124;&#124; position.Offset &lt; 0</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_MESSAGE_POSITION_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=440:712 -->
````go
func validatePosition(consumer string, position Position) error {
	if !consumerName.MatchString(consumer) || !topicName.MatchString(position.Topic) || position.Partition < 0 || position.Offset < 0 {
		return errors.New("SEARCH_MESSAGE_POSITION_INVALID")
	}
	return nil
}

````

## 第 8 步：validateEvent

插入位置：第 7 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func validateEvent(value domain.Event) error`。

**这一段为什么这样写：** 在SQL入口证明Event仍对应不可变原outbox行。

1. 恢复原行
2. 核摘要与原字节
3. 逐个比较业务字段


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>domain.Event</code> | 拟登记的事件<br>在此声明中的实际使用：<code>value.Original</code>；<code>value.RawDigest</code>；<code>value.ID</code>；<code>value.Type</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 非nil表示不能写入或重放该事件<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ORIGINAL_EVENT_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; sha256.Sum256(value.Original) != value.RawDigest &#124;&#124; !bytes.Equal(stored.Original, value.Original) &#124;&#124; 		stored.ID != value.ID &#124;&#124; stored.Type != value.Type &#124;&#124; stored.Do …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_ORIGINAL_EVENT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=712:1267 -->
````go
func validateEvent(value domain.Event) error {
	stored, err := domain.RestoreEvent(value.Original)
	if err != nil || sha256.Sum256(value.Original) != value.RawDigest || !bytes.Equal(stored.Original, value.Original) ||
		stored.ID != value.ID || stored.Type != value.Type || stored.Document != value.Document ||
		stored.Sequence != value.Sequence || stored.Release != value.Release ||
		stored.PublicationEpoch != value.PublicationEpoch || !stored.OccurredAt.Equal(value.OccurredAt) {
		return errors.New("SEARCH_ORIGINAL_EVENT_INVALID")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/repository/event/validation.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
