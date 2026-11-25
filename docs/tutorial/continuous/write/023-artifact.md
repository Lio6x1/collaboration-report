# 分步编写 `packages/platform/rebuildartifact/artifact.go`

先按 [F05 · 重建元数据、签名与原子制品](../lessons/foundation/05-rebuild-artifacts.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/023-artifact.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

为离线重建制品签名、首次验签及恢复已接受原字节。

制品可离线交接，但身份必须来自本机配置的信任根；RSA-PSS随机盐意味着重签会改变内容地址。

写之前先按这条执行路线理解各段的连接：

1. NewSigner检查RSA私钥与证书公钥一致；purposeRole限定prepare由Search签、snapshot由Document签。
2. Verifier.Verify核CA/精确单URI/clientAuth/有效期与PSS签名，然后计算envelope内容地址。
3. RestoreAccepted仅比对原库可信envelope与signer摘要，恢复曾验签接受的同一字节。

本文件需要守住的边界：

- 首次接受不得调用RestoreAccepted绕过验签；可信摘要不能来自CLI输入。
- 长时间恢复不重签原制品，证书过期不能改写已接受事实。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform/rebuildartifact' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/rebuildartifact/artifact.go') -and (Get-Item -LiteralPath 'packages/platform/rebuildartifact/artifact.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/rebuildartifact/artifact.go')) { New-Item -ItemType File -Path 'packages/platform/rebuildartifact/artifact.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:447 -->
````go
// Package rebuildartifact 提供重建 CLI 的离线身份签名。调用方须持久化首次生成的
// 原 envelope；RSA-PSS 带随机盐，重试重签会改变内容地址，不能替代原操作输入。
package rebuildartifact

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"time"
)

````

## 第 2 步：PrepareV1、SnapshotV1、MaxPayloadBytes、MaxEnvelopeBytes、maxCertificateBytes、maxChainLength

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`const (`。

<!-- write-step: 2 bytes=447:650 -->
````go
const (
	PrepareV1           = "prepare-v1"
	SnapshotV1          = "snapshot-v1"
	MaxPayloadBytes     = 1 << 20
	MaxEnvelopeBytes    = 2 << 20
	maxCertificateBytes = 32 << 10
	maxChainLength      = 8
)

````

## 第 3 步：ErrConfig、ErrEnvelope、ErrIdentity、ErrSignature

插入位置：第 2 步代码之后，紧接追加；完成稿约第 32 行。

固定错误避免将本服务密钥位置、证书内容及底层解析错误传播到 CLI 输出。

本步声明：`var (`。

<!-- write-step: 3 bytes=650:1019 -->
````go
// 固定错误避免将本服务密钥位置、证书内容及底层解析错误传播到 CLI 输出。
var (
	ErrConfig    = errors.New("REBUILD_ARTIFACT_CONFIG_INVALID")
	ErrEnvelope  = errors.New("REBUILD_ARTIFACT_ENVELOPE_INVALID")
	ErrIdentity  = errors.New("REBUILD_ARTIFACT_IDENTITY_INVALID")
	ErrSignature = errors.New("REBUILD_ARTIFACT_SIGNATURE_INVALID")
)

````

## 第 4 步：envelope

