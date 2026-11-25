package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"context"
	"net/http"
	"net/url"
)

type searchPageUsecase interface {
	Page(context.Context, *logic.Actor, domain.SearchQuery, string, logic.BindOutput) (dto.SearchResultPage, error)
}

type SearchPages struct {
	identity *Identity
	outputs  *Outputs
	usecase  searchPageUsecase
}

func NewSearchPages(identity *Identity, outputs *Outputs, usecase searchPageUsecase) *SearchPages {
	return &SearchPages{identity: identity, outputs: outputs, usecase: usecase}
}

func (h *SearchPages) Page(w http.ResponseWriter, r *http.Request) {
	// URL.Query会隐藏畸形百分号/分号解析错误；必须先核完整原参数集合。
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		httpx.Failure(w, domain.Violation(domain.InvalidRequest, "SEARCH_PARAMETERS_INVALID"))
		return
	}
	query, err := domain.ParseSearchQuery(values)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.OptionalActor(r, "READ_PUBLIC_DOCUMENT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	page, err := h.usecase.Page(r.Context(), actor, query, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		if h.outputs == nil {
			return nil
		}
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if output == nil {
		// 仅确实耗尽的匿名空页可普通输出；其他页没有真实双进程租约就是失败。
		if actor != nil || len(page.Items) != 0 || page.HasMore || page.NextCursor != nil {
			httpx.Failure(w, domain.Violation(domain.TemporarilyUnavailable, "SEARCH_OUTPUT_REQUIRED"))
			return
		}
		platform.WriteJSON(w, http.StatusOK, page)
		return
	}
	if err = output.SendBounded(r.Context(), http.StatusOK, page, 2<<20); err != nil {
		httpx.Failure(w, err)
	}
}
