# A07｜退出、自然到期和注销都靠原任务恢复，得到真实确认才写终态

## 这节课要实现什么

用户退出登录、会话到期或注销账号时，即使服务中途重启，也应把原来的关闭任务做完。本课实现这些任务的领取、重试和最终确认。

## 做完后能看到什么

接通后，旧会话不能再访问；响应丢失或进程退出不会让任务凭空消失。注销只有在职责和固定清理范围确认完成后才进入最终状态，处理中不会被直接改成成功。

## 先想清楚怎样实现

1. 先把退出与自然到期登记为持久任务。
2. 后台有界领取并向参与服务查询原结果。
3. 收到真实关闭和清理确认后，再完成回执与账号去标识化。

## 怎样判断这一课完成

A08后检查调度规则，06分开验证真实SQL领取、退出和完整注销；测试替身通过不代表远端连接已停止。

## 开始写之前

本课把前面 PREPARING/FROZEN 补成完整闭环。显式退出与自然到期关闭会话；改密关闭旧凭据来源；组织变化关闭旧成员/owner来源；注销还需要固定文档清理与本人去标识化。它们共享可靠领取思想，却有各自业务绑定和完成条件，不能混成一个“过几秒设成功”的worker。

功能讲义与分步页讲的是同一份代码，同名声明只输入一次。先理解本课功能推演，再沿分步页的完整顺序录入；章末全文只作核对。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

顺序分别为[模型](../../write/035-closure.md)、[会话SQL](../../write/056-session-closure-queries.md)、[注销收口SQL](../../write/046-account-close-completion-queries.md)、[Document客户端](../../write/044-document-authority-client.md)、[会话用例](../../write/067-session-closure-service.md)、[注销用例](../../write/057-account-close-completion-service.md)、[退出HTTP](../../write/073-logout.md)、[注销控制HTTP](../../write/068-account-close-completion-handlers.md)、[worker](../../write/081-closures-workers.md)、[依赖装配](../../write/083-organization-lifecycle-wiring.md)。每页逐段写齐；正文声明已写过只核对同名位置，不能重复追加。repository的statement/execute在本课会话SQL内，解决A01/A03的前向引用；A08再补响应来源和组合入口。

## 2. 写固定任务模型和消费者契约

