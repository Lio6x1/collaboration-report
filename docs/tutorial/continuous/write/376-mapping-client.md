# 分步编写 `services/search/internal/client/elastic/mapping_client.go`

先按 [S01 · Canal事件与Doc当前正式投影](../lessons/search/01-event-authority.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/376-mapping-client.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

创建或核验本owner严格索引mapping，支持安全清理和完整计数核验。

同名索引不一定属于本项目；重建不能覆盖未知索引，也不能把部分分片统计当完整。

写之前先按这条执行路线理解各段的连接：

1. properties把uint64字段映射keyword，chapters用object而非nested
2. verifyMapping严格比application/owner/schema/dynamic和全部字段
3. EnsureIndex先查再创建，创建不确定后重读同名归属
4. DeleteOwnedIndex只有已核owned且无alias才删除
5. RefreshCount先_refresh再_count，两步都要求所有分片成功

本文件需要守住的边界：

- 不能超时后删索引重建
- 隐藏墓碑也占一个ES文档，count核验必须包含
- 修改mapping版本需明确迁移，不可宽松接受旧schema

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/client/elastic' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/client/elastic/mapping_client.go') -and (Get-Item -LiteralPath 'services/search/internal/client/elastic/mapping_client.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/client/elastic/mapping_client.go')) { New-Item -ItemType File -Path 'services/search/internal/client/elastic/mapping_client.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:234 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
)

// schema.go：保留原实现，按业务合并文件。

````

## 第 2 步：schemaVersion

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`const schemaVersion = "publication-index-v1-uint64"`。

<!-- write-step: 2 bytes=234:287 -->
````go
const schemaVersion = "publication-index-v1-uint64"

````

## 第 3 步：properties

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`func properties() map[string]any`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>map&#91;string&#93;any{ 		&quot;document_id&quot;: keyword(), &quot;sequence&quot;: keyword(), &quot;sequence_key&quot;: keyword(), &quot;release_id&quot;: keyword(), 		&quot;authority_digest&quot;: keyword(), &quot;visible&quot;: map&#91;string&#93;any{&quot;t …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return map&#91;string&#93;any{&quot;type&quot;: &quot;keyword&quot;}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return map&#91;string&#93;any{&quot;type&quot;: &quot;text&quot;}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return map&#91;string&#93;any{ 		&quot;document_id&quot;: keyword(), &quot;sequence&quot;: keyword(), &quot;sequence_key&quot;: keyword(), &quot;release_id&quot;: keyword(), 		&quot;authority_digest&quot;: keyword(), &quot;visible&quot;: map&#91;string …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=287:976 -->
````go
func properties() map[string]any {
	keyword := func() any { return map[string]any{"type": "keyword"} }
	text := func() any { return map[string]any{"type": "text"} }
	return map[string]any{
		"document_id": keyword(), "sequence": keyword(), "sequence_key": keyword(), "release_id": keyword(),
		"authority_digest": keyword(), "visible": map[string]any{"type": "boolean"}, "visibility_policy": keyword(),
		// 查询使用完整父文档text，章节信息供定位；object不引入nested每文档对象数的额外低上限。
		"title": text(), "text": text(), "chapters": map[string]any{"properties": map[string]any{
			"unit_id": keyword(), "title": text(), "text": text(),
		}},
	}
}

````

## 第 4 步：Client.verifyMapping

