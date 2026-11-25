# 分步编写 `services/search/internal/domain/event/canal.go`

先按 [S01 · Canal事件与Doc当前正式投影](../lessons/search/01-event-authority.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/369-canal.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

严格解析Doc outbox INSERT Canal消息，绑定每行与payload并生成原事件摘要。

Search只应由公开outbox引用驱动；误接私有表、DDL或宽松payload会制造假成功或泄漏数据。

写之前先按这条执行路线理解各段的连接：

1. ParseCanal限制2MiB UTF8、document_db.outbox、INSERT、主键event_id及1..1024行
2. parseRow要求八个原字段和八个payload字段，核重复身份、schema=1和允许事件类型
3. 规范十进制uint64，发布必须release非空、撤下必须为空，时间必须UTC Z
4. 规范序列化原行并hash，整条所有行解析成功才交调用者

本文件需要守住的边界：

- Canal外层批次id/时间不是业务摘要的一部分
- 可见性改变可发生在未发布文档，不能据该事件推断匿名可读

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/domain/event' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/domain/event/canal.go') -and (Get-Item -LiteralPath 'services/search/internal/domain/event/canal.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/domain/event/canal.go')) { New-Item -ItemType File -Path 'services/search/internal/domain/event/canal.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:127 -->
````go
package event

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

````

## 第 2 步：MaxRecordBytes

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`const MaxRecordBytes = 2 << 20`。

<!-- write-step: 2 bytes=127:159 -->
````go
const MaxRecordBytes = 2 << 20

````

## 第 3 步：flatMessage

插入位置：第 2 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`type flatMessage struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>json.Number</code> | 序列化/校验标记：<code>`json:&quot;id&quot;`</code> |
| <code>Database</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;database&quot;`</code> |
| <code>Table</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;table&quot;`</code> |
| <code>PKNames</code> | <code>&#91;&#93;string</code> | 序列化/校验标记：<code>`json:&quot;pkNames&quot;`</code> |
| <code>IsDDL</code> | <code>*bool</code> | 序列化/校验标记：<code>`json:&quot;isDdl&quot;`</code> |
| <code>Type</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;type&quot;`</code> |
| <code>ES</code> | <code>json.Number</code> | 序列化/校验标记：<code>`json:&quot;es&quot;`</code> |
| <code>TS</code> | <code>json.Number</code> | 序列化/校验标记：<code>`json:&quot;ts&quot;`</code> |
| <code>SQL</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;sql&quot;`</code> |
| <code>SQLTypes</code> | <code>map&#91;string&#93;int</code> | 序列化/校验标记：<code>`json:&quot;sqlType&quot;`</code> |
| <code>MySQLType</code> | <code>map&#91;string&#93;string</code> | 序列化/校验标记：<code>`json:&quot;mysqlType&quot;`</code> |
| <code>Data</code> | <code>&#91;&#93;map&#91;string&#93;json.RawMessage</code> | 序列化/校验标记：<code>`json:&quot;data&quot;`</code> |
| <code>Old</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>序列化/校验标记：<code>`json:&quot;old&quot;`</code> |
| <code>GTID</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;gtid&quot;`</code> |

<!-- write-step: 3 bytes=159:953 -->
````go
type flatMessage struct {
	ID        json.Number                  `json:"id"`
	Database  string                       `json:"database"`
	Table     string                       `json:"table"`
	PKNames   []string                     `json:"pkNames"`
	IsDDL     *bool                        `json:"isDdl"`
	Type      string                       `json:"type"`
	ES        json.Number                  `json:"es"`
	TS        json.Number                  `json:"ts"`
	SQL       string                       `json:"sql"`
	SQLTypes  map[string]int               `json:"sqlType"`
	MySQLType map[string]string            `json:"mysqlType"`
	Data      []map[string]json.RawMessage `json:"data"`
	Old       json.RawMessage              `json:"old"`
	GTID      string                       `json:"gtid"`
}

````

## 第 4 步：payload

插入位置：第 3 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`type payload struct {`。

**这一段为什么这样写：** 原outbox payload的严格线形契约。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 与外层event_id相同<br>序列化/校验标记：<code>`json:&quot;event_id&quot;`</code> |
| <code>Type</code> | <code>string</code> | 与外层event_type相同<br>序列化/校验标记：<code>`json:&quot;event_type&quot;`</code> |
| <code>Schema</code> | <code>string</code> | 固定字符串1<br>序列化/校验标记：<code>`json:&quot;schema_version&quot;`</code> |
| <code>Document</code> | <code>string</code> | 与aggregate_id相同的规范十进制文档ID<br>序列化/校验标记：<code>`json:&quot;document_id&quot;`</code> |
| <code>Sequence</code> | <code>string</code> | 与aggregate_seq相同的规范十进制聚合序号<br>序列化/校验标记：<code>`json:&quot;aggregate_seq&quot;`</code> |
| <code>Release</code> | <code>*string</code> | 可为空发布ID，空值语义依事件类型<br>序列化/校验标记：<code>`json:&quot;release_id&quot;`</code> |
| <code>Epoch</code> | <code>string</code> | 发布代次字符串<br>序列化/校验标记：<code>`json:&quot;publication_epoch&quot;`</code> |
| <code>Occurred</code> | <code>string</code> | RFC3339Nano UTC业务时间<br>序列化/校验标记：<code>`json:&quot;occurred_at&quot;`</code> |

<!-- write-step: 4 bytes=953:1296 -->
````go
type payload struct {
	ID       string  `json:"event_id"`
	Type     string  `json:"event_type"`
	Schema   string  `json:"schema_version"`
	Document string  `json:"document_id"`
	Sequence string  `json:"aggregate_seq"`
	Release  *string `json:"release_id"`
	Epoch    string  `json:"publication_epoch"`
	Occurred string  `json:"occurred_at"`
}

````

## 第 5 步：validEventID

插入位置：第 4 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`var validEventID = regexp.MustCompile(\`^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$\`)`。

<!-- write-step: 5 bytes=1296:1373 -->
````go
var validEventID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$`)

````

## 第 6 步：ParseCanal

插入位置：第 5 步代码之后，紧接追加；完成稿约第 46 行。

ParseCanal只接受受限复制表的INSERT，不把DDL、其他私有表或未知schema当可消费成功。
一条Kafka记录可包含多行；调用方只有在全部行完成后才能同步提交该记录的offset。

本步声明：`func ParseCanal(raw []byte) ([]Event, error)`。

**这一段为什么这样写：** 以整条Kafka记录为解析边界，禁止只处理合法前缀。

1. 验证封包
2. 逐行严格解析
3. 全部成功才返回事件数组


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | Kafka记录原始Value字节<br>在此声明中的实际使用：<code>len(raw)</code>；<code>utf8.Valid(raw)</code>；<code>decodeStrict(raw, &amp;message)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;Event</code> | 按原data顺序的不可变事件列表<br>实际返回表达式：<code>nil</code>；<code>events</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 稳定CANAL错误码；非nil时整条记录不得提交offset<br>实际返回表达式：<code>errors.New(&quot;CANAL_RECORD_SIZE_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;CANAL_RECORD_BINDING_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) == 0 &#124;&#124; len(raw) &gt; MaxRecordBytes &#124;&#124; !utf8.Valid(raw)</code>。条件成立时执行 <code>return nil, errors.New(&quot;CANAL_RECORD_SIZE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>message.Database != &quot;document_db&quot; &#124;&#124; message.Table != &quot;outbox&quot; &#124;&#124; message.IsDDL == nil &#124;&#124; *message.IsDDL &#124;&#124; 		message.Type != &quot;INSERT&quot; &#124;&#124; len(message.PKNames) != 1 &#124;&#124; message.PKNam …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, errors.New(&quot;CANAL_RECORD_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return events, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1373:2407 -->
````go
// ParseCanal只接受受限复制表的INSERT，不把DDL、其他私有表或未知schema当可消费成功。
// 一条Kafka记录可包含多行；调用方只有在全部行完成后才能同步提交该记录的offset。
func ParseCanal(raw []byte) ([]Event, error) {
	if len(raw) == 0 || len(raw) > MaxRecordBytes || !utf8.Valid(raw) {
		return nil, errors.New("CANAL_RECORD_SIZE_INVALID")
	}
	var message flatMessage
	if err := decodeStrict(raw, &message); err != nil {
		return nil, err
	}
	if message.Database != "document_db" || message.Table != "outbox" || message.IsDDL == nil || *message.IsDDL ||
		message.Type != "INSERT" || len(message.PKNames) != 1 || message.PKNames[0] != "event_id" ||
		len(message.Data) == 0 || len(message.Data) > 1024 {
		return nil, errors.New("CANAL_RECORD_BINDING_INVALID")
	}
	events := make([]Event, 0, len(message.Data))
	for _, row := range message.Data {
		value, err := parseRow(row)
		if err != nil {
			return nil, err
		}
		events = append(events, value)
	}
	return events, nil
}

````

## 第 7 步：RestoreEvent

插入位置：第 6 步代码之后，紧接追加；完成稿约第 72 行。

RestoreEvent供私有记账核验原行；与真实Canal解析共用同一绑定规则，不引入较弱的恢复入口。

本步声明：`func RestoreEvent(raw []byte) (Event, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>utf8.Valid(raw)</code>；<code>decodeStrict(raw, &amp;row)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Event</code> | 使用 <code>Event</code> 类型；其定义处的职责：保存能够重新核验的原业务事件，而不是ES当前投影。<br>由 <code>parseRow(row)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>Event{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>parseRow(row)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;CANAL_RECORD_SIZE_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) == 0 &#124;&#124; len(raw) &gt; MaxRecordBytes &#124;&#124; !utf8.Valid(raw)</code>。条件成立时执行 <code>return Event{}, errors.New(&quot;CANAL_RECORD_SIZE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return Event{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return parseRow(row)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2407:2845 -->
````go
// RestoreEvent供私有记账核验原行；与真实Canal解析共用同一绑定规则，不引入较弱的恢复入口。
func RestoreEvent(raw []byte) (Event, error) {
	if len(raw) == 0 || len(raw) > MaxRecordBytes || !utf8.Valid(raw) {
		return Event{}, errors.New("CANAL_RECORD_SIZE_INVALID")
	}
	var row map[string]json.RawMessage
	if err := decodeStrict(raw, &row); err != nil {
		return Event{}, err
	}
	return parseRow(row)
}

````

## 第 8 步：parseRow

插入位置：第 7 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`func parseRow(row map[string]json.RawMessage) (Event, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>map&#91;string&#93;json.RawMessage</code> | 在此声明中的实际使用：<code>len(row)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Event</code> | 使用 <code>Event</code> 类型；其定义处的职责：保存能够重新核验的原业务事件，而不是ES当前投影。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;CANAL_ROW_FIELDS_INVALID&quot;)</code>；<code>errors.New(&quot;CANAL_EVENT_TYPE_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;CANAL_PAYLOAD_FIELDS_INVALID&quot;)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>len(row) != len(allowed)</code>。条件成立时执行 <code>return result, errors.New(&quot;CANAL_ROW_FIELDS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!allowed&#91;key&#93; &#124;&#124; json.Unmarshal(raw, &amp;text) != nil &#124;&#124; text == &quot;&quot;</code>。条件成立时执行 <code>return result, errors.New(&quot;CANAL_ROW_FIELDS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!validEventID.MatchString(values&#91;&quot;event_id&quot;&#93;) &#124;&#124; values&#91;&quot;aggregate_kind&quot;&#93; != &quot;DOCUMENT&quot; &#124;&#124; 		values&#91;&quot;schema_version&quot;&#93; != &quot;1&quot; &#124;&#124; 		(values&#91;&quot;event_type&quot;&#93; != &quot;DOCUMENT_PUBLISHED&quot; &amp;&amp; v …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return result, errors.New(&quot;CANAL_EVENT_TYPE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>len(fields) != 8</code>。条件成立时执行 <code>return result, errors.New(&quot;CANAL_PAYLOAD_FIELDS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!present</code>。条件成立时执行 <code>return result, errors.New(&quot;CANAL_PAYLOAD_FIELDS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>body.ID != values&#91;&quot;event_id&quot;&#93; &#124;&#124; body.Type != values&#91;&quot;event_type&quot;&#93; &#124;&#124; body.Schema != &quot;1&quot; &#124;&#124; 		body.Document != values&#91;&quot;aggregate_id&quot;&#93; &#124;&#124; body.Sequence != values&#91;&quot;aggregate_seq&quot;&#93;</code>。条件成立时执行 <code>return result, errors.New(&quot;CANAL_PAYLOAD_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=2845:4782 -->
````go
func parseRow(row map[string]json.RawMessage) (Event, error) {
	var result Event
	allowed := map[string]bool{"event_id": true, "aggregate_kind": true, "aggregate_id": true,
		"aggregate_seq": true, "event_type": true, "schema_version": true, "payload": true, "created_at": true}
	if len(row) != len(allowed) {
		return result, errors.New("CANAL_ROW_FIELDS_INVALID")
	}
	values := make(map[string]string, len(row))
	for key, raw := range row {
		var text string
		if !allowed[key] || json.Unmarshal(raw, &text) != nil || text == "" {
			return result, errors.New("CANAL_ROW_FIELDS_INVALID")
		}
		values[key] = text
	}
	if !validEventID.MatchString(values["event_id"]) || values["aggregate_kind"] != "DOCUMENT" ||
		values["schema_version"] != "1" ||
		(values["event_type"] != "DOCUMENT_PUBLISHED" && values["event_type"] != "DOCUMENT_UNPUBLISHED" &&
			values["event_type"] != "DOCUMENT_VISIBILITY_CHANGED") {
		return result, errors.New("CANAL_EVENT_TYPE_INVALID")
	}
	var body payload
	var fields map[string]json.RawMessage
	if err := decodeStrict([]byte(values["payload"]), &fields); err != nil {
		return result, err
	}
	if len(fields) != 8 {
		return result, errors.New("CANAL_PAYLOAD_FIELDS_INVALID")
	}
	for _, key := range []string{"event_id", "event_type", "schema_version", "document_id", "aggregate_seq", "release_id", "publication_epoch", "occurred_at"} {
		if _, present := fields[key]; !present {
			return result, errors.New("CANAL_PAYLOAD_FIELDS_INVALID")
		}
	}
	if err := decodeStrict([]byte(values["payload"]), &body); err != nil {
		return result, err
	}
	if body.ID != values["event_id"] || body.Type != values["event_type"] || body.Schema != "1" ||
		body.Document != values["aggregate_id"] || body.Sequence != values["aggregate_seq"] {
		return result, errors.New("CANAL_PAYLOAD_BINDING_INVALID")
	}
	var err error
	if result.Document, err = canonicalPositive(body.Document); err != nil {
		return result, err
	}
````

## 第 9 步：parseRow（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 129 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>body.Type == &quot;DOCUMENT_PUBLISHED&quot; &amp;&amp; body.Release == nil</code>。条件成立时执行 <code>return result, errors.New(&quot;CANAL_RELEASE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>body.Release != nil</code>。
- 检查 <code>body.Type == &quot;DOCUMENT_UNPUBLISHED&quot;</code>。条件成立时执行 <code>return result, errors.New(&quot;CANAL_RELEASE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !strings.HasSuffix(body.Occurred, &quot;Z&quot;)</code>。条件成立时执行 <code>return result, errors.New(&quot;CANAL_EVENT_TIME_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;CANAL_ORIGIN_ENCODING_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4782:6149 -->
````go
	if result.Sequence, err = canonicalPositive(body.Sequence); err != nil {
		return result, err
	}
	if result.PublicationEpoch, err = canonicalPositive(body.Epoch); err != nil {
		return result, err
	}
	if body.Type == "DOCUMENT_PUBLISHED" && body.Release == nil {
		return result, errors.New("CANAL_RELEASE_BINDING_INVALID")
	}
	if body.Release != nil {
		if body.Type == "DOCUMENT_UNPUBLISHED" {
			return result, errors.New("CANAL_RELEASE_BINDING_INVALID")
		}
		if result.Release, err = canonicalPositive(*body.Release); err != nil {
			return result, err
		}
	}
	// 可见性变化既可发生于未发布文档，也可绑定当前正式指针；不推导匿名可读或改变发布代次。
	// Search仍取Doc当前权威快照，原发布/撤下的非空/空指针约束不能随新类型放宽。
	result.OccurredAt, err = time.Parse(time.RFC3339Nano, body.Occurred)
	if err != nil || !strings.HasSuffix(body.Occurred, "Z") {
		return result, errors.New("CANAL_EVENT_TIME_INVALID")
	}
	result.ID, result.Type = body.ID, body.Type
	// map按键排序编码，保留原payload字符串及不可变行字段。外层批次元数据重投时会改变。
	origin, err := json.Marshal(values)
	if err != nil {
		return result, errors.New("CANAL_ORIGIN_ENCODING_FAILED")
	}
	result.RawDigest = sha256.Sum256(origin)
	result.Original = origin
	return result, nil
}

````

## 第 10 步：canonicalPositive

插入位置：第 9 步代码之后，紧接追加；完成稿约第 163 行。

本步声明：`func canonicalPositive(value string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>strconv.ParseUint(value, 10, 64)</code>；<code>strconv.FormatUint(number, 10) != value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>number</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;CANAL_UINT64_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; number == 0 &#124;&#124; strconv.FormatUint(number, 10) != value</code>。条件成立时执行 <code>return 0, errors.New(&quot;CANAL_UINT64_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return number, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6149:6400 -->
````go
func canonicalPositive(value string) (uint64, error) {
	number, err := strconv.ParseUint(value, 10, 64)
	if err != nil || number == 0 || strconv.FormatUint(number, 10) != value {
		return 0, errors.New("CANAL_UINT64_INVALID")
	}
	return number, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/domain/event/canal.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/01-event-authority.md)。
