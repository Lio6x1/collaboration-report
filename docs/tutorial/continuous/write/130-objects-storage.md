# 分步编写 `services/document/internal/storage/local/objects_storage.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/130-objects-storage.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供对象跨进程锁、精确上传临时文件清理和可Seek的校验暂存文件。

完整校验再交付可防半份对象或错误摘要流入HTTP；锁必须覆盖SQL预检、IO及确认。

写之前先按这条执行路线理解各段的连接：

1. Acquire/AcquireRead分别取得排他/共享锁
2. Stage流式校验长度和摘要并回到起点
3. CleanupUpload只清理此key普通临时文件
4. Temporary.Close清理私有暂存。

本文件需要守住的边界：

- 锁文件永久保留，删后重建会出现两个不同inode
- 不遍历子目录或跟随链接。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/storage/local' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/storage/local/objects_storage.go') -and (Get-Item -LiteralPath 'services/document/internal/storage/local/objects_storage.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/storage/local/objects_storage.go')) { New-Item -ItemType File -Path 'services/document/internal/storage/local/objects_storage.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:289 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package local

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math"
	"os"
	"strings"
	"sync"
)

// guard.go：保留原实现，按业务合并文件。

````

## 第 2 步：Store.Acquire

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

Acquire 非阻塞获取同一资源的跨进程排他锁。调用方必须在 SQL 预检前获取，
并覆盖上传、清理（含 Clear）及数据库最终确认；Put/Delete/CleanupUpload 不重复加锁。
锁文件永久保留：删除后重建会产生不同 inode，导致两个进程同时认为自己持锁。
本适配器要求各进程共享支持文件锁的本地目录；OSS 也复用此目录，仍是单机部署。

