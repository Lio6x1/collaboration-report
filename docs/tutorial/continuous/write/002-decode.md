# 分步编写 `contracts/content/decode.go`

先按 [F01 · 严格正文、规范字节与格式契约](../lessons/foundation/01-fixed-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/002-decode.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

严格读取固定正文JSON，保留唯一且可验证的输入含义。

标准json.Unmarshal会接受重复键并替换某些非法Unicode；固定正文用于摘要和跨Go/Node比较，不能容忍静默改写。

写之前先按这条执行路线理解各段的连接：

1. Decode先限32MiB并检查UTF8、JSON语法和代理项。
2. decodeValue逐token读取，拒绝重复对象键，限制深度128和总节点100万，整数限制在±2147483647。
3. validSurrogates要求高代理项紧随低代理项，拒绝孤立低代理项。

本文件需要守住的边界：

- 不能使用float64读取数字后再转整数；不能接受尾随第二份JSON。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'contracts/content' -Force | Out-Null
if ((Test-Path -LiteralPath 'contracts/content/decode.go') -and (Get-Item -LiteralPath 'contracts/content/decode.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'contracts/content/decode.go')) { New-Item -ItemType File -Path 'contracts/content/decode.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:98 -->
````go
package content

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

````

## 第 2 步：ErrInvalid

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`var ErrInvalid = errors.New("FIXED_CONTENT_INVALID")`。

<!-- write-step: 2 bytes=98:152 -->
````go
var ErrInvalid = errors.New("FIXED_CONTENT_INVALID")

````

## 第 3 步：Decode

插入位置：第 2 步代码之后，紧接追加；完成稿约第 14 行。

Decode拒绝非法UTF-8、孤立代理项、重复对象键和超出规范范围的数字，避免标准库静默替换正文。

本步声明：`func Decode(raw []byte) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>utf8.Valid(raw)</code>；<code>json.Valid(raw)</code>；<code>validSurrogates(raw)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrInvalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) == 0 &#124;&#124; len(raw) &gt; 32&lt;&lt;20 &#124;&#124; !utf8.Valid(raw) &#124;&#124; !json.Valid(raw) &#124;&#124; !validSurrogates(raw)</code>。条件成立时执行 <code>return nil, ErrInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrInvalid</code>。当前函数在这里结束。
- 检查 <code>err != io.EOF</code>。条件成立时执行 <code>return nil, ErrInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=152:729 -->
````go
// Decode拒绝非法UTF-8、孤立代理项、重复对象键和超出规范范围的数字，避免标准库静默替换正文。
func Decode(raw []byte) (any, error) {
	if len(raw) == 0 || len(raw) > 32<<20 || !utf8.Valid(raw) || !json.Valid(raw) || !validSurrogates(raw) {
		return nil, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	count := 0
	value, err := decodeValue(decoder, 0, &count)
	if err != nil {
		return nil, ErrInvalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return value, nil
}

````

## 第 4 步：decodeValue

插入位置：第 3 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func decodeValue(decoder *json.Decoder, depth int, count *int) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>decoder</code> | <code>*json.Decoder</code> | 在此声明中的实际使用：<code>decoder.Token</code>；<code>decoder.More</code>；<code>decodeValue(decoder, depth+1, count)</code> |
| <code>depth</code> | <code>int</code> | 在此声明中的实际使用：<code>depth &gt; 128</code>；<code>depth+1</code> |
| <code>count</code> | <code>*int</code> | 在此声明中的实际使用：<code>*count += 1</code>；<code>*count &gt; 1000000</code>；<code>decodeValue(decoder, depth+1, count)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>object</code>；<code>array</code>；<code>json.Number(strconv.FormatInt(value, 10))</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrInvalid</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>depth &gt; 128 &#124;&#124; *count &gt; 1000000</code>。条件成立时执行 <code>return nil, ErrInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return nil, ErrInvalid</code>。当前函数在这里结束。
- 检查 <code>exists</code>。条件成立时执行 <code>return nil, ErrInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; end != json.Delim(&#x27;}&#x27;)</code>。条件成立时执行 <code>return nil, ErrInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return object, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=729:1785 -->
````go
func decodeValue(decoder *json.Decoder, depth int, count *int) (any, error) {
	*count += 1
	if depth > 128 || *count > 1000000 {
		return nil, ErrInvalid
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token := token.(type) {
	case json.Delim:
		switch token {
		case '{':
			object := map[string]any{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, ErrInvalid
				}
				if _, exists := object[key]; exists {
					return nil, ErrInvalid
				}
				child, err := decodeValue(decoder, depth+1, count)
				if err != nil {
					return nil, err
				}
				object[key] = child
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return nil, ErrInvalid
			}
			return object, nil
		case '[':
			array := []any{}
			for decoder.More() {
				child, err := decodeValue(decoder, depth+1, count)
				if err != nil {
					return nil, err
				}
				array = append(array, child)
			}
````

## 第 5 步：decodeValue（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 78 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; end != json.Delim(&#x27;&#93;&#x27;)</code>。条件成立时执行 <code>return nil, ErrInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return array, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; value &lt; -2147483647 &#124;&#124; value &gt; 2147483647</code>。条件成立时执行 <code>return nil, ErrInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return json.Number(strconv.FormatInt(value, 10)), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return token, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1785:2191 -->
````go
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return nil, ErrInvalid
			}
			return array, nil
		}
	case json.Number:
		value, err := token.Int64()
		if err != nil || value < -2147483647 || value > 2147483647 {
			return nil, ErrInvalid
		}
		return json.Number(strconv.FormatInt(value, 10)), nil
	case string, bool, nil:
		return token, nil
	}
	return nil, ErrInvalid
}

````

## 第 6 步：validSurrogates

插入位置：第 5 步代码之后，紧接追加；完成稿约第 96 行。

本步声明：`func validSurrogates(raw []byte) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>raw&#91;i&#93; == &#x27;&quot;&#x27;</code>；<code>raw&#91;i&#93; != &#x27;\\&#x27;</code>；<code>raw&#91;i+1&#93; != &#x27;u&#x27;</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>!inside</code> |

**跟着本段代码走一遍：**

- 检查 <code>raw&#91;i&#93; == &#x27;&quot;&#x27;</code>。
- 检查 <code>!inside &#124;&#124; raw&#91;i&#93; != &#x27;\\&#x27;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>i+1 &gt;= len(raw)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>raw&#91;i+1&#93; != &#x27;u&#x27;</code>。
- 检查 <code>i+5 &gt;= len(raw)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>value &gt;= 0xd800 &amp;&amp; value &lt;= 0xdbff</code>。
- 检查 <code>i+11 &gt;= len(raw) &#124;&#124; raw&#91;i+6&#93; != &#x27;\\&#x27; &#124;&#124; raw&#91;i+7&#93; != &#x27;u&#x27;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; low &lt; 0xdc00 &#124;&#124; low &gt; 0xdfff</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>value &gt;= 0xdc00 &amp;&amp; value &lt;= 0xdfff</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return !inside</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2191:3008 -->
````go
func validSurrogates(raw []byte) bool {
	inside := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inside = !inside
			continue
		}
		if !inside || raw[i] != '\\' {
			continue
		}
		if i+1 >= len(raw) {
			return false
		}
		if raw[i+1] != 'u' {
			i++
			continue
		}
		if i+5 >= len(raw) {
			return false
		}
		value, err := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16)
		if err != nil {
			return false
		}
		if value >= 0xd800 && value <= 0xdbff {
			if i+11 >= len(raw) || raw[i+6] != '\\' || raw[i+7] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(raw[i+8:i+12]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 11
		} else {
			if value >= 0xdc00 && value <= 0xdfff {
				return false
			}
			i += 5
		}
	}
	return !inside
}
````

## 文件完成后的检查

```powershell
gofmt -w 'contracts/content/decode.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/01-fixed-content.md)。
