# services/document/internal/client/process_probe_other.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/114-process-probe-other.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`probeProcess`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
//go:build !windows && !linux

package client

import (
	"context"
	"errors"
)

// 未提供内核事实适配的平台不能猜测旧实例已经退出。
func (c *ProcessOwner) probeProcess(context.Context, uint64) (processFacts, error) {
	return processFacts{}, errors.New("PROCESS_PLATFORM_UNSUPPORTED")
}
````

UTF-8/LF全文SHA-256：`bc577d8641d713c6eb6d4939caa827d1ab0d557284ef37d14e3b7016cb11e6c4`。[参考实现](../../../../services/document/internal/client/process_probe_other.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
