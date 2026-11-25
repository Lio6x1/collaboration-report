# services/account/internal/domain/account/operation_receipt.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/041-operation-receipt.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`OperationReceipt`、`LogoutSelection`、`OperationProgress`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package account

type OperationReceipt struct {
	ActorID     string
	Kind        string
	State       string
	SessionHash string
}

type LogoutSelection struct {
	Session      Session
	Token        string
	Registered   bool
	ReceiptState string
}

// OperationProgress只携带持久结果或原关闭操作的状态，URL由HTTP适配层生成。
type OperationProgress struct {
	State  string
	Result []byte
}
````

UTF-8/LF全文SHA-256：`cb0347d15a0054699f0dd6250f9c73bd32eb1fef058f7d64ba429a0bbdbdaf68`。[参考实现](../../../../services/account/internal/domain/account/operation_receipt.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
