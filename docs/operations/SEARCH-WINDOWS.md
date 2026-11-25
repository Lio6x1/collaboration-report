# Windows真实消息依赖

工作目录为`D:\job\golang\projects\collaboration-report`。先按既有Windows说明完成本项目基础环境与登录依赖，保持Docker Desktop运行。使用当前实测Node24.19.0兼容选项：

```powershell
Set-Location -LiteralPath 'D:\job\golang\projects\collaboration-report'
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev scripts/search-environment.mjs
```

成功最后输出`SEARCH_DEPENDENCIES_READY`，`replicationConnected`为true。这个状态只证明受限复制、三分区Kafka与ES依赖已就绪。Go消费者已接入原产品主题；完整查询分页、重建及搜索页面仍在实现，不能据依赖就绪声称完整搜索成功。

初始化复用原MySQL，先检查binlog启用、ROW/FULL及非零server-id；条件不满足时返回`MYSQL_REPLICATION_SETTINGS_REQUIRED`，不偷偷重启数据库。当前实测binlog保留30天。复制账号仅有复制所需全局权限和`document_db.outbox`查询权限，其他三个业务表实际拒绝。密码及本地端口保存在ignored配置，不贴到终端或文档。

Kafka、ES、Canal均使用固定镜像摘要和本owner持久卷。Kafka初始化服务只修正卷根目录为固定镜像的1000:1000，不递归改日志、不删卷；Canal等待Kafka实际健康后启动，防止producer在broker DNS未出现时退出。Canal Java存活检查独立于容器外层shell，复制位置与H2表结构快照位于原`canal-position`卷，JVM实际上限256MiB。

重复初始化保持原账号秘密、卷和位置，不覆盖未知账号。普通bootstrap及登录环境重建会保留搜索端口和Canal密码；不要使用`down -v`、清空MySQL或删除复制位置处理启动问题。

ES当前Java heap为1GiB，容器上限2GiB。超过19MiB真实正文消费在旧256MiB堆中发生Java heap OOM退出；父文档全文和章节定位文本都会进入索引，完整内容不能截断。资源调整后同一容量反例通过，固定镜像摘要和原ES卷保持一致。这个配置是当前开发验收基线，整体容量与生产资源预算在第八批继续验证；Kafka/Canal仍使用各自原有上限。

本切片实际验收：

```powershell
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev --test --test-concurrency=1 tests/search/delivery.test.mjs tests/search/persistence.test.mjs
```

两项必须通过、0SKIP。第一项经真实HTTPS创建文档、发布两版并核对固定offset窗口；第二项保留原卷重建Kafka/Canal，核对原集群身份及不回绕的offset，证明停机期间提交的事件能从原复制位置补取。该演练只操作本项目归属环境，失败也恢复依赖；失败日志保留在`artifacts/batches/06`。

003迁移只追加Search的原消息行、物理代次应用状态及分区检查点；基础安装器按连续版本/父摘要加载，四库保存相同版本回执，已有任何对象却缺回执时明确拒绝半安装。001/002保持原字节。当前实例已实际升级，原100张表所有原行数保持；重复安装不重放DDL。Search账号只有新增私有表的逐表权限，仍不能查询其他服务私有库。

真实Go私有库验证需要显式指定本项目配置，缺少配置直接失败，不用SKIP作为通过：

```powershell
$env:GOCACHE = 'D:\job\golang\projects\collaboration-report\.runtime\go-cache'
$env:GOWORK = 'off'
$env:COLLAB_TEST_SEARCH_CONFIG = 'D:\job\golang\projects\collaboration-report\.runtime\services\search.json'
go -C services/search test -mod=readonly -p=1 -count=1 -v ./...
```

这一步使用真实私有SQL、Kafka和ES，验证事件绑定、原内容不可覆盖、事务回滚/独立物理代次、整条记录的offset记账、毒消息拒绝及消费恢复。必须串行执行实际故障组，避免同一环境并发停机或改代次。

产品链路及政策反例可分别执行：

```powershell
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev --test --test-concurrency=1 tests/search/indexing.test.mjs tests/search/indexing-recovery.test.mjs
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev --test --test-concurrency=1 tests/search/visibility.test.mjs tests/search/visibility-snapshot.test.mjs
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev --test --test-concurrency=1 tests/publication/public-reading-capacity.test.mjs
```

故障测试会核验本项目 owner，并在 `finally` 恢复原依赖和 Search 进程。Search 通过 mTLS 读取 Document 的权威快照。消费者分别记录 Canal 消息和权威正文的摘要，依次确认 ES 更新、SQL 应用记录和 SQL 检查点，然后提交 Kafka 消费位点。遇到未知结果时重读原消息，隐藏状态保留单调水位。

