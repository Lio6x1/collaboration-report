# services/document/internal/storage/oss/credentials.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/132-credentials.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：按明确授权操作资源；不将内部存储路径当作公开下载链接。

主要声明：`privateCredentials`、`readCredentials`、`credentialText`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`c7cb3ebf2a5582830d92eaf9345fcc9b81baec49dd057a174d23155301533ef5`。[参考实现](../../../../services/document/internal/storage/oss/credentials.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
