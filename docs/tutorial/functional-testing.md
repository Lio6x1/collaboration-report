# 跟着写一个功能，再证明它能用

学习目标已确认：你在Windows中创建目录、文件、模型和接口，接通调用，逐步完成后端；前端复用现成代码。重要模块的功能测试属于每个功能的交付步骤，不能只在最后运行一次，也不能把“编译通过”写成“功能通过”。

本页给出测试安排和当前实现的代表入口。主学习路线为[从空目录连续带写](continuous/README.md)：直接采用当前全量代码，各阶段先做独立规则/用例检查，四服务接齐后在自己的实例按模块做[真实功能及整站验收](continuous/06-integration.md)。历史章节仍使用当时配套，不把当前测试反套到未实现这些功能的旧版本；连续路线的实际证据见[验证记录](continuous/08-verification.md)。

## 每完成一个功能，按这条顺序验收

1. 看懂场景、输入输出及失败条件。
2. 创建目录、完整源码文件和必要模型，写出业务实现。
3. 接上repository、依赖装配、HTTP路由或RPC；异步功能接上worker。
4. 运行本节的规则测试，定位业务边界问题。
5. 启动自己的学习实例，运行该模块的真实接口或协作测试。
6. 用现成前端操作一次，观察结果，再记录命令、实际输出和未完成项。

测试教材需要给具体测试文件、前置条件、命令、预期与排错。关键断言要讲为什么检查它；已有测试已覆盖的内容直接复用，重要例子带着读懂和修改，不为每个普通赋值再造一个测试。

## 三类验证各证明什么

| 验证 | 什么时候做 | 能证明什么 |
| --- | --- | --- |
| 规则/单元测试 | 写完密码、权限矩阵、状态机等规则后 | 给定输入下规则正确，异常分类和边界符合要求；依赖可能是替身，不能证明真实数据库或邮件可用。 |
| 真实功能测试 | 一个模块的路由、依赖、存储与后台任务接通后 | 请求实际到达学习实例，检查HTTP/WSS、持久结果和异步终态，不能只断言200/202。 |
| 页面与整站验收 | 模块联调和课程收口时 | 页面入口可用，多模块能完成同一用户流程。模拟API的浏览器测试只证明相应界面交互，仍需真实接口验证。 |

`go build`、类型检查和语法检查是前置检查。它们不能代替上述功能测试。真实网络请求、模拟API和教师历史运行记录要分开记。

## 重要模块必须验证的内容

下列是当前测试入口，不是要求每次改一行都执行整张表。连续路线直接使用其当前配套；读历史章才取用那个版本的测试，不能反套当前文件。每次选与本次完成的用例直接相关的一组串行运行。

