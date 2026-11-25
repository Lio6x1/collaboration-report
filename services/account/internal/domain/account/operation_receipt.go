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
