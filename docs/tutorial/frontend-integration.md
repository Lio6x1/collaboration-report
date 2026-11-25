# 前端直接复用：后端怎样接入与联调

前端使用项目现成代码，不需要跟写Vue组件、样式、ProseMirror或IndexedDB。你只需要会运行页面、找到它调用的后端入口、检查请求和响应，再把自己写的后端接到同一契约。

## 运行同一入口

自己的连续学习工程使用[装配与真实功能验收](continuous/06-integration.md)，始终在原lessonRoot；现成产品按[Windows启动](../operations/WINDOWS.md)。前端构建命令在对应项目根执行：

```powershell
./scripts/npm.ps1 ci
./scripts/npm.ps1 run build --workspace web
node --no-maglev scripts/start.mjs
```

前两条只安装/构建前端，不负责初始化数据库；start前仍需按部署/章节完成环境初始化。打开start输出的实际HTTPS地址，别将历史端口硬编码到另一学习实例。

当前前端发相对路径`/v1/...`，浏览器访问同源Edge入口；Edge根据`contracts/http/routes.json`转给对应Account/Doc/Search服务，WSS也使用同一入口。通常无需给前端填三个Go服务URL，也不应让浏览器连接内部gRPC或MySQL。后端实际端口由当前实例配置管理。

## 一个按钮怎样找到后端

打开浏览器开发者工具Network，点击动作，先记Method、Path、Payload与状态码，再按下面的路径定位代码：

| 从页面观察 | 现成适配入口 | 后端应提供什么 |
| --- | --- | --- |
| 登录与当前用户 | web/src/features/account/session.ts | POST /v1/account/login、GET /v1/account/me |
| 普通JSON与修改动作 | web/src/lib/http.ts | 生成契约的请求/响应、稳定错误信封、原号状态 |
| 正式版搜索 | web/src/features/search/api.ts、rules.ts | 真实搜索页、游标和资格/代次错误 |
| 图片上传 | web/src/features/resources/api.ts | 预留/二进制PUT/完成，当前Cookie与CSRF |
| 编辑票据 | web/src/features/editor/controller.ts | POST /v1/documents/{documentId}/units/{unitId}/edit-sessions |
| 协作连接 | web/src/features/editor/transport/draft_socket.ts | 返回的wss_url、OPEN票据与原room/workspace_epoch |

请求类型由`contracts/http/generated.ts`定义。Go使用同一契约生成的httpdto，Node内部RPC用原proto；不要为了临时让页面显示成功改出另一套字段名或返回假数据。

## 登录态与CSRF

浏览器自动携带同源Cookie，http.ts使用`credentials:'same-origin'`。真实会话在HttpOnly Cookie中，前端不读取会话token；登录返回的csrf_token保存为所属用户的页面状态，修改请求带`X-CSRF-Token`。

当前版另外使用标签会话上下文：`http.ts`的 `sessionContextID`生成选择器并放在sessionStorage中，主动登录轮换槽位。Edge的 `scripts/browser-session-context.mjs`按该槽位选取Cookie，将其还原为原服务认识的标准Cookie名；响应时再给Set-Cookie加上槽位后缀。Account继续核验标准会话及CSRF，不负责读取标签选择器。这样同一浏览器不同标签能登录不同账号；选择器本身不是身份凭据，不可替代真实Cookie与CSRF。图片/下载由 `sessionContentURL`带同一 `session_context`选择器，不能手拼链接遗漏上下文。复制标签可能复制槽位，下一次主动登录会轮换，退出影响共享同一真实会话的副本，改密/注销影响同账号相关会话。下方继续说明接线和隔离边界。

`/me`暂时503时保留已登录状态并提示重试；真实401才清除身份。服务端当前 `Authenticate`同样区分数据库故障与真实失效，不能为了让界面恢复直接吞掉401。

后端同时检查真实Cookie、精确Origin和对应CSRF，不能只信前端的user.id、x-user-id或本地缓存。页面已显示登录也不代表服务端权限一直有效；改密/退出/撤权后旧请求仍需后端拒绝。前端的transportGeneration用于忽略旧会话结果，不能代替服务器授权。


### 标签会话的实际接线

1. `http.ts`生成槽位选择器并放入sessionStorage，主动登录轮换，以`X-Collab-Session-Context`发送。
2. `browserSessionContext`选择匹配的HttpOnly Cookie，Edge移除浏览器自报的原Cookie及选择器，只转发选中的标准Cookie。没有匹配槽位就没有身份，不能退回其他标签账号。
3. Account只核标准Cookie、真实会话及CSRF；登录结果的Set-Cookie由`scopedResponseHeaders`添加槽位后缀，同时保留安全、过期和删除属性。
4. 图片及下载只在允许的GET路径带`session_context`，Edge验证后移除再转发。WSS用本标签HTTP获得的一次性OPEN票据，不把槽位选择器当身份令牌。

