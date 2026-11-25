# A05｜向 Document 提供当前收件资格，并可靠转交账号通知

## 这节课要实现什么

组织所有者把职责交给另一位成员，对方应收到提醒；同一事件重试不应产生多条通知，离开组织的人也不应继续收到该组织的新提醒。本课实现账号服务一侧的收件资格与可靠投递。

## 做完后能看到什么

接通文档通知中心后，符合条件的接任者能看到一次提醒；投递暂时失败后仍可从持久事件继续处理。账号服务本课只提供资格与事件，不单独实现浏览器收件箱。

复用的通知中心每页显示10条提醒，下一页替换当前列表，上一页重新核验当前收件资格；本人标读保留真实持久结果，不能把本页未读数当成全部未读数。

## 先想清楚怎样实现

1. 先读取并锁定收件资格事实。
2. 再把组织变化对应的事件持久保存。
3. 后台读取待投递事件，得到对端确认后推进状态。

## 怎样判断这一课完成

A08后执行本课规则，06结合D09验证实际通知、重复投递和离组后的可见性；数据库中有事件不等于通知已送达。

## 开始写之前

用户转移组织所有者后，接任者应该在统一通知中心看到一条提醒；重复提交不能再来一条，退出组织后不能继续读组织提醒。本课写 Account 私有库的一侧：真实收件来源、管理员候选目录、待投递事件与确认。统一收件箱和浏览器呈现由 Document/前端接续，不能在 Account 用内存列表代替。

功能讲义与分步页讲的是同一份代码，同名声明只输入一次。先理解本课功能推演，再沿分步页的完整顺序录入；章末全文只作核对。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

按[管理员模型](../../write/039-notification-administrators.md)→[收件模型](../../write/040-notification-recipients.md)→[资格SQL](../../write/052-notifications-queries.md)→[事件SQL](../../write/051-notification-events.md)→[资格用例](../../write/063-notifications-service.md)→[事件用例](../../write/062-notification-events.md)→[HTTP](../../write/074-notifications-handlers.md)逐段输入。先有锁定的资格事实，才有“可以投递”的返回值。文件中 DepartureStatus 是组织离开未知提交的反查能力，也在本课讲清。

## 2. 先写收件模型和输入规则

