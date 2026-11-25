# services/account/cmd/hashcost/main.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/085-main.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`summary`、`main`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

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

func summary(v []float64) map[string]float64 {
	sort.Float64s(v)
	return map[string]float64{"median_ms": v[len(v)/2], "p95_ms": v[(len(v)*95+99)/100-1]}
}
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

UTF-8/LF全文SHA-256：`852535465b293a1ad3123a21f4e987f11ce21ecc688f342fdb377187df334ca1`。[参考实现](../../../../services/account/cmd/hashcost/main.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
