# 分步编写 `services/document/internal/storage/oss/credentials.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/132-credentials.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

从私有凭据文件读取并校验OSS访问身份。

凭据输入要有固定格式与访问边界，不能让SDK自行从环境选择另一个账号。

写之前先按这条执行路线理解各段的连接：

1. readCredentials读取私有文件并解析字段
2. credentialText检查凭据文本长度及字符
3. 失败返回固定错误供启动拒绝。

本文件需要守住的边界：

- 不要输出凭据内容或把私密文件复制进教材
- 不能把凭据读取失败当匿名访问继续。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/storage/oss' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/storage/oss/credentials.go') -and (Get-Item -LiteralPath 'services/document/internal/storage/oss/credentials.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/storage/oss/credentials.go')) { New-Item -ItemType File -Path 'services/document/internal/storage/oss/credentials.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:91 -->
````go
package oss

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

````

## 第 2 步：privateCredentials

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`type privateCredentials struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;accessKeyId&quot;`</code> |
| <code>Secret</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;accessKeySecret&quot;`</code> |
| <code>Token</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;securityToken&quot;`</code> |

<!-- write-step: 2 bytes=91:241 -->
````go
type privateCredentials struct {
	ID     string `json:"accessKeyId"`
	Secret string `json:"accessKeySecret"`
	Token  string `json:"securityToken"`
}

````

## 第 3 步：readCredentials

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func readCredentials(path string) (privateCredentials, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>filepath.IsAbs(path)</code>；<code>filepath.Clean(path)</code>；<code>os.Lstat(path)</code>；<code>os.Open(path)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>privateCredentials</code> | 实际返回表达式：<code>secret</code>；<code>privateCredentials{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrCredentials</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!filepath.IsAbs(path)</code>。条件成立时执行 <code>return secret, ErrCredentials</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; info.Mode()&amp;os.ModeSymlink != 0</code>。条件成立时执行 <code>return secret, ErrCredentials</code>。当前函数在这里结束。
- 检查 <code>current == filepath.Dir(current)</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>err != nil &#124;&#124; !before.Mode().IsRegular()</code>。条件成立时执行 <code>return secret, ErrCredentials</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return secret, ErrCredentials</code>。当前函数在这里结束。
- 登记 <code>file.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; !info.Mode().IsRegular() &#124;&#124; !os.SameFile(before, info) &#124;&#124; info.Size() &gt; 4096 &#124;&#124; !privateMode(info.Mode())</code>。条件成立时执行 <code>return secret, ErrCredentials</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(raw) &gt; 4096</code>。条件成立时执行 <code>return secret, ErrCredentials</code>。当前函数在这里结束。
- 检查 <code>decoder.Decode(&amp;secret) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; 		!credentialText(secret.ID, 16, 128) &#124;&#124; !credentialText(secret.Secret, 16, 256) &#124;&#124; 		(secret.Token != &quot;&quot; &amp;&amp; !credentialText(secret.Token, 1, 2048))</code>。条件成立时执行 <code>return privateCredentials{}, ErrCredentials</code>。当前函数在这里结束。
- 此处通过 <code>return secret, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=241:1911 -->
````go
func readCredentials(path string) (privateCredentials, error) {
	var secret privateCredentials
	if !filepath.IsAbs(path) {
		return secret, ErrCredentials
	}
	// 凭据不允许通过链接/目录替换读取；Windows ACL 由配置脚本和运行说明收紧，
	// Unix 另拒绝 group/other 可读文件。错误不带路径、内容或解析片段。
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return secret, ErrCredentials
		}
		if current == filepath.Dir(current) {
			break
		}
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() {
		return secret, ErrCredentials
	}
	file, err := os.Open(path)
	if err != nil {
		return secret, ErrCredentials
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(before, info) || info.Size() > 4096 || !privateMode(info.Mode()) {
		return secret, ErrCredentials
	}
	raw, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(raw) > 4096 {
		return secret, ErrCredentials
	}
	// Windows PowerShell 5 的 UTF8 文本可能带 BOM，允许该编码而非要求用户换终端。
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&secret) != nil || decoder.Decode(new(any)) != io.EOF ||
		!credentialText(secret.ID, 16, 128) || !credentialText(secret.Secret, 16, 256) ||
		(secret.Token != "" && !credentialText(secret.Token, 1, 2048)) {
		return privateCredentials{}, ErrCredentials
	}
	return secret, nil
}

````

## 第 4 步：credentialText

插入位置：第 3 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`func credentialText(value string, min, max int) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>len(value)</code>；<code>strings.TrimSpace(value)</code>；<code>strings.TrimSpace(value) != value</code> |
| <code>min</code> | <code>int</code> | 在此声明中的实际使用：<code>len(value) &lt; min</code> |
| <code>max</code> | <code>int</code> | 在此声明中的实际使用：<code>len(value) &gt; max</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(value) &lt; min &#124;&#124; len(value) &gt; max &#124;&#124; strings.TrimSpace(value) != value</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>character &lt; 33 &#124;&#124; character &gt; 126</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1911:2178 -->
````go
func credentialText(value string, min, max int) bool {
	if len(value) < min || len(value) > max || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 33 || character > 126 {
			return false
		}
	}
	return true
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/storage/oss/credentials.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