<!-- lesson-code: services/account/internal/domain/account/closure.go :: SessionClosure -->
````go
// SessionClosure是持久任务的固定绑定。后续重试不能改写原scope或关闭代次。
type SessionClosure struct {
	Kind           string `json:"kind,omitempty"`
	UserID         string `json:"user_id,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
	Role           string `json:"role,omitempty"`
	SessionHash    string `json:"session_hash"`
	Scope          string `json:"scope_key"`
	Epoch          string `json:"closed_epoch"`
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/closure.go :: SessionClosePreparation -->
````go
type SessionClosePreparation struct {
	SessionID   []byte
	UserID      string
	Epoch       uint64
	OperationID string
	Scope       string
	Target      []byte
	Payload     []byte
	Fingerprint []byte
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/session_closure_service.go :: ClosurePeer -->
````go
type ClosurePeer interface {
	FenceAccess(context.Context, *pb.FenceRequest) (*pb.FenceResult, error)
	CloseDepartedGrants(context.Context, domain.SessionClosure, string) error
	FailureCode(error) string
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/account_close_completion_service.go :: AccountCloseRecoveryPeer -->
````go
type AccountCloseRecoveryPeer interface {
	AccountClosurePreparer
	Plan(context.Context, platform.AccountClosureBinding) (platform.AccountClosureRetention, error)
	Relations(context.Context, platform.AccountClosureBinding) (platform.AccountClosureRetention, error)
	Page(context.Context, platform.AccountClosurePageRequest) (platform.AccountClosurePage, error)
	CleanDocument(context.Context, platform.AccountClosureBinding, string) error
	Complete(context.Context, platform.AccountClosureBinding) error
}
````
<!-- /lesson-code -->

SessionClosure.Scope/Epoch 永远表示要关闭的原来源；Kind决定收口动作，UserID/OrganizationID/Role携带成员或凭据变化，SessionHash用于单会话。ClosureTask.ID是队列记录，OperationID是业务原号，Payload为原绑定；TaskOwnership.Instance/State只证明谁可以提交，不能决定业务成功。Preparation聚合原session/user/epoch、scope、target/payload/fingerprint，全部同tx写入。

ClosurePeer.FenceAccess返回生成ACK；CloseDepartedGrants只为ORG_LEAVE/ORG_REMOVE关闭组织下属授权，降级保留独立授权；FailureCode只供稳定日志。AccountCloseRecoveryPeer嵌入Preparer，Plan/Relations/Page返回既有平台协议，CleanDocument/Complete返回error；调用者核原Binding/PlanDigest/游标与状态后才推进。CancelPeer只提供同Binding的Cancel，不能从超时猜取消结果。

## 3. 先写显式退出，随后复用同边界登记自然到期

<!-- lesson-code: services/account/internal/logic/account/session_closure_service.go :: Logout.Prepare -->
````go
func (s *Logout) Prepare(ctx context.Context, id string, session domain.Session) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	// 与登录/凭据修改相同，先锁用户再锁会话，防止相反锁顺序导致死锁。
	credential, err := s.mutations.LockUser(ctx, tx, session.UserID)
	if err != nil {
		return "", errors.New("LOGOUT_RECHECK_FAILED")
	}
	current, err := s.closures.LockExpirySession(ctx, tx, session.ID)
	if err != nil {
		return "", errors.New("LOGOUT_RECHECK_FAILED")
	}
	if credential.State != "ACTIVE" || credential.Epoch != session.CredentialEpoch || !current.Expires.After(time.Now()) {
		return "", domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	if current.State != "ACTIVE" {
		// 初次查询没有receipt的同号请求，等锁后必须重读持久绑定。
		bound, state, err := s.receipts.ConcurrentLogout(ctx, tx, id)
		if err == nil && bound == hex.EncodeToString(session.ID) {
			return state, nil
		}
		return "", domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	digest := hex.EncodeToString(session.ID)
	scope := "session:" + digest
	target, _ := json.Marshal(map[string]string{"session_hash": digest, "scope_key": scope})
	fingerprint := sha256.Sum256([]byte("SESSION_LOGOUT\n" + session.UserID + "\n" + digest))
	payload, _ := json.Marshal(map[string]string{
		"session_hash": digest,
		"scope_key":    scope,
		"closed_epoch": strconv.FormatUint(current.Epoch, 10),
	})
	preparation := domain.SessionClosePreparation{
		SessionID:   session.ID,
		UserID:      session.UserID,
		Epoch:       current.Epoch,
		OperationID: id,
		Scope:       scope,
		Target:      target,
		Payload:     payload,
		Fingerprint: fingerprint[:],
	}
	if s.closures.PrepareSessionClose(ctx, tx, preparation) != nil || tx.Commit() != nil {
		return "", errors.New("LOGOUT_COMMIT_UNKNOWN")
	}
	return "PREPARING", nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/repository/account/session_closure_queries.go :: Closure.PrepareSessionClose -->
````go
func (r *Closure) PrepareSessionClose(ctx context.Context, tx *sql.Tx, value domain.SessionClosePreparation) error {
	// 会话状态、代次和持久gate是关闭权威；准入申请、复核及后台续期都在相同
	// user→session锁下核验它们。不能在这里改写会话全部历史准入：长期会话可
	// 积累数万条记录，使短事务反复超时回滚，反而无法提交关闭点及可靠任务。
	// 保留原准入供审计，OPEN也不代表仍有效；会话关闭后原证明不能再次通过核验。
	return execute(ctx, tx, []statement{
		{"INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,expires_at) VALUES(?,?,'SESSION_LOGOUT',?,?,'PREPARING',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY))", []any{value.OperationID, value.UserID, string(value.Target), value.Fingerprint}},
		{"UPDATE session SET state='REVOKING',epoch=epoch+1 WHERE id=?", []any{value.SessionID}},
		{"INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state='BLOCKED',last_revocation_id=VALUES(last_revocation_id),revision=revision+1", []any{value.Scope, value.Epoch, value.OperationID}},
		{"INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at) VALUES(?,?,?,?,?,'FENCING','[\"document\"]',UTC_TIMESTAMP(6))", []any{value.OperationID, value.UserID, value.Scope, value.Epoch, string(value.Target)}},
		{"INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('SESSION_LOGOUT',?,?,?,'QUEUED',UTC_TIMESTAMP(6))", []any{value.OperationID, value.OperationID, string(value.Payload)}},
	})
}
````
<!-- /lesson-code -->

例：用户17、会话H、epoch3退出。Resolve先选当前原session；若回执已有，只允许actor/kind/sessionhash一致的primary或recovery秘密，不能用已经关闭会话发起新号。Prepare持user→session锁，核当前credential代次、到期和ACTIVE；同号并发等锁后若session已REVOKING，再读ConcurrentLogout拿原绑定结果。首次写PREPARING receipt→session REVOKING/epoch4→session:H gate BLOCKED/原epoch3→authorization_change FENCING→SESSION_LOGOUT持久任务，Commit后返回PREPARING。

handler核Origin、dto.Action、operation格式、CSRF，提交后回202。logoutCookies只在浏览器当前SessionCookie恰等于原token时清它，旧重试不能清掉后来新登录会话；RecoveryCookie保存原秘密24小时，仅为原操作进度恢复。

然后写 EnqueueExpired、ExpiredSession/LockExpiryUser/LockExpirySession。浏览器永不再访问也会发现到期ACTIVE会话，锁user→session重新核到期和epoch可递增，使用固定 `expiry-`+sessionhash 原号调用同一PrepareSessionClose。池外发现只是候选，不构成关闭授权。这里不批量改写该长期会话积累的全部identity_admission历史；权威session状态/epoch/gate已使后续准入与重验拒绝，短事务才能快速提交关闭点。

## 4. 写领取、网络确认和收口

<!-- lesson-code: services/account/internal/repository/account/session_closure_queries.go :: claimClosureTask -->
````go
func claimClosureTask(ctx context.Context, tx *sql.Tx, kinds []string) (domain.ClosureTask, error) {
	if len(kinds) == 0 {
		return domain.ClosureTask{}, sql.ErrNoRows
	}
	predicate := "kind IN (" + strings.TrimSuffix(strings.Repeat("?,", len(kinds)), ",") + ") AND ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6)) OR (state='RUNNING' AND lease_until<=UTC_TIMESTAMP(6)))"
	args := make([]any, len(kinds))
	for index, kind := range kinds {
		args[index] = kind
	}
	// 排序候选不加范围锁、不搬运载荷。FOR UPDATE的filesort会锁住多个候选，
	// 即使SKIP LOCKED也令其他worker无法领取；到期登记也可能被其间隙锁阻塞。
	rows, err := tx.QueryContext(ctx, "SELECT id FROM durable_task WHERE "+predicate+" ORDER BY id LIMIT 32", args...)
	if err != nil {
		return domain.ClosureTask{}, err
	}
	var ids []uint64
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return domain.ClosureTask{}, err
	}
	for _, id := range ids {
		var task domain.ClosureTask
		lockedArgs := append([]any{id}, args...)
		// 候选快照不授予执行资格；精确主键当前读重新核验种类、状态及期限。
		err = tx.QueryRowContext(ctx, "SELECT id,business_key,payload FROM durable_task WHERE id=? AND ("+predicate+") FOR UPDATE SKIP LOCKED", lockedArgs...).Scan(&task.ID, &task.OperationID, &task.Payload)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		return task, err
	}
	return domain.ClosureTask{}, sql.ErrNoRows
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/session_closure_service.go :: Closure.RecoverOne -->
````go
func (s *Closure) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()

	// 先提交固定任务的持久租约，网络调用在事务外；未知结果仍重取原任务。
	task, err := s.repository.Claim(ctx, tx)
	if err != nil {
		return
	}
	if s.repository.MarkRunning(ctx, tx, task.ID, instance) != nil || tx.Commit() != nil {
		return
	}
	var binding domain.SessionClosure
	var request *pb.FenceRequest
	if json.Unmarshal(task.Payload, &binding) == nil {
		epoch, epochError := strconv.ParseUint(binding.Epoch, 10, 64)
		session, sessionError := hex.DecodeString(binding.SessionHash)
		if epochError == nil && ((sessionError == nil && len(session) == 32) || binding.UserID != "") {
			request = &pb.FenceRequest{
				RevocationId: task.OperationID,
				ClosedScopes: []*pb.SourceEpoch{{ScopeKey: binding.Scope, Epoch: epoch}},
				Generation:   1,
			}
			if binding.UserID != "" {
				request.TargetDigest = platform.SourceClosureDigest(binding.Scope, epoch)
			} else {
				request.TargetDigest = platform.LogoutTargetDigest(binding.SessionHash)
			}
		}
	}
	if request == nil {
		s.repository.Retry(ctx, task.ID, instance, "LOGOUT_PAYLOAD_INVALID")
		return
	}
	// 离开关系才关闭组织下属授权；降级保留独立授权。持久授权关闭先于排空ACK。
	if binding.Kind == "ORG_LEAVE" || binding.Kind == "ORG_REMOVE" {
		if s.peer.CloseDepartedGrants(ctx, binding, task.OperationID) != nil {
			s.repository.Retry(ctx, task.ID, instance, "DEPARTURE_GRANTS_UNKNOWN")
			return
		}
	}
	rpcCtx, done := context.WithTimeout(ctx, 2*time.Second)
	ack, err := s.peer.FenceAccess(rpcCtx, request)
	done()
	if err != nil || !platform.MatchesBootstrapAck(request, ack, "document") {
		// 日志只记录稳定码，不输出私有payload、原始令牌和依赖驱动详情。
		slog.Warn("logout_fence_pending", "operation_id", task.OperationID, "rpc_code", s.peer.FailureCode(err))
		s.repository.Retry(ctx, task.ID, instance, "FENCE_NOT_CONFIRMED")
		return
	}
	s.finish(ctx, task, instance, binding, ack.ResultDigest)
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/session_closure_service.go :: Closure.finish -->
````go
func (s *Closure) finish(ctx context.Context, task domain.ClosureTask, instance string, binding domain.SessionClosure, digest []byte) {
	if binding.Kind == "PLATFORM_GOVERNANCE_REVOKE" {
		s.finishGovernance(ctx, task, instance, binding, digest)
		return
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	ownership, err := s.repository.LockOwnership(ctx, tx, task.ID)
	if err != nil || ownership.Instance != instance || ownership.State != "RUNNING" {
		return
	}
	isCredential := binding.Kind == "CHANGE_PASSWORD" || binding.Kind == "RESET_PASSWORD" || binding.Kind == "CHANGE_EMAIL"
	var oldEpoch uint64
	if isCredential || binding.OrganizationID != "" {
		oldEpoch, err = strconv.ParseUint(binding.Epoch, 10, 64)
		if err != nil || oldEpoch == ^uint64(0) {
			return
		}
	}
	session, _ := hex.DecodeString(binding.SessionHash)
	poll := "/v1/account/operations/" + task.OperationID
	terminal, _ := json.Marshal(dto.Operation{OperationId: task.OperationID, State: "SUCCEEDED", PollUrl: &poll})
	if s.repository.FinishBase(ctx, tx, task, binding, session, digest, terminal) != nil {
		return
	}
	if isCredential && s.repository.FinishCredential(ctx, tx, binding.UserID, oldEpoch) != nil {
		return
	}
	if binding.OrganizationID != "" {
		switch binding.Kind {
		case "ORG_ROLE", "ORG_OWNER_TRANSFER":
			if s.repository.FinishMemberRole(ctx, tx, binding.OrganizationID, binding.UserID, binding.Role, oldEpoch+1) != nil {
				return
			}
		case "ORG_REMOVE", "ORG_LEAVE":
			if s.repository.FinishMemberDeparture(ctx, tx, binding.OrganizationID, binding.UserID, oldEpoch+1) != nil {
				return
			}
		}
		if s.repository.FinishOrganizationTransition(ctx, tx, task.OperationID) != nil {
			return
		}
		if binding.Kind == "ORG_OWNER_TRANSFER" {
			if s.repository.CreateOwnerTransferNotice(ctx, tx, task.OperationID, binding.OrganizationID, binding.UserID) != nil {
				return
			}
		}
	}
	// commit响应丢失仍是未知事实；下轮恢复读原记录，不能补写假的终态。
	_ = tx.Commit()
}
````
<!-- /lesson-code -->

先普通查询最多32个候选ID，随后按精确主键 `FOR UPDATE SKIP LOCKED` 当前读核种类、状态、retry/lease到期。不能把ORDER BY候选扫描直接加FOR UPDATE：filesort/范围锁可能锁住其他候选，令多个worker仍串行，也挡住到期登记。

Claim与MarkRunning在同一tx提交5秒租约后才网络调用。RecoverOne总体4秒，Fence RPC最多2秒；原payload构造Generation=1及正确LogoutTargetDigest/SourceClosureDigest。退出/移除成员先CloseDepartedGrants，失败原任务RETRY；然后严格MatchesBootstrapAck(document)。RPC超时可能远端已成功，继续原请求重取，不新造revocation。

finish新tx核task Instance/RUNNING。FinishBase保存真实OUTPUT_STOPPED摘要、会话CLOSED、authorization/receipt/task终态；凭据再FinishCredential关闭旧代次会话、user在新credential代次恢复ACTIVE；成员role/owner转移恢复新epoch ACTIVE，离开/移除设CLOSED并关建库资格；组织transition成功，owner转移还在同tx创建A05通知。任一失败回滚全部，Commit回复丢失下轮查原记录，不补假终态。平台治理走A04独立finishGovernance锁角色再锁task。

## 5. 写注销取消与清理推进

先写ACCOUNT_CLOSE_CANCEL的Claim/Binding/Retry/Finish/Ready及Cancellations.RecoverOne：只允许Account已同tx保存的REJECTED原Binding去Document取消，远端权威确认后重核任务执行权结束；网络未知保留原号。

接着写Recoveries.read/advance/persist/requeueCurrent/finishTerminal/Ready，按以下状态推演：

<!-- lesson-code: services/account/internal/logic/account/account_close_completion_service.go :: AccountCloseRecoveries.advance -->
````go
func (s *AccountCloseRecoveries) advance(ctx context.Context, task domain.AccountCloseFenceTask) error {
	record, capture, plan, err := s.read(ctx, task)
	if err != nil {
		return err
	}
	if record.State == "SUCCEEDED" || record.State == "REJECTED" {
		return s.finishTerminal(ctx, task, record.User)
	}
	if record.State == "PREPARING" {
		result, err := s.closures.PrepareClose(ctx, task.Operation, s.peer)
		if err != nil {
			return err
		}
		if result.State == "PREPARING" && len(result.RequiredSuccessions) > 0 {
			return accountCloseConflict("ACCOUNT_CLOSE_HANDOFF_REQUIRED")
		}
		return s.requeueCurrent(ctx, task, record.User)
	}
	if capture.Binding == nil {
		return errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
	}
	binding := *capture.Binding
	switch record.State {
	case "FROZEN":
		confirmed, err := s.closures.Confirmed(ctx, binding)
		if err != nil {
			return err
		}
		if !confirmed {
			return accountCloseConflict("ACCOUNT_CLOSE_DRAIN_PENDING")
		}
		if _, err = s.peer.Plan(ctx, binding); err != nil {
			return err
		}
		request := platform.AccountClosurePageRequest{AccountClosureBinding: binding}
		page, err := s.peer.Page(ctx, request)
		if err != nil {
			return err
		}
		if !page.Matches(request) {
			return errors.New("ACCOUNT_CLOSE_PAGE_INVALID")
		}
		capture.Retention = &domain.AccountCloseRetention{PlanDigest: page.PlanDigest}
		plan.State = "PLANNED"
	case "PLANNED":
		if capture.Retention == nil {
			return errors.New("ACCOUNT_CLOSE_RETENTION_REQUIRED")
		}
		if _, err = s.peer.Relations(ctx, binding); err != nil {
			return err
		}
		plan.State = "CLEANING"
	case "CLEANING":
		if capture.Retention == nil {
			return errors.New("ACCOUNT_CLOSE_RETENTION_REQUIRED")
		}
		request := platform.AccountClosurePageRequest{AccountClosureBinding: binding, AfterDocument: capture.Retention.AfterDocument, PlanDigest: capture.Retention.PlanDigest}
		page, err := s.peer.Page(ctx, request)
		if err != nil {
			return err
		}
		if !page.Matches(request) {
			return errors.New("ACCOUNT_CLOSE_PAGE_INVALID")
		}
		if len(page.Documents) > 0 {
			if capture.Retention.DocCompleted {
				return errors.New("ACCOUNT_CLOSE_COMPLETION_CURSOR_CHANGED")
			}
			document := page.Documents[0]
			if err = s.peer.CleanDocument(ctx, binding, document.Document); err != nil {
				return err
			}
			capture.Retention.AfterDocument = document.Document
			if document.Release != "0" {
				plan.RetainedReleaseIds = append(plan.RetainedReleaseIds, document.Release)
			}
		} else {
			if !page.Done {
				return errors.New("ACCOUNT_CLOSE_PAGE_INCOMPLETE")
			}
			if page.State == "SUCCEEDED" {
				return s.finalize(ctx, task, record, capture, plan, page)
			}
			if capture.Retention.DocCompleted {
				return errors.New("ACCOUNT_CLOSE_DOC_COMPLETION_CHANGED")
			}
			if err = s.peer.Complete(ctx, binding); err != nil {
				return err
			}
			capture.Retention.DocCompleted = true
		}
	default:
		return errors.New("ACCOUNT_CLOSE_STATE_INVALID")
	}
	return s.persist(ctx, task, record, capture, plan)
}
````
<!-- /lesson-code -->

1. PREPARING调用A06 PrepareClose；有RequiredSuccessions则保持原任务等待交接，不宣布注销成功。FROZEN先Confirmed：全部原scope任务/authorization/正确ACK/blocked gate都要匹配，再Document.Plan/Page固定PlanDigest，进入PLANNED。
2. PLANNED调用Relations清理原关系，收到明确结果才进入CLEANING。
3. CLEANING以原Binding+PlanDigest+AfterDocument取Page并Matches。每次只清该页第一份Document；CleanDocument确认PAYLOADS_REMOVED后才推进AfterDocument，并记录该文档固定公开Release（非0）。响应未知就重清同一个文档，不跳游标、不保存全文。
4. 空页必须Done。若远端尚未SUCCEEDED，Complete确认后保存DocCompleted，下一轮重新Page；只有同计划空终页SUCCEEDED才finalize。客户端返回200但字段/摘要不合也不能推进。

persist用ReadCommitted tx锁回原user/receipt/account_close，比较预期state和Capture字节，核任务generation/instance/lease，再保存计划与回执、释放本轮任务以继续领取。两个worker读过同一页时只有持当前执行权且原cursor未变的一方能推进；业务原号不会因领取代次改变。

## 6. 最后写去标识化：原ACK、原终页、本地事实一起核对

<!-- lesson-code: services/account/internal/repository/account/account_close_completion_queries.go :: AccountClosures.ConfirmedScopes -->
````go
// 用户锁先于本方法：屏障完成事务也锁同一用户，确认读取不会把不同提交拼成ACK。
// 每页仅按种类和精确business_key访问本计划的任务，避免扫描历史队列。
func (r *AccountClosures) ConfirmedScopes(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture) (bool, error) {
	if err := validateAccountCloseFreeze(binding, capture); err != nil {
		return false, err
	}
	expected := make(map[string]domain.AccountCloseScope, len(capture.Scopes))
	for _, source := range capture.Scopes {
		expected[source.Revocation] = source
	}
	for offset := 0; offset < len(capture.Scopes); offset += 100 {
		end := offset + 100
		if end > len(capture.Scopes) {
			end = len(capture.Scopes)
		}
		args := []any{domain.AccountCloseFenceKind}
		for _, source := range capture.Scopes[offset:end] {
			args = append(args, source.Revocation)
		}
		rows, err := tx.QueryContext(ctx, `SELECT COALESCE(t.operation_id,''),t.business_key,t.payload,t.state,
			COALESCE(a.actor_id,0),COALESCE(a.scope_key,''),COALESCE(a.epoch,0),COALESCE(a.state,''),a.target_permission,a.participant_plan,
			COALESCE(g.max_epoch,0),COALESCE(g.state,''),COALESCE(g.last_revocation_id,''),ack.result_digest,
			COALESCE(ack.phase,'')
			FROM durable_task t LEFT JOIN authorization_change a ON a.revocation_id=t.business_key
			LEFT JOIN access_gate g ON g.scope_key=a.scope_key
			LEFT JOIN authorization_ack ack ON ack.revocation_id=a.revocation_id AND ack.participant='document' AND ack.scope_key=a.scope_key AND ack.epoch=a.epoch
			WHERE t.kind=? AND t.business_key IN (`+strings.TrimSuffix(strings.Repeat("?,", end-offset), ",")+`)`, args...)
		if err != nil {
			return false, err
		}
		count := 0
		for rows.Next() {
			var operation, revocation, taskState, scope, authorizationState, gateState, gateRevocation, phase string
			var actor, epoch, gateEpoch uint64
			var payload, target, participants, digest []byte
			err = rows.Scan(&operation, &revocation, &payload, &taskState, &actor, &scope, &epoch, &authorizationState, &target, &participants, &gateEpoch, &gateState, &gateRevocation, &digest, &phase)
			if err != nil {
				rows.Close()
				return false, err
			}
			source, found := expected[revocation]
			parsed, decodeErr := domain.DecodeAccountCloseFence(payload)
			if !found || decodeErr != nil || operation != binding.Operation || parsed.Operation != binding.Operation || parsed.User != binding.User || parsed.Revocation != revocation || parsed.Scope != source.Scope || parsed.Epoch != source.Epoch || actor != binding.User || scope != source.Scope || epoch != source.Epoch || gateEpoch != epoch || gateState != "BLOCKED" || gateRevocation != revocation {
				rows.Close()
				return false, errors.New("ACCOUNT_CLOSE_CONFIRMATION_BINDING_INVALID")
			}
			var targetBinding struct {
				Operation string `json:"account_close_operation"`
				Scope     string `json:"scope_key"`
				Epoch     uint64 `json:"epoch"`
			}
			var participantPlan []string
			if json.Unmarshal(target, &targetBinding) != nil || targetBinding.Operation != operation || targetBinding.Scope != scope || targetBinding.Epoch != epoch || json.Unmarshal(participants, &participantPlan) != nil || len(participantPlan) != 1 || participantPlan[0] != "document" {
				rows.Close()
				return false, errors.New("ACCOUNT_CLOSE_CONFIRMATION_BINDING_INVALID")
			}
			if taskState != "SUCCEEDED" || authorizationState != "SUCCEEDED" {
				rows.Close()
				return false, nil
			}
			request := &pb.FenceRequest{RevocationId: revocation, Generation: 1, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope, Epoch: epoch}}, TargetDigest: platform.SourceClosureDigest(scope, epoch)}
			if phase != "OUTPUT_STOPPED" || !bytes.Equal(digest, platform.BootstrapDigest(request, "document")) {
				rows.Close()
				return false, errors.New("ACCOUNT_CLOSE_CONFIRMATION_ACK_INVALID")
			}
			delete(expected, revocation)
			count++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return false, err
		}
		if count != end-offset {
			return false, nil
		}
	}
	return len(expected) == 0, nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/account_close_completion_service.go :: AccountCloseRecoveries.finalize -->
````go
// 仅接受原计划最后一页的真实Doc SUCCEEDED；没有页面/清理资格、网络未知或
// 本人冻结/任一ACK不完整都继续等待。最后个人信息、组织关系、回执和原任务同事务。
func (s *AccountCloseRecoveries) finalize(ctx context.Context, task domain.AccountCloseFenceTask, expected domain.AccountCloseRecord, capture domain.AccountCloseCapture, plan dto.AccountClosePlan, proof platform.AccountClosurePage) error {
	if capture.Binding == nil || capture.Retention == nil {
		return errors.New("ACCOUNT_CLOSE_FINAL_PROOF_REQUIRED")
	}
	request := platform.AccountClosurePageRequest{AccountClosureBinding: *capture.Binding, PlanDigest: capture.Retention.PlanDigest, AfterDocument: capture.Retention.AfterDocument}
	if !proof.Matches(request) || proof.State != "SUCCEEDED" || !proof.Done || len(proof.Documents) != 0 {
		return errors.New("ACCOUNT_CLOSE_FINAL_PROOF_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, original, _, err := s.closures.lockBinding(ctx, tx, *capture.Binding)
	if err != nil {
		return err
	}
	if current.State != "CLEANING" || current.State != expected.State || !bytes.Equal(current.Capture, expected.Capture) {
		return errors.New("ACCOUNT_CLOSE_FINAL_STATE_CHANGED")
	}
	user, err := s.closures.repository.LockUser(ctx, tx, capture.Binding.User)
	if err != nil {
		return err
	}
	if capture.IdentityEpoch == ^uint64(0) || user.State != "REVOKING" || user.IdentityEpoch != capture.IdentityEpoch+1 || user.Epoch != strconv.FormatUint(capture.CredentialEpoch, 10) {
		return errors.New("ACCOUNT_CLOSE_FINAL_IDENTITY_CHANGED")
	}
	confirmed, err := s.closures.repository.ConfirmedScopes(ctx, tx, *capture.Binding, original)
	if err != nil {
		return err
	}
	if !confirmed {
		return accountCloseConflict("ACCOUNT_CLOSE_FINAL_DRAIN_PENDING")
	}
	if err = repository.FinalizeAccountCloseData(ctx, tx, *capture.Binding, capture); err != nil {
		return err
	}
	if err = repository.ValidateAccountCloseRecovery(ctx, tx, task); err != nil {
		return err
	}
	plan.State = "SUCCEEDED"
	capture.Retention.DocCompleted = true
	rawCapture, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err = repository.SaveAccountCloseRetention(ctx, tx, task.Operation, current.State, plan.State, rawCapture, rawPlan); err != nil {
		return err
	}
	if err = repository.CompleteAccountCloseReceipt(ctx, tx, *capture.Binding, proof.PlanDigest, rawPlan); err != nil {
		return err
	}
	if err = repository.FinishAccountCloseRecovery(ctx, tx, task, true); err != nil {
		return err
	}
	return tx.Commit()
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/repository/account/account_close_completion_queries.go :: FinalizeAccountCloseData -->
````go
// 调用方已持原user/receipt/account_close锁且核对Doc完成证明和全部原ACK。
// 组织按原编号锁定，存续组织只关闭本人关系，不删除其他人的内容或分配负责人。
func FinalizeAccountCloseData(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture) error {
	for _, organization := range capture.Organizations {
		var owner, memberRevision uint64
		var state string
		if err := tx.QueryRowContext(ctx, "SELECT owner_id,lifecycle,member_revision FROM organization WHERE id=? FOR UPDATE", organization.ID).Scan(&owner, &state, &memberRevision); err != nil {
			return err
		}
		closing := slices.Contains(binding.ClosingOrganizations, organization.ID)
		if memberRevision == math.MaxUint64 || (closing && (state != "CLOSING" || owner != binding.User)) || (!closing && (state != "ACTIVE" || owner == binding.User)) {
			return errors.New("ACCOUNT_CLOSE_FINAL_ORGANIZATION_CHANGED")
		}
		var epoch uint64
		var membership string
		if err := tx.QueryRowContext(ctx, "SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE", organization.ID, binding.User).Scan(&membership, &epoch); err != nil {
			return err
		}
		if organization.Epoch == math.MaxUint64 || membership != "REVOKING" || epoch != organization.Epoch+1 {
			return errors.New("ACCOUNT_CLOSE_FINAL_MEMBERSHIP_CHANGED")
		}
		if closing {
			var others bool
			if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM organization_member WHERE organization_id=? AND user_id<>? AND state<>'CLOSED')", organization.ID, binding.User).Scan(&others); err != nil {
				return err
			}
			if others {
				return errors.New("ACCOUNT_CLOSE_FINAL_ORGANIZATION_NOT_EMPTY")
			}
			if _, err := tx.ExecContext(ctx, "UPDATE organization SET name='已关闭组织',lifecycle='CLOSED',closed_at=UTC_TIMESTAMP(6),member_revision=member_revision+1 WHERE id=?", organization.ID); err != nil {
				return err
			}
		} else if _, err := tx.ExecContext(ctx, "UPDATE organization SET member_revision=member_revision+1 WHERE id=?", organization.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE organization_member SET state='CLOSED' WHERE organization_id=? AND user_id=?", organization.ID, binding.User); err != nil {
			return err
		}
		var saturated bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND state='ACTIVE' AND revision=18446744073709551615)
		 OR EXISTS(SELECT 1 FROM organization_invitation WHERE organization_id=? AND inviter_id=? AND state='OPEN' AND revision=18446744073709551615)`, organization.ID, binding.User, organization.ID, binding.User).Scan(&saturated); err != nil {
			return err
		}
		if saturated {
			return errors.New("ACCOUNT_CLOSE_FINAL_REVISION_EXHAUSTED")
		}
		if _, err := tx.ExecContext(ctx, "UPDATE organization_creation_grant SET state='CLOSED',revision=revision+1 WHERE organization_id=? AND user_id=? AND state='ACTIVE' AND revision<18446744073709551615", organization.ID, binding.User); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE organization_invitation SET state='CLOSED',revision=revision+1 WHERE organization_id=? AND inviter_id=? AND state='OPEN' AND revision<18446744073709551615", organization.ID, binding.User); err != nil {
			return err
		}
	}
	var active bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM session WHERE user_id=? AND state='ACTIVE')", binding.User).Scan(&active); err != nil {
		return err
	}
	if active {
		return errors.New("ACCOUNT_CLOSE_FINAL_SESSION_NOT_FROZEN")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE session SET state='CLOSED' WHERE user_id=? AND state='REVOKING'", binding.User); err != nil {
		return err
	}
	var saturated bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM organization_application WHERE applicant_id=? AND state='PENDING' AND revision=18446744073709551615)", binding.User).Scan(&saturated); err != nil {
		return err
	}
	if saturated {
		return errors.New("ACCOUNT_CLOSE_FINAL_REVISION_EXHAUSTED")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE organization_application SET state='CANCELLED',revision=revision+1 WHERE applicant_id=? AND state='PENDING' AND revision<18446744073709551615", binding.User); err != nil {
		return err
	}
	changed, err := tx.ExecContext(ctx, `UPDATE user SET state='CLOSED',email=NULL,email_key=NULL,password_hash=NULL,password_algorithm=NULL,
	 display_name='已注销用户',email_verified_at=NULL,closed_at=UTC_TIMESTAMP(6),credential_epoch=credential_epoch+1
	 WHERE id=? AND state='REVOKING' AND identity_epoch=? AND credential_epoch=? AND credential_epoch<18446744073709551615`, binding.User, binding.IdentityEpoch+1, capture.CredentialEpoch)
	return accountCloseChanged(changed, err)
}
````
<!-- /lesson-code -->

