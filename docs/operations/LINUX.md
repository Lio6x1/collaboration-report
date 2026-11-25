# Linux 单机完整 Docker 发行

本文说明在 Ubuntu 24.04 amd64 上使用 Docker Compose 部署协作报告，包括构建应用镜像、配置 HTTPS 入口、启动四个服务及依赖，以及检查目标实例的就绪状态。

## 已验证场景

2026-10-09 已在独立的 amd64 Linux Docker 实例中验证镜像构建、Compose 启动和整站流程，覆盖双人协作保存、固定审核合并、图片与通知、正式发布、Kafka 消费与索引、浏览器阅读、PDF 导出和回收恢复。

故障恢复验证在导出运行期间停止 Document 进程并触发应用容器重启。同一作业在新的执行代次完成，PDF 校验和与使用原操作号重放的结果一致。部署到目标主机后，按本页步骤检查实际实例。

## 拓扑与持久事实

[server.yml](../../deploy/compose/server.yml)运行五个独立中间件容器和一个应用容器。MySQL、Redis、Kafka、Elasticsearch、Canal 都使用仓库已锁定的镜像摘要，只有 Edge 的 HTTPS 端口映射到宿主机。

| 组件          | 应用内/容器网络地址               | 持久内容                                              |
| ------------- | --------------------------------- | ----------------------------------------------------- |
| Account       | HTTP 20601、RPC 20602，loopback   | Account 私有库、登录 Redis 状态                       |
| Document      | HTTP 20603、RPC 20604，loopback   | Document 私有库、资源、固定导出、输出恢复             |
| Search        | HTTP 20605、RPC 20606，loopback   | Search 私有库、索引代次、物理日志身份和消费回执       |
| Collaboration | HTTP 20607、RPC 20608，loopback   | Collaboration 私有库、Yjs 更新与检查点                |
| Edge          | 容器 8443，宿主端口首次准备时指定 | 输出关闭门闩；完整前端来自镜像                        |
| MySQL         | `mysql:3306`                      | `<project>_mysql-data`，含四私有库与 binlog           |
| Redis         | `redis:6379`                      | `<project>_redis-data`，AOF 与登录/验证码状态         |
| Kafka         | `kafka:29092`                     | `<project>_kafka-data`，原 KRaft 身份、消息与消费位置 |
| Elasticsearch | `http://es:9200`                  | `<project>_es-data`，索引与 alias                     |
| Canal         | 私有网络，不映射宿主端口          | `<project>_canal-position`，实际复制位置              |

四个业务服务仍有独立入口、配置、证书身份、私有库权限和任务生命周期。它们与 Edge 在同一个应用容器 PID 空间中运行，是为了让 Document 和 Collaboration 能用 `/proc` 核验原输出进程的绝对入口及启动标记。把各角色随意拆到不同 PID 空间后，“看不到旧 PID”不能证明旧输出已经结束，因此当前发行不能只改 Compose 数量就拆分部署。

应用容器启用 `init: true`；前台[监督器](../../scripts/server-start.mjs)登记每次实际启动的标记，转发 SIGTERM/SIGINT并等待子进程。必要子进程异常退出会停止整组；正常停止保留业务数据。中间件和应用使用 `unless-stopped`，容器日志按 10MiB × 3 轮换。

MySQL 开启 ROW/FULL binlog 和固定 server-id。Canal 只获复制权限及 `document_db.outbox` 的 SELECT，不能读取各角色私有表。Kafka 主题固定为 `collab-publications-v1`、3 个分区；topic 初始化只核验或首次创建，不替换原日志。Search 的索引、alias、物理日志绑定和恢复水位继续由原 Go bootstrap 负责，不在安装脚本中手写或前移 checkpoint。

## Windows 本机构建与打包

在 Windows Docker Desktop 中选择 **Linux containers**。宿主机运行准备和包装脚本需要 Node 24.19.0 与 OpenSSL；Go、Pandoc、Chromium 和前端依赖在镜像内按锁定版本构建，宿主机不必为了本镜像另外安装 Go 或转换工具。

下面在源码根目录运行。发行前先保存已验收的源码提交；`git archive HEAD`只打包该提交，若镜像来自未提交工作区而源码包来自 HEAD，两者会不一致。

