# 分步编写 `contracts/content/validator.go`

先按 [F01 · 严格正文、规范字节与格式契约](../lessons/foundation/01-fixed-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/003-validator.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

编译内嵌正文/来源schema，并受限合并段落格式扩展。

验证规则必须由发布版本确定，不能让运行时网络或任意文件替换schema。

写之前先按这条执行路线理解各段的连接：

1. NewValidator从embed.FS读取契约，先Decode再编译，编译结果并发复用。
2. mergeParagraphFormattingExtension仅允许paragraph/heading新增align及firstLineIndent，克隆原schema后修改。
3. Body和Provenance把验证失败统一映射ErrInvalid。

本文件需要守住的边界：

- 扩展不得改required/additionalProperties或覆盖既有字段；embeddedOnly拒绝远端加载。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'contracts/content' -Force | Out-Null
if ((Test-Path -LiteralPath 'contracts/content/validator.go') -and (Get-Item -LiteralPath 'contracts/content/validator.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'contracts/content/validator.go')) { New-Item -ItemType File -Path 'contracts/content/validator.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:121 -->
````go
package content

import (
	"embed"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

````

## 第 2 步：schemas

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`var schemas embed.FS`。

<!-- write-step: 2 bytes=121:233 -->
````go
//go:embed content.schema.json provenance.schema.json paragraph-formatting.extension.json
var schemas embed.FS

````

## 第 3 步：Validator

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`type Validator struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>body</code> | <code>*jsonschema.Schema</code> | 已经编译的content正文schema，Body调用它验证固定正文树。 |
| <code>provenance</code> | <code>*jsonschema.Schema</code> | 已经编译的来源schema，Provenance校验固定内容出处。 |

<!-- write-step: 3 bytes=233:297 -->
````go
type Validator struct {
	body, provenance *jsonschema.Schema
}

````

## 第 4 步：embeddedOnly

插入位置：第 3 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type embeddedOnly struct{}`。

<!-- write-step: 4 bytes=297:325 -->
````go
type embeddedOnly struct{}

````

## 第 5 步：embeddedOnly.Load

插入位置：第 4 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func (embeddedOnly) Load(string) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 外部schema引用地址；参数故意不用，所有外部加载一律SCHEMA_NOT_EMBEDDED。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SCHEMA_NOT_EMBEDDED&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, errors.New(&quot;SCHEMA_NOT_EMBEDDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=325:422 -->
````go
func (embeddedOnly) Load(string) (any, error) { return nil, errors.New("SCHEMA_NOT_EMBEDDED") }

````

## 第 6 步：NewValidator

插入位置：第 5 步代码之后，紧接追加；完成稿约第 23 行。

编译既有契约一次即可并发复用，禁止运行时从网络或任意主机文件加载schema。

本步声明：`func NewValidator() (*Validator, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Validator</code> | 实际返回表达式：<code>nil</code>；<code>&amp;Validator{body: body, provenance: provenance}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>file == &quot;content.schema.json&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return compiler.Compile(id)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Validator{body: body, provenance: provenance}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=422:1616 -->
````go
// 编译既有契约一次即可并发复用，禁止运行时从网络或任意主机文件加载schema。
func NewValidator() (*Validator, error) {
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(embeddedOnly{})
	compile := func(file string) (*jsonschema.Schema, error) {
		raw, err := schemas.ReadFile(file)
		if err != nil {
			return nil, err
		}
		value, err := Decode(raw)
		if err != nil {
			return nil, err
		}
		if file == "content.schema.json" {
			rawExtension, err := schemas.ReadFile("paragraph-formatting.extension.json")
			if err != nil {
				return nil, err
			}
			extension, err := Decode(rawExtension)
			if err != nil {
				return nil, err
			}
			value, err = mergeParagraphFormattingExtension(value, extension)
			if err != nil {
				return nil, err
			}
		}
		id := value.(map[string]any)["$id"].(string)
		if err := compiler.AddResource(id, value); err != nil {
			return nil, err
		}
		return compiler.Compile(id)
	}
	body, err := compile("content.schema.json")
	if err != nil {
		return nil, err
	}
	provenance, err := compile("provenance.schema.json")
	if err != nil {
		return nil, err
	}
	return &Validator{body: body, provenance: provenance}, nil
}

````

## 第 7 步：mergeParagraphFormattingExtension

插入位置：第 6 步代码之后，紧接追加；完成稿约第 67 行。

与Node生成器一样，扩展只能追加段落/标题的两个可选属性。任何其它schema改写
都在编译前拒绝，冻结契约文件及其required/additionalProperties不受扩展影响。

本步声明：`func mergeParagraphFormattingExtension(baseline, extension any) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>baseline</code> | <code>any</code> | 在此声明中的实际使用：<code>json.Marshal(baseline)</code> |
| <code>extension</code> | <code>any</code> | 在此声明中的实际使用：<code>reflect.DeepEqual(extension, allowed)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>merged</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;CONTENT_PARAGRAPH_EXTENSION_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;CONTENT_PARAGRAPH_EXTENSION_OVERRIDE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!reflect.DeepEqual(extension, allowed)</code>。条件成立时执行 <code>return nil, errors.New(&quot;CONTENT_PARAGRAPH_EXTENSION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>attrs&#91;&quot;type&quot;&#93; != &quot;object&quot; &#124;&#124; attrs&#91;&quot;additionalProperties&quot;&#93; != false &#124;&#124; fields == nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;CONTENT_PARAGRAPH_EXTENSION_OVERRIDE&quot;)</code>。当前函数在这里结束。
- 检查 <code>exists</code>。条件成立时执行 <code>return nil, errors.New(&quot;CONTENT_PARAGRAPH_EXTENSION_OVERRIDE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return merged, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1616:3290 -->
````go
// 与Node生成器一样，扩展只能追加段落/标题的两个可选属性。任何其它schema改写
// 都在编译前拒绝，冻结契约文件及其required/additionalProperties不受扩展影响。
func mergeParagraphFormattingExtension(baseline, extension any) (any, error) {
	attributes := map[string]any{
		"align":           map[string]any{"enum": []any{"left", "center", "right"}},
		"firstLineIndent": map[string]any{"type": "boolean"},
	}
	allowed := map[string]any{
		"version":    "paragraph-formatting-v1",
		"nodes":      []any{"paragraph", "heading"},
		"attributes": attributes,
	}
	if !reflect.DeepEqual(extension, allowed) {
		return nil, errors.New("CONTENT_PARAGRAPH_EXTENSION_INVALID")
	}
	// 编译只发生一次，克隆后修改内存schema，避免调用方持有的冻结树被副作用改写。
	raw, err := json.Marshal(baseline)
	if err != nil {
		return nil, err
	}
	merged, err := Decode(raw)
	if err != nil {
		return nil, err
	}
	root, _ := merged.(map[string]any)
	defs, _ := root["$defs"].(map[string]any)
	for _, name := range []string{"paragraph", "heading"} {
		node, _ := defs[name].(map[string]any)
		properties, _ := node["properties"].(map[string]any)
		attrs, _ := properties["attrs"].(map[string]any)
		fields, _ := attrs["properties"].(map[string]any)
		if attrs["type"] != "object" || attrs["additionalProperties"] != false || fields == nil {
			return nil, errors.New("CONTENT_PARAGRAPH_EXTENSION_OVERRIDE")
		}
		for key, value := range attributes {
			if _, exists := fields[key]; exists {
				return nil, errors.New("CONTENT_PARAGRAPH_EXTENSION_OVERRIDE")
			}
			fields[key] = value
		}
	}
	return merged, nil
}

````

## 第 8 步：Validator.Body

插入位置：第 7 步代码之后，紧接追加；完成稿约第 111 行。

本步声明：`func (v *Validator) Body(value any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>v.body.Validate(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrInvalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>v.body.Validate(value) != nil</code>。条件成立时执行 <code>return ErrInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3290:3408 -->
````go
func (v *Validator) Body(value any) error {
	if v.body.Validate(value) != nil {
		return ErrInvalid
	}
	return nil
}

````

## 第 9 步：Validator.Provenance

插入位置：第 8 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func (v *Validator) Provenance(value any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>v.provenance.Validate(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrInvalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>v.provenance.Validate(value) != nil</code>。条件成立时执行 <code>return ErrInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3408:3537 -->
````go
func (v *Validator) Provenance(value any) error {
	if v.provenance.Validate(value) != nil {
		return ErrInvalid
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'contracts/content/validator.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/01-fixed-content.md)。
