param(
    [Parameter(Mandatory=$true)][string]$InputPath,
    [Parameter(Mandatory=$true)][string]$OutputPath
)
$ErrorActionPreference='Stop'
# Windows验收使用本机Word的真实排版。已有用户Word进程时拒绝接管，
# 只关闭本脚本新建的隐藏应用，源DOCX以只读打开且不保存任何修改。
if (Get-Process -Name WINWORD -ErrorAction SilentlyContinue) {
    throw 'WORD_QA_REQUIRES_NO_EXISTING_WORD_PROCESS'
}
$wordInput=(Resolve-Path -LiteralPath $InputPath).Path
$wordOutput=[IO.Path]::GetFullPath($OutputPath)
$wordApplication=$null
$wordDocument=$null
$wordSaveChanges=[object]0
try {
    $wordApplication=New-Object -ComObject Word.Application
    $wordApplication.Visible=$false
    $wordApplication.DisplayAlerts=0
    $wordApplication.AutomationSecurity=3
    $wordDocument=$wordApplication.Documents.Open($wordInput,$false,$true)
    $wordDocument.ExportAsFixedFormat($wordOutput,17)
    Write-Output 'WORD_NATIVE_RENDER_PASS'
} finally {
    if ($wordDocument) {
        try { $wordDocument.Close([ref]$wordSaveChanges) }
        finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($wordDocument) }
    }
    if ($wordApplication) {
        try { $wordApplication.Quit([ref]$wordSaveChanges) }
        finally { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($wordApplication) }
    }
}
