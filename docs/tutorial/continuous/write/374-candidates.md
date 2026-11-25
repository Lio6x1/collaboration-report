# 分步编写 `services/search/internal/client/elastic/candidates.go`

先按 [S03 · 签名批锚点、PIT与当前权限二次过滤](../lessons/search/03-candidates-cursor.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/374-candidates.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

通过PIT和原search_after读取完整100候选批，并严格验证分片与探测项。

候选只是文档/发布/序号引用；Search不输出旧索引正文，Doc再核当前权限及正式版本。

写之前先按这条执行路线理解各段的连接：

1. OpenPIT以原Expires剩余时长申请，禁止部分分片；失败若已有合法新PIT仍交回关闭线索
2. Candidates按score desc/document asc/shard asc查询visible投影，phrase slop0匹配title/text/章节标题
3. 请求size+1并验证全部hits及第101项，才截为100和More
4. 响应需明确timed_out=false、无terminated_early且全部分片成功
5. ClosePIT仅明确succeeded和num_freed>=0才确认关闭

本文件需要守住的边界：

- keep_alive向下取毫秒，不可为凑1ms延长原期限
- 损坏第101项不能当has_more=true
- 搜索错误带的新PIT也要保存，不能继续用旧句柄

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/client/elastic' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/client/elastic/candidates.go') -and (Get-Item -LiteralPath 'services/search/internal/client/elastic/candidates.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/client/elastic/candidates.go')) { New-Item -ItemType File -Path 'services/search/internal/client/elastic/candidates.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:212 -->
````go
package elastic

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/search/internal/domain/cursor"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

````

## 第 2 步：candidateResponseLimit

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`const candidateResponseLimit = 24 << 20`。

<!-- write-step: 2 bytes=212:252 -->
````go
const candidateResponseLimit = 24 << 20
````

## 第 3 步：pitHandleLimit

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`const pitHandleLimit = 64 << 10`。

<!-- write-step: 3 bytes=252:285 -->
````go
const pitHandleLimit = 64 << 10

````

## 第 4 步：CandidateBatch

插入位置：第 3 步代码之后，紧接追加；完成稿约第 19 行。

CandidateBatch仅携带最小候选；PIT不进入HTTP游标。即使响应不完整，只要已读到
合法的新PIT，错误返回仍保留它作为关闭线索，不能由调用者继续旧句柄。

本步声明：`type CandidateBatch struct {`。

**这一段为什么这样写：** 一次完整ES查询结果及最新资源句柄。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>PIT</code> | <code>string</code> | 响应中的新句柄；即使结果失败也尽量保留供关闭 |
| <code>Candidates</code> | <code>&#91;&#93;*pb.SearchCandidate</code> | 最小引用候选，不含标题/摘要正文 |
| <code>End</code> | <code>json.RawMessage</code> | 最后候选的原sort，空批保留before |
| <code>More</code> | <code>bool</code> | 已验证探测候选证明还有后续 |

<!-- write-step: 4 bytes=285:624 -->
````go
// CandidateBatch仅携带最小候选；PIT不进入HTTP游标。即使响应不完整，只要已读到
// 合法的新PIT，错误返回仍保留它作为关闭线索，不能由调用者继续旧句柄。
type CandidateBatch struct {
	PIT        string
	Candidates []*pb.SearchCandidate
	End        json.RawMessage
	More       bool
}

````

## 第 5 步：pitShards

插入位置：第 4 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`type pitShards struct {`。

**这一段为什么这样写：** 必须显式存在的分片完整性证明。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Total</code> | <code>*int</code> | 本次目标总分片数，必须正数<br>序列化/校验标记：<code>`json:&quot;total&quot;`</code> |
| <code>Successful</code> | <code>*int</code> | 已成功分片数，必须等于Total且包含Skipped<br>序列化/校验标记：<code>`json:&quot;successful&quot;`</code> |
| <code>Skipped</code> | <code>*int</code> | 可省略；若存在须0..Successful<br>序列化/校验标记：<code>`json:&quot;skipped&quot;`</code> |
| <code>Failed</code> | <code>*int</code> | 必须显式为0<br>序列化/校验标记：<code>`json:&quot;failed&quot;`</code> |

<!-- write-step: 5 bytes=624:787 -->
````go
type pitShards struct {
	Total      *int `json:"total"`
	Successful *int `json:"successful"`
	Skipped    *int `json:"skipped"`
	Failed     *int `json:"failed"`
}

````

## 第 6 步：pitShards.complete

插入位置：第 5 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func (s *pitShards) complete() bool`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>s != nil &amp;&amp; s.Total != nil &amp;&amp; s.Successful != nil &amp;&amp; s.Failed != nil &amp;&amp; 		*s.Total &gt; 0 &amp;&amp; *s.Successful == *s.Total &amp;&amp; *s.Failed == 0 &amp;&amp; 		(s.Skipped == nil &#124;&#124; (*s.Skipped &gt;= 0 &amp;&amp; *s.Skipped &lt;= *s.Successful))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return s != nil &amp;&amp; s.Total != nil &amp;&amp; s.Successful != nil &amp;&amp; s.Failed != nil &amp;&amp; 		*s.Total &gt; 0 &amp;&amp; *s.Successful == *s.Total &amp;&amp; *s.Failed == 0 &amp;&amp; 		(s.Skipped == nil &#124;&#124; (*s.Skipped &gt;= 0 &amp;&amp; *s.Skipped &lt;= *s.Successful))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=787:1241 -->
````go
func (s *pitShards) complete() bool {
	// ES的successful包含skipped，Open/refresh可省略后者；不能把skipped再相加。
	// total/successful/failed仍必须同时存在，避免缺summary被零值误判成完整成功。
	return s != nil && s.Total != nil && s.Successful != nil && s.Failed != nil &&
		*s.Total > 0 && *s.Successful == *s.Total && *s.Failed == 0 &&
		(s.Skipped == nil || (*s.Skipped >= 0 && *s.Skipped <= *s.Successful))
}

````

## 第 7 步：validPIT

插入位置：第 6 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`func validPIT(pit string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pit</code> | <code>string</code> | 在此声明中的实际使用：<code>pit != &quot;&quot;</code>；<code>len(pit)</code>；<code>utf8.ValidString(pit)</code>；<code>strings.IndexFunc(pit, func(r rune) bool { return r &lt;= &#x27; &#x27; &#124;&#124; r == 127 })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>pit != &quot;&quot; &amp;&amp; len(pit) &lt;= pitHandleLimit &amp;&amp; utf8.ValidString(pit) &amp;&amp; strings.IndexFunc(pit, func(r rune) bool { return r &lt;= &#x27; &#x27; &#124;&#124; r == 127 }) &lt; 0</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return pit != &quot;&quot; &amp;&amp; len(pit) &lt;= pitHandleLimit &amp;&amp; utf8.ValidString(pit) &amp;&amp; strings.IndexFunc(pit, func(r rune) bool { return r &lt;= &#x27; &#x27; &#124;&#124; r == 127 }) &lt; 0</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return r &lt;= &#x27; &#x27; &#124;&#124; r == 127</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1241:1431 -->
````go
func validPIT(pit string) bool {
	return pit != "" && len(pit) <= pitHandleLimit && utf8.ValidString(pit) && strings.IndexFunc(pit, func(r rune) bool { return r <= ' ' || r == 127 }) < 0
}

````

## 第 8 步：pitRemaining

插入位置：第 7 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`func pitRemaining(remaining time.Duration) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>remaining</code> | <code>time.Duration</code> | 一段持续时间，按 time.Second 等单位运算；与表示某个瞬间的 time.Time 区别开。<br>在此声明中的实际使用：<code>remaining &lt; time.Millisecond</code>；<code>remaining.Milliseconds</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>strconv.FormatInt(remaining.Milliseconds(), 10) + &quot;ms&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_EXPIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>remaining &lt; time.Millisecond</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;SEARCH_CURSOR_EXPIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return strconv.FormatInt(remaining.Milliseconds(), 10) + &quot;ms&quot;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1431:1770 -->
````go
func pitRemaining(remaining time.Duration) (string, error) {
	// 向下取整，绝不为了凑成1ms而把原绝对到期向后推。业务层每次从原expires计算remaining。
	if remaining < time.Millisecond {
		return "", errors.New("SEARCH_CURSOR_EXPIRED")
	}
	return strconv.FormatInt(remaining.Milliseconds(), 10) + "ms", nil
}

````

## 第 9 步：Client.OpenPIT

插入位置：第 8 步代码之后，紧接追加；完成稿约第 55 行。

本步声明：`func (c *Client) OpenPIT(ctx context.Context, physical string, remaining time.Duration) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(ctx, remaining)</code> |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>physicalName.MatchString(physical)</code>；<code>&quot;/&quot;+physical</code> |
| <code>remaining</code> | <code>time.Duration</code> | 一段持续时间，按 time.Second 等单位运算；与表示某个瞬间的 time.Time 区别开。<br>在此声明中的实际使用：<code>pitRemaining(remaining)</code>；<code>context.WithTimeout(ctx, remaining)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>response.ID</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_PHYSICAL_INDEX_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_ES_PIT_OPEN_UNKNOWN&quot;)</code>；<code>errors.New(&quot;SEARCH_ES_PIT_OPEN_INCOMPLETE&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(physical)</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;SEARCH_PHYSICAL_INDEX_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, remaining)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>status != 200 &#124;&#124; json.Unmarshal(data, &amp;response) != nil &#124;&#124; !validPIT(response.ID)</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;SEARCH_ES_PIT_OPEN_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!response.Shards.complete() &#124;&#124; requestCtx.Err() != nil</code>。条件成立时执行 <code>return response.ID, errors.New(&quot;SEARCH_ES_PIT_OPEN_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return response.ID, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=1770:2748 -->
````go
func (c *Client) OpenPIT(ctx context.Context, physical string, remaining time.Duration) (string, error) {
	if !physicalName.MatchString(physical) {
		return "", errors.New("SEARCH_PHYSICAL_INDEX_INVALID")
	}
	keepAlive, err := pitRemaining(remaining)
	if err != nil {
		return "", err
	}
	requestCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	status, data, err := c.request(requestCtx, "POST", "/"+physical+"/_pit?keep_alive="+url.QueryEscape(keepAlive)+"&allow_partial_search_results=false", nil, candidateResponseLimit)
	if err != nil {
		return "", err
	}
	var response struct {
		ID     string     `json:"id"`
		Shards *pitShards `json:"_shards"`
	}
	if status != 200 || json.Unmarshal(data, &response) != nil || !validPIT(response.ID) {
		return "", errors.New("SEARCH_ES_PIT_OPEN_UNKNOWN")
	}
	if !response.Shards.complete() || requestCtx.Err() != nil {
		return response.ID, errors.New("SEARCH_ES_PIT_OPEN_INCOMPLETE")
	}
	return response.ID, nil
}

````

## 第 10 步：candidateHit

插入位置：第 9 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`type candidateHit struct {`。

**这一段为什么这样写：** ES最小候选hit的传输结构。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Physical</code> | <code>string</code> | _index，必须与链绑定物理索引一致<br>序列化/校验标记：<code>`json:&quot;_index&quot;`</code> |
| <code>ID</code> | <code>string</code> | _id，必须与_source.document_id相同<br>序列化/校验标记：<code>`json:&quot;_id&quot;`</code> |
| <code>Source</code> | <code>struct { 		Document string `json:&quot;document_id&quot;` 		Release  string `json:&quot;release_id&quot;` 		Sequence string `json:&quot;sequence&quot;` 	}</code> | document_id/release_id/sequence三个规范十进制字符串<br>序列化/校验标记：<code>`json:&quot;_source&quot;`</code> |
| <code>Sort</code> | <code>json.RawMessage</code> | 原score/Doc/shard三元组JSON<br>序列化/校验标记：<code>`json:&quot;sort&quot;`</code> |

<!-- write-step: 10 bytes=2748:3029 -->
````go
type candidateHit struct {
	Physical string `json:"_index"`
	ID       string `json:"_id"`
	Source   struct {
		Document string `json:"document_id"`
		Release  string `json:"release_id"`
		Sequence string `json:"sequence"`
	} `json:"_source"`
	Sort json.RawMessage `json:"sort"`
}

````

## 第 11 步：candidateUnsigned

插入位置：第 10 步代码之后，紧接追加；完成稿约第 93 行。

本步声明：`func candidateUnsigned(value string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>strconv.ParseUint(value, 10, 64)</code>；<code>strconv.FormatUint(parsed, 10) != value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>parsed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ES_CANDIDATE_BINDING_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; parsed == 0 &#124;&#124; strconv.FormatUint(parsed, 10) != value</code>。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_ES_CANDIDATE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return parsed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3029:3296 -->
````go
func candidateUnsigned(value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 || strconv.FormatUint(parsed, 10) != value {
		return 0, errors.New("SEARCH_ES_CANDIDATE_BINDING_INVALID")
	}
	return parsed, nil
}

````

## 第 12 步：candidateHit.candidate

插入位置：第 11 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`func (hit candidateHit) candidate() (*pb.SearchCandidate, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.SearchCandidate</code> | 实际返回表达式：<code>nil</code>；<code>&amp;pb.SearchCandidate{DocumentId: document, ReleaseId: release, AggregateSeq: sequence, 		PhysicalIndex: hit.Physical, SortJsonUtf8: append(&#91;&#93;byte(nil), hit.Sort...)}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ES_CANDIDATE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(hit.Physical) &#124;&#124; hit.ID != hit.Source.Document &#124;&#124; cursor.ValidateSort(hit.Sort) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_ES_CANDIDATE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(hit.Sort, &amp;sort) != nil &#124;&#124; json.Unmarshal(sort&#91;1&#93;, &amp;sortDocument) != nil &#124;&#124; sortDocument != hit.Source.Document</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_ES_CANDIDATE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;pb.SearchCandidate{DocumentId: document, ReleaseId: release, AggregateSeq: sequence, 		PhysicalIndex: hit.Physical, SortJsonUtf8: append(&#91;&#93;byte(nil), hit.Sort...)}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3296:4266 -->
````go
func (hit candidateHit) candidate() (*pb.SearchCandidate, error) {
	if !physicalName.MatchString(hit.Physical) || hit.ID != hit.Source.Document || cursor.ValidateSort(hit.Sort) != nil {
		return nil, errors.New("SEARCH_ES_CANDIDATE_BINDING_INVALID")
	}
	var sort []json.RawMessage
	var sortDocument string
	if json.Unmarshal(hit.Sort, &sort) != nil || json.Unmarshal(sort[1], &sortDocument) != nil || sortDocument != hit.Source.Document {
		return nil, errors.New("SEARCH_ES_CANDIDATE_BINDING_INVALID")
	}
	document, err := candidateUnsigned(hit.Source.Document)
	if err != nil {
		return nil, err
	}
	release, err := candidateUnsigned(hit.Source.Release)
	if err != nil {
		return nil, err
	}
	sequence, err := candidateUnsigned(hit.Source.Sequence)
	if err != nil {
		return nil, err
	}
	return &pb.SearchCandidate{DocumentId: document, ReleaseId: release, AggregateSeq: sequence,
		PhysicalIndex: hit.Physical, SortJsonUtf8: append([]byte(nil), hit.Sort...)}, nil
}

````

## 第 13 步：Client.Candidates

插入位置：第 12 步代码之后，紧接追加；完成稿约第 126 行。

本步声明：`func (c *Client) Candidates(ctx context.Context, pit, query string, after json.RawMessage, size int, remaining time.Duration) (CandidateBatch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(ctx, remaining)</code> |
| <code>pit</code> | <code>string</code> | 在此声明中的实际使用：<code>validPIT(pit)</code>；<code>&quot;id&quot;: pit</code> |
| <code>query</code> | <code>string</code> | 在此声明中的实际使用：<code>utf8.ValidString(query)</code>；<code>strings.TrimSpace(query)</code>；<code>utf8.RuneCountInString(query)</code>；<code>&quot;query&quot;: query</code> |
| <code>after</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>在此声明中的实际使用：<code>len(after)</code>；<code>cursor.ValidateSort(after)</code>；<code>request&#91;&quot;search_after&quot;&#93; = after</code>；<code>append(json.RawMessage(nil), after...)</code> |
| <code>size</code> | <code>int</code> | 在此声明中的实际使用：<code>size &lt; 1</code>；<code>size &gt; 100</code>；<code>size + 1</code>；<code>size+1</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>remaining</code> | <code>time.Duration</code> | 一段持续时间，按 time.Second 等单位运算；与表示某个瞬间的 time.Time 区别开。<br>在此声明中的实际使用：<code>pitRemaining(remaining)</code>；<code>context.WithTimeout(ctx, remaining)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>CandidateBatch</code> | 使用 <code>CandidateBatch</code> 类型；其定义处的职责：一次完整ES查询结果及最新资源句柄。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ES_CANDIDATE_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_ES_CANDIDATES_INCOMPLETE&quot;)</code>；<code>errors.New(&quot;SEARCH_ES_CANDIDATE_BINDING_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!validPIT(pit) &#124;&#124; !utf8.ValidString(query) &#124;&#124; strings.TrimSpace(query) == &quot;&quot; &#124;&#124; utf8.RuneCountInString(query) &gt; 500 &#124;&#124; 		size &lt; 1 &#124;&#124; size &gt; 100 &#124;&#124; (len(after) != 0 &amp;&amp; cursor.ValidateSort(after) != nil)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_CANDIDATE_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>len(after) != 0</code>。
- 执行 <code>context.WithTimeout(ctx, remaining)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>status != 200 &#124;&#124; json.Unmarshal(data, &amp;response) != nil &#124;&#124; !validPIT(response.PIT)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_CANDIDATES_INCOMPLETE&quot;)</code>。当前函数在这里结束。

<!-- write-step: 13 bytes=4266:6513 -->
````go
func (c *Client) Candidates(ctx context.Context, pit, query string, after json.RawMessage, size int, remaining time.Duration) (CandidateBatch, error) {
	var result CandidateBatch
	if !validPIT(pit) || !utf8.ValidString(query) || strings.TrimSpace(query) == "" || utf8.RuneCountInString(query) > 500 ||
		size < 1 || size > 100 || (len(after) != 0 && cursor.ValidateSort(after) != nil) {
		return result, errors.New("SEARCH_ES_CANDIDATE_REQUEST_INVALID")
	}
	keepAlive, err := pitRemaining(remaining)
	if err != nil {
		return result, err
	}
	request := map[string]any{
		"size": size + 1, "track_total_hits": false,
		"_source": []string{"document_id", "release_id", "sequence"},
		"pit":     map[string]any{"id": pit, "keep_alive": keepAlive},
		"sort":    []any{map[string]string{"_score": "desc"}, map[string]string{"document_id": "asc"}, map[string]string{"_shard_doc": "asc"}},
		"query": map[string]any{"bool": map[string]any{
			"filter": []any{map[string]any{"term": map[string]any{"visible": true}}},
			// 中文标准分词会拆出单字；默认OR会把“设定”误匹配到只有“固定”的正文。
			// 要求同一字段中连续的词序，复用原索引，不以标题去重掩盖独立文档。
			"must": []any{map[string]any{"multi_match": map[string]any{
				"query": query, "type": "phrase", "slop": 0, "fields": []string{"title", "text", "chapters.title"},
			}}},
		}},
	}
	if len(after) != 0 {
		// RawMessage只保留位置的原JSON词法，不能经float64重新编码高位shard序号。
		request["search_after"] = after
	}
	requestCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	status, data, err := c.request(requestCtx, "POST", "/_search?allow_partial_search_results=false", request, candidateResponseLimit)
	if err != nil {
		return result, err
	}
	var response struct {
		PIT        string     `json:"pit_id"`
		TimedOut   *bool      `json:"timed_out"`
		Terminated bool       `json:"terminated_early"`
		Shards     *pitShards `json:"_shards"`
		Hits       *struct {
			Hits json.RawMessage `json:"hits"`
		} `json:"hits"`
	}
	if status != 200 || json.Unmarshal(data, &response) != nil || !validPIT(response.PIT) {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
````

## 第 14 步：Client.Candidates（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 172 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>response.TimedOut == nil &#124;&#124; *response.TimedOut &#124;&#124; response.Terminated &#124;&#124; !response.Shards.complete() &#124;&#124; 		response.Hits == nil &#124;&#124; requestCtx.Err() != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_CANDIDATES_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(response.Hits.Hits, &amp;hits) != nil &#124;&#124; hits == nil &#124;&#124; len(hits) &gt; size+1</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_CANDIDATES_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; seen&#91;candidate.GetDocumentId()&#93; &#124;&#124; (physical != &quot;&quot; &amp;&amp; physical != hit.Physical)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_CANDIDATE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>requestCtx.Err() != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ES_CANDIDATES_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>result.More</code>。
- 检查 <code>len(candidates) != 0</code>。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6513:7939 -->
````go
	result.PIT = response.PIT
	if response.TimedOut == nil || *response.TimedOut || response.Terminated || !response.Shards.complete() ||
		response.Hits == nil || requestCtx.Err() != nil {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
	var hits []candidateHit
	if json.Unmarshal(response.Hits.Hits, &hits) != nil || hits == nil || len(hits) > size+1 {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
	// 先验证整个响应，包括第B+1项，之后才能交付批次；损坏probe不能伪造成has_more。
	candidates := make([]*pb.SearchCandidate, 0, len(hits))
	seen := make(map[uint64]bool, len(hits))
	physical := ""
	for _, hit := range hits {
		candidate, err := hit.candidate()
		if err != nil || seen[candidate.GetDocumentId()] || (physical != "" && physical != hit.Physical) {
			return result, errors.New("SEARCH_ES_CANDIDATE_BINDING_INVALID")
		}
		physical = hit.Physical
		seen[candidate.DocumentId] = true
		candidates = append(candidates, candidate)
	}
	if requestCtx.Err() != nil {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
	result.More = len(candidates) > size
	if result.More {
		candidates = candidates[:size]
	}
	result.Candidates = candidates
	result.End = append(json.RawMessage(nil), after...)
	if len(candidates) != 0 {
		result.End = append(json.RawMessage(nil), candidates[len(candidates)-1].SortJsonUtf8...)
	}
	return result, nil
}

````

## 第 15 步：Client.ClosePIT

插入位置：第 14 步代码之后，紧接追加；完成稿约第 209 行。

本步声明：`func (c *Client) ClosePIT(ctx context.Context, pit string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.request(ctx, &quot;DELETE&quot;, &quot;/_pit&quot;, map&#91;string&#93;any{&quot;id&quot;: pit}, candidateResponseLimit)</code>；<code>ctx.Err</code> |
| <code>pit</code> | <code>string</code> | 在此声明中的实际使用：<code>validPIT(pit)</code>；<code>&quot;id&quot;: pit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ES_PIT_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_ES_PIT_CLOSE_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!validPIT(pit)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ES_PIT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>status != 200 &#124;&#124; json.Unmarshal(data, &amp;response) != nil &#124;&#124; !response.Succeeded &#124;&#124; response.Freed == nil &#124;&#124; *response.Freed &lt; 0 &#124;&#124; ctx.Err() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ES_PIT_CLOSE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7939:8636 -->
````go
func (c *Client) ClosePIT(ctx context.Context, pit string) error {
	if !validPIT(pit) {
		return errors.New("SEARCH_ES_PIT_INVALID")
	}
	status, data, err := c.request(ctx, "DELETE", "/_pit", map[string]any{"id": pit}, candidateResponseLimit)
	if err != nil {
		return err
	}
	var response struct {
		Succeeded bool `json:"succeeded"`
		Freed     *int `json:"num_freed"`
	}
	// num_freed=0也可能是原PIT已自然释放；仅ES明确succeeded才确认该句柄已关闭。
	if status != 200 || json.Unmarshal(data, &response) != nil || !response.Succeeded || response.Freed == nil || *response.Freed < 0 || ctx.Err() != nil {
		return errors.New("SEARCH_ES_PIT_CLOSE_UNKNOWN")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/client/elastic/candidates.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/03-candidates-cursor.md)。
