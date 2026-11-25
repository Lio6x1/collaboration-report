# 协作报告

团队协作报告与知识文档系统：账号组织、权限工作台、多人富文本与可靠保存、固定送审/审核合并、冲突处理、正式发布与搜索、图片/批注/@通知、三格式导出及完整回收/注销生命周期。

当前完整产品、Windows本机体验及Linux部署说明保留；教程采用唯一当前全量路线。最新检查、清理范围和限制见[当前审查](docs/delivery/CURRENT-REVIEW.md)。

配置统一入口：[configs/README.md](configs/README.md)。源码按完整业务分组，保持接口、用例、领域、SQL和运行依赖的职责边界；见[代码组织](docs/architecture/CODE-ORGANIZATION.md)。

## Windows本机运行

本机MySQL、Redis、Kafka、Elasticsearch、Canal全部由Docker Compose启动，无需安装它们的Windows版本。三个Go服务和一个Node协作服务在本机运行，便于调试，同源HTTPS入口提供前端与WSS。

准备Go1.25.10、完整Node24.19.0（含npm）、Docker Desktop Linux容器、OpenSSL、Python/jsonschema、Pandoc3.8及Edge。版本以锁文件与运维说明为准；PowerShell5/7兼容脚本保留UTF-8 BOM。其它机器不要复制本机.runtime中的owner、口令、密钥或数据库状态。

```powershell
Set-Location -LiteralPath 'D:/job/golang/projects/collaboration-report'
# 当前开发机可使用项目已核验Node；其它机器安装完整官方分发。
$projectNodeDirectory = Join-Path (Get-Location) '.runtime/toolchains/node-v24.19.0-win-x64'
if (Test-Path -LiteralPath (Join-Path $projectNodeDirectory 'node.exe')) {
    $env:PATH = "$projectNodeDirectory;$env:PATH"
}
$env:GOWORK = 'off'
./scripts/npm.ps1 ci
./scripts/npm.ps1 run typecheck --workspace web
./scripts/npm.ps1 run build --workspace web
node --no-maglev scripts/dev-environment.mjs up
node --no-maglev scripts/login-environment.mjs
node --no-maglev scripts/search-environment.mjs
node --no-maglev scripts/start.mjs
```

访问启动日志实际输出的HTTPS地址，四服务READY后使用。开发CA尚未被系统信任时浏览器会提示证书问题；内部服务仍严格核验独立CA与身份，不能关闭TLS验证解决。项目不会自行安装系统信任证书。

开发SMTP将验证码保存在当前项目私有目录，不发到真实邮箱。注册可使用example.test邮箱；在另一个终端读取：

```powershell
./scripts/show-mail.ps1 -Email 'demo-a@example.test' -Limit 1
```

停止只操作核验归属的本项目环境，保留持久卷：

```powershell
node --no-maglev scripts/start.mjs stop
node --no-maglev scripts/dev-environment.mjs stop
```

[Windows完整说明与排错](docs/operations/WINDOWS.md)、[发行部署](docs/operations/DEPLOYMENT.md)、[OSS](docs/operations/OSS.md)、[备份恢复](docs/operations/BACKUP-RESTORE.md)。先在本机完成完整功能验收，再将同一版本部署到Ubuntu24.04服务器；服务器配置完成前不宣称已上线。

## 学习与验收

从[连续带写主路线](docs/tutorial/continuous/README.md)开始，在Windows创建自己的空工程，亲写完整后端与SQL，复用明确列出的前端、生成物和测试。各阶段有文件顺序、规则检查，最后在自己的实例做真实模块及整站验收；无需克隆已完成业务或切换历史提交。

[项目介绍](docs/tutorial/project-introduction.md)、[演示脚本](docs/tutorial/demo.md)、[面试自测](docs/tutorial/interview.md)。源码分层与中文原因注释见[代码组织](docs/architecture/CODE-ORGANIZATION.md)，契约生成入口见[contracts](contracts/README.md)。

在已配置Go、Node、Python的项目根运行 `./scripts/verify-quality.ps1 -Tutorial`。结果在 `artifacts/current-quality/verification.json`；真实模块与整站验收按[课程06](docs/tutorial/continuous/06-integration.md)在自己的独立实例执行。静态检查与业务验收分别记录。
