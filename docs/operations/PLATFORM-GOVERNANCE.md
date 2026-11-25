# 平台公开留存治理

平台角色由Account独立保存，和组织OWNER/ADMIN、知识库ADMIN没有继承关系。
仅可撤下或删除注销后孤立的PUBLIC_RETAINED文章，不可改写正文或操作ACTIVE文档。
角色资格每次创建、查询和原号重放均复核；撤回沿真实来源关闭，不靠期限猜测完成。

## Windows本机运维

工作目录 `D:\job\golang\projects\collaboration-report`。运维者须有此部署主机以及
Account私有配置的访问权限；不要把包含数据库秘密的配置、命令结果之外的调试转储
放入教程、Git或浏览器。角色任免没有公开HTTP入口。正式部署仅向指定运维账号开放
配置文件和可执行文件的操作权限；应用普通用户无权读取部署配置。

```powershell
$env:GOWORK = 'off'
& 'D:\go1.25.10\bin\go.exe' -C services/account build -mod=readonly -o ../../.runtime/bin/platform-governance.exe ./cmd/platform-governance
$accountConfig = (Resolve-Path '.runtime/services/account.json').Path
$governanceUser = '实际账号编号'
& '.runtime/bin/platform-governance.exe' --config $accountConfig --user-id $governanceUser --action status
```

账号必须真实ACTIVE。首次不存在角色行时status返回state CLOSED、revision 0；首次授予
expected-revision用0，其余任免用status返回的当前revision。所有编号和修订都保持十进制
字符串，不经JavaScript Number转换。为一次任免生成并保留一个原号：

```powershell
$grantOperation = [guid]::NewGuid().ToString()
& '.runtime/bin/platform-governance.exe' --config $accountConfig --user-id $governanceUser --action grant --expected-revision 0 --operation-id $grantOperation
```

返回SUCCEEDED才是授予完成。回复丢失时使用同一原号、同一用户、同一expected-revision
重送，不能重新生成原号。输入冲突或当前revision改变时读取status再决定新动作，不能
自动改写未知原动作。当前已ACTIVE的新授予原号仅记录幂等结果，不额外递增epoch。

```powershell
$currentRole = (& '.runtime/bin/platform-governance.exe' --config $accountConfig --user-id $governanceUser --action status) | ConvertFrom-Json
$revokeOperation = [guid]::NewGuid().ToString()
& '.runtime/bin/platform-governance.exe' --config $accountConfig --user-id $governanceUser --action revoke --expected-revision $currentRole.revision --operation-id $revokeOperation
& '.runtime/bin/platform-governance.exe' --config $accountConfig --user-id $governanceUser --action status
```

撤回先返回PREPARING，角色REVOKING立即阻止新准入。Account后台等待原Doc/Node关闭
ACK，再同事务写CLOSED、原操作SUCCEEDED和任务终态。不要手动写SQL补成功；服务恢复
后会续接原任务。REVOKING期间拒绝重新授予。未知回复重复原revoke，终态重放返回原结果。

## 浏览器治理

使用获资格账号登录，打开「留存治理」或 `/governance`，输入实际留存文档编号并查询。
页面取得当前控制修订；普通成员、组织OWNER与无资格用户会被拒绝。仍在发布的文章可
撤下，撤下后仍可永久删除。永久删除必须勾选二次确认，显示最终成功前仍须等待真实清理。
连接失败后点击「查询或重试原操作」，沿原号/原输入继续，不能另发一份删除。

治理元数据GET是实施只读扩展，复用既有Document模型；旧正文来源始终保持关闭。
写入用冻结VersionedAction，仅含operation_id和expected_revision。角色撤回后旧治理
进度与重放也会403；已登记的固定关闭清理仍可自动安全完成，不能借此扩展目标或代写正文。

## 实际证据与范围

009迁移在原owner真实安装并READY，原数据保留。真实账号注销、原号授予、治理撤下、
浏览器丢回复重试、Node原清理、图片/导出物理排空与资格撤回见
`artifacts/retained-governance-actual-04.log`；首轮缺Edge来源白名单的失败仍保留。
此工具是受部署主机权限保护的运维入口，不等同普通用户管理后台。第八批正式部署应
将相同配置保护和运行账户边界写入部署手册并实际核验。
