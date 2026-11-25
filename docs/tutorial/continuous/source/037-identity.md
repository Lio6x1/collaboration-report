# services/account/internal/domain/account/identity.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/037-identity.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`ValidOperationID`、`ValidIdentityPurpose`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package account

import (
	"errors"
	"regexp"
)

var (
	operationID            = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$`)
	identityPurpose        = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,39}$`)
	ErrIdentityBinding     = errors.New("IDENTITY_BINDING_INVALID")
	ErrIdentityDenied      = errors.New("IDENTITY_NOT_CURRENT")
	ErrIdentityUnavailable = errors.New("IDENTITY_UNAVAILABLE")
)

func ValidOperationID(value string) bool     { return operationID.MatchString(value) }
func ValidIdentityPurpose(value string) bool { return identityPurpose.MatchString(value) }
````

UTF-8/LF全文SHA-256：`3dd1ae7e8222c9a67e5f1b3993acece19c1884d01c5d692563c61428b1a50555`。[参考实现](../../../../services/account/internal/domain/account/identity.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
