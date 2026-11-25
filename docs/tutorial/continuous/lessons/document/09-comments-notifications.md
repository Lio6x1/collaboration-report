# D09｜固定批注、提及与可靠通知

## 这节课要实现什么

读者选中一段文字提问，协作者回复或被提及时能收到提醒。本课实现有正文位置依据的评论、讨论回复和可靠通知。

## 做完后能看到什么

接通后，讨论绑定创建时的正文与位置；正文变化后仍能核对历史原文。重复请求不应重复评论或提醒，通知只有当前有资格的人可以读取。评论并不自动成为阻止审核的修改要求。

主评论列表每页10条讨论，按当前正文或历史上下文在服务端先筛选再分页；选中讨论后才按需读取回复，每批20条。讨论翻页保留当前选中线程和未提交草稿，通知中心另按10条显示，不会把一页里的记录当成全部历史。

## 先想清楚怎样实现

1. 先固定评论锚点与提及对象。
2. 将评论、原操作回执和待通知事件一并提交。
3. 后台核当前收件资格后投递，并接入账号侧事件和收件箱读取。

## 怎样判断这一课完成

D14后执行评论通知规则，06实际选文评论、回复、提及和重复提交，并验证历史定位与撤权后的可见性。

## 开始写之前

继续[D08](./08-resources-storage.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。批注锚点和通知收件集合要先固定，后续修改正文不能重算原通知。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/discussion_models.go**，用途是批注锚点和通知收件集合要先固定，后续修改正文不能重算原通知。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 先固定锚点与通知身份

discussion_models.go先写CommentNotification、ValidateCommentBody、ValidateCommentAnchor。CommentNotification.Document/Thread/Comment绑定原讨论，ContextKind/ContextID说明MAIN、BRANCH或REVIEW上下文，Version是原不可变正文，Actor是作者；Recipients是创建时显式@集合，ReplyRecipients是回复关联集合，DeliveryAfter保存已经提交到哪名收件人。恢复不读取后来编辑的body重新猜@谁。

在原文件ValidateCommentBody后输入：

<!-- lesson-code: services/document/internal/domain/document/discussion_models.go :: ValidateCommentAnchor -->
````go
func ValidateCommentAnchor(anchor dto.Anchor, body []byte) error {
	invalid := Violation(InvalidRequest, "COMMENT_ANCHOR_INVALID")
	if anchor.SchemaVersion != "anchor-v1" {
		return invalid
	}
	if _, err := CanonicalID(anchor.ContentVersionId); err != nil {
		return invalid
	}
	if anchor.Quote != nil && (!utf8.ValidString(*anchor.Quote) || utf8.RuneCountInString(*anchor.Quote) > 500) {
		return invalid
	}
	if anchor.BlockId == nil {
		if anchor.StartOffset != nil || anchor.EndOffset != nil || anchor.Quote != nil {
			return invalid
		}
		return nil
	}
	if !ValidName(*anchor.BlockId, 96) || (anchor.StartOffset == nil) != (anchor.EndOffset == nil) {
		return invalid
	}
	var root structNode
	if json.Unmarshal(body, &root) != nil {
		return Violation(TemporarilyUnavailable, "COMMENT_CONTENT_INVALID")
	}
	var found *structNode
	var visit func(*structNode)
	visit = func(node *structNode) {
		if id, ok := node.Attrs["id"].(string); ok && id == *anchor.BlockId {
			found = node
		}
		for i := range node.Content {
			visit(&node.Content[i])
		}
	}
	visit(&root)
	if found == nil {
		return invalid
	}
	if anchor.StartOffset == nil {
		if anchor.Quote != nil {
			return invalid
		}
		return nil
	}
	// 浏览器选区偏移按UTF-16单位核验；原子图片/公式占一个占位符，不能按UTF-8字节截断中文。
	var text func(structNode) string
	text = func(node structNode) string {
		if node.Type == "text" {
			return node.Text
		}
		if node.Type == "image" || node.Type == "formula" {
			return "\ufffc"
		}
		if node.Type == "hard_break" {
			return "\n"
		}
		var out strings.Builder
		for _, child := range node.Content {
			out.WriteString(text(child))
		}
		return out.String()
	}
	units := utf16.Encode([]rune(text(*found)))
	start, end := *anchor.StartOffset, *anchor.EndOffset
	if start < 0 || end < start || end > int64(len(units)) {
		return invalid
	}
	boundary := func(index int64) bool {
		return index == 0 || index == int64(len(units)) || !(units[index-1] >= 0xd800 && units[index-1] <= 0xdbff && units[index] >= 0xdc00 && units[index] <= 0xdfff)
	}
	if !boundary(start) || !boundary(end) {
		return invalid
	}
	if anchor.Quote != nil && string(utf16.Decode(units[start:end])) != *anchor.Quote {
		return invalid
	}
	return nil
}
````
<!-- /lesson-code -->

anchor-v1必需，content_version_id必须规范；block_id为空时不能夹offset/quote。浏览器offset按UTF-16单位，不按UTF-8字节也不简单按Go rune。用“甲🙂乙”推演：UTF-16长度4，🙂占[1,3)，选择[1,2)切断代理项应拒绝；quote若是🙂，合法范围必须是[1,3)。图片/公式各算一个占位符，hard_break算换行，才能与真实浏览器选区一致。

在notification_delivery.go先定义NotificationRecipientResolver。Resolve(ctx,document,ids)取得真实用户资格/来源，Administrators(ctx,organization,kb)取得当前管理员目标；返回platform的跨服务响应及error，网络失败不能替换为空收件人列表。

## 3. 将讨论、幂等回执和通知任务一起提交

先完成repository/comment/comment_queries.go：ThreadRow/CommentRow、InsertThread/InsertComment、QueueMentions、修订/状态/墓碑更新。版本与context归属必须SQL证明，用户说是REVIEW就不能直接信。通知repository保存原task、精确去重键及进度；departure_notifications.go保存负责人离开事件的真实上下文。

在comments_service.go的commentMentions后输入完整Create：

<!-- lesson-code: services/document/internal/logic/document/comments_service.go :: Comments.Create -->
````go
func (s *Comments) Create(ctx context.Context, actor Actor, document uint64, input dto.CommentThreadCreate) (Actor, MutationBody, MutationPresentation, error) {
	t, err := commentContextInput(document, input)
	if err != nil {
		return actor, nil, nil, err
	}
	if err = domain.ValidateCommentBody(input.Body); err != nil {
		return actor, nil, nil, err
	}
	mentions, err := commentMentions(input.Mentions)
	if err != nil {
		return actor, nil, nil, err
	}
	actor, a, err := s.authorize(ctx, actor, document, true)
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if err := s.validateContext(ctx, tx, actor, a, t, true); err != nil {
			return nil, err
		}
		id, err := comments.InsertThread(ctx, tx, document, actor.ID, t.Kind, t.Context, t.Anchor)
		if err != nil {
			return nil, err
		}
		comment, err := comments.InsertComment(ctx, tx, id, actor.ID, input.Body, nil)
		if err != nil {
			return nil, err
		}
		target := t
		target.ID = id
		if err := comments.QueueMentions(ctx, tx, input.OperationId, target, comment, actor.ID, mentions); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		id, err := commentReceipt(value)
		if err != nil {
			return nil, nil, err
		}
		current, err := comments.ThreadRow(ctx, tx, id, false)
		if err != nil {
			return nil, nil, err
		}
		if current.Document != document || current.Author != actor.ID {
			return nil, nil, domain.Violation(domain.StateConflict, "COMMENT_RECEIPT_MISMATCH")
		}
		// 已删除的线程仍保留原创建成功事实；当前资格须复核，但不恢复线程。
		current.Status = "OPEN"
		if err := s.validateContext(ctx, tx, actor, a, current, false); err != nil {
			return nil, nil, err
		}
		return value, a.Sources, nil
	}
	return actor, run, present, nil
}
````
<!-- /lesson-code -->

