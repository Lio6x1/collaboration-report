# 分步编写 `services/account/cmd/hashcost/main.go`

先按 [A01 · 安全登录、验证码、注册与凭据变更](../lessons/account/01-authentication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/085-main.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

用固定测试字符串测量本机 bcrypt cost=10 的生成和验证成本。

成本必须用实际机器测量，不能靠开发者主观估计；输出仅含性能指标。

写之前先按这条执行路线理解各段的连接：

1. main 各采样20次生成与比较，summary 排序得到中位数和p95。
2. 任一p95超过500ms输出FAIL并退出1，否则输出PASS和系统/Go版本。

本文件需要守住的边界：

- 本命令不用真实用户密码；测量通过只说明本机预算，不代表生产吞吐验收。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/cmd/hashcost' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/cmd/hashcost/main.go') -and (Get-Item -LiteralPath 'services/account/cmd/hashcost/main.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/cmd/hashcost/main.go')) { New-Item -ItemType File -Path 'services/account/cmd/hashcost/main.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:220 -->
````go
// hashcost在实际开发机器采样密码哈希成本，结果只记录耗时、不记录任何用户凭据。
package main

import (
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"os"
	"runtime"
	"sort"
	"time"
)

````

## 第 2 步：summary

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`func summary(v []float64) map[string]float64`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>v</code> | <code>&#91;&#93;float64</code> | 在此声明中的实际使用：<code>sort.Float64s(v)</code>；<code>&quot;median_ms&quot;: v&#91;len(v)/2&#93;</code>；<code>len(v)</code>；<code>&quot;p95_ms&quot;: v&#91;(len(v)*95+99)/100-1&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;float64</code> | 实际返回表达式：<code>map&#91;string&#93;float64{&quot;median_ms&quot;: v&#91;len(v)/2&#93;, &quot;p95_ms&quot;: v&#91;(len(v)*95+99)/100-1&#93;}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return map&#91;string&#93;float64{&quot;median_ms&quot;: v&#91;len(v)/2&#93;, &quot;p95_ms&quot;: v&#91;(len(v)*95+99)/100-1&#93;}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=220:375 -->
````go
func summary(v []float64) map[string]float64 {
	sort.Float64s(v)
	return map[string]float64{"median_ms": v[len(v)/2], "p95_ms": v[(len(v)*95+99)/100-1]}
}
````

## 第 3 步：main

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func main()`。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 检查 <code>bcrypt.CompareHashAndPassword(hash, password) != nil</code>。
- 检查 <code>h&#91;&quot;p95_ms&quot;&#93; &gt; 500 &#124;&#124; v&#91;&quot;p95_ms&quot;&#93; &gt; 500</code>。
- 检查 <code>state != &quot;PASS&quot;</code>。

<!-- write-step: 3 bytes=375:1344 -->
````go
func main() {
	const count = 20
	hashTimes, verifyTimes := []float64{}, []float64{}
	password := []byte("cost-sample-only-no-user-credential")
	for n := 0; n < count; n++ {
		start := time.Now()
		hash, err := bcrypt.GenerateFromPassword(password, 10)
		if err != nil {
			os.Exit(1)
		}
		hashTimes = append(hashTimes, float64(time.Since(start).Microseconds())/1000)
		start = time.Now()
		if bcrypt.CompareHashAndPassword(hash, password) != nil {
			os.Exit(1)
		}
		verifyTimes = append(verifyTimes, float64(time.Since(start).Microseconds())/1000)
	}
	h, v := summary(hashTimes), summary(verifyTimes)
	state := "PASS"
	if h["p95_ms"] > 500 || v["p95_ms"] > 500 {
		state = "FAIL"
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"state": state, "algorithm": "bcrypt", "cost": 10, "samples": count, "os": runtime.GOOS, "arch": runtime.GOARCH, "go_version": runtime.Version(), "hash": h, "verify": v, "budget_p95_ms": 500})
	if state != "PASS" {
		os.Exit(1)
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/cmd/hashcost/main.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/01-authentication.md)。
