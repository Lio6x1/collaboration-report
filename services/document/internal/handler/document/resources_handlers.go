// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"context"
	"encoding/base64"
	"errors"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"net/http"
	"strconv"
	"time"
)

// draft_body_capacity.go：保留原实现，按业务合并文件。

type DraftBodyCapacity struct{ usecase *logic.DraftResources }

func NewDraftBodyCapacity(usecase *logic.DraftResources) *DraftBodyCapacity {
	return &DraftBodyCapacity{usecase: usecase}
}

func (h *DraftBodyCapacity) Protect(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "collaboration") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input platform.DraftBodyProtection
	if !httpx.DecodeBodyLimit(w, r, &input, 256<<10) {
		return
	}
	req := new(pb.DraftResourceRequest)
	if protojson.Unmarshal(input.Protection, req) != nil {
		httpx.Fail(w, 400, "PROTECTION_INVALID")
		return
	}
	body, err := domain.ParseDraftBodyMeasure(input.BodyBytes, input.BodyHash)
	if err == nil {
		var receipt *pb.DraftResourceReceipt
		receipt, err = h.usecase.ProtectBody(r.Context(), req, body)
		if err == nil {
			platform.WriteJSON(w, 200, map[string]string{"state": receipt.State.String(), "resource_digest": base64.StdEncoding.EncodeToString(receipt.ResourceDigest)})
			return
		}
	}
	var protection domain.ProtectionFailure
	if errors.As(err, &protection) {
		status := 503
		switch protection.Kind {
		case domain.ProtectionInvalid:
			status = 400
		case domain.ProtectionForbidden:
			status = 403
		case domain.ProtectionMissing:
			status = 404
		case domain.ProtectionConflict, domain.ProtectionPrecondition:
			status = 409
		}
		httpx.Fail(w, status, protection.Code)
		return
	}
	httpx.Failure(w, err)
}

// resource_content.go：保留原实现，按业务合并文件。

