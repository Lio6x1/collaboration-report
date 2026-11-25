# services/account/internal/client/smtp.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/045-smtp.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`SMTP`、`NewSMTP`、`smtpCredentials`、`DeliveryMode`、`smtpLocalFile`、`smtpConfiguration`、`readSMTPCredentials`、`smtpTLSConfig`、`Send`、`smtpLoginAuth`、`Start`、`Next`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package client

import (
	domain "collaboration/account/internal/domain/account"
	"collaboration/platform"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const smtpTotalTimeout = 6 * time.Second
const smtpCommandTimeout = 3 * time.Second

type SMTP struct{ cfg *platform.SMTPConfig }

func NewSMTP(cfg *platform.SMTPConfig) *SMTP {
	if cfg == nil {
		return &SMTP{}
	}
	copy := *cfg
	return &SMTP{cfg: &copy}
}

type smtpCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DeliveryMode只说明实例的投递通道，不说明某个邮箱是否存在、用户名或服务器地址。
// 找回不存在的账号仍返回同一枚举，不能借邮件提示绕过既有防邮箱枚举规则。
func (s *SMTP) DeliveryMode() string {
	if s == nil || s.cfg == nil {
		return ""
	}
	_, security, err := smtpConfiguration(*s.cfg)
	if err != nil {
		return ""
	}
	if security == "development" {
		return "DEVELOPMENT"
	}
	return "SMTP"
}

func smtpLocalFile(path string) bool {
	// UNC路径可能把读取授权码变成远端文件访问；只接受本机绝对路径。
	return filepath.IsAbs(path) && !strings.HasPrefix(path, `\\`) && !strings.HasPrefix(path, "//")
}

func smtpConfiguration(cfg platform.SMTPConfig) (string, string, error) {
	host, port, err := net.SplitHostPort(cfg.Address)
	value, portErr := strconv.ParseUint(port, 10, 16)
	if err != nil || portErr != nil || value == 0 || host == "" || strings.ContainsAny(host, "\r\n\x00 ") ||
		!domain.ValidEmail(cfg.From) || strings.ContainsAny(cfg.From, "\r\n\x00") {
		return "", "", errors.New("SMTP_CONFIG_INVALID")
	}
	security := cfg.Security
	if security == "" {
		security = "development"
	}
	switch security {
	case "development":
		ip := net.ParseIP(host)
		// 明文不依赖DNS的localhost解释，也绝不加载或发送认证凭据。
		if ip == nil || !ip.IsLoopback() || cfg.CredentialsFile != "" || cfg.CAFile != "" {
			return "", "", errors.New("SMTP_CONFIG_INVALID")
		}
	case "tls", "starttls":
		if !smtpLocalFile(cfg.CredentialsFile) || cfg.CAFile != "" && !smtpLocalFile(cfg.CAFile) {
			return "", "", errors.New("SMTP_CONFIG_INVALID")
		}
	default:
		return "", "", errors.New("SMTP_CONFIG_INVALID")
	}
	return host, security, nil
}

func readSMTPCredentials(path string) (smtpCredentials, error) {
	invalid := errors.New("SMTP_CREDENTIALS_INVALID")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return smtpCredentials{}, invalid
	}
	file, err := os.Open(path)
	if err != nil {
		return smtpCredentials{}, invalid
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 8193))
	defer clear(raw)
	if err != nil || len(raw) > 8192 || !utf8.Valid(raw) {
		return smtpCredentials{}, invalid
	}
	// PowerShell生成的UTF-8文件可能带BOM；只去掉文件开头的BOM，不改密码字符。
	decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(string(raw), "\uFEFF")))
	decoder.DisallowUnknownFields()
	var value smtpCredentials
	if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF ||
		value.Username == "" || len(value.Username) > 320 || value.Password == "" || len(value.Password) > 4096 ||
		strings.ContainsAny(value.Username+value.Password, "\r\n\x00") {
		return smtpCredentials{}, invalid
	}
	return value, nil
}

func smtpTLSConfig(host, caFile string) (*tls.Config, error) {
	value := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if caFile == "" {
		return value, nil
	}
	// 公共服务沿系统信任；私有SMTP可显式追加CA，始终核验目标主机和证书链。
	// 此配置没有关闭证书校验的开关，也不借用业务服务mTLS的CA身份。
	file, err := os.Open(caFile)
	if err != nil {
		return nil, errors.New("SMTP_CONFIG_INVALID")
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (128<<10)+1))
	if err != nil || len(raw) > 128<<10 {
		return nil, errors.New("SMTP_CONFIG_INVALID")
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(raw) {
		return nil, errors.New("SMTP_CONFIG_INVALID")
	}
	value.RootCAs = pool
	return value, nil
}

