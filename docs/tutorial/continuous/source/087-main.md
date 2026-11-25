# services/account/cmd/seed/main.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/087-main.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`main`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

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

UTF-8/LF全文SHA-256：`9fa1df01b5ebdd2e7b8f802050cdafc861272ff4e586b9a4203a2fca5942ede3`。[参考实现](../../../../services/account/cmd/seed/main.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
