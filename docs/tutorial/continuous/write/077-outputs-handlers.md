# 分步编写 `services/account/internal/handler/account/outputs_handlers.go`

先按 [A08 · 输出保护与HTTP、RPC、worker完整装配](../lessons/account/08-output-and-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/077-outputs-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

缓冲Account业务响应，捕获来源并在实际输出前登记控制关系。

撤权需要能找到正在向浏览器输出的真实Edge handler；私有直连没有这项资格。

写之前先按这条执行路线理解各段的连接：

1. Protect创建OutputBinding放入请求context，用ResponseCapture收集完整短JSON。
2. 非匿名成功业务输出必须有X-Edge-Handler、非空且一致的来源，并且不超过2MiB；Register成功才CopyTo/Flush。
3. CaptureSessionSources记录identity、credential和session三个独立scope。

本文件需要守住的边界：

- 只给错误或精确最小回执有限豁免；分享链接和目录不能伪装成原操作状态。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/account/outputs_handlers.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/account/outputs_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/account/outputs_handlers.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/account/outputs_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:459 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	"collaboration/platform"
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// output.go：保留原实现，按业务合并文件。

````

## 第 2 步：Outputs

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`type Outputs struct{ logic *logic.Outputs }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>logic</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：核对响应捕获的全部来源，再到Doc注册该入口handler的输出。 |

<!-- write-step: 2 bytes=459:504 -->
````go
type Outputs struct{ logic *logic.Outputs }

````

## 第 3 步：NewOutputs

插入位置：第 2 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`func NewOutputs(service *logic.Outputs) *Outputs`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>service</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：核对响应捕获的全部来源，再到Doc注册该入口handler的输出。<br>在此声明中的实际使用：<code>logic: service</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Outputs</code> | 实际返回表达式：<code>&amp;Outputs{logic: service}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Outputs{logic: service}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=504:590 -->
````go
func NewOutputs(service *logic.Outputs) *Outputs { return &Outputs{logic: service} }

````

## 第 4 步：MinimalReceipt

插入位置：第 3 步代码之后，紧接追加；完成稿约第 24 行。

MinimalReceipt只放行原操作的最小结果，不能借操作号直接返回分享链接或敏感投影。

本步声明：`func MinimalReceipt(raw []byte) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>json.Unmarshal(raw, &amp;value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(raw, &amp;value) != nil &#124;&#124; len(value) &lt; 2 &#124;&#124; len(value) &gt; 4 &#124;&#124; value&#91;&quot;operation_id&quot;&#93; == nil &#124;&#124; value&#91;&quot;state&quot;&#93; == nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>key != &quot;operation_id&quot; &amp;&amp; key != &quot;state&quot; &amp;&amp; key != &quot;poll_url&quot; &amp;&amp; key != &quot;result&quot;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(value&#91;&quot;operation_id&quot;&#93;, &amp;id) != nil &#124;&#124; !domain.ValidOperationID(id) &#124;&#124; json.Unmarshal(value&#91;&quot;state&quot;&#93;, &amp;state) != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>result != nil</code>。
- 检查 <code>json.Unmarshal(result, &amp;fields) != nil &#124;&#124; len(fields) != 1 &#124;&#124; !domain.ValidEntityID(fields&#91;&quot;entity_id&quot;&#93;)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=590:1587 -->
````go
// MinimalReceipt只放行原操作的最小结果，不能借操作号直接返回分享链接或敏感投影。
func MinimalReceipt(raw []byte) bool {
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil || len(value) < 2 || len(value) > 4 || value["operation_id"] == nil || value["state"] == nil {
		return false
	}
	for key := range value {
		if key != "operation_id" && key != "state" && key != "poll_url" && key != "result" {
			return false
		}
	}
	var id, state string
	if json.Unmarshal(value["operation_id"], &id) != nil || !domain.ValidOperationID(id) || json.Unmarshal(value["state"], &state) != nil {
		return false
	}
	switch state {
	case "PREPARING", "FENCING", "SUCCEEDED", "REJECTED", "CANCELLED":
	default:
		return false
	}
	if result := value["result"]; result != nil {
		var fields map[string]string
		if json.Unmarshal(result, &fields) != nil || len(fields) != 1 || !domain.ValidEntityID(fields["entity_id"]) {
			return false
		}
	}
	return true
}

````

## 第 5 步：anonymousOutput

插入位置：第 4 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`func anonymousOutput(path string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>path == &quot;/v1/account/login&quot;</code>；<code>path == &quot;/v1/account/signup&quot;</code>；<code>path == &quot;/v1/account/challenges&quot;</code>；<code>path == &quot;/v1/account/password-resets&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>path == &quot;/v1/account/login&quot; &#124;&#124; path == &quot;/v1/account/signup&quot; &#124;&#124; path == &quot;/v1/account/challenges&quot; &#124;&#124; path == &quot;/v1/account/password-resets&quot;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return path == &quot;/v1/account/login&quot; &#124;&#124; path == &quot;/v1/account/signup&quot; &#124;&#124; path == &quot;/v1/account/challenges&quot; &#124;&#124; path == &quot;/v1/account/password-resets&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1587:1776 -->
````go
func anonymousOutput(path string) bool {
	return path == "/v1/account/login" || path == "/v1/account/signup" || path == "/v1/account/challenges" || path == "/v1/account/password-resets"
}

````

## 第 6 步：Outputs.Protect

插入位置：第 5 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func (h *Outputs) Protect(next http.HandlerFunc) http.HandlerFunc`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>next</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>next(w, r)</code>；<code>next(capture, r)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>实际返回表达式：<code>func(w http.ResponseWriter, r *http.Request) { 		if !strings.HasPrefix(r.URL.Path, &quot;/v1/&quot;) &#124;&#124; anonymousOutput(r.URL.Path) &#124;&#124; platform.HTTPCaller(r, &quot;document&quot;) { 			next(w, r) 			r …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(w http.ResponseWriter, r *http.Request) { 		if !strings.HasPrefix(r.URL.Path, &quot;/v1/&quot;) &#124;&#124; anonymousOutput(r.URL.Path) &#124;&#124; platform.HTTPCaller(r, &quot;document&quot;) { 			next(w,  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>!strings.HasPrefix(r.URL.Path, &quot;/v1/&quot;) &#124;&#124; anonymousOutput(r.URL.Path) &#124;&#124; platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 在本段的匿名函数内：此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>capture.Status() &gt;= 400 &#124;&#124; MinimalReceipt(capture.Body()) &#124;&#124; MinimalClosureReceipt(r.URL.Path, capture.Body())</code>。
- 在本段的匿名函数内：此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>r.Header.Get(&quot;X-Edge-Handler&quot;) == &quot;&quot; &#124;&#124; binding.Invalid &#124;&#124; len(binding.Sources) == 0 &#124;&#124; capture.Size() &gt; 2&lt;&lt;20</code>。
- 在本段的匿名函数内：此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：检查 <code>err.Error() == &quot;SOURCE_CHANGED&quot; &#124;&#124; err.Error() == &quot;SOURCE_INVALID&quot;</code>。
- 在本段的匿名函数内：此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1776:3180 -->
````go
func (h *Outputs) Protect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") || anonymousOutput(r.URL.Path) || platform.HTTPCaller(r, "document") {
			next(w, r)
			return
		}
		binding := &domain.OutputBinding{Sources: map[string]string{}}
		r = r.WithContext(context.WithValue(r.Context(), outputKey{}, binding))
		capture := httpx.NewResponseCapture()
		next(capture, r)
		if capture.Status() >= 400 || MinimalReceipt(capture.Body()) || MinimalClosureReceipt(r.URL.Path, capture.Body()) {
			capture.CopyTo(w)
			return
		}
		// 短JSON完全构造后才登记入口实际handler，私有直连没有可控制输出资格。
		if r.Header.Get("X-Edge-Handler") == "" || binding.Invalid || len(binding.Sources) == 0 || capture.Size() > 2<<20 {
			httpx.Failure(w, 503, "OUTPUT_LEASE_REQUIRED")
			return
		}
		if err := h.logic.Register(r.Context(), r.Header.Get("X-Edge-Handler"), binding); err != nil {
			reason := "OUTPUT_REGISTER_UNKNOWN"
			if err.Error() == "SOURCE_CHANGED" || err.Error() == "SOURCE_INVALID" {
				reason = err.Error()
			}
			slog.Warn("account_output_pending", "reason", reason)
			httpx.Failure(w, 503, "ACCOUNT_OUTPUT_NOT_CURRENT")
			return
		}
		capture.CopyTo(w)
		_ = http.NewResponseController(w).Flush()
	}
}

