# services/document/internal/storage/oss/credentials_unix.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/133-credentials-unix.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：按明确授权操作资源；不将内部存储路径当作公开下载链接。

主要声明：`privateMode`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
//go:build !windows

package oss

import "os"

func privateMode(mode os.FileMode) bool { return mode.Perm()&0077 == 0 }
````

UTF-8/LF全文SHA-256：`1dfa993137db33c37f0d73c7bf173fa4c3417173394731c13f295863dd05123f`。[参考实现](../../../../services/document/internal/storage/oss/credentials_unix.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
