// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// challenge.go：保留原实现，按业务合并文件。

type ChallengeMeta struct {
	Purpose string `json:"purpose"`
	Subject string `json:"subject"`
	UserID  string `json:"user_id"`
	Epoch   string `json:"credential_epoch"`
	ActorID string `json:"actor_id"`
}

type IssuedChallenge struct {
	ID      string
	Expires time.Time
}

type PublicReceipt struct {
	Kind        string
	Fingerprint []byte
	Result      []byte
}

// credential.go：保留原实现，按业务合并文件。

type CredentialChange struct {
	ID            string
	Kind          string
	UserID        string
	ExpectedEpoch string
	Session       *Session
	Proof         *ChallengeMeta
	ChallengeID   string
	Fingerprint   []byte
	NewHash       []byte
	OldHash       []byte
	NewEmail      string
}

type CredentialReceipt struct {
	OperationReceipt
	Fingerprint []byte
}

type CredentialRetry struct {
	Session Session
	State   string
}

type CredentialPreparation struct {
	Change  CredentialChange
	Epoch   uint64
	Scope   string
	Target  []byte
	Pending []byte
	Payload []byte
}

func CredentialEpoch(value string) (uint64, bool) {
	epoch, err := strconv.ParseUint(value, 10, 64)
	return epoch, err == nil && epoch != 0 && epoch != ^uint64(0) && strconv.FormatUint(epoch, 10) == value
}

// secrets.go：保留原实现，按业务合并文件。

type Secrets struct{ key []byte }

func NewSecrets(key []byte) *Secrets { return &Secrets{key: append([]byte(nil), key...)} }

func SubjectDigest(email string) string {
	value := sha256.Sum256([]byte(email))
	return hex.EncodeToString(value[:])
}

func (s *Secrets) Digest(value string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

// Fingerprint保持原JSON/HMAC编码，密码和验证码不能以明文写入receipt。
func (s *Secrets) Fingerprint(input any) []byte {
	raw, _ := json.Marshal(input)
	digest, _ := hex.DecodeString(s.Digest(string(raw)))
	return digest
}

// token.go：保留原实现，按业务合并文件。

func RandomToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	return token, hash[:], nil
}

func TokenHash(value string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != value {
		return nil, errors.New("SESSION_INVALID")
	}
	hash := sha256.Sum256(raw)
	return hash[:], nil
}

// validation.go：保留原实现，按业务合并文件。

// bcrypt按UTF-8字节限制密码，不能以字符数检查或静默截断中文密码。
func ValidPassword(value string) bool {
	return len([]byte(value)) >= 1 && len([]byte(value)) <= 72
}

// 新密码单独执行强度规则，登录/当前密码仍按原哈希验证，避免锁住已有账号。
// 大小写均属于英文字母类，但原文直接交给bcrypt，不能转小写或去除首尾字符。
func ValidNewPassword(value string) bool {
	if !ValidPassword(value) || !utf8.ValidString(value) || utf8.RuneCountInString(value) < 8 {
		return false
	}
	var letters, digits, symbols bool
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z':
			letters = true
		case character >= '0' && character <= '9':
			digits = true
		case unicode.IsPunct(character) || unicode.IsSymbol(character):
			symbols = true
		}
	}
	// 空格和中文文字可以保留在密码中，但不能冒充英文或特殊字符凑够类别。
	return letters && digits || letters && symbols || digits && symbols
}

func ValidEmail(value string) bool {
	if len(value) > 254 || value != strings.TrimSpace(value) {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}
