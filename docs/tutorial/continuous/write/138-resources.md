# 分步编写 `services/document/internal/storage/snapshot/resources.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/138-resources.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在全体应用停写的冷边界复制资源对象并生成快照清单。

备份应忠实记录完整与缺失，不能为让备份成功修改数据库状态或修补损坏对象。

写之前先按这条执行路线理解各段的连接：

1. Capture按库存逐项调用captureOne
2. 校验原key/size/SHA后复制到本地目标
3. 记录missingReady及每对象状态。

本文件需要守住的边界：

- 未知读取或摘要损坏让备份失败
- 不能把在线读取若干文件宣称一致冷备份。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/storage/snapshot' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/storage/snapshot/resources.go') -and (Get-Item -LiteralPath 'services/document/internal/storage/snapshot/resources.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/storage/snapshot/resources.go')) { New-Item -ItemType File -Path 'services/document/internal/storage/snapshot/resources.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:360 -->
````go
// Package snapshot 在已经停止全部应用写者的冷边界复制完整对象。
// 数据库状态不在这里修改；缺失与完整字节都照原事实记录，未知/损坏则失败。
package snapshot

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"collaboration/document/internal/storage/local"
)

````

## 第 2 步：Reader

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`type Reader interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>AcquireRead</code> | <code>(context.Context, string) (func(), error)</code> | ctx与资源存储键输入，返回读取锁释放函数与错误；持锁期间物理清理不能移除资源。 |
| <code>Open</code> | <code>(context.Context, string, uint64, &#91;32&#93;byte) (io.ReadCloser, error)</code> | ctx、存储键、预期字节数、预期SHA-256输入，返回需关闭的读流；快照只能读取已核验资源。 |

<!-- write-step: 2 bytes=360:513 -->
````go
type Reader interface {
	AcquireRead(context.Context, string) (func(), error)
	Open(context.Context, string, uint64, [32]byte) (io.ReadCloser, error)
}

````

## 第 3 步：Object

插入位置：第 2 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`type Object struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Key</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;key&quot;`</code> |
| <code>Size</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;size&quot;`</code> |
| <code>Checksum</code> | <code>&#91;32&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;-&quot;`</code> |
| <code>State</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;state&quot;`</code> |
| <code>MetadataOnly</code> | <code>bool</code> | 序列化/校验标记：<code>`json:&quot;-&quot;`</code> |
| <code>ChecksumAbsent</code> | <code>bool</code> | 序列化/校验标记：<code>`json:&quot;-&quot;`</code> |

<!-- write-step: 3 bytes=513:762 -->
````go
type Object struct {
	Key            string   `json:"key"`
	Size           uint64   `json:"size"`
	Checksum       [32]byte `json:"-"`
	State          string   `json:"state"`
	MetadataOnly   bool     `json:"-"`
	ChecksumAbsent bool     `json:"-"`
}

````

## 第 4 步：Entry

插入位置：第 3 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`type Entry struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Key</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;key&quot;`</code> |
| <code>Size</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;size&quot;`</code> |
| <code>Checksum</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;sha256&quot;`</code> |
| <code>State</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;sourceState&quot;`</code> |
| <code>Present</code> | <code>bool</code> | 序列化/校验标记：<code>`json:&quot;present&quot;`</code> |
| <code>MetadataOnly</code> | <code>bool</code> | 序列化/校验标记：<code>`json:&quot;metadataOnly,omitempty&quot;`</code> |

<!-- write-step: 4 bytes=762:1024 -->
````go
type Entry struct {
	Key          string `json:"key"`
	Size         uint64 `json:"size"`
	Checksum     string `json:"sha256"`
	State        string `json:"sourceState"`
	Present      bool   `json:"present"`
	MetadataOnly bool   `json:"metadataOnly,omitempty"`
}

````

## 第 5 步：Manifest

