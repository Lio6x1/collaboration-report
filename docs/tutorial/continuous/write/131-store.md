# 分步编写 `services/document/internal/storage/local/store.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/131-store.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在专用本地目录实现不可变对象Put/Open/Delete。

对象key和长度/SHA来自SQL权威事实，底层存储只验证字节，不自行授予权限或决定配额。

写之前先按这条执行路线理解各段的连接：

1. New要求绝对路径并拒绝符号链接祖先
2. Put校验同步临时文件后发布
3. Open返回已验证且回到起点的句柄
4. Delete只删除精确普通文件。

本文件需要守住的边界：

- 同key不同内容必须冲突
- 删除缺失是幂等成功，不能递归删除
- 生产调用需外层Acquire覆盖最终SQL。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/storage/local' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/storage/local/store.go') -and (Get-Item -LiteralPath 'services/document/internal/storage/local/store.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/storage/local/store.go')) { New-Item -ItemType File -Path 'services/document/internal/storage/local/store.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:253 -->
````go
// Package local 实现可由对象存储替换的不可变资源存储；权限与配额由调用方负责。
package local

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
)

````

## 第 2 步：ErrInvalidKey、ErrInvalidSize、ErrIntegrity、ErrConflict、ErrNotFound、ErrUnavailable、ErrBusy

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

固定错误不包含主机路径，业务层可以稳定映射为对外错误码。

本步声明：`var (`。

<!-- write-step: 2 bytes=253:764 -->
````go
// 固定错误不包含主机路径，业务层可以稳定映射为对外错误码。
var (
	ErrInvalidKey  = errors.New("RESOURCE_STORAGE_INVALID_KEY")
	ErrInvalidSize = errors.New("RESOURCE_STORAGE_INVALID_SIZE")
	ErrIntegrity   = errors.New("RESOURCE_STORAGE_INTEGRITY")
	ErrConflict    = errors.New("RESOURCE_STORAGE_CONFLICT")
	ErrNotFound    = errors.New("RESOURCE_STORAGE_NOT_FOUND")
	ErrUnavailable = errors.New("RESOURCE_STORAGE_UNAVAILABLE")
	ErrBusy        = errors.New("RESOURCE_STORAGE_BUSY")
)

````

## 第 3 步：Store

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`type Store struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 经过验证的专用存储根绝对路径。 |
| <code>identity</code> | <code>os.FileInfo</code> | 创建Store时记录的根目录文件身份，后续重开时比较，防止目录被替换。 |

<!-- write-step: 3 bytes=764:826 -->
````go
type Store struct {
	path     string
	identity os.FileInfo
}

````

## 第 4 步：New

插入位置：第 3 步代码之后，紧接追加；完成稿约第 33 行。

New 仅接受专用绝对目录；拒绝任何现有符号链接祖先，避免配置绕过存储边界。

本步声明：`func New(root string) (*Store, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>root</code> | <code>string</code> | 在此声明中的实际使用：<code>filepath.IsAbs(root)</code>；<code>root = filepath.Clean(root)</code>；<code>filepath.Clean(root)</code>；<code>checkDirectories(root)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Store</code> | 实际返回表达式：<code>nil</code>；<code>&amp;Store{path: root, identity: info}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrUnavailable</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!filepath.IsAbs(root)</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Store{path: root, identity: info}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=826:1413 -->
````go
// New 仅接受专用绝对目录；拒绝任何现有符号链接祖先，避免配置绕过存储边界。
func New(root string) (*Store, error) {
	if !filepath.IsAbs(root) {
		return nil, ErrUnavailable
	}
	root = filepath.Clean(root)
	if err := checkDirectories(root); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, ErrUnavailable
	}
	if err := checkDirectories(root); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, ErrUnavailable
	}
	return &Store{path: root, identity: info}, nil
}

````

## 第 5 步：checkDirectories

插入位置：第 4 步代码之后，紧接追加；完成稿约第 55 行。

