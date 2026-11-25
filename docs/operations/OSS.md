# 阿里云 OSS：Windows 配置与真实验证

Document 使用阿里云 OSS Go SDK、V4 签名和 TLS 访问对象。为每个实例指定自己的 Bucket、地域、Endpoint 和对象前缀。数据恢复步骤见[备份与恢复](BACKUP-RESTORE.md)。

## 你需要准备什么

Bucket 保持标准存储、私有、阻止公共访问、版本控制从未开启。为实例分配独立前缀，例如 `collaboration-report/dev/`，并与 RAM 策略保持一致。前缀是对象名的开头，无需在控制台提前创建文件夹。程序只访问该前缀下的对象。

在阿里云控制台搜索「RAM 访问控制」，新建仅供本项目程序访问的 RAM 用户并创建 AccessKey。不要使用主账号 AccessKey。创建自定义权限策略，把 [策略模板](../../configs/oss/aliyun-ram-policy.example.json) 的 JSON 粘贴进去，再把策略授权给该 RAM 用户。策略只给指定前缀的对象读、写、私有 ACL 和删除，以及指定 Bucket 的版本状态读取，没有创建/删除 Bucket、改版本设置或列举全部 Bucket 的权限。若改前缀，策略与存储配置必须同步改，且只能在首次接入前确定。

