# 分步编写 `packages/platform/rebuildmeta/entry.go`

先按 [F05 · 重建元数据、签名与原子制品](../lessons/foundation/05-rebuild-artifacts.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/027-entry.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

流式编码和校验Doc交给Search的重建元数据行。

大快照只需保留一行内存；摘要与计数只有完整流验证后才可作为封存事实。

写之前先按这条执行路线理解各段的连接：

1. Entry.Line固定字段顺序与LF，ID/seq用字符串，无正式版用null。
2. Scan要求文档ID严格升序、行字节与Line完全相同，逐行visit并累积SHA256。

本文件需要守住的边界：

- 最后一行缺LF、重复/逆序ID或非法release都失败；visit写入的部分结果在整流成功前不能提交。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform/rebuildmeta' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/rebuildmeta/entry.go') -and (Get-Item -LiteralPath 'packages/platform/rebuildmeta/entry.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/rebuildmeta/entry.go')) { New-Item -ItemType File -Path 'packages/platform/rebuildmeta/entry.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:207 -->
````go
// Package rebuildmeta定义两服务交接的元数据行，不包含正文或访问凭据。
package rebuildmeta

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"strconv"
)

````

## 第 2 步：Entry

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`type Entry struct{ Document, Minimum, Release uint64 }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>uint64</code> | 快照文档ID，必须非零且全流递增。 |
| <code>Minimum</code> | <code>uint64</code> | 该文档要求追到的最小索引事件序号。 |
| <code>Release</code> | <code>uint64</code> | 正式版提示ID，0在NDJSON编码为null。 |

<!-- write-step: 2 bytes=207:263 -->
````go
type Entry struct{ Document, Minimum, Release uint64 }

````

## 第 3 步：Entry.Line

插入位置：第 2 步代码之后，紧接追加；完成稿约第 16 行。

Line使用规范十进制字符串保留全uint64；无正式指针用null，不用字符串"0"冒充引用。

本步声明：`func (e Entry) Line() ([]byte, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>&#91;&#93;byte(`{&quot;document_id&quot;:&quot;` + strconv.FormatUint(e.Document, 10) + `&quot;,&quot;minimum_seq&quot;:&quot;` + strconv.FormatUint(e.Minimum, 10) + `&quot;,&quot;release_hint&quot;:` + release + &quot;}\n&quot;)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;REBUILD_ENTRY_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>e.Document == 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_ENTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>e.Release != 0</code>。
- 此处通过 <code>return &#91;&#93;byte(`{&quot;document_id&quot;:&quot;` + strconv.FormatUint(e.Document, 10) + `&quot;,&quot;minimum_seq&quot;:&quot;` + strconv.FormatUint(e.Minimum, 10) + `&quot;,&quot;release_hint&quot;:` + release + &quot;}\n&quot;), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=263:774 -->
````go
// Line使用规范十进制字符串保留全uint64；无正式指针用null，不用字符串"0"冒充引用。
func (e Entry) Line() ([]byte, error) {
	if e.Document == 0 {
		return nil, errors.New("REBUILD_ENTRY_INVALID")
	}
	release := "null"
	if e.Release != 0 {
		release = strconv.Quote(strconv.FormatUint(e.Release, 10))
	}
	return []byte(`{"document_id":"` + strconv.FormatUint(e.Document, 10) + `","minimum_seq":"` + strconv.FormatUint(e.Minimum, 10) + `","release_hint":` + release + "}\n"), nil
}

````

## 第 4 步：Scan

插入位置：第 3 步代码之后，紧接追加；完成稿约第 28 行。

Scan只保留一行，严格检查升序、规范原字节和完整LF尾部。回调可写入调用者自己的事务，
返回的摘要/总数只有整个流成功才有效，调用者不能在封存前提交部分导入。

本步声明：`func Scan(reader io.Reader, visit func(Entry) error) (uint64, [32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>reader</code> | <code>io.Reader</code> | 按需读取字节的输入流，不保证一次Read就拿到完整内容；调用者负责期限、体积限制和最终完整性核验。<br>在此声明中的实际使用：<code>io.TeeReader(reader, hash)</code> |
| <code>visit</code> | <code>func(Entry) error</code> | 在此声明中的实际使用：<code>visit != nil</code>；<code>visit(entry)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>fail()</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>0</code>；<code>count</code> |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 由 <code>fail()</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>&#91;32&#93;byte{}</code>；<code>digest</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>fail()</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return 0, &#91;32&#93;byte{}, errors.New(&quot;REBUILD_ENTRIES_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err == io.EOF &amp;&amp; len(line) == 0</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(line, &amp;wire) != nil</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; document &lt;= previous</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>wire.Release != nil</code>。
- 检查 <code>err != nil &#124;&#124; release == 0</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(canonical, line) &#124;&#124; count == ^uint64(0)</code>。条件成立时执行 <code>return fail()</code>。当前函数在这里结束。
- 检查 <code>visit != nil</code>。

<!-- write-step: 4 bytes=774:2328 -->
````go
// Scan只保留一行，严格检查升序、规范原字节和完整LF尾部。回调可写入调用者自己的事务，
// 返回的摘要/总数只有整个流成功才有效，调用者不能在封存前提交部分导入。
func Scan(reader io.Reader, visit func(Entry) error) (uint64, [32]byte, error) {
	hash := sha256.New()
	input := bufio.NewReaderSize(io.TeeReader(reader, hash), 4096)
	var count, previous uint64
	fail := func() (uint64, [32]byte, error) { return 0, [32]byte{}, errors.New("REBUILD_ENTRIES_INVALID") }
	for {
		line, err := input.ReadSlice('\n')
		if err == io.EOF && len(line) == 0 {
			break
		}
		if err != nil {
			return fail()
		}
		var wire struct {
			Document string  `json:"document_id"`
			Minimum  string  `json:"minimum_seq"`
			Release  *string `json:"release_hint"`
		}
		if json.Unmarshal(line, &wire) != nil {
			return fail()
		}
		document, err := strconv.ParseUint(wire.Document, 10, 64)
		if err != nil || document <= previous {
			return fail()
		}
		minimum, err := strconv.ParseUint(wire.Minimum, 10, 64)
		if err != nil {
			return fail()
		}
		var release uint64
		if wire.Release != nil {
			release, err = strconv.ParseUint(*wire.Release, 10, 64)
			if err != nil || release == 0 {
				return fail()
			}
		}
		entry := Entry{document, minimum, release}
		canonical, err := entry.Line()
		// 原字节比较同时拒绝重复/未知字段、非规范数字、重排和缺字段。
		if err != nil || !bytes.Equal(canonical, line) || count == ^uint64(0) {
			return fail()
		}
		if visit != nil {
````

## 第 5 步：Scan（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 73 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, &#91;32&#93;byte{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return count, digest, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2328:2530 -->
````go
			if err := visit(entry); err != nil {
				return 0, [32]byte{}, err
			}
		}
		previous, count = document, count+1
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return count, digest, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/rebuildmeta/entry.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/05-rebuild-artifacts.md)。
