# F02 先把数据归属、约束和迁移写清楚

## 这节课要实现什么

注册账号、保存文档和处理审核都需要长期保存数据。本课设计并亲写数据库结构，让这些功能有明确的数据归属和约束，服务重启后仍能找到原记录。

## 做完后能看到什么

完成后得到四个服务各自的表和连续迁移文件：邮箱唯一、会话归属、正文版本、审核关系和后台任务都有持久位置。此时只完成SQL材料，不会创建真实用户或修改体验数据库。

## 先想清楚怎样实现

1. 先划分四个服务各自负责的数据。
2. 再按账号、组织、文档、协作和搜索建立表与约束。
3. 最后补迁移记录、服务授权及空库回退规则。

## 怎样判断这一课完成

本课核对SQL顺序与完整性；在06自己的实例安装迁移，再通过注册、保存等接口验证约束，不把文件写完当成数据库已安装成功。

## 开始写之前

本课亲手编写 SQL，不连接现成体验数据库。四服务分别拥有私有库：Account 管账号组织，Document 管文档权威事实，Collaboration 管协作持久数据，Search 管消费进度和重建。跨服务通过契约调用，不通过跨库 JOIN 绕过权限。

## 1. 先创建迁移目录，写最早的真实模型

```powershell
# 工作目录：自己的同一个学习工程。
New-Item -ItemType Directory -Path 'db/migrations' -Force | Out-Null
```

先按 [001.up.sql 分段操作](../../write/005-001.up.md) 写文件头、UTC 设置、account_db 和 schema_migration，再逐张写 user、session 等表。每个操作段是完整 SQL 语句，紧接上段追加。此时没有 INSERT 真实账号，不生成可公开的密码。

以 `user` 为例，字段要解决的事情是：

| 字段/约束 | 为什么需要 |
| --- | --- |
| id 主键 | 内部稳定关联，不随昵称或邮箱变化。界面补零显示不改变数据库标识。 |
| email、email_key | 原邮箱与规范化唯一索引分开；`UNIQUE user_email(email_key)` 才能挡住两个并发注册，前端“查过没有”不能替代。 |
| display_name | 展示昵称，组织所有者和协作者显示此值，不拿 ID 代替人名。 |
| password_hash、password_algorithm | 保存不可逆密码摘要及算法，绝不保存明文。 |
| state | PENDING/ACTIVE/REVOKING/CLOSED 是持久状态，关闭流程进行中不能继续承接新权限。 |
| credential_epoch | 密码/凭据资格变化后识别旧会话；仅检查用户仍存在不够。 |
| identity_epoch | 账号身份本身的代次，与某次密码更新的范围不同。 |
| email_verified_at、closed_at、created_at | 验证、关闭、创建的不同事实，不用同一个更新时间含混替代。 |

随后写 session。`id` 为随机 session token 的 32 字节摘要，不是浏览器拿到的原 token；`csrf_hash` 对应独立 CSRF token。`user_id` 指向账号，`credential_epoch` 绑定登录时的凭据代次，`epoch` 是会话自身资格代次。`state` 控制准入，expires_at 控制期限，last_seen_at 与 created_at 分别描述使用及创建。用户关闭旧凭据后，不能因为 expires_at 尚未到就接受旧会话。

## 2. 再写组织和文档的业务约束

`organization.owner_id` 是组织所有者；`organization_member.role` 只有 MEMBER/ADMIN，不能把 OWNER 当作第三种成员角色硬写进去。owner 独立存在，所有权交接和成员角色变更是两种操作。organization 的 revision/member_revision/event_seq 分别用于对象修改、成员事实和事件顺序，不能看到一个数字就都当乐观锁版本。

接着按 001 的顺序写文档库、协作库和搜索库。不要把每一张表理解成独立页面：固定正文、固定清单、审核、发布、输出登记、门闩、持久任务等共同组成一个可恢复的业务过程。SQL 的 CHECK 只约束一行的合法状态；“操作者有没有权限”仍要由业务层在事务和固定事实中核验。

这里用删除反例理解持久任务：操作已提交，进程在发 RPC 前退出。如果只有内存 goroutine，重启后就没人继续；durable_task 把原操作号、状态和恢复所需事实落库，worker 可以接着做。保存任务不代表已经完成远端动作，后面要按回执确认。

### 2.1 跟着一张图片理解资源为什么分成三张表

设用户7在空间10上传一张4096字节PNG，随后把它放入文档30的正文，最后导出这份文档。这是业务推演，不是在本课执行上传。先读下面的解释，再沿[001操作页](../../write/005-001.up.md)原顺序写各张表和末尾外键；不要把解释里的示例行额外INSERT进迁移。

第一步只回答“这个物理对象是谁”：`resource`保存对象身份、完整性和上传生命周期。它还不能证明图片被哪份正文使用，更不能证明浏览器当前有权读取。