本步声明：`func checkDirectories(path string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>os.Lstat(path)</code>；<code>filepath.Dir(path)</code>；<code>parent == path</code>；<code>path = parent</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrUnavailable</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil &amp;&amp; (!info.IsDir() &#124;&#124; info.Mode()&amp;os.ModeSymlink != 0)</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>err != nil &amp;&amp; !os.IsNotExist(err)</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>parent == path</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=1413:1757 -->
````go
func checkDirectories(path string) error {
	for {
		info, err := os.Lstat(path)
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return ErrUnavailable
		}
		if err != nil && !os.IsNotExist(err) {
			return ErrUnavailable
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil
		}
		path = parent
	}
}

````

## 第 6 步：validKey

插入位置：第 5 步代码之后，紧接追加；完成稿约第 72 行。

本步声明：`func validKey(key string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>len(key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(key) != 64</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!(c &gt;= &#x27;0&#x27; &amp;&amp; c &lt;= &#x27;9&#x27; &#124;&#124; c &gt;= &#x27;a&#x27; &amp;&amp; c &lt;= &#x27;f&#x27;)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1757:1948 -->
````go
func validKey(key string) bool {
	if len(key) != 64 {
		return false
	}
	for _, c := range key {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

````

## 第 7 步：Store.openRoot

插入位置：第 6 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`func (s *Store) openRoot(ctx context.Context, key string) (*os.Root, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Err</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>validKey(key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*os.Root</code> | 实际返回表达式：<code>nil</code>；<code>root</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ErrInvalidKey</code>；<code>ErrUnavailable</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!validKey(key)</code>。条件成立时执行 <code>return nil, ErrInvalidKey</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !os.SameFile(s.identity, info)</code>。
- 此处通过 <code>return nil, ErrUnavailable</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return root, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1948:2436 -->
````go
func (s *Store) openRoot(ctx context.Context, key string) (*os.Root, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validKey(key) {
		return nil, ErrInvalidKey
	}
	if err := checkDirectories(s.path); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.path)
	if err != nil {
		return nil, ErrUnavailable
	}
	info, err := root.Stat(".")
	if err != nil || !os.SameFile(s.identity, info) {
		root.Close()
		return nil, ErrUnavailable
	}
	return root, nil
}

````

## 第 8 步：Store.Put

插入位置：第 7 步代码之后，紧接追加；完成稿约第 106 行。

Put 先流式验证并同步临时文件，再排他发布；同键重试必须内容完全相同。
生产调用方必须先 Acquire，并持有到数据库确认结束，避免清理与上传交错。

本步声明：`func (s *Store) Put(ctx context.Context, key string, input io.Reader, size uint64, checksum [32]byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>verifiedOpen(ctx, root, key, size, checksum)</code>；<code>io.LimitReader(contextReader{ctx, input}, int64(size)+1)</code>；<code>readError(ctx)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>root.Lstat(key)</code>；<code>verifiedOpen(ctx, root, key, size, checksum)</code>；<code>&quot;.upload-&quot; + key</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>io.Reader</code> | 按需读取字节的输入流，不保证一次Read就拿到完整内容；调用者负责期限、体积限制和最终完整性核验。<br>在此声明中的实际使用：<code>input == nil</code>；<code>io.LimitReader(contextReader{ctx, input}, int64(size)+1)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>size &gt;= math.MaxInt64</code>；<code>verifiedOpen(ctx, root, key, size, checksum)</code>；<code>int64(size)</code>；<code>uint64(count) != size</code> |
| <code>checksum</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>verifiedOpen(ctx, root, key, size, checksum)</code>；<code>bytes.Equal(hash.Sum(nil), checksum&#91;:&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrInvalidSize</code>；<code>err</code>；<code>ErrConflict</code>；<code>ErrUnavailable</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>size &gt;= math.MaxInt64 &#124;&#124; input == nil</code>。条件成立时执行 <code>return ErrInvalidSize</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>root.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err == nil</code>。
- 检查 <code>errors.Is(err, ErrIntegrity)</code>。条件成立时执行 <code>return ErrConflict</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return readError(ctx)</code>。当前函数在这里结束。
- 检查 <code>uint64(count) != size &#124;&#124; !bytes.Equal(hash.Sum(nil), checksum&#91;:&#93;)</code>。条件成立时执行 <code>return ErrIntegrity</code>。当前函数在这里结束。
- 此处通过 <code>return ctx.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!os.IsNotExist(err)</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=2436:4012 -->
````go
// Put 先流式验证并同步临时文件，再排他发布；同键重试必须内容完全相同。
// 生产调用方必须先 Acquire，并持有到数据库确认结束，避免清理与上传交错。
func (s *Store) Put(ctx context.Context, key string, input io.Reader, size uint64, checksum [32]byte) error {
	if size >= math.MaxInt64 || input == nil {
		return ErrInvalidSize
	}
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := root.Lstat(key); err == nil {
		// 原PUT可能已成功但响应丢失；先核对不可变对象，再校验重放正文，
		// 不为同一预留额另写完整临时副本。生产调用方持Acquire锁覆盖此路径。
		existing, err := verifiedOpen(ctx, root, key, size, checksum)
		if errors.Is(err, ErrIntegrity) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		if err := existing.Close(); err != nil {
			return ErrUnavailable
		}
		hash := sha256.New()
		count, err := io.Copy(hash, io.LimitReader(contextReader{ctx, input}, int64(size)+1))
		if err != nil {
			return readError(ctx)
		}
		if uint64(count) != size || !bytes.Equal(hash.Sum(nil), checksum[:]) {
			return ErrIntegrity
		}
		return ctx.Err()
	} else if !os.IsNotExist(err) {
		return ErrUnavailable
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return ErrUnavailable
	}
	temporary := ".upload-" + key + "-" + hex.EncodeToString(nonce[:])
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ErrUnavailable
	}
````

## 第 9 步：Store.Put（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 151 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 登记 <code>root.Remove(temporary)</code>，当前函数退出时执行，提前 return 也会触发。
- 登记 <code>file.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return readError(ctx)</code>。当前函数在这里结束。
- 检查 <code>uint64(count) != size &#124;&#124; !bytes.Equal(hash.Sum(nil), checksum&#91;:&#93;)</code>。条件成立时执行 <code>return ErrIntegrity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.Is(err, ErrIntegrity)</code>。条件成立时执行 <code>return ErrConflict</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, ErrNotFound)</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4012:4991 -->
````go
	defer root.Remove(temporary)
	defer file.Close()
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(contextReader{ctx, input}, int64(size)+1))
	if err != nil {
		return readError(ctx)
	}
	if uint64(count) != size || !bytes.Equal(hash.Sum(nil), checksum[:]) {
		return ErrIntegrity
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return ErrUnavailable
	}
	if err := file.Close(); err != nil {
		return ErrUnavailable
	}
	// Rename 在部分系统会覆盖旧文件；硬链接创建在 Windows/Unix 都保留排他语义。
	if err := root.Link(temporary, key); err == nil {
		return nil
	}
	existing, err := verifiedOpen(ctx, root, key, size, checksum)
	if err != nil {
		if errors.Is(err, ErrIntegrity) {
			return ErrConflict
		}
		if errors.Is(err, ErrNotFound) {
			return ErrUnavailable
		}
		return err
	}
	if err := existing.Close(); err != nil {
		return ErrUnavailable
	}
	return nil
}

````

## 第 10 步：Store.Open

插入位置：第 9 步代码之后，紧接追加；完成稿约第 190 行。

Open 返回已校验且回到起点的文件句柄，不将完整资源载入内存。

本步声明：`func (s *Store) Open(ctx context.Context, key string, size uint64, checksum [32]byte) (io.ReadCloser, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>verifiedOpen(ctx, root, key, size, checksum)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>verifiedOpen(ctx, root, key, size, checksum)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>size &gt;= math.MaxInt64</code>；<code>verifiedOpen(ctx, root, key, size, checksum)</code> |
| <code>checksum</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>verifiedOpen(ctx, root, key, size, checksum)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>io.ReadCloser</code> | 由 <code>verifiedOpen(ctx, root, key, size, checksum)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>verifiedOpen(ctx, root, key, size, checksum)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>ErrInvalidSize</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>size &gt;= math.MaxInt64</code>。条件成立时执行 <code>return nil, ErrInvalidSize</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>root.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return verifiedOpen(ctx, root, key, size, checksum)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4991:5401 -->
````go
// Open 返回已校验且回到起点的文件句柄，不将完整资源载入内存。
func (s *Store) Open(ctx context.Context, key string, size uint64, checksum [32]byte) (io.ReadCloser, error) {
	if size >= math.MaxInt64 {
		return nil, ErrInvalidSize
	}
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return verifiedOpen(ctx, root, key, size, checksum)
}

````

## 第 11 步：verifiedOpen

插入位置：第 10 步代码之后，紧接追加；完成稿约第 203 行。

本步声明：`func verifiedOpen(ctx context.Context, root *os.Root, key string, size uint64, checksum [32]byte) (*os.File, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>io.LimitReader(contextReader{ctx, file}, int64(size)+1)</code>；<code>readError(ctx)</code>；<code>ctx.Err</code> |
| <code>root</code> | <code>*os.Root</code> | 在此声明中的实际使用：<code>root.Lstat</code>；<code>root.Open</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>root.Lstat(key)</code>；<code>root.Open(key)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>uint64(opened.Size()) != size</code>；<code>int64(size)</code>；<code>uint64(count) != size</code> |
| <code>checksum</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>bytes.Equal(hash.Sum(nil), checksum&#91;:&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*os.File</code> | 实际返回表达式：<code>nil</code>；<code>file</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrNotFound</code>；<code>ErrUnavailable</code>；<code>ErrIntegrity</code>；<code>readError(ctx)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>os.IsNotExist(err)</code>。条件成立时执行 <code>return nil, ErrNotFound</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>!info.Mode().IsRegular()</code>。条件成立时执行 <code>return nil, ErrIntegrity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 登记 <code>func() { 		if !valid { 			file.Close() 		} 	}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>!valid</code>。
- 检查 <code>err != nil &#124;&#124; !opened.Mode().IsRegular() &#124;&#124; !os.SameFile(info, opened) &#124;&#124; opened.Size() &lt; 0 &#124;&#124; uint64(opened.Size()) != size</code>。条件成立时执行 <code>return nil, ErrIntegrity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, readError(ctx)</code>。当前函数在这里结束。
- 检查 <code>uint64(count) != size &#124;&#124; !bytes.Equal(hash.Sum(nil), checksum&#91;:&#93;)</code>。条件成立时执行 <code>return nil, ErrIntegrity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 此处通过 <code>return file, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5401:6488 -->
````go
func verifiedOpen(ctx context.Context, root *os.Root, key string, size uint64, checksum [32]byte) (*os.File, error) {
	info, err := root.Lstat(key)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	if !info.Mode().IsRegular() {
		return nil, ErrIntegrity
	}
	file, err := root.Open(key)
	if err != nil {
		return nil, ErrUnavailable
	}
	valid := false
	defer func() {
		if !valid {
			file.Close()
		}
	}()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) || opened.Size() < 0 || uint64(opened.Size()) != size {
		return nil, ErrIntegrity
	}
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(contextReader{ctx, file}, int64(size)+1))
	if err != nil {
		return nil, readError(ctx)
	}
	if uint64(count) != size || !bytes.Equal(hash.Sum(nil), checksum[:]) {
		return nil, ErrIntegrity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, ErrUnavailable
	}
	valid = true
	return file, nil
}

````

## 第 12 步：Store.Delete

插入位置：第 11 步代码之后，紧接追加；完成稿约第 246 行。

Delete 只删除精确的普通文件名，缺失视为幂等成功，绝不递归删除目录。
生产调用方必须持有 Acquire 返回的锁，覆盖数据库预检、删除与最终确认。

本步声明：`func (s *Store) Delete(ctx context.Context, key string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>ctx.Err</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>root.Lstat(key)</code>；<code>root.Remove(key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>ErrUnavailable</code>；<code>ErrIntegrity</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>root.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>os.IsNotExist(err)</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>!info.Mode().IsRegular()</code>。条件成立时执行 <code>return ErrIntegrity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &amp;&amp; !os.IsNotExist(err)</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6488:7173 -->
````go
// Delete 只删除精确的普通文件名，缺失视为幂等成功，绝不递归删除目录。
// 生产调用方必须持有 Acquire 返回的锁，覆盖数据库预检、删除与最终确认。
func (s *Store) Delete(ctx context.Context, key string) error {
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(key)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return ErrUnavailable
	}
	if !info.Mode().IsRegular() {
		return ErrIntegrity
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := root.Remove(key); err != nil && !os.IsNotExist(err) {
		return ErrUnavailable
	}
	return nil
}

````

## 第 13 步：contextReader

插入位置：第 12 步代码之后，紧接追加；完成稿约第 273 行。

本步声明：`type contextReader struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>reader</code> | <code>io.Reader</code> | 按需读取字节的输入流，不保证一次Read就拿到完整内容；调用者负责期限、体积限制和最终完整性核验。 |

<!-- write-step: 13 bytes=7173:7246 -->
````go
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

````

## 第 14 步：contextReader.Read

插入位置：第 13 步代码之后，紧接追加；完成稿约第 278 行。

本步声明：`func (r contextReader) Read(data []byte) (int, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>data</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>r.reader.Read(data)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 由 <code>r.reader.Read(data)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>0</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>r.reader.Read(data)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return r.reader.Read(data)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=7246:7389 -->
````go
func (r contextReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}

````

## 第 15 步：readError

插入位置：第 14 步代码之后，紧接追加；完成稿约第 285 行。

本步声明：`func readError(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ErrUnavailable</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return ErrUnavailable</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7389:7509 -->
````go
func readError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrUnavailable
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/storage/local/store.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
