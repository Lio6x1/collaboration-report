# contracts/content/validator.go

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/003-validator.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`Validator`、`embeddedOnly`、`Load`、`NewValidator`、`mergeParagraphFormattingExtension`、`Body`、`Provenance`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package content

import (
	"embed"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed content.schema.json provenance.schema.json paragraph-formatting.extension.json
var schemas embed.FS

type Validator struct {
	body, provenance *jsonschema.Schema
}

type embeddedOnly struct{}

func (embeddedOnly) Load(string) (any, error) { return nil, errors.New("SCHEMA_NOT_EMBEDDED") }

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

func (v *Validator) Body(value any) error {
	if v.body.Validate(value) != nil {
		return ErrInvalid
	}
	return nil
}

func (v *Validator) Provenance(value any) error {
	if v.provenance.Validate(value) != nil {
		return ErrInvalid
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`393555d5a74c1cc45782d327a6cdd7aebc00d137d554319615d865ef72952146`。[参考实现](../../../../contracts/content/validator.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
