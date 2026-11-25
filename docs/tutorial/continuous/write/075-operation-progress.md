# 分步编写 `services/account/internal/handler/account/operation_progress.go`

先按 [A04 · 原操作回执、授权证明与平台治理任免](../lessons/account/04-receipts-governance.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/075-operation-progress.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把已授权的账号原操作进度转换为HTTP JSON。

同一端点既处理旧会话最小状态，也处理有效会话可见结果，保持恢复权限的窄范围。

写之前先按这条执行路线理解各段的连接：

1. Logout.Progress 校验路径operationId，传入主Cookie和恢复Cookie。
2. logic返回State时构造ProgressDTO；否则把原Result按json.RawMessage写出。

本文件需要守住的边界：

- 操作号格式无效立即400；未授权查询由logic返回未找到，不能先泄露结果存在性。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/account/operation_progress.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/account/operation_progress.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/account/operation_progress.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/account/operation_progress.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:235 -->
````go
package account

import (
	"encoding/json"
	"net/http"

	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	"collaboration/platform"

	"github.com/zeromicro/go-zero/rest/pathvar"
)

````

## 第 2 步：Logout.Progress

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func (h *Logout) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 400, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>httpx.WriteError(w, err)</code>；<code>platform.WriteJSON(w, 200, ProgressDTO(id, result.State))</code>；<code>platform.WriteJSON(w, 200, json.RawMessage(result.Result))</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>httpx.CookieValue(r, httpx.SessionCookie)</code>；<code>httpx.CookieValue(r, httpx.RecoveryCookie)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(id)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>result.State != &quot;&quot;</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=235:788 -->
````go
func (h *Logout) Progress(w http.ResponseWriter, r *http.Request) {
	id := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(id) {
		httpx.Failure(w, 400, "OPERATION_ID_INVALID")
		return
	}
	result, err := h.logic.Progress(r.Context(), id, httpx.CookieValue(r, httpx.SessionCookie), httpx.CookieValue(r, httpx.RecoveryCookie))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if result.State != "" {
		platform.WriteJSON(w, 200, ProgressDTO(id, result.State))
		return
	}
	platform.WriteJSON(w, 200, json.RawMessage(result.Result))
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/account/operation_progress.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/04-receipts-governance.md)。
