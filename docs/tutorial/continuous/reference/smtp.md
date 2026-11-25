# Windows学习实例的真实SMTP发信

先完成[06开发模式验收](../06-integration.md)，再切换自己的发件通道。切换后依赖本地开发收件箱的自动测试不能照跑；继续按[功能测试前置](../../functional-testing.md)选择当前邮件模式支持的检查。


先初始化自己的学习环境；本节只配置该实例的发件通道。系统需要一个发件邮箱给注册邮箱发验证码，发件人自己也可以注册，不因此被排除。先在供应商后台开通SMTP并取得授权码，服务地址/端口与tls或starttls按其实际设置，不把普通网页登录密码当授权码。

```powershell
# 工作目录：自己的学习工程；只停止自己实例的服务组。
Set-Location -LiteralPath $lessonRoot
node --no-maglev scripts/start.mjs stop
if ($LASTEXITCODE -ne 0) { throw '停止失败，不修改发信配置' }
New-Item -ItemType Directory -Path '.runtime/private' -Force | Out-Null
$lessonMailCredentials = Join-Path (Get-Location) '.runtime/private/smtp-credentials.json'
$lessonMailSettings = Join-Path (Get-Location) '.runtime/private/smtp-settings.json'
if ((Test-Path -LiteralPath $lessonMailCredentials) -or (Test-Path -LiteralPath $lessonMailSettings)) {
    throw '配置文件已存在，直接在本机编辑，不覆盖原凭据'
}
Copy-Item -LiteralPath 'configs/smtp/credentials.example.json' -Destination $lessonMailCredentials
Copy-Item -LiteralPath 'configs/smtp/qq.example.json' -Destination $lessonMailSettings
```

用本机编辑器打开两个文件。凭据的username填完整发信邮箱/SMTP用户名，password填授权码；settings里from填被允许的发件地址，credentialsFile改成上面凭据的真实绝对路径。QQ示例使用465/tls；其它供应商按其设置修改。示例占位值不能原样使用，不把真实值贴到命令、聊天或公开配置中。

编辑完成后：

```powershell
# 仍在自己的工程根目录；settings只包含地址、模式和凭据路径，不含授权码。
node --no-maglev scripts/configure-smtp.mjs --settings $lessonMailSettings
if ($LASTEXITCODE -ne 0) { throw 'SMTP配置未完成，不继续宣称可发送' }
node --no-maglev scripts/start.mjs
if ($LASTEXITCODE -ne 0) { throw '重新启动未完成' }
```

预期配置返回CONFIGURED且requiresRestart=true，实际发送需用自己的邮箱点击获取并核收件箱/垃圾箱。适配器校验TLS证书、主机名及有限期限，不能关闭校验消除报错；开发收件模式不投公网。工具保护本机凭据并保留DSN/owner/端口，不重新生成部署身份。

这套脚本限定Windows。之后Linux服务器必须单独绑定容器可读的私密路径与完整SMTP字段，不能沿用D盘路径；详见[Linux说明](../../../operations/LINUX.md)。

### SMTP适配器为什么这样写

跟写[smtp.go分步操作](../write/045-smtp.md)时，先读 `smtpConfiguration`及凭据加载，再读 `Send`。development只允许数值loopback明文且不加载凭据；tls先握手，starttls必须检测并升级成功，两者都核证书链与主机名后才执行AUTH。服务器不支持升级时直接失败，不回退明文；仅支持LOGIN时使用当前适配器，支持PLAIN时优先标准实现。

凭据只从本机绝对路径的普通文件读取，限制大小和JSON字段，接受文件开头UTF-8 BOM而不修改授权码内容。连接、SMTP命令和整个发送都有有限期限，context取消会关闭底层连接；net/smtp自身没有context参数，所以仅传一个ctx并不足以停止阻塞调用。

DATA写入并收到最终接收确认后才认为邮件服务器已接收，后续QUIT失败不重复发同一封。服务器接收仍不等于收件箱送达，要实际检查收件箱/垃圾箱。原始SMTP错误只在适配边界转成固定类别，不能将远端原文或授权码返回用户。这些是通道职责，注册成功仍由用户/消费/回执的数据库事务决定。
