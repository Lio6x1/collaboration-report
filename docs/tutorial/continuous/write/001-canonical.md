# 分步编写 `contracts/content/canonical.go`

先按 [F01 · 严格正文、规范字节与格式契约](../lessons/foundation/01-fixed-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/001-canonical.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把已解码正文写成Go与Node一致的canonical-v1字节。

正文摘要必须针对相同字节；Go字符串UTF8排序与JavaScript UTF16键序可能不同，因此显式比较UTF16编码。

写之前先按这条执行路线理解各段的连接：

1. writeCanonical对对象键用nodeStringLess排序，数组保持原顺序，字符串保留空白和Unicode。
2. 整数只接受json.Number且落在规范范围，布尔/null固定编码，字符串关闭HTML转义。

本文件需要守住的边界：

- 不能Trim正文、排序数组或把数字转科学计数法；这些改变会破坏固定内容摘要。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'contracts/content' -Force | Out-Null
if ((Test-Path -LiteralPath 'contracts/content/canonical.go') -and (Get-Item -LiteralPath 'contracts/content/canonical.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'contracts/content/canonical.go')) { New-Item -ItemType File -Path 'contracts/content/canonical.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:107 -->
````go
package content

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

````

## 第 2 步：Canonical

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

Canonical与Node canonical-v1使用同一UTF-16键序，保留Unicode、正文空白与数组顺序。

本步声明：`func Canonical(value any) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>writeCanonical(&amp;result, value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>result.Bytes()</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return result.Bytes(), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=107:390 -->
````go
// Canonical与Node canonical-v1使用同一UTF-16键序，保留Unicode、正文空白与数组顺序。
func Canonical(value any) ([]byte, error) {
	var result bytes.Buffer
	if err := writeCanonical(&result, value); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

````

## 第 3 步：writeCanonical

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func writeCanonical(result *bytes.Buffer, value any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>result</code> | <code>*bytes.Buffer</code> | 在此声明中的实际使用：<code>result.WriteByte</code>；<code>writeCanonical(result, key)</code>；<code>writeCanonical(result, value&#91;key&#93;)</code>；<code>writeCanonical(result, child)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>value := value.(type)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ErrInvalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nodeStringLess(keys&#91;i&#93;, keys&#91;j&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>index != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>index != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!utf8.ValidString(value)</code>。条件成立时执行 <code>return ErrInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return ErrInvalid</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=390:1557 -->
````go
func writeCanonical(result *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return nodeStringLess(keys[i], keys[j]) })
		result.WriteByte('{')
		for index, key := range keys {
			if index != 0 {
				result.WriteByte(',')
			}
			if err := writeCanonical(result, key); err != nil {
				return err
			}
			result.WriteByte(':')
			if err := writeCanonical(result, value[key]); err != nil {
				return err
			}
		}
		result.WriteByte('}')
	case []any:
		result.WriteByte('[')
		for index, child := range value {
			if index != 0 {
				result.WriteByte(',')
			}
			if err := writeCanonical(result, child); err != nil {
				return err
			}
		}
		result.WriteByte(']')
	case string:
		if !utf8.ValidString(value) {
			return ErrInvalid
		}
		var encoded bytes.Buffer
		encoder := json.NewEncoder(&encoded)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(value); err != nil {
			return ErrInvalid
		}
		result.Write(bytes.TrimSuffix(encoded.Bytes(), []byte("\n")))
	case json.Number:
````

## 第 4 步：writeCanonical（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 66 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; number &lt; -2147483647 &#124;&#124; number &gt; 2147483647</code>。条件成立时执行 <code>return ErrInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1557:1878 -->
````go
		number, err := value.Int64()
		if err != nil || number < -2147483647 || number > 2147483647 {
			return ErrInvalid
		}
		result.WriteString(strconv.FormatInt(number, 10))
	case bool:
		result.WriteString(strconv.FormatBool(value))
	case nil:
		result.WriteString("null")
	default:
		return ErrInvalid
	}
	return nil
}

````

## 第 5 步：nodeStringLess

插入位置：第 4 步代码之后，紧接追加；完成稿约第 81 行。

本步声明：`func nodeStringLess(left, right string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>left</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;rune(left)</code> |
| <code>right</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;rune(right)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>a&#91;i&#93; &lt; b&#91;i&#93;</code>；<code>len(a) &lt; len(b)</code> |

**跟着本段代码走一遍：**

- 检查 <code>a&#91;i&#93; != b&#91;i&#93;</code>。条件成立时执行 <code>return a&#91;i&#93; &lt; b&#91;i&#93;</code>。当前函数在这里结束。
- 此处通过 <code>return len(a) &lt; len(b)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1878:2110 -->
````go
func nodeStringLess(left, right string) bool {
	a, b := utf16.Encode([]rune(left)), utf16.Encode([]rune(right))
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'contracts/content/canonical.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/01-fixed-content.md)。
