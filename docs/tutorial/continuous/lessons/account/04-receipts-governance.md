# A04｜只恢复原操作的结果，并持久授予或撤销平台治理资格

## 这节课要实现什么

用户提交的改密或撤权操作还在处理中，刷新后应能继续查看同一次操作；平台维护者也需要明确任免治理人员。本课实现原结果查询与受控的治理资格变更。

## 做完后能看到什么

接通后，原请求者能读取属于自己的操作进度和结果，换人或换目标不能复用回执；平台治理身份必须显式授予，普通注册不会自动获得。

## 先想清楚怎样实现

1. 先定义原回执和读取资格。
2. 再读取持久进度并核对撤权确认。
3. 最后通过受限运维入口授予或撤销治理角色，沿用可靠关闭流程。

## 怎样判断这一课完成

A08后运行回执与治理规则，06验证原操作恢复；治理命令只在自己隔离实例执行，不把运维入口当作公开注册接口。

## 开始写之前

A01 改密、A03 撤权已经会返回 PREPARING。本课补齐“用户刷新后怎样知道原结果”及“服务怎样证明旧权限真的停止”。同时实现仅本机运维可调用的平台治理角色任免，不给普通注册账号隐含管理员权力。

功能讲义与分步页讲的是同一份代码，同名声明只输入一次。先理解本课功能推演，再沿分步页的完整顺序录入；章末全文只作核对。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

按[回执模型](../../write/041-operation-receipt.md)→[回执SQL](../../write/053-operation-receipts-queries.md)→[治理SQL](../../write/049-governance-queries.md)→[回执用例](../../write/064-operation-receipts-service.md)→[治理用例](../../write/060-governance-service.md)→[进度HTTP](../../write/075-operation-progress.md)→[运维命令](../../write/086-main.md)分段输入。先完成类型，再编排读取/任免；HTTP Progress 是 A07 Logout 的方法，A07/A08 写齐后构建整个包。

## 2. 模型：恢复资格、持久快照与固定投影