Create返回actor、MutationBody、MutationPresentation、error。方法先校验正文、最多100个提及并规范去重排序，取得COMMENT资格；run在同一事务核上下文、插thread、插comment、排QueueMentions并返回固定entity。presentation重放时再核当前资格及原thread归属；已删除thread仍保留创建成功事实，但不会被恢复。

Change按来源→文档→thread/comment顺序加锁，回复需要当前COMMENT，修改只允许作者，删除可有管理例外；expected revision比较防止旧页面覆盖新评论。普通thread RESOLVED不触碰正式change_request，不能靠“解决批注”绕过审核阻止。

### 3.1 为原固定上下文增加只读筛选，再执行有界授权分页

继续写已有三份文件：在 `repository/comment/comment_queries.go` 定义 `ThreadFilter` 并扩展 `ListThreads` 的固定列条件；在 `logic/document/comments_service.go` 验证过滤对象并把它放入原签名cursor绑定；在 `handler/document/discussion_handlers.go` 从URL组成过滤对象，最后仍走原 `Transactions.Read`。原因是先取全文档10条、再由前端丢弃其它版本，会让当前版本明明有讨论却出现空白页。

| URL参数 / ThreadFilter字段 | 应怎样解释与验证 |
| --- | --- |
| `context_kind` / `Kind` | 可空；非空只能是 `MAIN/BRANCH/REVIEW`，不能把任意文字当成SQL列或权限类别。 |
| `context_id` / `Context` | 对应SQL `context_ref`；非空必须同时指定Kind，并是规范非零十进制ID。它是manifest/branch/review round的原上下文身份。 |
| `content_version_id` / `Version` | 可选原不可变正文版本ID，非空须规范；不是客户端希望读取的“最新版本”。 |
| `limit/cursor` | 线程界面传 `limit=10`；cursor绑定同一组过滤值、当前会话、文档、结构和实际来源代次摘要。首页不传cursor。 |

