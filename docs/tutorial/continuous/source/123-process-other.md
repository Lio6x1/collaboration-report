# services/document/internal/exporter/process_other.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/123-process-other.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：转换固定版本，管理进程和产物；不能重新读取变化中的正文。

主要声明：`runProgram`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
//go:build !windows && !linux

package exporter

import "os/exec"

// 未验收的平台只保留直接进程取消；正式部署支持Windows Job Object与Linux容器。
func runProgram(cmd *exec.Cmd) error { return cmd.Run() }
````

UTF-8/LF全文SHA-256：`8a0b5d7ff1dba6664b6045d5594fb701f6a303136ff05c033c996ae5c92642fd`。[参考实现](../../../../services/document/internal/exporter/process_other.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