候选RPC和单链关闭恢复切片使用同一实际配置。应用先以原Search代次锁预留游标，再调用ES；查询成功确认最新PIT后才返回批锚点。RPC批固定100、原绝对期限30秒不续期，最多128条未释放链/1024行总注册；不把这些开发初值当吞吐承诺。候选只供真实Doc mTLS调用，浏览器经Doc `/v1/search` 取得当前授权、固定正式内容及联合输出保护后的页。

```powershell
& 'D:/go1.25.10/bin/go.exe' -C services/search test -mod=readonly -p=1 -count=1 -v ./internal/repository/cursor ./internal/logic/candidates ./internal/svc
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev --test --test-concurrency=1 tests/search/cursors-recovery.test.mjs
```

第一条核验单链NEXT/REPLAY、真实SQL回滚、满界、跳锁调度和未知确认；svc还包含原消费者恢复。第二条实际退出并恢复当前owner Search，原锚点可重放；自有持久inflight夹具恢复成CLOSING且原自然期限后才释放。测试明确标注SQL夹具构造，不把它称为真实COMMIT响应断网。未知句柄关闭返回PROCESSING时，清理也必须等待原期限与实际CLOSED，不能直接删除登记行。两个故障组不能与其它Search停机、ES停机或代次切换测试同时执行。

Doc搜索核心可执行以下模块验收。真实路由及连接已装配；运行前当前owner Search须就绪，不与其它依赖停机/代次故障组并发：

```powershell
Set-Location 'D:/job/golang/projects/collaboration-report'
$env:GOWORK = 'off'
$env:GOCACHE = 'D:/job/golang/projects/collaboration-report/.runtime/go-cache'
$env:COLLAB_TEST_DOCUMENT_CONFIG = 'D:/job/golang/projects/collaboration-report/.runtime/services/document.json'
$env:COLLAB_TEST_SEARCH_CONFIG = 'D:/job/golang/projects/collaboration-report/.runtime/services/search.json'
& 'D:/go1.25.10/bin/go.exe' -C services/document test -mod=readonly -p=1 -count=1 -timeout=180s -v ./...
```

核心检查点实际166顶层PASS/0SKIP，接线后以受影响实际入口验证。固定内容夹具只在原事务构造自己的完整外键图并回滚；来源元数据异常夹具仅独占连接临时会话故障注入，不能全局关闭外键。摘要读取先按manifest字节元数据占单篇24MiB/累计64MiB含lookahead，再验证完整正文，不从MAIN/ES摘取用户正文。provenance_json为MySQL JSON列，先规范化语义；canonical_body为BLOB，原字节/hash/schema/资源严格不变。

已有开发配置若缺少Doc→Search peer，先执行下列专用配置器，再沿原owner正常停止/启动开发环境一次；配置器保留原密钥、数据库和其他peer，不替代全新安装配置生成。实际当前环境已完成此步骤。

```powershell
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev scripts/configure-search-pages.mjs
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev --test --test-concurrency=1 tests/search/pages-route.test.mjs tests/search/pages.test.mjs
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev --test --test-concurrency=1 tests/search/pages-output.test.mjs
```

输出故障组串行持有隔离实例的行锁并暂停对应 Edge 进程，结束时恢复原进程和锁。请单独运行该组。Windows 响应 Schema 校验使用 UTF-8 解码。

```powershell
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev --test --test-concurrency=1 tests/search/pages-expiry.test.mjs tests/search/pages-recovery.test.mjs
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev scripts/search-partial-test-environment.mjs --check
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev scripts/search-partial-test-environment.mjs --run
```

partial脚本只创建随机独立compose项目及两个新卷，需要已缓存的固定ES镜像及额外2GiB容器容量。主控先核验主机/Docker容量、其他故障组已退出；脚本核验owner/fixture/service标签后仅停止B，finally恢复精确B和删除该独立项目卷。新PIT必须明确Close成功；旧故障句柄未知单独记录，以两精确节点完整open_contexts=0核验上下文终态。恢复未知时保留资源并FAIL，不能手动全局prune。

规模关口现已实际通过，沿当前owner单独执行，不能与故障/切代次测试并行：

```powershell
Set-Location 'D:/job/golang/projects/collaboration-report'
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev --test tests/search/pages-scale.test.mjs
```

三个纯规则及一个实际HTTP关口全PASS。2305个自有SQL/ES样本（不是2305次创建HTTP）中前1200项当前权限拒绝，1105项完整遍历。原三秒请求/三十秒链不变，13页及终批REPLAY无漏重；helper等待原CLOSED/自然释放后才清理绑定样本，未知时保留并FAIL。首两次三秒超时证据和改为批量事实/来源锁的SQL反例均保留。

