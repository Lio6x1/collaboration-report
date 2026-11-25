# 分步编写 `packages/platform/rebuildartifact/files.go`

先按 [F05 · 重建元数据、签名与原子制品](../lessons/foundation/05-rebuild-artifacts.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/024-files.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按内容地址读写重建制品，保证发布前落盘并拒绝冲突覆盖。

读引用不能变成任意路径/URL读取；写制品要先完整落盘再公布，重复发布只能接受相同字节。

写之前先按这条执行路线理解各段的连接：

1. FileName只接受64位hex与prepare/snapshot扩展；ReadFile要求普通文件、大小受限并核实际内容地址。
2. PublishFile在可信目录创建临时文件，写入同时算SHA256，Sync/Close后Rename。
3. 目标已存在时核完整摘要一致才接受，不同内容返回FILE_CONFLICT。

本文件需要守住的边界：

- Reference只检查结构与地址，不代替验签；不能让符号链接目录/文件绕过受控inbox。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform/rebuildartifact' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/rebuildartifact/files.go') -and (Get-Item -LiteralPath 'packages/platform/rebuildartifact/files.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/rebuildartifact/files.go')) { New-Item -ItemType File -Path 'packages/platform/rebuildartifact/files.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:130 -->
````go
package rebuildartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

````

## 第 2 步：artifactFile

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`var artifactFile = regexp.MustCompile(\`^[0-9a-f]{64}\.(prepare\.json|snapshot\.json|entries\.ndjson)$\`)`。

<!-- write-step: 2 bytes=130:234 -->
````go
var artifactFile = regexp.MustCompile(`^[0-9a-f]{64}\.(prepare\.json|snapshot\.json|entries\.ndjson)$`)
````

## 第 3 步：fileFailure

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`var fileFailure = errors.New("REBUILD_ARTIFACT_FILE_FAILED")`。

<!-- write-step: 3 bytes=234:296 -->
````go
var fileFailure = errors.New("REBUILD_ARTIFACT_FILE_FAILED")

````

## 第 4 步：Reference

插入位置：第 3 步代码之后，紧接追加；完成稿约第 17 行。

Reference只计算结构合法制品的内容地址，不代替首次Verify或数据库接受事实。

本步声明：`func Reference(raw []byte, purpose string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>decodeEnvelope(raw)</code>；<code>sha256.Sum256(raw)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>purposeRole(purpose)</code>；<code>e.Purpose != purpose</code>；<code>&quot;rebuild-&quot; + purpose</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>&quot;rebuild-&quot; + purpose + &quot;:&quot; + hex.EncodeToString(digest&#91;:&#93;)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrEnvelope</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; purposeRole(purpose) == &quot;&quot; &#124;&#124; e.Purpose != purpose</code>。条件成立时执行 <code>return &quot;&quot;, ErrEnvelope</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;rebuild-&quot; + purpose + &quot;:&quot; + hex.EncodeToString(digest&#91;:&#93;), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=296:698 -->
````go
// Reference只计算结构合法制品的内容地址，不代替首次Verify或数据库接受事实。
func Reference(raw []byte, purpose string) (string, error) {
	e, err := decodeEnvelope(raw)
	if err != nil || purposeRole(purpose) == "" || e.Purpose != purpose {
		return "", ErrEnvelope
	}
	digest := sha256.Sum256(raw)
	return "rebuild-" + purpose + ":" + hex.EncodeToString(digest[:]), nil
}

````

## 第 5 步：FileName

插入位置：第 4 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func FileName(reference string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>reference</code> | <code>string</code> | 在此声明中的实际使用：<code>strings.HasPrefix(reference, prefix)</code>；<code>strings.TrimPrefix(reference, prefix)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>name</code>；<code>&quot;&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>ErrEnvelope</code> |

**跟着本段代码走一遍：**

- 检查 <code>strings.HasPrefix(reference, prefix)</code>。
- 检查 <code>artifactFile.MatchString(name)</code>。条件成立时执行 <code>return name, nil</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;&quot;, ErrEnvelope</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=698:1078 -->
````go
func FileName(reference string) (string, error) {
	for _, kind := range []string{"prepare", "snapshot"} {
		prefix := "rebuild-" + kind + "-v1:"
		if strings.HasPrefix(reference, prefix) {
			digest := strings.TrimPrefix(reference, prefix)
			name := digest + "." + kind + ".json"
			if artifactFile.MatchString(name) {
				return name, nil
			}
		}
	}
	return "", ErrEnvelope
}

````

## 第 6 步：ReadFile

插入位置：第 5 步代码之后，紧接追加；完成稿约第 41 行。

ReadFile只在装配层给出的本服务inbox内按内容地址读取，不接受路径或URL作为引用。

本步声明：`func ReadFile(dir, reference string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>dir</code> | <code>string</code> | 在此声明中的实际使用：<code>filepath.Join(dir, name)</code> |
| <code>reference</code> | <code>string</code> | 在此声明中的实际使用：<code>FileName(reference)</code>；<code>actual != reference</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>raw</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>fileFailure</code>；<code>ErrEnvelope</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !info.Mode().IsRegular() &#124;&#124; info.Size() &gt; MaxEnvelopeBytes</code>。条件成立时执行 <code>return nil, fileFailure</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, fileFailure</code>。当前函数在这里结束。
- 检查 <code>strings.HasSuffix(name, &quot;.snapshot.json&quot;)</code>。
- 检查 <code>err != nil &#124;&#124; actual != reference</code>。条件成立时执行 <code>return nil, ErrEnvelope</code>。当前函数在这里结束。
- 此处通过 <code>return raw, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1078:1791 -->
````go
// ReadFile只在装配层给出的本服务inbox内按内容地址读取，不接受路径或URL作为引用。
func ReadFile(dir, reference string) ([]byte, error) {
	name, err := FileName(reference)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxEnvelopeBytes {
		return nil, fileFailure
	}
	raw, err := readBounded(path)
	if err != nil {
		return nil, fileFailure
	}
	purpose := PrepareV1
	if strings.HasSuffix(name, ".snapshot.json") {
		purpose = SnapshotV1
	}
	actual, err := Reference(raw, purpose)
	if err != nil || actual != reference {
		return nil, ErrEnvelope
	}
	return raw, nil
}

````

## 第 7 步：PublishFile

插入位置：第 6 步代码之后，紧接追加；完成稿约第 67 行。

PublishFile先写受控临时文件并Sync，成功后才发布。Windows重试遇到原文件时核原字节摘要，
不覆盖不同内容。dir只由本服务可信配置构造，name只能是内容地址文件名。

本步声明：`func PublishFile(dir, name string, write func(io.Writer) error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>dir</code> | <code>string</code> | 在此声明中的实际使用：<code>os.MkdirAll(dir, 0700)</code>；<code>os.Lstat(dir)</code>；<code>os.CreateTemp(dir, &quot;.rebuild-*&quot;)</code>；<code>filepath.Join(dir, name)</code> |
| <code>name</code> | <code>string</code> | 在此声明中的实际使用：<code>artifactFile.MatchString(name)</code>；<code>filepath.Join(dir, name)</code> |
| <code>write</code> | <code>func(io.Writer) error</code> | 在此声明中的实际使用：<code>write == nil</code>；<code>write(io.MultiWriter(temp, hash))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>fileFailure</code>；<code>err</code>；<code>errors.New(&quot;REBUILD_ARTIFACT_FILE_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!artifactFile.MatchString(name) &#124;&#124; write == nil</code>。条件成立时执行 <code>return fileFailure</code>。当前函数在这里结束。
- 检查 <code>os.MkdirAll(dir, 0700) != nil</code>。条件成立时执行 <code>return fileFailure</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !info.IsDir() &#124;&#124; info.Mode()&amp;os.ModeSymlink != 0</code>。条件成立时执行 <code>return fileFailure</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fileFailure</code>。当前函数在这里结束。
- 登记 <code>os.Remove(path)</code>，当前函数退出时执行，提前 return 也会触发。
- 登记 <code>temp.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>temp.Sync() != nil &#124;&#124; temp.Close() != nil</code>。条件成立时执行 <code>return fileFailure</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>!info.Mode().IsRegular()</code>。条件成立时执行 <code>return fileFailure</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fileFailure</code>。当前函数在这里结束。
- 登记 <code>existing.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fileFailure</code>。当前函数在这里结束。
- 检查 <code>string(original.Sum(nil)) != string(hash.Sum(nil))</code>。条件成立时执行 <code>return errors.New(&quot;REBUILD_ARTIFACT_FILE_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!errors.Is(err, os.ErrNotExist)</code>。条件成立时执行 <code>return fileFailure</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=1791:3200 -->
````go
// PublishFile先写受控临时文件并Sync，成功后才发布。Windows重试遇到原文件时核原字节摘要，
// 不覆盖不同内容。dir只由本服务可信配置构造，name只能是内容地址文件名。
func PublishFile(dir, name string, write func(io.Writer) error) error {
	if !artifactFile.MatchString(name) || write == nil {
		return fileFailure
	}
	if os.MkdirAll(dir, 0700) != nil {
		return fileFailure
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fileFailure
	}
	temp, err := os.CreateTemp(dir, ".rebuild-*")
	if err != nil {
		return fileFailure
	}
	path := temp.Name()
	defer os.Remove(path)
	defer temp.Close()
	hash := sha256.New()
	if err := write(io.MultiWriter(temp, hash)); err != nil {
		return err
	}
	if temp.Sync() != nil || temp.Close() != nil {
		return fileFailure
	}
	target := filepath.Join(dir, name)
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() {
			return fileFailure
		}
		existing, err := os.Open(target)
		if err != nil {
			return fileFailure
		}
		defer existing.Close()
		original := sha256.New()
		if _, err := io.Copy(original, existing); err != nil {
			return fileFailure
		}
		if string(original.Sum(nil)) != string(hash.Sum(nil)) {
			return errors.New("REBUILD_ARTIFACT_FILE_CONFLICT")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
````

## 第 8 步：PublishFile（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 113 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return fileFailure</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>os.Rename(path, target) != nil</code>。条件成立时执行 <code>return fileFailure</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3200:3299 -->
````go
		return fileFailure
	}
	if os.Rename(path, target) != nil {
		return fileFailure
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/rebuildartifact/files.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/05-rebuild-artifacts.md)。
