# 00 Windows准备与自己的空工程

先按[Windows安装与编辑器准备](reference/windows-setup.md)安装并验证Go1.25.10、完整Node24.19.0、Python3.12以上、Git、Docker Desktop（Linux容器模式）、OpenSSL；其中也给出了命令找不到、脚本策略、编辑器保存格式的处理。Git不是编写或本机运行的前置，但以后做源码备份必须建立自己的本地提交。Word需要Pandoc3.8，PDF由锁定Playwright调用已安装Edge；到06导出前按该准备页补齐字体和浏览器验证。

教材在参考项目阅读，代码写在旁边的新文件夹。下面的两个路径只需第一次修改；以后一直使用同一个lessonRoot。空目录初始化默认预检，明确加write才写支持，已存在学习文件会被拒绝。

```powershell
# 工作目录：D:/job/golang/projects。PowerShell 5/7均可，路径含空格也保留引号。
Set-Location -LiteralPath 'D:/job/golang/projects'
$courseRoot = 'D:/job/golang/projects/collaboration-report'
$lessonRoot = 'D:/job/golang/projects/collaboration-report-learning'
$coursePrepare = Join-Path $courseRoot 'docs/tutorial/continuous/prepare.py'
python $coursePrepare --target $lessonRoot
if ($LASTEXITCODE -ne 0) { throw '空目录和支持包预检失败' }
python $coursePrepare --target $lessonRoot --write
if ($LASTEXITCODE -ne 0) { throw '支持初始化失败，保留现场排查' }
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$env:GOCACHE = Join-Path $lessonRoot 'artifacts/go-cache'
$env:npm_config_cache = Join-Path $lessonRoot 'artifacts/npm-cache'
./scripts/npm.ps1 ci
if ($LASTEXITCODE -ne 0) { throw '锁定依赖安装失败' }
```

预期先PREVIEW，再PREPARED，`businessWritten=0`、`servicesStarted=false`。支持清单中的测试文件会出现在services下，**这不是已经替你写了业务**；手写Go/Node业务文件及SQL仍不存在；生成的RPC/DTO和AJV校验器按支持清单保留，由契约生成器维护。`node_modules`只在你运行ci后安装。没有.git历史，没有.runtime，也没有体验库账号。

初始化器只解开[精确支持](support-index.md)，不导入已完成后端。生成DTO/RPC来自原契约，学习重点是调用和实现这些接口；需要重生成时按[当前契约生成说明](reference/generated-contracts.md)。老师全文页里所有相对路径都相对于lessonRoot。

支持初始化和npm安装成功后，继续准备本工程自己的Python依赖。这个目录位于artifacts，不能提前建在尚需保持为空的lessonRoot中：

```powershell
Set-Location -LiteralPath $lessonRoot
python -m venv artifacts/python
if ($LASTEXITCODE -ne 0) { throw 'Python虚拟环境创建失败' }
$lessonPython = Join-Path $lessonRoot 'artifacts/python/Scripts/python.exe'
& $lessonPython -m pip install 'jsonschema==4.23.0'
if ($LASTEXITCODE -ne 0) { throw 'JSON Schema依赖安装失败' }
$env:PATH = (Split-Path -Parent $lessonPython) + ';' + $env:PATH
python -c "from jsonschema import Draft202012Validator; print('JSONSCHEMA_READY')"
if ($LASTEXITCODE -ne 0) { throw '当前Python未使用学习环境' }
```

预期输出JSONSCHEMA_READY；[jsonschema安装说明](https://pypi.org/project/jsonschema/4.23.0/)给出了该验证器的用途。随后用编辑器“打开文件夹”打开lessonRoot，按操作页创建空文件，在编辑器里输入代码，按Ctrl+S保存为UTF-8、LF。PowerShell块在终端执行，Go/SQL/JavaScript块写进指定文件；不要把Markdown围栏或“本步声明”说明行写进源码。

第一份业务文件从[F01严格正文课](lessons/foundation/01-fixed-content.md)开始：先解释文件和接口，再按分步操作页创建decode.go、逐段实现。讲义和操作页展示的同名声明只写一次。文件所有分段写完后才用 `gofmt -w '文件路径'`；包级检查等讲义指定关口。生成物不手改。用自己的Git提交记录阶段可以，但Git不是编写/运行的前置。

新开PowerShell后变量不保留。每次继续学习先执行下面这一块（两个根路径与首次设置保持一致），不要重跑空目录初始化：

```powershell
$courseRoot = 'D:/job/golang/projects/collaboration-report'
$lessonRoot = 'D:/job/golang/projects/collaboration-report-learning'
$coursePrepare = Join-Path $courseRoot 'docs/tutorial/continuous/prepare.py'
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$env:GOCACHE = Join-Path $lessonRoot 'artifacts/go-cache'
$env:npm_config_cache = Join-Path $lessonRoot 'artifacts/npm-cache'
$lessonPython = Join-Path $lessonRoot 'artifacts/python/Scripts/python.exe'
if (-not (Test-Path -LiteralPath $lessonPython)) { throw '先完成上方Python环境准备' }
$env:PATH = (Split-Path -Parent $lessonPython) + ';' + $env:PATH
```

这些缓存只在自己的工程，不改全局配置。不使用体验项目路径运行初始化、测试或停止命令。

| 失败 | 怎样处理 |
| --- | --- |
| COURSE_TARGET_MUST_BE_EMPTY | 原目录已有成果，换一个新名字；不要删除自己的代码来让初始化通过。 |
| COURSE_REFERENCE_TARGET_REJECTED | 目标指到了参考产品或其内部；选择旁边的学习文件夹。 |
| COURSE_SUPPORT_ARCHIVE_CHANGED | 核对材料完整性，不修改SHA绕过。 |
| NPM_CLI_MISSING | Node分发缺少npm，补完整Node安装；包装脚本不修改全局npm。 |
| Go缓存拒绝访问 | 使用上面工程内GOCACHE，不更改系统目录权限。 |

支持写入中断会留下标记和已写文件，初始化器不会覆盖重试。保留现场，用新的空目录重试；不要递归清理不明确的路径。此阶段还不启动Docker业务环境。