插入位置：第 4 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`type Manifest struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>int</code> | 序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>State</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;state&quot;`</code> |
| <code>Objects</code> | <code>&#91;&#93;Entry</code> | 序列化/校验标记：<code>`json:&quot;objects&quot;`</code> |
| <code>MissingReady</code> | <code>int</code> | 序列化/校验标记：<code>`json:&quot;missingReady&quot;`</code> |

<!-- write-step: 5 bytes=1024:1209 -->
````go
type Manifest struct {
	Version      int     `json:"version"`
	State        string  `json:"state"`
	Objects      []Entry `json:"objects"`
	MissingReady int     `json:"missingReady"`
}

````

## 第 6 步：Capture

插入位置：第 5 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`func Capture(ctx context.Context, source Reader, objects []Object, destination string) (Manifest, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>captureOne(ctx, source, target, object)</code> |
| <code>source</code> | <code>Reader</code> | 在此声明中的实际使用：<code>captureOne(ctx, source, target, object)</code> |
| <code>objects</code> | <code>&#91;&#93;Object</code> | 在此声明中的实际使用：<code>len(objects)</code> |
| <code>destination</code> | <code>string</code> | 在此声明中的实际使用：<code>local.New(destination)</code>；<code>os.OpenRoot(destination)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Manifest</code> | 实际返回表达式：<code>manifest</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;RESOURCE_SNAPSHOT_BINDING_INVALID&quot;)</code>；<code>errors.New(&quot;RESOURCE_SNAPSHOT_MANIFEST_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return manifest, err</code>。当前函数在这里结束。
- 检查 <code>seen&#91;object.Key&#93; &#124;&#124; (object.State != &quot;UPLOADING&quot; &amp;&amp; object.State != &quot;READY&quot; &amp;&amp; object.State != &quot;DELETING&quot;)</code>。条件成立时执行 <code>return manifest, errors.New(&quot;RESOURCE_SNAPSHOT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return manifest, err</code>。当前函数在这里结束。
- 检查 <code>!entry.Present &amp;&amp; object.State == &quot;READY&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return manifest, errors.New(&quot;RESOURCE_SNAPSHOT_MANIFEST_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>root.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return manifest, errors.New(&quot;RESOURCE_SNAPSHOT_MANIFEST_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>writeErr != nil &#124;&#124; syncErr != nil &#124;&#124; closeErr != nil</code>。条件成立时执行 <code>return manifest, errors.New(&quot;RESOURCE_SNAPSHOT_MANIFEST_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return manifest, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1209:2962 -->
````go
func Capture(ctx context.Context, source Reader, objects []Object, destination string) (Manifest, error) {
	manifest := Manifest{Version: 1, State: "PREPARING", Objects: make([]Entry, 0, len(objects))}
	target, err := local.New(destination)
	if err != nil {
		return manifest, err
	}
	seen := make(map[string]bool)
	for _, object := range objects {
		if seen[object.Key] || (object.State != "UPLOADING" && object.State != "READY" && object.State != "DELETING") {
			return manifest, errors.New("RESOURCE_SNAPSHOT_BINDING_INVALID")
		}
		seen[object.Key] = true
		entry, err := captureOne(ctx, source, target, object)
		if err != nil {
			return manifest, err
		}
		manifest.Objects = append(manifest.Objects, entry)
		if !entry.Present && object.State == "READY" {
			manifest.MissingReady++
		}
	}
	// 原环境包含历史 metadata-only 测试夹具时，明确记录实际缺失；复制不
	// 伪造正文、不改 READY、不声称缺失对象可读，恢复时仍核对同一事实。
	manifest.State = "COMPLETE"
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, errors.New("RESOURCE_SNAPSHOT_MANIFEST_FAILED")
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return manifest, errors.New("RESOURCE_SNAPSHOT_MANIFEST_FAILED")
	}
	defer root.Close()
	file, err := root.OpenFile(filepath.Base("resource-snapshot.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return manifest, errors.New("RESOURCE_SNAPSHOT_MANIFEST_FAILED")
	}
	_, writeErr := file.Write(append(raw, '\n'))
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return manifest, errors.New("RESOURCE_SNAPSHOT_MANIFEST_FAILED")
	}
	return manifest, nil
}

````

## 第 7 步：captureOne

插入位置：第 6 步代码之后，紧接追加；完成稿约第 93 行。

本步声明：`func captureOne(ctx context.Context, source Reader, target *local.Store, object Object) (Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>verifier.VerifyLegacyAbsence(ctx, object.Key)</code>；<code>source.AcquireRead(ctx, object.Key)</code>；<code>source.Open(ctx, object.Key, object.Size, object.Checksum)</code>；<code>target.Put(ctx, object.Key, file, object.Size, object.Checksum)</code> |
| <code>source</code> | <code>Reader</code> | 在此声明中的实际使用：<code>verifier, ok := source.(interface { 			VerifyLegacyAbsence(context.Context, string) error 		})</code>；<code>source.AcquireRead</code>；<code>source.Open</code> |
| <code>target</code> | <code>*local.Store</code> | 使用 <code>local.Store</code> 类型；其定义处的职责：在专用本地目录实现不可变对象Put/Open/Delete。<br>在此声明中的实际使用：<code>target.Put</code> |
| <code>object</code> | <code>Object</code> | 在此声明中的实际使用：<code>object.Key</code>；<code>object.Size</code>；<code>object.Checksum</code>；<code>object.State</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Entry</code> | 实际返回表达式：<code>entry</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;RESOURCE_SNAPSHOT_LEGACY_UNSUPPORTED&quot;)</code>；<code>verifier.VerifyLegacyAbsence(ctx, object.Key)</code>；<code>err</code>；<code>nil</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>object.MetadataOnly</code>。
- 检查 <code>object.ChecksumAbsent</code>。
- 检查 <code>!ok</code>。条件成立时执行 <code>return entry, errors.New(&quot;RESOURCE_SNAPSHOT_LEGACY_UNSUPPORTED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return entry, verifier.VerifyLegacyAbsence(ctx, object.Key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return entry, err</code>。当前函数在这里结束。
- 登记 <code>release()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>errors.Is(err, local.ErrNotFound)</code>。条件成立时执行 <code>return entry, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return entry, err</code>。当前函数在这里结束。
- 检查 <code>putErr != nil</code>。条件成立时执行 <code>return entry, putErr</code>。当前函数在这里结束。
- 检查 <code>closeErr != nil</code>。条件成立时执行 <code>return entry, closeErr</code>。当前函数在这里结束。
- 此处通过 <code>return entry, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2962:4176 -->
````go
func captureOne(ctx context.Context, source Reader, target *local.Store, object Object) (Entry, error) {
	entry := Entry{Key: object.Key, Size: object.Size, Checksum: hex.EncodeToString(object.Checksum[:]), State: object.State}
	if object.MetadataOnly {
		if object.ChecksumAbsent {
			entry.Checksum = ""
		}
		entry.MetadataOnly = true
		verifier, ok := source.(interface {
			VerifyLegacyAbsence(context.Context, string) error
		})
		if !ok {
			return entry, errors.New("RESOURCE_SNAPSHOT_LEGACY_UNSUPPORTED")
		}
		return entry, verifier.VerifyLegacyAbsence(ctx, object.Key)
	}
	release, err := source.AcquireRead(ctx, object.Key)
	if err != nil {
		return entry, err
	}
	defer release()
	file, err := source.Open(ctx, object.Key, object.Size, object.Checksum)
	if errors.Is(err, local.ErrNotFound) {
		return entry, nil
	}
	if err != nil {
		return entry, err
	}
	// 本地目标也以独占发布/完整SHA确认，不能把中断的半份临时文件作为快照。
	putErr := target.Put(ctx, object.Key, file, object.Size, object.Checksum)
	closeErr := file.Close()
	if putErr != nil {
		return entry, putErr
	}
	if closeErr != nil {
		return entry, closeErr
	}
	entry.Present = true
	return entry, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/storage/snapshot/resources.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
