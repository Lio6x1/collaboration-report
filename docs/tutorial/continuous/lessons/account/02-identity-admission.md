# A02｜把真实会话变成短期身份准入，并让每次写入可重试

## 这节课要实现什么

文档服务收到一名用户的请求，需要知道他现在是否仍是组织成员或管理员；用户重试一次组织操作时，也不应重复写入。本课实现跨服务身份确认和可恢复的写入基础。

## 做完后能看到什么

接通后，身份确认会绑定本次用途和目标，过期或已撤销资格不能继续使用；同号同内容重试得到原结果，同号换内容不能冒充原请求。这里还不创建具体组织业务。

## 先想清楚怎样实现

1. 先用真实会话取得短期身份凭证。
2. 再重验资格来源并提供姓名查询和续期。
3. 最后将业务写入与原操作回执放进同一数据库事务。

## 怎样判断这一课完成

账号包在A08齐备后执行相关规则；真实身份接口和同号重试在06验证，不能用自填用户编号代替登录会话。

## 开始写之前

接着 A01 的同一学习工程写。本课处理两个真实需求：Document 不能读账号库，却必须核验当前成员/管理员资格；组织写接口遇到响应丢失，需要同号得到原事实。认证证明“当前是谁”，准入还绑定本次用途、目标和来源代次，Mutation 则将本地业务与回执放在同一事务。

功能讲义与分步页讲的是同一份代码，同名声明只输入一次。先理解本课功能推演，再沿分步页的完整顺序录入；章末全文只作核对。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先写[仓储操作页](../../write/050-identity-queries.md)的 package/import、Queryer、Identity 与查询方法，解决 A01 的 Queryer 引用；然后写[用例操作页](../../write/061-identity-service.md)，按下面身份→Mutation→游标→授权结果四遍理解；最后写[身份 HTTP 操作页](../../write/072-identity-handlers.md)及[分页/内部会话操作页](../../write/071-common-handlers.md)。每页逐声明输入，不能只输入正文挑出的函数。`Identity.admissionSources` 的平台管理员分支在 A04、来源捕获在 A08，完整包检查仍留 A08。

## 2. 先理解模型和实际依赖形状

复用生成 `pb.IdentityRequest/IdentityAdmission/IdentityRevalidation`，不抄 DTO。Request 的 OperationId 是本业务幂等号；SessionToken 是解码后的 32 字节原始会话随机值，进入账号事务前做 SHA-256，区别于 HTTP Cookie 的 Base64 字符串。Purpose 必须等于 Target.Purpose；Target 定位文档动作，IdentityTarget 定位账号/组织资格；RequiredIdentityCapability 是此次要求的 ACTIVE_ACCOUNT、组织成员/管理员/所有者或建库资格。

返回 IdentityAdmission 包括 AdmissionId、ActorId、SessionRef 摘要、Sources 列表、绝对到期毫秒、Purpose、两个目标、Capability、OperationId。Sources 中每项 `(scope_key,epoch)` 都是撤权来源，必须一起保存。Proof 不是“30秒内永远允许”：真正业务提交前仍要重验。

现有 Identity 依赖具体 `*repository.Identity`，没有另一个消费者仓储接口；不要为了课程虚构接口。仓储 `Queryer` 是真实的最小查询能力，`*sql.DB` 与 `*sql.Tx` 都可实现；带锁方法统一接调用方 tx。Mutation 的消费者契约则明确用回调：

<!-- lesson-code: services/account/internal/repository/account/identity_queries.go :: Queryer -->
````go
// Queryer使只读查询可以明确使用原池或调用方事务；repository不会自行开启事务。
type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/identity_service.go :: MutationBody -->
````go
// MutationBody使用同一私有库事务，不能另开池。成员离开等明确需要跨服务互斥的
// 用例仍在来源锁内调用有deadline的内部检查；调用本身不能代替本地持久关闭事实。
type MutationBody func(context.Context, *sql.Tx, domain.Session) (map[string]any, error)
````
<!-- /lesson-code -->

`MutationBody(ctx,tx,Session) (map[string]any,error)` 必须在传入的同一个事务上操作；返回 map 由平台固定编码成回执，error 触发回滚。A01 的 `domain.Mutation` 携带 OperationID、Kind、Path、Fingerprint 与真实 Session；Kind 由具体用例设定，不能让请求正文伪造。