| resource字段或约束 | 在这次上传中的含义与原因 |
| --- | --- |
| id、space_id、uploader_id | id是图片的稳定业务编号，例如50；space_id=10记录原资源空间，uploader_id=7记录上传人且允许NULL。后续转移计费空间使用010的独立分摊表，不覆盖原资源归属。 |
| storage_key、UNIQUE resource_storage | 受控对象键定位实际字节，不是公开URL。唯一键防两个资源行误认同一物理键；对象访问仍经业务授权。 |
| checksum BINARY(32)、byte_size、media_type | SHA-256原始32字节、4096字节长度和image/png描述应校验的对象。checksum在001允许NULL是表结构兼容边界，当前读取实现仍要求完整32字节，不能据此让新上传跳过校验。 |
| state、revision、created_at | CHECK只允许UPLOADING/READY/DELETING/DELETED/UNKNOWN，revision标识资源事实变化。READY必须有实际完整字节证据；UNKNOWN不能解释成已删除，created_at也不是删除成功证据。 |
| upload_operation_id、UNIQUE resource_upload | 绑定原上传动作。回复丢失后用同号恢复资源50，不能生成资源51重复占额。同号是否换了请求仍由业务指纹检查，唯一键只挡重复行。 |
| INDEX resource_state(space_id,state,id) | 按一个空间和状态有序定位资源，支持清理/恢复扫描；索引不代替可用状态或权限核验。 |

上传开始还要在同一事务锁`quota_account(space_id)`，检查limit_bytes、used_bytes、reserved_bytes，再写以operation_id为主键的`quota_reservation`。4096字节先是RESERVED；实际存储完整确认后才COMMITTED并结算。quota_reservation的expires_at只是恢复/过期判断输入，不能证明未知上传没有写入。空间used_bytes还含010记录的正文容量，所以不能断言“上传后空间总额恰好4096”。完整业务到[D08资源与存储](../document/08-resources-storage.md)带写。

### 2.2 图片写进正文前，先保存保护意图，再确认实际引用

第二步回答“谁还需要这张图片”：`resource_reference`是一对多的引用关系。资源50可能同时被当前草稿、固定正文版本和导出任务使用；只在resource行上放一个document_id或一个引用计数，会丢掉每个占用者的身份，无法恢复原操作或准确释放其中一份。

| resource_reference字段或约束 | 为什么这样建模 |
| --- | --- |
| resource_id、document_id | 前者指向资源50，后者在有文档上下文时为30，允许NULL的通用引用仍必须满足具体业务规则。001末尾分别添加资源/文档外键，均为RESTRICT；不能直接删父行并让引用悄悄消失。 |
| owner_kind、owner_ref、purpose | 分别说明占用者类型、准确身份和用途。比如OPERATION/原保护号/DRAFT_UPDATE、DRAFT/房间身份/ROOM_DRAFT、VERSION/固定版本号/CONTENT。owner_kind的CHECK只允许DDL列出的八类；owner_ref/purpose的真实合法形状由业务校验。 |
| PRIMARY KEY(resource_id,owner_kind,owner_ref,purpose) | 同一占用事实只保存一次，原号重试不会无限增加引用；不同VERSION或EXPORT仍可各有一行。generation不在主键中，所以推进同一占用者的代次是更新原事实，不是多算一个占用者。 |
| generation、created_at | generation绑定占用者的处理代次，旧回复不能确认新代次；时间只记录创建，不授权按年龄忽略活跃引用。 |
| resource_owner、resource_document索引 | 前者按占用者找待释放资源，后者按文档找资源集合。一次导出结束只释放自己的EXPORT引用，不能删同资源的VERSION引用。 |

但Document记下引用与Collaboration保存正文不在一个数据库事务里。为避免“图片还在跨服务保存途中，清理器却先删掉它”，第三张`resource_protection`保存一次正文UPDATE的保护协议：

| resource_protection字段或约束 | 在原UPDATE中的作用 |
| --- | --- |
| operation_id主键、request_fingerprint | 原保护号只能绑定同一请求摘要；同号换资源集合/目标应冲突，不能覆盖成新保护。 |
| document_id、room_id、grant_id、workspace_epoch | 绑定文档30、实际协作房间、编辑准入及工作区代次。room_id是跨服务身份，001仅为document_id建立文档外键，没有跨库room外键；必须在协议中验证。 |
| generation、resource_ids、source_epochs | generation是本次恢复代次；两个JSON分别保存确切资源集合和授权来源代次。JSON列只能保证合法JSON，不能自动保证数组完整、资源READY或来源当前有效。 |
| state | CHECK允许PROTECTED/CONFIRMED/RELEASED/UNKNOWN。PROTECTED代表已防止资源被提前清理，不代表Node正文已落库；未知结果仍需沿原操作确认。 |
| node_receipt_ref、durable_seq | 两列允许NULL，因为发起时还没有持久确认。后来保存原Node回执和可靠序号，确认必须匹配原房间/epoch/操作，不能填一个最新seq当作原UPDATE已保存。 |
| INDEX protection_room(document_id,room_id,state)、created_at | 索引用于定位该文档房间的保护状态；创建时间帮助定位，不能独自充当清理/释放证明。 |

