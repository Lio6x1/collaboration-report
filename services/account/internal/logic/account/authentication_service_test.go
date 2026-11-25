package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	domain "collaboration/account/internal/domain/account"
	dto "collaboration/contracts/httpdto"
)

func TestWeakNewPasswordsRejectedBeforeChallengesOrStorage(t *testing.T) {
	// 三个业务入口都必须执行规则；空依赖能证明拒绝发生在验证码、哈希和持久写入之前。
	for name, action := range map[string]func() error{
		"注册": func() error {
			_, err := (&Signups{}).Register(context.Background(), dto.Signup{Password: "abcdefgh"}, nil)
			return err
		},
		"找回密码": func() error {
			_, err := (&Credentials{}).ResetPassword(context.Background(), dto.ResetPassword{NewPassword: "abcdefgh"}, nil)
			return err
		},
		"修改密码": func() error {
			_, err := (&Credentials{}).ChangePassword(context.Background(), domain.Session{}, dto.PasswordChange{NewPassword: "abcdefgh"}, nil)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			var violation domain.RuleViolation
			if err := action(); !errors.As(err, &violation) || violation.Code != "PASSWORD_POLICY_INVALID" {
				t.Fatalf("弱密码应被明确拒绝，实际=%v", err)
			}
		})
	}
}

type authenticationReadStore struct {
	AuthenticationStore
	session                       domain.Session
	credential                    domain.Credential
	sessionError, credentialError error
}

func (s authenticationReadStore) Session(context.Context, []byte) (domain.Session, error) {
	return s.session, s.sessionError
}
func (s authenticationReadStore) Credential(context.Context, string) (domain.Credential, error) {
	return s.credential, s.credentialError
}

func TestAuthenticationDistinguishesInvalidIdentityFromReadFailure(t *testing.T) {
	// 数据库超时不能被转换为401并导致前端清除有效登录；真实失效仍须拒绝。
	token, _, err := domain.RandomToken()
	if err != nil {
		t.Fatal(err)
	}
	readFailure := errors.New("database temporarily unavailable")
	for _, sample := range []struct {
		name                          string
		sessionError, credentialError error
		state, epoch                  string
		expired                       bool
		want                          error
	}{
		{name: "valid", state: "ACTIVE", epoch: "2"},
		{name: "missing session", sessionError: sql.ErrNoRows, want: domain.ErrSessionInvalid},
		{name: "wrapped missing session", sessionError: fmt.Errorf("lookup: %w", sql.ErrNoRows), want: domain.ErrSessionInvalid},
		{name: "session database unavailable", sessionError: readFailure, want: readFailure},
		{name: "credential timeout", state: "ACTIVE", credentialError: context.DeadlineExceeded, want: context.DeadlineExceeded},
		{name: "credential cancelled", state: "ACTIVE", credentialError: context.Canceled, want: context.Canceled},
		{name: "deleted account", state: "ACTIVE", credentialError: fmt.Errorf("lookup: %w", sql.ErrNoRows), want: domain.ErrSessionInvalid},
		{name: "closed session", state: "CLOSED", want: domain.ErrSessionInvalid},
		{name: "expired session", state: "ACTIVE", expired: true, want: domain.ErrSessionInvalid},
		{name: "changed credential epoch", state: "ACTIVE", epoch: "3", want: domain.ErrSessionInvalid},
	} {
		t.Run(sample.name, func(t *testing.T) {
			expires := time.Now().Add(time.Hour)
			if sample.expired {
				expires = time.Now().Add(-time.Hour)
			}
			store := authenticationReadStore{
				session:      domain.Session{UserID: "7", State: sample.state, Expires: expires, CredentialEpoch: "2"},
				credential:   domain.Credential{State: "ACTIVE", Epoch: sample.epoch, IdentityEpoch: 9},
				sessionError: sample.sessionError, credentialError: sample.credentialError,
			}
			value, err := NewAuthentication(nil, store, nil, nil).Authenticate(context.Background(), token)
			if !errors.Is(err, sample.want) {
				t.Fatalf("wrong identity classification: %v", err)
			}
			if sample.want == nil && value.IdentityEpoch != 9 {
				t.Fatal("valid identity epoch lost")
			}
			if sample.want != nil && sample.want != domain.ErrSessionInvalid && errors.Is(err, domain.ErrSessionInvalid) {
				t.Fatal("read failure must not revoke the identity")
			}
		})
	}
	// Cookie词法错误在读取数据库之前就拒绝，不能被当成依赖故障。
	if _, err := (&Authentication{}).Authenticate(context.Background(), "invalid-cookie"); !errors.Is(err, domain.ErrSessionInvalid) {
		t.Fatalf("malformed token: %v", err)
	}
}

type challengeReadStore struct {
	ChallengeStore
	metaError error
}

func (s challengeReadStore) Meta(context.Context, string) (any, error) {
	return "", s.metaError
}

func TestChallengeFailuresKeepTheirCategoryAcrossBusinessEntries(t *testing.T) {
	// 三个消费者直接传递验证码错误，避免复制字符串判断后把Redis故障误报为用户输错。
	const id = "1234567890abcdef1234567890abcdef"
	for _, sample := range []struct {
		name            string
		metaError, want error
	}{
		{"invalid", nil, domain.ErrChallengeInvalid},
		{"store unavailable", errors.New("private redis connection error"), domain.ErrChallengeStoreUnavailable},
	} {
		t.Run(sample.name, func(t *testing.T) {
			challenges := NewChallenges(challengeReadStore{metaError: sample.metaError}, nil, nil)
			credentials := NewCredentials(nil, nil, nil, challenges)
			for name, action := range map[string]func() error{
				"signup": func() error {
					_, err := NewSignups(nil, nil, challenges).Register(context.Background(), dto.Signup{Password: "Abcdef12", ChallengeId: id, Code: "000000"}, nil)
					return err
				},
				"reset": func() error {
					_, err := credentials.ResetPassword(context.Background(), dto.ResetPassword{NewPassword: "Abcdef12", ChallengeId: id, Code: "000000"}, nil)
					return err
				},
				"email": func() error {
					_, err := credentials.ChangeEmail(context.Background(), domain.Session{}, dto.EmailChange{ChallengeId: id, Code: "000000"}, nil)
					return err
				},
			} {
				t.Run(name, func(t *testing.T) {
					if err := action(); !errors.Is(err, sample.want) {
						t.Fatalf("lost challenge category: %v", err)
					}
				})
			}
		})
	}
}