## 3. 先写 Acquire，再写来源行锁和 gate

在用例文件 Identity 构造器之后，输入 AcquireIdentityAdmission；完整实现依赖的 currentSources、lockIdentityGates 随后按操作页写入。

<!-- lesson-code: services/account/internal/logic/account/identity_service.go :: Identity.AcquireIdentityAdmission -->
````go
func (s *Identity) AcquireIdentityAdmission(ctx context.Context, req *pb.IdentityRequest) (*pb.IdentityAdmission, error) {
	if req == nil || !domain.ValidOperationID(req.OperationId) || len(req.SessionToken) != 32 || !domain.ValidIdentityPurpose(req.Purpose) || req.Target == nil || req.Target.Purpose != req.Purpose || req.IdentityTarget == nil {
		return nil, domain.ErrIdentityBinding
	}
	hash := sha256.Sum256(req.SessionToken)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, identityUnavailable()
	}
	defer tx.Rollback()
	// 先只定位actor，真正资格在固定锁顺序user→session→组织→member→gate中核验。
	var actor uint64
	if s.repository.SessionActor(ctx, tx, hash[:], &actor) != nil {
		return nil, identityDenied()
	}
	sources, err := s.admissionSources(ctx, tx, actor, hash[:], req.RequiredIdentityCapability, req.IdentityTarget, req.Purpose)
	if err != nil {
		return nil, err
	}
	if err = s.lockIdentityGates(ctx, tx, sources); err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(30 * time.Second)
	// 准入寿命取实际会话与协议上限的较小值，不能在会话自然到期后继续发出旧资格。
	var sessionExpiry time.Time
	if s.repository.SessionExpiry(ctx, tx, hash[:], &sessionExpiry) != nil {
		return nil, identityUnavailable()
	}
	if sessionExpiry.Before(expires) {
		expires = sessionExpiry
	}
	proof := &pb.IdentityAdmission{
		AdmissionId:    platform.NewTraceID(),
		ActorId:        actor,
		SessionRef:     hash[:],
		Sources:        sources,
		ExpiresUnixMs:  expires.UnixMilli(),
		Purpose:        req.Purpose,
		Target:         req.Target,
		IdentityTarget: req.IdentityTarget,
		Capability:     req.RequiredIdentityCapability,
		OperationId:    req.OperationId,
	}
	raw, _ := proto.Marshal(proof)
	binding, _ := json.Marshal(admissionBinding{Proof: raw})
	if err = s.repository.CreateAdmission(ctx, tx, proof.AdmissionId, actor, hash[:], req.OperationId, req.Purpose, string(binding), expires); err != nil {
		return nil, identityUnavailable()
	}
	for _, source := range sources {
		if err = s.repository.AttachSource(ctx, tx, proof.AdmissionId, source.ScopeKey, source.Epoch); err != nil {
			return nil, identityUnavailable()
		}
	}
	if tx.Commit() != nil {
		return nil, identityUnavailable()
	}
	return proof, nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/repository/account/identity_queries.go :: Identity.LockSession -->
