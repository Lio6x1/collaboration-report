# 协作报告技术说明

协作报告是一个采用微服务架构的章节级文档协作平台。团队可以共同编辑同一章节，将支线改动送审后合入主线，并把正式版本发布、检索和导出。浏览器通过同源 HTTPS 和 WebSocket 入口访问三个 Go 服务和一个 Node.js 服务。 [系统架构](../README.md#系统架构)

## 技术栈

| 范围 | 技术 | 在项目中的用途 |
| --- | --- | --- |
| Go 后端 | Go 1.25.10、go-zero、MySQL | 实现账号、文档和搜索服务。各自拥有私有数据库，由业务用例控制 SQL 事务。 |
| 实时协作 | Node.js 24、Yjs CRDT、WebSocket、MySQL | 合并同一工作区内的编辑增量，记录持久化日志，用检查点恢复房间。 |
| 浏览器 | Vue 3、TypeScript、Vite、ProseMirror、y-prosemirror、IndexedDB | 构建工作台和富文本编辑器。在本机保留尚未收到保存确认的编辑增量。 |
| 服务通信 | gRPC、Protocol Buffers、mTLS | 定义 Go 与 Node.js 的内部契约，验证服务身份并限制可调用方法。 |
| 账号与限流 | bcrypt、Redis Lua、SMTP、HttpOnly Cookie | 保存密码哈希，处理验证码、登录限流和会话。前端写请求携带 CSRF 校验信息。 |
| 全文搜索 | Canal、Kafka、Elasticsearch | 消费数据库变更并维护正式版本的全文索引。使用 PIT 和 `search_after` 续页。 |
| 资源与导出 | 本地文件、阿里云 OSS、Pandoc、Playwright/Chromium | 保存正文引用的附件。用固定版本生成 DOCX、Markdown 资源包和 PDF。 |
| 部署与验证 | Docker Compose、HTTPS、Go/Node 测试、Playwright | 分配服务依赖，运行本机和 Linux 实例，验证模型规则、接口和浏览器流程。 |

## 服务边界

| 模块 | 职责 |
| --- | --- |
| [Account](../services/account) | 账号、验证码、会话、组织成员和身份验证。 |
| [Document](../services/document) | 知识库、章节结构、权限、审核合并、发布、评论、资源和导出。 |
| [Collaboration](../services/collaboration) | 实时房间、Yjs 增量校验、持久保存、房间恢复和富文本合并计算。 |
| [Search](../services/search) | 消费发布变更、维护索引、查询候选及重建索引。 |

Edge 提供前端静态文件。账号和组织请求转给 Account，文档与正式版搜索请求转给 Document，编辑用 WebSocket 转给 Collaboration。四个服务按各自权限访问同一个 MySQL 实例中的私有库，内部 gRPC 调用通过 mTLS 验证服务身份。 [接口契约](../contracts/README.md)

- **编辑会话**。Document 校验章节编辑资格后签发票据。Collaboration 通过内部 RPC 兑换票据，建立绑定对应工作区的房间连接。
- **审核合并**。Document 管理审核状态和版本变更。Collaboration 基于固定输入计算富文本合并候选，Document 校验候选并更新主线。
- **正式版检索**。Search 消费发布事件，向 Document 读取正式版快照并构建可重建的索引。Document 执行搜索候选的最终资格检查。

Linux 默认部署在同一应用容器中统一启停四个服务和 Edge。Document 与 Collaboration 在共享 PID 命名空间中核验原输出进程是否结束，这是撤权与恢复流程的部署约束。 [Linux 部署拓扑](operations/LINUX.md#拓扑与持久事实)

## 核心流程

### 多人编辑与可靠保存

编辑器通过 Yjs CRDT 合并同一章节工作区的增量。本机先将待发增量写入 IndexedDB，服务端用房间串行队列校验候选正文和资源引用，在 MySQL 写入日志后再广播并确认保存。断线后按原更新编号重试，服务重启则用一致检查点和连续日志恢复。送审前固定已持久化的编辑边界。

### 章节版本与三方合并

主线和共同支线按章节维护。送审时固定正文、资源和样式。合并以共同基线、送审稿、目标主线为输入，用稳定内容块身份和文本差异定位变化，合并兼容修改并报告重叠或结构冲突。拉取主线、人工修复和历史反向变更都围绕固定输入生成候选，再经过各自的校验与安装流程。

### 发布、索引和权限

Document 在事务中登记正式版本与索引事件。Canal 将数据库变更送入 Kafka，Search 更新 Elasticsearch，并在持久记录处理结果后推进消费进度。查询时 Document 复核候选的当前发布状态和访问资格。撤权时按授权来源查找仍在读取、下载或连接的请求，等待实际输出关闭后确认完成。

### 幂等与后台任务

关键写入绑定操作编号、发起者和输入摘要，业务修改和回执在同一事务中提交。断连时查询原操作状态，关闭、导出等任务由后台继续推进。导出固定正文及资源，限制并发与临时磁盘用量，并校验最终下载资格。

## 代码与运行

- Go 按请求处理、业务用例、领域规则、数据访问和依赖装配分层。业务事务使用同一连接，跨服务调用通过显式客户端完成。
- Node.js 协作服务按接入、房间用例、领域规则、持久层和后台恢复组织。当前运行方式为单活动协作实例。
- Vue 页面按 [功能目录](../web/src/features)组织，复用生成的 API 类型和公共控件。富文本正文保留自己的文档格式。
- [项目 README](../README.md)提供本机运行和测试入口。[配置说明](../configs/README.md)列出公开示例。[Linux 部署](operations/LINUX.md)说明服务编排及升级步骤。
