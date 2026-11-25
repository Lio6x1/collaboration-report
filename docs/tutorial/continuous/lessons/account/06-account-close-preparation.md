# A06｜注销前固定本人身份和组织集合，职责未交接就停在准备期

## 这节课要实现什么

用户想注销账号，但他可能仍是组织所有者或唯一管理员。本课实现注销前的检查和准备，先解决职责交接，再暂停旧身份继续产生业务。

## 做完后能看到什么

接通后，未交接职责会阻止注销继续；准备成功会固定这次处理范围并推进关闭旧资格。账号和文档的最终清理要等A07及文档服务配合，不能在准备成功时显示已注销。

## 先想清楚怎样实现

1. 先核本人会话与密码。
2. 再记录本次负责的组织和下游交接预留。
3. 网络调用返回后重新检查事实，最后冻结原范围并逐项等待停止输出。

## 怎样判断这一课完成

A08后运行准备规则；06用有未交接职责和已完成交接两种账号验证，并检查同号重试仍对应原注销请求。

## 开始写之前

用户注销不能只 UPDATE user=CLOSED。还有组织owner、唯一管理员、知识库管理职责及正在输出的数据。本课先完成准备→下游预留→重新核对→冻结→逐来源排空；A07 才清理固定私有载荷并去标识化。两课共用同一原操作号，不新建一份“重试注销”。

功能讲义与分步页讲的是同一份代码，同名声明只输入一次。先理解本课功能推演，再沿分步页的完整顺序录入；章末全文只作核对。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

按[模型操作页](../../write/033-account-close-preparation-models.md)→[SQL操作页](../../write/047-account-close-preparation-queries.md)→[用例操作页](../../write/058-account-close-preparation-service.md)→[HTTP操作页](../../write/069-account-close-preparation-handlers.md)输入。每页先 package/import，逐声明建立完整文件。正文展示的是同一声明的课堂讲解与插入位置，已经按操作页写过该声明时只核对，不能追加第二份。A07 补 QueueAccountCloseRecovery、Resolve/Progress 与取消/最终恢复能力后包才齐。

## 2. 先定义 Capture 和消费者接口