若需诊断阶段成本，可独立运行下条；它包含自有样本准备/清理，但只诊断RPC、SQL及固定摘要，不代表HTTP/Edge/ACK验收：

```powershell
& '.runtime/toolchains/node-v24.19.0-win-x64/node.exe' --no-maglev scripts/search-page-scale-profile.mjs
```

`search_profile` Go标签只供此诊断，常规模块不含它；不要自行放宽时间或清除OPEN注册行。完整Doc实际模块最新173顶层PASS/零跳过（02证据），第一轮缺Search配置FAIL仍保留。完整浏览器页面、重建和整批06仍待交付。

## 索引重建操作（2026-10-07）

当前已提供准备、Doc封存、Search导入、Start/Get及后台种子/重放/核验/切换。重建的目标代次必须是当前SQL代次加一；不要从旧验收记录复制固定的2。操作前保留原operation、目标代次、prepare_ref和snapshot_ref，后续重试始终复用这四项。数据库维护人员可使用Search自己的账号执行只读语句 `SELECT alias_name,physical_index,generation,revision FROM index_generation` 确认当前事实，不打印配置中的密码。

在项目根目录构建维护命令：

```powershell
$env:GOWORK = 'off'
$env:GOCACHE = 'D:/job/golang/projects/collaboration-report/.runtime/go-cache'
& 'D:/go1.25.10/bin/go.exe' -C services/search build -mod=readonly -o ../../.runtime/bin/search-rebuild-prepare.exe ./cmd/search-rebuild-prepare
& 'D:/go1.25.10/bin/go.exe' -C services/search build -mod=readonly -o ../../.runtime/bin/search-rebuild-import.exe ./cmd/search-rebuild-import
& 'D:/go1.25.10/bin/go.exe' -C services/document build -mod=readonly -o ../../.runtime/bin/search-snapshot.exe ./cmd/search-snapshot
& 'D:/go1.25.10/bin/go.exe' -C services/document build -mod=readonly -o ../../.runtime/bin/search-rebuild.exe ./cmd/search-rebuild
```

交接顺序如下，每个命令非零退出后先检查固定错误码，不继续下一步：

1. Search准备：`search-rebuild-prepare.exe --config .runtime/services/search.json --operation <原操作号> --generation <下一代次>`。返回prepare引用；对应SHA文件在`.runtime/rebuild/search/outbox/<sha>.prepare.json`。
2. 将该文件原字节复制到`.runtime/rebuild/document/inbox`。Doc封存：`search-snapshot.exe --config .runtime/services/document.json --operation <原操作号> --prepare-ref <原prepare引用>`。
3. Doc返回snapshot引用，outbox位于`.runtime/rebuild/document/outbox`。把同SHA的`.snapshot.json`和`.entries.ndjson`两份原文件复制到`.runtime/rebuild/search/inbox`，不修改或拼接JSON。
4. Search导入：`search-rebuild-import.exe --config .runtime/services/search.json --operation <原操作号> --snapshot-ref <原snapshot引用>`。
5. Doc身份启动：`search-rebuild.exe --config .runtime/services/document.json --operation <原操作号> --snapshot-ref <原snapshot引用> --action start`。命令位于`.runtime/bin`，PowerShell调用本地文件时使用`& '.runtime/bin/命令.exe'`。
6. 查询时使用完全相同参数，只把action改为`status`。PROCESSING继续等待原worker；SUCCEEDED包含固定结果字节、摘要及目标代次，重复查询结果不变。

参数中的尖括号内容是需填写的原始值，不是可直接执行的文字。相对配置路径以项目根目录为基准；带空格路径用单引号包住。准备/封存CLI分别只访问本服务私有数据库，正文仍由Doc mTLS权威快照提供，不给Search跨库授权。

种子和追近期间继续服务旧索引。封存固定H进入SWITCHING后，搜索暂时明确不可用；后台完成逐页核验后才切alias和SQL。中断后保留原任务与文件，恢复同一owner服务；不要删任务、改checkpoint、解除屏障或新建操作绕过未知状态。ES已到目标但SQL仍为源时由原任务补完SQL，不手动再增加代次。已有旧物理索引和应用账本不会自动删除。

005以后普通消费者固定cluster/topic UUID；既有消费历史首次升级需要匹配本owner先前签名prepare。`SEARCH_LOG_IDENTITY_BOOTSTRAP_REQUIRED`需要核对历史来源，不能通过清空checkpoint解决；`SEARCH_LOG_IDENTITY_CHANGED`表示当前日志身份不再匹配，必须保留阻断事实，不能仅按同名topic恢复。
