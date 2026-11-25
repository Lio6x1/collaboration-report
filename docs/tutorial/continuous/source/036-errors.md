# services/account/internal/domain/account/errors.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/036-errors.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：完整定义见下方源码。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`55a709811d88b1f02268203324c98ea243957a47d9823212a1a924a87a2fcb09`。[参考实现](../../../../services/account/internal/domain/account/errors.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