| 模块 | 至少要证明的行为 | 当前代表测试与课程 |
| --- | --- | --- |
| 账号、验证码和会话 | 正常登录/注册；弱密码、验证码错误和冷却被拒绝；邮箱占用不会新增账号；退出后旧会话失效；数据库故障不能冒充密码错误。 | [登录接口](../../tests/login/auth.test.mjs)、[验证码](../../tests/account/challenge.test.mjs)、[凭据](../../tests/account/credential.test.mjs)、[当前账号规则](continuous/lessons/account/01-authentication.md)。 |
| 组织、知识库与权限 | 有权成员成功，无权成员被拒绝；不能伪造身份；修改权限后旧读取资格按协议失效。 | [组织](../../tests/account/organization.test.mjs)、[权限](../../tests/workspace/permissions.test.mjs)、[分享来源](../../tests/workspace/sharing.test.mjs)、[第04章](continuous/lessons/document/02-workspace-content.md)。 |
| 协作和可靠保存 | UPDATE持久化后才能SAVED；重复更新不重复计入；FLUSH确认原输入；断线恢复不丢已确认内容。 | [真实WSS](../../tests/collaboration/wss.test.mjs)、[恢复](../../tests/collaboration/recovery.test.mjs)、[第05章](continuous/04-collaboration.md)。 |
| 目录与多人拆分 | 表格归属、父章及子节正确；在线输入先保存；未应答或冲突不能强行覆盖支线；旧预览不能误用。 | [纯迁移规则](../../services/collaboration/tests/structure-migration.test.mjs)、[在线协调](../../services/collaboration/tests/structure-editor-coordination.test.mjs)、[当前结构课](continuous/lessons/document/04-structure.md)。 |
| 审核、合并与冲突 | 提交后继续改草稿不改变原送审内容；权限符合范围；合并固定版本，冲突保留并要求解决。 | [固定提交](../../tests/review/submission.test.mjs)、[合并](../../tests/review/merge.test.mjs)、[冲突解决](../../tests/review/conflict-resolutions.test.mjs)、[第06章](continuous/lessons/document/05-submission-review.md)。 |
| 发布与搜索 | 发布冻结正文；真实事件被消费并生成索引；搜索结果仍按当前权限裁剪；撤下后旧公开资格失效。 | [发布](../../tests/publication/releases.test.mjs)、[真实索引](../../tests/search/indexing.test.mjs)、[可见范围](../../tests/search/visibility.test.mjs)、[第07章](continuous/05-search.md)。 |
| 图片、评论与通知 | 上传、结算与引用完整；越权读取被拒绝；评论保持原引用；通知交给当前有权收件人。 | [上传](../../tests/resources/upload.test.mjs)、[评论](../../tests/comments/threads.test.mjs)、[评论通知](../../tests/notifications/comment-delivery.test.mjs)、[第09章](continuous/lessons/document/09-comments-notifications.md)。 |
| 导出 | 原作业生成并下载实际Word/PDF/Markdown；固定内容正确；多作业遵守worker上限；未知结果沿原号恢复。 | [真实格式](../../tests/exports/formats.test.mjs)、[队列](../../tests/exports/pool.test.mjs)、[恢复](../../tests/exports/recovery.test.mjs)、[当前导出课](continuous/lessons/document/10-exports.md)。 |
| 回收、恢复与清理 | 回收撤掉旧入口；恢复保持私有未发布语义；永久清理完成须与后台物理清理一致。 | [回收恢复](../../tests/lifecycle/trash-restore.test.mjs)、[永久清理](../../tests/lifecycle/purge.test.mjs)、[第10章](continuous/lessons/document/12-trash-transfer.md)。 |
| 长列表、目录搜索与草稿保留 | 翻页替换当前页，上一页重新核权限；全目录搜索先筛选再按可见limit组页；本人资格、固定选择和未提交草稿不由当前显示页决定。 | [游标规则](../../tests/unit/list-pagination.test.mjs)、[工作台组件](../../tests/unit/workspace-pagination.test.mjs)、[组织组件](../../tests/unit/organization-management.test.mjs)、[历史组件](../../tests/unit/history-pagination.test.mjs)、[真实目录API](../../tests/workspace/list-pagination.test.mjs)、[模拟API分页页面](../../tests/browser/list-pagination.test.mjs)、[模拟API讨论页面](../../tests/browser/reading-comments.test.mjs)。 |

跨模块必须保留三个反例：无权用户不能执行；同一操作重复提交不重复生效；提交后断网不能直接认定失败，须查询原操作号。哪一组现有测试已经覆盖，就在该节明确引用，不重复建立一套相同测试。

## 分页这一关怎样做、怎样分开记证据

前端由支持包复用，不要求学习者把Vue再亲写一遍；后端的当前身份、搜索条件、签名cursor和SQL过滤仍按A03/D02/D09逐步亲写。先在自己的 `$lessonRoot` 完成本课代码、支持文件和锁定依赖，不复制教师实例的 `.runtime/private`、Cookie、密码或服务端口。

| 界面 | 每页数量与独立范围 |
| --- | --- |
| 统一通知 | 10条；列表页不等于全量未读数，标读仍由实际收件人执行。 |
| 主评论/讨论卡片 | 10条线程；正文Kind/Context/Version筛选在服务端执行，回复在选中线程后每批20条按需续读。 |
| 知识库卡片 | 12个；当前选中的后页知识库单独保存在详情，不追加到首页网格。 |
| 组织、组织成员、组织审批、邀请、个人入组申请 | 各20条，各有自己的游标；本人资格来自组织详情的 `current_member`，不扫描全成员目录找本人。 |
| 文档、个人协作申请、知识库成员 | 各20条；显示成员页不能替代授权候选picker。 |
| 审核请求、轮次、修改要求、正式版历史 | 各20条；选定的固定正文、原确认目标和本机说明/处理草稿不随列表页变化。 |
| 回收站、个人转移历史 | 各20条；转移接收目标库另按需加载，不能共用历史cursor。 |

共同规则：下一页替换当前列表，上一页重新发HTTP核当前授权，不显示旧授权缓存；只有一页时隐藏分页栏。搜索提交、空间/文档/账号/方向等相关范围变化回第1页，丢弃迟到响应；空页有next仍能继续。目录选择、空间选择、提及与授权候选仍按需加载。讨论列表翻页保留原选中线程与新评论/回复/编辑草稿；回复“查看更多”只续当前线程，不能为了显示10张卡片下载全部回复。