<!-- lesson-code: services/account/internal/domain/account/operation_receipt.go :: OperationReceipt -->
````go
type OperationReceipt struct {
	ActorID     string
	Kind        string
	State       string
	SessionHash string
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/operation_receipt.go :: LogoutSelection -->
````go
type LogoutSelection struct {
	Session      Session
	Token        string
	Registered   bool
	ReceiptState string
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/repository/account/operation_receipts_queries.go :: OperationResultSnapshot -->
````go
// OperationResultSnapshot包括原不可变绑定和同一行的版本/结果，不能把多次查询的
// state与generation拼接；授权前后由用例核验该快照仍为同一个持久事实。
type OperationResultSnapshot struct {
	Actor, Kind, State, SessionHash string
	Fingerprint, Result             []byte
	Generation                      uint64
	Terminal                        bool
}
````
<!-- /lesson-code -->

OperationReceipt.ActorID/Kind/SessionHash 绑定发起人、操作族和原会话，State 是持久状态。LogoutSelection.Token 是该次恢复可用的原会话秘密；Registered 表示操作已经登记，ReceiptState 不等于用户今天的权限。OperationProgress.State/Result 二选路径：被关闭的原会话只读最小进度；当前有效身份才有机会读普通业务结果。

Snapshot 将 Actor/Kind/State/SessionHash、Fingerprint、Generation、Result、Terminal 在一次 SQL 中读取，不能拿两次 SELECT 的 state 和 generation 拼接。generation 来自原回执，不是 worker 租约代次。RPC 生成 pb.AccountOperationQuery 的 SessionToken/RecoveryReceipt 复用 HTTP 原文恢复秘密，不能自创一个 operation_id 就当 bearer token。

OperationResults 使用 Logout/AccountClosures 的真实 Progress 来核授权，没有额外仓储接口；SQL 读取是 ReadOperationResult。GovernanceOperatorInput 明确 Operation、User、Action、Expected，Expected 是治理 grant 的修订；普通 HTTP Mutation 的 HMAC 与该本地运维输入的 SHA-256 指纹各有固定编码，不随意混用。

## 3. 写回执查询和 Progress，再写固定 RPC 结果

先输入 OperationReceipts.Lookup/ConcurrentLogout/CurrentCredential/Result 和 ReadOperationResult。随后写 Logout.Progress、OperationResult helper、OperationResults.Get。

<!-- lesson-code: services/account/internal/logic/account/operation_receipts_service.go :: Logout.Progress -->
````go
// Progress对已关闭的原会话只返回该操作的最小进度，不交付目标或其他业务投影。
func (s *Logout) Progress(ctx context.Context, id, primaryToken, recoveryToken string) (domain.OperationProgress, error) {
	receipt, err := s.receipts.Lookup(ctx, id)
	notFound := domain.RuleViolation{Kind: domain.EntityNotFound, Code: "OPERATION_NOT_FOUND"}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OperationProgress{}, notFound
	}
	if err != nil {
		return domain.OperationProgress{}, err
	}
	boundOperation := receipt.Kind == "SESSION_LOGOUT" || receipt.Kind == "CHANGE_PASSWORD" || receipt.Kind == "CHANGE_EMAIL"
	for _, token := range []string{primaryToken, recoveryToken} {
		session, err := s.authentication.Session(ctx, token)
		if err == nil && boundOperation && receipt.ActorID == session.UserID && receipt.SessionHash == hex.EncodeToString(session.ID) {
			return domain.OperationProgress{State: receipt.State}, nil
		}
	}
	if !boundOperation {
		session, err := s.authentication.Session(ctx, primaryToken)
		if err == nil && session.State == "ACTIVE" && session.Expires.After(time.Now()) && receipt.ActorID == session.UserID {
			credential, err := s.receipts.CurrentCredential(ctx, session.UserID)
			if err == nil && credential.State == "ACTIVE" && credential.Epoch == session.CredentialEpoch {
				raw, err := s.receipts.Result(ctx, id)
				if err == nil && len(raw) > 0 {
					return domain.OperationProgress{Result: raw}, nil
				}
			}
		}
	}
	return domain.OperationProgress{}, notFound
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/operation_receipts_service.go :: OperationResults.Get -->
````go
// Get仅查询已存在的原操作。recovery_receipt沿HTTP恢复Cookie的真实语义处理：
// 它是原会话秘密，不是新发明的bearer；权限全部复用原Progress，不调用Resolve/Start。
func (s *OperationResults) Get(ctx context.Context, query *pb.AccountOperationQuery) (*pb.OperationResult, error) {
	if query == nil || !domain.ValidOperationID(query.OperationId) {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "OPERATION_ID_INVALID"}
	}
	if s == nil || s.db == nil || s.logout == nil || s.closures == nil {
		return nil, operationResultUnavailable()
	}
	before, err := repository.ReadOperationResult(ctx, s.db, query.OperationId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, operationResultNotFound()
	}
	if err != nil {
		return nil, err
	}
	primary, recovery := string(query.SessionToken), string(query.RecoveryReceipt)
	var authorized minimalAccountResult
	if before.Kind == "ACCOUNT_CLOSE" {
		plan, err := s.closures.Progress(ctx, query.OperationId, primary, recovery)
		if err != nil {
			return nil, normalizeOperationResultError(err)
		}
		authorized = minimalAccountResult{Operation: plan.OperationId, State: plan.State}
	} else {
		progress, err := s.logout.Progress(ctx, query.OperationId, primary, recovery)
		if err != nil {
			return nil, normalizeOperationResultError(err)
		}
		if progress.State != "" {
			authorized = minimalAccountResult{Operation: query.OperationId, State: progress.State}
		} else if json.Unmarshal(progress.Result, &authorized) != nil {
			return nil, operationResultUnavailable()
		}
	}
	// 原权限返回和本次RPC必须属于同一操作/会话绑定。读后比较整份持久快照，
	// 连同结果字节核验：即使关闭器没有递增generation也不能混用两阶段结果。
	after, err := repository.ReadOperationResult(ctx, s.db, query.OperationId)
	if err != nil || !sameOperationResult(before, after) {
		return nil, operationResultUnavailable()
	}
	if authorized.Operation != query.OperationId || after.Generation == 0 || len(after.Fingerprint) != 32 {
		return nil, operationResultUnavailable()
	}
	if len(after.Result) > 0 {
		var stored minimalAccountResult
		if json.Unmarshal(after.Result, &stored) != nil || stored != authorized {
			return nil, operationResultUnavailable()
		}
	} else if authorized.State != after.State || after.Kind == "ACCOUNT_CLOSE" {
		return nil, operationResultUnavailable()
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: after.Generation, State: pb.OperationResult_PROCESSING}
	terminal := true
	switch after.State {
	case "PREPARING":
		terminal = false
	case "SUCCEEDED":
		result.State = pb.OperationResult_SUCCEEDED
	case "REJECTED":
		result.State = pb.OperationResult_REJECTED
	case "CANCELLED":
		result.State = pb.OperationResult_CANCELLED
	default:
		return nil, operationResultUnavailable()
	}
	raw, err := json.Marshal(authorized)
	if err != nil {
		return nil, err
	}
	if !terminal {
		if after.Terminal {
			return nil, operationResultUnavailable()
		}
		result.CurrentStateJsonUtf8 = raw
		return result, nil
	}
	if !after.Terminal || len(after.Result) == 0 || authorized.State != after.State {
		return nil, operationResultUnavailable()
	}
	// generation来自原receipt；terminal_revision定义为持久终态的receipt代次，
	// 不采用后台租约代次或当前身份epoch。摘要只覆盖最小固定投影的JSON字节，
	// 不冒充完整result_json摘要，也不向已关闭会话交付实体/交接/保留清单。
	result.TerminalRevision = after.Generation
	result.FixedResultJsonUtf8 = raw
	digest := sha256.Sum256(raw)
	result.ResultDigest = digest[:]
	return result, nil
}
````
<!-- /lesson-code -->

推演 operation=`logout-course-001`、actor17、原 session 摘要 H：

1. GET 原操作先 Lookup。查无回执为 OPERATION_NOT_FOUND，数据库故障仍是 error，不假造 PREPARING。
2. SESSION_LOGOUT/CHANGE_PASSWORD/CHANGE_EMAIL 允许 primary 或 recovery token 对应原 Session，必须 actor17、SessionHash=H；即使会话已关闭也只返回该回执 State。这不是允许它调用 `/me` 或另一次变更。
3. 普通操作要求 primary session ACTIVE、未过期、当前 user ACTIVE/credential_epoch吻合，再读 Result；无资格统一404，不能泄露别人的操作存在性。
4. RPC Get 在授权前后各读一份 Snapshot，并完整比较 actor/kind/state/sessionhash/generation/terminal/fingerprint/result 字节。授权过程中 worker 从 PREPARING 写到 SUCCEEDED，返回不可用，让客户端重读，不能混合两代结果。
5. PREPARING 只填 CurrentStateJsonUtf8，TerminalRevision 与固定摘要都不填。终态 SUCCEEDED/REJECTED/CANCELLED 还需 terminal_at 和结果存在且状态一致，才填 FixedResultJsonUtf8、TerminalRevision=原generation、SHA-256 摘要。

最小 JSON 仅 operation_id/state，不携带实体、交接清单、个人保留数据。RPC state=SUCCEEDED 只有在持久终态证据完整时才成立；结果 JSON 自称成功但 terminal_at 缺失必须不可用。这个双读校验不是长事务锁定，职责是拒绝混阶段快照，不冒充全局串行化。

## 4. 给授权关闭签证明：核原范围和真实 ACK

回到 A02 已输入的 AuthorizationResults 与仓储 AuthorizationResultFacts.Read，理解其完整输入链。Read 联查 authorization_change、access_gate、原 operation_receipt、durable_task、参与者 ACK，以及需要时账号/组织关闭原计划。

<!-- lesson-code: services/account/internal/logic/account/identity_service.go :: authorizationProof -->
````go
func authorizationProof(operation string, f repository.AuthorizationResultFacts) (*pb.FenceRequest, error) {
	var participants []string
	if !platform.ValidAccountOutputScope(f.Scope) || f.Epoch == 0 || f.Actor != f.ReceiptActor || len(f.Fingerprint) != sha256.Size ||
		(f.State != "FENCING" && f.State != "SUCCEEDED") || (f.ReceiptState != "PREPARING" && f.ReceiptState != "SUCCEEDED") ||
		json.Unmarshal(f.Participants, &participants) != nil || len(participants) != 1 || participants[0] != "document" || !validAuthorizationTaskState(f.TaskState) {
		return nil, authorizationUnavailable()
	}
	if !validAuthorizationOriginal(operation, f) {
		return nil, authorizationUnavailable()
	}
	// 普通关闭在同一事务结束receipt；账号/组织关闭的来源ACK先于全局业务终态，
	// 不能把它们仍在清理的父回执误当缺少来源证明。
	if (f.State == "FENCING" && f.ReceiptState != "PREPARING") ||
		(f.State == "SUCCEEDED" && f.TaskKind != domain.AccountCloseFenceKind && f.TaskKind != repository.OrganizationCloseRecoveryKind && (f.ReceiptState != "SUCCEEDED" || !f.ReceiptTerminal)) {
		return nil, authorizationUnavailable()
	}
	request := &pb.FenceRequest{RevocationId: operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: f.Scope, Epoch: f.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(f.Scope, f.Epoch)}
	if strings.HasPrefix(f.Scope, "session:") {
		request.TargetDigest = platform.LogoutTargetDigest(strings.TrimPrefix(f.Scope, "session:"))
	}
	if !platform.ValidBootstrapFence(request) {
		return nil, authorizationUnavailable()
	}
	// 同代gate须仍是原关闭点；更高代次只允许带真实终态ACK的旧范围重读。
	// 新身份来源可重新OPEN，但不能抹掉已完成的旧epoch或替旧未知范围补证明。
	if f.GateEpoch < f.Epoch || (f.GateEpoch == f.Epoch && (f.GateState != "BLOCKED" || f.GateRevocation != operation)) ||
		(f.GateEpoch > f.Epoch && (f.State != "SUCCEEDED" || (f.GateState != "OPEN" && f.GateState != "BLOCKED"))) {
		return nil, authorizationUnavailable()
	}
	if f.State == "SUCCEEDED" {
		if f.AckEpoch != f.Epoch || f.AckPhase != "OUTPUT_STOPPED" || !f.AckCompleted || !bytes.Equal(f.AckDigest, platform.BootstrapDigest(request, "document")) ||
			(f.TaskKind != repository.OrganizationCloseRecoveryKind && f.TaskState != "SUCCEEDED") {
			return nil, authorizationUnavailable()
		}
	} else if len(f.AckDigest) != 0 || f.TaskState == "SUCCEEDED" {
		return nil, authorizationUnavailable()
	}
	return request, nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/identity_service.go :: AuthorizationResults.Fence -->
````go
// Fence只反向证明Account原关闭范围，绝不登记新范围或再调用Doc Fence。
// 真实Doc ACK未落库时返回PROCESSING，由原Account持久协调器继续恢复。
func (s *AuthorizationResults) Fence(ctx context.Context, input *pb.FenceRequest) (*pb.FenceResult, error) {
	if !platform.ValidBootstrapFence(input) {
		return nil, authorizationInvalid()
	}
	f, original, err := s.facts(ctx, input.RevocationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(original, input) {
		return nil, authorizationInvalid()
	}
	if f.State != "SUCCEEDED" {
		return &pb.FenceResult{State: pb.FenceResult_PROCESSING, RevocationId: original.RevocationId, ClosedScopes: original.ClosedScopes, Generation: original.Generation}, nil
	}
	// Account摘要只在原持久关闭和实际参与者ACK已核验后签出，不把空参与者当成功。
	return platform.BootstrapAck(original, "account"), nil
}
````
<!-- /lesson-code -->

比如 credential:17、epoch4、revocation=`password-course-01`：请求必须与原范围/原generation=1/原 TargetDigest 完全相等；participant_plan 必须恰为 document，任务payload的kind、user、scope、epoch要和原回执相符。FENCING 返回 PROCESSING，不能把空参与者或超时当 ACK。SUCCEEDED 必须有 document 的 OUTPUT_STOPPED、完成时间、相同 epoch 与正确 BootstrapDigest，且相关任务和回执达到协议要求，才签 Account ACK。

更高 gate 水位可以允许旧终态证明重读，但不能给旧未知范围补成功。账号关闭子撤权的 scope/epoch/revocation 必须在父 capture 恰好出现一次；指纹/目标取原 ACCOUNT_CLOSE 父回执，不能从当前用户权限倒推出历史。GetAuthorizationChange 的 Target 由原 scope、epoch、两个规范 JSON 目标摘要固定映射，文档ID字段保持零，不能凭空猜文档ID。反向证明入口只读原事实，不再调用 Document Fence，避免循环关闭。

## 5. 写平台治理任免事务与撤权收口

先写 GovernanceUserActive/GovernanceGrantRow/GrantGovernance/RevokeGovernance/CompleteGovernanceRevocation/InsertGovernanceOperatorReceipt，再写 admissionSources 的专用用途分支、GovernanceOperator.Change/Current/Ready 和 Closure.finishGovernance。

<!-- lesson-code: services/account/internal/logic/account/governance_service.go :: GovernanceOperatorInput -->
````go
type GovernanceOperatorInput struct {
	Operation string `json:"operation_id"`
	User      uint64 `json:"user_id"`
	Action    string `json:"action"`
	Expected  uint64 `json:"expected_revision"`
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/governance_service.go :: GovernanceOperator.Change -->
````go
func (s *GovernanceOperator) Change(ctx context.Context, input GovernanceOperatorInput) (json.RawMessage, error) {
	if input.User == 0 || !domain.ValidOperationID(input.Operation) || (input.Action != "GRANT" && input.Action != "REVOKE") {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "PLATFORM_GOVERNANCE_INPUT_INVALID"}
	}
	raw, _ := json.Marshal(input)
	fingerprint := sha256.Sum256(raw)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	active, err := repository.GovernanceUserActive(ctx, tx, input.User)
	if err != nil || !active {
		return nil, errors.New("PLATFORM_GOVERNANCE_USER_NOT_ACTIVE")
	}
	// 持目标user锁序列化同一角色变更；原号重取不能用后来修订改写原请求。
	receipt, err := repository.NewMutation().Receipt(ctx, tx, input.Operation)
	if err == nil {
		if receipt.ActorID != "0" || receipt.Kind != "PLATFORM_GOVERNANCE_OPERATOR" || !bytes.Equal(receipt.Fingerprint, fingerprint[:]) {
			return nil, errors.New("PLATFORM_GOVERNANCE_OPERATION_CONFLICT")
		}
		return receipt.Result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	grant, err := repository.GovernanceGrantRow(ctx, tx, input.User, true)
	if errors.Is(err, sql.ErrNoRows) {
		grant = repository.GovernanceGrant{}
	} else if err != nil {
		return nil, err
	}
	if grant.Revision != input.Expected || grant.State == "REVOKING" {
		return nil, errors.New("PLATFORM_GOVERNANCE_REVISION_CHANGED")
	}
	if grant.Epoch == ^uint64(0) || grant.Revision == ^uint64(0) {
		return nil, errors.New("PLATFORM_GOVERNANCE_EPOCH_EXHAUSTED")
	}
	state := "SUCCEEDED"
	switch input.Action {
	case "GRANT":
		if grant.State != "ACTIVE" {
			if err = repository.GrantGovernance(ctx, tx, input.User, input.Operation, grant); err != nil {
				return nil, err
			}
		}
	case "REVOKE":
		if grant.State == "ACTIVE" {
			if err = repository.RevokeGovernance(ctx, tx, input.User, input.Operation); err != nil {
				return nil, err
			}
			closure := domain.SessionClosure{Kind: "PLATFORM_GOVERNANCE_REVOKE", UserID: strconv.FormatUint(input.User, 10), Scope: "platform-admin:" + strconv.FormatUint(input.User, 10), Epoch: strconv.FormatUint(grant.Epoch, 10)}
			if err = EnqueueSourceClosure(ctx, tx, input.Operation, "0", closure); err != nil {
				return nil, err
			}
			state = "PREPARING"
		}
	}
	result, _ := json.Marshal(map[string]string{"operation_id": input.Operation, "state": state})
	if err = repository.InsertGovernanceOperatorReceipt(ctx, tx, input.Operation, fingerprint, raw, result, state); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, errors.New("PLATFORM_GOVERNANCE_COMMIT_UNKNOWN")
	}
	return result, nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/governance_service.go :: Closure.finishGovernance -->
````go
func (s *Closure) finishGovernance(ctx context.Context, task domain.ClosureTask, instance string, binding domain.SessionClosure, digest []byte) {
	user, err := strconv.ParseUint(binding.UserID, 10, 64)
	if err != nil || user == 0 || strconv.FormatUint(user, 10) != binding.UserID || binding.Scope != "platform-admin:"+binding.UserID {
		return
	}
	epoch, err := strconv.ParseUint(binding.Epoch, 10, 64)
	if err != nil || epoch == 0 || epoch == ^uint64(0) || strconv.FormatUint(epoch, 10) != binding.Epoch {
		return
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	// 先角色，再任务ownership，与任免入口保持同一锁顺序；真实Doc ACK已经核验。
	grant, err := repository.GovernanceGrantRow(ctx, tx, user, true)
	if err != nil || grant.State != "REVOKING" || grant.Epoch != epoch+1 || grant.Operation != task.OperationID {
		return
	}
	ownership, err := s.repository.LockOwnership(ctx, tx, task.ID)
	if err != nil || ownership.Instance != instance || ownership.State != "RUNNING" {
		return
	}
	if err = repository.CompleteGovernanceRevocation(ctx, tx, user, epoch+1, task.OperationID); err != nil {
		return
	}
	terminal, _ := json.Marshal(dto.Operation{OperationId: task.OperationID, State: "SUCCEEDED"})
	if err = s.repository.FinishBase(ctx, tx, task, binding, nil, digest, terminal); err != nil {
		return
	}
	// 未知提交仍重取原角色/回执；绝不按租约结束补写成功。
	_ = tx.Commit()
}
````
<!-- /lesson-code -->

正常授予 user17、Action=GRANT、Expected=0：ReadCommitted tx→锁 ACTIVE user→读原号receipt→锁 grant；不存在视 revision0→插入 ACTIVE grant→写 actor0/kind=PLATFORM_GOVERNANCE_OPERATOR 回执与审计→Commit→SUCCEEDED。已存在同号先核完整指纹再取原结果；Expected错、REVOKING、epoch/revision耗尽都拒绝。

撤销 ACTIVE grant revision3/epoch2：Revoke 改 REVOKING、epoch3、revision4；EnqueueSourceClosure 固定 platform-admin:17 的旧epoch2，写 gate、authorization_change、PLATFORM_GOVERNANCE_CHANGE 任务；回执 PREPARING。A07 Closure 获得真实 Document ACK 后 finishGovernance 先锁 grant，要求仍 REVOKING/epoch3/changed_operation_id原号，再锁任务owner，最后条件 UPDATE CLOSED 与 FinishBase 同 tx。旧worker、后来任免、错原号都不能提交。

RETAINED_GOVERNANCE 准入不接受“普通ACTIVE_ACCOUNT就够了”：Identity.admissionSources 还锁当前平台 grant，要求 ACTIVE，添加 platform-admin 来源。撤权关闭旧来源，使已有准入失效。GovernanceRecoveryReady 在重启时核角色、固定payload、gate和authorization绑定一致；提交回复丢失保留 `PLATFORM_GOVERNANCE_COMMIT_UNKNOWN`，重取原事实，不能按租约结束补成功。

## 6. 接入口与可复现检查

HTTP `GET /v1/account/operations/:operationId` → Logout.Progress handler → 上面用例，URL只负责路由参数，不提供授权。A08 server.Identity.GetAccountOperation/GetAuthorizationChange/FenceAccess 接固定 RPC，并由实际 peer 策略保护；BindOperationResults 与 BindAuthorizationResults 在装配时明确注入。

治理不注册公共 HTTP。A08 包齐、06自己的配置就绪后，先查询自己的测试账号再任免；下列17只是示例，必须改成自己创建的 ID，不能照抄去改体验环境：

```powershell
Set-Location -LiteralPath (Join-Path $lessonRoot 'services/account')
$AccountConfig = Join-Path $lessonRoot '.runtime/services/account.json'
$TestUserID = '17'
go run ./cmd/platform-governance --config $AccountConfig --user-id $TestUserID --action status
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
# 首次 status 返回 revision=0 时才使用0；之后使用实际 status 修订。
go run ./cmd/platform-governance --config $AccountConfig --user-id $TestUserID --action grant --operation-id governance-grant-0001 --expected-revision 0
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
# 授予成功后读取实际修订，再演练撤销自己的测试账号资格。
$GrantStatus = go run ./cmd/platform-governance --config $AccountConfig --user-id $TestUserID --action status | ConvertFrom-Json
if ($LASTEXITCODE -ne 0) { throw '未能读取治理角色，不能猜预期修订' }
go run ./cmd/platform-governance --config $AccountConfig --user-id $TestUserID --action revoke --operation-id governance-revoke-0001 --expected-revision $GrantStatus.revision
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

配置扩展名和实际路径以06生成结果为准，上面的路径变量必须指向自己的 Account 配置。run 要求绝对路径、Role=account、规范十进制ID，整体三秒期限；终端只输出稳定业务码，不打印SQL/配置原文。首次grant应SUCCEEDED且status为ACTIVE，revoke原ACTIVE资格应PREPARING，真实关闭worker确认后status为CLOSED。UNKNOWN重试原号和原参数；再次读取Current只观察状态，不替原操作补成功。

离线检查在 A08 包齐后执行，不需要四服务：

```powershell
go test ./internal/logic/account -run '^(TestAccountOperationResultOriginalRecoveryAndMinimalStableTerminal|TestAccountOperationResultRecoveryCannotReplaceCurrentSessionOrMixGeneration|TestAccountOperationResultProcessingAndUnprovenTerminalRemainUnfixed)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
go test ./internal/logic/account -run '^(TestAuthorizationRPCOriginalProofAndFixedQueryBinding|TestAuthorizationRPCPendingAndCorruptEvidenceNeverAcknowledge|TestAuthorizationRPCAccountCloseChildUsesOriginalParentAndScope|TestAuthorizationRPCExistingSourceFamiliesKeepOriginalBindings)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

应 RUN/PASS 全部精确测试，覆盖原秘密恢复、快照变更拒绝、最小终态、未知不固定、错target/指纹/ACK和父子范围拒绝。本次文档任务未运行。没有发现名称先核支持测试；状态不合预期检查原 receipt/authorization/task/ACK 绑定，不只看一个 state 字符串。

真实 SQL任免与旧治理准入失效等06四服务验收；上述测试不能代替它。PREPARING 超时应保持原号检查恢复链；不能重新GRANT绕开未知撤权。

## 7. 章末核对

[模型](../../source/041-operation-receipt.md)、[回执SQL](../../source/053-operation-receipts-queries.md)、[治理SQL](../../source/049-governance-queries.md)、[回执用例](../../source/064-operation-receipts-service.md)、[治理用例](../../source/060-governance-service.md)、[HTTP进度](../../source/075-operation-progress.md)、[本机运维命令](../../source/086-main.md)。为什么RPC终态不带完整实体结果？原关闭会话只保留恢复原操作的有限资格，不能借回执取得关闭后的业务投影。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
Set-Location -LiteralPath $lessonRoot
$paths = @(
 'services/account/internal/domain/account/operation_receipt.go',
 'services/account/internal/repository/account/operation_receipts_queries.go',
 'services/account/internal/repository/account/governance_queries.go',
 'services/account/internal/logic/account/operation_receipts_service.go',
 'services/account/internal/logic/account/governance_service.go',
 'services/account/internal/handler/account/operation_progress.go',
 'services/account/cmd/platform-governance/main.go'
)
foreach ($path in $paths) {
 New-Item -ItemType Directory -Force (Split-Path $path) | Out-Null
 if (-not (Test-Path -LiteralPath $path)) { New-Item -ItemType File $path | Out-Null }
}
```
