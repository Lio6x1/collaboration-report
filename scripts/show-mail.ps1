param([string]$Email='', [ValidateRange(1,20)][int]$Limit=5)
$ErrorActionPreference='Stop'
$projectRoot=Split-Path -Parent $PSScriptRoot
$mailFolder=Join-Path $projectRoot '.runtime/smtp/inbox'
if(-not (Test-Path -LiteralPath $mailFolder)){throw 'SMTP_INBOX_MISSING: 请先启动开发收件器'}
# 隔离收件器不向浏览器开放公共收件箱。此脚本仅在本机显式读取自己的开发邮件。
# 验证码只用于本地演示，不能把输出当作日志上传或提交到Git。
$shown=0
foreach($file in (Get-ChildItem -LiteralPath $mailFolder -Filter '*.json' | Sort-Object LastWriteTimeUtc -Descending)){
 $mail=Get-Content -LiteralPath $file.FullName -Raw -Encoding UTF8 | ConvertFrom-Json
 if($Email -and ([string]$mail.recipient).Trim('<','>') -ne $Email){continue}
 Write-Host $mail.body
 $shown++
 if($shown -ge $Limit){break}
}
if($shown -eq 0){Write-Host '没有找到匹配邮件。请检查邮箱或先发送验证码。'}