插入位置：第 3 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`type envelope struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Purpose</code> | <code>string</code> | prepare-v1或snapshot-v1用途，决定允许的签名服务角色。<br>序列化/校验标记：<code>`json:&quot;purpose&quot;`</code> |
| <code>Certificates</code> | <code>&#91;&#93;&#91;&#93;byte</code> | 叶证书在前的DER证书链，最多8张；信任根仍来自本地配置。<br>序列化/校验标记：<code>`json:&quot;certificates&quot;`</code> |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 原规范Prepare或Seal字节，最多1MiB。<br>序列化/校验标记：<code>`json:&quot;payload&quot;`</code> |
| <code>Signature</code> | <code>&#91;&#93;byte</code> | 对域隔离摘要的RSA-PSS签名，随机盐使首次envelope必须持久保存。<br>序列化/校验标记：<code>`json:&quot;signature&quot;`</code> |

<!-- write-step: 4 bytes=1019:1212 -->
````go
type envelope struct {
	Purpose      string   `json:"purpose"`
	Certificates [][]byte `json:"certificates"`
	Payload      []byte   `json:"payload"`
	Signature    []byte   `json:"signature"`
}

````

## 第 5 步：Signer

插入位置：第 4 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`type Signer struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>*rsa.PrivateKey</code> | 已验证与叶证书配对的本服务RSA私钥。 |
| <code>certificates</code> | <code>&#91;&#93;&#91;&#93;byte</code> | 原DER证书链，签名时写入envelope。 |
| <code>role</code> | <code>string</code> | search或document，与purposeRole必须匹配。 |
| <code>verifier</code> | <code>*Verifier</code> | 用本地CA构造的验证器，签名时再次检查证书仍有效。 |

<!-- write-step: 5 bytes=1212:1334 -->
````go
type Signer struct {
	key          *rsa.PrivateKey
	certificates [][]byte
	role         string
	verifier     *Verifier
}

````

## 第 6 步：Verifier

插入位置：第 5 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`type Verifier struct{ roots *x509.CertPool }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>roots</code> | <code>*x509.CertPool</code> | 仅从本地显式CA文件加载的信任根，禁止从制品决定根。 |

<!-- write-step: 6 bytes=1334:1380 -->
````go
type Verifier struct{ roots *x509.CertPool }

````

## 第 7 步：Verified

插入位置：第 6 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`type Verified struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 通过首次验签或可信持久恢复后得到的原业务元数据字节。 |
| <code>Reference</code> | <code>string</code> | 整个原envelope的内容地址，包含用途与SHA256。 |
| <code>SignerFingerprint</code> | <code>string</code> | 叶证书原DER SHA256 hex，接受时与envelope摘要一起存库。 |
| <code>Role</code> | <code>string</code> | 由用途与已验证证书确定的search/document身份。 |

<!-- write-step: 7 bytes=1380:1510 -->
````go
type Verified struct {
	Payload           []byte
	Reference         string
	SignerFingerprint string
	Role              string
}

````

## 第 8 步：NewSigner

插入位置：第 7 步代码之后，紧接追加；完成稿约第 63 行。

NewSigner 只读取显式配置的本服务 PEM 文件；不从制品中选择信任根或私钥。
certPath 的 PEM 顺序是叶证书在前，其余为中间证书。