顺序因此是：保护事务保存原意图及OPERATION引用→Node保存UPDATE和原确认事实→Document验证回执，按实际结果维护DRAFT引用并释放临时保护。若Node写入后回复丢失，保留保护并查询原结果；不能因为超时就移除OPERATION引用。送审固定版本时再建立VERSION引用，之后草稿删图也不能删除被原固定版本使用的物理图片。保护、确认、引用迁移的SQL与恢复流程在[D08](../document/08-resources-storage.md)，可靠UPDATE在[C02](../collaboration/02-admission-save.md)。

### 2.3 固定导出另存作业与临时预算，不再上传一份用户图片

第三步：用户7导出文档30的固定manifest80为PDF。`export_job`保存“做哪个固定任务”，不保存正在变化的编辑器内容。队列领取后进程可能退出，因此HTTP返回任务号与转换完成必须分开。

| export_job字段或约束 | 任务中的具体含义 |
| --- | --- |
| id、operation_id、UNIQUE export_operation | id是作业身份，operation_id绑定原请求。重复同号取得原任务；换格式/固定输入的同号请求由指纹拒绝。 |
| requester_id、document_id、manifest_id、release_id | requester_id记录申请者且允许NULL；document_id=30、manifest_id=80固定正文清单，公开导出另绑定release_id。001末尾的(manifest_id,document_id)及(release_id,document_id)复合外键防止引用另一文档的清单/正式版；NULL/非NULL的具体业务组合仍要校验。 |
| target_kind、format及CHECK | 目标仅MAIN/PUBLIC，格式仅DOCX/MARKDOWN/PDF。不能用任意URL或任意转换器参数作为导出目标。 |
| state、generation | 状态仅QUEUED/RUNNING/SUCCEEDED/REJECTED/CANCELLED。generation识别领取/恢复代次，旧worker不能提交覆盖新代次的结果；RUNNING不等于文件已完成。 |
| retry_at、last_error、INDEX export_due(state,retry_at,id) | 有界恢复按状态与到期时间取候选，记录稳定错误码。retry_at为NULL也可能可领取；索引本身不会替worker领取行锁。 |
| artifact_resource_id、created_at | 001保留可空的制品资源外键及创建时间；当前导出实际制品由008的export_control独立记账，不能把这列非空当作当前完成条件。 |

排队事务还按manifest80内固定VERSION引用建立`owner_kind=EXPORT, owner_ref=作业ID, purpose=CONTENT`的资源引用。因此导出等待期间图片50仍被准确占用；不得从当前草稿临时寻找图片或使用外部公开URL补图。释放该作业的EXPORT引用也不影响仍存在的VERSION/RELEASE引用。

现在区分两种“额度”。用户空间的quota_account控制持久正文与资源；转换中的临时文件和保留期制品由导出预算控制，不应扣成用户又上传了一张图片。001的`temporary_budget`只有kind主键、limit_bytes、reserved_bytes、revision和created_at，按kind描述临时预算；它没有used_bytes，也没有“reserved不超过limit”的CHECK。**当前导出实现实际锁的是008新增的export_budget，而非temporary_budget。** 保留001原字节，再按编号写008/012，不能把旧表改成新表来省迁移。

008的`export_control`以job_id为主键，fingerprint绑定原请求，snapshot保存固定输入；reserved_bytes保存本作业预留，artifact_key/size/checksum记录实际制品，expires_at限定保留期。`export_budget`以id为主键且CHECK(id=1)，用同一行的reserved_bytes/used_bytes统一计算全池额度。008原CHECK各限256MiB，012扩到2GiB；DDL分别限制两个字段，实际“已用+预留+新增不能超过配置预算”仍由持锁业务计算。不能仅看两个字段各未超上限就准入。

当前每个worker对应256MiB总预算，作业转换预留192MiB，最终制品最大32MiB，从登记起一小时的保留期内占used。转换确认成功后在原任务代次下把预留转为真实制品用量；失败或取消仍要确认转换进程退出与原文件处理结果，不能直接归零预算。具体实现、参数和真实池测试见[D10固定导出](../document/10-exports.md)。

### 2.4 最后清理：先逐项解除占用，再证明物理删除