finalize先验原Binding/Retention/终页Matches、SUCCEEDED/Done/空Documents，再锁原数据：必须仍CLEANING、Capture未变、本人REVOKING/identity=原+1/credential相同，全部原ACK齐。FinalizeAccountCloseData按固定组织顺序核owner/lifecycle/member epoch；属于ClosingOrganizations且无其他存续成员的组织改名“已关闭组织”并CLOSED，存续组织只关闭本人关系和增member_revision，不删别人的内容。

随后关闭本人建库资格/邀请/待审批申请和旧sessions，拒绝饱和revision或还ACTIVE的session；user email/email_key/password_hash/algorithm/email_verified_at置NULL、display_name“已注销用户”、state CLOSED、credential_epoch+1。保存account_close/receipt SUCCEEDED、审计和原task终态同tx提交。公开保留事实由Document固定release计划决定；账号去标识化不能提前删除私有载荷，也不把后来私有正文当公开版本留下。

Resolve/Progress让冻结原会话仅读原AccountClosePlan；不相符/匿名统一ACCOUNT_CLOSE_NOT_FOUND，恢复Cookie不能成为新的登录资格。MinimalClosureReceipt只放行精确closures路由、原号、四字段、合法编号与规范JSON字节，不许夹带姓名/正文/目录或重复key；A08将它作为专用输出规则使用。

