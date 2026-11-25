# 分步编写 `services/document/internal/svc/resources_outputs_wiring.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/278-resources-outputs-wiring.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

装配对象存储、配额、导出、公开阅读、输出关闭和搜索页。

存储实现差异应止于适配边界，业务不能依赖本地文件或拥有Search私库凭据。

写之前先按这条执行路线理解各段的连接：

1. newResourceFiles选择local/OSS
2. resourceFiles映射完整性错误
3. NewOutputs建立真实实例及退出证明器
4. NewExports配置转换器
5. NewSearchPages独立用途派生游标密钥。

本文件需要守住的边界：

- 密钥失败不能回退空key或开发常量
- 输出恢复沿原PID/run-owner
- OSS并不改变单机锁前提。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/svc' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/svc/resources_outputs_wiring.go') -and (Get-Item -LiteralPath 'services/document/internal/svc/resources_outputs_wiring.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/svc/resources_outputs_wiring.go')) { New-Item -ItemType File -Path 'services/document/internal/svc/resources_outputs_wiring.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:786 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	"collaboration/document/internal/client"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/exporter"
	logic "collaboration/document/internal/logic/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/document/internal/storage/local"
	ossstore "collaboration/document/internal/storage/oss"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// exports.go：保留原实现，按业务合并文件。

````

## 第 2 步：NewExports

插入位置：第 1 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`func NewExports(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, public *logic.PublicReading, outputs *logic.Outputs, resources *logic.Resources) (*logic.Exports, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg.Root</code>；<code>cfg.Exports</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewExports(store.DB, identity, access, public, outputs, resources, renderer, c.Workers)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewExports(store.DB, identity, access, public, outputs, resources, renderer, c.Workers)</code> |
| <code>public</code> | <code>*logic.PublicReading</code> | 使用 <code>logic.PublicReading</code> 类型；其定义处的职责：按当前正式指针与当前受众输出公开文档、受控历史和搜索快照。<br>在此声明中的实际使用：<code>logic.NewExports(store.DB, identity, access, public, outputs, resources, renderer, c.Workers)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>logic.NewExports(store.DB, identity, access, public, outputs, resources, renderer, c.Workers)</code> |
| <code>resources</code> | <code>*logic.Resources</code> | 使用 <code>logic.Resources</code> 类型；其定义处的职责：编排上传预留、流式写入、完成结算和未引用对象回收。<br>在此声明中的实际使用：<code>logic.NewExports(store.DB, identity, access, public, outputs, resources, renderer, c.Workers)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Exports</code> | 使用 <code>logic.Exports</code> 类型；其定义处的职责：Exports编排权威准入与持久作业；格式转换/文件存储通过构造注入，HTTP与SQL分别留在适配层。<br>由 <code>logic.NewExports(store.DB, identity, access, public, outputs, resources, renderer, c.Workers)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>logic.NewExports(store.DB, identity, access, public, outputs, resources, renderer, c.Workers)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>cfg.Exports != nil</code>。
- 检查 <code>c.Root == &quot;&quot;</code>。
- 检查 <code>c.Pandoc == &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>c.Node == &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>c.Browser == &quot;&quot; &amp;&amp; runtime.GOOS == &quot;windows&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewExports(store.DB, identity, access, public, outputs, resources, renderer, c.Workers)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=786:2017 -->
````go
func NewExports(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, public *logic.PublicReading, outputs *logic.Outputs, resources *logic.Resources) (*logic.Exports, error) {
	c := platform.ExportConfig{Root: filepath.Join(cfg.Root, ".runtime", "exports", "document")}
	if cfg.Exports != nil {
		c = *cfg.Exports
	}
	if c.Root == "" {
		c.Root = filepath.Join(cfg.Root, ".runtime", "exports", "document")
	}
	// 开发环境可发现PATH中的转换器；生产和不同Windows安装位置可以显式配置。
	var err error
	if c.Pandoc == "" {
		c.Pandoc, err = exec.LookPath("pandoc")
		if err != nil {
			return nil, err
		}
	}
	if c.Node == "" {
		c.Node, err = exec.LookPath("node")
		if err != nil {
			return nil, err
		}
	}
	if c.Browser == "" && runtime.GOOS == "windows" {
		c.Browser = "msedge"
	}
	renderer, err := exporter.New(exporter.Config{Root: c.Root, Pandoc: c.Pandoc, Node: c.Node, PDFScript: filepath.Join(cfg.Root, "scripts", "export-pdf.mjs"), Browser: c.Browser})
	if err != nil {
		return nil, err
	}
	return logic.NewExports(store.DB, identity, access, public, outputs, resources, renderer, c.Workers)
}

// fence.go：保留原实现，按业务合并文件。

````

## 第 3 步：NewFences

插入位置：第 2 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`func NewFences(store *platform.Store, cfg platform.Config, outputs *logic.Outputs) *logic.Fences`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>client.NewCollaborationFence(cfg)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>logic.NewFences(store.DB, repository.NewFences(store.DB), outputs, client.NewCollaborationFence(cfg))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Fences</code> | 使用 <code>logic.Fences</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。<br>实际返回表达式：<code>logic.NewFences(store.DB, repository.NewFences(store.DB), outputs, client.NewCollaborationFence(cfg))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewFences(store.DB, repository.NewFences(store.DB), outputs, client.NewCollaborationFence(cfg))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2017:2288 -->
````go
func NewFences(store *platform.Store, cfg platform.Config, outputs *logic.Outputs) *logic.Fences {
	return logic.NewFences(store.DB, repository.NewFences(store.DB), outputs, client.NewCollaborationFence(cfg))
}

// output.go：保留原实现，按业务合并文件。

````

## 第 4 步：NewOutputs

插入位置：第 3 步代码之后，紧接追加；完成稿约第 70 行。

启动恢复使用同一私有池和既定运行归属，不创建新的数据库或替换退出证明。

本步声明：`func NewOutputs(store *platform.Store, cfg platform.Config) (*logic.Outputs, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code>；<code>store.Context</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>platform.HTTPPeer(cfg, &quot;edge&quot;)</code>；<code>cfg.Peers</code>；<code>cfg.Root</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>实际返回表达式：<code>nil</code>；<code>manager</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;OUTPUT_OWNER_REQUIRED&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>owner == &quot;&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;OUTPUT_OWNER_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return manager, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2288:3271 -->
````go
// 启动恢复使用同一私有池和既定运行归属，不创建新的数据库或替换退出证明。
func NewOutputs(store *platform.Store, cfg platform.Config) (*logic.Outputs, error) {
	owner := os.Getenv("COLLAB_RUN_OWNER")
	if owner == "" {
		return nil, errors.New("OUTPUT_OWNER_REQUIRED")
	}
	digest := sha256.Sum256([]byte("document\n" + owner))
	httpClient, err := platform.HTTPPeer(cfg, "edge")
	if err != nil {
		return nil, err
	}
	self := domain.OutputInstance{ID: owner + "-document", PID: uint64(os.Getpid()), Boot: hex.EncodeToString(digest[:])}
	manager := logic.NewOutputs(store.DB, repository.NewOutputs(store.DB), self, client.NewEdgeOutput(cfg.Peers["edge"], httpClient), client.NewProcessOwner(cfg.Root))
	if err := manager.Initialize(store.Context); err != nil {
		slog.Warn("output_recovery_blocked", "code", logic.OutputRecoveryCode(err))
		return nil, err
	}
	return manager, nil
}

// public_reading.go：保留原实现，按业务合并文件。

````

## 第 5 步：NewPublicReading

插入位置：第 4 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`func NewPublicReading(store *platform.Store, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs, pagination *logic.Pagination) (*logic.PublicReading, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewPublicReading(store.DB, identity, access, outputs, pagination, validator)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewPublicReading(store.DB, identity, access, outputs, pagination, validator)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>logic.NewPublicReading(store.DB, identity, access, outputs, pagination, validator)</code> |
| <code>pagination</code> | <code>*logic.Pagination</code> | 使用 <code>logic.Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>logic.NewPublicReading(store.DB, identity, access, outputs, pagination, validator)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.PublicReading</code> | 使用 <code>logic.PublicReading</code> 类型；其定义处的职责：按当前正式指针与当前受众输出公开文档、受控历史和搜索快照。<br>实际返回表达式：<code>nil</code>；<code>logic.NewPublicReading(store.DB, identity, access, outputs, pagination, validator)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewPublicReading(store.DB, identity, access, outputs, pagination, validator), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=3271:3691 -->
````go
func NewPublicReading(store *platform.Store, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs, pagination *logic.Pagination) (*logic.PublicReading, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewPublicReading(store.DB, identity, access, outputs, pagination, validator), nil
}

// publication.go：保留原实现，按业务合并文件。

````

## 第 6 步：NewPublications

插入位置：第 5 步代码之后，紧接追加；完成稿约第 102 行。

本步声明：`func NewPublications(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Publications, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>platform.HTTPPeer(cfg, &quot;collaboration&quot;)</code>；<code>client.NewMerges(cfg, peer)</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewPublications(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewPublications(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs.Self</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Publications</code> | 使用 <code>logic.Publications</code> 类型；其定义处的职责：创建正式版本、切历史正式版或撤销发布，并恢复可靠MAIN捕获。<br>实际返回表达式：<code>nil</code>；<code>logic.NewPublications(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewPublications(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3691:4216 -->
````go
func NewPublications(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Publications, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewPublications(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID), nil
}

// resources.go：保留原实现，按业务合并文件。

````

## 第 7 步：NewResources

插入位置：第 6 步代码之后，紧接追加；完成稿约第 116 行。

本步声明：`func NewResources(store *platform.Store, cfg platform.Config, access *logic.Access) (*logic.Resources, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg.Root</code>；<code>cfg.Resources</code>；<code>cfg.Origin</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewResources(store.DB, access, logic.ResourceLimits{SpaceBytes: c.SpaceBytes, UploadBytes: c.UploadBytes, TemporaryBytes: c.TemporaryBytes, UploadTTL: time.Duration(c.UploadT …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Resources</code> | 使用 <code>logic.Resources</code> 类型；其定义处的职责：编排上传预留、流式写入、完成结算和未引用对象回收。<br>实际返回表达式：<code>nil</code>；<code>logic.NewResources(store.DB, access, logic.ResourceLimits{SpaceBytes: c.SpaceBytes, UploadBytes: c.UploadBytes, TemporaryBytes: c.TemporaryBytes, UploadTTL: time.Duration(c.UploadT …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;RESOURCE_CONFIG_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>cfg.Resources != nil</code>。
- 检查 <code>c.SpaceBytes == 0 &#124;&#124; c.UploadBytes == 0 &#124;&#124; c.UploadBytes &gt; 32&lt;&lt;20 &#124;&#124; c.TemporaryBytes &lt; c.UploadBytes &#124;&#124; c.UploadTTLSeconds &lt; 60 &#124;&#124; c.UploadTTLSeconds &gt; 86400</code>。条件成立时执行 <code>return nil, errors.New(&quot;RESOURCE_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewResources(store.DB, access, logic.ResourceLimits{SpaceBytes: c.SpaceBytes, UploadBytes: c.UploadBytes, TemporaryBytes: c.TemporaryBytes, UploadTTL: time.Duration(c. …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4216:5229 -->
````go
func NewResources(store *platform.Store, cfg platform.Config, access *logic.Access) (*logic.Resources, error) {
	// 开发默认值集中在装配层；生产可显式配置目录与预算，不让仓储隐含容量。
	c := platform.ResourceConfig{Root: filepath.Join(cfg.Root, ".runtime", "resources", "document"), SpaceBytes: 1 << 30, UploadBytes: 32 << 20, TemporaryBytes: 256 << 20, UploadTTLSeconds: 3600}
	if cfg.Resources != nil {
		c = *cfg.Resources
	}
	if c.SpaceBytes == 0 || c.UploadBytes == 0 || c.UploadBytes > 32<<20 || c.TemporaryBytes < c.UploadBytes || c.UploadTTLSeconds < 60 || c.UploadTTLSeconds > 86400 {
		return nil, errors.New("RESOURCE_CONFIG_INVALID")
	}
	files, err := newResourceFiles(c)
	if err != nil {
		return nil, err
	}
	return logic.NewResources(store.DB, access, logic.ResourceLimits{SpaceBytes: c.SpaceBytes, UploadBytes: c.UploadBytes, TemporaryBytes: c.TemporaryBytes, UploadTTL: time.Duration(c.UploadTTLSeconds) * time.Second}, cfg.Origin, resourceFiles{files}), nil
}

````

## 第 8 步：newResourceFiles

插入位置：第 7 步代码之后，紧接追加；完成稿约第 132 行。

本步声明：`func newResourceFiles(c platform.ResourceConfig) (logic.ResourceFiles, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>c</code> | <code>platform.ResourceConfig</code> | 使用 <code>platform.ResourceConfig</code> 类型；其定义处的职责：加载服务自身配置，创建有界数据库池，并以迁移和恢复事实判定就绪。<br>在此声明中的实际使用：<code>c.OSS</code>；<code>c.Root</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>logic.ResourceFiles</code> | 使用 <code>logic.ResourceFiles</code> 类型；其定义处的职责：统一的资源存储接口，保持本地和OSS都具备锁定、完整性核验、释放与删除语义。<br>由 <code>ossstore.New(c.Root, ossstore.Config{Region: c.OSS.Region, Endpoint: c.OSS.Endpoint, 			Bucket: c.OSS.Bucket, Prefix: c.OSS.Prefix, CredentialsFile: c.OSS.CredentialsFile})</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>files</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>ossstore.New(c.Root, ossstore.Config{Region: c.OSS.Region, Endpoint: c.OSS.Endpoint, 			Bucket: c.OSS.Bucket, Prefix: c.OSS.Prefix, CredentialsFile: c.OSS.CredentialsFile})</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>c.OSS != nil</code>。条件成立时执行 <code>return ossstore.New(c.Root, ossstore.Config{Region: c.OSS.Region, Endpoint: c.OSS.Endpoint, 			Bucket: c.OSS.Bucket, Prefix: c.OSS.Prefix, CredentialsFile: c.OSS.CredentialsFile})</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return files, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=5229:5700 -->
````go
func newResourceFiles(c platform.ResourceConfig) (logic.ResourceFiles, error) {
	if c.OSS != nil {
		return ossstore.New(c.Root, ossstore.Config{Region: c.OSS.Region, Endpoint: c.OSS.Endpoint,
			Bucket: c.OSS.Bucket, Prefix: c.OSS.Prefix, CredentialsFile: c.OSS.CredentialsFile})
	}
	files, err := local.New(c.Root)
	if err != nil {
		return nil, err
	}
	if err := files.RequireLocalBackend(context.Background()); err != nil {
		return nil, err
	}
	return files, nil
}

````

## 第 9 步：resourceFiles

插入位置：第 8 步代码之后，紧接追加；完成稿约第 147 行。

适配器在装配边界把存储校验失败映射为业务错误，不让业务依赖本地文件实现。

本步声明：`type resourceFiles struct{ backend logic.ResourceFiles }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>backend</code> | <code>logic.ResourceFiles</code> | 使用 <code>logic.ResourceFiles</code> 类型；其定义处的职责：统一的资源存储接口，保持本地和OSS都具备锁定、完整性核验、释放与删除语义。 |

<!-- write-step: 9 bytes=5700:5870 -->
````go
// 适配器在装配边界把存储校验失败映射为业务错误，不让业务依赖本地文件实现。
type resourceFiles struct{ backend logic.ResourceFiles }

````

## 第 10 步：resourceFiles.Put

插入位置：第 9 步代码之后，紧接追加；完成稿约第 150 行。

本步声明：`func (f resourceFiles) Put(ctx context.Context, key string, body io.Reader, size uint64, digest [32]byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>f.backend.Put(ctx, key, body, size, digest)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>f.backend.Put(ctx, key, body, size, digest)</code> |
| <code>body</code> | <code>io.Reader</code> | 按需读取字节的输入流，不保证一次Read就拿到完整内容；调用者负责期限、体积限制和最终完整性核验。<br>在此声明中的实际使用：<code>f.backend.Put(ctx, key, body, size, digest)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>f.backend.Put(ctx, key, body, size, digest)</code> |
| <code>digest</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>f.backend.Put(ctx, key, body, size, digest)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>resourceFileError(f.backend.Put(ctx, key, body, size, digest))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return resourceFileError(f.backend.Put(ctx, key, body, size, digest))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5870:6057 -->
````go
func (f resourceFiles) Put(ctx context.Context, key string, body io.Reader, size uint64, digest [32]byte) error {
	return resourceFileError(f.backend.Put(ctx, key, body, size, digest))
}
````

## 第 11 步：resourceFiles.Open

插入位置：第 10 步代码之后，紧接追加；完成稿约第 153 行。

本步声明：`func (f resourceFiles) Open(ctx context.Context, key string, size uint64, digest [32]byte) (io.ReadCloser, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>f.backend.Open(ctx, key, size, digest)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>f.backend.Open(ctx, key, size, digest)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>f.backend.Open(ctx, key, size, digest)</code> |
| <code>digest</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>f.backend.Open(ctx, key, size, digest)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>io.ReadCloser</code> | 实际返回表达式：<code>file</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>resourceFileError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return file, resourceFileError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6057:6265 -->
````go
func (f resourceFiles) Open(ctx context.Context, key string, size uint64, digest [32]byte) (io.ReadCloser, error) {
	file, err := f.backend.Open(ctx, key, size, digest)
	return file, resourceFileError(err)
}
````

## 第 12 步：resourceFiles.Acquire

插入位置：第 11 步代码之后，紧接追加；完成稿约第 157 行。

本步声明：`func (f resourceFiles) Acquire(ctx context.Context, key string) (func(), error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>f.backend.Acquire(ctx, key)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>f.backend.Acquire(ctx, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>func()</code> | 实际返回表达式：<code>release</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>resourceFileError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return release, resourceFileError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6265:6434 -->
````go
func (f resourceFiles) Acquire(ctx context.Context, key string) (func(), error) {
	release, err := f.backend.Acquire(ctx, key)
	return release, resourceFileError(err)
}
````

## 第 13 步：resourceFiles.AcquireRead

插入位置：第 12 步代码之后，紧接追加；完成稿约第 161 行。

本步声明：`func (f resourceFiles) AcquireRead(ctx context.Context, key string) (func(), error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>f.backend.AcquireRead(ctx, key)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>f.backend.AcquireRead(ctx, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>func()</code> | 实际返回表达式：<code>release</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>resourceFileError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return release, resourceFileError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6434:6612 -->
````go
func (f resourceFiles) AcquireRead(ctx context.Context, key string) (func(), error) {
	release, err := f.backend.AcquireRead(ctx, key)
	return release, resourceFileError(err)
}

````

## 第 14 步：resourceFiles.CleanupUpload

插入位置：第 13 步代码之后，紧接追加；完成稿约第 166 行。

本步声明：`func (f resourceFiles) CleanupUpload(ctx context.Context, key string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>f.backend.CleanupUpload(ctx, key)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>f.backend.CleanupUpload(ctx, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>resourceFileError(f.backend.CleanupUpload(ctx, key))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return resourceFileError(f.backend.CleanupUpload(ctx, key))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6612:6754 -->
````go
func (f resourceFiles) CleanupUpload(ctx context.Context, key string) error {
	return resourceFileError(f.backend.CleanupUpload(ctx, key))
}

````

## 第 15 步：resourceFiles.Delete

插入位置：第 14 步代码之后，紧接追加；完成稿约第 170 行。

本步声明：`func (f resourceFiles) Delete(ctx context.Context, key string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>f.backend.Delete(ctx, key)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>f.backend.Delete(ctx, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>resourceFileError(f.backend.Delete(ctx, key))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return resourceFileError(f.backend.Delete(ctx, key))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6754:6881 -->
````go
func (f resourceFiles) Delete(ctx context.Context, key string) error {
	return resourceFileError(f.backend.Delete(ctx, key))
}
````

## 第 16 步：resourceFileError

插入位置：第 15 步代码之后，紧接追加；完成稿约第 173 行。

本步声明：`func resourceFileError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.Is(err, local.ErrIntegrity)</code>；<code>errors.Is(err, local.ErrConflict)</code>；<code>errors.Is(err, local.ErrNotFound)</code>；<code>errors.Is(err, local.ErrBusy)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;RESOURCE_CHECKSUM_MISMATCH&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_INCOMPLETE&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_BUSY&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;RESOURCE_CHECKSUM_MISMATCH&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_INCOMPLETE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;RESOURCE_UPLOAD_BUSY&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=6881:7404 -->
````go
func resourceFileError(err error) error {
	switch {
	case errors.Is(err, local.ErrIntegrity), errors.Is(err, local.ErrConflict):
		return domain.Violation(domain.StateConflict, "RESOURCE_CHECKSUM_MISMATCH")
	case errors.Is(err, local.ErrNotFound):
		return domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_INCOMPLETE")
	case errors.Is(err, local.ErrBusy):
		return domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_BUSY")
	default:
		return err
	}
}

// search.go：保留原实现，按业务合并文件。

````

## 第 17 步：NewSearchPages

插入位置：第 16 步代码之后，紧接追加；完成稿约第 188 行。

Search连接由bootstrap建立及释放，此处只装配Doc私有依赖；不引入Search库/ES凭据。
沿现有本地私钥作为master来源，独立用途派生且不回退到空key或固定开发常量。

本步声明：`func NewSearchPages(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs, peer pb.SearchInternalClient) (*logic.SearchPages, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg.KeyFile</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewSearchPages(store.DB, identity, access, outputs, client.NewSearch(peer), cursors, validator)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewSearchPages(store.DB, identity, access, outputs, client.NewSearch(peer), cursors, validator)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>logic.NewSearchPages(store.DB, identity, access, outputs, client.NewSearch(peer), cursors, validator)</code> |
| <code>peer</code> | <code>pb.SearchInternalClient</code> | 在此声明中的实际使用：<code>client.NewSearch(peer)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.SearchPages</code> | 使用 <code>logic.SearchPages</code> 类型；其定义处的职责：收集搜索候选、生成固定正式版摘要并以整页事务重新鉴权输出。<br>实际返回表达式：<code>nil</code>；<code>logic.NewSearchPages(store.DB, identity, access, outputs, client.NewSearch(peer), cursors, validator)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_PAGE_KEY_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; len(raw) == 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_PAGE_KEY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>clear(raw)</code>，当前函数退出时执行，提前 return 也会触发。
- 登记 <code>clear(key&#91;:&#93;)</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewSearchPages(store.DB, identity, access, outputs, client.NewSearch(peer), cursors, validator), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=7404:8402 -->
````go
// Search连接由bootstrap建立及释放，此处只装配Doc私有依赖；不引入Search库/ES凭据。
// 沿现有本地私钥作为master来源，独立用途派生且不回退到空key或固定开发常量。
func NewSearchPages(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs, peer pb.SearchInternalClient) (*logic.SearchPages, error) {
	raw, err := os.ReadFile(cfg.KeyFile)
	if err != nil || len(raw) == 0 {
		return nil, errors.New("SEARCH_PAGE_KEY_UNAVAILABLE")
	}
	defer clear(raw)
	input := append([]byte("collaboration/document/search-page-master/v1\x00"), raw...)
	key := sha256.Sum256(input)
	clear(input)
	defer clear(key[:])
	cursors, err := domain.NewSearchCursorCodec(key[:])
	if err != nil {
		return nil, err
	}
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewSearchPages(store.DB, identity, access, outputs, client.NewSearch(peer), cursors, validator), nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/svc/resources_outputs_wiring.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
