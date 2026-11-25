# 契约与生成代码

`BASELINE.json`固定2026-10-03定稿文件的SHA-256。`http/openapi.json`含117操作、121模型，`proto/internal.proto`含34业务RPC，内容与来源schema及5个字节样本在`content/`。

实施通知跳转时发现，批注通知只携带 threadId，原接口无法直接定位所属文档。
审核通知同样只携 requestId，不能从当前请求推断事件发生时的原轮次；
`GET /v1/notifications/{notificationId}/review-round` 仅向当前有权的原收件人返回保存的原轮次。
`http/read-extensions.json` 因此追加 `GET /v1/comment-threads/{threadId}`，复用既有
`CommentThread`/`Error` 模型，只返回经当前权限和原上下文校验的线程元数据。
原 `BASELINE.json` 与冻结 OpenAPI 字节保持不变；`scripts/load-http-contract.mjs`
将只读扩展合并给生成器及响应校验，拒绝覆盖已有 path/method、重复操作名、路径参数
别名与未知模型引用。新响应模型仅按明确白名单追加，不改变既有通知或批注写入契约。

完整账号注销需要当前身份代次，但冻结User响应原先只含凭据代次。只读扩展的
`response_fields.User.identity_epoch`因此补充`/v1/account/me`实际身份水位；两种代次
不可互相代替。加载器只接受明确白名单中的只读字段，拒绝覆盖原模型字段及任意模型
改写；生成器和真实响应校验共同读取扩展。`openapi.json`及`BASELINE.json`原字节保持不变，
原注销POST/GET请求和返回计划沿冻结契约实现。

留存治理追加只读 `GET /v1/retained-public/documents/{documentId}`，复用 `Document`/`Error`，
让治理页面取得当前控制修订，不能借此读取已关闭正文。原撤下/删除仍使用冻结
`VersionedAction`，只含 `operation_id`/`expected_revision`，不接受额外reason或确认字段；
永久删除的二次确认由页面提供。

冻结内部RPC未单列平台角色枚举。已有 `AcquireIdentity` 的 purpose/Target 和来源代次可以
表达附加资格：`RETAINED_GOVERNANCE` 必须先是本人 `ACTIVE_ACCOUNT`，Account再从自身
私有库核验当前独立平台角色，追加 `platform-admin:<actorId>` 来源。Acquire和Revalidate
都检查这一事实，普通活跃证明和组织角色不能替代。HTTP/内部客户端仍使用原RPC类型，
没有改写 `internal.proto` 或 `BASELINE.json`。任免和真实撤权见
[平台治理运维](../docs/operations/PLATFORM-GOVERNANCE.md)。

个人文档协作者姓名复用既有 `IdentityRevalidation` 和 `DocumentCollaborator` DTO，
通过专用内部 HTTP `POST /infra/identity/document-collaborator` 适配，不改冻结 proto 或公开
HTTP schema。该入口只允许经 CA 核验的 `document` 服务证书；Doc 先核验文档管理权限、
从实际协作关系按用户 ID 分页选出候选，再提交该候选原 `DOCUMENT_COLLABORATORS` 准入绑定。
Account 在原会话、目标身份来源和 gate 重验的同一事务中读取活跃账号 `display_name`，
只返回用户 ID 和姓名；它不建立浏览器用户目录，也不返回邮箱。最终响应仍登记管理 ACL
和每个目标的身份代次，使撤权或注销能够关闭在途输出。

`GET /v1/documents/{documentId}/mention-candidates`复用这一最小分页响应，明确要求当前文档
`COMMENT`资格，用于按姓名选择提及目标。它与仍要求`DOCUMENT_MANAGEMENT`的`/collaborators`
是两个独立读取用途：游标绑定各自purpose、actor、文档和ACL修订，不互相续读。
个人候选仅取真实文档协作关系；组织候选复用当前可见成员目录。Account内部姓名适配只为
这些明确用途白名单重验原目标准入，不开放通用用户搜索。候选被选择不代表能收到通知，
原投递链路仍核验目标对评论原固定正文的当前读取资格。

