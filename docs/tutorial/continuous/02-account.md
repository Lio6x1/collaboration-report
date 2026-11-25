# 02 账号、验证码、组织与会话：功能带写

这一阶段实现用户进入系统后的账号与团队功能：邮箱注册、登录、修改密码和邮箱、创建组织、邀请或审批成员、接收通知，以及安全退出与注销。接通后，合法用户能完成这些操作，无效凭据或不足权限会被拒绝，断网重试可以恢复原结果。

先从“输入邮箱和密码怎样登录”理解一条完整链路，再扩展验证码、身份与组织管理，最后补通知、关闭和恢复。A08把业务接入HTTP、RPC和后台任务；账号包存在前向引用，整体构建到A08执行，真实邮件与页面流程在06验证。

先按下表进入功能课，跟随具体输入、字段和函数逐段落笔。[文件与完整答案索引](02-files.md)只用于查找，表内文件序号不是教学顺序。

| 课次 | 按功能一步一步实现 | 所属文件 |
| --- | --- | --- |
| A01 | [安全登录、验证码、注册与凭据变更](lessons/account/01-authentication.md) | 11 |
| A02 | [短期身份准入、重验与幂等写入](lessons/account/02-identity-admission.md) | 4 |
| A03 | [组织、邀请申请、成员变更与组织关闭](lessons/account/03-organizations.md) | 4 |
| A04 | [原操作回执、授权证明与平台治理任免](lessons/account/04-receipts-governance.md) | 7 |
| A05 | [通知收件资格、事件投递与确认](lessons/account/05-notification-delivery.md) | 7 |
| A06 | [账号注销准备、职责交接与来源冻结](lessons/account/06-account-close-preparation.md) | 4 |
| A07 | [会话退出、到期与账号注销可靠恢复](lessons/account/07-closure-recovery.md) | 10 |
| A08 | [输出保护与HTTP、RPC、worker完整装配](lessons/account/08-output-and-wiring.md) | 9 |

## 阶段检查

先完成各课规定的依赖；下列为本阶段的收口命令。工作目录仍是自己的lessonRoot，GOWORK/GOCACHE按00配置；coursePrepare/lessonRoot在00设置，新开终端须重新设置。

```powershell
# 工作目录：原lessonRoot。
go -C services/account build -mod=readonly ./...
if ($LASTEXITCODE -ne 0) { throw '账号服务构建失败' }
go -C services/account test -mod=readonly ./internal/domain/account ./internal/logic/account -run '^(TestNewPasswordPolicyKeepsLegacyInputSeparate|TestWeakNewPasswordsRejectedBeforeChallengesOrStorage|TestAuthenticationDistinguishesInvalidIdentityFromReadFailure|TestChallengeFailuresKeepTheirCategoryAcrossBusinessEntries)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '账号规则或用例失败' }
python $coursePrepare --target $lessonRoot --check 02
if ($LASTEXITCODE -ne 0) { throw '账号文件未齐' }
```

构建应退出0，四个具名规则/用例应RUN/PASS、无SKIP。弱新密码在验证码和存储前拒绝，旧密码核验保留兼容输入；数据库故障不冒充401。真实SMTP/Redis/注册与组织流程按A08和06，在自己的实例执行。

缺声明先查本阶段分步是否漏写；重复声明检查是否把完整答案又追加一遍。不可用、超时或未知结果按各课说明保留原操作，不能以删断言、跳过测试或另造新号制造成功。真实功能和页面按[06完整装配](06-integration.md)继续。
