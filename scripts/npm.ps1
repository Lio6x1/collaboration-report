param([Parameter(ValueFromRemainingArguments=$true)][string[]]$NpmArgs)
$ErrorActionPreference='Stop'
# 直接使用Node安装目录的CLI，避开本机残缺的全局npm入口；不修改全局环境。
$projectNode=(Get-Command node -ErrorAction Stop).Source
$projectNpm=Join-Path (Split-Path -Parent $projectNode) 'node_modules/npm/bin/npm-cli.js'
if(-not(Test-Path -LiteralPath $projectNpm)){throw 'NPM_CLI_MISSING: 请安装固定版本Node，并确认安装目录包含npm'}
& $projectNode $projectNpm @NpmArgs
exit $LASTEXITCODE
