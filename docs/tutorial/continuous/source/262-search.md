# services/document/internal/handler/document/search.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/262-search.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`searchPageUsecase`、`SearchPages`、`NewSearchPages`、`Page`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`444cd47e8c79581553f1813a98626a4466beb63024ae58f8f17cd9795624ff07`。[参考实现](../../../../services/document/internal/handler/document/search.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
