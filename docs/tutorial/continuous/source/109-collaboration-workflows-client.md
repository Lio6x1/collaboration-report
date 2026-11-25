# services/document/internal/client/collaboration_workflows_client.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/109-collaboration-workflows-client.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`CollaborationFence`、`NewCollaborationFence`、`Close`、`CommittedCut`、`FixedDraft`、`DraftTransitions`、`NewDraftTransitions`、`Preview`、`Apply`、`Prepare`、`Merges`、`NewMerges`、`MainRevision`、`CaptureMain`、`Compute`、`PrepareMain`、`ReconcileMain`、`NodePersistence`、`NewNodePersistence`、`UpdateOutcome`、`PersonalTransferStorage`、`NewPersonalTransferStorage`、`MeasurePersonalTransfer`、`FixedStructure`、`SubmissionCaptures`、`NewSubmissionCaptures`、`Capture`、`Finalize`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package client

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// collaboration_fence.go：保留原实现，按业务合并文件。

type CollaborationFence struct{ cfg platform.Config }

func NewCollaborationFence(cfg platform.Config) *CollaborationFence {
	return &CollaborationFence{cfg: cfg}
}
func (c *CollaborationFence) Close(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error) {
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	callCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	return pb.NewCollaborationInternalClient(peer).FenceAccess(callCtx, request)
}

// committed_submission_cut.go：保留原实现，按业务合并文件。

func (c *Merges) CommittedCut(ctx context.Context, query *pb.OperationQuery) (domain.CommittedSubmissionCut, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	var fact domain.CommittedSubmissionCut
	raw, err := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(query)
	if err != nil {
		return fact, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.cfg.Peers["collaborationHttp"]+"/infra/persistence/submission-cut", bytes.NewReader(raw))
	if err != nil {
		return fact, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return fact, domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 8192)).Decode(&fact) != nil {
		return fact, domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	return fact, nil
}

// discard_previews.go：保留原实现，按业务合并文件。

func (c *DraftTransitions) FixedDraft(ctx context.Context, input *pb.DraftPreviewRequest) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	// protobuf的uint64/bytes用官方JSON编码；普通encoding/json会使HTTP与RPC的原指纹绑定不一致。
	raw, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(input)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.cfg.Peers["collaborationHttp"]+"/infra/draft/fixed", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == 413 {
		return nil, domain.Violation(domain.StateConflict, "DISCARD_TRANSFER_LIMIT")
	}
	if response.StatusCode != 200 {
		return nil, errors.New("DISCARD_FIXED_UNAVAILABLE")
	}
	value, err := io.ReadAll(io.LimitReader(response.Body, (24<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(value) > 24<<20 {
		return nil, domain.Violation(domain.StateConflict, "DISCARD_TRANSFER_LIMIT")
	}
	return value, nil
}

// draft_transitions.go：保留原实现，按业务合并文件。

type DraftTransitions struct {
	cfg  platform.Config
	http *http.Client
}

func NewDraftTransitions(cfg platform.Config, peer *http.Client) *DraftTransitions {
	return &DraftTransitions{cfg: cfg, http: peer}
}
func (c *DraftTransitions) Preview(ctx context.Context, input *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).CaptureDraftPreview(ctx, input, grpc.MaxCallRecvMsgSize(32<<20))
}
func (c *DraftTransitions) Apply(ctx context.Context, input *pb.DraftResolutionRequest) (*pb.OperationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).ApplyDraftResolution(ctx, input, grpc.MaxCallSendMsgSize(32<<20))
}

// 准备入口仅传原上下文和摘要，正文从Node不可变预览读取，避免把浏览器或调用方正文当可靠草稿。
func (c *DraftTransitions) Prepare(ctx context.Context, input any) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.Peers["collaborationHttp"]+"/infra/draft/prepare", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusRequestEntityTooLarge {
		return nil, domain.Violation(domain.StateConflict, "DRAFT_TRANSFER_LIMIT")
	}
	if response.StatusCode != 200 {
		return nil, errors.New("DRAFT_PREPARE_UNAVAILABLE")
	}
	value, err := io.ReadAll(io.LimitReader(response.Body, (32<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(value) > 32<<20 {
		return nil, errors.New("DRAFT_PREPARE_TOO_LARGE")
	}
	return value, nil
}

// merges.go：保留原实现，按业务合并文件。

type Merges struct {
	cfg  platform.Config
	http *http.Client
}

func NewMerges(cfg platform.Config, peer *http.Client) *Merges { return &Merges{cfg: cfg, http: peer} }

// 只读MAIN的可靠尾部摘要，不做Capture；正文仍必须走原读取准入和固定版本接口。
func (c *Merges) MainRevision(ctx context.Context, document uint64) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	raw, err := json.Marshal(map[string]string{"document_id": domain.TextID(document)})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Peers["collaborationHttp"]+"/infra/main/revision", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "MAIN_REVISION_UNAVAILABLE")
	}
	defer response.Body.Close()
	var revision string
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&revision) != nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "MAIN_REVISION_UNAVAILABLE")
	}
	digest, err := hex.DecodeString(revision)
	if err != nil || len(digest) != 32 {
		return "", domain.Violation(domain.TemporarilyUnavailable, "MAIN_REVISION_UNAVAILABLE")
	}
	return revision, nil
}