审核请求列表与详情增加可选只读 `ReviewRequest.summary`，引用受限五字段模型
`ReviewRequestSummary`：`title`、`excerpt`、`round_no`、`submitter_id`、`submitted_at`
均为字符串。题名来自当前审核轮次固定结构中该单元的章节；摘要仅投影同轮固定正文的
可见纯文本，最多140个Unicode码点，截断时追加一个省略号。轮次编号、提交人及UTC提交时间
取自同一 `review_round`。读取在原审核准入事务中完成，不取MAIN或共同草稿，不表示差异，
也不为列表展开完整轮次、样式或资源。扩展不改变审核写流程或冻结合同字节。

个人跨空间转让需要接收方接受，冻结的组织负责人直接转交接口不包含接收动作。
`http/implementation-extensions.json` 因此追加六个个人转让入口和三个专用模型，原写入
接口及模型不覆盖；`scripts/http-implementation-extensions.mjs` 只接受这组明确的路径/
方法和模型名，拒绝旧路径覆盖、参数别名及未知引用。申请注册成功只代表已发送申请，
接受前归属不变；accept沿原号/真实关闭和容量预留完成迁移。执行
`node scripts/generate-personal-transfer-contract.mjs` 维护实施扩展，再执行通用生成器；
当前合并合同为128操作/127模型，原冻结合同仍为117操作/121模型。

执行`node scripts/generate-contracts.mjs`生成TS、Go HTTP DTO、入口路由与Go/Node共用RPC调用方策略。Go proto用以下命令生成（本轮实际protoc 5.29.3、protoc-gen-go 1.28.1、protoc-gen-go-grpc 1.2.0，需安装到PATH；运行库分别为protobuf-go 1.36.12、grpc-go 1.83.2）：

```powershell
protoc --proto_path=contracts/proto --go_out=contracts --go_opt=module=collaboration/contracts --go-grpc_out=contracts --go-grpc_opt=module=collaboration/contracts --descriptor_set_out=contracts/proto/internal.descriptor.pb contracts/proto/internal.proto
```

自动生成的DTO只定义传输形状，不代替请求校验或权限检查。Go中无法直接表达的oneOf/anyOf/allOf保留为`json.RawMessage`，业务实现必须按固定schema进一步核验，不能直接信任它。HTTP uint64以十进制字符串传输，Node proto-loader使用`longs:String`，真实Node→Go字节测试覆盖uint64最大值。

阅读时先看OpenAPI中的目标操作、proto中的方法，再看手写handler和测试；无需逐字重敲自动生成的121模型与RPC stub。

Go 固定正文校验直接在 `content/` 嵌入原 content/provenance schema，使用锁定的 `github.com/santhosh-tekuri/jsonschema/v6 v6.0.3` 编译验证，不复制一份规则，也不读取远程 schema。编译器、嵌入资源 loader 与 Validate 的 API 已依据[该锁定版本官方文档](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6@v6.0.3)核对。手写严格 JSON 解码拒绝非法 Unicode、重复对象键与越界数字，规范编码沿用 UTF-16 键排序；五个既定 Node/Go 字节样本继续逐字、大小和 SHA-256 核对。

个人文档分享管理通过严格白名单`sharing_management: true`追加筛选和归档协议：邀请支持ACTIVE/INACTIVE/ALL，申请支持PENDING/PROCESSED/ALL；服务端先筛选再倒序分页，游标绑定筛选值，UI每页20条。Invitation/Application追加可选created_at，申请追加可选applicant_display_name。姓名只接受新的DOCUMENT_APPLICATION_NAMES用途及原文档、目标、身份来源绑定，不是通用账号查询。

`POST /v1/document-invitations/{invitationId}/archive`和`POST /v1/document-applications/{applicationId}/archive`复用VersionedAction及Operation，仅允许当前个人文档管理者归档失效邀请或已处理申请。同一事务核验expected_revision，追加document_sharing_archive并推进原记录revision；审批事实、申请人自己的历史和已有授权保留。不存在与不可见记录统一403及拒绝码，依赖失败单独处理。冻结OpenAPI和BASELINE仍不改写，HTTP生成器与真实响应验证共用load-http-contract.mjs。