func (h *Resources) Content(w http.ResponseWriter, r *http.Request) {
	resource, err := domain.CanonicalID(pathvar.Vars(r)["resourceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	query := r.URL.Query()
	for _, key := range []string{"document_id", "context_kind", "context_id"} {
		if len(query[key]) != 1 {
			httpx.Fail(w, 400, "RESOURCE_CONTEXT_INVALID")
			return
		}
	}
	document, err := domain.CanonicalID(query.Get("document_id"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	context, err := domain.CanonicalID(query.Get("context_id"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.OptionalActor(r, "READ_RESOURCE", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	file, err := h.reading.Open(r.Context(), actor, logic.ResourceTarget{Resource: resource, Document: document, Context: context, Kind: query.Get("context_kind")}, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.requests.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	defer file.Close()
	started, err := output.SendResource(r.Context(), file)
	// 已发送文件头后只截断连接，不能把JSON错误拼入二进制正文冒充完整文件。
	if err != nil && !started {
		httpx.Failure(w, err)
	}
}

// resource_output.go：保留原实现，按业务合并文件。

func (o *ResponseOutput) SendResource(ctx context.Context, file *logic.ResourceDownload) (started bool, err error) {
	return o.sendFile(ctx, file, "", false)
}

func (o *ResponseOutput) SendExport(ctx context.Context, file *logic.ResourceDownload, name string) (bool, error) {
	return o.sendFile(ctx, file, name, true)
}

func (o *ResponseOutput) sendFile(ctx context.Context, file *logic.ResourceDownload, name string, export bool) (started bool, err error) {
	defer o.finish()
	if err = o.open(ctx); err != nil {
		return false, err
	}
	if file.Size > 32<<20 {
		return false, domain.Violation(domain.TemporarilyUnavailable, "RESOURCE_TRANSFER_LIMIT")
	}
	controller := http.NewResponseController(o.w)
	_ = controller.SetWriteDeadline(time.Now().Add(4 * time.Second))
	if err = o.checkStopped(ctx); err != nil {
		return false, err
	}
	media, disposition := file.MediaType, "inline"
	if export {
		disposition = "attachment"
	} else {
		switch media {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
		default:
			media, disposition = "application/octet-stream", "attachment"
		}
		name = "resource-" + domain.TextID(file.ID)
	}
	o.w.Header().Set("Content-Type", media)
	o.w.Header().Set("Content-Disposition", disposition+`; filename="`+name+`"`)
	o.w.Header().Set("Content-Length", strconv.FormatUint(file.Size, 10))
	o.w.Header().Set("Cache-Control", "no-store")
	o.w.Header().Set("X-Content-Type-Options", "nosniff")
	o.w.WriteHeader(http.StatusOK)
	started = true
	defer func() {
		if err != nil {
			_ = controller.SetWriteDeadline(time.Now())
		}
	}()
	buffer := make([]byte, 32<<10)
	left := file.Size
	for left > 0 {
		if err = o.checkStopped(ctx); err != nil {
			return true, err
		}
		size := uint64(len(buffer))
		if left < size {
			size = left
		}
		var n int
		n, err = io.ReadFull(file.Reader, buffer[:int(size)])
		if err != nil {
			return true, err
		}
		if err = o.checkStopped(ctx); err != nil {
			return true, err
		}
		var written int
		written, err = o.w.Write(buffer[:n])
		if err != nil {
			return true, err
		}
		if written != n {
			return true, io.ErrShortWrite
		}
		left -= uint64(n)
	}
	// Flush实际结束后才结算Doc租约；Edge仍持有自己的输出直到浏览器写出结束。
	return true, controller.Flush()
}

// resources.go：保留原实现，按业务合并文件。

type Resources struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Resources
	reading  *logic.ResourceReading
}

func NewResources(identity *Identity, requests *Transactions, usecase *logic.Resources, reading *logic.ResourceReading) *Resources {
	return &Resources{identity: identity, requests: requests, usecase: usecase, reading: reading}
}

func (h *Resources) BeginUpload(w http.ResponseWriter, r *http.Request) {
	var input dto.ResourceUpload
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "RESOURCE_UPLOAD", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.PrepareUpload(r.Context(), actor, document, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 配额预留和上传登记必须沿同一幂等事务提交；句柄投影再核当前授权，
	// 并由统一输出租约保护，避免重放回执时绕过撤权。
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "RESOURCE_UPLOAD", input, http.StatusCreated, run, present)
}

func (h *Resources) Complete(w http.ResponseWriter, r *http.Request) {
	var input dto.UploadComplete
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["resourceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// URL只有资源ID，入口先核验会话和CSRF；用例从持久化上传记录确定
	// 文档并检查上传者与编辑权限，不能信任客户端另报的文档范围。
	actor, err := h.identity.Actor(r, true, "RESOURCE_COMPLETE", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	release, err := h.usecase.HoldUpload(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	defer release()
	actor, run, present, err := h.usecase.PrepareComplete(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 文件核验由用例在SQL锁外完成；容量结算和READY状态仍在统一事务内确认。
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "RESOURCE_COMPLETE", input, http.StatusOK, run, present)
}

func (h *Resources) UploadBytes(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["resourceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "RESOURCE_UPLOAD_BYTES", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, err := h.usecase.WriteUpload(r.Context(), actor, id, r.Body)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, actor.Sources, 0, "RESOURCE_UPLOAD_BYTES", run)
}

func (h *Resources) Quota(w http.ResponseWriter, r *http.Request) {
	space, err := domain.CanonicalID(pathvar.Vars(r)["spaceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_RESOURCE_QUOTA", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.PrepareQuota(r.Context(), actor, space)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 配额属于空间，由用例补齐空间身份来源；余额和版本通过同一读取事务输出。
	h.requests.Read(w, r, actor, sources, 0, "READ_RESOURCE_QUOTA", run)
}
