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
