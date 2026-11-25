# 分步编写 `services/document/internal/storage/local/backend.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/126-backend.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在资源目录固定后端身份，并为冷备份验证旧元数据对象确实不存在。

更换Bucket/prefix或退回本地会让SQL key失去实际对象，应在启动时阻止静默切换。

写之前先按这条执行路线理解各段的连接：

1. BindBackend持久非秘密地址身份
2. RequireLocalBackend核本地模式
3. VerifyLegacyAbsence只检查旧键不存在。

本文件需要守住的边界：

- 首次接OSS允许读取原目录旧对象，不擅自搬迁
- 旧元数据缺摘要不能伪造摘要进入普通对象路径。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/storage/local' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/storage/local/backend.go') -and (Get-Item -LiteralPath 'services/document/internal/storage/local/backend.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/storage/local/backend.go')) { New-Item -ItemType File -Path 'services/document/internal/storage/local/backend.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:102 -->
````go
package local

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"regexp"
)

````

## 第 2 步：legacySegment

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`var legacySegment = regexp.MustCompile(\`^[A-Za-z0-9_-]{1,160}$\`)`。

<!-- write-step: 2 bytes=102:168 -->
````go
var legacySegment = regexp.MustCompile(`^[A-Za-z0-9_-]{1,160}$`)

````

## 第 3 步：Store.VerifyLegacyAbsence

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

VerifyLegacyAbsence 只用于全部写者已退出的备份边界。旧实验元数据可能
没有合法业务 key/摘要；先证明目录内确实不存在，不能伪造摘要后当普通对象。