头与查询选择器冲突、重复参数或重复同槽Cookie应拒绝。无选择器的旧API调用继续使用无后缀Cookie，不自动选任何标签账号；当前新登录最多保留16个已有Cookie槽，关闭标签不等于登出。练习时两标签主动登录A/B，刷新各自权限；A退出后B仍可用；不存在槽位不能回落到其他账号。现有反例在`tests/unit/browser-session-context.test.mjs`和`tests/browser/tab-accounts.test.mjs`。

## 异步操作怎样联调

创建、审核、退出、回收等请求可能返回202/原operation与poll_url。观察原号状态，确定SUCCEEDED/REJECTED/CANCELLED后再展示结果；网络失败可能已经提交，重试必须保留原号和固定输入，不能为了消除页面提示每次生成新号。

http.ts已经有原号观察与待确认恢复逻辑，直接复用。你主要确认后端是否返回正确状态、稳定错误码和原轮次/版本，是否真的将业务和回执提交；不要改前端把所有202或503都显示为成功。

## WSS怎样看

先看edit-sessions的HTTP结果，再看Network中的WebSocket `/collaboration`。客户端校验返回地址属于当前同源入口，握手后发OPEN：protocol、ticket、client_session_id、connection_id、workspace_epoch。

观察SYNC、UPDATE、SAVED、FLUSH等原消息与身份字段。看到UPDATE表示消息到达，不证明自己的更新已经持久保存；SAVED和FLUSH仍必须匹配原客户端/更新/epoch。你无需重写DraftSocket或编辑器，只需要理解后端为何在提交后返回这些确认。

不要把ticket、Cookie、CSRF和整篇私有正文复制到公开日志/截图。WSS地址不是可长期复用的授权，重新进入工作区仍要当前合法票据。

## 常见联调问题

当前工作台使用固定侧栏与单一内容区，知识库设置按基本信息/成员/权限分区，新建/加入为短弹窗，编辑工具为保留实例的紧凑浮层；知识库使用卡片，文档阅读按正文/协作/审核/评论/版本/设置分开，编辑在当前标签切换页面工作区，刷新可恢复。评论、图片、版本和后台错误的当前对应关系见[产品联调更新课](continuous/reference/page-acceptance.md)；不需要重写现成前端。

工作台先点“个人空间／组织空间”，组织空间下再从“组织”选择具体名称。中间位置、目录和新建目标应对应同一空间；个人文档“协作申请”只出现在个人空间下。有后页时点“更多组织空间”，不能把当前页未加载到误报为没有组织。后端复用原`Space.kind`与`space_id`接口，切换先清旧目录；刷新按地址重读当前资格。组织详情的内容区已固定在选定组织，不再重复提供全局空间切换。

顶部“更多 → 账号设置”依次显示账号信息、修改密码、修改邮箱，桌面和手机均为单列。布局改动已包含在当前前端支持包中，后端继续提供账号课的同一接口。若仍看到两列，按下表“页面还是旧代码”检查构建和实例入口。

通知当前保留9类事件的基础“打开相关内容”与“返回通知”。刷新后恢复目标、精准定位讨论、进入当前审核处理和返回列表位置等增强，已按2026-10-11用户决定暂缓；不要将其误判为自己的后端漏写。现状与边界见[通知入口说明](continuous/reference/page-acceptance.md)。

| 现象 | 先检查 |
| --- | --- |
| 页面存在但接口404/501 | 后端该阶段是否实现该功能、路由是否注册；未完成装配时不要用占位成功冒充真实登录 |
| 401 | Cookie是否属于当前入口和会话、账号/凭据代次、服务端真实会话状态 |
| 403/ORIGIN_FORBIDDEN | 当前HTTPS入口与后端Origin是否一致，不混不同学习实例端口 |
| 403/CSRF_INVALID | token与Cookie是否来自同一次登录，前端是否沿现成http.ts传输 |
| 请求503 | 所属服务READY、依赖与原任务状态；不要先把前端缓存用户清掉当修复 |
| WSS握手后关闭 | edit-sessions资格、票据期限/绑定、协议字段与当前epoch，不能直接手造票据 |
| 页面还是旧代码 | 是否在当前项目根重新构建web/dist，是否打开正确实例入口 |

前端构建和真实页面检查保留用于证明联调；学习重点仍是后端实现和原因。完整流程见[演示](demo.md)，后端带写规则见[连续带写课程](continuous/README.md)。
