# 本机与服务器部署

本页说明 Windows 单机开发、发行配置和升级。Ubuntu 24.04 的 Docker Compose 安装由 [Linux 部署](LINUX.md)说明。

## Windows单机发行与升级

Windows 部署使用 Docker Desktop 的 Linux 容器和四服务监督器。连接发信供应商时按[SMTP 设置](../../configs/README.md#smtp设置)配置 TLS 或 STARTTLS。域名、证书、邮件和 OSS 需在部署环境分别验证。

## 新机器准备

本机的MySQL、Redis、Kafka、Elasticsearch、Canal全部通过Docker Compose运行，无需分别安装Windows版本。下面的初始化脚本负责启动对应容器；Go、Node及前端开发工具在本机安装，用于构建和调试应用。数据库持久卷属于当前项目owner，停止或重启容器不能删除卷。

安装完整Node24.19.0、Go1.25.10、Docker Desktop、OpenSSL、Python及Edge。Docker镜像使用仓库已有摘要，npm用package-lock，Go构建使用各服务go.mod/go.sum和readonly；不要复制开发机口令、关闭高水位或用户数据来初始化空环境。导出需要Pandoc3.8，先确认`pandoc --version`首行为`pandoc 3.8`；Windows默认以已安装Edge作PDF转换，Playwright来自锁文件，无需另外下载Chromium。路径可含空格，下面均在项目根运行。

```powershell
$env:GOWORK = 'off'
./scripts/npm.ps1 ci
./scripts/npm.ps1 run build --workspace web
node --no-maglev scripts/dev-environment.mjs up
node --no-maglev scripts/login-environment.mjs
node --no-maglev scripts/search-environment.mjs
node --no-maglev scripts/start.mjs
```

首次初始化会创建本项目owner/随机秘密与逐表账号，安装不可变迁移，并配置Canal/Kafka/ES。任何摘要不符、半安装或身份未知都保留现场，不删卷绕过。查看监督器输出的HTTPS入口；四服务真实READY后才使用。自签开发CA只用于本机，公网证书不能通过关闭TLS验证或删除服务身份检查接入。

`.runtime`包含私密配置、证书、日志和业务对象，Git已经忽略；Windows运行账号应限制目录权限。新部署可由当前用户收紧权限，使用明确项目绝对路径，不能递归改其它工程：

```powershell
$taskPrivateRoot = Join-Path (Get-Location) '.runtime'
$taskWindowsIdentity = (& whoami).Trim()
icacls "$taskPrivateRoot" /inheritance:r /grant:r "${taskWindowsIdentity}:(OI)(CI)F"
```

备份/恢复新目录还会独立收紧ACL。不要在聊天、Git或报告中粘贴JSON私密配置、Cookie、密钥或完整Compose渲染输出。

## 选择发行配置

```powershell
node --no-maglev scripts/release.mjs check
node --no-maglev scripts/start.mjs stop
node --no-maglev scripts/release.mjs apply
node --no-maglev scripts/start.mjs
```

`check`核对固定镜像、五个依赖、本owner及loopback绑定，保存不含口令的源码/锁文件/迁移摘要。`apply`必须在应用停止后核验已有容器归属，持久选择`release.yml`层，沿原卷重新配置五个依赖。日志使用local驱动，每个容器20MiB×5轮换；依赖异常退出可重启，显式stop仍停止。应用保持监督器的绝对入口/run-owner核验，不按node.exe等进程名重启。依赖启动成功不等于应用READY，第二次start须真实确认。

只选配置而暂不启动依赖时用`release.mjs enable`。选择存在`.runtime/dev.json`的`deployment`中，后续compose/备份/恢复均沿用；不要手改旁边项目的Compose环境。既有MySQL2GiB/512MiB缓冲池、ES2GiB/1GiB堆和Kafka768MiB预算保留。CPU/正文工作集与总机器规格最终依据第八批真实容量结果，不以镜像上限求和充当承诺。

## 升级与回退

先记录发行摘要并做[一致冷备份](BACKUP-RESTORE.md)，再更新源码、执行锁定依赖安装和前端构建。逐条迁移只由安装器执行；已有迁移摘要不允许改写。先升级隔离恢复环境核对固定正文、图片、关闭来源、原号任务和消费水位，再升级原环境。不要在新版服务正在运行时覆盖旧二进制、配置或数据库文件。

失败时停止本owner，保留失败记录和卷。回退必须沿冷备份的新隔离目录/新卷恢复，不能只checkout旧源码却继续使用不兼容新库；验收成功后再由运维选择入口切换。当前工具不会自动接管旧入口、修改域名或公开部署。

## 输出与进程约束

导出采用持久队列和固定工作池。`exports.workers`省略、0或1时为单worker，允许1～8；当前2核2GiB服务器和本机体验默认均为1。提升后同时执行的任务最多为该Document进程配置的worker数，其余继续排队，前端操作方式相同。修改实际`.runtime/services/document.json`后按本项目监督器完整停止/启动服务组，Linux重新启动应用容器；不要只换Document的PID或并行启动不同配置的实例。

临时磁盘总预算为`256MiB × workers`，每项转换预留192MiB，制品最大32MiB，保留一小时期间继续占用预算。预算不足时等待原额度释放，不无限启动转换器；此数值不限制浏览器进程的实际内存。降低并发后保留原RUNNING及其预留，沿原任务顺序恢复，不能把仍在转换的任务标成已取消。012增量迁移把数据库硬上限扩大到2GiB，安装器核验原约束、新约束和四库回执；既有环境需先停止服务并按原安装流程追加迁移、更新绑定配置，不能改008摘要或手工跳过版本检查。

每个任务的OS排他锁覆盖转换器、临时目录与最终generation提交；多个worker跳过仍持锁的任务。文档排空只等待该文档的转换，过期制品回收仍串行。当前方案要求Document实例共享支持文件锁的本地目录，默认单机单Document进程；没有将它改成跨主机分布式转换集群。

Edge对业务正文/文件设置no-store，权限与最后输出资格仍由所属服务检查；前置代理必须保留同源HTTPS/WSS、完整流式生命周期，禁止缓存正文/图片/PDF或代替来源关闭ACK。独立公网反向代理尚未配置，不声明已验证其行为。

Windows转换程序通过`CREATE_NO_WINDOW`与`HideWindow`在后台启动，不弹出终端窗口；标准输出、错误及退出码仍由Document收集，Job Object继续管理转换进程树。当前[Linux完整发行](LINUX.md)把四服务和Edge置于同一应用容器PID空间，由前台监督器和Docker init管理：Document异常退出会停止整组，容器退出时原命名空间内的转换进程随之终结；原作业在新的启动标记下恢复。独立Linux实例已实际验证RUNNING导出期间终止Doc、应用容器重启、同一作业更高generation完成与PDF校验和/原号重放一致，见`artifacts/batch09-linux-export-recovery-02.log`。先前单独转换器PID1实验是历史窄范围证据，不能据此要求把完整服务拆到相互不可见的PID空间；也不能把本机Linux容器验收称作用户服务器已经部署。
