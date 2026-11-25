# services/account/main.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/088-main.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`main`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package main

import (
	"collaboration/account/internal/bootstrap"
	"collaboration/account/internal/server"
	"collaboration/account/internal/svc"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"

	"github.com/zeromicro/go-zero/rest"
	"google.golang.org/grpc"
)

func main() {
	var identity *server.Identity
	platform.Main("account", func(rpc *grpc.Server, store *platform.Store, _ platform.Config) {
		identity = server.NewIdentity(svc.NewIdentity(store))
		identity.BindAuthorizationResults(svc.NewAuthorizationResults(store))
		pb.RegisterIdentityInternalServer(rpc, identity)
	}, func(http *rest.Server, store *platform.Store, cfg platform.Config) error {
		return bootstrap.RegisterAccount(http, store, cfg, identity.BindOperationResults)
	})
}
````

UTF-8/LF全文SHA-256：`9b14b8b565d81bb5c0169af3d975c22ca0e9fd9d5d0285ce90f6be2368ff7efb`。[参考实现](../../../../services/account/main.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
