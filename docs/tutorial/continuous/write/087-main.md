# 分步编写 `services/account/cmd/seed/main.go`

先按 [A01 · 安全登录、验证码、注册与凭据变更](../lessons/account/01-authentication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/087-main.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

为隔离开发库生成一份学习账号及本机密码文件。

课程初期需要可体验账号；随机密码只保存在本地文件，终端仅告知路径。

写之前先按这条执行路线理解各段的连接：

1. main 校验 --config 参数并打开Account库；已有 .runtime/login-seed.json 则返回EXISTS。
2. 生成随机密码和bcrypt摘要，INSERT user，成功后写0600密码文件及哈希耗时记录。

本文件需要守住的边界：

- 数据库写入后文件写入失败可能留下账号，应检查状态而非覆盖或清库。
- 不要把运行生成的密码文件或实例配置放入教程。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/cmd/seed' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/cmd/seed/main.go') -and (Get-Item -LiteralPath 'services/account/cmd/seed/main.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/cmd/seed/main.go')) { New-Item -ItemType File -Path 'services/account/cmd/seed/main.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:266 -->
````go
// seed仅给隔离开发环境创建学习账号，产品注册流程在后续批次实现。
package main

import (
	"collaboration/platform"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"os"
	"path/filepath"
	"time"
)

````

## 第 2 步：main

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`func main()`。

**跟着本段代码走一遍：**

- 检查 <code>len(os.Args) != 3 &#124;&#124; os.Args&#91;1&#93; != &quot;--config&quot;</code>。
- 检查 <code>err != nil</code>。
- 登记 <code>store.DB.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err == nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>os.WriteFile(file, data, 0600) != nil</code>。

<!-- write-step: 2 bytes=266:1915 -->
````go
func main() {
	if len(os.Args) != 3 || os.Args[1] != "--config" {
		os.Exit(1)
	}
	cfg, err := platform.LoadConfig(os.Args[2])
	if err != nil {
		os.Exit(1)
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		os.Exit(1)
	}
	defer store.DB.Close()
	file := filepath.Join(cfg.Root, ".runtime", "login-seed.json")
	if _, err := os.Stat(file); err == nil {
		fmt.Println(`{"state":"EXISTS","passwordFile":".runtime/login-seed.json"}`)
		return
	}
	raw := make([]byte, 24)
	if _, err = rand.Read(raw); err != nil {
		os.Exit(1)
	}
	password := base64.RawURLEncoding.EncodeToString(raw)
	start := time.Now()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		os.Exit(1)
	}
	email := "learner@example.test"
	result, err := store.DB.Exec("INSERT INTO user(email,email_key,display_name,password_hash,password_algorithm,state,email_verified_at) VALUES(?,?,?,?,'bcrypt','ACTIVE',UTC_TIMESTAMP(6))", email, []byte(email), "学习账号", hash)
	if err != nil {
		fmt.Println(`{"code":"SEED_REQUIRES_INSPECTION"}`)
		os.Exit(1)
	}
	id, _ := result.LastInsertId()
	data, _ := json.MarshalIndent(map[string]any{"email": email, "password": password, "user_id": fmt.Sprint(id)}, "", "  ")
	if os.WriteFile(file, data, 0600) != nil {
		os.Exit(1)
	}
	bench, _ := json.Marshal(map[string]any{"cost": 10, "hashMilliseconds": elapsed, "measuredAt": time.Now().UTC()})
	_ = os.WriteFile(filepath.Join(cfg.Root, "artifacts", "batches", "02-login", "bcrypt.json"), bench, 0600)
	fmt.Println(`{"state":"READY","email":"learner@example.test","passwordFile":".runtime/login-seed.json"}`)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/cmd/seed/main.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/01-authentication.md)。