继续同一例子：作业在原代次下落定终态时释放自己的EXPORT引用；成功制品仍在export_control中占used预算。之后PDF到期，清理器在受控退出/文件处理边界下清原制品并释放剩余额度。资源50若仍被manifest80的VERSION引用使用，图片必须保留。以后相关固定版本也被合法清理、其他草稿/保护/导出都不再占用，才进入资源删除和配额回收流程。外键RESTRICT阻止关系尚在时随意删父行，业务恢复协议则覆盖数据库之外的真实对象。

若对象删除请求超时，UNKNOWN仍可能表示“已删但回复丢失”或“尚未删”。保留原键与任务核实；只有确切物理结果与原资源/分摊账目一致，才能落定删除及退款。数据库一条DELETE成功、保护租约到期或HTTP202都不能替代这份证明。回到001写表时，逐项回答：这一列保存身份、固定输入、占用关系、状态还是确认结果；每个唯一键和索引分别防什么重复、支持什么查询。这个上传→引用→导出→清理过程在[D08](../document/08-resources-storage.md)/[D10](../document/10-exports.md)继续落成完整代码，本课还不连接真实数据库。

## 3. 逐份编写增量迁移，保持原编号字节

001 写完再顺序写 [002](../../write/006-002.up.md)、[003](../../write/007-003.up.md)、[004](../../write/008-004.up.md)、[005](../../write/009-005.up.md)、[006](../../write/010-006.up.md)、[007](../../write/011-007.up.md)、[008](../../write/012-008.up.md)、[009](../../write/013-009.up.md)、[010](../../write/014-010.up.md)、[011](../../write/015-011.up.md)、[012](../../write/016-012.up.md)、[013](../../write/411-013.up.md)。每份都继续在原 `db/migrations`，不改已经安装过的旧迁移来“修正当前表”。

具体列、索引、约束在各操作段中先出现，再被账号/文档/协作/搜索课的查询使用。阅读时把 SQL 的唯一键与后续 repository 的冲突处理对应起来：唯一约束处理并发，不只服务一次正常请求。

012 是一个可完整推演的小例子：导出池最多 8 个 worker，每个临时预算 256MiB，因此硬上限从 008 的单 worker 范围扩展到 2147483648 字节。它只替换 export_budget 的一个 CHECK；实际并发准入仍锁同一预算行计算，不能以“已经加了线程池”就绕过共享磁盘预算。

<!-- lesson-code: db/migrations/012.up.sql :: document_db -->
````sql
-- 008的单worker预算约束保持原字节；这里仅扩大可配置工作池的硬上限。
-- 实际准入仍由同一预算行锁按256MiB*workers核算，最大8个worker=2GiB。
USE document_db;

````
<!-- /lesson-code -->

上面的 USE 只选择库，不改变约束。下一段操作页中的 ALTER TABLE 才修改 CHECK。区分这两步，排查时才能判断是连错库，还是原 CHECK 尚未安装。

按[013 分段操作](../../write/411-013.up.md)创建管理归档表。它只标记失效邀请和已处理申请从管理列表移除，不删除审批事实或权限；创建后还须在service-grants.sql给Document应用SELECT、INSERT。后续[D01](../document/01-identity-access.md)将实现同事务revision核验和归档操作。

## 4. 最后写授权和空库回退文件

按 [service-grants.sql](../../write/017-service-grants.md) 编写逐表权限。迁移管理员和应用账号分开，应用不能自行改 schema；服务也不能读取别人的私有表。文件中的公开授权模板不是体验实例的凭据。

按 [001.empty-only.down.sql](../../write/004-001.empty-only.down.md) 写空库限定回退。名字中的 empty-only 是运行条件；它不是有数据时一键降级的方案，也不是遇到测试错误就应运行的清库命令。本主学习路线正常推进不执行 down。

## 5. 接线、检查和排错

SQL 不挂 HTTP 路由。06 中自己的环境准备工具按编号安装迁移、记录每个文件摘要，应用通过 Config 的 ExpectedSQLSHA/ExpectedAdditionalMigrations 与实际 schema_migration 做精确比较。F03 现在就编写这个比较器。

本课先确认 15 个 SQL 文件都已写入并核对 SQL 语句顺序，不在体验实例执行任何语句。真正的安装验收在 [06](../../06-integration.md)：独立 owner 的 MySQL、001—013、各私有库授权、四服务 READY 必须一起成立。只看到 CREATE TABLE 成功不能证明后续迁移已经安装。

安装时报 duplicate table，先判断是否对同一库重复应用 001，保留迁移记录排查；出现 SCHEMA_VERSION_MISMATCH，比较缺失版本/摘要，不篡改数据库中的 checksum 来让健康检查变绿；报权限不足，检查应用 DSN 是否错用了别的服务账号，不给应用追加全局 root。

继续 [F03 进程底座](03-runtime-and-security.md)。SQL 完整答案只在 [阶段索引](../../01-files.md) 核对。
