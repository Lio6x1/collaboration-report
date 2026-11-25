# services/document/internal/storage/oss/credentials_windows.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/134-credentials-windows.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：按明确授权操作资源；不将内部存储路径当作公开下载链接。

主要声明：`privateMode`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
//go:build windows

package oss

import "os"

// Windows 文件权限由 ACL 表达，不能把 os.FileMode 的 0666 当成实际公开访问。
func privateMode(os.FileMode) bool { return true }
````

UTF-8/LF全文SHA-256：`86733f44f43a33fd566cbb590c2bf32e8c1aef43a482c0046ab47307229a4a65`。[参考实现](../../../../services/document/internal/storage/oss/credentials_windows.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