## 7. 接好真实客户端、worker生命周期和svc

document_authority_client.go按操作页写全部AccountClosureControl、Document、Departure、AccountOutput、OrganizationCloseControl适配器。它们接平台已建立的mTLS *http.Client/RPC连接，send严格读完整JSON/状态200；Prepare/Plan/Relations/Complete分别核FROZEN或HANDOFF_REQUIRED、PLANNED、CLEANING、SUCCEEDED；Page核Matches，CleanDocument核PAYLOADS_REMOVED及原operation/document。不能把浏览器自报服务名当证书身份。

<!-- lesson-code: services/account/internal/worker/closures_workers.go :: RunSessionClosures -->
````go
func RunSessionClosures(ctx context.Context, service sessionClosures) {
	// 到期登记不等待下游关闭：浏览器离线产生的一批原号应先持久入队。
	// 固定4个领取者使用既有SKIP LOCKED、有限租约及真实ACK逻辑，限制SQL/RPC压力；
	// 任一不可达任务只占用一个worker，不能阻塞其他独立范围的恢复。
	var workers sync.WaitGroup
	for n := 0; n < 4; n++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			instance := platform.NewTraceID()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					service.RecoverOne(ctx, instance)
				}
			}
		}()
	}
	defer workers.Wait()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.EnqueueExpired(ctx)
		}
	}
}
````
<!-- /lesson-code -->