这三个条件属于既有 `GET /v1/documents/:documentId/comment-threads` 的只读URL扩展，不修改创建评论的锚点请求体。仓储 `ListThreads(..., filters ...ThreadFilter)` 和业务 `Comments.List(..., filters ...comments.ThreadFilter)` 兼容原不传过滤对象的调用：零参数是空条件，当前仅消费首个对象，不合并多个OR条件。业务先检查Kind、Context依赖及两个ID，再把整个filter的JSON放入 `commentPageBinding` 的target。换版本、上下文或筛选后原cursor必须失效，不能默默按原条件续页。

仓储先执行 `document_id=? AND id>? AND status<>'DELETED'`，再按非空字段增加固定的 `context_kind=?/context_ref=?/content_version_id=?`，最后 `ORDER BY id LIMIT ?`。列名来自代码内固定清单，所有用户值经占位符；不会把任意URL文本拼成列名。业务层在原输出事务中再次 `ValidateAccess`，逐线程 `validateContext` 核原版本与当前/历史资格，再按请求limit组成可见页，当前界面每页10项；每次最多扫描1000条。预算耗尽可能返回空页加next，前端仍可继续，不泄露隐藏讨论总数。筛选只是缩小范围，不能跳过BRANCH谱系、REVIEW归属或历史正文读取权限。

当前正文通常同时传Kind/Context/Version。MAIN历史浏览保留Kind、放开Context与Version；BRANCH历史保留Kind与原branch Context、放开Version。这样历史范围仍由服务端SQL和原权限约束，而不是下载全部讨论后客户端筛选；选中历史线程依然按它自己的原锚点定位。

### 3.2 复用讨论页，保留原线程草稿并按需查看回复

复用 `web/src/features/comments/api.ts`、`useComments.ts`、`CommentsPanel.vue` 及阅读入口，不在后端新增一份临时讨论缓存。主评论下一页替换10张卡片，上一页也HTTP重读，只有一页时隐藏分页栏；筛选或文档/会话范围改变重置页入口，丢弃旧响应。单纯列表翻页不重建当前讨论面板，因此选中的原线程、新评论正文、原线程回复和编辑草稿不被新页清掉；不能把草稿改绑到新页第一条讨论。切到另一真实文档/账号或失去权限时，旧可读对象与操作目标必须清除，不能把旧资格带到新范围。

卡片进入可视区域才可单独读取首条摘要，`limit=1`且最多两个并发；不能为了画10张卡片预取所有线程回复。用户选中线程才取该线程首批20条评论，回复先折叠；“查看回复”展开已取得的内容，“查看更多回复”才按该线程next_cursor继续读取20条。这里回复是同一线程内的按需续读，与替换式主评论分页分开，不能把线程页游标用于回复。输入草稿和HTTP授权结果也分开保存，翻回旧页必须重新核验权限。

## 4. 后台投递与账号事件导入

输入NotificationDelivery.Recover及完整辅助声明。它在SQL锁外向账号服务查资格，然后短事务重核Doc ACL、原上下文/版本、当前thread/comment、原task代次，再写各收件人通知与持久DeliveryAfter；分批每次最多100名。作者不通知自己，重复@只投一条。某收件人当前无权则不泄露正文；依赖未知时保留task重试，不能认作确定无权后丢掉通知。

account_notifications.go通过真实document mTLS读取Account未确认事件，先在account_notification_import.go同事务导入原event身份并去重，提交后才AckEvents。不能只记“最大已见ID”，迟提交的小ID可能仍未导入；网络丢ACK时重读同event也不会新增通知。

