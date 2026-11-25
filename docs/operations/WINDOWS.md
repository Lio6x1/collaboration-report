# Windows运行与排错

首次运行见[本机安装](DEPLOYMENT.md#新机器准备)。以下说明已有实例的重启、排错与验证。

所有命令在项目根目录执行。需要Docker Desktop已启动并使用Linux容器，Go/Node/OpenSSL/Python在PATH。Python依赖`jsonschema`用于离线schema验收；Edge用于真实浏览器测试。PowerShell 5读中文脚本需要BOM，修改`.ps1`时请保留UTF-8 BOM；`.gitattributes`不转换其字节。

`scripts/npm.ps1`从Node安装目录调用npm CLI，不改系统npm配置。若出现`Cannot find module .../npm-cli.js`，先用包装脚本；若包装脚本仍报NPM_CLI_MISSING，检查Node安装是否包含npm。`npm ci`仅安装锁文件版本；不要把升级全局npm当成项目安装步骤。

当前完整环境首次启动顺序：`./scripts/npm.ps1 ci`→前端build→`node --no-maglev scripts/dev-environment.mjs up`→`node --no-maglev scripts/login-environment.mjs`→`node --no-maglev scripts/search-environment.mjs`→`node --no-maglev scripts/start.mjs`。搜索环境步骤补齐Kafka、Elasticsearch及Canal绑定，不能按早期只有账号的步骤省略。账号环境步骤启动Redis、设置私有前缀ACL并生成SMTP/身份服务配置。首次启动会选空闲端口、生成私密配置和开发证书，后续使用原owner与卷。配置只写`.runtime/`，不能提交、转发或打印其中的密码/私钥。后台辅助进程隐藏窗口。已有实例重启不重新运行首次配置生成。

MySQL容器、卷、网络均有项目owner标记，Docker操作也检查Compose项目标记。应用仅持有自己的逐表数据库账号；部署脚本的root仅用于安装隔离开发实例。未配置自动删库/删卷。`up`不会覆盖旧迁移：摘要不符或半安装时返回非零，保留现场供检查。无需通过删除卷来“修复”未知状态。

当前开发MySQL上限为2GiB，InnoDB缓冲池固定512MiB。真实大正文同时持有原意图、候选与恢复事实；默认128MiB缓冲池曾在连续恢复核验中耗尽空闲页。Docker Desktop需要为MySQL和其余服务保留足够内存；完整消息/搜索/导出环境的总预算在后续整站验收中记录。调整容器配置沿项目owner与原卷执行，不能通过清库缩小数据集获得PASS。数据库恢复检查仍沿用原期限和原事实断言。

`start`记录PID、绝对入口和本轮随机owner，ready响应也必须属于本轮。停止时先核验整组，再终止。若返回PROCESS_OWNER_MISMATCH/PROCESS_IDENTITY_UNAVAILABLE，先查看项目注册表与指定进程命令行；不要改为按node.exe名称批量杀进程。Windows停止是进程终止，当前业务恢复器在重新启动后按持久事实续接未完成操作。

若SERVICE_READINESS_TIMEOUT，查看`.runtime/logs/<服务>.log`的稳定错误码，检查端口、数据库、schema摘要和恢复状态。已实现的账号/Doc授权任务由实际恢复器续接；未知任务、输出实例或Node既存房间仍保持503。BLOCKED关闭水位不能按TTL删除。不要把pending记录改成SUCCEEDED来绕过检查。

监督器的HTTPS就绪探针要求完整响应、200和本轮准确owner；握手未完成、头后截断及持续慢字节都必须在原两秒预算内结束。定位启动阶段可在当前PowerShell会话设置`$env:COLLAB_SUPERVISOR_DIAGNOSTICS='1'`后运行`node ./scripts/start.mjs`，结束后`Remove-Item Env:COLLAB_SUPERVISOR_DIAGNOSTICS`。诊断只含固定阶段名，不打印私密配置；非零或原生退出不能以曾输出READY替代成功。

Windows 26200 / Node 24.15.0 的持续验收曾观察到原生退出 `0xC0000409`，包括显式关闭Maglev的后续样本，没有JavaScript崩溃栈，原因未查明。当前使用官方完整Node **24.19.0**及显式 `--no-maglev`；运行配置的验证通过不代表已证明早期原生退出根因。

安装 Node.js 24.19.0 后，用 `node --version` 核对当前终端版本。`--no-maglev` 由监督器、Node 服务和测试恢复助手显式继承。开发环境运行参数留在当前终端的命令中。

第四批最终采用增量复验：原干净1304e1f的公共/Go/API已通过，62236fc只改两份浏览器测试及文档，完整浏览器另行54/54通过。`artifacts/batches/04/final-qualification.json`记录两轮commit和原日志摘要；原第十一轮`verification-attempt-11.json`保留FAIL。重新执行完整入口会产生新的独立结果，不能拿本机最后一次原FAIL误读成未记录的整次PASS。

可选启动诊断还记录旧输出恢复、全部房间校验、资源确认恢复、门闩恢复与 RPC/HTTPS 监听阶段及耗时。协作服务计时从业务 `run()` 开始，不包含此前模块导入。观测到的 14～17 秒主要在全部既存房间校验；这不等于所有超时或原生退出都已归因，不能省略房间校验来换取 READY。

## 重启已有实例

启动 Docker Desktop 后，在原项目根目录使用已有的 `.runtime/` 和数据卷。先核验已有容器归属并启动依赖，再启动应用。已初始化的实例无需重新执行首次配置。

```powershell
node --no-maglev scripts/release.mjs check
if ($LASTEXITCODE -ne 0) { throw '现有实例检查失败' }
node --no-maglev --input-type=module -e "import { ownedContainer, compose } from './scripts/runtime-lib.mjs'; for (const role of ['mysql','redis','kafka','es','canal']) ownedContainer(role); compose('up','-d','mysql','redis','kafka','es','canal');"
if ($LASTEXITCODE -ne 0) { throw '现有依赖未启动' }
node --no-maglev scripts/start.mjs
```

以 `SERVER_READY` 给出的 HTTPS 地址为准。用 `node --no-maglev scripts/start.mjs stop` 停应用，用 `node --no-maglev scripts/dev-environment.mjs stop` 停依赖。

## 验证

在项目根运行 `./scripts/verify-quality.ps1`，结果见 `artifacts/current-quality/verification.json`。真实数据库、接口、WSS 和故障恢复测试应按对应测试文件的准备条件，在独立实例串行运行。跳过用例须单独记录。

第七批集中故障演练中两次协作服务启动因 `PROTOCOL_SEQUENCE_TIMEOUT` 失败；后续真实原PURGE恢复通过，根因尚未确定。设置诊断环境变量后仅打印 `ROOM_LIST`/`ROOM_CHECKPOINT`/`ROOM_UPDATE_LOG` 等固定阶段与稳定驱动码，不含原始SQL/凭据。未延长期限或跳过既存房间校验，继续在第八批检查启动容量。

第五批同样按明确来源收口：干净a8b3da6首轮16组通过，恢复组8/10失败、浏览器未执行；原样两项2/2复现通过，完整恢复10/10与浏览器9/9随后通过，均无跳过，原503根因未确定。`artifacts/batches/05/final-qualification.json`核对同一源码、完整18组和每份日志摘要；通用verification.json仍保留首轮FAIL。运行新的完整入口会产生独立结果，不能把增量qualification说成单次完整入口全绿。

固定正文读取路由有32MiB缓冲上限，普通JSON仍2MiB。实际4MiB以上正文已做全文比对；公开整篇及文件交付还须第六至八批自己的实际验收，不能用这条单元读取结果替代。关闭排空仍等真实handler/socket与持久原ACK，不因RPC或租约到期记完成。

账号验收会通过开发SMTP创建隔离测试身份，无需预先生成学习seed。历史`login-seed.json`可留给学习演示，验收不读取它。确认写入后读取失败的页面会保留原操作续接；密码/验证码只在应用内存中，硬刷新后需要凭原操作编号核验进度。

开发验证码用`./scripts/show-mail.ps1 -Email '你的邮箱@example.test' -Limit 1`显式读取，只用于当前本机演示。开发收件器没有公共HTTP收件箱，也不会向真实邮箱投递。收件正文含验证码，留在忽略目录，不复制进测试报告。

依赖安装保留Go正常checksum验证。已安装环境可用`$env:GOPROXY='off'`读取缓存；新机器不要复制临时禁用checksum的设置。契约工具的`go.sum`继承已验证依赖摘要，随后由正式模块独立维护。
