# 分步编写 `services/account/internal/logic/account/output.go`

先按 [A08 · 输出保护与HTTP、RPC、worker完整装配](../lessons/account/08-output-and-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/066-output.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

核对响应捕获的全部来源，再到Doc注册该入口handler的输出。

将当前代次贴给旧结果会越过撤权；必须与构造响应时捕获的epoch逐项相等。

写之前先按这条执行路线理解各段的连接：

1. Outputs.Register校验scope，CurrentEpoch逐项比对，排序成稳定来源列表。
2. 全部吻合后调用OutputRegistrar.Register；错误原样交给保护器拒绝。

本文件需要守住的边界：

- 注册未知不等于成功，不能发送已经缓冲的用户数据。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/logic/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/logic/account/output.go') -and (Get-Item -LiteralPath 'services/account/internal/logic/account/output.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/logic/account/output.go')) { New-Item -ItemType File -Path 'services/account/internal/logic/account/output.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:205 -->
````go
package account

import (
	"context"
	"errors"
	"sort"

	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	"collaboration/platform"
)

````

## 第 2 步：OutputRegistrar

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`type OutputRegistrar interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Register</code> | <code>(context.Context, string, &#91;&#93;domain.OutputSource) error</code> | 顺序接收ctx、输出编号、实际权限来源列表；把Account输出纳入Doc的停止追踪，登记失败不得先发送受保护内容。 |

<!-- write-step: 2 bytes=205:305 -->
````go
type OutputRegistrar interface {
	Register(context.Context, string, []domain.OutputSource) error
}

````

## 第 3 步：Outputs

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`type Outputs struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>repository</code> | <code>*repository.Outputs</code> | 使用 <code>repository.Outputs</code> 类型；其定义处的职责：按输出来源类型重读Account当前权威代次。 |
| <code>registrar</code> | <code>OutputRegistrar</code> | 跨服务输出登记接口；本服务保存原输出绑定，Doc追踪关闭参与者。 |

<!-- write-step: 3 bytes=305:390 -->
````go
type Outputs struct {
	repository *repository.Outputs
	registrar  OutputRegistrar
}

````

## 第 4 步：NewOutputs

插入位置：第 3 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`func NewOutputs(repo *repository.Outputs, registrar OutputRegistrar) *Outputs`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>repo</code> | <code>*repository.Outputs</code> | 使用 <code>repository.Outputs</code> 类型；其定义处的职责：按输出来源类型重读Account当前权威代次。<br>在此声明中的实际使用：<code>repository: repo</code> |
| <code>registrar</code> | <code>OutputRegistrar</code> | 在此声明中的实际使用：<code>registrar: registrar</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Outputs</code> | 实际返回表达式：<code>&amp;Outputs{repository: repo, registrar: registrar}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Outputs{repository: repo, registrar: registrar}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=390:530 -->
````go
func NewOutputs(repo *repository.Outputs, registrar OutputRegistrar) *Outputs {
	return &Outputs{repository: repo, registrar: registrar}
}

````

## 第 5 步：Outputs.Register

插入位置：第 4 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func (s *Outputs) Register(ctx context.Context, handler string, binding *domain.OutputBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.CurrentEpoch(ctx, key)</code>；<code>s.registrar.Register(ctx, handler, sources)</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>s.registrar.Register(ctx, handler, sources)</code> |
| <code>binding</code> | <code>*domain.OutputBinding</code> | 使用 <code>domain.OutputBinding</code> 类型；其定义处的职责：收集一次响应实际依赖的来源与代次，并记录捕获冲突。<br>在此声明中的实际使用：<code>binding.Sources</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SOURCE_INVALID&quot;)</code>；<code>errors.New(&quot;SOURCE_CHANGED&quot;)</code>；<code>s.registrar.Register(ctx, handler, sources)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.ValidAccountOutputScope(key)</code>。条件成立时执行 <code>return errors.New(&quot;SOURCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; current != epoch</code>。条件成立时执行 <code>return errors.New(&quot;SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return sources&#91;i&#93;.Key &lt; sources&#91;j&#93;.Key</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return s.registrar.Register(ctx, handler, sources)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=530:1283 -->
````go
func (s *Outputs) Register(ctx context.Context, handler string, binding *domain.OutputBinding) error {
	sources := make([]domain.OutputSource, 0, len(binding.Sources))
	for key, epoch := range binding.Sources {
		if !platform.ValidAccountOutputScope(key) {
			return errors.New("SOURCE_INVALID")
		}
		// 重检构造响应时捕获的代次；不能把新成员/角色代次贴到旧查询结果上。
		current, err := s.repository.CurrentEpoch(ctx, key)
		if err != nil || current != epoch {
			return errors.New("SOURCE_CHANGED")
		}
		sources = append(sources, domain.OutputSource{Key: key, Epoch: epoch})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Key < sources[j].Key })
	return s.registrar.Register(ctx, handler, sources)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/logic/account/output.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/08-output-and-wiring.md)。