RunAccountCloseRecovery每种流程独立100ms轮询、各自instance；RunSessionClosures固定4个独立领取者，每100ms恢复一项，到期登记另在主循环，不被某个不可达范围阻塞。Context通知停止，WaitGroup等受控workers退出。svc.NewClosure/NewLogout和组织各构造器传同一Store.DB及Mutation、Document peer、CaptureSource；A08 bootstrap启动Fence/Cancel/Recover/组织恢复/Session workers，并由Store.Context统一退出。

## 8. 检查：离线有界调度、真实SQL领取、整条注销分别记录

离线检查在A08包齐后：

```powershell
Set-Location -LiteralPath (Join-Path $lessonRoot 'services/account')
go test ./internal/worker -run '^TestSessionClosureBacklogUsesBoundedIndependentOwners$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

预期RUN/PASS：恰4个非空独立owner，最多4个同时活动，到期登记继续、取消后受控退出。它用替身，不证明SQL锁行为。本次文档任务未执行。

下面真实SQL测试等06自己的实例迁移就绪，显式设置自己的配置；未设置会SKIP，不能算PASS：

```powershell
$env:COLLAB_TEST_ACCOUNT_CONFIG = Join-Path $lessonRoot '.runtime/services/account.json'
Set-Location "$lessonRoot\services\account"
go test ./internal/repository/account -run '^TestActualClosureClaimsDoNotLockOtherCandidatesOrRegistration$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