服务端搜索要同时检查**过滤在组页之前**与**条件进入cursor签名范围**：组织成员q匹配昵称子串或去补零后的完整账号，KB名称q在SQL WHERE中匹配名称，再LIMIT；KB成员q在服务端有界扫描、Account姓名解析及目标资格重验后按可见limit组页，不能跨私库SQL联表。三种q均trim/lower、最多100个Unicode字符。评论的 `context_kind/context_id/content_version_id` 先验证再加入固定SQL条件并绑定原cursor，Context非空必须带Kind。改q/上下文复用旧cursor应失败，不能回退成全量查询；筛选命中不增加任何当前权限。

先跑不依赖真实服务的行为检查，前置是支持包中的Vue编译器、TypeScript与Node依赖已经按01安装：

```powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-concurrency=1 tests/unit/list-pagination.test.mjs tests/unit/workspace-pagination.test.mjs tests/unit/organization-management.test.mjs tests/unit/history-pagination.test.mjs
if ($LASTEXITCODE -ne 0) { throw '分页规则或组件行为失败，保留具名失败输出' }
```

`list-pagination`验证页入口、返回前页后丢弃旧后续历史、空页续读和范围重置；`workspace-pagination`编译真实组件setup，验证12/20条替换、全目录查询URL、旧响应丢弃与后页当前库独立；`organization-management`核当前本人关系与20条成员页独立；`history-pagination`核回收/转移/审核/发布列表与选择、草稿和整体资格独立。后三份的HTTP响应由测试替换，所以通过只证明组件行为，不证明真实SQL或撤权协议。

自己的四服务、迁移、开发SMTP夹具和后台worker按06 READY后，再执行真实接口测试：

```powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-concurrency=1 tests/workspace/list-pagination.test.mjs tests/comments/threads.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实目录或评论筛选分页失败' }
```

第一份创建真实账号/组织/知识库夹具，核 `current_member`、后页补零账号与知识库名称查询、改q旧cursor拒绝和q长度边界；昵称子串的接口语义按A03/D02另做页面操作，不扩大为该文件已有的具名断言。第二份写真实固定讨论与回复，核上下文在组页前过滤、跨上下文cursor拒绝、墓碑/修订及普通评论不解除正式阻止意见。它们会持久写专用夹具，要求使用自己的学习实例与当前实际归属；不指向体验数据，不用模拟返回200代替真实结果。

最后确认自己的前端资源已经由学习实例提供、Edge/msedge与锁定浏览器依赖可用，再跑页面交互：

```powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-concurrency=1 tests/browser/list-pagination.test.mjs tests/browser/reading-comments.test.mjs
if ($LASTEXITCODE -ne 0) { throw '分页或讨论的模拟API浏览器交互失败' }
```

这两份打开真实浏览器，但拦截全部 `/v1/**` 业务API。第一份检查通知替换页、返回首页重新请求、本人不在成员页时管理入口仍在及移动布局；第二份检查固定/历史锚点、同一讨论面板、未提交草稿与折叠回复按需展开。它们不证明真实后台权限、数据库、投递或原号恢复。各组须有实际用例、fail=0且skip=0；把结果分别记为“组件行为/真实API/模拟API浏览器”，不要把作者或参考产品的PASS填写成自己的验收。源码摘要和教学同步检查也另记，不能代替这些功能证据。

排错按边界定位：搜不到先核当前空间与规范条件；空页加next先确认有界扫描而非全量结束；400/409旧cursor错误回同条件首页，别改签名；管理入口消失查是否错误依赖当前成员页；翻页丢草稿查是否卸载详情面板；回页还能看到被撤权限内容查是否复用了结果缓存或漏掉最终来源检查。遇到真实未确认写入仍先续原操作号，不因翻页重发新写请求。

## 一个例子：密码规则怎样测

完成[当前账号课](continuous/lessons/account/01-authentication.md)后，打开[完整规则测试文件](../../services/account/internal/domain/account/validation_test.go)，定位`TestNewPasswordPolicyKeepsLegacyInputSeparate`。

它用一张输入/预期表说明业务要求：`Abcdef12`、`abcd!efg`、`1234567!`三种两类组合应通过；不足8个字符、只有字母或只有数字应拒绝。`abcdEFGH`仍只有英文字母一类，不能把大小写当成两类。测试还检查UTF-8字节上限与旧密码登录规则。

先讲清表中每个反例为什么存在，再看循环怎样调用`ValidNewPassword`并比较预期。失败信息不输出实际密码。另一个[用例测试文件](../../services/account/internal/logic/account/authentication_service_test.go)检查弱密码在访问验证码或存储前被拒绝，以及身份失效、数据库故障和验证码失败不会混成同一种错误。不要只验证页面提示。

