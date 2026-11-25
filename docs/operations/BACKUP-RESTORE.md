# Windows一致备份与恢复

在待备份实例的项目根目录执行。Docker Desktop 使用 Linux 容器。先记录当前源码版本，再分别验证冷备份与隔离恢复结果。

阿里云接入后的九包冷备份/隔离恢复也已实际通过（`backup-20261008-oss-02` / `restore-20261008-oss-02`，源码 `79e9d1c`）。全库11迁移、10050个固定正文版本、40原回执、15消费水位及49个实际完整资源核验 PASS；其中一个原本仅有云对象的图片由恢复账号真实登录、BRANCH引用授权读回完整字节。恢复的 Doc 使用独立本地对象快照，无原 Bucket 写配置。128条 READY 和39条 DELETING 旧metadata-only夹具保留已证明的缺失，不伪造图片或更改 SQL，首次快照失败仍保留。应用及依赖已经停止，卷保留。

启用 OSS 后原应用/Canal先退出，在 MySQL 只读清单窗口取得资源 key/大小/摘要/状态；完整云正文复制到受保护 staging 后再停止 MySQL并归档。未知/损坏拒绝 COMPLETE；归档不包含原 `.oss-binding`。恢复先逐文件校验字节/摘要及原缺失，再移除恢复配置的 OSS 能力。凭据文件独立在原 `.runtime/private`，不属于配置包。本文下方已有命令同样适用于 OSS，必须使用新的备份与恢复目录。

备份将停止当前项目应用和依赖，保留原容器、卷和未完成原号；不会停止其它项目。首先核验本项目owner和五个指定持久卷。Kafka实际数据位于显式`kafka-data`，镜像自带匿名配置/未使用数据目录不替代它。

```powershell
Set-Location D:/job/golang/projects/collaboration-report
# 使用全新、尚不存在的私密子目录；不会覆盖已有备份。
node --no-maglev scripts/backup.mjs backup --destination "$PWD/.runtime/backups/backup-20261008-01"
node --no-maglev scripts/backup.mjs inspect --source "$PWD/.runtime/backups/backup-20261008-01"
```

制品包含四私有库所在的完整MySQL卷（含binlog）、Redis持久状态、Kafka日志/消费位置、ES索引和Canal位置，以及证书、私密配置、资源/导出文件和固定提交源码。各制品保存长度/SHA256，`inspect`全部核验。新目录移除继承权限，仅授予当前Windows账号完整访问；包含密码/私钥/业务内容，不可加入Git或分享。

Edge关闭高水位文件也必须归档，恢复会拒绝缺失该文件的旧包。进程PID登记不会复制为运行状态；恢复先验证摘要，再解压固定源码/私密对象并创建新的owner卷和端口，保持停止状态。

```powershell
node --no-maglev scripts/backup.mjs restore --source "$PWD/.runtime/backups/backup-20261008-01" --target "$PWD/.runtime/restores/restore-20261008-01"
# 切换到输出的恢复源码目录；安装固定版本完整Node/Go等依赖后：
Set-Location "$PWD/.runtime/restores/restore-20261008-01"
./scripts/npm.ps1 ci
./scripts/npm.ps1 run build --workspace web
node --input-type=module -e "import {compose} from './scripts/runtime-lib.mjs'; compose('up','-d','mysql','redis','kafka','es','canal');"
node --no-maglev scripts/start.mjs
```

不要在恢复目标运行首次环境bootstrap覆盖配置。实际索引owner与消费group属于持久来源身份，必须沿归档保留；新部署owner只隔离容器/卷。端口选择避开Windows实际动态TCP范围，而非使用listen(0)给服务分配临时端口。恢复目标限定`.runtime/restores`内全新私密子目录；现有卷/目录不覆盖，失败现场保留。

部署验收可在服务启动前运行`node tests/deployment/restore-baseline.mjs <恢复绝对目录>`保存私密基线，启动后设置`$env:COLLAB_RESTORE_ROOT='<恢复绝对目录>'`，从原工程执行`node --no-maglev --test tests/deployment/restore.test.mjs`。测试身份仅使用历史开发夹具，不能把这些固定密码当成生产用户密码。

只接受`.runtime/backups`内绝对子目录，拒绝已有目标和链接祖先。中断的PREPARING/ARCHIVING/FAILED清单不属于完整备份；不能改成COMPLETE继续恢复。脚本失败保留原事实和临时制品。

备份后保持停止状态。明确需要恢复原环境时，依次执行：

```powershell
# 不更换原owner/卷或重新安装数据。
node --input-type=module -e "import {compose} from './scripts/runtime-lib.mjs'; compose('start','mysql','redis','kafka','es','canal');"
node --no-maglev scripts/start.mjs
```

依赖启动成功不等于应用READY，必须取得监督器真实就绪。数据或原任务未知不能修改SQL终态、清库或删卷绕过。