```powershell
Set-Location 'D:\job\golang\projects\collaboration-report'
git status --short
git rev-parse HEAD
docker version
docker compose version
node --version
openssl version

docker build --platform linux/amd64 -f deploy/docker/application.Dockerfile -t collaboration-report:linux-development .
docker image inspect --format '{{.Os}}/{{.Architecture}}' collaboration-report:linux-development
```

构建失败时保留完整日志并定位具体失败阶段。成功的架构输出应为 `linux/amd64`。镜像内含 Account、Document、Search 二进制、Collaboration、Edge、完整前端，以及既有导出需要的 Pandoc 3.8、Playwright Chromium 和中文字体；[Dockerfile](../../deploy/docker/application.Dockerfile)固定构建依赖，[.dockerignore](../../.dockerignore)排除运行目录、用户数据、凭据和私钥。

把镜像和同一提交源码打包；这个目录与现有 Windows owner 的配置无关：

```powershell
$taskPackageDirectory = Join-Path $PWD '.runtime/server-package'
New-Item -ItemType Directory -Force -Path "$taskPackageDirectory" | Out-Null
docker save --output "$taskPackageDirectory/collaboration-report-linux-amd64.tar" collaboration-report:linux-development
git archive --format=tar --output "$taskPackageDirectory/source.tar" HEAD

$taskImageHash = (Get-FileHash "$taskPackageDirectory/collaboration-report-linux-amd64.tar" -Algorithm SHA256).Hash.ToLowerInvariant()
$taskSourceHash = (Get-FileHash "$taskPackageDirectory/source.tar" -Algorithm SHA256).Hash.ToLowerInvariant()
@("$taskImageHash  collaboration-report-linux-amd64.tar", "$taskSourceHash  source.tar") | Set-Content -Encoding ascii "$taskPackageDirectory/SHA256SUMS"
```

后续实际部署时，用用户选定的传输方式把这两个 tar 和 `SHA256SUMS`传到服务器。例如以下 Linux 步骤约定它们已经放在 `/srv/collab-import`。不传原 Windows `.runtime`，也不把本机生成的部署目录当跨机器可直接复用的配置。应用镜像包不包含五个中间件镜像，服务器首次启动需要能拉取 Compose 中的固定摘要；隔离网络环境需要另行准备完整依赖镜像分发，不能把单个应用 tar 当离线整站包。

## Ubuntu 24.04 前置环境