func (c *Merges) CaptureMain(ctx context.Context, input *pb.CaptureMainRequest) (*pb.MainCaptureReply, error) {
	// worker租约覆盖多个步骤；单次跨服务调用仍受既定五秒边界约束，不能直接转发十五秒任务期限。
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).CaptureMainView(ctx, input, grpc.MaxCallRecvMsgSize(32<<20))
}

func (c *Merges) Compute(ctx context.Context, input *pb.MergeCandidateRequest) (*pb.MergeCandidateReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	// 三份各16MiB固定正文；仅本调用提高发送预算，不改变单份正文上限。
	return pb.NewCollaborationInternalClient(peer).ComputeMergeCandidate(ctx, input, grpc.MaxCallSendMsgSize(64<<20), grpc.MaxCallRecvMsgSize(32<<20))
}

func (c *Merges) PrepareMain(ctx context.Context, input *pb.PrepareMainCommitRequest) (*pb.MainBarrierReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).PrepareMainCommit(ctx, input)
}

func (c *Merges) ReconcileMain(ctx context.Context, input *pb.ReconcileMainCommitRequest) (*pb.InstallReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).ReconcileMainCommit(ctx, input, grpc.MaxCallSendMsgSize(32<<20), grpc.MaxCallRecvMsgSize(32<<20))
}

// node_persistence.go：保留原实现，按业务合并文件。

// NodePersistence只读取Node的持久提交事实，不替Doc决定资源能否解除保护。
type NodePersistence struct {
	http *http.Client
	url  string
}

func NewNodePersistence(httpClient *http.Client, url string) *NodePersistence {
	return &NodePersistence{http: httpClient, url: url}
}
func (s *NodePersistence) UpdateOutcome(ctx context.Context, req *pb.DraftResourceRequest) (domain.NodeUpdateProof, error) {
	var proof domain.NodeUpdateProof
	if s.http == nil {
		return proof, domain.RejectProtection(domain.ProtectionUnknown, "NODE_OUTCOME_UNKNOWN")
	}
	raw, _ := json.Marshal(map[string]string{"room_id": domain.TextID(req.RoomId), "workspace_epoch": domain.TextID(req.WorkspaceEpoch), "client_session_id": req.ClientSessionId, "client_update_id": req.ClientUpdateId})
	request, err := http.NewRequestWithContext(ctx, "POST", s.url+"/infra/persistence/update", bytes.NewReader(raw))
	if err != nil {
		return proof, err
	}
	request.Header.Set("Content-Type", "application/json")
	res, err := s.http.Do(request)
	if err != nil {
		return proof, domain.RejectProtection(domain.ProtectionUnknown, "NODE_OUTCOME_UNKNOWN")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 8192)).Decode(&proof) != nil {
		return proof, domain.RejectProtection(domain.ProtectionUnknown, "NODE_OUTCOME_UNKNOWN")
	}
	return proof, nil
}

// personal_transfer_storage.go：保留原实现，按业务合并文件。

type PersonalTransferStorage struct {
	endpoint string
	http     *http.Client
}

func NewPersonalTransferStorage(endpoint string, client *http.Client) *PersonalTransferStorage {
	return &PersonalTransferStorage{endpoint: endpoint, http: client}
}