插入位置：第 3 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func (c *Client) verifyMapping(ctx context.Context, physical string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.request(ctx, &quot;GET&quot;, &quot;/&quot;+physical+&quot;/_mapping&quot;, nil, 1&lt;&lt;20)</code> |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;/&quot;+physical</code>；<code>actual, exists := response&#91;physical&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>errors.New(&quot;SEARCH_ES_MAPPING_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_INDEX_SCHEMA_MISMATCH&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>status == 404</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>status != 200</code>。条件成立时执行 <code>return false, errors.New(&quot;SEARCH_ES_MAPPING_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(data, &amp;response) != nil &#124;&#124; len(response) != 1</code>。条件成立时执行 <code>return false, errors.New(&quot;SEARCH_INDEX_SCHEMA_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>!exists &#124;&#124; actual.Mappings.Dynamic != &quot;strict&quot; &#124;&#124; actual.Mappings.Meta.Application != &quot;collaboration-report&quot; &#124;&#124; 		actual.Mappings.Meta.Owner != c.owner &#124;&#124; actual.Mappings.Meta.Sche …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false, errors.New(&quot;SEARCH_INDEX_SCHEMA_MISMATCH&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=976:2153 -->
````go
func (c *Client) verifyMapping(ctx context.Context, physical string) (bool, error) {
	status, data, err := c.request(ctx, "GET", "/"+physical+"/_mapping", nil, 1<<20)
	if err != nil {
		return false, err
	}
	if status == 404 {
		return false, nil
	}
	if status != 200 {
		return false, errors.New("SEARCH_ES_MAPPING_UNAVAILABLE")
	}
	var response map[string]struct {
		Mappings struct {
			Dynamic string `json:"dynamic"`
			Meta    struct {
				Application string `json:"application"`
				Owner       string `json:"owner"`
				Schema      string `json:"schema"`
			} `json:"_meta"`
			Properties map[string]any `json:"properties"`
		} `json:"mappings"`
	}
	if json.Unmarshal(data, &response) != nil || len(response) != 1 {
		return false, errors.New("SEARCH_INDEX_SCHEMA_MISMATCH")
	}
	actual, exists := response[physical]
	if !exists || actual.Mappings.Dynamic != "strict" || actual.Mappings.Meta.Application != "collaboration-report" ||
		actual.Mappings.Meta.Owner != c.owner || actual.Mappings.Meta.Schema != schemaVersion || !reflect.DeepEqual(actual.Mappings.Properties, properties()) {
		return false, errors.New("SEARCH_INDEX_SCHEMA_MISMATCH")
	}
	return true, nil
}

````

## 第 5 步：Client.EnsureIndex

插入位置：第 4 步代码之后，紧接追加；完成稿约第 61 行。

EnsureIndex不接管同名未知索引；超时后只能重读原名称/owner/schema，不能删除重建。

本步声明：`func (c *Client) EnsureIndex(ctx context.Context, physical string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.verifyMapping(ctx, physical)</code>；<code>c.request(ctx, &quot;PUT&quot;, &quot;/&quot;+physical, map&#91;string&#93;any{ 		&quot;settings&quot;: map&#91;string&#93;any{&quot;number_of_shards&quot;: 1, &quot;number_of_replicas&quot;: 0}, 		&quot;mappings&quot;: map&#91;string&#93;any{&quot;dynamic&quot;: &quot;strict&quot;,  …</code>（长表达式，按本步完整代码继续） |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>physicalName.MatchString(physical)</code>；<code>c.verifyMapping(ctx, physical)</code>；<code>&quot;/&quot;+physical</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_PHYSICAL_INDEX_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_INDEX_CREATE_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(physical)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_PHYSICAL_INDEX_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; exists</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>status != 200 &amp;&amp; status != 400</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_INDEX_CREATE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!exists</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_INDEX_CREATE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2153:3168 -->
````go
// EnsureIndex不接管同名未知索引；超时后只能重读原名称/owner/schema，不能删除重建。
func (c *Client) EnsureIndex(ctx context.Context, physical string) error {
	if !physicalName.MatchString(physical) {
		return errors.New("SEARCH_PHYSICAL_INDEX_INVALID")
	}
	exists, err := c.verifyMapping(ctx, physical)
	if err != nil || exists {
		return err
	}
	status, _, err := c.request(ctx, "PUT", "/"+physical, map[string]any{
		"settings": map[string]any{"number_of_shards": 1, "number_of_replicas": 0},
		"mappings": map[string]any{"dynamic": "strict", "_meta": map[string]any{
			"application": "collaboration-report", "owner": c.owner, "schema": schemaVersion,
		}, "properties": properties()},
	}, 1<<20)
	if err != nil {
		return err
	}
	if status != 200 && status != 400 {
		return errors.New("SEARCH_INDEX_CREATE_UNKNOWN")
	}
	exists, err = c.verifyMapping(ctx, physical)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("SEARCH_INDEX_CREATE_UNKNOWN")
	}
	return nil
}

````

## 第 6 步：Client.DeleteOwnedIndex

插入位置：第 5 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`func (c *Client) DeleteOwnedIndex(ctx context.Context, physical string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.verifyMapping(ctx, physical)</code>；<code>c.request(ctx, &quot;GET&quot;, &quot;/&quot;+physical+&quot;/_alias&quot;, nil, 1&lt;&lt;20)</code>；<code>c.request(ctx, &quot;DELETE&quot;, &quot;/&quot;+physical, nil, 1&lt;&lt;20)</code> |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>physicalName.MatchString(physical)</code>；<code>c.verifyMapping(ctx, physical)</code>；<code>&quot;/&quot;+physical</code>；<code>aliases&#91;physical&#93;.Aliases</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_PHYSICAL_INDEX_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_INDEX_STILL_ALIASED&quot;)</code>；<code>errors.New(&quot;SEARCH_INDEX_DELETE_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(physical)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_PHYSICAL_INDEX_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !exists</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; status != 200 &#124;&#124; json.Unmarshal(data, &amp;aliases) != nil &#124;&#124; len(aliases) != 1 &#124;&#124; len(aliases&#91;physical&#93;.Aliases) != 0</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_INDEX_STILL_ALIASED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>status != 200</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_INDEX_DELETE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3168:4037 -->
````go
func (c *Client) DeleteOwnedIndex(ctx context.Context, physical string) error {
	if !physicalName.MatchString(physical) {
		return errors.New("SEARCH_PHYSICAL_INDEX_INVALID")
	}
	exists, err := c.verifyMapping(ctx, physical)
	if err != nil || !exists {
		return err
	}
	status, data, err := c.request(ctx, "GET", "/"+physical+"/_alias", nil, 1<<20)
	var aliases map[string]struct {
		Aliases map[string]any `json:"aliases"`
	}
	if err != nil || status != 200 || json.Unmarshal(data, &aliases) != nil || len(aliases) != 1 || len(aliases[physical].Aliases) != 0 {
		return errors.New("SEARCH_INDEX_STILL_ALIASED")
	}
	status, _, err = c.request(ctx, "DELETE", "/"+physical, nil, 1<<20)
	if err != nil {
		return err
	}
	if status != 200 {
		return errors.New("SEARCH_INDEX_DELETE_UNKNOWN")
	}
	return nil
}

// validation.go：保留原实现，按业务合并文件。

````

## 第 7 步：Client.RefreshCount

插入位置：第 6 步代码之后，紧接追加；完成稿约第 119 行。

RefreshCount用于写屏障后的完整目标核验；部分分片成功不能作为完整统计。

本步声明：`func (c *Client) RefreshCount(ctx context.Context, physical string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.request(ctx, method, path, nil, 1&lt;&lt;20)</code> |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>physicalName.MatchString(physical)</code>；<code>&quot;/&quot; + physical</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>count</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_PHYSICAL_INDEX_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_SHARDS_INCOMPLETE&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_COUNT_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(physical)</code>。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_PHYSICAL_INDEX_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>method == &quot;GET&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 检查 <code>status != 200 &#124;&#124; json.Unmarshal(raw, &amp;reply) != nil &#124;&#124; reply.Shards.Total == nil &#124;&#124; reply.Shards.Successful == nil &#124;&#124; reply.Shards.Failed == nil &#124;&#124; *reply.Shards.Total &lt;= 0 &#124;&#124; *rep …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_REBUILD_SHARDS_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>method == &quot;GET&quot;</code>。
- 检查 <code>reply.Count == nil</code>。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_REBUILD_COUNT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return count, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4037:5247 -->
````go
// RefreshCount用于写屏障后的完整目标核验；部分分片成功不能作为完整统计。
func (c *Client) RefreshCount(ctx context.Context, physical string) (uint64, error) {
	if !physicalName.MatchString(physical) {
		return 0, errors.New("SEARCH_PHYSICAL_INDEX_INVALID")
	}
	var count uint64
	for _, method := range []string{"POST", "GET"} {
		path := "/" + physical + "/_refresh"
		if method == "GET" {
			path = "/" + physical + "/_count"
		}
		status, raw, err := c.request(ctx, method, path, nil, 1<<20)
		if err != nil {
			return 0, err
		}
		var reply struct {
			Count  *uint64                                  `json:"count"`
			Shards struct{ Total, Successful, Failed *int } `json:"_shards"`
		}
		if status != 200 || json.Unmarshal(raw, &reply) != nil || reply.Shards.Total == nil || reply.Shards.Successful == nil || reply.Shards.Failed == nil || *reply.Shards.Total <= 0 || *reply.Shards.Failed != 0 || *reply.Shards.Successful != *reply.Shards.Total {
			return 0, errors.New("SEARCH_REBUILD_SHARDS_INCOMPLETE")
		}
		if method == "GET" {
			if reply.Count == nil {
				return 0, errors.New("SEARCH_REBUILD_COUNT_INVALID")
			}
			count = *reply.Count
		}
	}
	return count, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/client/elastic/mapping_client.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/01-event-authority.md)。
