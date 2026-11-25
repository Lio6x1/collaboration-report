# Windows安装、编辑器与首次检查

本页在[00空工程](../00-start.md)之前完成工具安装，安装器需要的管理员确认在Windows中操作。本文命令均是PowerShell；不要在Git Bash或cmd中粘贴。已安装正确工具时只做验证，不重复安装。课程锁定版本来自工程配置，不跟随下载页的“最新版”自动升级。

## 先安装能运行课程的工具

1. 在Windows“设置 → 系统 → 系统信息”查看系统类型。当前课程使用64位Windows；Intel/AMD机器选x64/amd64，ARM机器需选对应ARM64包，不能把x64验收记录视为ARM验证。
2. 从[Go官方发布目录](https://go.dev/dl/)展开历史版本，选择Go **1.25.10** 的Windows安装包，运行MSI。按[官方安装说明](https://go.dev/doc/install)完成后关闭旧终端并新开PowerShell，执行 `go version`，应包含 `go1.25.10 windows/amd64`（ARM机器架构不同）。
3. 从[Node官方下载页](https://nodejs.org/en/download)进入previous releases/archive，选择 **24.19.0** 的Windows Installer，保留npm安装选项。安装完整分发，不只复制node.exe。新开终端执行 `node --version`，应为 `v24.19.0`。工程的scripts/npm.ps1会从同一Node安装目录找npm。
4. 从[Python Windows下载页](https://www.python.org/downloads/windows/)安装Python **3.12或以上**的稳定版，包含pip并加入PATH。新开终端执行 `python --version`。如果弹出商店而不是版本，在Windows“管理应用执行别名”核对python别名和安装器的PATH选项；先让同一个 `python` 命令正确工作，再进入00。
5. 从[Git官方Windows页](https://git-scm.com/install/windows)安装Git for Windows，让命令行可调用git。执行 `git --version`。它用于自己的版本快照；无需GitHub账号或克隆业务历史。
6. 按[Docker Desktop官方Windows安装步骤](https://docs.docker.com/desktop/setup/install/windows-install/)核对系统和虚拟化要求，安装并选择WSL 2后端。安装器要求重启时完成重启，再启动Docker Desktop，等待引擎运行；使用Linux容器模式。此时只验证引擎，不创建课程数据库。

OpenSSL用于课程脚本生成本机证书。先执行 `openssl version`；若找不到，可检查刚安装的完整Git for Windows是否包含它。以下仅检查默认安装位置，路径不同要改为实际Git安装目录：

```powershell
$lessonOpenSSL = 'C:/Program Files/Git/mingw64/bin/openssl.exe'
if (-not (Test-Path -LiteralPath $lessonOpenSSL)) { throw '当前Git安装未在此路径提供OpenSSL，核对实际安装位置' }
& $lessonOpenSSL version
if ($LASTEXITCODE -ne 0) { throw 'OpenSSL无法运行' }
$env:PATH = (Split-Path -Parent $lessonOpenSSL) + ';' + $env:PATH
openssl version
```

若分发不含该程序，从[OpenSSL项目列出的二进制分发信息](https://openssl-library.org/source/)选择适合本机的受维护Windows分发，安装后验证实际openssl.exe。不要把源码压缩包当可运行安装器。为了新终端也可用，在“编辑账户的环境变量 → Path → 新建”加入实际包含openssl.exe的目录，确定后新开终端；加入的是目录，不是exe完整路径。

```powershell
# 工具总检查；此时不要求lessonRoot已存在。
Get-Command go,node,python,git,docker,openssl | Select-Object Name,Source
go version
node --version
python --version
git --version
docker compose version
docker info --format '{{.OSType}}'
if ($LASTEXITCODE -ne 0) { throw 'Docker引擎未就绪，先打开Docker Desktop' }
openssl version
```

docker info应输出linux；只看到compose版本不代表引擎已启动。找不到命令时用Get-Command检查实际路径，补PATH后重开终端。多个版本并存时先解决路径优先级，不改课程锁文件迁就错误版本。

## 用编辑器落笔，不把源码当终端命令

可以使用现有编辑器；没有时按[VS Code官方Windows安装说明](https://code.visualstudio.com/docs/setup/windows)安装。00初始化成功后，用“文件 → 打开文件夹”打开自己的lessonRoot。左侧文件树只打开学习工程；参考教材在另一窗口阅读。

操作页中的PowerShell块负责建目录/空文件，执行后在文件树打开该文件，将Go/SQL/JavaScript代码块依次输入文件。代码围栏、步骤标题和“本步声明”不写进去。同一声明在讲义与操作页出现时只写一次；建议先读讲义推演，再以操作页为唯一录入清单。中途停下记住已完成的步骤号，下次从下一步继续。

在编辑器状态栏将文件编码设为UTF-8、行尾设为LF，再Ctrl+S。不要用PowerShell 5的默认Out-File保存源码，它可能写成UTF-16。未闭合的长函数先按原步骤完成，不自动补括号凑gofmt成功；操作页末尾才运行格式检查。扩展名应是.go/.mjs/.sql，不是.go.txt。

若scripts/npm.ps1报“禁止运行脚本”，先检查 `Get-ExecutionPolicy -List`。在允许本机开发脚本的个人环境，可在当前PowerShell执行 `Set-ExecutionPolicy -Scope Process -ExecutionPolicy RemoteSigned` 后重试；它只影响该终端。组织策略优先时由设备管理员处理，不能反复更改全局策略。[Microsoft执行策略说明](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.security/set-executionpolicy)

## 进入06前确认导出与浏览器

从[Pandoc官方安装页](https://pandoc.org/installing.html)进入发布历史，安装 **3.8** 的Windows包；新开终端执行 `pandoc --version`，首行应为pandoc 3.8。Node依赖按00安装，PDF使用工程锁定Playwright和本机Edge。检查Windows开始菜单能启动Microsoft Edge；工程不是让Pandoc经LaTeX生成PDF。

Windows“设置 → 个性化 → 字体”中检查宋体（SimSun）、仿宋（FangSong）、黑体（SimHei）、Times New Roman、Consolas。缺少中文字体时，在Windows可选功能中安装简体中文补充字体，再重启使用字体的应用；具体可用名称随Windows版本变化，也可安装具有合法许可的对应字体。字体缺失时浏览器可能静默替换，因此最终三种导出仍须目视检查中文、标题和代码。

在00已安装npm依赖的自己的工程根目录执行：

```powershell
Set-Location -LiteralPath $lessonRoot
pandoc --version
if ($LASTEXITCODE -ne 0) { throw 'Pandoc未进入PATH' }
node --input-type=module -e "import {chromium} from 'playwright'; const b=await chromium.launch({channel:'msedge',headless:true}); await b.close(); console.log('EDGE_READY');"
if ($LASTEXITCODE -ne 0) { throw 'Edge或锁定Playwright无法启动' }
python -c "from jsonschema import Draft202012Validator; print('JSONSCHEMA_READY')"
if ($LASTEXITCODE -ne 0) { throw '先执行00的新终端环境块' }
```

预期EDGE_READY、JSONSCHEMA_READY并退出0。ERR_MODULE_NOT_FOUND先核00的npm ci；Executable doesn't exist先核本机Edge安装；导出失败仍须在06检查实际作业，工具能启动不代表正文已成功转换。本页提供安装与验证操作，未宣称已在一台新Windows机器完成复现。
