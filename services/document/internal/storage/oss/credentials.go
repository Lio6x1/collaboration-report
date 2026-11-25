package oss

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type privateCredentials struct {
	ID     string `json:"accessKeyId"`
	Secret string `json:"accessKeySecret"`
	Token  string `json:"securityToken"`
}

func readCredentials(path string) (privateCredentials, error) {
	var secret privateCredentials
	if !filepath.IsAbs(path) {
		return secret, ErrCredentials
	}
	// 凭据不允许通过链接/目录替换读取；Windows ACL 由配置脚本和运行说明收紧，
	// Unix 另拒绝 group/other 可读文件。错误不带路径、内容或解析片段。
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return secret, ErrCredentials
		}
		if current == filepath.Dir(current) {
			break
		}
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() {
		return secret, ErrCredentials
	}
	file, err := os.Open(path)
	if err != nil {
		return secret, ErrCredentials
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(before, info) || info.Size() > 4096 || !privateMode(info.Mode()) {
		return secret, ErrCredentials
	}
	raw, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(raw) > 4096 {
		return secret, ErrCredentials
	}
	// Windows PowerShell 5 的 UTF8 文本可能带 BOM，允许该编码而非要求用户换终端。
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&secret) != nil || decoder.Decode(new(any)) != io.EOF ||
		!credentialText(secret.ID, 16, 128) || !credentialText(secret.Secret, 16, 256) ||
		(secret.Token != "" && !credentialText(secret.Token, 1, 2048)) {
		return privateCredentials{}, ErrCredentials
	}
	return secret, nil
}

func credentialText(value string, min, max int) bool {
	if len(value) < min || len(value) > max || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 33 || character > 126 {
			return false
		}
	}
	return true
}