本步声明：`func (s *Store) Acquire(ctx context.Context, key string) (func(), error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.acquire(ctx, key, true)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.acquire(ctx, key, true)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>func()</code> | 由 <code>s.acquire(ctx, key, true)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.acquire(ctx, key, true)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.acquire(ctx, key, true)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=289:846 -->
````go
// Acquire 非阻塞获取同一资源的跨进程排他锁。调用方必须在 SQL 预检前获取，
// 并覆盖上传、清理（含 Clear）及数据库最终确认；Put/Delete/CleanupUpload 不重复加锁。
// 锁文件永久保留：删除后重建会产生不同 inode，导致两个进程同时认为自己持锁。
// 本适配器要求各进程共享支持文件锁的本地目录；OSS 也复用此目录，仍是单机部署。
func (s *Store) Acquire(ctx context.Context, key string) (func(), error) {
	return s.acquire(ctx, key, true)
}

````

## 第 3 步：Store.AcquireRead

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

下载之间共享锁，删除/上传仍需排他锁；多个读者不能互相造成假冲突。

本步声明：`func (s *Store) AcquireRead(ctx context.Context, key string) (func(), error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.acquire(ctx, key, false)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.acquire(ctx, key, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>func()</code> | 由 <code>s.acquire(ctx, key, false)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.acquire(ctx, key, false)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.acquire(ctx, key, false)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=846:1064 -->
````go
// 下载之间共享锁，删除/上传仍需排他锁；多个读者不能互相造成假冲突。
func (s *Store) AcquireRead(ctx context.Context, key string) (func(), error) {
	return s.acquire(ctx, key, false)
}

````

## 第 4 步：Store.acquire

插入位置：第 3 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func (s *Store) acquire(ctx context.Context, key string, exclusive bool) (func(), error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>ctx.Err</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>&quot;.lock-&quot; + key</code> |
| <code>exclusive</code> | <code>bool</code> | 在此声明中的实际使用：<code>lockFile(file, exclusive)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>func()</code> | 实际返回表达式：<code>nil</code>；<code>release</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ErrIntegrity</code>；<code>ErrUnavailable</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>root.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>os.IsExist(err)</code>。
- 检查 <code>statErr != nil &#124;&#124; !info.Mode().IsRegular()</code>。条件成立时执行 <code>return nil, ErrIntegrity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; pathErr != nil &#124;&#124; !opened.Mode().IsRegular() &#124;&#124; !current.Mode().IsRegular() &#124;&#124; !os.SameFile(opened, current)</code>。
- 此处通过 <code>return nil, ErrIntegrity</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return release, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1064:2111 -->
````go
func (s *Store) acquire(ctx context.Context, key string, exclusive bool) (func(), error) {
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	name := ".lock-" + key
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if os.IsExist(err) {
		info, statErr := root.Lstat(name)
		if statErr != nil || !info.Mode().IsRegular() {
			return nil, ErrIntegrity
		}
		file, err = root.OpenFile(name, os.O_RDWR, 0600)
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	opened, err := file.Stat()
	current, pathErr := root.Lstat(name)
	if err != nil || pathErr != nil || !opened.Mode().IsRegular() || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		file.Close()
		return nil, ErrIntegrity
	}
	if err := lockFile(file, exclusive); err != nil {
		file.Close()
		return nil, err
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			unlockFile(file)
			file.Close()
		})
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

````

## 第 5 步：Store.CleanupUpload

插入位置：第 4 步代码之后，紧接追加；完成稿约第 74 行。

CleanupUpload 要求调用方持有 Acquire 锁，仅清理精确属于该键的普通临时文件。
不遍历子目录、不跟随符号链接，也不触碰正式文件、锁文件或其他资源的临时文件。

本步声明：`func (s *Store) CleanupUpload(ctx context.Context, key string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>ctx.Err</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>&quot;.upload-&quot; + key</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ErrUnavailable</code>；<code>ErrIntegrity</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>root.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 登记 <code>directory.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!strings.HasPrefix(name, prefix)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>len(nonce) != 32 &#124;&#124; strings.ToLower(nonce) != nonce</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>os.IsNotExist(err)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>!info.Mode().IsRegular()</code>。条件成立时执行 <code>return ErrIntegrity</code>。当前函数在这里结束。
- 检查 <code>err != nil &amp;&amp; !os.IsNotExist(err)</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=2111:3346 -->
````go
// CleanupUpload 要求调用方持有 Acquire 锁，仅清理精确属于该键的普通临时文件。
// 不遍历子目录、不跟随符号链接，也不触碰正式文件、锁文件或其他资源的临时文件。
func (s *Store) CleanupUpload(ctx context.Context, key string) error {
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return err
	}
	defer root.Close()
	directory, err := root.Open(".")
	if err != nil {
		return ErrUnavailable
	}
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return ErrUnavailable
	}
	prefix := ".upload-" + key + "-"
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		nonce := strings.TrimPrefix(name, prefix)
		if len(nonce) != 32 || strings.ToLower(nonce) != nonce {
			continue
		}
		if _, err := hex.DecodeString(nonce); err != nil {
			continue
		}
		info, err := root.Lstat(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return ErrUnavailable
		}
		if !info.Mode().IsRegular() {
			return ErrIntegrity
		}
		if err := root.Remove(name); err != nil && !os.IsNotExist(err) {
			return ErrUnavailable
		}
	}
````

## 第 6 步：Store.CleanupUpload（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 121 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3346:3423 -->
````go
	return nil
}

// temporary.go：保留原实现，按业务合并文件。

````

## 第 7 步：Temporary

插入位置：第 6 步代码之后，紧接追加；完成稿约第 126 行。

Temporary 是已经完整校验并回到起点的私有临时文件。OSS 使用可 Seek 的文件
上传，读取也先落盘校验再输出，避免把半份或摘要错误的云对象交给 HTTP。
创建/关闭期间调用者持有对应资源锁；进程退出后的残留沿 CleanupUpload 清理。

本步声明：`type Temporary struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>os.File</code> | <code>*os.File</code> | 嵌入的类型/接口，组合其字段或方法集合。  |
| <code>root</code> | <code>*os.Root</code> | 已打开且被限制在专用存储根的os.Root句柄，临时操作相对它执行。 |
| <code>name</code> | <code>string</code> | 本次临时文件的相对名称。 |
| <code>once</code> | <code>sync.Once</code> | 保证临时文件收尾最多执行一次，避免重复关闭/删除。 |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。 |

<!-- write-step: 7 bytes=3423:3830 -->
````go
// Temporary 是已经完整校验并回到起点的私有临时文件。OSS 使用可 Seek 的文件
// 上传，读取也先落盘校验再输出，避免把半份或摘要错误的云对象交给 HTTP。
// 创建/关闭期间调用者持有对应资源锁；进程退出后的残留沿 CleanupUpload 清理。
type Temporary struct {
	*os.File
	root *os.Root
	name string
	once sync.Once
	err  error
}

````

## 第 8 步：Store.Stage

插入位置：第 7 步代码之后，紧接追加；完成稿约第 137 行。

本步声明：`func (s *Store) Stage(ctx context.Context, key string, body io.Reader, size uint64, digest [32]byte) (*Temporary, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>io.LimitReader(contextReader{ctx, body}, int64(size)+1)</code>；<code>readError(ctx)</code>；<code>ctx.Err</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.openRoot(ctx, key)</code>；<code>&quot;.upload-&quot; + key</code> |
| <code>body</code> | <code>io.Reader</code> | 按需读取字节的输入流，不保证一次Read就拿到完整内容；调用者负责期限、体积限制和最终完整性核验。<br>在此声明中的实际使用：<code>body == nil</code>；<code>io.LimitReader(contextReader{ctx, body}, int64(size)+1)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>size &gt; 32&lt;&lt;20</code>；<code>size &gt;= math.MaxInt64</code>；<code>int64(size)</code>；<code>uint64(count) != size</code> |
| <code>digest</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>bytes.Equal(hash.Sum(nil), digest&#91;:&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Temporary</code> | 使用 <code>Temporary</code> 类型；其定义处的职责：Temporary 是已经完整校验并回到起点的私有临时文件。OSS 使用可 Seek 的文件 上传，读取也先落盘校验再输出，避免把半份或摘要错误的云对象交给 HTTP。 创建/关闭期间调用者持有对应资源锁；进程退出后的残留沿 CleanupUpload 清理。<br>实际返回表达式：<code>nil</code>；<code>temporary</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrInvalidSize</code>；<code>err</code>；<code>ErrUnavailable</code>；<code>readError(ctx)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>body == nil &#124;&#124; size &gt; 32&lt;&lt;20 &#124;&#124; size &gt;= math.MaxInt64</code>。条件成立时执行 <code>return nil, ErrInvalidSize</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return nil, ErrUnavailable</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return nil, ErrUnavailable</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 登记 <code>func() { 		if !valid { 			temporary.Close() 		} 	}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>!valid</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, readError(ctx)</code>。当前函数在这里结束。
- 检查 <code>uint64(count) != size &#124;&#124; !bytes.Equal(hash.Sum(nil), digest&#91;:&#93;)</code>。条件成立时执行 <code>return nil, ErrIntegrity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>file.Sync() != nil</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrUnavailable</code>。当前函数在这里结束。
- 此处通过 <code>return temporary, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3830:5089 -->
````go
func (s *Store) Stage(ctx context.Context, key string, body io.Reader, size uint64, digest [32]byte) (*Temporary, error) {
	if body == nil || size > 32<<20 || size >= math.MaxInt64 {
		return nil, ErrInvalidSize
	}
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return nil, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		root.Close()
		return nil, ErrUnavailable
	}
	name := ".upload-" + key + "-" + hex.EncodeToString(nonce[:])
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		root.Close()
		return nil, ErrUnavailable
	}
	temporary := &Temporary{File: file, root: root, name: name}
	valid := false
	defer func() {
		if !valid {
			temporary.Close()
		}
	}()
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(contextReader{ctx, body}, int64(size)+1))
	if err != nil {
		return nil, readError(ctx)
	}
	if uint64(count) != size || !bytes.Equal(hash.Sum(nil), digest[:]) {
		return nil, ErrIntegrity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if file.Sync() != nil {
		return nil, ErrUnavailable
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, ErrUnavailable
	}
	valid = true
	return temporary, nil
}

````

## 第 9 步：Temporary.Close

插入位置：第 8 步代码之后，紧接追加；完成稿约第 184 行。

本步声明：`func (t *Temporary) Close() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>t.err</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>t.File.Close() != nil</code>。
- 在本段的匿名函数内：检查 <code>statErr != nil &#124;&#124; pathErr != nil &#124;&#124; !current.Mode().IsRegular() &#124;&#124; !os.SameFile(opened, current)</code>。
- 在本段的匿名函数内：检查 <code>err != nil &amp;&amp; !os.IsNotExist(err)</code>。
- 此处通过 <code>return t.err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5089:5658 -->
````go
func (t *Temporary) Close() error {
	t.once.Do(func() {
		opened, statErr := t.File.Stat()
		current, pathErr := t.root.Lstat(t.name)
		if t.File.Close() != nil {
			t.err = ErrUnavailable
		}
		// 不删除被替换成目录/链接或另一个文件的路径。未知清理留待原键重试。
		if statErr != nil || pathErr != nil || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
			t.err = ErrIntegrity
		} else if err := t.root.Remove(t.name); err != nil && !os.IsNotExist(err) {
			t.err = ErrUnavailable
		}
		t.root.Close()
	})
	return t.err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/storage/local/objects_storage.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