先核验机器架构为 `x86_64`。安装 Docker Engine 与 Compose 插件，使用[Docker 官方 Ubuntu 安装说明](https://docs.docker.com/engine/install/ubuntu/)及[Compose 插件说明](https://docs.docker.com/compose/install/linux/)；官方安装资料包含 Ubuntu 24.04。这里不替服务器执行安装或改网络规则。

宿主 Node 使用 [24.19.0 官方发行目录](https://nodejs.org/download/release/v24.19.0/)，应能从运维账号及 `sudo`环境找到 `node`。不要用 Ubuntu 默认仓库中版本不符的 Node 替代。宿主 OpenSSL 可由 Ubuntu 软件包提供；它仅用于首次准备证书。无需在服务器安装 Go、Pandoc 或 Chromium。

```bash
uname -m
node --version
openssl version
sudo docker version
sudo docker compose version
```

确认 `node --version`输出 `v24.19.0`，Docker daemon可用，`docker compose`可用。本文 Linux 命令使用 sudo，是为了保持私密部署目录只有 root 可读，并避免同时假设用户已经加入 Docker 组。若 Docker 或 Node 安装方式不同，先解决命令访问权限，不把私密目录改成全员可读。

核验传输包，解压到一个新的源码目录，再载入应用镜像：

```bash
cd /srv/collab-import
sha256sum -c SHA256SUMS
sudo install -d -m 0755 /opt/collaboration-report
sudo tar -xf source.tar -C /opt/collaboration-report
sudo docker load --input collaboration-report-linux-amd64.tar
sudo docker image inspect --format '{{.Os}}/{{.Architecture}}' collaboration-report:linux-development
cd /opt/collaboration-report
```

`/opt/collaboration-report`应是新目录。已有运行实例升级时，不直接把源码 tar覆盖在正在运行的目录上；先保留发行提交、镜像与一致备份，再按下文停止原 owner、替换经验证的新镜像，仍使用原私密配置和卷。当前 Dockerfile仅支持 amd64，不把 arm64机器当作已验证目标。

仅 Edge 的宿主 HTTPS 端口需要对体验者可达；MySQL、Redis、Kafka、ES 和 Canal没有公有映射。云安全组和服务器已有网络策略应与选定入口一致。Docker 的端口映射与宿主防火墙规则之间的关系按[官方 Ubuntu 安装说明的防火墙部分](https://docs.docker.com/engine/install/ubuntu/#firewall-limitations)核对，不仅依据 UFW 状态判断入口是否公开。

## 首次准备：先确定入口和私密目录

[server-prepare.mjs](../../scripts/server-prepare.mjs)只生成文件、随机秘密和证书，不启动 Docker、不迁移原 Windows配置。默认部署目录为源码根目录下的 `.runtime/server-deployment`；使用 `--directory`可准备独立部署。目标须为空，或包含本脚本已有的同一份 manifest；不能指向原 `.runtime`本身、源码根或其祖先目录。

本机 Linux 容器演示可以采用：

```powershell
Set-Location 'D:\job\golang\projects\collaboration-report'
node scripts/server-prepare.mjs --directory "$PWD/.runtime/server-deployment" --origin 'https://localhost:8443'
node scripts/server-compose.mjs config --directory "$PWD/.runtime/server-deployment"
```

这组命令准备另一个 owner，保留原 Windows产品数据和账号。Windows新目录可以单独收紧ACL，不递归修改原 `.runtime`：

```powershell
$taskServerPrivateDirectory = Join-Path $PWD '.runtime/server-deployment'
$taskWindowsIdentity = (& whoami).Trim()
icacls "$taskServerPrivateDirectory" /inheritance:r /grant:r "${taskWindowsIdentity}:(OI)(CI)F"
```

首次部署前确认最终使用的域名或地址，将示例域名替换为已经指向目标服务器的入口。

```bash
cd /opt/collaboration-report
sudo node scripts/server-prepare.mjs --directory /srv/collaboration-report/private --origin https://reports.example.com --https-port 443
sudo node scripts/server-compose.mjs config --directory /srv/collaboration-report/private
```

`--origin`必须是纯 HTTPS origin，不含路径、用户名、密码、查询或 fragment。省略 `--https-port`时采用 origin的端口；无显式端口时是443。应用内部 Edge始终是8443。正常直接映射时，浏览器 origin的端口应与宿主映射端口一致；只有经过另外验证的代理拓扑才可出现差异。

准备输出 `PREPARED`；以同一路径和参数重试会输出 `ALREADY_PREPARED`，保留原 owner、秘密和证书。已有 origin、端口或 SMTP参数冲突时返回 `SERVER_EXISTING_CONFIGURATION_CONFLICT`，不覆盖它们。当前没有改 origin、迁移部署目录、轮换证书或替换 owner的CLI。移动 Windows已准备目录到 Linux会因绝对目录绑定不一致而被拒绝。新服务器应首次生成自己的部署；既有数据迁移需另外设计并验证持久身份转换，不能只改 manifest绕过核验。

私密目录结构：

```text
/srv/collaboration-report/private/
  manifest.json              部署目录、owner/project、origin和准备状态
  compose.env                Compose路径/身份/端口/镜像名，不含口令
  application/               挂载到应用容器 /app/.runtime
    dev.json                 无口令的应用拓扑，root固定为 /app/
    services/                每个角色自己的DSN、证书路径和业务秘密
    pki/                     CA公钥及五个角色的证书/私钥
    resources/document/      本地资源、校验和共享锁事实
    exports/document/        固定导出和转换临时事实
    logs/                    各子进程日志
    initialized.json         初始化成功后生成
    startup.json             每次启动的状态和启动标记
    processes.json           每次启动的真实进程登记
    smtp/inbox/              开发收件器收件后生成
  admin/                     不挂载到application
    credentials.json         root/Canal/Redis管理员及首次生成的秘密
    mysql-root-password      MySQL启动专用文件
    canal.env                Canal专用密码环境文件
    redis/                   Redis配置与ACL摘要
    ca-key.pem               CA签名私钥
```

Linux准备脚本使用目录0700、文件0600。服务配置中的PEM和持久目录使用容器绝对路径；MySQL root仅在安装器读取的明确私密挂载中出现。Canal有自己的密码文件，Redis有自己的管理员ACL；这些秘密不会放进应用 `dev.json`或业务进程环境。私密目录不是分享制品，不把其JSON、Cookie、验证码或私钥贴到聊天、Git或日志报告中。

## 证书、域名和 SMTP的实际能力

首次prepare生成当前部署自己的自签CA和角色证书。内部证书保留 `localhost`、`127.0.0.1`及 `spiffe://collaboration-report/<role>`身份；Edge额外加入公有origin的域名或IP SAN。CA签名私钥只留在admin，应用只挂CA公钥和角色材料。浏览器用最终origin访问，否则host/origin/CSRF或TLS校验会失败。

当前自签CA适合受控演示。体验者只取得 `application/pki/ca.pem`公钥，并在自己的测试浏览器/系统中明确信任该CA，或使用 `curl --cacert`作命令行验证；不能关闭内部TLS验证。示例验证：

```bash
sudo curl --cacert /srv/collaboration-report/private/application/pki/ca.pem https://reports.example.com/
```

prepare没有 `--cert`、`--key`或公有CA接入开关。Edge同一份证书也用于内部mTLS，它同时需要原CA链、localhost SAN、角色URI及相应客户端身份；普通网站证书不能直接覆盖 `edge.pem`。公网受信证书和续期需要明确设计外部入口证书与内部身份的配置，并验证完整HTTPS/WSS、同源Cookie/CSRF及受控流式输出。当前没有已验证的反向代理方案，不给手改密钥路径或关闭验证的替代命令。

默认 `developmentSMTP: true`启动只监听应用容器loopback的收件器。它把验证码邮件正文保存在 `application/smtp/inbox/*.json`，不会向公网邮箱转发。它只用于本机/隔离体验；`READY`不说明真实邮箱已经收到邮件。查看该目录时按收件人和时间定位自己的验证码，不共享其它用户邮件正文。

prepare提供 `--smtp-address`和 `--smtp-from`，显式传入地址会关闭开发收件器。当前 `serviceConfiguration`为外部地址生成完整SMTP段：465端口默认tls，其它端口默认starttls，credentialsFile指向 `/app/.runtime/private/smtp-credentials.json`。这只生成配置引用，不生成邮箱授权码文件，也不证明该供应商支持相应模式。[SMTP适配器](../../services/account/internal/client/smtp.go)支持tls/starttls与可选CA文件，始终核验证书/主机名；development仍只接受数值loopback明文且不发送凭据。

Windows 的发信配置见[SMTP 设置](../../configs/README.md#smtp设置)。Linux 服务器首次准备如使用自己的 QQ 发信邮箱，用下例替换首次 `prepare` 命令。已有实例沿用原准备参数。

```bash
# 工作目录：服务器上的自己的完整源码；先将域名与发件地址替换为实际值。
cd /opt/collaboration-report
sudo node scripts/server-prepare.mjs --directory /srv/collaboration-report/private --origin https://reports.example.com --https-port 443 --smtp-address smtp.qq.com:465 --smtp-from your-sender@qq.com
# application挂载为容器/app/.runtime，所以宿主私密文件与配置引用对应如下。
sudo install -d -m 0700 /srv/collaboration-report/private/application/private
if sudo test -e /srv/collaboration-report/private/application/private/smtp-credentials.json; then
  echo '凭据已存在，请在本机编辑，不复制覆盖'
else
  sudo install -m 0600 configs/smtp/credentials.example.json /srv/collaboration-report/private/application/private/smtp-credentials.json
fi
sudoedit /srv/collaboration-report/private/application/private/smtp-credentials.json
sudo node scripts/server-compose.mjs config --directory /srv/collaboration-report/private
```

在本机编辑器把username/password占位值替换为完整发信用户名和SMTP授权码，保存后再执行下节up；不要把真实值写进命令参数。私密目录中Account配置的credentialsFile是容器路径，不是宿主 `/srv/...`或Windows D盘路径。其它供应商按实际TLS模式设置；需要可选CA时只追加可信CA，不能关闭证书验证。

已有部署调整SMTP时先用本owner包装器停止应用，在本机编辑 `application/services/account.json`的smtp段和私密凭据，保留其它配置；当前校验会核SMTP公开字段，后续prepare保留既有SMTP段。不要改manifest、DSN、owner或证书来切换发件人。原manifest如果仍启用开发收件器，监督器可能继续启动该本地辅助进程；真正的发件通道以Account配置为准，开发inbox不再是远程邮件的收件位置。没有用于改变这项manifest开关的CLI。

配置 SMTP 后，用目标实例验证验证码是否送达、能否在有效期内使用，以及发信失败时的页面提示。

## 初始化、启动和真实就绪

从源码根运行宿主[包装器](../../scripts/server-compose.mjs)。它在操作前核验准备目录绑定、本project所有已有容器、五个同名卷和default网络的owner，归属不符就停止。正常操作使用包装器；直接 `docker compose up`会绕过这层已有资源归属核验。`config`只做 `--quiet`静态解析，普通Compose渲染可能把Canal `env_file`中的密码展开。

Linux首次完整启动：

```bash
cd /opt/collaboration-report
sudo node scripts/server-compose.mjs up --directory /srv/collaboration-report/private
sudo node scripts/server-compose.mjs status --directory /srv/collaboration-report/private
sudo node scripts/server-compose.mjs logs --directory /srv/collaboration-report/private
```

Windows独立Linux容器演示使用同一包装器：

```powershell
node scripts/server-compose.mjs up --directory "$PWD/.runtime/server-deployment"
node scripts/server-compose.mjs status --directory "$PWD/.runtime/server-deployment"
node scripts/server-compose.mjs logs --directory "$PWD/.runtime/server-deployment"
```

启动依赖顺序是 MySQL/Redis/Kafka/ES → 实际3分区topic → `initialize` → Canal → `replication-ready` → application。复制屏障使用只挂给一次性安装器的root凭据，检查原受限Canal账号的真实binlog dump线程；Canal外层shell/Java存活不足以放行发布。包装器启用 `initialize`profile；安装器与复制屏障成功退出不是故障。Kafka卷准备也会成功退出，它仅修正卷根归属，不格式化既有KRaft元数据。

仅准备底座而暂不启动应用时：

```bash
sudo node scripts/server-compose.mjs initialize --directory /srv/collaboration-report/private
```

该命令会启动所需依赖并执行一次性安装，不等于整站已经启动。[安装器](../../scripts/server-initialize.mjs)真实安装并校验全迁移链，使用已保存秘密证明原账号、逐项核验私有表权限、Canal权限和Redis ACL。已有未知账号不会被 `ALTER USER`接管；半安装或摘要不符保留现场，不吞SQL失败。

| 观察结果                         | 可以说明的事实                                             |
| -------------------------------- | ---------------------------------------------------------- |
| `PREPARED` / `ALREADY_PREPARED`  | 私密配置已生成或已有绑定已核验                             |
| `COMPOSE_VALID`                  | Compose能够解析；尚未验证运行                              |
| `DEPENDENCIES_INITIALIZED`       | 真实迁移、账号和ACL检查完成；应用仍未就绪                  |
| `CANAL_REPLICATION_READY`        | 原受限Canal账号已实际建立binlog dump连接；应用随后启动     |
| 应用 `startup.json`为 `STARTING` | 已开始启动或恢复；不能交给用户正常使用                     |
| `OBSERVATION_TIMEOUT`            | 观察预算内尚未就绪；存活实例继续恢复，旧输出结果仍可能未知 |
| 监督器 `READY`及容器 `healthy`   | 当次标记的四服务、Edge和必要开发SMTP实际健康检查通过       |
| 浏览器代表性流程完成             | 已验证该项用户行为；不能用健康检查替代全部功能验收         |

应用[健康检查](../../scripts/server-health.mjs)访问四角色完整 `/health/ready`响应、Edge前端和必要SMTP banner，同时核对原启动标记。Search ready包含原索引/alias/日志窗口和消费恢复检查，不靠某个“容器Running”状态代替。`up --wait`最多等待300秒；失败返回后可能仍有存活容器，先查看状态与日志，不据超时认定进程结束。

目标实例就绪后，用浏览器依次检查登录、协作保存、审核与发布、搜索、图片和导出。升级已有实例时，同时复核原数据和未完成的任务。

## 日志与稳定错误

`status`显示本owner容器状态；`logs`显示应用监督器最近100行。具体角色日志在私密目录 `application/logs/{account,document,search,collaboration,edge,smtp}.log`，角色尚未启动时对应文件可能不存在。只在自己的运维终端查看需要的部分；排错报告使用稳定错误码与阶段，不粘贴完整私密配置或Docker inspect环境。

| 错误/现象                                                       | 核对方向                                                        |
| --------------------------------------------------------------- | --------------------------------------------------------------- |
| `SERVER_DIRECTORY_NOT_EMPTY`                                    | 新目录已有未知文件；选明确的新目录，不删除旁边环境              |
| `SERVER_EXISTING_CONFIGURATION_CONFLICT`                        | origin/映射端口/SMTP与原绑定冲突；按原参数复用                  |
| `SERVER_MANIFEST_INVALID` / `SERVER_COMPOSE_BINDING_MISMATCH`   | 路径或Compose环境被移动/修改；保留原文件，核对部署身份          |
| `CONTAINER_OWNER_MISMATCH`                                      | 同名容器、卷或网络属于其它owner；停止操作，不接管               |
| `SERVER_CERTIFICATE_GENERATION_FAILED`                          | 宿主OpenSSL路径、配置与私密目录权限；不要关闭TLS检查            |
| `MIGRATION_PARTIAL_INSTALLATION` / `MIGRATION_VERSION_MISMATCH` | 真实DDL/四库回执或迁移摘要不一致；保留卷及原回执检查            |
| `SERVER_ACCOUNT_PRIVILEGES_EXCESS`                              | 已有账号具有计划外授权；保留身份核对，不自动撤权/重置密码       |
| `SERVER_DOCKER_COMMAND_FAILED`                                  | 读取此前实际Compose失败阶段；不是可以清卷重试的理由             |
| Redis unhealthy                                                 | 核对固定镜像入口与私密挂载；当前降权工具是 `/usr/bin/setpriv`   |
| `SMTP_CONFIG_INVALID` / `SMTP_SETTINGS_INVALID`                  | 核完整address/from/security和容器内绝对凭据路径；development只允许loopback，tls/starttls要求凭据及有效证书 |
| `SERVER_NOT_READY` / `OBSERVATION_TIMEOUT`                      | 查看仍在恢复的原服务/任务，不修改checkpoint或伪造终态           |

## 停止、重启与数据保留

正常停止本owner：

```bash
cd /opt/collaboration-report
sudo node scripts/server-compose.mjs stop --directory /srv/collaboration-report/private
sudo node scripts/server-compose.mjs status --directory /srv/collaboration-report/private
```

Windows演示目录采用：

```powershell
node scripts/server-compose.mjs stop --directory "$PWD/.runtime/server-deployment"
node scripts/server-compose.mjs status --directory "$PWD/.runtime/server-deployment"
```

stop等待容器正常终止，保留五个持久卷和整个私密部署目录。不要使用 `down -v`、`volume prune`、删除私密目录或更换owner处理就绪失败；这些会丢失数据库、登录状态、日志身份、复制位置或资源事实。

再次启动仍使用同一源码发行、镜像、目录和owner：

```bash
sudo node scripts/server-compose.mjs up --directory /srv/collaboration-report/private
sudo node scripts/server-compose.mjs status --directory /srv/collaboration-report/private
```

初始化重试核验原迁移、原秘密与已有权限。若报 `SERVER_ACCOUNT_GRANT_OPTION_NOT_ALLOWED`，请保留现场并核对账号是否多了转授权能力。Search 沿用原消费组和物理日志身份。监督器为新进程生成启动标记，恢复流程核验旧进程是否实际退出。应用恢复健康后，复核代表性原数据。

## 一致备份与恢复边界

本部署的完整备份必须包含同一停止窗口内的五个命名卷及整个私密目录：MySQL/binlog、Redis AOF、Kafka日志和消费位置、ES、Canal位置，以及角色配置/证书、资源、导出和Edge门闩。还应保存对应源码提交和完整镜像。只拷贝MySQL或图片目录不能恢复搜索、可靠协作及权限关闭事实。

先用包装器status核验归属，再stop并确认应用和全部依赖已经停止，之后才进行冷归档或存储快照。归档目录使用0700、制品0600，保存每个制品的长度/SHA256；备份包含真实私钥、用户内容和口令，不混入源码/镜像发布包。归档完成后，按上一节同目录up恢复原实例。

现有[BACKUP-RESTORE.md](BACKUP-RESTORE.md)及 `scripts/backup.mjs`绑定Windows运行布局，不能直接套用于这个server目录。当前没有已验证的Linux一键备份/跨机器恢复命令。恢复应在全新隔离目标中核验全部摘要，保持原库、索引owner/group、物理日志身份和复制位置，再明确处理部署目录和容器卷的归属绑定；不能在新库里重建同名Kafka主题后沿用旧offset，也不能只复制旧startup.json冒称原进程已结束。Linux完整冷备份、恢复和入口切换需独立验证，当前不声称已通过。
