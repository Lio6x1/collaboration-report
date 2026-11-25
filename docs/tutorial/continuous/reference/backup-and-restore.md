# 保存自己的源码，再做冷备份与隔离恢复

本页是06本机验收之后的操作。先按[页面验收表](page-acceptance.md)确认自己的功能；OSS接入使用现行[OSS配置说明](../../../operations/OSS.md)，Ubuntu部署使用[Linux说明](../../../operations/LINUX.md)。备份不是跳过本机学习验收的替代品。

## 1. 先建立学习源码的本地提交

00得到的工程没有.git。backup.mjs调用git rev-parse HEAD和git archive，归档的是**提交中的源码**，不会把未提交的手写成果自动放进source.tar。因此正常编写可暂不使用Git，第一次备份前必须在自己的lessonRoot初始化并提交当前完成版本。

```powershell
Set-Location -LiteralPath $lessonRoot
git init
if ($LASTEXITCODE -ne 0) { throw '学习工程Git初始化失败' }
# 只为这个学习仓库设置本地提交身份；可改为自己的名字，不要求GitHub账号。
git config user.name 'Learning User'
git config user.email 'learning@example.invalid'
git check-ignore .runtime/private/example.json artifacts/example.log node_modules/example
if ($LASTEXITCODE -ne 0) { throw '先确认支持包.gitignore可隔离运行数据和缓存' }
git status --short
git add .
git diff --cached --name-only
```

上面末尾只列暂存路径。检查没有.runtime、artifacts、node_modules、私密证书/账号文件，且包含自己全部手写源码、SQL、前端、锁文件与公开支持。默认.gitignore不会认识你另取名字的私人文件；第一次提交前发现时先用 `git rm --cached -- '具体文件相对路径'` 撤出该文件的暂存（本机文件保留），补排除规则后再检查。核对通过后提交：

```powershell
git commit -m 'Complete local learning version'
if ($LASTEXITCODE -ne 0) { throw '尚未形成可归档源码提交' }
git rev-parse HEAD
git status --short
```

预期有40位提交号且status无待提交源码。后续学习修改也要重新核对并提交，不能把旧HEAD当新成果。这里只写本地历史，不推送到远端。已有仓库不必重复初始化/改身份，直接检查并提交本次差异。

## 2. 按当前运维说明备份与恢复

保存提交后，继续[Windows一致备份与恢复](../../../operations/BACKUP-RESTORE.md)。那份说明维护冷停顺序、九包范围、OSS快照、隔离恢复和真实检查，本页不另维护第二套实现。执行时将其示例工程路径替换为自己的lessonRoot；绝不切回教师体验产品目录。先固定三个自己的绝对路径：

```powershell
Set-Location -LiteralPath $lessonRoot
$lessonSourceRoot = $lessonRoot
$lessonBackup = Join-Path $lessonRoot '.runtime/backups/learning-01'
$lessonRestore = Join-Path $lessonRoot '.runtime/restores/learning-01'
node --no-maglev scripts/backup.mjs backup --destination $lessonBackup
if ($LASTEXITCODE -ne 0) { throw '冷备份失败，保留现场' }
node --no-maglev scripts/backup.mjs inspect --source $lessonBackup
if ($LASTEXITCODE -ne 0) { throw '备份未通过完整性检查' }
node --no-maglev scripts/backup.mjs restore --source $lessonBackup --target $lessonRestore
if ($LASTEXITCODE -ne 0) { throw '隔离恢复失败，保留原备份' }
```

两个目标必须尚不存在；再次练习换新编号。预期依次COMPLETE/九包、VERIFIED/九包、RESTORED_STOPPED。随后运维说明里的“恢复目录”就是lessonRestore，“原工程”就是lessonSourceRoot，测试的COLLAB_RESTORE_ROOT也设为lessonRestore。切到恢复副本后将GOCACHE/npm_config_cache设到该副本的artifacts目录，再安装锁定依赖、启动已有依赖并等四服务READY。不要在恢复目标执行00或首次环境初始化。

备份会停止原实例，恢复成功不会自动重启原实例；需恢复原实例时按运维说明的最后一组compose start与start命令，先明确回lessonSourceRoot，并恢复00的新终端环境。恢复测试的身份夹具需要现有数据，缺前置或SKIP不算通过；也要登录恢复副本检查自己的正文、图片与原任务。

已有目标、路径链接、摘要损坏和不完整清单都会拒绝。不能把PREPARING/FAILED改成COMPLETE，不能删卷或改SQL终态绕过故障。备份包含私密配置和业务内容，保留在私密路径。本页新增说明未执行备份、恢复或服务器验收。
