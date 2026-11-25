package account

import "errors"

// 领域错误不包含HTTP状态；同一用例可以由HTTP、RPC和后台流程调用。
var (
	ErrLoginInput     = errors.New("LOGIN_INPUT_INVALID")
	ErrLoginLimited   = errors.New("LOGIN_RATE_LIMITED")
	ErrLoginInvalid   = errors.New("LOGIN_INVALID")
	ErrSessionInvalid = errors.New("SESSION_INVALID")
	// 已知基础设施故障用哨兵错误跨层传递；不能靠驱动正文判断，也不能丢失可解释的失败类别。
	ErrChallengeStoreUnavailable = errors.New("CHALLENGE_STORE_UNAVAILABLE")
	ErrSMTPDeliveryFailed        = errors.New("SMTP_DELIVERY_FAILED")
	ErrChallengeInvalid          = errors.New("CHALLENGE_INVALID")
	ErrChallengeRateLimited      = errors.New("CHALLENGE_RATE_LIMITED")
)