<!-- lesson-code: services/account/internal/domain/account/account_close_preparation_models.go :: AccountCloseCapture -->
````go
// AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。
// Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。
type AccountCloseCapture struct {
	SessionHash     string                          `json:"session_hash"`
	CredentialEpoch uint64                          `json:"credential_epoch"`
	IdentityEpoch   uint64                          `json:"identity_epoch"`
	Binding         *platform.AccountClosureBinding `json:"binding"`
	Organizations   []AccountCloseOrganization      `json:"organizations,omitempty"`
	Scopes          []AccountCloseScope             `json:"scopes,omitempty"`
	Retention       *AccountCloseRetention          `json:"retention,omitempty"`
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/account_close_preparation_models.go :: AccountCloseOrganization -->
````go
// MemberRevision是组织成员集合水位，MembershipRevision是本人那一行的水位，
// 两者都固定才能在关闭前发现其他成员加入与本人角色变化。
type AccountCloseOrganization struct {
	ID                      uint64 `json:"id"`
	Owner                   uint64 `json:"owner"`
	Revision                uint64 `json:"revision"`
	MemberRevision          uint64 `json:"member_revision"`
	MembershipRevision      uint64 `json:"membership_revision"`
	Epoch                   uint64 `json:"epoch"`
	Role                    string `json:"role"`
	MemberState             string `json:"member_state"`
	Lifecycle               string `json:"lifecycle"`
	LiveMembers             int    `json:"live_members"`
	AvailableAdministrators int    `json:"available_administrators"`
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/account_close_preparation_models.go :: AccountCloseFenceTask -->
````go
type AccountCloseFenceTask struct {
	ID, Generation                         uint64
	Operation, Revocation, State, Instance string
	Payload                                []byte
	LeaseValid                             bool
}
````
<!-- /lesson-code -->

Capture.SessionHash 只存原会话摘要，CredentialEpoch/IdentityEpoch 固定原凭据与本人身份；原密码不入计划，HMAC指纹在receipt。Binding 为 nil 表示尚未固定下游范围，不能用空组织集合冒充完成规划。Organizations 存固定组织图；Scopes 存每个原关闭来源和子撤权号；Retention 的PlanDigest/AfterDocument/DocCompleted在A07记录清理进度。

Organization.Revision/MemberRevision 是组织实体和集合水位；MembershipRevision 是本人那一行的修订；Epoch 是本人成员来源代次。LiveMembers 含 ACTIVE/REVOKING，AvailableAdministrators 只数当前ACTIVE管理员/owner。两种数量回答不同问题，不能把撤权中管理员当作可交接人。

FenceTask.ID/Generation/Instance/LeaseValid 是任务执行权，Operation/Revocation/Payload 是原业务绑定。领取generation增加不能改变协议generation=1；否则恢复会成为另一撤权请求。

<!-- lesson-code: services/account/internal/logic/account/account_close_preparation_service.go :: AccountClosurePreparer -->
````go
type AccountClosurePreparer interface {
	Prepare(context.Context, platform.AccountClosureBinding) (platform.AccountClosurePreparation, error)
	Cancel(context.Context, platform.AccountClosureBinding) error
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/account_close_preparation_service.go :: AccountCloseFencePeer -->
````go
type AccountCloseFencePeer interface {
	FenceAccess(context.Context, *pb.FenceRequest) (*pb.FenceResult, error)
}
````
<!-- /lesson-code -->

Preparer.Prepare(ctx,Binding) 返回 RequiredKnowledgeBases/ReservedKnowledgeBases 及状态，Cancel(ctx,Binding) 只接受同一权威拒绝范围。FencePeer.FenceAccess(ctx,*pb.FenceRequest) 返回生成 FenceResult/error；必须核ACK绑定，nil error 不足以证明成功。数据库依赖是已有 *repository.AccountClosures，不虚构另一接口。

## 3. 写本地 Start：先续原回执，再锁外验密码、锁内复核

先写仓储 Hash/LockUser/LockSession/Receipt/Record/HasPending/Prepare，然后在用例写 AccountClosures、构造器、Start、beginClose、begin。

<!-- lesson-code: services/account/internal/logic/account/account_close_preparation_service.go :: AccountClosures.beginClose -->
````go
func (s *AccountClosures) beginClose(ctx context.Context, session domain.Session, input dto.AccountClose, fingerprint []byte, automatic bool) (dto.AccountClosePlan, error) {
	user, err := strconv.ParseUint(session.UserID, 10, 64)
	epoch, valid := domain.CredentialEpoch(input.ExpectedIdentityEpoch)
	if err != nil || user == 0 || strconv.FormatUint(user, 10) != session.UserID || !valid || !domain.ValidOperationID(input.OperationId) || len(session.ID) != 32 || len(fingerprint) != 32 {
		return dto.AccountClosePlan{}, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "ACCOUNT_CLOSE_INPUT_INVALID"}
	}
	// 原会话只允许读取完全相同的原操作。先查回执再验密码，身份进入REVOKING后
	// 仍能恢复原请求；另一个操作编号必须重新通过当前凭据与会话资格检查。
	plan, found, err := s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, nil, automatic)
	if err != nil || found {
		return plan, err
	}
	hash, err := s.repository.Hash(ctx, user)
	if err != nil {
		return plan, err
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(input.CurrentPassword)) != nil {
		return plan, domain.RuleViolation{Kind: domain.PermissionDenied, Code: "CURRENT_PASSWORD_INVALID"}
	}
	// bcrypt在事务外运行；取得user锁之后比较原hash及两个代次，防止旧密码验证穿越凭据变更。
	plan, _, err = s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, hash, automatic)
	return plan, err
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/account_close_preparation_service.go :: AccountClosures.begin -->
````go
func (s *AccountClosures) begin(ctx context.Context, user, epoch uint64, session domain.Session, operation string, fingerprint, verifiedHash []byte, automatic bool) (dto.AccountClosePlan, bool, error) {
	var plan dto.AccountClosePlan
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return plan, false, err
	}
	defer tx.Rollback()
	credential, err := s.repository.LockUser(ctx, tx, user)
	if err != nil {
		return plan, false, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, operation)
	if err == nil {
		if receipt.User != user || receipt.Kind != "ACCOUNT_CLOSE" || receipt.SessionHash != hex.EncodeToString(session.ID) || !bytes.Equal(receipt.Fingerprint, fingerprint) {
			return plan, true, accountCloseConflict("OPERATION_ID_CONFLICT")
		}
		record, err := s.repository.Record(ctx, tx, operation)
		if err != nil {
			return plan, true, err
		}
		capture, err := domain.DecodeAccountCloseCapture(record.Capture)
		if err != nil {
			return plan, true, err
		}
		if record.User != user || capture.SessionHash != receipt.SessionHash || capture.IdentityEpoch != epoch || strconv.FormatUint(capture.CredentialEpoch, 10) != session.CredentialEpoch {
			return plan, true, accountCloseConflict("OPERATION_ID_CONFLICT")
		}
		if capture.Binding != nil && (capture.Binding.User != user || capture.Binding.Operation != operation) {
			return plan, true, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
		}
		plan, err = accountCloseSavedPlan(operation, record, receipt)
		if err == nil && automatic {
			if err = repository.QueueAccountCloseRecovery(ctx, tx, operation, user); err == nil {
				err = tx.Commit()
			}
		}
		return plan, true, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return plan, false, err
	}
	if verifiedHash == nil {
		return plan, false, nil
	}
	if credential.State != "ACTIVE" || credential.IdentityEpoch != epoch || credential.Epoch != session.CredentialEpoch || !bytes.Equal(credential.Hash, verifiedHash) {
		return plan, false, accountCloseConflict("ACCOUNT_CLOSE_IDENTITY_CHANGED")
	}
	current, err := s.repository.LockSession(ctx, tx, session.ID)
	if err != nil {
		return plan, false, err
	}
	if current.UserID != session.UserID || current.State != "ACTIVE" || current.CredentialEpoch != credential.Epoch || current.Epoch != session.Epoch || !current.Expires.After(time.Now()) {
		return plan, false, domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	pending, err := s.repository.HasPending(ctx, tx, user)
	if err != nil {
		return plan, false, err
	}
	if pending {
		return plan, false, accountCloseConflict("ACCOUNT_CLOSE_IN_PROGRESS")
	}
	credentialEpoch, valid := domain.CredentialEpoch(credential.Epoch)
	if !valid {
		return plan, false, accountCloseConflict("ACCOUNT_CLOSE_CREDENTIAL_INVALID")
	}
	capture := domain.AccountCloseCapture{SessionHash: hex.EncodeToString(session.ID), CredentialEpoch: credentialEpoch, IdentityEpoch: epoch}
	plan = dto.AccountClosePlan{OperationId: operation, State: "PREPARING", RetainedReleaseIds: []string{}, RequiredSuccessions: []struct {
		Kind     string `json:"kind"`
		TargetId string `json:"target_id"`
	}{}}
	rawCapture, err := json.Marshal(capture)
	if err != nil {
		return plan, false, err
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return plan, false, err
	}
	target, err := json.Marshal(map[string]string{"user_id": session.UserID, "session_hash": capture.SessionHash})
	if err != nil {
		return plan, false, err
	}
	if err = s.repository.Prepare(ctx, tx, user, operation, rawCapture, target, fingerprint, rawPlan); err != nil {
		return plan, false, err
	}
	if automatic {
		if err = repository.QueueAccountCloseRecovery(ctx, tx, operation, user); err != nil {
			return plan, false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return plan, false, errors.New("ACCOUNT_CLOSE_COMMIT_UNKNOWN")
	}
	return plan, true, nil
}
````
<!-- /lesson-code -->

正常输入：user17、真实session、expected_identity_epoch=`2`、正确current_password、operation_id=`account-close-0001`、32字节指纹。begin 第一次持 user 锁检查是否已有同号receipt；同号必须 actor/kind/sessionhash/fingerprint/原两个epoch全部相符，再返回原plan（Start还补齐自动恢复任务）。不存在且尚未验密码则回退到锁外读取hash与bcrypt比较；第二次 begin 持user锁重核ACTIVE、identity2、原credential/hash，再锁原session检查actor、epoch、ACTIVE和到期。

同用户另一注销未完成返回 ACCOUNT_CLOSE_IN_PROGRESS。第一次登记仅把原Capture、PREPARING receipt/account_close 与自动ACCOUNT_CLOSE_RECOVER任务同tx写下，Commit后回202；此时本人未冻结，可继续交接职责。Begin 是旧控制协议准备入口，Start 才是用户完整用例并带自动恢复意图，不能把调用Begin说成用户已走完整注销。

错误密码是 CURRENT_PASSWORD_INVALID；锁外验完密码后另一改密已发生，锁内必须 ACCOUNT_CLOSE_IDENTITY_CHANGED。提交不明为 ACCOUNT_CLOSE_COMMIT_UNKNOWN，浏览器保留原号重试，不能用新号跳过既有未知任务。

## 4. 写固定组织图和下游预留：网络前后各有事务

<!-- lesson-code: services/account/internal/repository/account/account_close_preparation_queries.go :: AccountClosures.LockOrganizations -->
````go
// LockOrganizations由已持有user锁的调用方使用。发现阶段不锁成员行，随后逐个
// 按组织编号取组织锁，避免两个成员分别持有自身成员行后再交叉等待组织锁。
func (r *AccountClosures) LockOrganizations(ctx context.Context, tx *sql.Tx, user uint64) ([]domain.AccountCloseOrganization, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM organization WHERE lifecycle IN ('ACTIVE','CLOSING') AND
		(owner_id=? OR id IN (SELECT organization_id FROM organization_member WHERE user_id=? AND state IN ('ACTIVE','REVOKING')))
		ORDER BY id`, user, user)
	if err != nil {
		return nil, err
	}
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	organizations := []domain.AccountCloseOrganization{}
	for _, id := range ids {
		org := domain.AccountCloseOrganization{ID: id}
		err = tx.QueryRowContext(ctx, `SELECT owner_id,revision,member_revision,lifecycle FROM organization WHERE id=? FOR UPDATE`, id).
			Scan(&org.Owner, &org.Revision, &org.MemberRevision, &org.Lifecycle)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if org.Lifecycle != "ACTIVE" && org.Lifecycle != "CLOSING" {
			continue
		}
		// 当前锁定读不受发现阶段RR快照影响，等待组织锁后必须看到刚提交的角色与成员状态。
		rows, err = tx.QueryContext(ctx, `SELECT user_id,role,state,epoch,revision FROM organization_member WHERE organization_id=? ORDER BY user_id FOR UPDATE`, id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var member, epoch, revision uint64
			var role, state string
			if err = rows.Scan(&member, &role, &state, &epoch, &revision); err != nil {
				rows.Close()
				return nil, err
			}
			if state == "ACTIVE" || state == "REVOKING" {
				org.LiveMembers++
			}
			if state == "ACTIVE" && (role == "ADMIN" || member == org.Owner) {
				org.AvailableAdministrators++
			}
			if member == user {
				org.Role, org.MemberState, org.Epoch, org.MembershipRevision = role, state, epoch, revision
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if org.Owner == user || org.MemberState == "ACTIVE" || org.MemberState == "REVOKING" {
			organizations = append(organizations, org)
		}
	}
	return organizations, nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/account_close_preparation_service.go :: AccountClosures.PrepareClose -->
````go
// PrepareClose先固定Account组织事实，再取得Doc预留，最后复核同一快照并原子冻结。
// 网络调用不持SQL锁；中间事实改变时只能拒绝原计划，不能静默替换关闭组织集合。
func (s *AccountClosures) PrepareClose(ctx context.Context, operation string, peer AccountClosurePreparer) (dto.AccountClosePlan, error) {
	plan, binding, err := s.prepareCloseStage(ctx, operation, nil)
	if err != nil || binding == nil {
		return plan, err
	}
	if peer == nil {
		return plan, errors.New("ACCOUNT_CLOSE_CONTROL_REQUIRED")
	}
	if plan.State == "REJECTED" {
		return plan, peer.Cancel(ctx, *binding)
	}
	prepared, err := peer.Prepare(ctx, *binding)
	if err != nil {
		// 回复未知仍保持同一PREPARING记录，重试原号向Doc取回已保存的预留。
		return plan, err
	}
	plan, binding, err = s.prepareCloseStage(ctx, operation, &prepared)
	if err == nil && plan.State == "REJECTED" && binding != nil {
		err = peer.Cancel(ctx, *binding)
	}
	return plan, err
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/account_close_preparation_service.go :: AccountClosures.prepareCloseStage -->
````go
func (s *AccountClosures) prepareCloseStage(ctx context.Context, operation string, prepared *platform.AccountClosurePreparation) (dto.AccountClosePlan, *platform.AccountClosureBinding, error) {
	var plan dto.AccountClosePlan
	user, err := s.repository.UserForOperation(ctx, operation)
	if err != nil {
		return plan, nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return plan, nil, err
	}
	defer tx.Rollback()
	credential, err := s.repository.LockUser(ctx, tx, user)
	if err != nil {
		return plan, nil, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, operation)
	if err != nil {
		return plan, nil, err
	}
	record, err := s.repository.Record(ctx, tx, operation)
	if err != nil {
		return plan, nil, err
	}
	plan, err = accountCloseSavedPlan(operation, record, receipt)
	if err != nil {
		return plan, nil, err
	}
	capture, err := domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return plan, nil, err
	}
	if record.User != user || receipt.SessionHash != capture.SessionHash || (capture.Binding != nil && (capture.Binding.User != user || capture.Binding.Operation != operation)) {
		return plan, nil, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
	}
	if record.State == "REJECTED" {
		return plan, capture.Binding, nil
	}
	if record.State != "PREPARING" {
		return plan, nil, nil
	}
	organizations, err := s.repository.LockOrganizations(ctx, tx, user)
	if err != nil {
		return plan, nil, err
	}
	identityMatches := credential.State == "ACTIVE" && credential.IdentityEpoch == capture.IdentityEpoch && credential.Epoch == strconv.FormatUint(capture.CredentialEpoch, 10)
	if !identityMatches || (capture.Binding != nil && !slices.Equal(capture.Organizations, organizations)) {
		plan.State = "REJECTED"
		raw, marshalErr := json.Marshal(plan)
		if marshalErr != nil {
			return plan, nil, marshalErr
		}
		if err = s.repository.Reject(ctx, tx, operation, "PREPARATION_CHANGED", raw); err != nil {
			return plan, nil, err
		}
		if err = tx.Commit(); err != nil {
			return plan, nil, errors.New("ACCOUNT_CLOSE_REJECTION_UNKNOWN")
		}
		return plan, capture.Binding, nil
	}
	plan.RequiredSuccessions = plan.RequiredSuccessions[:0]
	closing := []uint64{}
	for _, org := range organizations {
		if org.Lifecycle != "ACTIVE" || org.MemberState != "ACTIVE" {
			return plan, nil, accountCloseConflict("ACCOUNT_CLOSE_ORGANIZATION_PENDING")
		}
		if org.Owner == user {
			if org.LiveMembers == 1 {
				closing = append(closing, org.ID)
			} else {
				appendAccountCloseSuccession(&plan, "ORGANIZATION_OWNER", org.ID)
			}
		} else if org.Role == "ADMIN" && org.AvailableAdministrators <= 1 {
			appendAccountCloseSuccession(&plan, "ORGANIZATION_ADMIN", org.ID)
		}
	}
	if capture.Binding == nil && len(plan.RequiredSuccessions) == 0 {
		capture.Binding = &platform.AccountClosureBinding{Operation: operation, User: user, IdentityEpoch: capture.IdentityEpoch, ClosingOrganizations: closing}
		capture.Organizations = organizations
	}
	freeze := false
	if prepared != nil {
		if capture.Binding == nil || prepared.RequiredKnowledgeBases == nil || prepared.ReservedKnowledgeBases == nil {
			return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
		}
		switch prepared.State {
		case "HANDOFF_REQUIRED":
			if len(prepared.RequiredKnowledgeBases) == 0 || len(prepared.ReservedKnowledgeBases) != 0 {
				return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
			}
			for _, kb := range prepared.RequiredKnowledgeBases {
				if kb == 0 {
					return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
				}
				appendAccountCloseSuccession(&plan, "KNOWLEDGE_BASE_ADMIN", kb)
			}
		case "FROZEN":
			if len(plan.RequiredSuccessions) != 0 || len(prepared.RequiredKnowledgeBases) != 0 {
				return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
			}
			freeze = true
			plan.State = "FROZEN"
			capture.Scopes = accountCloseScopes(*capture.Binding, organizations)
		default:
			return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_CONFLICT")
		}
	}
	rawCapture, err := json.Marshal(capture)
	if err != nil {
		return plan, nil, err
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return plan, nil, err
	}
	if freeze {
		err = s.repository.Freeze(ctx, tx, *capture.Binding, capture, rawPlan)
	} else {
		err = s.repository.SavePreparation(ctx, tx, operation, rawCapture, rawPlan)
	}
	if err != nil {
		return plan, nil, err
	}
	if err = tx.Commit(); err != nil {
		return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_UNKNOWN")
	}
	return plan, capture.Binding, nil
}
````
<!-- /lesson-code -->

先写 LockOrganizations：发现相关组织ID后逐个按ID升序锁组织，再按user_id锁成员集合。等待锁后使用当前锁定读，不能拿发现时旧快照算角色。准备事务采用ReadCommitted，锁user→receipt/account_close→固定组织集合，核Capture的原身份/凭据代次。

例：17是组织9的owner且有3名存续成员，plan.RequiredSuccessions 增加 ORGANIZATION_OWNER/9；在组织10是唯一ACTIVE管理员则加 ORGANIZATION_ADMIN/10。本人是唯一存续成员的组织11加入 ClosingOrganizations。只要组织交接未完成，Binding仍不固定、plan保持PREPARING，用户先完成这些交接。

没有待交接项时固定 Binding(Operation,User,IdentityEpoch,ClosingOrganizations) 和完整Organizations。释放事务锁后调用Document Prepare；Document若要求知识库管理员交接，返回HANDOFF_REQUIRED且只准有RequiredKnowledgeBases、无Reserved；Account添加KNOWLEDGE_BASE_ADMIN条目。Document若回FROZEN，要求无任何待交接项，Account第二次事务锁回原身份与组织图，必须与固定Organizations完全相等；别人加入、角色/修订改变时拒绝原计划，不能悄悄换一组待删组织。

Prepare的网络回复丢失，保留同一PREPARING和Binding，重试原号取回下游已保存的预留。准备拒绝与ACCOUNT_CLOSE_CANCEL意图同事务落库，A07重试取消；未固定Binding的本地拒绝不用造一个虚构取消范围。FROZEN之后不能再通过准备拒绝释放下游预留，因为可能已有不可逆清理事实。

## 5. 写冻结与排空：每个来源有自己固定子号

<!-- lesson-code: services/account/internal/repository/account/account_close_preparation_queries.go :: AccountClosures.Freeze -->
````go
// Freeze必须由已复核凭据/组织快照、且已确认Doc同一固定计划FROZEN的用例调用。
// 全部来源冻结、通知下游的持久任务与状态推进使用同一事务；不在此宣称注销完成。
func (r *AccountClosures) Freeze(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture, result []byte) error {
	if err := validateAccountCloseFreeze(binding, capture); err != nil {
		return err
	}
	changed, err := tx.ExecContext(ctx, `UPDATE user SET state='REVOKING',identity_epoch=identity_epoch+1
		WHERE id=? AND state='ACTIVE' AND identity_epoch=? AND credential_epoch=? AND identity_epoch<18446744073709551615`, binding.User, binding.IdentityEpoch, capture.CredentialEpoch)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	if err = freezeAccountCloseSessions(ctx, tx, binding.User); err != nil {
		return err
	}
	closing := map[uint64]bool{}
	for _, id := range binding.ClosingOrganizations {
		closing[id] = true
	}
	for _, org := range capture.Organizations {
		if org.MemberState != "ACTIVE" {
			return errors.New("ACCOUNT_CLOSE_MEMBERSHIP_CHANGED")
		}
		changed, err = tx.ExecContext(ctx, `UPDATE organization_member SET state='REVOKING',epoch=epoch+1,revision=revision+1
			WHERE organization_id=? AND user_id=? AND role=? AND state='ACTIVE' AND epoch=? AND revision=?
			AND epoch<18446744073709551615 AND revision<18446744073709551615`, org.ID, binding.User, org.Role, org.Epoch, org.MembershipRevision)
		if err = accountCloseChanged(changed, err); err != nil {
			return err
		}
		if closing[org.ID] {
			changed, err = tx.ExecContext(ctx, `UPDATE organization SET lifecycle='CLOSING',revision=revision+1,member_revision=member_revision+1
				WHERE id=? AND owner_id=? AND lifecycle=? AND revision=? AND member_revision=?
				AND revision<18446744073709551615 AND member_revision<18446744073709551615`, org.ID, org.Owner, org.Lifecycle, org.Revision, org.MemberRevision)
		} else {
			changed, err = tx.ExecContext(ctx, `UPDATE organization SET member_revision=member_revision+1
				WHERE id=? AND owner_id=? AND lifecycle=? AND revision=? AND member_revision=? AND member_revision<18446744073709551615`, org.ID, org.Owner, org.Lifecycle, org.Revision, org.MemberRevision)
		}
		if err = accountCloseChanged(changed, err); err != nil {
			return err
		}
	}
	for _, scope := range capture.Scopes {
		if err = freezeAccountCloseScope(ctx, tx, binding, scope); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE account_close SET state='FROZEN',retention_plan=?,generation=generation+1,retry_at=UTC_TIMESTAMP(6)
		WHERE operation_id=? AND user_id=? AND state='PREPARING' AND generation<18446744073709551615`, string(raw), binding.Operation, binding.User)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1
		WHERE operation_id=? AND actor_id=? AND kind='ACCOUNT_CLOSE' AND state='PREPARING' AND generation<18446744073709551615`, string(result), binding.Operation, binding.User)
	return accountCloseChanged(changed, err)
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/account_close_preparation_service.go :: accountCloseScopes -->
````go
func accountCloseScopes(binding platform.AccountClosureBinding, organizations []domain.AccountCloseOrganization) []domain.AccountCloseScope {
	user := strconv.FormatUint(binding.User, 10)
	scopes := []domain.AccountCloseScope{{Revocation: binding.Operation, Scope: "identity:" + user, Epoch: binding.IdentityEpoch}}
	for _, org := range organizations {
		scope := "org-member:" + strconv.FormatUint(org.ID, 10) + ":" + user
		// 子撤权号固定且有界，不因原操作号接近最大长度而截断产生碰撞。
		digest := sha256.Sum256([]byte(binding.Operation + "\n" + scope))
		scopes = append(scopes, domain.AccountCloseScope{Revocation: "account-close-" + hex.EncodeToString(digest[:]), Scope: scope, Epoch: org.Epoch})
	}
	return scopes
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/account_close_preparation_service.go :: AccountCloseFences.RecoverOne -->
````go
func (s *AccountCloseFences) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	task, err := s.repository.Claim(ctx, tx, instance)
	if err != nil {
		_ = tx.Rollback()
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	payload, err := s.repository.Binding(ctx, s.db, task)
	if err != nil {
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_BINDING_INVALID")
		return
	}
	request := &pb.FenceRequest{RevocationId: payload.Revocation, Generation: 1,
		ClosedScopes: []*pb.SourceEpoch{{ScopeKey: payload.Scope, Epoch: payload.Epoch}},
		TargetDigest: platform.SourceClosureDigest(payload.Scope, payload.Epoch)}
	// 持久租约已提交后才调用真实Doc屏障。协议generation固定为1，不能把任务领取代次
	// 填入原屏障请求，否则恢复会悄悄变成另一份业务操作。
	rpcCtx, done := context.WithTimeout(ctx, 2*time.Second)
	ack, err := s.peer.FenceAccess(rpcCtx, request)
	done()
	if err != nil || !platform.MatchesBootstrapAck(request, ack, "document") {
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_NOT_CONFIRMED")
		return
	}
	finished, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_FINISH_UNKNOWN")
		return
	}
	if err = s.repository.Finish(ctx, finished, task, payload, ack.ResultDigest); err != nil {
		_ = finished.Rollback()
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_FINISH_UNKNOWN")
		return
	}
	if err = finished.Commit(); err != nil {
		// 条件重试不会改写已提交成功的任务；提交响应丢失仍按未知结果处理。
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_FINISH_UNKNOWN")
	}
}
````
<!-- /lesson-code -->

Freeze 同 tx 条件更新本人ACTIVE/原identity/credential，变REVOKING且identity_epoch+1；封旧sessions；将固定成员REVOKING且epoch/revision+1，关闭组织进入CLOSING，逐scope关闭gate/准入并写authorization_change和ACCOUNT_CLOSE_FENCE任务；最后account_close FROZEN、generation+1与receipt结果更新。每个条件UPDATE必须影响恰好一行，失败整事务回滚。

identity:17 使用父operation作撤权号，org-member:9:17 等子scope由 SHA256(operation+换行+scope) 生成有界固定号；原号再长也不能截断碰撞。Freeze不是物理删除，旧来源停止证明还没全部齐。

Fence worker先Claim/Commit持久租约→读原payload与capture严格核绑定→两秒期限向Document FenceAccess→核真实document BootstrapAck→新事务按user/receipt/record/task复核generation、instance、RUNNING、lease仍有效、payload相同→插ACK并核原digest→authorization SUCCEEDED、task SUCCEEDED。网络未知、ACK不同或提交未知都重试原号，任务代次不能填入业务FenceRequest的Generation。

BindingForDocument 只交付持久原Binding，不能用当前identity99替原identity2补造；Rejected/Confirmed 查询分别是拒绝与已排空的权威证明，查无不等于拒绝，故障不等于取消成功。

## 6. 接真实入口与检查

`POST /v1/account/closures` → AccountClosures.Start handler → A07 Resolve选原会话/恢复秘密 → Origin/CSRF → Start用例。准备期保留主会话方便交接，另设24小时HttpOnly RecoveryCookie；冻结后仅恢复原操作。`GET /v1/account/closures/:operationId` 返回原plan。`/infra/lifecycle/account-close-binding` 仅document证书可取原范围；A08 RegisterAccountCloseDecisions/Closures登记。

离线两条测试在A08包齐后执行：

```powershell
Set-Location -LiteralPath (Join-Path $lessonRoot 'services/account')
go test ./internal/logic/account -run '^TestAccountLifecycleBindingUsesOnlyOriginalPersistentScope$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
go test ./internal/handler/account -run '^TestAccountLifecycleBindingRequiresExactDocumentControlIdentity$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

应RUN/PASS：当前epoch改变仍只返回原固定范围，未固定/被拒绝不给新范围；控制入口仅接受精确document证书身份。它们是替身/handler反例，未验证真实MySQL和mTLS，本次编写未执行。

06自己的四服务、开发SMTP、两私有库迁移齐备后才可运行已有真实准备协议检查（会创建隔离夹具，不对体验实例执行）：

```powershell
Set-Location -LiteralPath $lessonRoot
$env:COLLAB_TEST_ACCOUNT_CONFIG = Join-Path $PWD '.runtime/services/account.json'
$env:COLLAB_TEST_DOCUMENT_CONFIG = Join-Path $PWD '.runtime/services/document.json'
Set-Location services/account
go test ./internal/logic/account -run '^TestActualAccountCloseCaptureAndAuthoritativeCancellation$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

预期测试实际执行并PASS，覆盖固定组织图、下游预留回复丢失、权威拒绝取消等；它不能替代A07公开注销/载荷清理/浏览器验收。ACCOUNT_CLOSE_HANDOFF_REQUIRED 回到原plan交接；BINDING_CONFLICT核原capture而非当前图；FENCE_NOT_CONFIRMED核document/collaboration排空链，不能手插ACK。

## 7. 章末核对

[模型](../../source/033-account-close-preparation-models.md)、[SQL](../../source/047-account-close-preparation-queries.md)、[用例](../../source/058-account-close-preparation-service.md)、[HTTP](../../source/069-account-close-preparation-handlers.md)。

为什么Prepare前后不持同一事务等网络？网络可能长时间不可达，先固定计划并提交，再在回复后锁回相同事实核对，既保留原意图又不长期占SQL锁。为什么组织图变化不能更新原计划继续？下游已经为原范围预留，偷偷替换会把两个范围混成一次注销。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
Set-Location -LiteralPath $lessonRoot
$paths = @(
 'services/account/internal/domain/account/account_close_preparation_models.go',
 'services/account/internal/repository/account/account_close_preparation_queries.go',
 'services/account/internal/logic/account/account_close_preparation_service.go',
 'services/account/internal/handler/account/account_close_preparation_handlers.go'
)
foreach ($path in $paths) {
 New-Item -ItemType Directory -Force (Split-Path $path) | Out-Null
 if (-not (Test-Path -LiteralPath $path)) { New-Item -ItemType File $path | Out-Null }
}
```