用户选中固定正文写批注→discussion_handlers.go→Comments.Create→Transactions.Mutation→QueueMentions→RunNotifications→NotificationDelivery.Recover→Notifications列表。D14的NewComments、NewNotificationDelivery注入真实Account NotificationRecipients，identity_notifications_workers.go驱动通知与账号导入，使用store.Context退出。通知标读仅本人可做，读取列表仍按当前资格隐藏撤权内容。

## 5. 测试与排错

包齐后离线：

~~~powershell
go -C services/document test -mod=readonly ./internal/logic/document -run '^(TestDocumentMentionCandidatesRequireCommentAndBindCurrentTargets|TestDocumentMentionCandidatesUseVisibleOrganizationMembers)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '提及资格规则失败' }
~~~

workspace_directory_test.go通过测试依赖验证COMMENT门槛和当前组织范围；没有现成独立锚点单测时不伪造测试名称。锚点、持久讨论及通知真实行为由下列现有功能测试覆盖。

06 READY后：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/comments/threads.test.mjs tests/notifications/comment-delivery.test.mjs tests/notifications/review-delivery.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实批注/通知失败' }
~~~

固定审核批注支持幂等、分页、作者修订与墓碑且不解除正式阻止意见 核讨论和审核分界；批注通知在身份依赖未知后可靠重试，精确去重收件且撤权后隐藏 核未知不丢事件。锚点400查原version、block及UTF-16范围/quote；重复通知查原event去重键；通知迟到查原task与Account可达性，不把后台错误吞掉并标成功。

`tests/comments/threads.test.mjs` 还用真实API验证其它上下文不占满筛选页、过滤后有下一页，以及把REVIEW游标移到MAIN被拒绝。它请求学习实例并持久写夹具，不能用模拟页面通过代替。先核 `context_kind/context_id/content_version_id` 和原签名cursor，非法ID或缺Kind不能回退成全量讨论。

页面草稿、折叠回复和手机布局在自己的前端资源就绪后另运行：

~~~powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-concurrency=1 tests/browser/reading-comments.test.mjs tests/browser/list-pagination.test.mjs
if ($LASTEXITCODE -ne 0) { throw '讨论与通知分页页面交互失败' }
~~~

这两份使用真实 Edge/msedge 浏览器并拦截业务API。reading-comments检查同一个讨论面板、固定/历史锚点、翻页后草稿保留及折叠回复按需展开；list-pagination检查通知10条与组织成员分页。预期具名用例实际运行、fail/skip为0，不证明真实SQL、权限停止或后台通知已送达。若翻页丢草稿，先查是否随列表变化卸载面板；回复发错位置查是否把selected线程与新页第一项混用。[功能测试安排](../../../functional-testing.md)说明三类证据如何分别记录。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/discussion_models.go | [逐声明/函数带写](../../write/091-discussion-models.md) |
| 2 | services/document/internal/repository/comment/comment_queries.go | [逐声明/函数带写](../../write/139-comment-queries.md) |
| 3 | services/document/internal/repository/document/departure_notifications.go | [逐声明/函数带写](../../write/150-departure-notifications.md) |
| 4 | services/document/internal/repository/notification/notification_queries.go | [逐声明/函数带写](../../write/186-notification-queries.md) |
| 5 | services/document/internal/logic/document/account_notification_import.go | [逐声明/函数带写](../../write/191-account-notification-import.md) |
| 6 | services/document/internal/logic/document/comments_service.go | [逐声明/函数带写](../../write/196-comments-service.md) |
| 7 | services/document/internal/logic/document/notification_delivery.go | [逐声明/函数带写](../../write/212-notification-delivery.md) |
| 8 | services/document/internal/logic/document/notifications_service.go | [逐声明/函数带写](../../write/213-notifications-service.md) |
| 9 | services/document/internal/client/account_notifications.go | [逐声明/函数带写](../../write/108-account-notifications.md) |
| 10 | services/document/internal/handler/document/discussion_handlers.go | [逐声明/函数带写](../../write/252-discussion-handlers.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D10](./10-exports.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/discussion_models.go',
    'services/document/internal/repository/comment/comment_queries.go',
    'services/document/internal/repository/document/departure_notifications.go',
    'services/document/internal/repository/notification/notification_queries.go',
    'services/document/internal/logic/document/account_notification_import.go',
    'services/document/internal/logic/document/comments_service.go',
    'services/document/internal/logic/document/notification_delivery.go',
    'services/document/internal/logic/document/notifications_service.go',
    'services/document/internal/client/account_notifications.go',
    'services/document/internal/handler/document/discussion_handlers.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