func (c *PersonalTransferStorage) MeasurePersonalTransfer(ctx context.Context, plan domain.PersonalTransferPlan) (logic.PersonalTransferStorage, error) {
	var result logic.PersonalTransferStorage
	if plan.Document == 0 || !domain.ValidOperationID(plan.Operation) || len(plan.Scopes) != 2 {
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_PLAN_INVALID")
	}
	input := platform.PersonalTransferStorageRequest{Operation: plan.Operation, Document: domain.TextID(plan.Document), Generation: "1"}
	for _, scope := range plan.Scopes {
		if scope.Epoch == 0 || !domain.ValidOperationID(scope.Operation) ||
			(scope.Key != "doc-acl:"+input.Document && scope.Key != "publication:"+input.Document) {
			return result, errors.New("PERSONAL_TRANSFER_STORAGE_PLAN_INVALID")
		}
		input.Closures = append(input.Closures, platform.PersonalTransferStorageClosure{Revocation: scope.Operation, Scope: scope.Key, Epoch: domain.TextID(scope.Epoch)})
	}
	sort.Slice(input.Closures, func(i, j int) bool { return input.Closures[i].Scope < input.Closures[j].Scope })
	if input.Closures[0].Scope == input.Closures[1].Scope {
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_PLAN_INVALID")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/infra/lifecycle/personal-transfer-storage", bytes.NewReader(raw))
	if err != nil {
		return result, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_UNAVAILABLE")
	}
	defer response.Body.Close()
	// 丢回复只能再次查询相同原号。完整严格解码及目标核对防止另一篇的字节/摘要冒充本篇。
	var measured platform.PersonalTransferStorageMeasurement
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2048))
	decoder.DisallowUnknownFields()
	if response.StatusCode != 200 || decoder.Decode(&measured) != nil || decoder.Decode(new(any)) != io.EOF ||
		measured.Operation != input.Operation || measured.Document != input.Document || measured.Generation != "1" || !measured.SourceClosed {
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_UNCONFIRMED")
	}
	count, err := strconv.ParseUint(measured.Bytes, 10, 64)
	if err != nil || strconv.FormatUint(count, 10) != measured.Bytes {
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_UNCONFIRMED")
	}
	digest, err := hex.DecodeString(measured.Digest)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != measured.Digest {
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_UNCONFIRMED")
	}
	return logic.PersonalTransferStorage{Operation: plan.Operation, Document: plan.Document, Bytes: count, Digest: digest}, nil
}

// structure_previews.go：保留原实现，按业务合并文件。

func (c *DraftTransitions) FixedStructure(ctx context.Context, input *pb.DraftPreviewRequest) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	raw, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(input)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.cfg.Peers["collaborationHttp"]+"/infra/structure/fixed", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == 413 {
		return nil, domain.Violation(domain.StateConflict, "STRUCTURE_TRANSFER_LIMIT")
	}
	if response.StatusCode != 200 {
		return nil, errors.New("STRUCTURE_FIXED_UNAVAILABLE")
	}
	value, err := io.ReadAll(io.LimitReader(response.Body, (32<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(value) > 32<<20 {
		return nil, errors.New("STRUCTURE_FIXED_TOO_LARGE")
	}
	return value, nil
}

// submission_captures.go：保留原实现，按业务合并文件。

type SubmissionCaptures struct{ cfg platform.Config }

func NewSubmissionCaptures(cfg platform.Config) *SubmissionCaptures {
	return &SubmissionCaptures{cfg: cfg}
}

func (c *SubmissionCaptures) Capture(ctx context.Context, request *pb.CaptureSubmissionRequest) (*pb.CaptureReply, error) {
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	client := pb.NewCollaborationInternalClient(peer)
	// 固定正文允许 16 MiB；逐调用提高接收上限，不能意外沿用 gRPC 默认 4 MiB。
	query := &pb.OperationQuery{OperationId: request.Context.OperationId, Target: request.Context.Target, RequestFingerprint: request.Context.RequestFingerprint}
	known, err := client.GetCapture(ctx, query, grpc.MaxCallRecvMsgSize(32<<20))
	if err != nil {
		return nil, err
	}
	if known.State == pb.CaptureReply_FIXED {
		return known, nil
	}
	reply, err := client.CaptureSubmission(ctx, request, grpc.MaxCallRecvMsgSize(32<<20))
	// 仅这个明确的原回执绑定拒绝属于非法用户输入；来源关闭、超时或候选冲突仍由原号恢复。
	if status.Code(err) == codes.PermissionDenied && status.Convert(err).Message() == "FLUSH_BINDING_INVALID" {
		return nil, domain.Violation(domain.StateConflict, "SUBMISSION_FLUSH_INVALID")
	}
	return reply, err
}

func (c *SubmissionCaptures) Finalize(ctx context.Context, request *pb.FinalizeCaptureRequest) (*pb.OperationResult, error) {
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).FinalizeCapture(ctx, request, grpc.MaxCallRecvMsgSize(32<<20))
}
````

UTF-8/LF全文SHA-256：`f7c3f9b9b4cc6420688b2f507a48f99bbff07f3a5b2aac1ee10a3a1fc8280d46`。[参考实现](../../../../services/document/internal/client/collaboration_workflows_client.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