<!-- lesson-code: services/account/internal/domain/account/notification_recipients.go :: NotificationRecipientIdentity -->
````go
type NotificationRecipientIdentity struct {
	ID, IdentityEpoch, CredentialEpoch uint64
	State                              string
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/notification_recipients.go :: NotificationRecipientMembership -->
````go
type NotificationRecipientMembership struct {
	Epoch       uint64
	Role, State string
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/notification_recipients.go :: NotificationRecipientIDs -->
````go
// 固定升序用户锁避免同一批次反向遍历；不默默去重，调用者必须核对一一对应结果。
func NotificationRecipientIDs(ids []uint64) ([]uint64, error) {
	if len(ids) == 0 || len(ids) > 100 {
		return nil, ErrNotificationRecipientsInput
	}
	ordered := append([]uint64(nil), ids...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for i, id := range ordered {
		if id == 0 || (i > 0 && id == ordered[i-1]) {
			return nil, ErrNotificationRecipientsInput
		}
	}
	return ordered, nil
}
````
<!-- /lesson-code -->

Identity.ID/IdentityEpoch/CredentialEpoch/State 来自 user；Organization.Owner/State 来自 organization；Membership.Epoch/Role/State 来自 organization_member。返回平台协议 Recipient 包含 UserID、Eligible、身份/凭据/成员代次、Role、OrgOwner、Sources，整个响应还有 OrganizationID 与 ExpiresUnixMS。这里没有 Session，不创建用户准入，后台通知不能伪造一次浏览器登录。

NotificationRecipientIDs 对1–100个非零、不重复ID复制后排序。输入 `[22,17]` 变 `[17,22]`，返回顺序也是按ID排序；调用方按 UserID 对应，不能按输入数组位置猜配。`[17,17]` 或空数组直接 NOTIFICATION_RECIPIENTS_INVALID，不默默去重隐藏调用方错误。

当前 NotificationRecipients 直接依赖 `*sql.DB` 与仓储函数，没有另立消费者接口；最小内部HTTP输入/输出使用平台既有类型，不再抄一份。管理员候选输入 OrganizationID/After，输出有 UserIDs/NextAfter，只是候选，不保证下一时刻仍有资格。

## 3. 写 SQL 行锁，再写五秒资格快照

先依次输入 NotificationRecipientUser/Org/Member；它们都显式接 `*sql.Tx` 并 FOR UPDATE。随后在逻辑文件写 Check 的完整实现。

<!-- lesson-code: services/account/internal/repository/account/notifications_queries.go :: NotificationRecipientUser -->
````go
// 所有资格事实来自同一事务权威行锁，SQL未知交给业务层报不可用，不能等同不存在。
func NotificationRecipientUser(ctx context.Context, tx *sql.Tx, id uint64) (domain.NotificationRecipientIdentity, error) {
	value := domain.NotificationRecipientIdentity{ID: id}
	err := tx.QueryRowContext(ctx, `SELECT state,identity_epoch,credential_epoch FROM user WHERE id=? FOR UPDATE`, id).Scan(&value.State, &value.IdentityEpoch, &value.CredentialEpoch)
	return value, err
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/notifications_service.go :: NotificationRecipients.Check -->
````go
// Check 不创建用户准入或伪造会话。Doc仅可在短期限内锁全部返回来源后提交投递。
// 当前角色变更关闭org-member，所有者转移关闭org-owner，密码变更关闭credential；
// 将来开放账号注销/组织关闭前，必须同样无条件广播identity/成员来源关闭高水位。
func (s *NotificationRecipients) Check(ctx context.Context, input platform.NotificationRecipientsRequest) (platform.NotificationRecipientsResponse, error) {
	result := platform.NotificationRecipientsResponse{OrganizationID: input.OrganizationID, Recipients: []platform.NotificationRecipient{}}
	ids, err := domain.NotificationRecipientIDs(input.UserIDs)
	if err != nil {
		return result, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	users := make([]domain.NotificationRecipientIdentity, len(ids))
	for i, id := range ids {
		users[i], err = repository.NotificationRecipientUser(ctx, tx, id)
		if err != nil && err != sql.ErrNoRows {
			return result, err
		}
	}
	var organization domain.NotificationRecipientOrganization
	if input.OrganizationID != 0 {
		organization, err = repository.NotificationRecipientOrg(ctx, tx, input.OrganizationID)
		if err != nil && err != sql.ErrNoRows {
			return result, err
		}
	}
	for _, user := range users {
		item := platform.NotificationRecipient{UserID: user.ID, IdentityEpoch: user.IdentityEpoch, CredentialEpoch: user.CredentialEpoch, Sources: []platform.NotificationRecipientSource{}}
		item.Eligible = user.State == "ACTIVE"
		if item.Eligible && (user.IdentityEpoch == 0 || user.CredentialEpoch == 0) {
			return result, errors.New("NOTIFICATION_IDENTITY_INCONSISTENT")
		}
		if input.OrganizationID != 0 {
			member, err := repository.NotificationRecipientMember(ctx, tx, input.OrganizationID, user.ID)
			if err != nil && err != sql.ErrNoRows {
				return result, err
			}
			item.Role, item.MemberEpoch = member.Role, member.Epoch
			item.OrgOwner = organization.Owner == user.ID
			item.Eligible = item.Eligible && organization.State == "ACTIVE" && member.State == "ACTIVE"
			if item.Eligible && (member.Epoch == 0 || (member.Role != "ADMIN" && member.Role != "MEMBER")) {
				return result, errors.New("NOTIFICATION_MEMBERSHIP_INCONSISTENT")
			}
		}
		if item.Eligible {
			userID := strconv.FormatUint(user.ID, 10)
			item.Sources = append(item.Sources,
				platform.NotificationRecipientSource{ScopeKey: "identity:" + userID, Epoch: item.IdentityEpoch},
				platform.NotificationRecipientSource{ScopeKey: "credential:" + userID, Epoch: item.CredentialEpoch})
			if input.OrganizationID != 0 {
				suffix := strconv.FormatUint(input.OrganizationID, 10) + ":" + userID
				item.Sources = append(item.Sources, platform.NotificationRecipientSource{ScopeKey: "org-member:" + suffix, Epoch: item.MemberEpoch})
				if item.Role == "ADMIN" {
					item.Sources = append(item.Sources, platform.NotificationRecipientSource{ScopeKey: "org-admin:" + suffix, Epoch: item.MemberEpoch})
				}
				if item.OrgOwner {
					item.Sources = append(item.Sources, platform.NotificationRecipientSource{ScopeKey: "org-owner:" + suffix, Epoch: item.MemberEpoch})
				}
			}
		}
		result.Recipients = append(result.Recipients, item)
	}
	// 在释放事实锁前确定期限，提交或传输耗时不能重新延长快照有效期。
	result.ExpiresUnixMS = time.Now().Add(5 * time.Second).UnixMilli()
	if err := tx.Commit(); err != nil {
		return platform.NotificationRecipientsResponse{}, err
	}
	return result, nil
}
````
<!-- /lesson-code -->

代入组织9、用户17与22：事务先升序锁两个 user，再锁组织9，随后逐个锁成员。用户17 ACTIVE、身份代次2、凭据4、成员epoch5、ADMIN，生成 identity:17/2、credential:17/4、org-member:9:17/5、org-admin:9:17/5；如还是 owner 加 org-owner。22不存在或已关闭会返回对应ID但 Eligible=false、空Sources；查无行是这个明确结果，SQL故障则 error，全批不签成功快照。

ACTIVE 却 epoch=0、非法成员role 等不一致事实必须报错，不能生成一个貌似有效的来源。期限在释放事实锁之前确定 now+5秒，Commit成功后返回；传输慢不能重新把期限延到“到达后五秒”。Document 收到后还要在期限内锁住全部来源才提交投递，不能把 Eligible 缓存成永久许可。改密码关闭 credential，改角色关闭 member，转owner关闭 owner，注销关闭identity；同一通知投递始终受这些来源约束。

Administrators 查询 DISTINCT 活跃 user/组织/成员且角色ADMIN或owner，user_id>After ORDER BY user_id，取101条、返回100条并给NextAfter；不能在100人边界静默漏掉管理员。不存在/关闭组织为空目录，数据库错误仍不可用。

## 4. 写事件生成、领取读取与确认

本课事件从 A07 `Closure.finish` 的所有者转移成功事务调用 CreateOwnerTransferNotice。先输入该方法、PendingNotificationEvents、verifyNotificationEvent、AcknowledgeNotificationEvent 和 Ready，随后输入 NotificationEvents 用例。

<!-- lesson-code: services/account/internal/repository/account/notification_events.go :: Closure.CreateOwnerTransferNotice -->
````go
// 成功回执、真实转移记录和通知使用调用方的同一事务，避免终态已可见但事件丢失。
func (r *Closure) CreateOwnerTransferNotice(ctx context.Context, tx *sql.Tx, operation, organization, actor string) error {
	var recipient uint64
	err := tx.QueryRowContext(ctx, `SELECT t.target_user_id FROM organization_transition t
		JOIN operation_receipt r ON r.operation_id=t.operation_id
		WHERE t.operation_id=? AND t.organization_id=? AND t.actor_id=?
		AND t.kind='TRANSFER_OWNER' AND t.state='SUCCEEDED'
		AND r.kind='ORG_OWNER_TRANSFER' AND r.state='SUCCEEDED' FOR UPDATE`, operation, organization, actor).Scan(&recipient)
	if err != nil {
		return err
	}
	// 所有操作号统一散列，避免短原号恰好等于另一长原号摘要时撞到同一事件编号。
	digest := sha256.Sum256([]byte(operation))
	eventID := "account-owner:" + hex.EncodeToString(digest[:])
	target, err := json.Marshal(map[string]string{"organization_id": organization})
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json)
		VALUES(?,?,'ORG_OWNER_TRANSFER',?) ON DUPLICATE KEY UPDATE id=id`, recipient, eventID, string(target)); err != nil {
		return err
	}
	var id uint64
	var original []byte
	if err = tx.QueryRowContext(ctx, `SELECT id,target_json FROM notification WHERE recipient_id=? AND event_id=? AND kind='ORG_OWNER_TRANSFER' FOR UPDATE`, recipient, eventID).Scan(&id, &original); err != nil {
		return err
	}
	var saved struct {
		OrganizationID string `json:"organization_id"`
	}
	if json.Unmarshal(original, &saved) != nil || saved.OrganizationID != organization {
		return ErrNotificationEventBinding
	}
	orgID, err := strconv.ParseUint(organization, 10, 64)
	if err != nil || orgID == 0 || recipient == 0 {
		return ErrNotificationEventBinding
	}
	event := platform.AccountNotificationEvent{ID: id, RecipientID: recipient, OrganizationID: orgID, EventID: eventID, Kind: "ORG_OWNER_TRANSFER"}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		VALUES('ACCOUNT_NOTICE_DELIVERY',?,?,?,'QUEUED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE id=id`, strconv.FormatUint(id, 10), operation, string(payload))
	return err
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/notification_events.go :: NotificationEvents.Acknowledge -->
````go
func (s *NotificationEvents) Acknowledge(ctx context.Context, input platform.AccountNotificationAckRequest) error {
	if len(input.IDs) == 0 || len(input.IDs) > 100 {
		return ErrNotificationEventsInput
	}
	ids := append([]uint64(nil), input.IDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		if id == 0 || (i > 0 && ids[i-1] == id) {
			return ErrNotificationEventsInput
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// 按稳定ID顺序加锁；任何不存在或不匹配的确认都回滚整批，不丢弃其他事件。
	for _, id := range ids {
		if err = repository.AcknowledgeNotificationEvent(ctx, tx, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, repository.ErrNotificationEventBinding) {
				return ErrNotificationEventsInput
			}
			return err
		}
	}
	return tx.Commit()
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/repository/account/notification_events.go :: AcknowledgeNotificationEvent -->
````go
func AcknowledgeNotificationEvent(ctx context.Context, tx *sql.Tx, id uint64) error {
	var event platform.AccountNotificationEvent
	var target, payload []byte
	var taskID uint64
	var state string
	// 重新锁定原始事件及对应任务，不接受调用者提供的收件人或事件内容替代绑定。
	err := tx.QueryRowContext(ctx, `SELECT n.id,n.recipient_id,n.event_id,n.kind,n.target_json,d.payload,d.id,d.state
		FROM notification n JOIN durable_task d ON d.business_key=CAST(n.id AS CHAR) AND d.kind='ACCOUNT_NOTICE_DELIVERY'
		WHERE n.id=? FOR UPDATE`, id).Scan(&event.ID, &event.RecipientID, &event.EventID, &event.Kind, &target, &payload, &taskID, &state)
	if err != nil {
		return err
	}
	if err = verifyNotificationEvent(&event, target, payload); err != nil {
		return err
	}
	if state != "QUEUED" && state != "SUCCEEDED" {
		return ErrNotificationEventBinding
	}
	_, err = tx.ExecContext(ctx, `UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL WHERE id=?`, taskID)
	return err
}
````
<!-- /lesson-code -->

原操作=`owner-transfer-0001`、组织9转给22：同 tx 先查成功 organization_transition 与成功 ORG_OWNER_TRANSFER receipt，目标接任人必须一致；Account event_id 固定 `account-owner:` + SHA256(原操作号)。插 notification 时 recipient+event 唯一，重试不造第二条；重读并核 organization_id，随后写 ACCOUNT_NOTICE_DELIVERY durable_task，payload保存通知ID/recipient/organization/event/kind。事件与成功转移同生共死，不能 Commit 后才临时发HTTP而丢通知。

PendingNotificationEvents 每次查 QUEUED 任务，按任务ID取最多100；不采用“已经见到最大ID”的水位，因为并发事务可能先分配小ID、后提交，单调水位会漏掉迟提交事件。verify 逐项核通知与payload完全匹配，坏绑定不可投递。

Document 可靠落入统一收件箱后才向 Account ACK IDs。Acknowledge 先拒0/重复/超过100，再升序锁每份通知和对应任务，重新核固定事件。QUEUED 可改 SUCCEEDED，已经SUCCEEDED可幂等确认；任意不存在或不匹配使整批回滚。提交回复丢失允许重发相同IDs，不能先删本地事件后等待远端确认。Account的冒号 event_id 是本库身份；Document导入统一通知有自己的映射，不能因浏览器侧连字符形式就修改源事件算法。

Ready 联查 QUEUED 通知是否源自成功真实转移，核actor/recipient/组织/payload，不允许以“只是队列任务”豁免坏绑定。Account不为这组事件创建发送worker；Document的账号通知导入worker轮询内部Events→可靠写入→ACK，A08只负责Account端路由与Ready。

## 5. 补 DepartureStatus：超时后查询真实准备结果

<!-- lesson-code: services/account/internal/logic/account/notifications_service.go :: Organizations.DepartureStatus -->
````go
// DepartureStatus不根据超时猜测准备结果；同一组织/member锁保证读取原事务的持久事实。
func (s *Organizations) DepartureStatus(ctx context.Context, organization, user, operation, expectedEpoch string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	unknown := domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "DEPARTURE_STATUS_UNKNOWN"}
	if s.repository.LockDepartureOrganization(ctx, tx, organization) != nil {
		return "", unknown
	}
	state, epoch, err := s.repository.LockDepartureMember(ctx, tx, organization, user)
	if err != nil {
		return "", unknown
	}
	err = s.repository.LockDepartureReceipt(ctx, tx, operation)
	result := "COMMITTED"
	if err == sql.ErrNoRows {
		if state == "ACTIVE" && epoch == expectedEpoch {
			result = "ROLLED_BACK"
		} else {
			result = "SUPERSEDED"
		}
	} else if err != nil {
		return "", unknown
	}
	if tx.Commit() != nil {
		return "", unknown
	}
	return result, nil
}
````
<!-- /lesson-code -->

Document 在成员离开准备请求回复未知时，以组织/user/原operation/旧epoch反查。Account按 organization→member→receipt 锁同一权威事实：receipt存在是 COMMITTED；不存在且成员仍ACTIVE且epoch等于expected是 ROLLED_BACK；不存在但状态/代次已变为 SUPERSEDED。查询或Commit故障是 DEPARTURE_STATUS_UNKNOWN，不能按时间长短猜回滚。这个结果是跨服务恢复协议的一部分，不给普通浏览器查询范围。

## 6. 接线、检查与排错

四个内部POST路由 `/infra/notification/recipients`、`/administrators`、`/events`、`/events/ack` 都先要求实际document证书，正文最多8192字节、拒多余JSON/未知字段。Events只接对象（通常 `{}`），null不接受；ACK回 Acknowledged=true 只在全批事务提交后。`/infra/lifecycle/member-departure-status` 是GET，校验组织/user/operation，调用前面的状态反查。A08 RegisterNotificationRecipients/Events、RegisterOrganizationReads 负责完整登记，bootstrap以同一Store.DB构造用例。

本模块没有单独的 NotificationRecipientIDs Go 单测，不能把构建说成行为验证。A08可先 `go test ./internal/logic/account ./internal/repository/account -run '^$'` 验包能编译，该命令故意不执行用例。真实投递测试必须等06自己的四服务READY、开发SMTP收件夹具、迁移、Edge（msedge）及锁定浏览器依赖齐备：

```powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 --test-name-pattern='^组织转让与文档通知统一分页，原号不重复、本人已读且退出后隐藏$' tests/notifications/account-delivery.test.mjs
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

该实际测试应PASS且skip=0：真实组织owner转移与文档负责人转移产生统一分页通知，同原号不重复；其他人标读被拒；浏览器本人标读持久化、可打开组织；先交回职责再退出后原组织提醒隐藏。本次编写未运行，也未把它当离线检查。测试创建自己的随机用户/组织，必须在学习实例串行执行。

提醒一直没有：依次看原owner transfer是否SUCCEEDED、Account QUEUED事件/固定payload、Document导入是否写收件箱、ACK是否回Account。403先查服务证书角色；503区分收件资格不可核验与事件绑定坏；退出后不隐藏应查Document通知读取时是否重新核当前来源。不要用直接插入假notification代替真实转移。

### 6.1 把可靠投递与10条通知展示页接起来

本课后端仍写 Account 的收件资格、事件与确认；收件箱路由和读取资格接在 D09 的 Document 服务。前端是支持包中的 `web/src/features/notifications/api.ts`、`useNotifications.ts`、`NotificationsPanel.vue` 与公共 `PaginationControls.vue`，无需在 Account 再创建一套通知页面或数据库权威。

页面调用既有 `GET /v1/notifications?limit=10`，继续时加原 `cursor`，只保留页入口而不缓存授权结果；`POST /v1/notifications/:notificationId/read` 仍只能由实际收件人执行。只有一页时隐藏分页栏，翻页中禁用重复操作，换账号清除旧页和游标并忽略旧响应。标读后的当前页重新读服务端，返回上一页也要重新读，所以离组或撤权后旧提醒不能借浏览器缓存继续展示。10条是产品展示大小，不是后台100名收件人批次，更不是通知总量上限。

浏览器布局和翻页可在自己的学习工程已提供当前前端资源、安装 Edge/msedge 与锁定依赖后检查：

~~~powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-concurrency=1 tests/browser/list-pagination.test.mjs
if ($LASTEXITCODE -ne 0) { throw '通知与组织分页页面交互失败' }
~~~

该测试启动真实浏览器，但拦截全部 `/v1/**` 业务 API，验证通知10条替换、返回首页重新请求、成员分页时管理入口仍可用及手机布局；它不证明邮件、真实数据库或可靠投递。前面的 `account-delivery.test.mjs` 才走真实账号转移、Document导入、本人标读与退出隐藏链。各自必须有实际用例且 fail/skip 为0，结果分开记录；本段命令是学习者验收步骤，不代表本轮已经执行。

## 7. 章末核对

[管理员模型](../../source/039-notification-administrators.md)、[收件模型](../../source/040-notification-recipients.md)、[资格SQL](../../source/052-notifications-queries.md)、[事件SQL](../../source/051-notification-events.md)、[资格用例](../../source/063-notifications-service.md)、[事件用例](../../source/062-notification-events.md)、[HTTP](../../source/074-notifications-handlers.md)。

为什么 ACK 失败不能清空事件？远端可能已收下而本地未确认，持久原事件身份让重投去重；清空会把未知变成永远丢失。为什么管理员列表还需Check？候选目录是某次语句快照，最终投递须用新的锁定资格及撤权来源。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
Set-Location -LiteralPath $lessonRoot
$paths = @(
 'services/account/internal/domain/account/notification_administrators.go',
 'services/account/internal/domain/account/notification_recipients.go',
 'services/account/internal/repository/account/notifications_queries.go',
 'services/account/internal/repository/account/notification_events.go',
 'services/account/internal/logic/account/notifications_service.go',
 'services/account/internal/logic/account/notification_events.go',
 'services/account/internal/handler/account/notifications_handlers.go'
)
foreach ($path in $paths) {
 New-Item -ItemType Directory -Force (Split-Path $path) | Out-Null
 if (-not (Test-Path -LiteralPath $path)) { New-Item -ItemType File $path | Out-Null }
}
```