本步声明：`func NewSigner(certPath, keyPath, caPath, role string) (*Signer, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>certPath</code> | <code>string</code> | 在此声明中的实际使用：<code>readBounded(certPath)</code> |
| <code>keyPath</code> | <code>string</code> | 在此声明中的实际使用：<code>readBounded(keyPath)</code> |
| <code>caPath</code> | <code>string</code> | 在此声明中的实际使用：<code>NewVerifier(caPath)</code> |
| <code>role</code> | <code>string</code> | 在此声明中的实际使用：<code>role != &quot;search&quot;</code>；<code>role != &quot;document&quot;</code>；<code>v.identity(certs, role)</code>；<code>role: role</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Signer</code> | 实际返回表达式：<code>nil</code>；<code>&amp;Signer{key: key, certificates: certs, role: role, verifier: v}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrConfig</code>；<code>err</code>；<code>ErrIdentity</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>role != &quot;search&quot; &amp;&amp; role != &quot;document&quot;</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 检查 <code>block == nil &#124;&#124; len(bytes.TrimSpace(rest)) != 0</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; key == nil &#124;&#124; key.Validate() != nil</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!pub.Equal(&amp;key.PublicKey)</code>。条件成立时执行 <code>return nil, ErrIdentity</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=1510:2749 -->
````go
// NewSigner 只读取显式配置的本服务 PEM 文件；不从制品中选择信任根或私钥。
// certPath 的 PEM 顺序是叶证书在前，其余为中间证书。
func NewSigner(certPath, keyPath, caPath, role string) (*Signer, error) {
	if role != "search" && role != "document" {
		return nil, ErrConfig
	}
	v, err := NewVerifier(caPath)
	if err != nil {
		return nil, err
	}
	certPEM, err := readBounded(certPath)
	if err != nil {
		return nil, ErrConfig
	}
	certs, err := parseCertificates(certPEM)
	if err != nil {
		return nil, ErrConfig
	}
	keyPEM, err := readBounded(keyPath)
	if err != nil {
		return nil, ErrConfig
	}
	block, rest := pem.Decode(keyPEM)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, ErrConfig
	}
	var key *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		var parsed any
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		key, _ = parsed.(*rsa.PrivateKey)
	default:
		return nil, ErrConfig
	}
	if err != nil || key == nil || key.Validate() != nil {
		return nil, ErrConfig
	}
	pub, _, err := v.identity(certs, role)
	if err != nil {
		return nil, err
	}
	if !pub.Equal(&key.PublicKey) {
````

## 第 9 步：NewSigner（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 108 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, ErrIdentity</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &amp;Signer{key: key, certificates: certs, role: role, verifier: v}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2749:2858 -->
````go
		return nil, ErrIdentity
	}
	return &Signer{key: key, certificates: certs, role: role, verifier: v}, nil
}

````

## 第 10 步：NewVerifier

插入位置：第 9 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`func NewVerifier(caPath string) (*Verifier, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>caPath</code> | <code>string</code> | 在此声明中的实际使用：<code>readBounded(caPath)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Verifier</code> | 实际返回表达式：<code>nil</code>；<code>&amp;Verifier{roots: roots}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrConfig</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !cert.IsCA</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Verifier{roots: roots}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2858:3298 -->
````go
func NewVerifier(caPath string) (*Verifier, error) {
	data, err := readBounded(caPath)
	if err != nil {
		return nil, ErrConfig
	}
	certs, err := parseCertificates(data)
	if err != nil {
		return nil, ErrConfig
	}
	roots := x509.NewCertPool()
	for _, der := range certs {
		cert, err := x509.ParseCertificate(der)
		if err != nil || !cert.IsCA {
			return nil, ErrConfig
		}
		roots.AddCert(cert)
	}
	return &Verifier{roots: roots}, nil
}

````

## 第 11 步：Signer.Sign

插入位置：第 10 步代码之后，紧接追加；完成稿约第 133 行。

本步声明：`func (s *Signer) Sign(purpose string, payload []byte) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>purposeRole(purpose)</code>；<code>signingDigest(purpose, payload)</code>；<code>Purpose: purpose</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(payload)</code>；<code>signingDigest(purpose, payload)</code>；<code>append(&#91;&#93;byte{}, payload...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrEnvelope</code>；<code>ErrIdentity</code>；<code>err</code>；<code>ErrSignature</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>role == &quot;&quot; &#124;&#124; len(payload) &gt; MaxPayloadBytes</code>。条件成立时执行 <code>return nil, ErrEnvelope</code>。当前函数在这里结束。
- 检查 <code>s == nil &#124;&#124; s.role != role</code>。条件成立时执行 <code>return nil, ErrIdentity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrSignature</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(result) &gt; MaxEnvelopeBytes</code>。条件成立时执行 <code>return nil, ErrEnvelope</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3298:4182 -->
````go
func (s *Signer) Sign(purpose string, payload []byte) ([]byte, error) {
	role := purposeRole(purpose)
	if role == "" || len(payload) > MaxPayloadBytes {
		return nil, ErrEnvelope
	}
	if s == nil || s.role != role {
		return nil, ErrIdentity
	}
	// 长寿命 CLI 也可能跨过证书到期点，不能仅在构造时校验。
	if _, _, err := s.verifier.identity(s.certificates, role); err != nil {
		return nil, err
	}
	digest := signingDigest(purpose, payload)
	sig, err := rsa.SignPSS(rand.Reader, s.key, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
	if err != nil {
		return nil, ErrSignature
	}
	result, err := json.Marshal(envelope{Purpose: purpose, Certificates: s.certificates, Payload: append([]byte{}, payload...), Signature: sig})
	if err != nil || len(result) > MaxEnvelopeBytes {
		return nil, ErrEnvelope
	}
	return result, nil
}

````

## 第 12 步：Verifier.Verify

插入位置：第 11 步代码之后，紧接追加；完成稿约第 157 行。

Verify 用当前可信时钟验证首次接受的制品，不接受签名者自报时间。
已接受制品的长期恢复应由调用方核对持久化接受事实与原始摘要，而非重新签名。

本步声明：`func (v *Verifier) Verify(raw []byte, purpose string) (Verified, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>decodeEnvelope(raw)</code>；<code>sha256.Sum256(raw)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>purposeRole(purpose)</code>；<code>e.Purpose != purpose</code>；<code>signingDigest(purpose, e.Payload)</code>；<code>&quot;rebuild-&quot; + purpose</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Verified</code> | 实际返回表达式：<code>Verified{}</code>；<code>Verified{Payload: e.Payload, Reference: &quot;rebuild-&quot; + purpose + &quot;:&quot; + hex.EncodeToString(hash&#91;:&#93;), SignerFingerprint: fingerprint, Role: role}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrEnvelope</code>；<code>err</code>；<code>ErrSignature</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>role == &quot;&quot;</code>。条件成立时执行 <code>return Verified{}, ErrEnvelope</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; e.Purpose != purpose</code>。条件成立时执行 <code>return Verified{}, ErrEnvelope</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return Verified{}, err</code>。当前函数在这里结束。
- 检查 <code>len(e.Signature) != pub.Size()</code>。条件成立时执行 <code>return Verified{}, ErrSignature</code>。当前函数在这里结束。
- 检查 <code>rsa.VerifyPSS(pub, crypto.SHA256, digest&#91;:&#93;, e.Signature, &amp;rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}) != nil</code>。条件成立时执行 <code>return Verified{}, ErrSignature</code>。当前函数在这里结束。
- 此处通过 <code>return Verified{Payload: e.Payload, Reference: &quot;rebuild-&quot; + purpose + &quot;:&quot; + hex.EncodeToString(hash&#91;:&#93;), SignerFingerprint: fingerprint, Role: role}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4182:5232 -->
````go
// Verify 用当前可信时钟验证首次接受的制品，不接受签名者自报时间。
// 已接受制品的长期恢复应由调用方核对持久化接受事实与原始摘要，而非重新签名。
func (v *Verifier) Verify(raw []byte, purpose string) (Verified, error) {
	role := purposeRole(purpose)
	if role == "" {
		return Verified{}, ErrEnvelope
	}
	e, err := decodeEnvelope(raw)
	if err != nil || e.Purpose != purpose {
		return Verified{}, ErrEnvelope
	}
	pub, fingerprint, err := v.identity(e.Certificates, role)
	if err != nil {
		return Verified{}, err
	}
	if len(e.Signature) != pub.Size() {
		return Verified{}, ErrSignature
	}
	digest := signingDigest(purpose, e.Payload)
	if rsa.VerifyPSS(pub, crypto.SHA256, digest[:], e.Signature, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}) != nil {
		return Verified{}, ErrSignature
	}
	hash := sha256.Sum256(raw)
	return Verified{Payload: e.Payload, Reference: "rebuild-" + purpose + ":" + hex.EncodeToString(hash[:]), SignerFingerprint: fingerprint, Role: role}, nil
}

````

## 第 13 步：RestoreAccepted

插入位置：第 12 步代码之后，紧接追加；完成稿约第 183 行。

RestoreAccepted仅恢复本服务数据库已经通过Verify并持久接受的原字节。
两个期望摘要必须来自该原操作的可信持久行，不能来自CLI输入；它不是首次验签接口。
原证书过期不改变已接受事实，也不允许重新签名、换证书或换payload续接原任务。

本步声明：`func RestoreAccepted(raw []byte, purpose string, expectedEnvelope, expectedSigner [32]byte) (Verified, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>sha256.Sum256(raw)</code>；<code>decodeEnvelope(raw)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>purposeRole(purpose)</code>；<code>e.Purpose != purpose</code>；<code>&quot;rebuild-&quot; + purpose</code> |
| <code>expectedEnvelope</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>expectedEnvelope == (&#91;32&#93;byte{})</code>；<code>sha256.Sum256(raw) != expectedEnvelope</code>；<code>hex.EncodeToString(expectedEnvelope&#91;:&#93;)</code> |
| <code>expectedSigner</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>expectedSigner == (&#91;32&#93;byte{})</code>；<code>sha256.Sum256(e.Certificates&#91;0&#93;) != expectedSigner</code>；<code>hex.EncodeToString(expectedSigner&#91;:&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Verified</code> | 实际返回表达式：<code>Verified{}</code>；<code>Verified{Payload: e.Payload, Reference: &quot;rebuild-&quot; + purpose + &quot;:&quot; + hex.EncodeToString(expectedEnvelope&#91;:&#93;), 		SignerFingerprint: hex.EncodeToString(expectedSigner&#91;:&#93;), Role: role}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrEnvelope</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>role == &quot;&quot; &#124;&#124; expectedEnvelope == (&#91;32&#93;byte{}) &#124;&#124; expectedSigner == (&#91;32&#93;byte{}) &#124;&#124; 		len(raw) &gt; MaxEnvelopeBytes &#124;&#124; sha256.Sum256(raw) != expectedEnvelope</code>。条件成立时执行 <code>return Verified{}, ErrEnvelope</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; e.Purpose != purpose &#124;&#124; len(e.Certificates) == 0 &#124;&#124; sha256.Sum256(e.Certificates&#91;0&#93;) != expectedSigner</code>。条件成立时执行 <code>return Verified{}, ErrEnvelope</code>。当前函数在这里结束。
- 此处通过 <code>return Verified{Payload: e.Payload, Reference: &quot;rebuild-&quot; + purpose + &quot;:&quot; + hex.EncodeToString(expectedEnvelope&#91;:&#93;), 		SignerFingerprint: hex.EncodeToString(expectedSigner&#91;:&#93;), Role: role}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5232:6288 -->
````go
// RestoreAccepted仅恢复本服务数据库已经通过Verify并持久接受的原字节。
// 两个期望摘要必须来自该原操作的可信持久行，不能来自CLI输入；它不是首次验签接口。
// 原证书过期不改变已接受事实，也不允许重新签名、换证书或换payload续接原任务。
func RestoreAccepted(raw []byte, purpose string, expectedEnvelope, expectedSigner [32]byte) (Verified, error) {
	role := purposeRole(purpose)
	if role == "" || expectedEnvelope == ([32]byte{}) || expectedSigner == ([32]byte{}) ||
		len(raw) > MaxEnvelopeBytes || sha256.Sum256(raw) != expectedEnvelope {
		return Verified{}, ErrEnvelope
	}
	e, err := decodeEnvelope(raw)
	if err != nil || e.Purpose != purpose || len(e.Certificates) == 0 || sha256.Sum256(e.Certificates[0]) != expectedSigner {
		return Verified{}, ErrEnvelope
	}
	return Verified{Payload: e.Payload, Reference: "rebuild-" + purpose + ":" + hex.EncodeToString(expectedEnvelope[:]),
		SignerFingerprint: hex.EncodeToString(expectedSigner[:]), Role: role}, nil
}

````

## 第 14 步：purposeRole

插入位置：第 13 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`func purposeRole(purpose string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>purpose</code> | <code>string</code> | 制品用途，仅prepare-v1映射search、snapshot-v1映射document，其余返回空角色并拒绝。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;search&quot;</code>；<code>&quot;document&quot;</code>；<code>&quot;&quot;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;search&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &quot;document&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &quot;&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6288:6438 -->
````go
func purposeRole(purpose string) string {
	switch purpose {
	case PrepareV1:
		return "search"
	case SnapshotV1:
		return "document"
	}
	return ""
}

````

## 第 15 步：signingDigest

插入位置：第 14 步代码之后，紧接追加；完成稿约第 210 行。

本步声明：`func signingDigest(purpose string, payload []byte) [32]byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(purpose)</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 原规范元数据字节，与固定域标签及purpose分别加8字节长度前缀后哈希。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 实际返回表达式：<code>digest</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return digest</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6438:6912 -->
````go
func signingDigest(purpose string, payload []byte) [32]byte {
	h := sha256.New()
	// 每一段带长度，避免拼接歧义；固定域隔离 TLS 和其他协议的签名字节。
	for _, part := range [][]byte{[]byte("collaboration-report/rebuild-artifact/v1"), []byte(purpose), payload} {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(part)))
		h.Write(size[:])
		h.Write(part)
	}
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return digest
}

````

## 第 16 步：Verifier.identity

插入位置：第 15 步代码之后，紧接追加；完成稿约第 224 行。

本步声明：`func (v *Verifier) identity(ders [][]byte, role string) (*rsa.PublicKey, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ders</code> | <code>&#91;&#93;&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(ders)</code> |
| <code>role</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;spiffe://collaboration-report/&quot;+role</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*rsa.PublicKey</code> | 实际返回表达式：<code>nil</code>；<code>pub</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>hex.EncodeToString(fingerprint&#91;:&#93;)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrIdentity</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>v == nil &#124;&#124; v.roots == nil &#124;&#124; len(ders) == 0 &#124;&#124; len(ders) &gt; maxChainLength</code>。条件成立时执行 <code>return nil, &quot;&quot;, ErrIdentity</code>。当前函数在这里结束。
- 检查 <code>len(der) == 0 &#124;&#124; len(der) &gt; maxCertificateBytes</code>。条件成立时执行 <code>return nil, &quot;&quot;, ErrIdentity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, &quot;&quot;, ErrIdentity</code>。当前函数在这里结束。
- 检查 <code>i == 0</code>。
- 检查 <code>!ok &#124;&#124; pub.N.BitLen() &lt; 2048 &#124;&#124; pub.N.BitLen() &gt; 8192 &#124;&#124; leaf.IsCA &#124;&#124; leaf.KeyUsage&amp;x509.KeyUsageDigitalSignature == 0</code>。条件成立时执行 <code>return nil, &quot;&quot;, ErrIdentity</code>。当前函数在这里结束。
- 检查 <code>len(leaf.URIs) != 1 &#124;&#124; leaf.URIs&#91;0&#93;.String() != &quot;spiffe://collaboration-report/&quot;+role</code>。条件成立时执行 <code>return nil, &quot;&quot;, ErrIdentity</code>。当前函数在这里结束。
- 检查 <code>usage == x509.ExtKeyUsageClientAuth</code>。
- 检查 <code>!clientAuth</code>。条件成立时执行 <code>return nil, &quot;&quot;, ErrIdentity</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, &quot;&quot;, ErrIdentity</code>。当前函数在这里结束。
- 此处通过 <code>return pub, hex.EncodeToString(fingerprint&#91;:&#93;), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=6912:8387 -->
````go
func (v *Verifier) identity(ders [][]byte, role string) (*rsa.PublicKey, string, error) {
	if v == nil || v.roots == nil || len(ders) == 0 || len(ders) > maxChainLength {
		return nil, "", ErrIdentity
	}
	var leaf *x509.Certificate
	intermediates := x509.NewCertPool()
	for i, der := range ders {
		if len(der) == 0 || len(der) > maxCertificateBytes {
			return nil, "", ErrIdentity
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, "", ErrIdentity
		}
		if i == 0 {
			leaf = cert
		} else {
			intermediates.AddCert(cert)
		}
	}
	pub, ok := leaf.PublicKey.(*rsa.PublicKey)
	if !ok || pub.N.BitLen() < 2048 || pub.N.BitLen() > 8192 || leaf.IsCA || leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return nil, "", ErrIdentity
	}
	if len(leaf.URIs) != 1 || leaf.URIs[0].String() != "spiffe://collaboration-report/"+role {
		return nil, "", ErrIdentity
	}
	// Verify 的 Any EKU 兼容规则不能替代本协议显式 clientAuth 准入。
	clientAuth := false
	for _, usage := range leaf.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			clientAuth = true
		}
	}
	if !clientAuth {
		return nil, "", ErrIdentity
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: v.roots, Intermediates: intermediates, CurrentTime: time.Now(), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		return nil, "", ErrIdentity
	}
	fingerprint := sha256.Sum256(leaf.Raw)
	return pub, hex.EncodeToString(fingerprint[:]), nil
}

````

## 第 17 步：readBounded

插入位置：第 16 步代码之后，紧接追加；完成稿约第 268 行。

本步声明：`func readBounded(path string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>os.Open(path)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>data</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrConfig</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 登记 <code>f.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; len(data) &gt; MaxEnvelopeBytes</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 此处通过 <code>return data, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=8387:8682 -->
````go
func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrConfig
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxEnvelopeBytes+1))
	if err != nil || len(data) > MaxEnvelopeBytes {
		return nil, ErrConfig
	}
	return data, nil
}

````

## 第 18 步：parseCertificates

插入位置：第 17 步代码之后，紧接追加；完成稿约第 281 行。

本步声明：`func parseCertificates(data []byte) ([][]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>data</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.TrimSpace(data)</code>；<code>pem.Decode(data)</code>；<code>data = rest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>certs</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrConfig</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>block == nil &#124;&#124; block.Type != &quot;CERTIFICATE&quot; &#124;&#124; len(block.Bytes) &gt; maxCertificateBytes &#124;&#124; len(certs) &gt;= maxChainLength</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 检查 <code>len(certs) == 0</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 此处通过 <code>return certs, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=8682:9109 -->
````go
func parseCertificates(data []byte) ([][]byte, error) {
	var certs [][]byte
	for len(bytes.TrimSpace(data)) > 0 {
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" || len(block.Bytes) > maxCertificateBytes || len(certs) >= maxChainLength {
			return nil, ErrConfig
		}
		certs = append(certs, block.Bytes)
		data = rest
	}
	if len(certs) == 0 {
		return nil, ErrConfig
	}
	return certs, nil
}

````

## 第 19 步：decodeEnvelope

插入位置：第 18 步代码之后，紧接追加；完成稿约第 297 行。

本步声明：`func decodeEnvelope(raw []byte) (envelope, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>bytes.NewReader(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>envelope</code> | 实际返回表达式：<code>e</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrEnvelope</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) == 0 &#124;&#124; len(raw) &gt; MaxEnvelopeBytes</code>。条件成立时执行 <code>return e, ErrEnvelope</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; first != json.Delim(&#x27;{&#x27;)</code>。条件成立时执行 <code>return e, ErrEnvelope</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !ok &#124;&#124; seen&#91;name&#93;</code>。条件成立时执行 <code>return e, ErrEnvelope</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; (len(values) == 0 &#124;&#124; len(values) &gt; maxChainLength)</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>err == nil &amp;&amp; value == nil</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>name == &quot;payload&quot;</code>。

<!-- write-step: 19 bytes=9109:10244 -->
````go
func decodeEnvelope(raw []byte) (envelope, error) {
	var e envelope
	if len(raw) == 0 || len(raw) > MaxEnvelopeBytes {
		return e, ErrEnvelope
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return e, ErrEnvelope
	}
	seen := make(map[string]bool, 4)
	for d.More() {
		token, err := d.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			return e, ErrEnvelope
		}
		seen[name] = true
		switch name {
		case "purpose":
			err = d.Decode(&e.Purpose)
		case "certificates":
			var values []string
			err = d.Decode(&values)
			if err == nil && (len(values) == 0 || len(values) > maxChainLength) {
				err = ErrEnvelope
			}
			if err == nil {
				for _, value := range values {
					var der []byte
					der, err = decodeBase64(value, maxCertificateBytes)
					if err != nil {
						break
					}
					e.Certificates = append(e.Certificates, der)
				}
			}
		case "payload", "signature":
			var value *string
			err = d.Decode(&value)
			if err == nil && value == nil {
				err = ErrEnvelope
			}
			if err == nil {
				if name == "payload" {
````

## 第 20 步：decodeEnvelope（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 342 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return e, ErrEnvelope</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return e, ErrEnvelope</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; end != json.Delim(&#x27;}&#x27;) &#124;&#124; len(seen) != 4</code>。条件成立时执行 <code>return e, ErrEnvelope</code>。当前函数在这里结束。
- 检查 <code>err != io.EOF</code>。条件成立时执行 <code>return e, ErrEnvelope</code>。当前函数在这里结束。
- 检查 <code>e.Payload == nil &#124;&#124; len(e.Payload) &gt; MaxPayloadBytes &#124;&#124; len(e.Signature) &lt; 256 &#124;&#124; len(e.Signature) &gt; 1024 &#124;&#124; len(e.Certificates) == 0 &#124;&#124; len(e.Certificates) &gt; maxChainLength</code>。条件成立时执行 <code>return e, ErrEnvelope</code>。当前函数在这里结束。
- 此处通过 <code>return e, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=10244:10873 -->
````go
					e.Payload, err = decodeBase64(*value, MaxPayloadBytes)
				} else {
					e.Signature, err = decodeBase64(*value, 1024)
				}
			}
		default:
			return e, ErrEnvelope
		}
		if err != nil {
			return e, ErrEnvelope
		}
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') || len(seen) != 4 {
		return e, ErrEnvelope
	}
	if _, err := d.Token(); err != io.EOF {
		return e, ErrEnvelope
	}
	if e.Payload == nil || len(e.Payload) > MaxPayloadBytes || len(e.Signature) < 256 || len(e.Signature) > 1024 || len(e.Certificates) == 0 || len(e.Certificates) > maxChainLength {
		return e, ErrEnvelope
	}
	return e, nil
}

````

## 第 21 步：decodeBase64

插入位置：第 20 步代码之后，紧接追加；完成稿约第 367 行。

本步声明：`func decodeBase64(value string, maximum int) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>len(value)</code>；<code>base64.StdEncoding.Strict().DecodeString(value)</code>；<code>base64.StdEncoding.EncodeToString(decoded) != value</code> |
| <code>maximum</code> | <code>int</code> | 在此声明中的实际使用：<code>base64.StdEncoding.EncodedLen(maximum)</code>；<code>len(decoded) &gt; maximum</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>decoded</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrEnvelope</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(value) &gt; base64.StdEncoding.EncodedLen(maximum)</code>。条件成立时执行 <code>return nil, ErrEnvelope</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(decoded) &gt; maximum &#124;&#124; base64.StdEncoding.EncodeToString(decoded) != value</code>。条件成立时执行 <code>return nil, ErrEnvelope</code>。当前函数在这里结束。
- 此处通过 <code>return decoded, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=10873:11333 -->
````go
func decodeBase64(value string, maximum int) ([]byte, error) {
	if len(value) > base64.StdEncoding.EncodedLen(maximum) {
		return nil, ErrEnvelope
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	// 标准解码器容忍换行；制品要求标准唯一表示，故再核对编码结果。
	if err != nil || len(decoded) > maximum || base64.StdEncoding.EncodeToString(decoded) != value {
		return nil, ErrEnvelope
	}
	return decoded, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/rebuildartifact/artifact.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/05-rebuild-artifacts.md)。
