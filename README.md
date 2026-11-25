# 协作报告

协作报告是一个面向团队的文档平台。可以在知识库中组织文档、按章节共同编辑、提交审核并合并修改。正式发布后支持全文搜索，也可导出 DOCX、PDF 或 Markdown 资源包。

项目由 Go 账号、文档、搜索服务及 Node.js 协作服务组成，前端使用 Vue 3。完整技术栈、服务职责和核心流程见[技术说明](docs/TECHNICAL.md)。

## 运行与部署

### Windows 本机

本机MySQL、Redis、Kafka、Elasticsearch、Canal全部由Docker Compose启动，无需安装它们的Windows版本。三个Go服务和一个Node协作服务在本机运行，便于调试，同源HTTPS入口提供前端与WSS。

准备 Go 1.25.10、Node.js 24.19.0（含 npm）、Docker Desktop（Linux 容器）、OpenSSL、Python（含 `jsonschema`）、Pandoc 3.8 和 Edge。在克隆后的项目根目录运行以下命令。

```powershell
$env:GOWORK = 'off'
./scripts/npm.ps1 ci
./scripts/npm.ps1 run typecheck --workspace web
./scripts/npm.ps1 run build --workspace web
node --no-maglev scripts/dev-environment.mjs up
node --no-maglev scripts/login-environment.mjs
node --no-maglev scripts/search-environment.mjs
node --no-maglev scripts/start.mjs
```

四个服务就绪后，打开启动日志给出的 HTTPS 地址。首次准备生成当前实例的 `.runtime/` 配置和凭据。重启已有实例时使用原配置和数据卷，按[Windows 运行与排错](docs/operations/WINDOWS.md)操作。

开发SMTP将验证码保存在当前项目私有目录，不发到真实邮箱。注册可使用example.test邮箱；在另一个终端读取：

```powershell
./scripts/show-mail.ps1 -Email 'demo-a@example.test' -Limit 1
```

停止只操作核验归属的本项目环境，保留持久卷：

```powershell
node --no-maglev scripts/start.mjs stop
node --no-maglev scripts/dev-environment.mjs stop
```

### Linux 服务器

[Linux 部署](docs/operations/LINUX.md)提供 Ubuntu 24.04 的 Docker Compose 安装与升级步骤。SMTP、对象存储和运行配置见[配置说明](configs/README.md)。实例迁移和数据恢复见[备份与恢复](docs/operations/BACKUP-RESTORE.md)。

## 检查与测试

在已配置 Go、Node.js 和 Python 的项目根运行 `./scripts/verify-quality.ps1`，检查格式、离线规则、契约、前端类型和生产构建。结果保存在 `artifacts/current-quality/verification.json`。真实接口、数据库、WebSocket 与故障恢复测试应使用独立实例，参见[Windows 测试说明](docs/operations/WINDOWS.md#验证)。
