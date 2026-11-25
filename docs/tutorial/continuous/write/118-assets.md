# 分步编写 `services/document/internal/exporter/assets.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/118-assets.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将固定资源中的合法图片转换为受控PNG供Word和PDF共用。

图片压缩字节小不代表解码内存小，必须先检查尺寸和像素预算。

写之前先按这条执行路线理解各段的连接：

1. writeAssets先DecodeConfig评估像素，再解码首帧并输出固定PNG
2. 返回资源ID到本地文件的映射。

本文件需要守住的边界：

- 源图片SHA由业务层已核验
- 不能将用户资源地址变成转换器可访问的远程URL。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/exporter' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/exporter/assets.go') -and (Get-Item -LiteralPath 'services/document/internal/exporter/assets.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/exporter/assets.go')) { New-Item -ItemType File -Path 'services/document/internal/exporter/assets.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:202 -->
````go
package exporter

import (
	"bytes"
	domain "collaboration/document/internal/domain/document"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

````

## 第 2 步：writeAssets

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

所有合法图片转换成固定PNG，Word与PDF共用同一首帧；先看像素预算再解码，
防止小压缩文件诱发无限像素分配。资源已经由业务完整checksum核验。

本步声明：`func writeAssets(directory string, items []domain.ExportAsset) (map[string]string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>directory</code> | <code>string</code> | 在此声明中的实际使用：<code>filepath.Join(directory, &quot;assets&quot;)</code>；<code>filepath.Join(directory, filepath.FromSlash(name))</code> |
| <code>items</code> | <code>&#91;&#93;domain.ExportAsset</code> | 使用 <code>domain.ExportAsset</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;string</code> | 实际返回表达式：<code>nil</code>；<code>paths</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_IMAGE_PIXEL_LIMIT&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_RESOURCE_INVALID&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_ASSET_LIMIT&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; config.Width &lt; 1 &#124;&#124; config.Height &lt; 1 &#124;&#124; uint64(config.Width)*uint64(config.Height) &gt; 16&lt;&lt;20</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;EXPORT_IMAGE_PIXEL_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;EXPORT_RESOURCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>output.Len() &gt; 32&lt;&lt;20 &#124;&#124; total &gt; (32&lt;&lt;20)-output.Len()</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;EXPORT_ASSET_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return paths, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=202:1678 -->
````go
// 所有合法图片转换成固定PNG，Word与PDF共用同一首帧；先看像素预算再解码，
// 防止小压缩文件诱发无限像素分配。资源已经由业务完整checksum核验。
func writeAssets(directory string, items []domain.ExportAsset) (map[string]string, error) {
	if err := os.Mkdir(filepath.Join(directory, "assets"), 0700); err != nil {
		return nil, err
	}
	paths := map[string]string{}
	var total int
	for _, item := range items {
		if _, err := domain.CanonicalID(item.ID); err != nil {
			return nil, err
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(item.Data))
		if err != nil || config.Width < 1 || config.Height < 1 || uint64(config.Width)*uint64(config.Height) > 16<<20 {
			return nil, domain.Violation(domain.StateConflict, "EXPORT_IMAGE_PIXEL_LIMIT")
		}
		picture, _, err := image.Decode(bytes.NewReader(item.Data))
		if err != nil {
			return nil, domain.Violation(domain.StateConflict, "EXPORT_RESOURCE_INVALID")
		}
		var output bytes.Buffer
		if err = png.Encode(&output, picture); err != nil {
			return nil, err
		}
		if output.Len() > 32<<20 || total > (32<<20)-output.Len() {
			return nil, domain.Violation(domain.StateConflict, "EXPORT_ASSET_LIMIT")
		}
		total += output.Len()
		name := "assets/resource-" + item.ID + ".png"
		if err = os.WriteFile(filepath.Join(directory, filepath.FromSlash(name)), output.Bytes(), 0600); err != nil {
			return nil, err
		}
		paths[item.ID] = name
	}
	return paths, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/exporter/assets.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
