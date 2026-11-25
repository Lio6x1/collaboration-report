# 项目是什么，为什么值得学习

这是一个面向团队的协作报告和知识文档系统。用户建立组织与知识库，按权限共同编辑富文本，保存主线或个人支线，再固定送审、审核合并、正式发布和搜索。图片、批注、@通知、导出、回收站、成员退出和账号注销贯穿同一套权限与数据生命周期。

课程代号见[39课目录](continuous/lessons/README.md)。

## 你能展示的功能

| 用户需求 | 已实施的行为 | 对应章节 |
| --- | --- | --- |
| 团队协作 | 注册登录、组织申请审批、空间/KB/文档授权 | A01～A03、D01～D02 |
| 共同写作 | 富文本、中文输入、图片、多人同步、可靠保存、本机输入恢复 | C01～C02、D08 |
| 质量控制 | 固定版本送审、审核、合并、冲突处理、主动拉取主线 | D05～D06 |
| 正式交付 | 固定正式版、匿名阅读、真实消息索引、分页检索、重建 | D07、S01～S05 |
| 权限失效 | 退出、改凭据、撤权、自然到期时关闭原输出 | A02、C03、D11 |
| 持续维护 | 讨论通知、回收私有恢复、永久清理、转让、组织/账号关闭 | A05～A07、D09、D12～D13 |
| 下载与运维 | DOCX/PDF/Markdown包、OSS、发行升级、冷备份与隔离恢复 | D10、06联调及reference运维 |

本机完整产品验收与实际故障修复见[交付台账](../delivery/CURRENT-REVIEW.md)。正常完整Linux Docker发行已在隔离实例实际验证，见[Linux说明](../operations/LINUX.md)；用户服务器在自己的学习版本本机正常后部署，隔离验证不替代实际服务器验收。

## 技术各自做什么

| 技术 | 在项目中的职责 |
| --- | --- |
| Go、go-zero、database/sql及go-zero sqlx | Account、Doc、Search服务；显式SQL、事务、持久任务和领域规则。这里的sqlx是go-zero的包，不是jmoiron/sqlx |
| Node.js、mysql2 | 第四个实时协作服务；操作协作私有库。mysql2是Node数据库驱动，与Go数据库包分开 |
| gRPC、Protobuf、mTLS | 内部契约和真实服务身份，方法权限、调用期限与未知结果 |
| Vue、TypeScript、Vite、Element Plus | 工作台、审核、搜索和生命周期页面，按features组织 |
| ProseMirror、Yjs、WSS、IndexedDB | 富文本结构、协作增量、加密WebSocket运输、本机待保存输入 |
| MySQL、Redis | 四个私有库的权威事实；Redis承担登录限流等辅助能力 |
| Canal、Kafka、Elasticsearch | Doc outbox经真实消息进入搜索投影，消费回执、投影水位与索引重建 |
| 阿里云OSS官方Go SDK、本地存储策略 | 资源字节、完整性校验、引用保护和备份恢复 |
| Pandoc、Playwright/浏览器 | 固定内容的Word、Markdown资源包和PDF转换 |
| Docker Compose、PowerShell、Node运维脚本 | 本机中间件、环境归属、初始化、构建、停止、发行与备份恢复 |

具体版本以各go.mod、package-lock和Compose镜像摘要为准。源码分层见[代码组织](../architecture/CODE-ORGANIZATION.md)：Go的handler/server→logic→domain与repository/client，bootstrap/svc装配，worker调度；Node分transport/application/domain/repository/worker；Vue按功能划分视图、composable和API适配。

## 超出普通CRUD的地方

**保存有可证明的边界。** 本机显示文字、WebSocket发送成功、Yjs内存合并都不能证明数据库保存。SAVED依赖候选校验、资源保护和协作库提交；FLUSH还绑定原客户端更新依赖。你需要解释每个阶段失败后下一次如何继续。

**权限会随操作变化。** 获得过一次读取资格，不代表可以无限继续输出。撤权先关闭准入，再等待真实HTTP/WSS输出结束，最后确认原关闭号。主线、审核历史、公开版本和图片各有固定上下文与当前资格。

**跨服务用持久原号处理未知结果。** 四个私有库没有一笔跨库SQL事务。请求超时可能发生在提交之后；原operation、固定摘要、任务与回执让恢复继续观察原操作，而不是重新创建一个相似请求。

**正式版本和搜索有独立事实。** 编辑后的主线不会悄悄替换旧正式版。搜索由真实Canal/Kafka/ES链路生成，不直接往ES写假结果；索引重建需要固定边界与查询代次。

**资源和任务完整收尾。** 引用未释放的图片不能按年龄直接删除；导出固定输入并继续受权限约束；恢复验证包含真实字节、原任务、消费水位和登录。上述内容有实际产品验收证据，不能仅靠组件数量证明项目成熟。

它仍是校招项目，实际使用规模、线上运行时长和服务器性能需如实陈述。适合在面试里深入讲两三条你亲自掌握的关键链路，少用未经验证的“高并发”“生产级”标签。
