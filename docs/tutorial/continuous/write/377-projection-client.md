# 分步编写 `services/search/internal/client/elastic/projection_client.go`

先按 [S01 · Canal事件与Doc当前正式投影](../lessons/search/01-event-authority.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/377-projection-client.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

用原子脚本按uint64水位更新ES，并重读实际已应用状态。

ES可能已经有更高事件或上次请求虽超时已成功；SQL必须记实际高水位而不是本次请求旧水位。

写之前先按这条执行路线理解各段的连接：

1. project核Authority，文档ID/seq/release转字符串，隐藏态输出空正文
2. applyScript以20位sequence_key比较，新序替换，同序不同digest报错，旧序noop
3. Apply成功或noop后getProjection重读元数据并核水位不低于请求
4. Get读取完整投影供重建全量核验，隐藏墓碑必须无title/text/chapters

本文件需要守住的边界：

- 不要使用ES long版本保存uint64序号
- script成功不等于可直接写SQL请求水位
- 同序不同摘要是冲突，不能通过retry覆盖

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/client/elastic' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/client/elastic/projection_client.go') -and (Get-Item -LiteralPath 'services/search/internal/client/elastic/projection_client.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/client/elastic/projection_client.go')) { New-Item -ItemType File -Path 'services/search/internal/client/elastic/projection_client.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:323 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package elastic

import (
	index "collaboration/search/internal/domain/index"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"unicode/utf8"
)

// application.go：保留原实现，按业务合并文件。

````

## 第 2 步：applyScript

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`const applyScript = \``。

<!-- write-step: 2 bytes=323:809 -->
````go
const applyScript = `
if (ctx.op == 'create' || !ctx._source.containsKey('sequence_key')) {
  ctx._source = params.document;
} else {
  int order = params.document.sequence_key.compareTo(ctx._source.sequence_key);
  if (order > 0) {
    ctx._source = params.document;
  } else if (order == 0 && params.document.authority_digest != ctx._source.authority_digest) {
    throw new IllegalArgumentException('SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT');
  } else {
    ctx.op = 'noop';
  }
}`

````

## 第 3 步：Client.Apply

插入位置：第 2 步代码之后，紧接追加；完成稿约第 30 行。

Apply以确定Doc ID原子替换投影。成功或noop后重读实际ES水位，
SQL不能把这次请求的旧水位冒充已经存在的更高水位。

本步声明：`func (c *Client) Apply(ctx context.Context, physical string, value index.Authority) (index.State, error)`。

**这一段为什么这样写：** 跨ES/SQL恢复依赖的单调写入原语。

1. 验证投影
2. 脚本更新确定document ID
3. 重读实际ES状态
4. 拒绝水位回退或同序换摘要


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 父调用期限<br>在此声明中的实际使用：<code>c.request(ctx, &quot;POST&quot;, &quot;/&quot;+physical+&quot;/_update/&quot;+strconv.FormatUint(value.Document, 10)+&quot;?retry_on_conflict=3&quot;, map&#91;string&#93;any{ 		&quot;scripted_upsert&quot;: true, &quot;upsert&quot;: map&#91;string&#93;any{} …</code>（长表达式，按本步完整代码继续）；<code>c.getProjection(ctx, physical, value.Document, true)</code> |
| <code>physical</code> | <code>string</code> | 已核owner的物理索引名<br>在此声明中的实际使用：<code>physicalName.MatchString(physical)</code>；<code>&quot;/&quot;+physical</code>；<code>c.getProjection(ctx, physical, value.Document, true)</code> |
| <code>value</code> | <code>index.Authority</code> | Doc当前权威投影<br>在此声明中的实际使用：<code>project(value)</code>；<code>value.Document</code>；<code>value.Sequence</code>；<code>value.Digest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>index.State</code> | ES实际存储水位，可高于value.Sequence<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 未知/冲突错误；SQL不可先标成功<br>实际返回表达式：<code>errors.New(&quot;SEARCH_PHYSICAL_INDEX_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT&quot;)</code>；<code>errors.New(&quot;SEARCH_ES_APPLY_FAILED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(physical)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_PHYSICAL_INDEX_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>status != 200 &amp;&amp; status != 201</code>。
- 检查 <code>sameSequenceConflict(data)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, errors.New(&quot;SEARCH_ES_APPLY_FAILED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>result.Sequence &lt; value.Sequence &#124;&#124; (result.Sequence == value.Sequence &amp;&amp; result.Digest != value.Digest)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_APPLICATION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=809:2257 -->
````go
// Apply以确定Doc ID原子替换投影。成功或noop后重读实际ES水位，
// SQL不能把这次请求的旧水位冒充已经存在的更高水位。
func (c *Client) Apply(ctx context.Context, physical string, value index.Authority) (index.State, error) {
	var result index.State
	if !physicalName.MatchString(physical) {
		return result, errors.New("SEARCH_PHYSICAL_INDEX_INVALID")
	}
	projection, err := project(value)
	if err != nil {
		return result, err
	}
	status, data, err := c.request(ctx, "POST", "/"+physical+"/_update/"+strconv.FormatUint(value.Document, 10)+"?retry_on_conflict=3", map[string]any{
		"scripted_upsert": true, "upsert": map[string]any{}, "script": map[string]any{"lang": "painless", "source": applyScript,
			"params": map[string]any{"document": projection}},
	}, 1<<20)
	if err != nil {
		return result, err
	}
	if status != 200 && status != 201 {
		if sameSequenceConflict(data) {
			return result, errors.New("SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT")
		}
		return result, errors.New("SEARCH_ES_APPLY_FAILED")
	}
	stored, err := c.getProjection(ctx, physical, value.Document, true)
	if err != nil {
		return result, err
	}
	result, err = stored.state(value.Document)
	if err != nil {
		return result, err
	}
	if result.Sequence < value.Sequence || (result.Sequence == value.Sequence && result.Digest != value.Digest) {
		return result, errors.New("SEARCH_ES_APPLICATION_UNKNOWN")
	}
	return result, nil
}

````

## 第 4 步：sameSequenceConflict

插入位置：第 3 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`func sameSequenceConflict(raw []byte) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>json.Unmarshal(raw, &amp;response)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(raw, &amp;response) != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>node&#91;&quot;type&quot;&#93; == &quot;illegal_argument_exception&quot; &amp;&amp; node&#91;&quot;reason&quot;&#93; == &quot;SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT&quot;</code>。条件成立时执行 <code>return true</code>。当前函数在这里结束。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2257:2704 -->
````go
func sameSequenceConflict(raw []byte) bool {
	var response struct {
		Error map[string]any `json:"error"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return false
	}
	for node, count := response.Error, 0; node != nil && count < 8; count++ {
		if node["type"] == "illegal_argument_exception" && node["reason"] == "SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT" {
			return true
		}
		node, _ = node["caused_by"].(map[string]any)
	}
	return false
}

````

## 第 5 步：Client.getProjection

插入位置：第 4 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`func (c *Client) getProjection(ctx context.Context, physical string, document uint64, metadataOnly bool) (projection, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.request(ctx, &quot;GET&quot;, path, nil, maximum)</code> |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>physicalName.MatchString(physical)</code>；<code>&quot;/&quot; + physical</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>document == 0</code>；<code>strconv.FormatUint(document, 10)</code> |
| <code>metadataOnly</code> | <code>bool</code> | true只取文档、序号、正式版和权威摘要等元数据，响应上限1MiB；false取完整投影。元数据核验不能冒充正文已比对。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>projection</code> | 实际返回表达式：<code>result</code>；<code>response.Source</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_INDEX_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_ES_APPLICATION_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(physical) &#124;&#124; document == 0</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_INDEX_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>metadataOnly</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>status != 200 &#124;&#124; json.Unmarshal(data, &amp;response) != nil &#124;&#124; !response.Found</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_APPLICATION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return response.Source, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2704:3614 -->
````go
func (c *Client) getProjection(ctx context.Context, physical string, document uint64, metadataOnly bool) (projection, error) {
	var result projection
	if !physicalName.MatchString(physical) || document == 0 {
		return result, errors.New("SEARCH_INDEX_BINDING_INVALID")
	}
	path := "/" + physical + "/_doc/" + strconv.FormatUint(document, 10)
	maximum := int64(64 << 20)
	if metadataOnly {
		path += "?_source_includes=document_id,sequence,sequence_key,release_id,authority_digest,visible,visibility_policy"
		maximum = 1 << 20
	}
	status, data, err := c.request(ctx, "GET", path, nil, maximum)
	if err != nil {
		return result, err
	}
	var response struct {
		Found  bool       `json:"found"`
		Source projection `json:"_source"`
	}
	if status != 200 || json.Unmarshal(data, &response) != nil || !response.Found {
		return result, errors.New("SEARCH_ES_APPLICATION_UNKNOWN")
	}
	return response.Source, nil
}

````

## 第 6 步：Client.Get

插入位置：第 5 步代码之后，紧接追加；完成稿约第 109 行。

本步声明：`func (c *Client) Get(ctx context.Context, physical string, document uint64) (index.Authority, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.getProjection(ctx, physical, document, false)</code> |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>c.getProjection(ctx, physical, document, false)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>c.getProjection(ctx, physical, document, false)</code>；<code>stored.state(document)</code>；<code>Document: document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>index.Authority</code> | 使用 <code>index.Authority</code> 类型；其定义处的职责：Doc当前完整固定快照生成的索引权威。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_ES_CHAPTER_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_ES_TOMBSTONE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; unit == 0 &#124;&#124; strconv.FormatUint(unit, 10) != chapter.Unit</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_CHAPTER_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!result.Visible &amp;&amp; (result.Title != &quot;&quot; &#124;&#124; result.Text != &quot;&quot; &#124;&#124; len(result.Chapters) != 0)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_TOMBSTONE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3614:4830 -->
````go
func (c *Client) Get(ctx context.Context, physical string, document uint64) (index.Authority, error) {
	var result index.Authority
	stored, err := c.getProjection(ctx, physical, document, false)
	if err != nil {
		return result, err
	}
	state, err := stored.state(document)
	if err != nil {
		return result, err
	}
	result = index.Authority{Document: document, Sequence: state.Sequence, Release: state.Release, Digest: state.Digest,
		Visible: state.Visible, Policy: state.Policy, Title: stored.Title, Text: stored.Text, Chapters: []index.Chapter{}}
	for _, chapter := range stored.Chapters {
		unit, err := strconv.ParseUint(chapter.Unit, 10, 64)
		if err != nil || unit == 0 || strconv.FormatUint(unit, 10) != chapter.Unit {
			return result, errors.New("SEARCH_ES_CHAPTER_INVALID")
		}
		result.Chapters = append(result.Chapters, index.Chapter{Unit: unit, Title: chapter.Title, Text: chapter.Text})
	}
	if _, err = project(result); err != nil {
		return result, err
	}
	if !result.Visible && (result.Title != "" || result.Text != "" || len(result.Chapters) != 0) {
		return result, errors.New("SEARCH_ES_TOMBSTONE_INVALID")
	}
	return result, nil
}

// projection.go：保留原实现，按业务合并文件。

````

## 第 7 步：wireChapter

插入位置：第 6 步代码之后，紧接追加；完成稿约第 139 行。

本步声明：`type wireChapter struct {`。

**这一段为什么这样写：** ES内以字符串保留完整uint64单元身份。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Unit</code> | <code>string</code> | 规范十进制unit_id<br>序列化/校验标记：<code>`json:&quot;unit_id&quot;`</code> |
| <code>Title</code> | <code>string</code> | 章节标题文字<br>序列化/校验标记：<code>`json:&quot;title&quot;`</code> |
| <code>Text</code> | <code>string</code> | 章节可检索文字<br>序列化/校验标记：<code>`json:&quot;text&quot;`</code> |

<!-- write-step: 7 bytes=4830:4946 -->
````go
type wireChapter struct {
	Unit  string `json:"unit_id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}
````

## 第 8 步：projection

插入位置：第 7 步代码之后，紧接追加；完成稿约第 144 行。

本步声明：`type projection struct {`。

**这一段为什么这样写：** ES _source固定字段布局。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>string</code> | 十进制文档ID字符串<br>序列化/校验标记：<code>`json:&quot;document_id&quot;`</code> |
| <code>Sequence</code> | <code>string</code> | 实际聚合序号字符串<br>序列化/校验标记：<code>`json:&quot;sequence&quot;`</code> |
| <code>Watermark</code> | <code>string</code> | 20位补零序号，供脚本词典序比较<br>序列化/校验标记：<code>`json:&quot;sequence_key&quot;`</code> |
| <code>Release</code> | <code>*string</code> | 可空发布ID指针，隐藏为nil<br>序列化/校验标记：<code>`json:&quot;release_id&quot;`</code> |
| <code>Digest</code> | <code>string</code> | Authority摘要十六进制<br>序列化/校验标记：<code>`json:&quot;authority_digest&quot;`</code> |
| <code>Visible</code> | <code>bool</code> | 投影可见标记<br>序列化/校验标记：<code>`json:&quot;visible&quot;`</code> |
| <code>Policy</code> | <code>string</code> | 可见策略keyword<br>序列化/校验标记：<code>`json:&quot;visibility_policy&quot;`</code> |
| <code>Title</code> | <code>string</code> | 可见正式标题<br>序列化/校验标记：<code>`json:&quot;title&quot;`</code> |
| <code>Text</code> | <code>string</code> | 可见全篇纯文本<br>序列化/校验标记：<code>`json:&quot;text&quot;`</code> |
| <code>Chapters</code> | <code>&#91;&#93;wireChapter</code> | 可见章节定位文本数组<br>序列化/校验标记：<code>`json:&quot;chapters&quot;`</code> |

<!-- write-step: 8 bytes=4946:5422 -->
````go
type projection struct {
	Document  string        `json:"document_id"`
	Sequence  string        `json:"sequence"`
	Watermark string        `json:"sequence_key"`
	Release   *string       `json:"release_id"`
	Digest    string        `json:"authority_digest"`
	Visible   bool          `json:"visible"`
	Policy    string        `json:"visibility_policy"`
	Title     string        `json:"title"`
	Text      string        `json:"text"`
	Chapters  []wireChapter `json:"chapters"`
}

````

## 第 9 步：project

插入位置：第 8 步代码之后，紧接追加；完成稿约第 157 行。

本步声明：`func project(value index.Authority) (projection, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>index.Authority</code> | 使用 <code>index.Authority</code> 类型；其定义处的职责：Doc当前完整固定快照生成的索引权威。<br>在此声明中的实际使用：<code>value.Document</code>；<code>value.Title</code>；<code>value.Text</code>；<code>value.Chapters</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>projection</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_INDEX_PROJECTION_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_INDEX_PROJECTION_LIMIT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>value.Document == 0 &#124;&#124; !utf8.ValidString(value.Title) &#124;&#124; !utf8.ValidString(value.Text) &#124;&#124; 		len(value.Title) &gt; 1024 &#124;&#124; len(value.Text) &gt; 24&lt;&lt;20 &#124;&#124; len(value.Chapters) &gt; 100000</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_INDEX_PROJECTION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>value.Visible</code>。
- 检查 <code>value.Sequence == 0</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_INDEX_PROJECTION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>chapter.Unit == 0 &#124;&#124; seen&#91;chapter.Unit&#93; &#124;&#124; !utf8.ValidString(chapter.Title) &#124;&#124; !utf8.ValidString(chapter.Text) &#124;&#124; bytes &gt; 48&lt;&lt;20</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_INDEX_PROJECTION_LIMIT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5422:6937 -->
````go
func project(value index.Authority) (projection, error) {
	var result projection
	if value.Document == 0 || !utf8.ValidString(value.Title) || !utf8.ValidString(value.Text) ||
		len(value.Title) > 1024 || len(value.Text) > 24<<20 || len(value.Chapters) > 100000 {
		return result, errors.New("SEARCH_INDEX_PROJECTION_INVALID")
	}
	if _, err := index.ShouldApply(index.State{}, value); err != nil {
		return result, err
	}
	result = projection{Document: strconv.FormatUint(value.Document, 10), Sequence: strconv.FormatUint(value.Sequence, 10),
		Watermark: index.Watermark(value.Sequence), Digest: hex.EncodeToString(value.Digest[:]), Visible: value.Visible, Policy: value.Policy,
		Chapters: []wireChapter{}}
	if value.Visible {
		if value.Sequence == 0 {
			return result, errors.New("SEARCH_INDEX_PROJECTION_INVALID")
		}
		release := strconv.FormatUint(value.Release, 10)
		result.Release, result.Title, result.Text = &release, value.Title, value.Text
		seen := make(map[uint64]bool)
		bytes := len(value.Text)
		for _, chapter := range value.Chapters {
			bytes += len(chapter.Title) + len(chapter.Text)
			if chapter.Unit == 0 || seen[chapter.Unit] || !utf8.ValidString(chapter.Title) || !utf8.ValidString(chapter.Text) || bytes > 48<<20 {
				return result, errors.New("SEARCH_INDEX_PROJECTION_LIMIT")
			}
			seen[chapter.Unit] = true
			result.Chapters = append(result.Chapters, wireChapter{Unit: strconv.FormatUint(chapter.Unit, 10), Title: chapter.Title, Text: chapter.Text})
		}
	}
	return result, nil
}

````

## 第 10 步：projection.state

插入位置：第 9 步代码之后，紧接追加；完成稿约第 189 行。

本步声明：`func (p projection) state(document uint64) (index.State, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>strconv.FormatUint(document, 10)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>index.State</code> | 使用 <code>index.State</code> 类型；其定义处的职责：某物理索引对一个文档实际应用的水位。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ES_DOCUMENT_BINDING_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_ES_WATERMARK_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_ES_AUTHORITY_DIGEST_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_ES_RELEASE_INVALID&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>p.Document != strconv.FormatUint(document, 10)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_DOCUMENT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; strconv.FormatUint(sequence, 10) != p.Sequence &#124;&#124; p.Watermark != index.Watermark(sequence)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_WATERMARK_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(digest) != 32</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_AUTHORITY_DIGEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>p.Release != nil</code>。
- 检查 <code>err != nil &#124;&#124; result.Release == 0 &#124;&#124; strconv.FormatUint(result.Release, 10) != *p.Release</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_RELEASE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6937:8133 -->
````go
func (p projection) state(document uint64) (index.State, error) {
	var result index.State
	if p.Document != strconv.FormatUint(document, 10) {
		return result, errors.New("SEARCH_ES_DOCUMENT_BINDING_INVALID")
	}
	sequence, err := strconv.ParseUint(p.Sequence, 10, 64)
	if err != nil || strconv.FormatUint(sequence, 10) != p.Sequence || p.Watermark != index.Watermark(sequence) {
		return result, errors.New("SEARCH_ES_WATERMARK_INVALID")
	}
	digest, err := hex.DecodeString(p.Digest)
	if err != nil || len(digest) != 32 {
		return result, errors.New("SEARCH_ES_AUTHORITY_DIGEST_INVALID")
	}
	result.Sequence, result.Policy, result.Visible = sequence, p.Policy, p.Visible
	copy(result.Digest[:], digest)
	if p.Release != nil {
		result.Release, err = strconv.ParseUint(*p.Release, 10, 64)
		if err != nil || result.Release == 0 || strconv.FormatUint(result.Release, 10) != *p.Release {
			return result, errors.New("SEARCH_ES_RELEASE_INVALID")
		}
	}
	if _, err = index.ShouldApply(index.State{}, index.Authority{Sequence: result.Sequence, Release: result.Release, Visible: result.Visible, Policy: result.Policy, Digest: result.Digest}); err != nil {
		return result, err
	}
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/client/elastic/projection_client.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/01-event-authority.md)。
