# 分步编写 `packages/platform/operation_receipt.go`

先按 [F04 · 原操作、资格来源与跨服务身份](../lessons/foundation/04-original-operation.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/021-operation-receipt.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在持久回执写入前验证原操作号、状态并得到确定JSON字节。

任意业务map若原号错误或包含不可编码值，不能先提交业务再发现回执坏了。

写之前先按这条执行路线理解各段的连接：

1. EncodeOperationReceipt要求value.operation_id与operation相等，state仅允许四种状态。
2. json.Marshal沿既有map编码得到固定JSON，返回State与JSON供同事务写入。

本文件需要守住的边界：

- 不能改成另一种字段排序或补写未授权DTO字段，固定回执摘要可能依赖旧字节。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/operation_receipt.go') -and (Get-Item -LiteralPath 'packages/platform/operation_receipt.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/operation_receipt.go')) { New-Item -ItemType File -Path 'packages/platform/operation_receipt.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:64 -->
````go
package platform

import (
	"encoding/json"
	"errors"
	"fmt"
)

````

## 第 2 步：EncodedOperationReceipt

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

EncodedOperationReceipt 是写入回执所需的已校验状态和固定 JSON，不是另一套 HTTP DTO。
业务结果可以携带生成契约允许的不同字段，但原号、状态与可编码性必须在持久化前确认。

本步声明：`type EncodedOperationReceipt struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | 已校验的业务状态，可直接写operation_receipt.state。 |
| <code>JSON</code> | <code>json.RawMessage</code> | 同一次校验后产生的确定JSON字节，与业务同事务持久化。 |

<!-- write-step: 2 bytes=64:380 -->
````go
// EncodedOperationReceipt 是写入回执所需的已校验状态和固定 JSON，不是另一套 HTTP DTO。
// 业务结果可以携带生成契约允许的不同字段，但原号、状态与可编码性必须在持久化前确认。
type EncodedOperationReceipt struct {
	State string
	JSON  json.RawMessage
}

````

## 第 3 步：EncodeOperationReceipt

插入位置：第 2 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`func EncodeOperationReceipt(operation string, value map[string]any) (EncodedOperationReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>operation == &quot;&quot;</code>；<code>id != operation</code> |
| <code>value</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>id, ok := value&#91;&quot;operation_id&quot;&#93;.(string)</code>；<code>state, ok := value&#91;&quot;state&quot;&#93;.(string)</code>；<code>json.Marshal(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>EncodedOperationReceipt</code> | 使用 <code>EncodedOperationReceipt</code> 类型；其定义处的职责：EncodedOperationReceipt 是写入回执所需的已校验状态和固定 JSON，不是另一套 HTTP DTO。 业务结果可以携带生成契约允许的不同字段，但原号、状态与可编码性必须在持久化前确认。<br>实际返回表达式：<code>EncodedOperationReceipt{}</code>；<code>EncodedOperationReceipt{State: state, JSON: raw}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;OPERATION_RESULT_BINDING_INVALID&quot;)</code>；<code>errors.New(&quot;OPERATION_RESULT_STATE_INVALID&quot;)</code>；<code>fmt.Errorf(&quot;OPERATION_RESULT_ENCODING_FAILED: %w&quot;, err)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok &#124;&#124; operation == &quot;&quot; &#124;&#124; id != operation</code>。条件成立时执行 <code>return EncodedOperationReceipt{}, errors.New(&quot;OPERATION_RESULT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return EncodedOperationReceipt{}, errors.New(&quot;OPERATION_RESULT_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return EncodedOperationReceipt{}, fmt.Errorf(&quot;OPERATION_RESULT_ENCODING_FAILED: %w&quot;, err)</code>。当前函数在这里结束。
- 此处通过 <code>return EncodedOperationReceipt{State: state, JSON: raw}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=380:1277 -->
````go
func EncodeOperationReceipt(operation string, value map[string]any) (EncodedOperationReceipt, error) {
	id, ok := value["operation_id"].(string)
	if !ok || operation == "" || id != operation {
		return EncodedOperationReceipt{}, errors.New("OPERATION_RESULT_BINDING_INVALID")
	}
	state, ok := value["state"].(string)
	if !ok {
		return EncodedOperationReceipt{}, errors.New("OPERATION_RESULT_STATE_INVALID")
	}
	switch state {
	case "PREPARING", "SUCCEEDED", "REJECTED", "CANCELLED":
	default:
		return EncodedOperationReceipt{}, errors.New("OPERATION_RESULT_STATE_INVALID")
	}
	// 继续使用旧 map 的确定性 JSON 编码，避免固定回执/摘要因字段顺序变化而改变。
	raw, err := json.Marshal(value)
	if err != nil {
		return EncodedOperationReceipt{}, fmt.Errorf("OPERATION_RESULT_ENCODING_FAILED: %w", err)
	}
	return EncodedOperationReceipt{State: state, JSON: raw}, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/operation_receipt.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/04-original-operation.md)。
