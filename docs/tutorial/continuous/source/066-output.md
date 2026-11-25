# services/account/internal/logic/account/output.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/066-output.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`OutputRegistrar`、`Outputs`、`NewOutputs`、`Register`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package account

import (
	"context"
	"errors"
	"sort"

	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	"collaboration/platform"
)

type OutputRegistrar interface {
	Register(context.Context, string, []domain.OutputSource) error
}

type Outputs struct {
	repository *repository.Outputs
	registrar  OutputRegistrar
}

func NewOutputs(repo *repository.Outputs, registrar OutputRegistrar) *Outputs {
	return &Outputs{repository: repo, registrar: registrar}
}

func (s *Outputs) Register(ctx context.Context, handler string, binding *domain.OutputBinding) error {
	sources := make([]domain.OutputSource, 0, len(binding.Sources))
	for key, epoch := range binding.Sources {
		if !platform.ValidAccountOutputScope(key) {
			return errors.New("SOURCE_INVALID")
		}
		// 重检构造响应时捕获的代次；不能把新成员/角色代次贴到旧查询结果上。
		current, err := s.repository.CurrentEpoch(ctx, key)
		if err != nil || current != epoch {
			return errors.New("SOURCE_CHANGED")
		}
		sources = append(sources, domain.OutputSource{Key: key, Epoch: epoch})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Key < sources[j].Key })
	return s.registrar.Register(ctx, handler, sources)
}
````

UTF-8/LF全文SHA-256：`f49de9db6c1b06dbf870005808a0436cc76f60c51e897d91cae5d6bdc2340937`。[参考实现](../../../../services/account/internal/logic/account/output.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
