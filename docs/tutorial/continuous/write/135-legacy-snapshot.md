# 分步编写 `services/document/internal/storage/oss/legacy_snapshot.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/135-legacy-snapshot.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在冷备份边界验证旧metadata-only键本地及云端均不存在。

历史实验元数据可能没有规范key或SHA，只有明确不存在才能记录缺失，不能假造正常对象。

写之前先按这条执行路线理解各段的连接：

1. VerifyLegacyAbsence检查本地旧文件及原OSS前缀，沿原位置取得缺失证据。

本文件需要守住的边界：

- 需全部写者停止
- 权限、网络未知不是不存在，必须使快照失败。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/storage/oss' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/storage/oss/legacy_snapshot.go') -and (Get-Item -LiteralPath 'services/document/internal/storage/oss/legacy_snapshot.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/storage/oss/legacy_snapshot.go')) { New-Item -ItemType File -Path 'services/document/internal/storage/oss/legacy_snapshot.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:152 -->
````go
package oss

import (
	"context"
	"errors"

	"collaboration/document/internal/storage/local"
	sdk "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
)

````

## 第 2 步：Store.VerifyLegacyAbsence

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

VerifyLegacyAbsence 不放宽业务 key 规则。仅在停止全部写者的冷备份中，
证明旧 metadata-only key 在本地和原云前缀均明确不存在；权限/网络未知失败。

本步声明：`func (s *Store) VerifyLegacyAbsence(ctx context.Context, key string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.local.VerifyLegacyAbsence(ctx, key)</code>；<code>s.client.GetObject(ctx, &amp;sdk.GetObjectRequest{ 		Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(s.config.Prefix + key), 	})</code>；<code>storageError(ctx, err)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.local.VerifyLegacyAbsence(ctx, key)</code>；<code>s.config.Prefix + key</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>local.ErrUnavailable</code>；<code>local.ErrIntegrity</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.Is(storageError(ctx, err), local.ErrNotFound)</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 此处通过 <code>return local.ErrUnavailable</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>result != nil &amp;&amp; result.Body != nil</code>。
- 此处通过 <code>return local.ErrIntegrity</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=152:872 -->
````go
// VerifyLegacyAbsence 不放宽业务 key 规则。仅在停止全部写者的冷备份中，
// 证明旧 metadata-only key 在本地和原云前缀均明确不存在；权限/网络未知失败。
func (s *Store) VerifyLegacyAbsence(ctx context.Context, key string) error {
	if err := s.local.VerifyLegacyAbsence(ctx, key); err != nil {
		return err
	}
	result, err := s.client.GetObject(ctx, &sdk.GetObjectRequest{
		Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(s.config.Prefix + key),
	})
	if err != nil {
		if errors.Is(storageError(ctx, err), local.ErrNotFound) {
			return nil
		}
		return local.ErrUnavailable
	}
	if result != nil && result.Body != nil {
		result.Body.Close()
	}
	return local.ErrIntegrity
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/storage/oss/legacy_snapshot.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
