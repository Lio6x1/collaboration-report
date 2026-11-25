# 分步编写 `services/account/cmd/platform-governance/main.go`

先按 [A04 · 原操作回执、授权证明与平台治理任免](../lessons/account/04-receipts-governance.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/086-main.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供仅本机运维使用的平台管理员status/grant/revoke命令。

任免依赖Account部署配置和明确原号/修订，防止普通HTTP用户自提权。

写之前先按这条执行路线理解各段的连接：

1. run 检查规范用户ID、绝对config路径和account角色；status读取当前revision。
2. grant/revoke要求expected-revision，将动作转大写，在3秒上下文内调用GovernanceOperator.Change。

本文件需要守住的边界：

- 未知提交继续原operation-id；日志只输出PLATFORM_GOVERNANCE稳定码，不输出SQL或配置原文。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/cmd/platform-governance' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/cmd/platform-governance/main.go') -and (Get-Item -LiteralPath 'services/account/cmd/platform-governance/main.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/cmd/platform-governance/main.go')) { New-Item -ItemType File -Path 'services/account/cmd/platform-governance/main.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:312 -->
````go
// 本机运维命令只读取Account自己的部署配置，不向浏览器开放平台管理员任免。
package main

import (
	logic "collaboration/account/internal/logic/account"
	"collaboration/platform"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

````

## 第 2 步：main

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func main()`。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 检查 <code>strings.HasPrefix(err.Error(), &quot;PLATFORM_GOVERNANCE_&quot;)</code>。

<!-- write-step: 2 bytes=312:741 -->
````go
func main() {
	value, err := run()
	if err != nil {
		code := "PLATFORM_GOVERNANCE_OPERATOR_FAILED"
		// SQL/配置错误原文可能包含部署信息；只向运维终端返回自身稳定业务码。
		if strings.HasPrefix(err.Error(), "PLATFORM_GOVERNANCE_") {
			code = err.Error()
		}
		_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"code": code})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(value)
}

````

## 第 3 步：run

插入位置：第 2 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func run() (any, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 由 <code>operator.Current(ctx, user)</code> 的第 1 个返回项原样转交。<br>由 <code>operator.Change(ctx, logic.GovernanceOperatorInput{User: user, Action: strings.ToUpper(*action), Operation: *operation, Expected: revision})</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>operator.Current(ctx, user)</code> 的第 2 个返回项原样转交。<br>由 <code>operator.Change(ctx, logic.GovernanceOperatorInput{User: user, Action: strings.ToUpper(*action), Operation: *operation, Expected: revision})</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;PLATFORM_GOVERNANCE_INPUT_INVALID&quot;)</code>；<code>errors.New(&quot;PLATFORM_GOVERNANCE_ACCOUNT_CONFIG_REQUIRED&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; user == 0 &#124;&#124; strconv.FormatUint(user, 10) != *userText &#124;&#124; !filepath.IsAbs(*config) &#124;&#124; flag.NArg() != 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;PLATFORM_GOVERNANCE_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; cfg.Role != &quot;account&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;PLATFORM_GOVERNANCE_ACCOUNT_CONFIG_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>store.DB.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>context.WithTimeout(context.Background(), 3*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>*action == &quot;status&quot;</code>。条件成立时执行 <code>return operator.Current(ctx, user)</code>。当前函数在这里结束。
- 检查 <code>*action != &quot;grant&quot; &amp;&amp; *action != &quot;revoke&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;PLATFORM_GOVERNANCE_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; strconv.FormatUint(revision, 10) != *revisionText</code>。条件成立时执行 <code>return nil, errors.New(&quot;PLATFORM_GOVERNANCE_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return operator.Change(ctx, logic.GovernanceOperatorInput{User: user, Action: strings.ToUpper(*action), Operation: *operation, Expected: revision})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=741:2415 -->
````go
func run() (any, error) {
	config := flag.String("config", "", "Account部署配置的绝对路径（包含数据库凭据，不得分享）")
	userText := flag.String("user-id", "", "当前实际账号的十进制ID")
	action := flag.String("action", "status", "status、grant或revoke")
	operation := flag.String("operation-id", "", "任免原号；未知结果继续使用同一个原号")
	revisionText := flag.String("expected-revision", "", "status返回的当前修订；首次授予为0")
	flag.Parse()
	user, err := strconv.ParseUint(*userText, 10, 64)
	if err != nil || user == 0 || strconv.FormatUint(user, 10) != *userText || !filepath.IsAbs(*config) || flag.NArg() != 0 {
		return nil, errors.New("PLATFORM_GOVERNANCE_INPUT_INVALID")
	}
	cfg, err := platform.LoadConfig(*config)
	if err != nil || cfg.Role != "account" {
		return nil, errors.New("PLATFORM_GOVERNANCE_ACCOUNT_CONFIG_REQUIRED")
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		return nil, err
	}
	defer store.DB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	operator := logic.NewGovernanceOperator(store.DB)
	if *action == "status" {
		return operator.Current(ctx, user)
	}
	if *action != "grant" && *action != "revoke" {
		return nil, errors.New("PLATFORM_GOVERNANCE_INPUT_INVALID")
	}
	revision, err := strconv.ParseUint(*revisionText, 10, 64)
	if err != nil || strconv.FormatUint(revision, 10) != *revisionText {
		return nil, errors.New("PLATFORM_GOVERNANCE_INPUT_INVALID")
	}
	return operator.Change(ctx, logic.GovernanceOperatorInput{User: user, Action: strings.ToUpper(*action), Operation: *operation, Expected: revision})
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/cmd/platform-governance/main.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/04-receipts-governance.md)。
