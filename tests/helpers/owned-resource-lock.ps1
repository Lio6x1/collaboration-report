param(
    [Parameter(Mandatory = $true)][string]$ProjectRoot,
    [Parameter(Mandatory = $true)][string]$StorageKey,
    [Parameter(Mandatory = $true)][string]$ReleaseSignal,
    [Parameter(Mandatory = $true)][int]$ParentProcessId
)
$ErrorActionPreference = 'Stop'
if ($StorageKey -notmatch '^[a-f0-9]{64}$') {
    throw 'RESOURCE_KEY_INVALID'
}
$projectAbsolute = [IO.Path]::GetFullPath($ProjectRoot)
$config = Get-Content -LiteralPath (Join-Path $projectAbsolute '.runtime/services/document.json') -Raw | ConvertFrom-Json
if ([IO.Path]::GetFullPath($config.root) -ne $projectAbsolute) {
    throw 'RESOURCE_PROJECT_MISMATCH'
}
$storageAbsolute = [IO.Path]::GetFullPath((Join-Path $projectAbsolute '.runtime/resources/document'))
if ($config.resources.root -and [IO.Path]::GetFullPath($config.resources.root) -ne $storageAbsolute) {
    throw 'RESOURCE_STORAGE_MISMATCH'
}
$fileAbsolute = [IO.Path]::GetFullPath((Join-Path $storageAbsolute $StorageKey))
if ([IO.Path]::GetDirectoryName($fileAbsolute) -ne $storageAbsolute) {
    throw 'RESOURCE_PATH_INVALID'
}
$signalAbsolute = [IO.Path]::GetFullPath($ReleaseSignal)
$signalDirectory = [IO.Path]::GetFullPath((Join-Path $projectAbsolute '.runtime/faults'))
$signalNamePattern = '^gc-release-' + $StorageKey + '-[a-f0-9-]{36}$'
if ([IO.Path]::GetDirectoryName($signalAbsolute) -ne $signalDirectory -or
    [IO.Path]::GetFileName($signalAbsolute) -notmatch $signalNamePattern) {
    throw 'RESOURCE_SIGNAL_PATH_INVALID'
}
$parent = [Diagnostics.Process]::GetProcessById($ParentProcessId)
$null = $parent.Handle
$heldFile = $null
try {
    # 真实Windows句柄允许读、禁止删除；不会改文件或把SQL状态伪造为DELETING。
    $heldFile = [IO.File]::Open(
        $fileAbsolute, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::Read
    )
    Write-Output '{"state":"LOCKED"}'
    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    # PowerShell宿主可能提前把Console.In视作EOF。使用本项目精确的一次性信号，
    # 句柄持续持有到父进程明确释放，避免无故障的正常GC被误写成失败恢复证明。
    while (-not (Test-Path -LiteralPath $signalAbsolute) -and
        -not $parent.HasExited -and [DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Milliseconds 10
    }
} finally {
    if ($heldFile) {
        $heldFile.Dispose()
    }
    $parent.Dispose()
    Write-Output '{"state":"RELEASED"}'
}
