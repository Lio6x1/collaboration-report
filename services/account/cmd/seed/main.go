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