````go
func (r *Identity) LockSession(ctx context.Context, tx *sql.Tx, session []byte, actor *uint64, state *string, credential, epoch *uint64, expires *time.Time) error {
	return tx.QueryRowContext(ctx, "SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE", session).Scan(actor, state, credential, epoch, expires)
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/identity_service.go :: Identity.currentSources -->
````go
func (s *Identity) currentSources(ctx context.Context, tx *sql.Tx, actor uint64, session []byte, capability pb.IdentityCapability, target *pb.IdentityTarget) ([]*pb.SourceEpoch, error) {
	var state string
	var credential, identity uint64
	if s.repository.LockUser(ctx, tx, actor, &state, &credential, &identity) != nil || state != "ACTIVE" {
		return nil, identityDenied()
	}
	var sessionActor, sessionCredential, sessionEpoch uint64
	var expiry time.Time
	if s.repository.LockSession(ctx, tx, session, &sessionActor, &state, &sessionCredential, &sessionEpoch, &expiry) != nil || sessionActor != actor || state != "ACTIVE" || sessionCredential != credential || !expiry.After(time.Now()) {
		return nil, identityDenied()
	}
	user := strconv.FormatUint(actor, 10)
	sources := []*pb.SourceEpoch{{ScopeKey: "identity:" + user, Epoch: identity}, {ScopeKey: "credential:" + user, Epoch: credential}, {ScopeKey: "session:" + hex.EncodeToString(session), Epoch: sessionEpoch}}
	if capability == pb.IdentityCapability_ACTIVE_ACCOUNT || capability == pb.IdentityCapability_PERSONAL_SPACE_OWNER {
		if target.Kind != pb.IdentityTarget_USER || target.UserId == 0 || target.OrganizationId != 0 || target.SpaceId != 0 || (capability == pb.IdentityCapability_PERSONAL_SPACE_OWNER && target.UserId != actor) {
			return nil, identityDenied()
		}
		// 文档管理者的个人协作授权需要核验实际目标账号；这里只证明活跃资格，
		// 文档管理权限仍由文档服务负责。证明携带目标身份来源，关闭账号会使它失效。
		if target.UserId != actor {
			var targetEpoch uint64
			if s.repository.LockTargetIdentity(ctx, tx, target.UserId, &state, &targetEpoch) != nil || state != "ACTIVE" {
				return nil, identityDenied()
			}
			sources = append(sources, &pb.SourceEpoch{ScopeKey: "identity:" + strconv.FormatUint(target.UserId, 10), Epoch: targetEpoch})
		}
		return sources, nil
	}
	if target.Kind != pb.IdentityTarget_ORGANIZATION || target.OrganizationId == 0 || target.SpaceId != 0 {
		return nil, identityDenied()
	}
	var targetIdentityEpoch uint64
	if target.UserId != 0 && target.UserId != actor {
		if capability != pb.IdentityCapability_ORGANIZATION_ADMIN && capability != pb.IdentityCapability_ORGANIZATION_OWNER && capability != pb.IdentityCapability_ORGANIZATION_MEMBER {
			return nil, identityDenied()
		}
		// 目标user先于组织锁，与注销固定组织集合使用同一顺序，
		// 避免目标注销持user等组织、目标证明持组织等user的反向等待。
		if s.repository.LockTargetIdentity(ctx, tx, target.UserId, &state, &targetIdentityEpoch) != nil || state != "ACTIVE" {
			return nil, identityDenied()
		}
	}
	org := strconv.FormatUint(target.OrganizationId, 10)
	var owner uint64
	if s.repository.LockOrganization(ctx, tx, target.OrganizationId, &owner, &state) != nil || state != "ACTIVE" {
		return nil, identityDenied()
	}
	var role string
	var memberEpoch uint64
	if s.repository.LockMember(ctx, tx, target.OrganizationId, actor, &role, &state, &memberEpoch) != nil || state != "ACTIVE" {
		return nil, identityDenied()
	}
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-member:" + org + ":" + user, Epoch: memberEpoch})
	switch capability {
	case pb.IdentityCapability_ORGANIZATION_MEMBER:
	case pb.IdentityCapability_ORGANIZATION_ADMIN:
		if owner != actor && role != "ADMIN" {
			return nil, identityDenied()
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-admin:" + org + ":" + user, Epoch: memberEpoch})
	case pb.IdentityCapability_ORGANIZATION_OWNER:
		if owner != actor {
			return nil, identityDenied()
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-owner:" + org + ":" + user, Epoch: memberEpoch})
	case pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE:
		if owner == actor || role == "ADMIN" {
			sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-admin:" + org + ":" + user, Epoch: memberEpoch})
		} else {
			var grantState string
			var revision uint64
			if s.repository.LockCreationGrant(ctx, tx, target.OrganizationId, actor, &grantState, &revision) != nil || grantState != "ACTIVE" {
				return nil, identityDenied()
			}
			sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-create:" + org + ":" + user, Epoch: revision})
		}
	default:
		return nil, identityDenied()
	}
	// 任免目标也必须是当前成员；目标来源写入依赖，防止检查后被移除。
	if target.UserId != 0 && target.UserId != actor {
		var targetEpoch uint64
		if s.repository.LockTargetMember(ctx, tx, target.OrganizationId, target.UserId, &state, &targetEpoch) != nil || state != "ACTIVE" {
			return nil, identityDenied()
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-member:" + org + ":" + strconv.FormatUint(target.UserId, 10), Epoch: targetEpoch})
		// 目标必须活跃本身也是权限依赖；否则注销预留封住identity后，
		// 另一管理员仍可能凭旧目标证明把新的KB职责授给待注销账号。
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "identity:" + strconv.FormatUint(target.UserId, 10), Epoch: targetIdentityEpoch})
	}
	return sources, nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/identity_service.go :: Identity.lockIdentityGates -->
````go
func (s *Identity) lockIdentityGates(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch) error {
	sort.Slice(sources, func(i, j int) bool { return sources[i].ScopeKey < sources[j].ScopeKey })
	for _, source := range sources {
		if err := s.repository.EnsureGate(ctx, tx, source.ScopeKey, source.Epoch); err != nil {
			return identityUnavailable()
		}
		var epoch uint64
		var state string
		if s.repository.LockGate(ctx, tx, source.ScopeKey, &epoch, &state) != nil {
			return identityUnavailable()
		}
		if source.Epoch < epoch || (source.Epoch == epoch && state != "OPEN") {
			return identityDenied()
		}
		// 只有重新核验权威来源后才可导入更高代次；旧关闭水位留存，不靠TTL删除。
		if source.Epoch > epoch {
			if err := s.repository.AdvanceGate(ctx, tx, source.ScopeKey, source.Epoch); err != nil {
				return identityUnavailable()
			}
		}
	}
	return nil
}
````
<!-- /lesson-code -->

用用户 17、credential_epoch=4、identity_epoch=2、session epoch=3，申请组织 9 的管理员准入推演：

1. 先校验 OperationId、token 字节数、用途和目标完整性。Target.Purpose 不同直接 `IDENTITY_BINDING_INVALID`，没有数据库写入。
2. SessionActor 只定位 actor，不授权。事务再按 user→session→目标user（需要时）→organization→member→gate 的顺序锁权威行。会话 user 不等于 actor、过期、状态非 ACTIVE 或 credential 代次不同都拒绝。
3. 至少生成 identity:17/2、credential:17/4、session:摘要/3；组织成员再加 org-member:9:17/成员epoch；ADMIN 或 owner 再加 org-admin 来源。普通成员没有管理员能力；建库资格检查独立 creation_grant 的 revision，目录上显示能建库不代替此步。
4. 若 Target.UserId 指向另一个被任免用户，还锁该用户和目标成员，并记录目标 identity/member 来源，防止目标注销或被移除后沿旧证明继续授权。
5. sources 按 ScopeKey 排序后逐个 EnsureGate/LockGate。旧 epoch 或同 epoch 但 BLOCKED 拒绝；只有已经核验真实来源更高 epoch 才可 AdvanceGate。关闭水位不能靠 TTL 抹去。
6. 到期取 now+30秒与实际 session 到期的较早者；创建 identity_admission 和 admission_source，同 tx Commit 成功才返回 proof。数据库故障/提交不明返回 `IDENTITY_UNAVAILABLE`，不会签发一个未确认入库的证明。

正常请求返回一份绑定组织9、本操作、本用途的短期证明。把同一 AdmissionId 移到组织10、改 Purpose 或目标用户，后面 revalidate 拒绝；角色已降级则当前来源不同，不能借旧证明恢复权限。

## 4. 写重验、协作者姓名与续期

<!-- lesson-code: services/account/internal/logic/account/identity_service.go :: Identity.revalidateIdentityAdmission -->
````go
func (s *Identity) revalidateIdentityAdmission(ctx context.Context, req *pb.IdentityRevalidation, read func(context.Context, *sql.Tx, *pb.IdentityAdmission) error) (*pb.IdentityAdmission, error) {
	if req == nil || !domain.ValidOperationID(req.AdmissionId) || req.Target == nil || req.IdentityTarget == nil {
		return nil, domain.ErrIdentityBinding
	}
	var raw []byte
	var state string
	var expires time.Time
	if s.repository.Admission(ctx, s.db, req.AdmissionId, &raw, &state, &expires) != nil {
		return nil, identityDenied()
	}
	var binding admissionBinding
	proof := &pb.IdentityAdmission{}
	if json.Unmarshal(raw, &binding) != nil || proto.Unmarshal(binding.Proof, proof) != nil || state != "OPEN" || !expires.After(time.Now()) || proof.OperationId != req.OperationId || proof.Purpose != req.Purpose || !proto.Equal(proof.Target, req.Target) || !proto.Equal(proof.IdentityTarget, req.IdentityTarget) {
		return nil, identityDenied()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, identityUnavailable()
	}
	defer tx.Rollback()
	current, err := s.admissionSources(ctx, tx, proof.ActorId, proof.SessionRef, proof.Capability, proof.IdentityTarget, proof.Purpose)
	if err != nil {
		return nil, err
	}
	if !sameSources(proof.Sources, current) {
		return nil, identityDenied()
	}
	if err = s.lockIdentityGates(ctx, tx, current); err != nil {
		return nil, err
	}
	// 关闭事务也先锁相同权威行；此处锁admission防止查询后被并发关闭。
	if s.repository.LockAdmission(ctx, tx, req.AdmissionId, &state, &expires) != nil || state != "OPEN" || !expires.After(time.Now()) {
		return nil, identityDenied()
	}
	if read != nil {
		if err = read(ctx, tx, proof); err != nil {
			return nil, err
		}
	}
	if tx.Commit() != nil {
		return nil, identityUnavailable()
	}
	return proof, nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/identity_service.go :: Identity.DocumentCollaborator -->
````go
// DocumentCollaborator只解析Doc已选定的协作者、提及候选或申请人，不提供任意用户目录。
// 原证明绑定真实会话、文档、用途和目标；姓名与重验共用已锁定的账号事务，邮箱不参与查询。
func (s *Identity) DocumentCollaborator(ctx context.Context, req *pb.IdentityRevalidation) (dto.DocumentCollaborator, error) {
	var result dto.DocumentCollaborator
	// 明确限定这三个读取用途，不能让普通活跃账号证明变成任意账号姓名查询。
	if req == nil || (req.Purpose != "DOCUMENT_COLLABORATORS" && req.Purpose != "DOCUMENT_MENTION_CANDIDATES" && req.Purpose != "DOCUMENT_APPLICATION_NAMES") || req.Target == nil || req.Target.Purpose != req.Purpose || req.Target.DocumentId == 0 ||
		req.Target.UnitId != 0 || req.Target.BranchId != 0 || req.Target.StructureId != 0 || req.Target.WorkspaceEpoch != 0 ||
		req.IdentityTarget == nil || req.IdentityTarget.Kind != pb.IdentityTarget_USER || req.IdentityTarget.UserId == 0 || req.IdentityTarget.OrganizationId != 0 || req.IdentityTarget.SpaceId != 0 {
		return result, domain.ErrIdentityBinding
	}
	_, err := s.revalidateIdentityAdmission(ctx, req, func(ctx context.Context, tx *sql.Tx, proof *pb.IdentityAdmission) error {
		if proof.Capability != pb.IdentityCapability_ACTIVE_ACCOUNT {
			return identityDenied()
		}
		name, err := s.repository.TargetDisplayName(ctx, tx, proof.IdentityTarget.UserId)
		if err != nil || strings.TrimSpace(name) == "" {
			return identityUnavailable()
		}
		result = dto.DocumentCollaborator{UserId: strconv.FormatUint(proof.IdentityTarget.UserId, 10), DisplayName: name}
		return nil
	})
	return result, err
}
````
<!-- /lesson-code -->

重验先读持久 proof 核对 OPEN、未到期、原 OperationId/Purpose/两个目标，再开启事务复查同一来源，锁 gate，最后锁 admission 防并发关闭。Read 回调（有才做）与身份核验共享 tx；因此协作者姓名不会在身份已变更后被另一普通读取悄悄取出。

DocumentCollaborator 只接受 DOCUMENT_COLLABORATORS、DOCUMENT_MENTION_CANDIDATES 或 DOCUMENT_APPLICATION_NAMES，Target 必须是单个非零文档且其余 unit/branch/structure/workspace 字段为零，IdentityTarget 必须是非零 USER，proof.Capability 必须 ACTIVE_ACCOUNT。返回生成 `dto.DocumentCollaborator{UserId,DisplayName}`，不查邮箱、不提供任意用户目录。具体管理权限与候选选择仍由 Document 完成。

随后输入 RenewOperationAdmission。它仅允许源码列出的 SUBMISSION、MERGE、MAIN_CONTROL、PUBLICATION、PULL_MAIN、STRUCTURE_PREVIEW、DISCARD_PREVIEW、DISCARD_DRAFT、REVERSE、CONFLICT_RESOLUTION。旧 proof 过期可以进入续期判断，但必须仍 OPEN、原会话当前有效、sources 完全一致；新 AdmissionId 与新到期不能改变操作号和权限。注销/降权后续期拒绝，不能用新身份替旧操作补证。

## 5. 写幂等 Mutation 与 HTTP 绑定

先完成仓储 Mutation 的 LockUser、LockSession、Receipt、InsertReceipt、InsertAudit，再完成用例 CheckSession/Run；所有 SQL 都使用 tx。

<!-- lesson-code: services/account/internal/logic/account/identity_service.go :: Mutations.Run -->
````go
func (s *Mutations) Run(ctx context.Context, input domain.Mutation, body MutationBody) (json.RawMessage, error) {
	if !domain.ValidOperationID(input.OperationID) {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "OPERATION_ID_INVALID"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// 原user→session→receipt锁顺序和提交边界不变；同号重试也核验当前会话。
	if err := s.CheckSession(ctx, tx, input.Session); err != nil {
		return nil, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, input.OperationID)
	if err == nil {
		if receipt.ActorID != input.Session.UserID || receipt.Kind != input.Kind || !bytes.Equal(receipt.Fingerprint, input.Fingerprint) {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "OPERATION_ID_CONFLICT"}
		}
		if len(receipt.Result) == 0 {
			return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "OPERATION_PENDING"}
		}
		return receipt.Result, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	value, err := body(ctx, tx, input.Session)
	if err != nil {
		return nil, err
	}
	result, err := platform.EncodeOperationReceipt(input.OperationID, value)
	if err != nil {
		return nil, err
	}
	target, err := json.Marshal(map[string]string{"path": input.Path})
	if err != nil {
		return nil, err
	}
	if s.repository.InsertReceipt(ctx, tx, input, result.State, result.JSON, target) != nil {
		return nil, errors.New("OPERATION_COMMIT_UNKNOWN")
	}
	if err := s.repository.InsertAudit(ctx, tx, input, target); err != nil {
		return nil, err
	}
	if tx.Commit() != nil {
		return nil, errors.New("OPERATION_COMMIT_UNKNOWN")
	}
	return result.JSON, nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/handler/account/identity_handlers.go :: MutationRequests.Bind -->
````go
func (h *MutationRequests) Bind(w http.ResponseWriter, r *http.Request, id string, input any) (domain.Mutation, bool) {
	if !domain.ValidOperationID(id) {
		httpx.Failure(w, 400, "OPERATION_ID_INVALID")
		return domain.Mutation{}, false
	}
	session, ok := h.authentication.Authenticate(w, r, true)
	if !ok {
		return domain.Mutation{}, false
	}
	// 路径和规范化后的输入仍使用原HMAC编码，已提交的幂等记录无需重写。
	fingerprint := h.fingerprint(struct {
		Path  string `json:"path"`
		Input any    `json:"input"`
	}{r.URL.Path, input})
	return domain.Mutation{
		OperationID: id,
		Path:        r.URL.Path,
		Fingerprint: fingerprint,
		Session:     session,
	}, true
}
````
<!-- /lesson-code -->

例如用户17创建组织，OperationID=`org-create-00001`：handler 先 Origin/CSRF/当前会话，Fingerprint 对 `{path,input}` 做 HMAC，所以同一 ID 改 URL 或正文也不能复用。Run 按 user→session→receipt 锁顺序验证当前身份。同号同 actor/kind/fingerprint 有结果便返回原 JSON；不符则 409。首次执行 body，编码稳定回执、写 operation_receipt 与 audit，Commit 后返回；提交错误 `OPERATION_COMMIT_UNKNOWN` 保留原号。这个 Run 的重复请求仍要求当前有效会话；原失效会话仅恢复特定关闭操作的机制在 A04/A07，不是通用免认证入口。

最后完成 Cursors.Decode/Encode 和 Pagination.Page/Next。游标负载含 Binding、Last、Expires，HMAC 签名且 5 分钟过期；绑定值由组织/操作者/目录修订等组成，用户切换不能继续使用另一列表的游标。limit 默认20、有效1–100；limit=0 返回 PAGE_LIMIT_INVALID，篡改或错绑定返回 CURSOR_INVALID。

## 6. 接线：内部 HTTP 与固定 RPC 各自验服务身份

`/infra/identity/renew-operation` → IdentityRenewal.Renew → RenewOperationAdmission；`/infra/identity/document-collaborator` → DocumentCollaborator。handler 只接 `platform.HTTPCaller(r,"document")` 验证后的证书身份，读最多8192字节 protojson；浏览器 Cookie 或自报角色不能代替服务证书。错误分为 400绑定、403非当前资格、503不可核验。

`/infra/identity/verify-session` 在 common_handlers.go，由 Document 证书调用，返回 actor_id 与真实 session_expires_unix_ms；mutation 由内部头声明后仍核 Origin/CSRF。RPC 在 A08 的 server.Identity 注册 AcquireIdentityAdmission/RevalidateIdentityAdmission；svc.NewIdentity 注入同一 Store.DB，证明查询不复制私有账号库。

identity_service.go 开头的 AuthorizationResults 及仓储事实读取也要按操作页输入。它们反向证明原授权关闭，不能调用 Document 再关闭一次；A04 用原任务/ACK 的输入推演详细讲解这组声明。

## 7. 可判定的检查与后续验收

等 A08 包齐后，下面是离线用例/handler 检查，不需要四业务服务：

```powershell
Set-Location -LiteralPath (Join-Path $lessonRoot 'services/account')
go test ./internal/logic/account -run '^TestDocumentCollaboratorNameUsesCurrentOriginalIdentity$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
go test ./internal/handler/account -run '^TestCollaboratorNameRequiresDocumentControlCertificate$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

预期全部具体测试 RUN/PASS：原证明同会话/同文档/同用途可以得到姓名；换目标、撤权、错误能力或失效来源不能；无证书、错服务证书不能查询。测试名真实存在但本次文档编写未执行这些测试。`no tests to run` 检查是否00支持测试已取齐、目录是否在 account 模块；接口缺声明按A03/A04/A08写完包，不加空返回骗过编译。

06真实验收：Document 动作申请并提交准入，在成员降权/移除后原证明拒绝，长期操作续期保持原绑定；检查两个私有库各自持久记录。自己的完整实例READY、开发SMTP可创建夹具后，在学习工程根目录运行：

```powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/account/identity.test.mjs
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

精确测试名是 `identity admission cannot outlive the actual session expiry` 和 `real identity admission binds session, operation, target and purpose then closes on logout`。预期都PASS、skip0，证明实际会话到期上限、原绑定与退出后的旧准入拒绝。403 先分辨 mTLS caller 不符还是 IDENTITY_NOT_CURRENT；后者应查来源代次和 gate，不能延长 TTL 绕过。只跑上述替身测试不代表真实 RPC/mTLS/事务竞态通过。

## 8. 章末答案与原因题

核对[仓储](../../source/050-identity-queries.md)、[用例](../../source/061-identity-service.md)、[身份HTTP](../../source/072-identity-handlers.md)、[分页与内部会话](../../source/071-common-handlers.md)。

为什么先锁目标 user 再锁组织？目标注销持 user 后锁固定组织集合，同顺序避免一方持组织等 user、另一方持 user 等组织。为什么证明字段完全相等还要读当前来源？字段只能证明过去核验过；撤权使来源 epoch/state 改变，事务内重查才能拒绝旧资格。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
Set-Location -LiteralPath $lessonRoot
$paths = @(
 'services/account/internal/repository/account/identity_queries.go',
 'services/account/internal/logic/account/identity_service.go',
 'services/account/internal/handler/account/identity_handlers.go',
 'services/account/internal/handler/account/common_handlers.go'
)
foreach ($path in $paths) {
 New-Item -ItemType Directory -Force (Split-Path $path) | Out-Null
 if (-not (Test-Path -LiteralPath $path)) { New-Item -ItemType File $path | Out-Null }
}
```
