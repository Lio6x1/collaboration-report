# F01 从一段正文开始：严格解码、校验和固定字节

## 这节课要实现什么

用户写好一段包含文字、表格和图片引用的正文，编辑服务与文档服务要认定它是同一份内容。本课实现正文的严格读取、格式校验和统一编码，为后续保存、送审提供共同基础。

## 做完后能看到什么

合法正文能被接受并得到稳定字节；重复JSON键、非法字符和不符合正文格式的内容会被拒绝。这里只做正文处理规则，还没有编辑页面或数据库保存。

## 先想清楚怎样实现

1. 先从原始字节检查输入是否合法。
2. 再把解析出的树编码成统一字节。
3. 最后按正文契约检查节点和属性，给调用方返回正文或明确错误。

## 怎样判断这一课完成

本课末即可运行正文纯规则测试，比较合法输入与拒绝样例；Go与Node共同处理正文的完整链路在后续协作课及06验证。

## 开始写之前

本课要解决一个具体问题：同一份正文经 Go 和 Node 处理后，必须得到相同的字节和摘要。否则你在共同支线看到的“已保存版本”和文档服务送审的“固定版本”可能不是同一内容。先实现不需要数据库的这条链路，本课结束即可运行真实纯函数测试。

学习工程仍是 `D:/job/golang/projects/collaboration-report-learning`，已经按 [00](../../00-start.md) 取到契约 JSON 和测试，但没有业务 Go 文件。这里不新建第二个工程。

## 1. 先创建严格解码文件

```powershell
# 工作目录：自己的同一个学习工程。
New-Item -ItemType Directory -Path 'contracts/content' -Force | Out-Null
```

先进入 [decode.go 分步操作](../../write/002-decode.md)，创建 `contracts/content/decode.go`。文件头声明 `package content`；这里导入 `encoding/json`，是使用标准库的词法解析，不表示接受标准库的所有宽松行为。先写 `ErrInvalid`，让调用方通过同一错误表示“不满足固定正文契约”，避免把底层解析错误或正文内容直接返回用户。

随后写 `Decode(raw []byte) (any, error)`。`raw` 是收到的原字节，不能先转成对象再来做字节合法性检查；`any` 是解出的 JSON 树，`error` 非空时调用者不得使用树。

<!-- lesson-code: contracts/content/decode.go :: Decode -->
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
<!-- /lesson-code -->

沿输入 `{"text":"你好","count":2}` 推演：先检查大小/UTF-8/完整 JSON/代理项，再创建 decoder；`UseNumber` 让数字保留为 `json.Number`，避免先变成 float64 丢失精确性。`count := 0` 是整个递归树共用的节点计数，传地址使子调用累加同一个计数。最后再读一次 token，只有 EOF 才说明没有第二个 JSON 值混在尾部。

接着在同一文件末尾按操作页写 `decodeValue` 和 `validSurrogates`。`depth` 是当前嵌套层数，超过 128 拒绝；`*count` 超过 1000000 拒绝，防止很小的嵌套值树占住大量资源。对象分支先检查 key 是否已经存在，之后才能赋值；如果直接 `json.Unmarshal` 到 map，`{"id":"a","id":"b"}` 会悄悄覆盖原字段。数组保持原次序。数字只允许 -2147483647 到 2147483647 的整数，这是两种语言共同遵守的正文范围，不是 Go int 的机器范围。

Unicode 分支有两个容易误判的输入：`"\ud800"` 是孤立高代理项，应拒绝；`"\\ud800"` 表示文字反斜杠加 u，不是一个 Unicode 转义，应保留。`validSurrogates` 必须跳过被转义的反斜杠。真正的 `"\ud83d\ude00"` 是合法成对代理项，允许解成 😀。

## 2. 把合法树编码成唯一字节

新建 [canonical.go](../../write/001-canonical.md)。先写 `Canonical(value any) ([]byte,error)`，再写其调用的 `writeCanonical`，最后写 `nodeStringLess`；三个声明都在这个文件内，按操作页逐段追加，不另建同名示例函数。

<!-- lesson-code: contracts/content/canonical.go :: Canonical -->
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
<!-- /lesson-code -->