应确认实际RUN/PASS且无SKIP，证明一项精确领取不锁其他候选/到期登记。完整四服务、开发SMTP、Kafka/ES/WSS、迁移与Edge齐备后，回自己的工程根目录串行执行以下真实已有测试：

```powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/account/expiry.test.mjs
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
node --no-maglev --test --test-reporter=tap --test-concurrency=1 --test-name-pattern='^account closure persists and completes original public/private cleanup before deidentification$' tests/lifecycle/account-close.test.mjs
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
node --no-maglev --test --test-reporter=tap --test-concurrency=1 --test-name-pattern='^account closure resumes the persisted original task after actual owner process exits$' tests/lifecycle/account-close-recovery.test.mjs
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

第一条精确名 `expired sessions are durably fenced even when the browser never sends logout`：推进本例会话到期事实后核401及真实关闭。第二条从真实注册/WSS发布注销，核公开固定版本保留、后来私有数据清理先于去标识化。第三条会停止/重启自己的owner服务，用真实Node排空未知和Account退出验证原任务恢复；只GET原进度，不重POST，不造ACK。它有180秒场景上限，属于专用恢复演练，普通文档更新不重复运行。本次均未执行。

失败先查原task/state/lease与绑定，再查authorization ACK和Document固定page。长期FROZEN可能真实下游不可达，CLEANING卡住可能PAYLOADS_REMOVED未确认；不能删除原任务、清空数据库或手改SUCCEEDED。进程归属失败停在原实例排查，不能按node/go进程名批量结束。

## 9. 章末核对

[关闭模型](../../source/035-closure.md)、[会话SQL](../../source/056-session-closure-queries.md)、[注销SQL](../../source/046-account-close-completion-queries.md)、[客户端](../../source/044-document-authority-client.md)、[会话用例](../../source/067-session-closure-service.md)、[注销用例](../../source/057-account-close-completion-service.md)、[退出HTTP](../../source/073-logout.md)、[控制HTTP](../../source/068-account-close-completion-handlers.md)、[worker](../../source/081-closures-workers.md)、[svc](../../source/083-organization-lifecycle-wiring.md)。

为什么CleanDocument返回未知不能先推进游标？可能尚未删除，也可能已删除但回复丢失；停在原文档重取明确幂等回执才能兼顾不漏删与不重复改变范围。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
Set-Location -LiteralPath $lessonRoot
$paths = @(
 'services/account/internal/domain/account/closure.go',
 'services/account/internal/repository/account/session_closure_queries.go',
 'services/account/internal/repository/account/account_close_completion_queries.go',
 'services/account/internal/client/document_authority_client.go',
 'services/account/internal/logic/account/session_closure_service.go',
 'services/account/internal/logic/account/account_close_completion_service.go',
 'services/account/internal/handler/account/logout.go',
 'services/account/internal/handler/account/account_close_completion_handlers.go',
 'services/account/internal/worker/closures_workers.go',
 'services/account/internal/svc/organization_lifecycle_wiring.go'
)
foreach ($path in $paths) {
 New-Item -ItemType Directory -Force (Split-Path $path) | Out-Null
 if (-not (Test-Path -LiteralPath $path)) { New-Item -ItemType File $path | Out-Null }
}
```
