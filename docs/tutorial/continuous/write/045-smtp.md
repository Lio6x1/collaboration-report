# 分步编写 `services/account/internal/client/smtp.go`

先按 [A01 · 安全登录、验证码、注册与凭据变更](../lessons/account/01-authentication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/045-smtp.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

发送验证码邮件，隔离开发明文通道和真实 TLS SMTP，并限制所有网络步骤。

net/smtp 没有 context 参数，因此要在取消时关闭连接并给每个命令设置期限，避免请求退出后仍悬挂。

写之前先按这条执行路线理解各段的连接：

1. smtpConfiguration 验证地址/发件邮箱；development 只允许回环IP且不得配置凭据，tls/starttls要求本地绝对凭据路径。
2. Send 验证输入，读取限8KiB严格JSON凭据，在TLS证书链验证后执行AUTH、MAIL、RCPT和DATA。
3. writer.Close 收到 DATA 最终确认才返回成功；之后 QUIT 失败不重发邮件。

本文件需要守住的边界：

- 不得把 STARTTLS 失败降级为明文认证；不得关闭证书校验。
- 邮件头字段拒绝换行和NUL；只返回固定错误码，避免泄露授权码或服务器原文。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/client/smtp.go') -and (Get-Item -LiteralPath 'services/account/internal/client/smtp.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/client/smtp.go')) { New-Item -ItemType File -Path 'services/account/internal/client/smtp.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:278 -->
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

````

## 第 2 步：smtpTotalTimeout

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`const smtpTotalTimeout = 6 * time.Second`。

<!-- write-step: 2 bytes=278:319 -->
````go
const smtpTotalTimeout = 6 * time.Second
````

## 第 3 步：smtpCommandTimeout

插入位置：第 2 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`const smtpCommandTimeout = 3 * time.Second`。

<!-- write-step: 3 bytes=319:363 -->
````go
const smtpCommandTimeout = 3 * time.Second

````

## 第 4 步：SMTP

插入位置：第 3 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`type SMTP struct{ cfg *platform.SMTPConfig }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>*platform.SMTPConfig</code> | 构造时复制的SMTP配置；nil表示未装配发信通道，不在这里保存读取出的授权密码。 |

<!-- write-step: 4 bytes=363:409 -->
````go
type SMTP struct{ cfg *platform.SMTPConfig }

````

## 第 5 步：NewSMTP

插入位置：第 4 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func NewSMTP(cfg *platform.SMTPConfig) *SMTP`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>*platform.SMTPConfig</code> | 使用 <code>platform.SMTPConfig</code> 类型；其定义处的职责：加载服务自身配置，创建有界数据库池，并以迁移和恢复事实判定就绪。<br>在此声明中的实际使用：<code>cfg == nil</code>；<code>copy := *cfg</code>；<code>cfg: &amp;copy</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*SMTP</code> | 实际返回表达式：<code>&amp;SMTP{}</code>；<code>&amp;SMTP{cfg: &amp;copy}</code> |

**跟着本段代码走一遍：**

- 检查 <code>cfg == nil</code>。条件成立时执行 <code>return &amp;SMTP{}</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;SMTP{cfg: &amp;copy}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=409:536 -->
````go
func NewSMTP(cfg *platform.SMTPConfig) *SMTP {
	if cfg == nil {
		return &SMTP{}
	}
	copy := *cfg
	return &SMTP{cfg: &copy}
}

````

## 第 6 步：smtpCredentials

插入位置：第 5 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`type smtpCredentials struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Username</code> | <code>string</code> | 本地私密凭据文件中的SMTP认证用户名。<br>序列化/校验标记：<code>`json:&quot;username&quot;`</code> |
| <code>Password</code> | <code>string</code> | SMTP授权密码，仅用于TLS通道认证，不能回显或当成用户登录密码。<br>序列化/校验标记：<code>`json:&quot;password&quot;`</code> |

<!-- write-step: 6 bytes=536:639 -->
````go
type smtpCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

````

## 第 7 步：SMTP.DeliveryMode

插入位置：第 6 步代码之后，紧接追加；完成稿约第 41 行。

DeliveryMode只说明实例的投递通道，不说明某个邮箱是否存在、用户名或服务器地址。
找回不存在的账号仍返回同一枚举，不能借邮件提示绕过既有防邮箱枚举规则。

本步声明：`func (s *SMTP) DeliveryMode() string`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>&quot;DEVELOPMENT&quot;</code>；<code>&quot;SMTP&quot;</code> |

**跟着本段代码走一遍：**

- 检查 <code>s == nil &#124;&#124; s.cfg == nil</code>。条件成立时执行 <code>return &quot;&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;</code>。当前函数在这里结束。
- 检查 <code>security == &quot;development&quot;</code>。条件成立时执行 <code>return &quot;DEVELOPMENT&quot;</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;SMTP&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=639:1103 -->
````go
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

````

## 第 8 步：smtpLocalFile

插入位置：第 7 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func smtpLocalFile(path string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>filepath.IsAbs(path)</code>；<code>strings.HasPrefix(path, `\\`)</code>；<code>strings.HasPrefix(path, &quot;//&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>filepath.IsAbs(path) &amp;&amp; !strings.HasPrefix(path, `\\`) &amp;&amp; !strings.HasPrefix(path, &quot;//&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return filepath.IsAbs(path) &amp;&amp; !strings.HasPrefix(path, `\\`) &amp;&amp; !strings.HasPrefix(path, &quot;//&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1103:1337 -->
````go
func smtpLocalFile(path string) bool {
	// UNC路径可能把读取授权码变成远端文件访问；只接受本机绝对路径。
	return filepath.IsAbs(path) && !strings.HasPrefix(path, `\\`) && !strings.HasPrefix(path, "//")
}

````

## 第 9 步：smtpConfiguration

插入位置：第 8 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func smtpConfiguration(cfg platform.SMTPConfig) (string, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>platform.SMTPConfig</code> | 使用 <code>platform.SMTPConfig</code> 类型；其定义处的职责：加载服务自身配置，创建有界数据库池，并以迁移和恢复事实判定就绪。<br>在此声明中的实际使用：<code>cfg.Address</code>；<code>cfg.From</code>；<code>cfg.Security</code>；<code>cfg.CredentialsFile</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>host</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>security</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SMTP_CONFIG_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; portErr != nil &#124;&#124; value == 0 &#124;&#124; host == &quot;&quot; &#124;&#124; strings.ContainsAny(host, &quot;\r\n\x00 &quot;) &#124;&#124; 		!domain.ValidEmail(cfg.From) &#124;&#124; strings.ContainsAny(cfg.From, &quot;\r\n\x00&quot;)</code>。条件成立时执行 <code>return &quot;&quot;, &quot;&quot;, errors.New(&quot;SMTP_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>security == &quot;&quot;</code>。
- 检查 <code>ip == nil &#124;&#124; !ip.IsLoopback() &#124;&#124; cfg.CredentialsFile != &quot;&quot; &#124;&#124; cfg.CAFile != &quot;&quot;</code>。条件成立时执行 <code>return &quot;&quot;, &quot;&quot;, errors.New(&quot;SMTP_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!smtpLocalFile(cfg.CredentialsFile) &#124;&#124; cfg.CAFile != &quot;&quot; &amp;&amp; !smtpLocalFile(cfg.CAFile)</code>。条件成立时执行 <code>return &quot;&quot;, &quot;&quot;, errors.New(&quot;SMTP_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return host, security, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=1337:2389 -->
````go
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

````

## 第 10 步：readSMTPCredentials

插入位置：第 9 步代码之后，紧接追加；完成稿约第 90 行。

本步声明：`func readSMTPCredentials(path string) (smtpCredentials, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>os.Lstat(path)</code>；<code>os.Open(path)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>smtpCredentials</code> | 实际返回表达式：<code>smtpCredentials{}</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; !info.Mode().IsRegular()</code>。条件成立时执行 <code>return smtpCredentials{}, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return smtpCredentials{}, invalid</code>。当前函数在这里结束。
- 登记 <code>file.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 登记 <code>clear(raw)</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; len(raw) &gt; 8192 &#124;&#124; !utf8.Valid(raw)</code>。条件成立时执行 <code>return smtpCredentials{}, invalid</code>。当前函数在这里结束。
- 检查 <code>decoder.Decode(&amp;value) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; 		value.Username == &quot;&quot; &#124;&#124; len(value.Username) &gt; 320 &#124;&#124; value.Password == &quot;&quot; &#124;&#124; len(value.Password) &gt; 4096 &#124;&#124;  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return smtpCredentials{}, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2389:3450 -->
````go
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

````

## 第 11 步：smtpTLSConfig

插入位置：第 10 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func smtpTLSConfig(host, caFile string) (*tls.Config, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>host</code> | <code>string</code> | 在此声明中的实际使用：<code>ServerName: host</code> |
| <code>caFile</code> | <code>string</code> | 在此声明中的实际使用：<code>caFile == &quot;&quot;</code>；<code>os.Open(caFile)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*tls.Config</code> | 实际返回表达式：<code>value</code>；<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;SMTP_CONFIG_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>caFile == &quot;&quot;</code>。条件成立时执行 <code>return value, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SMTP_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 登记 <code>file.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; len(raw) &gt; 128&lt;&lt;10</code>。条件成立时执行 <code>return nil, errors.New(&quot;SMTP_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>!pool.AppendCertsFromPEM(raw)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SMTP_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3450:4303 -->
````go
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

````

## 第 12 步：SMTP.Send

插入位置：第 11 步代码之后，紧接追加；完成稿约第 145 行。

本步声明：`func (s *SMTP) Send(ctx context.Context, email, id, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Err</code>；<code>ctx, cancel := context.WithTimeout(ctx, smtpTotalTimeout)</code>；<code>context.WithTimeout(ctx, smtpTotalTimeout)</code>；<code>(&amp;net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, &quot;tcp&quot;, cfg.Address)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>email</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidEmail(email)</code>；<code>email+id</code>；<code>client.Rcpt(email)</code>；<code>fmt.Sprintf(&quot;Date: %s\r\nMessage-ID: &lt;%s@%s&gt;\r\nFrom: %s\r\nTo: %s\r\nSubject: Collaboration verification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nCha …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>id == &quot;&quot;</code>；<code>email+id</code>；<code>fmt.Sprintf(&quot;Date: %s\r\nMessage-ID: &lt;%s@%s&gt;\r\nFrom: %s\r\nTo: %s\r\nSubject: Collaboration verification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nCha …</code>（长表达式，按本步完整代码继续） |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>code == &quot;&quot;</code>；<code>email+id+code</code>；<code>fmt.Sprintf(&quot;Date: %s\r\nMessage-ID: &lt;%s@%s&gt;\r\nFrom: %s\r\nTo: %s\r\nSubject: Collaboration verification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nCha …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SMTP_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SMTP_INPUT_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SMTP_TLS_FAILED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>s == nil &#124;&#124; s.cfg == nil</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidEmail(email) &#124;&#124; id == &quot;&quot; &#124;&#124; code == &quot;&quot; &#124;&#124; strings.ContainsAny(email+id+code, &quot;\r\n\x00&quot;)</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>security != &quot;development&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, smtpTotalTimeout)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>conn.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 登记 <code>stopCancellation()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return ctx.Err()</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>ok &amp;&amp; absolute.Before(deadline)</code>。

<!-- write-step: 12 bytes=4303:5842 -->
````go
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
````

## 第 13 步：SMTP.Send（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 191 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return step()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>security == &quot;tls&quot;</code>。
- 检查 <code>command(func() error { return encrypted.HandshakeContext(ctx) }) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_TLS_FAILED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return encrypted.HandshakeContext(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>command(func() error { client, err = smtp.NewClient(transport, host); return err }) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 登记 <code>client.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>command(func() error { return client.Hello(&quot;localhost&quot;) }) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return client.Hello(&quot;localhost&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>security == &quot;starttls&quot;</code>。
- 检查 <code>!available &#124;&#124; command(func() error { return client.StartTLS(tlsConfig) }) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_TLS_FAILED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return client.StartTLS(tlsConfig)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>security != &quot;development&quot;</code>。
- 检查 <code>!encrypted &#124;&#124; !state.HandshakeComplete &#124;&#124; len(state.VerifiedChains) == 0</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_TLS_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>mechanism == &quot;PLAIN&quot;</code>。
- 检查 <code>mechanism == &quot;LOGIN&quot;</code>。
- 检查 <code>!available &#124;&#124; auth == nil &#124;&#124; command(func() error { return client.Auth(auth) }) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_AUTH_FAILED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return client.Auth(auth)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5842:7448 -->
````go
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
````

## 第 14 步：SMTP.Send（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 236 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return errors.New(&quot;SMTP_AUTH_FAILED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>command(func() error { return client.Mail(cfg.From) }) != nil &#124;&#124; command(func() error { return client.Rcpt(email) }) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_DELIVERY_FAILED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return client.Mail(cfg.From)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return client.Rcpt(email)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>command(func() error { writer, err = client.Data(); return err }) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_DELIVERY_FAILED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>command(func() error { _, err := io.WriteString(writer, message); return err }) != nil &#124;&#124; command(writer.Close) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SMTP_DELIVERY_FAILED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=7448:8778 -->
````go
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

````

## 第 15 步：smtpLoginAuth

插入位置：第 14 步代码之后，紧接追加；完成稿约第 258 行。

LOGIN用于只提供该机制的SMTP（包括部分邮箱服务）；PLAIN优先沿Go标准实现。
两种认证都在已核验证书的TLS之后调用，不能沿PlainAuth的localhost例外发凭据。

本步声明：`type smtpLoginAuth struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>host</code> | <code>string</code> | 配置中的SMTP主机名；Start还要求服务器报告的名称一致并已启用TLS。 |
| <code>credentials</code> | <code>smtpCredentials</code> | SMTP认证用用户名和密码，来自私密配置，不能进入HTTP响应或日志。 |
| <code>step</code> | <code>int</code> | LOGIN认证的挑战步骤计数；第1步发用户名、第2步发密码，其他步骤拒绝。 |

<!-- write-step: 15 bytes=8778:9085 -->
````go
// LOGIN用于只提供该机制的SMTP（包括部分邮箱服务）；PLAIN优先沿Go标准实现。
// 两种认证都在已核验证书的TLS之后调用，不能沿PlainAuth的localhost例外发凭据。
type smtpLoginAuth struct {
	host        string
	credentials smtpCredentials
	step        int
}

````

## 第 16 步：smtpLoginAuth.Start

插入位置：第 15 步代码之后，紧接追加；完成稿约第 266 行。

本步声明：`func (a *smtpLoginAuth) Start(server *smtp.ServerInfo) (string, []byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*smtp.ServerInfo</code> | 在此声明中的实际使用：<code>server.TLS</code>；<code>server.Name</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>&quot;LOGIN&quot;</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SMTP_AUTH_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!server.TLS &#124;&#124; server.Name != a.host</code>。条件成立时执行 <code>return &quot;&quot;, nil, errors.New(&quot;SMTP_AUTH_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;LOGIN&quot;, nil, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=9085:9290 -->
````go
func (a *smtpLoginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS || server.Name != a.host {
		return "", nil, errors.New("SMTP_AUTH_FAILED")
	}
	return "LOGIN", nil, nil
}

````

## 第 17 步：smtpLoginAuth.Next

插入位置：第 16 步代码之后，紧接追加；完成稿约第 273 行。

本步声明：`func (a *smtpLoginAuth) Next(_ []byte, more bool) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>_</code> | <code>&#91;&#93;byte</code> | 这个位置的输入按接口约定接收，但本实现明确不使用它；下划线避免产生未使用变量。 |
| <code>more</code> | <code>bool</code> | 在此声明中的实际使用：<code>!more</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>&#91;&#93;byte(a.credentials.Username)</code>；<code>&#91;&#93;byte(a.credentials.Password)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;SMTP_AUTH_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!more</code>。条件成立时执行 <code>return nil, nil</code>。当前函数在这里结束。
- 此处通过 <code>return &#91;&#93;byte(a.credentials.Username), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &#91;&#93;byte(a.credentials.Password), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil, errors.New(&quot;SMTP_AUTH_FAILED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=9290:9586 -->
````go
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

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/client/smtp.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/01-authentication.md)。
