# 分步编写 `services/document/internal/storage/oss/objects_storage.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/136-objects-storage.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在私有OSS中写入、读取和确认物理删除不可变资源。

PUT/DELETE响应丢失可能已执行，必须核验原key真实字节或NoSuchKey后才结算。

写之前先按这条执行路线理解各段的连接：

1. Put先本地Stage校验，再ForbidOverwrite上传并完整回读
2. Open只在明确NoSuchKey时回读旧本地对象
3. Delete以GET错误码证明缺失并删除旧本地副本。

本文件需要守住的边界：

- HEAD无正文404、NoSuchBucket或204不能单独证明可退款
- 版本化Bucket和delete marker不受支持。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/storage/oss' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/storage/oss/objects_storage.go') -and (Get-Item -LiteralPath 'services/document/internal/storage/oss/objects_storage.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/storage/oss/objects_storage.go')) { New-Item -ItemType File -Path 'services/document/internal/storage/oss/objects_storage.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:304 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package oss

import (
	"collaboration/document/internal/storage/local"
	"context"
	sdk "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"io"
)

// cleanup.go：保留原实现，按业务合并文件。

````

## 第 2 步：Store.Delete

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`func (s *Store) Delete(ctx context.Context, key string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.objectKey(ctx, key)</code>；<code>s.requireUnversioned(ctx)</code>；<code>s.client.DeleteObject(ctx, &amp;sdk.DeleteObjectRequest{Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(object)})</code>；<code>s.client.GetObject(ctx, &amp;sdk.GetObjectRequest{Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(object)})</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.objectKey(ctx, key)</code>；<code>s.local.Delete(ctx, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ErrVersioning</code>；<code>local.ErrUnavailable</code>；<code>s.local.Delete(ctx, key)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>deleteErr == nil &amp;&amp; (result == nil &#124;&#124; result.DeleteMarker &#124;&#124; (result.VersionId != nil &amp;&amp; *result.VersionId != &quot;&quot;))</code>。条件成立时执行 <code>return ErrVersioning</code>。当前函数在这里结束。
- 检查 <code>remaining != nil &amp;&amp; remaining.Body != nil</code>。
- 检查 <code>getErr == nil</code>。条件成立时执行 <code>return local.ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>err != local.ErrNotFound</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return s.local.Delete(ctx, key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=304:1586 -->
````go
func (s *Store) Delete(ctx context.Context, key string) error {
	object, err := s.objectKey(ctx, key)
	if err != nil {
		return err
	}
	if err := s.requireUnversioned(ctx); err != nil {
		return err
	}
	result, deleteErr := s.client.DeleteObject(ctx, &sdk.DeleteObjectRequest{Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(object)})
	if deleteErr == nil && (result == nil || result.DeleteMarker || (result.VersionId != nil && *result.VersionId != "")) {
		return ErrVersioning
	}
	// DELETE 的 204 或连接断开都不能单独结算退款。GET 的 XML 错误码能明确
	// 区分 NoSuchKey 与 NoSuchBucket；HEAD 的无正文 404 不足以做退款证明。
	// 未删除、GET 未知、Bucket 不存在/无权限都让 SQL 保持 DELETING。
	remaining, getErr := s.client.GetObject(ctx, &sdk.GetObjectRequest{Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(object)})
	if remaining != nil && remaining.Body != nil {
		remaining.Body.Close()
	}
	if getErr == nil {
		return local.ErrUnavailable
	}
	if err := storageError(ctx, getErr); err != local.ErrNotFound {
		return err
	}
	// 旧本地对象也必须真实删除后才允许退款；失败时保留原任务继续清理。
	return s.local.Delete(ctx, key)
}

// reading.go：保留原实现，按业务合并文件。

````

## 第 3 步：Store.remoteOpen

插入位置：第 2 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func (s *Store) remoteOpen(ctx context.Context, key string, size uint64, digest [32]byte) (io.ReadCloser, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.objectKey(ctx, key)</code>；<code>s.client.GetObject(ctx, &amp;sdk.GetObjectRequest{Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(object)})</code>；<code>storageError(ctx, err)</code>；<code>s.local.Stage(ctx, key, result.Body, size, digest)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.objectKey(ctx, key)</code>；<code>s.local.Stage(ctx, key, result.Body, size, digest)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>size &gt; 32&lt;&lt;20</code>；<code>uint64(result.ContentLength) != size</code>；<code>s.local.Stage(ctx, key, result.Body, size, digest)</code> |
| <code>digest</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>s.local.Stage(ctx, key, result.Body, size, digest)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>io.ReadCloser</code> | 实际返回表达式：<code>nil</code>；<code>file</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>local.ErrInvalidSize</code>；<code>storageError(ctx, err)</code>；<code>local.ErrUnavailable</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>size &gt; 32&lt;&lt;20</code>。条件成立时执行 <code>return nil, local.ErrInvalidSize</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, storageError(ctx, err)</code>。当前函数在这里结束。
- 检查 <code>result == nil &#124;&#124; result.Body == nil</code>。条件成立时执行 <code>return nil, local.ErrUnavailable</code>。当前函数在这里结束。
- 登记 <code>result.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>result.ContentLength &lt; 0 &#124;&#124; uint64(result.ContentLength) != size &#124;&#124; 		(result.VersionId != nil &amp;&amp; *result.VersionId != &quot;&quot;) &#124;&#124; result.ContentRange != nil</code>。条件成立时执行 <code>return nil, local.ErrIntegrity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return file, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1586:2437 -->
````go
func (s *Store) remoteOpen(ctx context.Context, key string, size uint64, digest [32]byte) (io.ReadCloser, error) {
	object, err := s.objectKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if size > 32<<20 {
		return nil, local.ErrInvalidSize
	}
	result, err := s.client.GetObject(ctx, &sdk.GetObjectRequest{Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(object)})
	if err != nil {
		return nil, storageError(ctx, err)
	}
	if result == nil || result.Body == nil {
		return nil, local.ErrUnavailable
	}
	defer result.Body.Close()
	if result.ContentLength < 0 || uint64(result.ContentLength) != size ||
		(result.VersionId != nil && *result.VersionId != "") || result.ContentRange != nil {
		return nil, local.ErrIntegrity
	}
	file, err := s.local.Stage(ctx, key, result.Body, size, digest)
	if err != nil {
		return nil, err
	}
	return file, nil
}

````

## 第 4 步：Store.Open

插入位置：第 3 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func (s *Store) Open(ctx context.Context, key string, size uint64, digest [32]byte) (io.ReadCloser, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.remoteOpen(ctx, key, size, digest)</code>；<code>s.local.Open(ctx, key, size, digest)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.remoteOpen(ctx, key, size, digest)</code>；<code>s.local.Open(ctx, key, size, digest)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.remoteOpen(ctx, key, size, digest)</code>；<code>s.local.Open(ctx, key, size, digest)</code> |
| <code>digest</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>s.remoteOpen(ctx, key, size, digest)</code>；<code>s.local.Open(ctx, key, size, digest)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>io.ReadCloser</code> | 实际返回表达式：<code>file</code>；<code>nil</code>；<code>legacy</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != local.ErrNotFound</code>。条件成立时执行 <code>return file, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return legacy, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2437:3033 -->
````go
func (s *Store) Open(ctx context.Context, key string, size uint64, digest [32]byte) (io.ReadCloser, error) {
	file, err := s.remoteOpen(ctx, key, size, digest)
	if err != local.ErrNotFound {
		return file, err
	}
	// 接入云存储前的已验收本地对象留在原目录，仅云端明确 NoSuchKey 才回读。
	// 云端超时、权限错误或损坏不能绕过为本地成功，也不主动搬运旧业务数据。
	legacy, err := s.local.Open(ctx, key, size, digest)
	if err != nil {
		return nil, err
	}
	return legacy, nil
}

// upload.go：保留原实现，按业务合并文件。

````

## 第 5 步：Store.Put

插入位置：第 4 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`func (s *Store) Put(ctx context.Context, key string, body io.Reader, size uint64, digest [32]byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.objectKey(ctx, key)</code>；<code>s.local.Stage(ctx, key, body, size, digest)</code>；<code>s.requireUnversioned(ctx)</code>；<code>s.remoteOpen(ctx, key, size, digest)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.objectKey(ctx, key)</code>；<code>s.local.Stage(ctx, key, body, size, digest)</code>；<code>s.remoteOpen(ctx, key, size, digest)</code> |
| <code>body</code> | <code>io.Reader</code> | 按需读取字节的输入流，不保证一次Read就拿到完整内容；调用者负责期限、体积限制和最终完整性核验。<br>在此声明中的实际使用：<code>s.local.Stage(ctx, key, body, size, digest)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.local.Stage(ctx, key, body, size, digest)</code>；<code>s.remoteOpen(ctx, key, size, digest)</code>；<code>int64(size)</code> |
| <code>digest</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>s.local.Stage(ctx, key, body, size, digest)</code>；<code>s.remoteOpen(ctx, key, size, digest)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>existing.Close()</code>；<code>local.ErrConflict</code>；<code>verified.Close()</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>file.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return existing.Close()</code>。当前函数在这里结束。
- 检查 <code>err == local.ErrIntegrity</code>。条件成立时执行 <code>return local.ErrConflict</code>。当前函数在这里结束。
- 检查 <code>err != local.ErrNotFound</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>verifyErr == nil</code>。条件成立时执行 <code>return verified.Close()</code>。当前函数在这里结束。
- 检查 <code>verifyErr == local.ErrIntegrity</code>。条件成立时执行 <code>return local.ErrConflict</code>。当前函数在这里结束。
- 检查 <code>putErr != nil</code>。条件成立时执行 <code>return storageError(ctx, putErr)</code>。当前函数在这里结束。
- 检查 <code>verifyErr == local.ErrNotFound</code>。条件成立时执行 <code>return local.ErrUnavailable</code>。当前函数在这里结束。
- 此处通过 <code>return verifyErr</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=3033:4525 -->
````go
func (s *Store) Put(ctx context.Context, key string, body io.Reader, size uint64, digest [32]byte) error {
	object, err := s.objectKey(ctx, key)
	if err != nil {
		return err
	}
	// 先完整检查客户端原流。即使对象已存在，错误的重试正文也不能被冒充成功。
	file, err := s.local.Stage(ctx, key, body, size, digest)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := s.requireUnversioned(ctx); err != nil {
		return err
	}
	existing, err := s.remoteOpen(ctx, key, size, digest)
	if err == nil {
		return existing.Close()
	}
	if err == local.ErrIntegrity {
		return local.ErrConflict
	}
	if err != local.ErrNotFound {
		return err
	}
	_, putErr := s.client.PutObject(ctx, &sdk.PutObjectRequest{
		Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(object), Body: file.File,
		ContentLength: sdk.Ptr(int64(size)), ForbidOverwrite: sdk.Ptr("true"),
		ContentType: sdk.Ptr("application/octet-stream"), Acl: sdk.ObjectACLPrivate,
	})
	// 无论成功、409 还是回复丢失，都核验同一个实际对象的完整长度/SHA。
	// 原 context 已结束时直接保留未知，由业务原号恢复，绝不另起宽期限。
	verified, verifyErr := s.remoteOpen(ctx, key, size, digest)
	if verifyErr == nil {
		return verified.Close()
	}
	if verifyErr == local.ErrIntegrity {
		return local.ErrConflict
	}
	if putErr != nil {
		return storageError(ctx, putErr)
	}
	if verifyErr == local.ErrNotFound {
		return local.ErrUnavailable
	}
	return verifyErr
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/storage/oss/objects_storage.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