func (s *SMTP) Send(ctx context.Context, email, id, code string) error {
	if s == nil || s.cfg == nil {
		return errors.New("SMTP_UNAVAILABLE")
	}
	// 在建立连接之前拒绝完整邮箱与头注入；不能依赖handler永远是唯一调用者。
	if !domain.ValidEmail(email) || id == "" || code == "" || strings.ContainsAny(email+id+code, "\r\n\x00") {
		return errors.New("SMTP_INPUT_INVALID")
	}
	if ctx.Err() != nil {
		return errors.New("SMTP_UNAVAILABLE")
	}
	cfg := *s.cfg
	host, security, err := smtpConfiguration(cfg)
	if err != nil {
		return err
	}
	var credentials smtpCredentials
	var tlsConfig *tls.Config
	if security != "development" {
		credentials, err = readSMTPCredentials(cfg.CredentialsFile)
		if err != nil {
			return err
		}
		tlsConfig, err = smtpTLSConfig(host, cfg.CAFile)
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, smtpTotalTimeout)
	defer cancel()
	conn, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp", cfg.Address)
	if err != nil {
		return errors.New("SMTP_UNAVAILABLE")
	}
	defer conn.Close()
	// net/smtp没有ctx参数。取消主动关闭底层连接，避免用户退出后仍等到命令期限。
	stopCancellation := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopCancellation()
	command := func(step func() error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		deadline := time.Now().Add(smtpCommandTimeout)
		if absolute, ok := ctx.Deadline(); ok && absolute.Before(deadline) {
			deadline = absolute
		}
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
		return step()
	}
	var transport net.Conn = conn
	if security == "tls" {
		encrypted := tls.Client(conn, tlsConfig)
		if command(func() error { return encrypted.HandshakeContext(ctx) }) != nil {
			return errors.New("SMTP_TLS_FAILED")
		}
		transport = encrypted
	}
	var client *smtp.Client
	if command(func() error { client, err = smtp.NewClient(transport, host); return err }) != nil {
		return errors.New("SMTP_UNAVAILABLE")
	}
	defer client.Close()
	if command(func() error { return client.Hello("localhost") }) != nil {
		return errors.New("SMTP_UNAVAILABLE")
	}
	if security == "starttls" {
		available, _ := client.Extension("STARTTLS")
		// 远程服务器没有升级能力或升级失败必须中止，不能回退明文MAIL/AUTH。
		if !available || command(func() error { return client.StartTLS(tlsConfig) }) != nil {
			return errors.New("SMTP_TLS_FAILED")
		}
	}
	if security != "development" {
		state, encrypted := client.TLSConnectionState()
		if !encrypted || !state.HandshakeComplete || len(state.VerifiedChains) == 0 {
			return errors.New("SMTP_TLS_FAILED")
		}
		available, mechanisms := client.Extension("AUTH")
		var auth smtp.Auth
		for _, mechanism := range strings.Fields(mechanisms) {
			if mechanism == "PLAIN" {
				auth = smtp.PlainAuth("", credentials.Username, credentials.Password, host)
				break
			}
			if mechanism == "LOGIN" {
				auth = &smtpLoginAuth{host: host, credentials: credentials}
			}
		}
		if !available || auth == nil || command(func() error { return client.Auth(auth) }) != nil {
			return errors.New("SMTP_AUTH_FAILED")
		}
	}
	if command(func() error { return client.Mail(cfg.From) }) != nil || command(func() error { return client.Rcpt(email) }) != nil {
		return errors.New("SMTP_DELIVERY_FAILED")
	}
	var writer io.WriteCloser
	if command(func() error { writer, err = client.Data(); return err }) != nil {
		return errors.New("SMTP_DELIVERY_FAILED")
	}
	// 真实邮件补齐Date/Message-ID和MIME头；消息身份由本机生成，不能把用户字段拼进头。
	senderDomain := cfg.From[strings.LastIndexByte(cfg.From, '@')+1:]
	message := fmt.Sprintf("Date: %s\r\nMessage-ID: <%s@%s>\r\nFrom: %s\r\nTo: %s\r\nSubject: Collaboration verification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nChallenge: %s\r\nCode: %s\r\nExpires: 10 minutes\r\n", time.Now().UTC().Format(time.RFC1123Z), platform.NewTraceID(), senderDomain, cfg.From, email, id, code)
	if command(func() error { _, err := io.WriteString(writer, message); return err }) != nil || command(writer.Close) != nil {
		return errors.New("SMTP_DELIVERY_FAILED")
	}
	// Close收到DATA的最终250才确认服务器接收。QUIT失败不推翻已确认事实，
	// 也不重发验证码；所有网络/认证错误仅返回固定码，不泄露服务器原文或授权码。
	_ = command(client.Quit)
	return nil
}

// LOGIN用于只提供该机制的SMTP（包括部分邮箱服务）；PLAIN优先沿Go标准实现。
// 两种认证都在已核验证书的TLS之后调用，不能沿PlainAuth的localhost例外发凭据。
type smtpLoginAuth struct {
	host        string
	credentials smtpCredentials
	step        int
}

func (a *smtpLoginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS || server.Name != a.host {
		return "", nil, errors.New("SMTP_AUTH_FAILED")
	}
	return "LOGIN", nil, nil
}

func (a *smtpLoginAuth) Next(_ []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	a.step++
	switch a.step {
	case 1:
		return []byte(a.credentials.Username), nil
	case 2:
		return []byte(a.credentials.Password), nil
	default:
		return nil, errors.New("SMTP_AUTH_FAILED")
	}
}
````

UTF-8/LF全文SHA-256：`76e775d28168d4b68e0c96328bead59ed48a2c2d4778d1ffd2595b865e1b600c`。[参考实现](../../../../services/account/internal/client/smtp.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
