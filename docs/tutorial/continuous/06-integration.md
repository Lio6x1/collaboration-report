# 06 接通自己的实例并验证真实功能

01～05完成后，仍在**同一个**lessonRoot；不用第13章复原、两轮合并或current增量覆盖。本路线全量快照已经含这些改动。先完成前述构建/规则检查，再按下面准备自己的运行配置。代码已连接Main、bootstrap、svc、路由/RPC/worker，运行阶段验证的是这些接线确实可用。

## 前置与首次启动

Go/Node/Docker/OpenSSL按00准备，Docker Desktop使用Linux容器，Python有jsonschema；Word需Pandoc3.8，PDF和浏览器验收由锁定Playwright调用已安装Edge（msedge通道），并需要所用字体。这些转换器在运行服务的本机/未来服务器执行，访问者不用安装。先完成[Windows准备页的导出与浏览器检查](reference/windows-setup.md#进入06前确认导出与浏览器)，预期EDGE_READY与正确Pandoc版本，再执行下面的首次启动。

```powershell
# 工作目录：原lessonRoot。新开终端先重新设置00的三个变量。
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$env:GOCACHE = Join-Path $lessonRoot 'artifacts/go-cache'
python $coursePrepare --target $lessonRoot --check 05
if ($LASTEXITCODE -ne 0) { throw '后端/SQL文件未齐' }
./scripts/npm.ps1 run typecheck --workspace web
if ($LASTEXITCODE -ne 0) { throw '前端类型检查失败' }
./scripts/npm.ps1 run build --workspace web
if ($LASTEXITCODE -ne 0) { throw '前端构建失败' }
node --no-maglev scripts/dev-environment.mjs up
if ($LASTEXITCODE -ne 0) { throw 'MySQL与迁移准备失败，保留原卷排查' }
node --no-maglev scripts/login-environment.mjs
if ($LASTEXITCODE -ne 0) { throw 'Redis/开发SMTP配置失败' }
node --no-maglev scripts/search-environment.mjs
if ($LASTEXITCODE -ne 0) { throw 'Kafka/ES/Canal准备失败' }
node --no-maglev scripts/start.mjs
if ($LASTEXITCODE -ne 0) { throw '完整服务尚未就绪，检查原启动记录' }
```

预期迁移链安装到013，中间件READY，最后四个服务和HTTPS入口READY。首次准备分配独立owner、Compose项目、端口、口令和卷；读取最后启动输出的URL，不固定用20609，也不复制参考实例.runtime。开发SMTP只收进自己的本地忽略目录，自动测试可取验证码，它不向真实邮箱发信；真实发信按[本学习实例SMTP步骤](reference/smtp.md)，OSS按[现行存储说明](../../operations/OSS.md)配置，不能把本地收件成功称为真实邮件送达。

如果首次启动观察超时，保留startup状态，查看稳定日志码，再沿原启动运行start恢复观察；不要先创建第二组进程。如果已经READY，重复start会拒绝，不能把它当初始化失败重新删库。

## 一组功能怎样验收

每次只运行已学完的相关组，串行执行，退出码非零立即停下看第一个具体失败。下面是完整课程的关键模块序列；每个文件调用真实学习实例，注册随机测试身份或新建独立测试文档，不是模拟API。

```powershell
# 工作目录：原lessonRoot，必须已有完整READY；默认开发SMTP模式。
$lessonGroups = @(
    @{ Name='真实协作就绪'; Files=@('services/collaboration/tests/readiness.test.mjs') },
    @{ Name='账号与组织'; Files=@('tests/account/challenge.test.mjs','tests/account/credential.test.mjs','tests/login/auth.test.mjs','tests/account/organization.test.mjs') },
    @{ Name='知识库与权限'; Files=@('tests/workspace/permissions.test.mjs','tests/workspace/sharing.test.mjs') },
    @{ Name='可靠协作'; Files=@('tests/collaboration/wss.test.mjs','tests/collaboration/recovery.test.mjs') },
    @{ Name='固定审核'; Files=@('tests/review/submission.test.mjs','tests/review/merge.test.mjs','tests/review/conflict-resolutions.test.mjs') },
    @{ Name='发布与搜索'; Files=@('tests/publication/releases.test.mjs','tests/search/indexing.test.mjs','tests/search/visibility.test.mjs') },
    @{ Name='图片评论通知'; Files=@('tests/resources/upload.test.mjs','tests/comments/threads.test.mjs','tests/notifications/comment-delivery.test.mjs') },
    @{ Name='固定导出'; Files=@('tests/exports/formats.test.mjs','tests/exports/pool.test.mjs') },
    @{ Name='回收恢复'; Files=@('tests/lifecycle/trash-restore.test.mjs') }
)
foreach ($lessonGroup in $lessonGroups) {
    Write-Host ('正在验证：' + $lessonGroup.Name)
    & node --no-maglev --test --test-reporter=tap --test-concurrency=1 @($lessonGroup.Files)
    if ($LASTEXITCODE -ne 0) { throw ('功能验证失败：' + $lessonGroup.Name) }
}
```

预期各文件实际执行的用例PASS，fail/skip为0。部分权限/恢复测试会临时修改**本学习实例**的夹具事实，串行运行并保留原清理逻辑，不同时跑多组，也不要对体验实例执行。正式邮件模式没有本地收件箱时，这些开发夹具不能照跑；按[测试前置说明](../functional-testing.md)改用适合的显式身份流程，缺环境不算通过。

| 关口 | 关键断言为什么存在 | 失败首先检查 |
| --- | --- | --- |
| 账号/组织 | 重复邮箱不新增；验证码冷却后端有效；密码更新与退出关闭原资格；Origin/CSRF及伪造头拒绝。 | Redis ACL、开发SMTP、handler路由和account事务；503不能当密码错。 |
| 知识库/权限 | 同一用户的授权能力取并集；输出依赖的全部来源须有效，撤销任一依赖会停止旧输出，无权成员不能创建/读；撤权后旧资格失效。 | 真实身份RPC、expected_revision、权限来源及逐表授权。 |
| WSS/恢复 | UPDATE落库后才SAVED，重复更新去重，FLUSH确认原输入；恢复读取已确认内容。 | 票据、epoch、房间持久任务、mTLS；不能只看本地编辑器文字。 |
| 审核 | 原送审固定、继续写草稿不变原轮次；相同operation重试不重复；冲突必须保留。 | 固定manifest、guard、原轮次/提交者绑定及冲突集合。 |
| 搜索 | 发布引用进入真实投影；ES成功不代表SQL/Kafka都提交；结果按当前资格裁剪。 | Canal/Kafka/ES READY、原事件回执及索引消费日志；不重置水位。 |
| 图片/评论/通知 | 上传、配额结算与正文引用完整；历史选区可定位；无权用户不收到泄露内容。 | 原resource/quote/收件资格，结算恢复；不是换公开图片URL。 |
| 导出 | 原作业实际生成并下载三种产物，固定内容、图片和长代码正确；worker并发有限。 | Pandoc3.8/Edge、字体、产物路径、临时预算及原任务；202不是完成。 |
| 生命周期 | 回收撤资格，恢复仍私有未发布；原操作恢复和资源引用保持。 | 原关闭/清理事实和输出排空；不手改任务为SUCCEEDED。 |

这是主线代表验收，不重复历史全部容量和故障矩阵。需要深入练习某模块时沿[功能测试指南](../functional-testing.md)选已有相应文件；大正文、重建、永久清理、并发退出/转移等测试有各自前置，不能把未执行的项目写进自己PASS记录。

两处容易误写的断言：空间used_bytes包含正文容量，新建空文档也有费用，上传测试应核真实初始余额加精确资源增量，而不是总额等于图片字节数；回收文档会发隐藏索引事件，KB变更测试的基线要包含该事件已推进的水位，不能仍假定未发布文档永远是0。配套测试保留幂等、退款、原事件真实消费和后续水位断言。

导出池测试会临时把本学习实例workers改为2，实际观察两个RUNNING和一个QUEUED，然后锁住这三个测试作业的完成事务边界，核验本owner进程并确认它退出后释放行锁。这样避免Windows归属核验期间转换已经完成，却把旧观察误当中断；不改状态、代次或成功事实。恢复默认单worker后，两个被中断原作业从代次2到3，原排队作业从1到2，最终核正文、校验和、预算归还及下载资格。测试会重启本实例，务必串行在专用学习实例运行，finally恢复原配置字节。默认开发SMTP创建独立测试身份；已有账号文件只在测试明确支持时显式提供，不从体验实例取凭据。

## 真实整站与页面

模块通过后运行一次：

```powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/deployment/full-workflow.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实跨模块整站流程失败' }
```

它在真实接口、WSS与浏览器中完成同文档工作流，并核数据库、Kafka/ES、图片、审核、发布、导出及回收结果。浏览器中route.fulfill的界面单测属于模拟API验证，不能代替这一条。报告保留实际命令、退出码、失败/skip和环境，不复制Cookie、验证码或私密配置到报告。

随后打开自己启动输出的URL，按本路线的[当前页面验收表](reference/page-acceptance.md)操作：注册冷却与双密码、同浏览器两账号、昵称与八位编号、空间两级选择、知识库卡片、当前标签编辑、阅读自动更新、图片/文字格式、主线与落后支线拆分、评论定位及折叠回复、审核差异、发布搜索、三格式导出、回收入口。逐行记录自己的实际结果，自动测试通过不代表这些手工操作已完成。

## 停止、重启与排错

首次准备脚本不在每次重启都执行。已有实例使用原owner、卷和配置：

```powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev scripts/start.mjs stop
if ($LASTEXITCODE -ne 0) { throw '归属检查或停止失败，不按进程名强杀' }
node --no-maglev scripts/start.mjs
if ($LASTEXITCODE -ne 0) { throw '原实例重启未完成，保留现场' }
```

start以隐藏窗口运行后台进程，转换进程同样隐藏；多人导出由有限worker领取，不是每次点击弹一个cmd。停止服务不会删除数据库卷。PROCESS_OWNER_MISMATCH时核指定PID、入口与本轮标记，不批量终止node.exe；SERVICE_READINESS_TIMEOUT看 `.runtime/logs/<角色>.log` 的稳定错误码，先核迁移/端口/恢复事实。旧表或未知任务不能用删除卷、清空日志或延长业务期限掩盖。

配置/日志中含敏感信息，排错只抄稳定错误码。学习版本机全部正常后，按[自己的源码快照、冷备份与隔离恢复](reference/backup-and-restore.md)准备，再沿[Linux当前说明](../../operations/LINUX.md)部署；远端、公网证书、正式SMTP/OSS仍需实际环境验收。

## Docker提示地址池已用完

本机保存很多历史实验网络时，首次MySQL准备可能只返回DOCKER_COMMAND_FAILED。用当前学习配置的project定位Compose错误；若是 `all predefined address pools have been fully subnetted`，这是Docker网段分配失败，还没有安装数据库迁移。不要运行network prune清理别的项目。

先只读列出 `docker network ls`，逐个用 `docker network inspect <网络名>` 核IPAM.Subnet，并用 `Get-NetRoute -AddressFamily IPv4` 核主机/局域网/VPN的DestinationPrefix。选择一个私有IPv4网段，确认与这些现有网段和非默认路由不重叠。下面命令只给本学习owner预建自己的默认网络；网段必须填你实际核对的值，不能把教师用过的地址当作通用配置。

```powershell
# 工作目录：自己的lessonRoot；只在已经核对网段且本学习默认网络尚不存在时执行。
$lessonSubnet = Read-Host '输入已核对不重叠的私有IPv4 CIDR，例如10.x.y.0/24'
$lessonConfig = Get-Content -LiteralPath '.runtime/dev.json' -Raw | ConvertFrom-Json
docker network create --driver bridge --subnet $lessonSubnet `
    --label ('com.docker.compose.project=' + $lessonConfig.project) `
    --label 'com.docker.compose.network=default' `
    --label ('collaboration.report.owner=' + $lessonConfig.owner) ($lessonConfig.project + '_default')
if ($LASTEXITCODE -ne 0) { throw '本学习网络创建失败，保留原有网络' }
node --no-maglev scripts/dev-environment.mjs up
if ($LASTEXITCODE -ne 0) { throw '继续检查本学习Compose错误' }
```

之后回到首次启动顺序的login/search/start，保持原owner和数据卷。如果地址冲突或需要变更，应先查实际使用方；本教程不授权删除其他项目网络，也不要求修改Docker全局地址池。