本步声明：`func (s *Store) VerifyLegacyAbsence(ctx context.Context, key string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.openRoot(ctx, backendKey)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>legacySegment.MatchString(key)</code>；<code>root.Lstat(key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrInvalidKey</code>；<code>err</code>；<code>nil</code>；<code>ErrUnavailable</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!legacySegment.MatchString(key)</code>。条件成立时执行 <code>return ErrInvalidKey</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>root.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>os.IsNotExist(err)</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 此处通过 <code>return ErrIntegrity</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=168:746 -->
````go
// VerifyLegacyAbsence 只用于全部写者已退出的备份边界。旧实验元数据可能
// 没有合法业务 key/摘要；先证明目录内确实不存在，不能伪造摘要后当普通对象。
func (s *Store) VerifyLegacyAbsence(ctx context.Context, key string) error {
	if !legacySegment.MatchString(key) {
		return ErrInvalidKey
	}
	root, err := s.openRoot(ctx, backendKey)
	if err != nil {
		return err
	}
	defer root.Close()
	_, err = root.Lstat(key)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return ErrUnavailable
	}
	return ErrIntegrity
}

````

## 第 4 步：backendKey

插入位置：第 3 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`var backendKey = func() string {`。

<!-- write-step: 4 bytes=746:891 -->
````go
var backendKey = func() string {
	digest := sha256.Sum256([]byte("RESOURCE_STORAGE_BACKEND_BINDING"))
	return hex.EncodeToString(digest[:])
}()

````

## 第 5 步：Store.BindBackend

插入位置：第 4 步代码之后，紧接追加；完成稿约第 41 行。

BindBackend 把非秘密的对象地址固定在原对象目录。若换 Bucket/prefix 或退回
本地模式，已有 SQL key 会失去实际对象；启动时拒绝这类静默切换。
初次接入 OSS 可继续读取同一目录的旧本地对象，不搬动或覆盖已有数据。

本步声明：`func (s *Store) BindBackend(ctx context.Context, identity []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Acquire(ctx, backendKey)</code>；<code>s.openRoot(ctx, backendKey)</code>；<code>s.Stage(ctx, backendKey, bytes.NewReader(identity), uint64(len(identity)), digest)</code> |
| <code>identity</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(identity)</code>；<code>bytes.Equal(current, identity)</code>；<code>sha256.Sum256(identity)</code>；<code>bytes.NewReader(identity)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrInvalidSize</code>；<code>err</code>；<code>ErrConflict</code>；<code>nil</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>len(identity) == 0 &#124;&#124; len(identity) &gt; 4096</code>。条件成立时执行 <code>return ErrInvalidSize</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>release()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>root.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err == nil</code>。
- 检查 <code>!bytes.Equal(current, identity)</code>。条件成立时执行 <code>return ErrConflict</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != ErrNotFound</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>file.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>root.Link(file.name, &quot;.oss-binding&quot;) != nil</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=891:2073 -->
````go
// BindBackend 把非秘密的对象地址固定在原对象目录。若换 Bucket/prefix 或退回
// 本地模式，已有 SQL key 会失去实际对象；启动时拒绝这类静默切换。
// 初次接入 OSS 可继续读取同一目录的旧本地对象，不搬动或覆盖已有数据。
func (s *Store) BindBackend(ctx context.Context, identity []byte) error {
	if len(identity) == 0 || len(identity) > 4096 {
		return ErrInvalidSize
	}
	release, err := s.Acquire(ctx, backendKey)
	if err != nil {
		return err
	}
	defer release()
	root, err := s.openRoot(ctx, backendKey)
	if err != nil {
		return err
	}
	defer root.Close()
	if current, err := readBackend(root); err == nil {
		if !bytes.Equal(current, identity) {
			return ErrConflict
		}
		return nil
	} else if err != ErrNotFound {
		return err
	}
	digest := sha256.Sum256(identity)
	file, err := s.Stage(ctx, backendKey, bytes.NewReader(identity), uint64(len(identity)), digest)
	if err != nil {
		return err
	}
	defer file.Close()
	// 完整文件写入与 Sync 后再独占发布；崩溃不会留下半份配置绑定。
	if root.Link(file.name, ".oss-binding") != nil {
		return ErrUnavailable
	}
	return nil
}

````

## 第 6 步：Store.RequireLocalBackend

插入位置：第 5 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func (s *Store) RequireLocalBackend(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.openRoot(ctx, backendKey)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>ErrConflict</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>root.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err == ErrNotFound</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return ErrConflict</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2073:2358 -->
````go
func (s *Store) RequireLocalBackend(ctx context.Context) error {
	root, err := s.openRoot(ctx, backendKey)
	if err != nil {
		return err
	}
	defer root.Close()
	_, err = readBackend(root)
	if err == ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrConflict
}

````

## 第 7 步：readBackend

插入位置：第 6 步代码之后，紧接追加；完成稿约第 95 行。

本步声明：`func readBackend(root *os.Root) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>root</code> | <code>*os.Root</code> | 在此声明中的实际使用：<code>root.Lstat</code>；<code>root.Open</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>raw</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrNotFound</code>；<code>ErrUnavailable</code>；<code>ErrIntegrity</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>os.IsNotExist(err)</code>。条件成立时执行 <code>return nil, ErrNotFound</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>!info.Mode().IsRegular() &#124;&#124; info.Size() &gt; 4096</code>。条件成立时执行 <code>return nil, ErrIntegrity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 登记 <code>file.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; !os.SameFile(info, opened)</code>。条件成立时执行 <code>return nil, ErrIntegrity</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(raw) == 0 &#124;&#124; len(raw) &gt; 4096</code>。条件成立时执行 <code>return nil, ErrIntegrity</code>。当前函数在这里结束。
- 此处通过 <code>return raw, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2358:3003 -->
````go
func readBackend(root *os.Root) ([]byte, error) {
	info, err := root.Lstat(".oss-binding")
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return nil, ErrIntegrity
	}
	file, err := root.Open(".oss-binding")
	if err != nil {
		return nil, ErrUnavailable
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, ErrIntegrity
	}
	raw, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(raw) == 0 || len(raw) > 4096 {
		return nil, ErrIntegrity
	}
	return raw, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/storage/local/backend.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