下面是当前路径的定向命令；此时你自己的完整文件和依赖装配应已写完。`-run`限定4个具名测试，`-count=1`避免直接使用上次缓存的结果，`-v`显示具体用例：

```powershell
# 工作目录：你自己的当前版本学习工程根目录，先进入它；不要指向体验产品。
$env:GOWORK = 'off'
go -C services/account test -mod=readonly ./internal/domain/account ./internal/logic/account -run '^(TestNewPasswordPolicyKeepsLegacyInputSeparate|TestWeakNewPasswordsRejectedBeforeChallengesOrStorage|TestAuthenticationDistinguishesInvalidIdentityFromReadFailure|TestChallengeFailuresKeepTheirCategoryAcrossBusinessEntries)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '账号规则或用例测试失败，先检查具名失败原因' }
```

预期这4个测试均实际RUN/PASS、退出码0。它们不启动服务、不证明真实SMTP已送达，也不证明注册数据库事务已经跑通。下一步才是自己实例上的真实功能验证。

## 怎样运行真实模块测试

先按[连续路线的实例运行步骤](continuous/06-integration.md)完成本阶段源码、锁定依赖、迁移和中间件初始化，等实际READY。测试从当前学习工程的`.runtime`读取实际入口和归属；不复制参考产品的私密配置，也不套用参考产品端口。

会自动创建随机账号的场景依赖开发SMTP收件器读取验证码，例如`tests/helpers/account.mjs`中的`createUser`。使用真实SMTP的学习实例，要按该测试明确支持的专用身份方式准备，或另建使用开发收件器的测试实例；不能以为随便设置一个账号变量就能替代所有场景的注册前置。真实发信送达单独通过自己的邮箱验证。

例如当前权限模块接通后，在自己的已就绪实例根目录执行：

```powershell
# 会创建专用测试账号、组织和文档，验证真实HTTP与数据库权限事实。
node --no-maglev --test --test-concurrency=1 tests/workspace/permissions.test.mjs tests/workspace/sharing.test.mjs
if ($LASTEXITCODE -ne 0) { throw '权限功能验证失败，保留原输出并按失败用例排查' }
```

`--test-concurrency=1`让文件串行执行，减少同一学习实例上的测试相互干扰。预期指定场景实际运行、fail=0且skip=0；没有测试被发现、全部跳过或服务未就绪都不算通过。接口测试的夹具会准备新样本；先读测试前置与副作用，不拿自己的重要文档替换夹具。

每组测试后至少手工操作一次相应页面。例如权限测试后，用A/B两个测试账号确认成员选择、授权反馈、无权提示与撤权后的页面行为。自动化断言与手工观察一起用于定位接口正确而界面未接通的问题。

## 怎么看失败，怎么记录通过

| 现象 | 先检查什么 |
| --- | --- |
| undefined、重复声明、无法导入 | 完整文件是否输入完，package/import是否正确，是否重复追加旧函数。 |
| 连接被拒绝、缺少运行配置 | 工作目录、本阶段初始化、实际READY及测试读取的实例；不要先改断言。 |
| 找不到验证码或SMTP目录 | 开发收件器是否启动，邮件模式与测试前置是否一致。 |
| 401/403与预期不符 | 同一次登录的Cookie/CSRF、真实成员来源和当前权限；区分身份失败与依赖故障。 |
| 操作一直处理中 | 记录原操作号，核worker、RPC与持久任务；查询或恢复原号，不另换号冒充成功。 |
| 测试退出0但只有skip | 是否选错平台、用例名称或版本；本节必须实际执行的测试不能以跳过代替。 |

记录所在版本/修改范围、命令、实际结果、测试类型及未验证内容。不记录密码、Cookie或云密钥。教师的历史PASS、模拟API通过和源码摘要一致不能填成自己的真实功能已通过。

## 完成课程时再做一次整站闭环

模块通过后，按[连续路线的最终装配与验收](continuous/06-integration.md)完成同一文档的双人协作、可靠保存、固定审核、合并、图片/评论、发布/搜索和实际导出。当前代表入口是[整站真实流程](../../tests/deployment/full-workflow.test.mjs)；它有完整环境与转换工具前置，不能在登录刚写完时运行。

故障恢复、容量和并发测试按教学目标选择。关键权限、可靠保存、固定版本、幂等和原号恢复仍要验证；大规模数据、完整冷恢复及全部历史故障排列无需每章重跑。编写一个重要模块、运行直接相关测试、修正后继续下一模块，课程收口再联调完整流程。

本页是学习者的操作与验收说明，不表示上述真实模块测试已经在你的实例执行。作者本轮执行范围另记在教学同步记录。
