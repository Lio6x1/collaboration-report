# services/search/main.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/410-main.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`main`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package main

import (
	"collaboration/platform"
	"collaboration/search/internal/bootstrap"
)

func main() {
	platform.Main("search", bootstrap.RegisterSearch, nil)
}
````

UTF-8/LF全文SHA-256：`6fd0f2c97685568728fa1e05981962040e5946c0fcdf47972eafba6feba25e2a`。[参考实现](../../../../services/search/main.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
