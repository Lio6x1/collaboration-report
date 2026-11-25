# 分步编写 `services/document/internal/handler/document/resources_handlers.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/259-resources-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供上传预约、字节流、完成确认、下载与正文容量保护入口。

二进制传输的状态和JSON业务回执不同，但同样必须受权限、输出租约及文件锁控制。

写之前先按这条执行路线理解各段的连接：

1. BeginUpload解DTO并准备预留
2. UploadBytes持HoldUpload直到响应结束
3. Complete提交READY结算
4. Content使用ResourceReading.Open
5. sendFile检查输出并流式发送。

本文件需要守住的边界：

- 开始发送后不能再追加JSON错误正文
- SendExport需安全文件名
- 私有Protect仍核UPDATE绑定。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/resources_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/resources_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/resources_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/resources_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:639 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"context"
	"encoding/base64"
	"errors"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"net/http"
	"strconv"
	"time"
)

// draft_body_capacity.go：保留原实现，按业务合并文件。

````

## 第 2 步：DraftBodyCapacity

插入位置：第 1 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`type DraftBodyCapacity struct{ usecase *logic.DraftResources }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.DraftResources</code> | 使用 <code>logic.DraftResources</code> 类型；其定义处的职责：为协作UPDATE预保护资源并原子预留正文容量。 |

<!-- write-step: 2 bytes=639:703 -->
````go
type DraftBodyCapacity struct{ usecase *logic.DraftResources }

````

## 第 3 步：NewDraftBodyCapacity

插入位置：第 2 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func NewDraftBodyCapacity(usecase *logic.DraftResources) *DraftBodyCapacity`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.DraftResources</code> | 使用 <code>logic.DraftResources</code> 类型；其定义处的职责：为协作UPDATE预保护资源并原子预留正文容量。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*DraftBodyCapacity</code> | 实际返回表达式：<code>&amp;DraftBodyCapacity{usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;DraftBodyCapacity{usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=703:829 -->
````go
func NewDraftBodyCapacity(usecase *logic.DraftResources) *DraftBodyCapacity {
	return &DraftBodyCapacity{usecase: usecase}
}

````

## 第 4 步：DraftBodyCapacity.Protect

插入位置：第 3 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`func (h *DraftBodyCapacity) Protect(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBodyLimit(w, r, &amp;input, 256&lt;&lt;10)</code>；<code>httpx.Fail(w, 400, &quot;PROTECTION_INVALID&quot;)</code>；<code>platform.WriteJSON(w, 200, map&#91;string&#93;string{&quot;state&quot;: receipt.State.String(), &quot;resource_digest&quot;: base64.StdEncoding.EncodeToString(receipt.ResourceDigest)})</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;collaboration&quot;)</code>；<code>httpx.DecodeBodyLimit(w, r, &amp;input, 256&lt;&lt;10)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;collaboration&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBodyLimit(w, r, &amp;input, 256&lt;&lt;10)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(input.Protection, req) != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err == nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>errors.As(err, &amp;protection)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=829:2158 -->
````go
func (h *DraftBodyCapacity) Protect(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "collaboration") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input platform.DraftBodyProtection
	if !httpx.DecodeBodyLimit(w, r, &input, 256<<10) {
		return
	}
	req := new(pb.DraftResourceRequest)
	if protojson.Unmarshal(input.Protection, req) != nil {
		httpx.Fail(w, 400, "PROTECTION_INVALID")
		return
	}
	body, err := domain.ParseDraftBodyMeasure(input.BodyBytes, input.BodyHash)
	if err == nil {
		var receipt *pb.DraftResourceReceipt
		receipt, err = h.usecase.ProtectBody(r.Context(), req, body)
		if err == nil {
			platform.WriteJSON(w, 200, map[string]string{"state": receipt.State.String(), "resource_digest": base64.StdEncoding.EncodeToString(receipt.ResourceDigest)})
			return
		}
	}
	var protection domain.ProtectionFailure
	if errors.As(err, &protection) {
		status := 503
		switch protection.Kind {
		case domain.ProtectionInvalid:
			status = 400
		case domain.ProtectionForbidden:
			status = 403
		case domain.ProtectionMissing:
			status = 404
		case domain.ProtectionConflict, domain.ProtectionPrecondition:
			status = 409
		}
		httpx.Fail(w, status, protection.Code)
		return
	}
	httpx.Failure(w, err)
}

// resource_content.go：保留原实现，按业务合并文件。

````

## 第 5 步：Resources.Content

插入位置：第 4 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`func (h *Resources) Content(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;RESOURCE_CONTEXT_INVALID&quot;)</code>；<code>h.requests.outputs.Bind(w, registration, sources)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.URL</code>；<code>h.identity.OptionalActor(r, &quot;READ_RESOURCE&quot;, document)</code>；<code>r.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>len(query&#91;key&#93;) != 1</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return output</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 登记 <code>file.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &amp;&amp; !started</code>。

<!-- write-step: 5 bytes=2158:3658 -->
````go
func (h *Resources) Content(w http.ResponseWriter, r *http.Request) {
	resource, err := domain.CanonicalID(pathvar.Vars(r)["resourceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	query := r.URL.Query()
	for _, key := range []string{"document_id", "context_kind", "context_id"} {
		if len(query[key]) != 1 {
			httpx.Fail(w, 400, "RESOURCE_CONTEXT_INVALID")
			return
		}
	}
	document, err := domain.CanonicalID(query.Get("document_id"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	context, err := domain.CanonicalID(query.Get("context_id"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.OptionalActor(r, "READ_RESOURCE", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	file, err := h.reading.Open(r.Context(), actor, logic.ResourceTarget{Resource: resource, Document: document, Context: context, Kind: query.Get("context_kind")}, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.requests.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	defer file.Close()
	started, err := output.SendResource(r.Context(), file)
	// 已发送文件头后只截断连接，不能把JSON错误拼入二进制正文冒充完整文件。
	if err != nil && !started {
		httpx.Failure(w, err)
	}
}

// resource_output.go：保留原实现，按业务合并文件。

````

## 第 6 步：ResponseOutput.SendResource

插入位置：第 5 步代码之后，紧接追加；完成稿约第 121 行。

本步声明：`func (o *ResponseOutput) SendResource(ctx context.Context, file *logic.ResourceDownload) (started bool, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>o.sendFile(ctx, file, &quot;&quot;, false)</code> |
| <code>file</code> | <code>*logic.ResourceDownload</code> | 使用 <code>logic.ResourceDownload</code> 类型；其定义处的职责：按PUBLIC、MAIN、REVIEW或BRANCH实际引用打开资源，并提供空间配额读取。<br>在此声明中的实际使用：<code>o.sendFile(ctx, file, &quot;&quot;, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>started</code> | <code>bool</code> | 由 <code>o.sendFile(ctx, file, &quot;&quot;, false)</code> 的第 1 个返回项原样转交。 |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>o.sendFile(ctx, file, &quot;&quot;, false)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return o.sendFile(ctx, file, &quot;&quot;, false)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3658:3819 -->
````go
func (o *ResponseOutput) SendResource(ctx context.Context, file *logic.ResourceDownload) (started bool, err error) {
	return o.sendFile(ctx, file, "", false)
}

````

## 第 7 步：ResponseOutput.SendExport

插入位置：第 6 步代码之后，紧接追加；完成稿约第 125 行。

本步声明：`func (o *ResponseOutput) SendExport(ctx context.Context, file *logic.ResourceDownload, name string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>o.sendFile(ctx, file, name, true)</code> |
| <code>file</code> | <code>*logic.ResourceDownload</code> | 使用 <code>logic.ResourceDownload</code> 类型；其定义处的职责：按PUBLIC、MAIN、REVIEW或BRANCH实际引用打开资源，并提供空间配额读取。<br>在此声明中的实际使用：<code>o.sendFile(ctx, file, name, true)</code> |
| <code>name</code> | <code>string</code> | 在此声明中的实际使用：<code>o.sendFile(ctx, file, name, true)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>o.sendFile(ctx, file, name, true)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>o.sendFile(ctx, file, name, true)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return o.sendFile(ctx, file, name, true)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3819:3980 -->
````go
func (o *ResponseOutput) SendExport(ctx context.Context, file *logic.ResourceDownload, name string) (bool, error) {
	return o.sendFile(ctx, file, name, true)
}

````

## 第 8 步：ResponseOutput.sendFile

插入位置：第 7 步代码之后，紧接追加；完成稿约第 129 行。

本步声明：`func (o *ResponseOutput) sendFile(ctx context.Context, file *logic.ResourceDownload, name string, export bool) (started bool, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>o.open(ctx)</code>；<code>o.checkStopped(ctx)</code> |
| <code>file</code> | <code>*logic.ResourceDownload</code> | 使用 <code>logic.ResourceDownload</code> 类型；其定义处的职责：按PUBLIC、MAIN、REVIEW或BRANCH实际引用打开资源，并提供空间配额读取。<br>在此声明中的实际使用：<code>file.Size</code>；<code>file.MediaType</code>；<code>file.ID</code>；<code>file.Reader</code> |
| <code>name</code> | <code>string</code> | 在此声明中的实际使用：<code>name = &quot;resource-&quot; + domain.TextID(file.ID)</code>；<code>disposition+`; filename=&quot;`+name</code> |
| <code>export</code> | <code>bool</code> | true把导出结果作为attachment并保留导出文件名；false让允许的图片inline显示，其他类型转下载并生成resource-ID文件名。两者均受同一传输上限和停止检查。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>started</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOURCE_TRANSFER_LIMIT&quot;)</code>；<code>io.ErrShortWrite</code>；<code>controller.Flush()</code> |

**跟着本段代码走一遍：**

- 登记 <code>o.finish()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>file.Size &gt; 32&lt;&lt;20</code>。条件成立时执行 <code>return false, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOURCE_TRANSFER_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>export</code>。
- 登记 <code>func() { 		if err != nil { 			_ = controller.SetWriteDeadline(time.Now()) 		} 	}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return true, err</code>。当前函数在这里结束。
- 检查 <code>left &lt; size</code>。

<!-- write-step: 8 bytes=3980:5436 -->
````go
func (o *ResponseOutput) sendFile(ctx context.Context, file *logic.ResourceDownload, name string, export bool) (started bool, err error) {
	defer o.finish()
	if err = o.open(ctx); err != nil {
		return false, err
	}
	if file.Size > 32<<20 {
		return false, domain.Violation(domain.TemporarilyUnavailable, "RESOURCE_TRANSFER_LIMIT")
	}
	controller := http.NewResponseController(o.w)
	_ = controller.SetWriteDeadline(time.Now().Add(4 * time.Second))
	if err = o.checkStopped(ctx); err != nil {
		return false, err
	}
	media, disposition := file.MediaType, "inline"
	if export {
		disposition = "attachment"
	} else {
		switch media {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
		default:
			media, disposition = "application/octet-stream", "attachment"
		}
		name = "resource-" + domain.TextID(file.ID)
	}
	o.w.Header().Set("Content-Type", media)
	o.w.Header().Set("Content-Disposition", disposition+`; filename="`+name+`"`)
	o.w.Header().Set("Content-Length", strconv.FormatUint(file.Size, 10))
	o.w.Header().Set("Cache-Control", "no-store")
	o.w.Header().Set("X-Content-Type-Options", "nosniff")
	o.w.WriteHeader(http.StatusOK)
	started = true
	defer func() {
		if err != nil {
			_ = controller.SetWriteDeadline(time.Now())
		}
	}()
	buffer := make([]byte, 32<<10)
	left := file.Size
	for left > 0 {
		if err = o.checkStopped(ctx); err != nil {
			return true, err
		}
		size := uint64(len(buffer))
		if left < size {
			size = left
		}
````

## 第 9 步：ResponseOutput.sendFile（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 175 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return true, err</code>。当前函数在这里结束。
- 检查 <code>written != n</code>。条件成立时执行 <code>return true, io.ErrShortWrite</code>。当前函数在这里结束。
- 此处通过 <code>return true, controller.Flush()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5436:5996 -->
````go
		var n int
		n, err = io.ReadFull(file.Reader, buffer[:int(size)])
		if err != nil {
			return true, err
		}
		if err = o.checkStopped(ctx); err != nil {
			return true, err
		}
		var written int
		written, err = o.w.Write(buffer[:n])
		if err != nil {
			return true, err
		}
		if written != n {
			return true, io.ErrShortWrite
		}
		left -= uint64(n)
	}
	// Flush实际结束后才结算Doc租约；Edge仍持有自己的输出直到浏览器写出结束。
	return true, controller.Flush()
}

// resources.go：保留原实现，按业务合并文件。

````

## 第 10 步：Resources

插入位置：第 9 步代码之后，紧接追加；完成稿约第 199 行。

本步声明：`type Resources struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Resources</code> | 使用 <code>logic.Resources</code> 类型；其定义处的职责：编排上传预留、流式写入、完成结算和未引用对象回收。 |
| <code>reading</code> | <code>*logic.ResourceReading</code> | 使用 <code>logic.ResourceReading</code> 类型；其定义处的职责：按PUBLIC、MAIN、REVIEW或BRANCH实际引用打开资源，并提供空间配额读取。 |

<!-- write-step: 10 bytes=5996:6127 -->
````go
type Resources struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Resources
	reading  *logic.ResourceReading
}

````

## 第 11 步：NewResources

插入位置：第 10 步代码之后，紧接追加；完成稿约第 206 行。

本步声明：`func NewResources(identity *Identity, requests *Transactions, usecase *logic.Resources, reading *logic.ResourceReading) *Resources`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Resources</code> | 使用 <code>logic.Resources</code> 类型；其定义处的职责：编排上传预留、流式写入、完成结算和未引用对象回收。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>reading</code> | <code>*logic.ResourceReading</code> | 使用 <code>logic.ResourceReading</code> 类型；其定义处的职责：按PUBLIC、MAIN、REVIEW或BRANCH实际引用打开资源，并提供空间配额读取。<br>在此声明中的实际使用：<code>reading: reading</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Resources</code> | 实际返回表达式：<code>&amp;Resources{identity: identity, requests: requests, usecase: usecase, reading: reading}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Resources{identity: identity, requests: requests, usecase: usecase, reading: reading}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6127:6358 -->
````go
func NewResources(identity *Identity, requests *Transactions, usecase *logic.Resources, reading *logic.ResourceReading) *Resources {
	return &Resources{identity: identity, requests: requests, usecase: usecase, reading: reading}
}

````

## 第 12 步：Resources.BeginUpload

插入位置：第 11 步代码之后，紧接追加；完成稿约第 210 行。

本步声明：`func (h *Resources) BeginUpload(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;RESOURCE_UPLOAD&quot;, input, http.StatusCreated, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;RESOURCE_UPLOAD&quot;, document)</code>；<code>r.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6358:7199 -->
````go
func (h *Resources) BeginUpload(w http.ResponseWriter, r *http.Request) {
	var input dto.ResourceUpload
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "RESOURCE_UPLOAD", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.PrepareUpload(r.Context(), actor, document, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 配额预留和上传登记必须沿同一幂等事务提交；句柄投影再核当前授权，
	// 并由统一输出租约保护，避免重放回执时绕过撤权。
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "RESOURCE_UPLOAD", input, http.StatusCreated, run, present)
}

````

## 第 13 步：Resources.Complete

插入位置：第 12 步代码之后，紧接追加；完成稿约第 235 行。

本步声明：`func (h *Resources) Complete(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;RESOURCE_COMPLETE&quot;, input, http.StatusOK, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;RESOURCE_COMPLETE&quot;, 0)</code>；<code>r.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 登记 <code>release()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=7199:8263 -->
````go
func (h *Resources) Complete(w http.ResponseWriter, r *http.Request) {
	var input dto.UploadComplete
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["resourceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// URL只有资源ID，入口先核验会话和CSRF；用例从持久化上传记录确定
	// 文档并检查上传者与编辑权限，不能信任客户端另报的文档范围。
	actor, err := h.identity.Actor(r, true, "RESOURCE_COMPLETE", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	release, err := h.usecase.HoldUpload(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	defer release()
	actor, run, present, err := h.usecase.PrepareComplete(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 文件核验由用例在SQL锁外完成；容量结算和READY状态仍在统一事务内确认。
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "RESOURCE_COMPLETE", input, http.StatusOK, run, present)
}

````

## 第 14 步：Resources.UploadBytes

插入位置：第 13 步代码之后，紧接追加；完成稿约第 267 行。

本步声明：`func (h *Resources) UploadBytes(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, actor.Sources, 0, &quot;RESOURCE_UPLOAD_BYTES&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;RESOURCE_UPLOAD_BYTES&quot;, 0)</code>；<code>r.Context</code>；<code>r.Body</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=8263:8782 -->
````go
func (h *Resources) UploadBytes(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["resourceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "RESOURCE_UPLOAD_BYTES", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, err := h.usecase.WriteUpload(r.Context(), actor, id, r.Body)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, actor.Sources, 0, "RESOURCE_UPLOAD_BYTES", run)
}

````

## 第 15 步：Resources.Quota

插入位置：第 14 步代码之后，紧接追加；完成稿约第 286 行。

本步声明：`func (h *Resources) Quota(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;READ_RESOURCE_QUOTA&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;READ_RESOURCE_QUOTA&quot;, 0)</code>；<code>r.Context</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;READ_RESOURCE_QUOTA&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=8782:9393 -->
````go
func (h *Resources) Quota(w http.ResponseWriter, r *http.Request) {
	space, err := domain.CanonicalID(pathvar.Vars(r)["spaceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_RESOURCE_QUOTA", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.PrepareQuota(r.Context(), actor, space)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 配额属于空间，由用例补齐空间身份来源；余额和版本通过同一读取事务输出。
	h.requests.Read(w, r, actor, sources, 0, "READ_RESOURCE_QUOTA", run)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/resources_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