官方说明：[RAM 权限策略](https://help.aliyun.com/zh/oss/user-guide/ram-policy-overview)、[对象 ACL](https://help.aliyun.com/zh/oss/developer-reference/putobjectacl/)、[版本状态读取](https://help.aliyun.com/zh/oss/developer-reference/getbucketversioning)。上传显式设为私有，程序写入/删除前拒绝 Enabled 和 Suspended；运行期间不得更改 Bucket 版本设置。

## 将凭据保存在本机

先按[Windows 初始化步骤](DEPLOYMENT.md#新机器准备)生成环境，在该实例的项目目录运行以下命令。命令会检查目标文件是否已存在。

```powershell
$lessonOSSRoot = (Get-Location).Path
$lessonOSSCredentials = Join-Path $lessonOSSRoot '.runtime/private/oss-credentials.json'
$lessonOSSSettings = Join-Path $lessonOSSRoot '.runtime/private/oss-settings.json'
if ((Test-Path -LiteralPath $lessonOSSCredentials) -or (Test-Path -LiteralPath $lessonOSSSettings)) {
    throw '已有OSS私密文件，请在本机检查和编辑，不复制覆盖'
}
New-Item -ItemType Directory -Force -Path '.runtime\private' | Out-Null
Copy-Item -LiteralPath 'configs/oss/aliyun-credentials.example.json' -Destination $lessonOSSCredentials
Copy-Item -LiteralPath 'configs/oss/aliyun-storage.example.json' -Destination $lessonOSSSettings
./scripts/protect-oss-credentials.ps1 -Path $lessonOSSCredentials
if ($LASTEXITCODE -ne 0) { throw '凭据权限保护失败' }
notepad $lessonOSSCredentials
notepad $lessonOSSSettings
```

在记事本里用 RAM 用户的 AccessKey ID 和 AccessKey Secret 替换中文占位值。普通 RAM AccessKey 的 `securityToken` 留空；只有使用临时 STS 凭据时才填写配套 token。保存 UTF-8；Windows PowerShell5 的 UTF-8 BOM 也支持。不要把密钥发到聊天、放入命令参数、截图、Git 或公共日志。上述保护脚本会替换文件访问规则，只允许当前 Windows 账号；运行 Doc 的账号必须与之相同。

settings中逐项核对region、endpoint、bucket、prefix，并把credentialsFile改为上面的真实学习凭据绝对路径；模板里的D盘产品路径和体验前缀必须替换。RAM策略限定的Bucket/前缀与settings一致，endpoint和region也必须一致。配置过程不需要向聊天发送密钥或文件内容。

## 接入原 Doc 配置

凭据与settings在本机保存好后执行。配置助手要求当前实例实际应用进程已停止，保留MySQL等原owner数据，不重建环境。新开终端时重新设置上面的学习目录与变量：

```powershell
node --no-maglev scripts/start.mjs stop
if ($LASTEXITCODE -ne 0) { throw '当前实例未确认停止，不修改OSS配置' }
node --no-maglev scripts/configure-oss.mjs --settings $lessonOSSSettings
if ($LASTEXITCODE -ne 0) { throw 'OSS配置失败，检查稳定错误码，不继续启动' }
node --no-maglev scripts/start.mjs
if ($LASTEXITCODE -ne 0) { throw '实例尚未启动完成' }
node --no-maglev scripts/server-health.mjs
if ($LASTEXITCODE -ne 0) { throw '实例尚未就绪' }
```

配置助手保留资源目录、配额与原上传期限，只把官方存储配置写入 `.runtime/services/document.json`；凭据仍单独存放。首次启动在原资源目录写入不含秘密的 `.oss-binding`，固定 Bucket/前缀/地域/Endpoint。后续可轮换凭据文件，但不能删绑定文件来切换 Bucket 或退回纯本地模式。迁移初始化脚本会保留已设置的资源和导出配置。

旧本地对象不搬动、不覆盖，云端明确返回原 key 的 `NoSuchKey` 时仍可读取同一目录里的旧对象。云端权限错误、超时或损坏不会回退成成功。自然清理时云对象和旧本地对象都真实不存在后才允许原 SQL 任务释放配额。因此原资源目录仍必须保留，备份也必须覆盖它。

## 实现与验证边界

- 当前合并路径 `services/document/internal/storage/local/objects_storage.go`的 `Stage`用私有临时文件检查完整长度/SHA-256；云端正文未校验完不对HTTP输出，最大单文件继续为32MiB。历史 `temporary.go`已并入此文件。
- `services/document/internal/storage/oss/objects_storage.go`的 `Put`先检查客户端原流，使用禁止覆盖和私有ACL；PUT回复未知时读取同一key，完整校验后才接受。期限已结束则由原业务号恢复，不另起更宽超时。历史 `upload.go`已并入此文件。
- 同文件的 `Delete`对DELETE成功和回复未知都读取原key验证缺失。GET的 `NoSuchKey`可区分对象不存在和Bucket不存在；无正文HEAD 404不用作退款证明。历史 `cleanup.go`已并入此文件。
- 本地跨进程资源锁继续覆盖上传/删除及最后 SQL 确认，读取使用共享锁。只支持单机共享本地目录，不宣称多台 Doc 节点的分布式锁。
- `store_test.go` 经过官方 SDK 的真实签名、请求序列化和错误解码，验证回复未知、损坏、不可覆盖及原本地对象。HTTP 网络由测试替身代替，不能证明真实阿里云账号权限和网络。

真实 SDK 验证可在 `services/document` 工作目录运行以下命令。这个显式 build tag 缺少配置时直接失败，不用 SKIP 伪装成已验收：

```powershell
$env:GOWORK = 'off'
$env:COLLAB_OSS_SETTINGS = $lessonOSSSettings
Push-Location -LiteralPath (Join-Path $lessonOSSRoot 'services/document')
try {
    go test -mod=readonly -tags=osscloud ./internal/storage/oss -run '^TestCloudOriginalObjectRecovery$' -count=1 -v
    if ($LASTEXITCODE -ne 0) { throw '真实云对象恢复检查失败，保留原号和诊断记录' }
} finally { Pop-Location }
```

云端对象备份扩展已经接入原 `backup.mjs backup`：原应用真实退出、Canal/消息依赖停止后，MySQL保持只读查询窗口；`cmd/resourcesnapshot` 取得原 resource key/大小/SHA/状态，将实际完整对象保存到私密 staging，原本地对象/临时文件也保留。未知或损坏读取使备份失败，明确缺失的原 READY metadata 单独计数，不伪造正文或修改 SQL。然后沿原九包冷备份保存本地快照，归档不带 `.oss-binding`。隔离恢复逐个复核完整文件及原缺失事实，删除恢复配置的 OSS 写能力，使用独立本地副本；原 Bucket 的对象与凭据不随副本获得写访问。官方 SDK HTTP 边界与恢复配置/实际快照字节校验已通过；原冷备份历史证据不冒称已包含云对象。

真实产品关口已通过：`batch08-oss-product-actual-01.log` 实际两人协作、图片引用、审核合并、发布搜索、PDF、回收恢复与最后清理/退款全流程 PASS。`batch08-oss-cold-backup-02.log` 九包 COMPLETE，`batch08-oss-restored-verification-02.log` 真实隔离恢复/登录/引用授权图片完整字节 PASS；恢复副本配置没有 OSS 写能力，原环境继续使用原 Bucket。SDK 网络替身与这些真实云关口在台账中分别记录。

旧实验留下非业务 key/NULL checksum 的 metadata-only 行，首次真实快照因此明确失败（cold-backup-01）。修复只在全部写者停止的备份边界逐键证明本地不存在与原云 GET `NoSuchKey`，保留 `metadataOnly:true`/原空摘要及缺失状态；云端存在/权限或网络未知仍拒绝 COMPLETE。正常上传/读取的64小写hex key及完整摘要规则不变，不修改原 SQL。新快照216项，包括49个完整对象、128条 READY 缺失及39条 DELETING 缺失；真实产品上传的图片明确只有云端正文，恢复到本地后实际 HTTP 字节一致。隔离副本已停止并保留数据。
