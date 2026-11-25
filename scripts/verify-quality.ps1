$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $projectRoot
$auditRoot = Join-Path $projectRoot 'artifacts/current-quality'
New-Item -ItemType Directory -Path $auditRoot -Force | Out-Null
$env:GOWORK = 'off'
$env:GOMAXPROCS = '4'
$env:GOCACHE = Join-Path $projectRoot '.runtime/go-cache'
$env:PYTHONDONTWRITEBYTECODE = '1'
$results = [Collections.Generic.List[object]]::new()
# 运行离线规则和构建。数据库及故障恢复用例须在独立实例执行。
function Check([string]$Name, [scriptblock]$Run) {
    $log = Join-Path $auditRoot ($Name + '.txt')
    $started = [DateTime]::UtcNow
    try {
        & $Run *> $log
        $code = $LASTEXITCODE
        if ((Get-Content -LiteralPath $log -Raw) -match '--- SKIP:|(?m)^# skipped [1-9]') { $code = 1 }
    } catch {
        $_ | Out-String | Add-Content -LiteralPath $log
        $code = 1
    }
    $results.Add(@{name=$Name; exitCode=$code; log=($Name + '.txt'); startedAt=$started.ToString('o')})
    Write-Host "$Name : $code"
}
Check 'format' { & ./scripts/npm.ps1 run format:check }
Check 'go-format' {
    # 排除各模块内忽略的运行诊断文件，并分组避免Windows命令行长度上限。
    $files = @(Get-ChildItem services,packages,contracts,tools -Recurse -Filter '*.go' -File |
        Where-Object { $_.FullName -notmatch '[\\/](\.runtime|node_modules|\.git|artifacts)[\\/]' } |
        ForEach-Object { $_.FullName })
    for ($offset = 0; $offset -lt $files.Count; $offset += 40) {
        $last = [Math]::Min($offset + 39, $files.Count - 1)
        $unformatted = @(& gofmt -l @($files[$offset..$last]))
        if ($LASTEXITCODE -ne 0 -or $unformatted.Count) { $unformatted; throw 'GO_FORMAT_FAILED' }
    }
}
foreach ($module in @('contracts','packages/content','packages/platform','packages/rebuild','tools/contractscheck','services/account','services/document','services/search')) {
    if (-not (Test-Path -LiteralPath (Join-Path $module 'go.mod'))) { continue }
    Check ($module.Replace('/','-') + '-vet') { & go -C $module vet -mod=readonly ./... }
}
Check 'node-unit' { & node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/unit/*.test.mjs }
Check 'collaboration-rules' {
    $files = @(Get-ChildItem services/collaboration/tests -Filter '*.test.mjs' | Where-Object { $_.Name -ne 'readiness.test.mjs' } | ForEach-Object { $_.FullName })
    & node --no-maglev --test --test-reporter=tap --test-concurrency=1 @files
}
Check 'content-rules' { & go -C contracts test -mod=readonly ./content -count=1 -v }
foreach ($module in @('account','document','search')) {
    Check ($module + '-domain') { & go -C ('services/' + $module) test -p=2 -mod=readonly ./internal/domain/... -count=1 -v }
}
Check 'authentication-errors' {
    & go -C services/account test -mod=readonly ./internal/logic/account -run '^(TestWeakNewPasswordsRejectedBeforeChallengesOrStorage|TestAuthenticationDistinguishesInvalidIdentityFromReadFailure|TestChallengeFailuresKeepTheirCategoryAcrossBusinessEntries)$' -count=1 -v
}
Check 'document-conflicts' {
    & go -C services/document test -mod=readonly ./internal/logic/document -run '^(TestResolutionBodyRejectsDuplicateStableBlockIdentities|TestResolutionBodyPreservesRepeatedInlineResourceReferences|TestResolutionRejectsOnlyDefiniteProposalOrTransferFailures)$' -count=1 -v
}
Check 'consumer-watermark' {
    & go -C services/search test -mod=readonly ./internal/logic/consumer -run '^TestWindowRejectsLostHistoryAndBrokerPositionAheadOfSQL$' -count=1 -v
}
Check 'generated-content' { & node --no-maglev scripts/generate-content.mjs --check }
Check 'schemas' { & python scripts/validate-schemas.py }
Check 'frontend-types' { & ./scripts/npm.ps1 run typecheck --workspace web }
Check 'frontend-build' { & ./scripts/npm.ps1 run build --workspace web }
$failed = @($results | Where-Object { $_.exitCode -ne 0 })
@{state=$(if ($failed.Count) {'FAIL'} else {'PASS'}); scope='OFFLINE_STATIC_UNIT_BUILD'; checks=$results.ToArray(); servicesStarted=$false; completedAt=[DateTime]::UtcNow.ToString('o')} |
    ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $auditRoot 'verification.json') -Encoding utf8
if ($failed.Count) { exit 1 }
