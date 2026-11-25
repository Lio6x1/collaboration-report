// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"context"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/encoding/protojson"
	"net/http"
	"sync"
	"time"
)

// draft_sources.go：保留原实现，按业务合并文件。

type DraftSources struct {
	pulls       *logic.PullMain
	controls    *logic.DraftControls
	reverse     *logic.ReverseChanges
	resolutions *logic.Resolutions
}

func NewDraftSources(pulls *logic.PullMain, controls *logic.DraftControls, reverse *logic.ReverseChanges, resolutions *logic.Resolutions) *DraftSources {
	return &DraftSources{pulls: pulls, controls: controls, reverse: reverse, resolutions: resolutions}
}

func (h *DraftSources) Source(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	input := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(raw, input) != nil || input.Context == nil || input.Context.Target == nil {
		httpx.Fail(w, 400, "DRAFT_SOURCE_INVALID")
		return
	}
	// 证书只识别协作服务；具体正文资格必须由原操作用途分派并重新核验，不能靠客户端指定候选。
	var value map[string]any
	var err error
	switch input.Context.Target.Purpose {
	case "PULL_MAIN":
		value, err = h.pulls.DraftSource(r.Context(), input)
	case "DISCARD_PREVIEW":
		value, err = h.controls.PreviewSource(r.Context(), input)
	case "DISCARD_DRAFT":
		value, err = h.controls.DiscardSource(r.Context(), input)
	case "REVERSE":
		value, err = h.reverse.Source(r.Context(), input)
	case "CONFLICT_RESOLUTION":
		value, err = h.resolutions.Source(r.Context(), input)
	default:
		httpx.Fail(w, 400, "DRAFT_SOURCE_PURPOSE_INVALID")
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

func (h *DraftSources) Committed(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	query := &pb.OperationQuery{}
	if protojson.Unmarshal(raw, query) != nil || query.Target == nil {
		httpx.Fail(w, 400, "DRAFT_COMMIT_QUERY_INVALID")
		return
	}
	var value map[string]any
	var err error
	switch query.Target.Purpose {
	case "PULL_MAIN":
		value, err = h.pulls.CommittedSource(r.Context(), query)
	case "DISCARD_DRAFT":
		value, err = h.controls.DiscardCommittedSource(r.Context(), query)
	case "REVERSE":
		value, err = h.reverse.CommittedSource(r.Context(), query)
	case "CONFLICT_RESOLUTION":
		value, err = h.resolutions.CommittedSource(r.Context(), query)
	default:
		httpx.Fail(w, 400, "DRAFT_COMMIT_PURPOSE_INVALID")
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

// response_output.go：保留原实现，按业务合并文件。

// done只能在实际写入与Flush结束后关闭，取消信号本身不构成排空ACK。
type ResponseOutput struct {
	m       *logic.Outputs
	local   domain.OutputLease
	edge    *domain.OutputLease
	done    chan struct{}
	w       http.ResponseWriter
	stopped chan struct{}
	once    sync.Once
}

func (o *ResponseOutput) Abort() { close(o.done); o.m.Local().Remove(o.local.ID) }
func (o *ResponseOutput) Send(ctx context.Context, status int, value any) error {
	return o.send(ctx, status, value, 0)
}

// 固定整篇正文使用独立有限预算；编码失败或超限时尚未写出成功头/敏感字节。
func (o *ResponseOutput) SendBounded(ctx context.Context, status int, value any, budget int) error {
	return o.send(ctx, status, value, budget)
}

func (o *ResponseOutput) send(ctx context.Context, status int, value any, budget int) error {
	defer o.finish()
	if err := o.open(ctx); err != nil {
		return err
	}
	_ = http.NewResponseController(o.w).SetWriteDeadline(time.Now().Add(1500 * time.Millisecond))
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if budget > 0 && len(raw) > budget {
		return domain.Violation(domain.TemporarilyUnavailable, "FIXED_RESPONSE_TRANSFER_LIMIT")
	}
	// 设置期限与构造正文后再检查，避免覆盖撤权时设置的立即停止期限。
	if err := o.checkStopped(ctx); err != nil {
		return err
	}
	o.w.Header().Set("Content-Type", "application/json; charset=utf-8")
	o.w.Header().Set("Cache-Control", "no-store")
	o.w.WriteHeader(status)
	if _, err = o.w.Write(raw); err != nil {
		return err
	}
	return http.NewResponseController(o.w).Flush()
}

func (o *ResponseOutput) open(ctx context.Context) error {
	if o.edge != nil {
		var result struct {
			State string `json:"state"`
			ID    string `json:"instance_id"`
		}
		if err := o.m.EdgeCall(ctx, "/infra/output/open", o.edge, &result); err != nil {
			return err
		}
		if result.State != "OPEN" || result.ID != o.edge.Instance {
			return errors.New("OUTPUT_BINDING_MISMATCH")
		}
	}
	return o.checkStopped(ctx)
}

func (o *ResponseOutput) checkStopped(ctx context.Context) error {
	select {
	case <-o.stopped:
		return errors.New("OUTPUT_STOPPED")
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
func (o *ResponseOutput) finish() {
	close(o.done)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if o.m.Settle(ctx, o.local.ID, o.local.Instance) == nil {
		o.m.Local().Remove(o.local.ID)
	}
}

// transactions.go：保留原实现，按业务合并文件。

type Transactions struct {
	usecase *logic.Transactions
	outputs *Outputs
}

func NewTransactions(usecase *logic.Transactions, outputs *Outputs) *Transactions {
	return &Transactions{usecase: usecase, outputs: outputs}
}
func (h *Transactions) Read(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody) {
	h.ReadProjected(w, r, actor, sources, document, purpose, run, nil)
}

// 业务提交之后才编码传输投影；HTTP头与响应对象不进入用例事务。
func (h *Transactions) ReadProjected(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, present func(any) any) {
	h.readStatus(w, r, actor, sources, document, purpose, run, present, http.StatusOK)
}

// 已登记的异步操作也通过真实输出租约发送202；状态码适配不能绕过撤权与输出生命周期。
func (h *Transactions) ReadAccepted(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody) {
	h.readStatus(w, r, actor, sources, document, purpose, run, nil, http.StatusAccepted)
}

func (h *Transactions) readStatus(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, present func(any) any, status int) {
	h.readStatusBounded(w, r, actor, sources, document, purpose, run, present, status, 0)
}

func (h *Transactions) ReadBounded(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, budget int) {
	h.readStatusBounded(w, r, actor, sources, document, purpose, run, nil, http.StatusOK, budget)
}

func (h *Transactions) readStatusBounded(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, present func(any) any, status, budget int) {
	var output *ResponseOutput
	value, err := h.usecase.Read(r.Context(), actor, sources, document, purpose, r.Header.Get("X-Edge-Handler"), run, func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if present != nil {
		value = present(value)
	}
	if err := output.SendBounded(r.Context(), status, value, budget); err != nil {
		httpx.Failure(w, err)
	}
}
func (h *Transactions) PresentedMutation(w http.ResponseWriter, r *http.Request, actor logic.Actor, id, kind string, input any, status int, run logic.MutationBody, present logic.MutationPresentation) {
	h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input}, status, run, present)
}

func (h *Transactions) PreparedMutation(w http.ResponseWriter, r *http.Request, actor logic.Actor, id, kind string, input any, status int, initial map[string]any, run logic.MutationBody, present logic.MutationPresentation) {
	h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input, InitialResult: initial}, status, run, present)
}

func (h *Transactions) presentMutation(w http.ResponseWriter, r *http.Request, actor logic.Actor, command logic.OperationCommand, status int, run logic.MutationBody, present logic.MutationPresentation) {
	var output *ResponseOutput
	value, err := h.usecase.Mutation(r.Context(), actor, command, r.Header.Get("X-Edge-Handler"), run, present, func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err := output.Send(r.Context(), status, value); err != nil {
		httpx.Failure(w, err)
	}
}
func (h *Transactions) Replay(w http.ResponseWriter, r *http.Request, actor logic.Actor, id, kind string, input any, status int, policy logic.ReplayPolicy, present logic.MutationPresentation) bool {
	var output *ResponseOutput
	command := logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input}
	handled, value, err := h.usecase.TryReplay(r.Context(), actor, command, r.Header.Get("X-Edge-Handler"), policy, present, func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if !handled {
		return false
	}
	if err != nil {
		httpx.Failure(w, err)
		return true
	}
	if err := output.Send(r.Context(), status, value); err != nil {
		httpx.Failure(w, err)
	}
	return true
}
