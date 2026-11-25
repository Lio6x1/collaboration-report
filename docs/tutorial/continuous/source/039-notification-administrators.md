# services/account/internal/domain/account/notification_administrators.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/039-notification-administrators.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：完整定义见下方源码。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package account

import "errors"

const NotificationAdministratorPageSize = 100

var ErrNotificationAdministratorsInput = errors.New("NOTIFICATION_ADMINISTRATORS_INVALID")
````

UTF-8/LF全文SHA-256：`1bebb103b2c9716b8b8593f909c4c217c34754125b9dcde2d3226ea2f812beb1`。[参考实现](../../../../services/account/internal/domain/account/notification_administrators.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
