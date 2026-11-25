# 分步编写 `services/document/internal/domain/document/resources_models.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/099-resources-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

表达草稿正文计量、资源保护绑定及Node提交证明。

资源预保护与正文容量预留要绑定同一可靠UPDATE，不能因客户端断线误判资源已不用。

写之前先按这条执行路线理解各段的连接：

1. ParseDraftBodyMeasure规范大小和hash
2. ProtectionDigest固定资源集合
3. ProtectionReceiptDigest绑定operation/generation/state
4. ProtectionFailure区分冲突与证据不足。

本文件需要守住的边界：

- 有保护回执不代表Node已提交
- 最终释放或结算需要真实持久结果。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/resources_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/resources_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/resources_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/resources_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:249 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// draft_body_capacity.go：保留原实现，按业务合并文件。

````

## 第 2 步：DraftBodyMeasure

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`type DraftBodyMeasure struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Bytes</code> | <code>uint64</code> | 规范草稿正文的实际字节数 |
| <code>Hash</code> | <code>&#91;&#93;byte</code> | 该规范正文SHA-256，不是Yjs update hash |

<!-- write-step: 2 bytes=249:311 -->
````go
type DraftBodyMeasure struct {
	Bytes uint64
	Hash  []byte
}

````

## 第 3 步：ParseDraftBodyMeasure

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func ParseDraftBodyMeasure(size, hash string) (DraftBodyMeasure, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>size</code> | <code>string</code> | 在此声明中的实际使用：<code>CanonicalID(size)</code> |
| <code>hash</code> | <code>string</code> | 在此声明中的实际使用：<code>hex.DecodeString(hash)</code>；<code>hex.EncodeToString(digest) != hash</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>DraftBodyMeasure</code> | 实际返回表达式：<code>DraftBodyMeasure{}</code>；<code>DraftBodyMeasure{Bytes: bytes, Hash: digest}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;DRAFT_BODY_MEASURE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; bytes &gt; 16&lt;&lt;20 &#124;&#124; decodeErr != nil &#124;&#124; len(digest) != 32 &#124;&#124; hex.EncodeToString(digest) != hash</code>。条件成立时执行 <code>return DraftBodyMeasure{}, Violation(InvalidRequest, &quot;DRAFT_BODY_MEASURE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return DraftBodyMeasure{Bytes: bytes, Hash: digest}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=311:798 -->
````go
func ParseDraftBodyMeasure(size, hash string) (DraftBodyMeasure, error) {
	bytes, err := CanonicalID(size)
	digest, decodeErr := hex.DecodeString(hash)
	if err != nil || bytes > 16<<20 || decodeErr != nil || len(digest) != 32 || hex.EncodeToString(digest) != hash {
		return DraftBodyMeasure{}, Violation(InvalidRequest, "DRAFT_BODY_MEASURE_INVALID")
	}
	return DraftBodyMeasure{Bytes: bytes, Hash: digest}, nil
}

// resource_protection.go：保留原实现，按业务合并文件。

````

## 第 4 步：ProtectionFailureKind

插入位置：第 3 步代码之后，紧接追加；完成稿约第 28 行。

ProtectionFailure区分请求冲突与缺少持久证据；gRPC状态由server映射，领域不依赖RPC包。

本步声明：`type ProtectionFailureKind uint8`。

<!-- write-step: 4 bytes=798:947 -->
````go
// ProtectionFailure区分请求冲突与缺少持久证据；gRPC状态由server映射，领域不依赖RPC包。
type ProtectionFailureKind uint8

````

## 第 5 步：ProtectionInvalid、ProtectionForbidden、ProtectionMissing、ProtectionConflict、ProtectionPrecondition、ProtectionUnknown

插入位置：第 4 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`const (`。

<!-- write-step: 5 bytes=947:1109 -->
````go
const (
	ProtectionInvalid ProtectionFailureKind = iota
	ProtectionForbidden
	ProtectionMissing
	ProtectionConflict
	ProtectionPrecondition
	ProtectionUnknown
)

````

## 第 6 步：ProtectionFailure

插入位置：第 5 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`type ProtectionFailure struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Kind</code> | <code>ProtectionFailureKind</code> | 冲突或持久证据不足的保护失败类别 |
| <code>Code</code> | <code>string</code> | 稳定保护协议错误码 |

<!-- write-step: 6 bytes=1109:1185 -->
````go
type ProtectionFailure struct {
	Kind ProtectionFailureKind
	Code string
}

````

## 第 7 步：ProtectionFailure.Error

插入位置：第 6 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func (e ProtectionFailure) Error() string`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>e.Code</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return e.Code</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1185:1245 -->
````go
func (e ProtectionFailure) Error() string { return e.Code }
````

## 第 8 步：RejectProtection

插入位置：第 7 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`func RejectProtection(kind ProtectionFailureKind, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>kind</code> | <code>ProtectionFailureKind</code> | 使用 <code>ProtectionFailureKind</code> 类型；其定义处的职责：ProtectionFailure区分请求冲突与缺少持久证据；gRPC状态由server映射，领域不依赖RPC包。<br>在此声明中的实际使用：<code>Kind: kind</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>Code: code</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ProtectionFailure{Kind: kind, Code: code}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return ProtectionFailure{Kind: kind, Code: code}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1245:1368 -->
````go
func RejectProtection(kind ProtectionFailureKind, code string) error {
	return ProtectionFailure{Kind: kind, Code: code}
}
````

## 第 9 步：ProtectionDigest

插入位置：第 8 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`func ProtectionDigest(ids []uint64) []byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ids</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(ids)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>sum&#91;:&#93;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return sum&#91;:&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=1368:1579 -->
````go
func ProtectionDigest(ids []uint64) []byte {
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = TextID(id)
	}
	raw, _ := json.Marshal(values)
	sum := sha256.Sum256(raw)
	return sum[:]
}
````

## 第 10 步：ProtectionReceiptDigest

插入位置：第 9 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`func ProtectionReceiptDigest(operation string, generation uint64, resources []byte, state string) []byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>operation + &quot;\n&quot;</code> |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>TextID(generation)</code> |
| <code>resources</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>hex.EncodeToString(resources)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>operation + &quot;\n&quot; + TextID(generation) + &quot;\n&quot; + hex.EncodeToString(resources) + &quot;\n&quot; + state</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>sum&#91;:&#93;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return sum&#91;:&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=1579:1827 -->
````go
func ProtectionReceiptDigest(operation string, generation uint64, resources []byte, state string) []byte {
	sum := sha256.Sum256([]byte(operation + "\n" + TextID(generation) + "\n" + hex.EncodeToString(resources) + "\n" + state))
	return sum[:]
}

````

## 第 11 步：ProtectionRecord

插入位置：第 10 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`type ProtectionRecord struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | UPDATE的资源保护原号 |
| <code>Document</code> | <code>uint64</code> | 真实文档 |
| <code>Room</code> | <code>uint64</code> | 原协作房间 |
| <code>Epoch</code> | <code>uint64</code> | 原工作区代次 |
| <code>Generation</code> | <code>uint64</code> | 原保护代次 |
| <code>Grant</code> | <code>string</code> | 已兑换协作grant ID |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 保护完整绑定摘要 |
| <code>ResourceJSON</code> | <code>string</code> | 规范资源ID集合 |
| <code>SourceJSON</code> | <code>string</code> | 原EDIT准入全部来源 |

<!-- write-step: 11 bytes=1827:2070 -->
````go
type ProtectionRecord struct {
	Operation                         string
	Document, Room, Epoch, Generation uint64
	Grant                             string
	Fingerprint                       []byte
	ResourceJSON, SourceJSON          string
}
````

## 第 12 步：ProtectionState

插入位置：第 11 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`type ProtectionState struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 已登记保护请求摘要，防同号换内容 |
| <code>State</code> | <code>string</code> | 保护持久阶段，如CONFIRMED或RELEASED |

<!-- write-step: 12 bytes=2070:2142 -->
````go
type ProtectionState struct {
	Fingerprint []byte
	State       string
}
````

## 第 13 步：NodeUpdateProof

插入位置：第 12 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`type NodeUpdateProof struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | Node原UPDATE持久结果<br>序列化/校验标记：<code>`json:&quot;state&quot;`</code> |
| <code>Seq</code> | <code>string</code> | 可靠日志序号文本<br>序列化/校验标记：<code>`json:&quot;durable_seq&quot;`</code> |
| <code>Hash</code> | <code>string</code> | 原二进制UPDATE摘要<br>序列化/校验标记：<code>`json:&quot;update_hash&quot;`</code> |
| <code>Protection</code> | <code>string</code> | 原保护操作号<br>序列化/校验标记：<code>`json:&quot;protection_operation_id&quot;`</code> |
| <code>Grant</code> | <code>string</code> | 原协作grant<br>序列化/校验标记：<code>`json:&quot;grant_id&quot;`</code> |
| <code>BodyBytes</code> | <code>string</code> | Node确认的规范正文大小文本<br>序列化/校验标记：<code>`json:&quot;body_bytes&quot;`</code> |
| <code>BodyHash</code> | <code>string</code> | Node确认的规范正文摘要文本<br>序列化/校验标记：<code>`json:&quot;body_hash&quot;`</code> |

<!-- write-step: 13 bytes=2142:2454 -->
````go
type NodeUpdateProof struct {
	State      string `json:"state"`
	Seq        string `json:"durable_seq"`
	Hash       string `json:"update_hash"`
	Protection string `json:"protection_operation_id"`
	Grant      string `json:"grant_id"`
	BodyBytes  string `json:"body_bytes"`
	BodyHash   string `json:"body_hash"`
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/resources_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
