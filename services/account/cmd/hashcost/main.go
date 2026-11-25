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