// output_binding.go：保留原实现，按业务合并文件。

````

## 第 7 步：outputKey

插入位置：第 6 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`type outputKey struct{}`。

<!-- write-step: 7 bytes=3180:3205 -->
````go
type outputKey struct{}

````

## 第 8 步：CaptureSource

插入位置：第 7 步代码之后，紧接追加；完成稿约第 94 行。

本步声明：`func CaptureSource(ctx context.Context, key string, epoch uint64)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Value</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>binding.Sources&#91;key&#93; != &quot;&quot;</code>；<code>binding.Sources&#91;key&#93; != value</code>；<code>binding.Sources&#91;key&#93; = value</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>strconv.FormatUint(epoch, 10)</code>；<code>epoch == 0</code> |

**跟着本段代码走一遍：**

- 检查 <code>binding == nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>epoch == 0 &#124;&#124; (binding.Sources&#91;key&#93; != &quot;&quot; &amp;&amp; binding.Sources&#91;key&#93; != value)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3205:3560 -->
````go
func CaptureSource(ctx context.Context, key string, epoch uint64) {
	binding, _ := ctx.Value(outputKey{}).(*domain.OutputBinding)
	if binding == nil {
		return
	}
	value := strconv.FormatUint(epoch, 10)
	if epoch == 0 || (binding.Sources[key] != "" && binding.Sources[key] != value) {
		binding.Invalid = true
		return
	}
	binding.Sources[key] = value
}

````

## 第 9 步：CaptureSessionSources

插入位置：第 8 步代码之后，紧接追加；完成稿约第 107 行。

本步声明：`func CaptureSessionSources(ctx context.Context, session domain.Session, identity uint64)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>CaptureSource(ctx, &quot;identity:&quot;+session.UserID, identity)</code>；<code>CaptureSource(ctx, &quot;credential:&quot;+session.UserID, credential)</code>；<code>CaptureSource(ctx, &quot;session:&quot;+hex.EncodeToString(session.ID), session.Epoch)</code> |
| <code>session</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>在此声明中的实际使用：<code>session.CredentialEpoch</code>；<code>session.UserID</code>；<code>session.ID</code>；<code>session.Epoch</code> |
| <code>identity</code> | <code>uint64</code> | 在此声明中的实际使用：<code>CaptureSource(ctx, &quot;identity:&quot;+session.UserID, identity)</code> |

<!-- write-step: 9 bytes=3560:3920 -->
````go
func CaptureSessionSources(ctx context.Context, session domain.Session, identity uint64) {
	credential, _ := strconv.ParseUint(session.CredentialEpoch, 10, 64)
	CaptureSource(ctx, "identity:"+session.UserID, identity)
	CaptureSource(ctx, "credential:"+session.UserID, credential)
	CaptureSource(ctx, "session:"+hex.EncodeToString(session.ID), session.Epoch)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/account/outputs_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/08-output-and-wiring.md)。