`bytes.Buffer` 是本次调用私有的累积结果，取地址传给递归函数；没有共享可变全局缓冲区。只在整个递归成功时返回 `result.Bytes()`，中途失败的半截 JSON 不返回给调用者。

写 `writeCanonical` 的顺序是对象、数组、字符串、整数、布尔、null。对象先收集键，再按 `nodeStringLess` 排序；数组不能排序，因为段落先后本来就是正文的一部分。字符串的 `SetEscapeHTML(false)` 避免 Go 把普通字符转成与 Node 不同的编码表示；去掉编码器附加的换行，换行不能偷偷进入正文摘要。

<!-- lesson-code: contracts/content/canonical.go :: nodeStringLess -->
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
<!-- /lesson-code -->

这里把 rune 转成 UTF-16 单元后比较，是为了与 JavaScript 字符串排序一致。测试输入 `{"\ue000":1,"😀":2}` 的规范输出为 `{"😀":2,"\ue000":1}`；直接按 UTF-8 字节或 Go rune 排序会在这个例子上改变键序。这个比较器返回 bool，`sort.Slice` 需要的就是“左项是否先于右项”，不是三值比较结果。

## 3. 规定什么树才是本项目的正文

最后新建 [validator.go](../../write/003-validator.md)。先写嵌入的 `schemas`、`Validator`、`embeddedOnly`，再写构造函数及 Body/Provenance 方法。

`Validator.body` 保存已编译的正文 schema，`provenance` 保存来源 schema。字段为未导出的指针，调用方只能通过校验方法使用。`embeddedOnly.Load` 返回错误，是禁止 schema 库在运行时下载外部定义或加载任意本地文件；契约来自支持包中被 `go:embed` 指定的三份文件。

<!-- lesson-code: contracts/content/validator.go :: NewValidator -->
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
<!-- /lesson-code -->

按构造过程走一遍：创建编译器→限制加载器→读嵌入文件→使用刚写的严格 Decode→仅对正文合并段落格式扩展→按 `$id` 登记并编译→分别保存正文/来源 schema。构造失败返回 nil,error，服务启动者不能跳过它继续接正文。

段落扩展只允许 `align=left/center/right` 和布尔 `firstLineIndent`，节点范围仅 paragraph/heading。`mergeParagraphFormattingExtension` 先比较扩展本身是否完全符合允许形状，再克隆基准 schema 添加两个字段。这样支持居中和首行缩进，也不会把 `additionalProperties:false` 放开成任意 CSS 输入。

`Body(value)` 和 `Provenance(value)` 的入参都已经是树；校验失败统一返回 ErrInvalid，成功返回 nil。Decode 负责原字节合法，Body 负责正文结构合法，Canonical 负责固定字节。三者职责不同，不能删掉其中一个，以为“JSON 能解析”就代表可送审正文。

## 4. 此时怎样被后续功能调用

后续 Go 文档流程按 `content.Decode → validator.Body → content.Canonical → SHA-256` 固定正文；Node 侧实现同一规范。F01 尚不注册 HTTP 路由，因为它是正文入口和固定版本流程共同依赖的库。D03/C01 会把它接入真实写入和固定读取，本课不伪造保存接口。

## 5. 运行本课测试

```powershell
# 工作目录：自己的学习工程；support 已提供 go.mod/go.sum 和 codec_test.go。
$env:GOWORK = 'off'
go -C contracts test -mod=readonly ./content -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '固定正文规则失败' }
```

预期六个顶层 Test 均出现 RUN/PASS，无 SKIP，不连接数据库。重点看 `TestStrictJSONRejectsInvalidIdentityOfBytes` 的非法 Unicode、重复键、超范围数字；`TestCanonicalNodeGoVectors` 同时比正文、长度和 SHA，证明与锁定样本一致；格式扩展测试证明新格式没有突破冻结规则。测试数量不代替阅读反例。

若缺少 schema 文件，先检查是否把文件写到自己的 `contracts/content`，并确认支持初始化完成；不要从体验实例复制文件。若类型断言失败，检查是否漏写 UseNumber；若 Unicode 样本摘要不同，先查 nodeStringLess 和字符串编码，不能修改测试样本迁就实现。

完成后进入 [F02 数据库](02-private-databases.md)。本课三个文件的完整答案在 [阶段答案索引](../../01-files.md)，仅在写完后核对。
