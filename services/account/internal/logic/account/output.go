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
