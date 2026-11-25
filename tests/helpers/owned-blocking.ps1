param([Parameter(Mandatory=$true)][string]$ProjectRoot, [Parameter(Mandatory=$true)][int]$ParentProcessId)
$ErrorActionPreference='Stop'
$Role=$env:COLLAB_TEST_BLOCK_ROLE
if($Role -notin @('collaboration','edge')){throw 'OWNED_BLOCKING_ROLE_INVALID'}
$projectAbsolute=[IO.Path]::GetFullPath($ProjectRoot)
$ownedItems=Get-Content -LiteralPath (Join-Path $projectAbsolute '.runtime/processes.json') -Raw | ConvertFrom-Json
$ownedNode=@($ownedItems | Where-Object { $_.role -eq $Role })
if($ownedNode.Count -ne 1){throw 'OWNED_NODE_REQUIRED'}
$ownedNode=$ownedNode[0]
$roleEntry=@{collaboration='services/collaboration/src/main.mjs';edge='scripts/edge.mjs'}
$expectedEntry=Join-Path $projectAbsolute $roleEntry[$Role]
if([IO.Path]::GetFullPath($ownedNode.entry) -ne [IO.Path]::GetFullPath($expectedEntry)){throw 'PROCESS_ENTRY_MISMATCH'}
$heldProcess=[Diagnostics.Process]::GetProcessById([int]$ownedNode.pid)
# 固定原进程对象，随后即使 PID 被复用也不对替代进程实施故障。
$null=$heldProcess.Handle
$facts=Get-CimInstance Win32_Process -Filter "ProcessId=$([int]$ownedNode.pid)"
if(-not $facts -or $heldProcess.HasExited -or -not $facts.CommandLine.Contains($ownedNode.entry) -or -not $facts.CommandLine.Contains('--run-owner '+$ownedNode.owner)){throw 'PROCESS_OWNER_MISMATCH'}
if([IO.Path]::GetFileName($facts.ExecutablePath) -ne [IO.Path]::GetFileName($ownedNode.executable)){throw 'PROCESS_EXECUTABLE_MISMATCH'}

# 这是有限故障注入，不能用于业务线程同步；每个成功增加的 suspend count 在 finally 恰好减一次。
# https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-suspendthread
# https://learn.microsoft.com/en-us/windows/desktop/api/processthreadsapi/nf-processthreadsapi-resumethread
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class OwnedThreadFault {
    [DllImport("kernel32.dll", SetLastError=true)] public static extern IntPtr OpenThread(uint access, bool inherit, uint id);
    [DllImport("kernel32.dll", SetLastError=true)] public static extern uint GetProcessIdOfThread(IntPtr thread);
    [DllImport("kernel32.dll", SetLastError=true)] public static extern uint SuspendThread(IntPtr thread);
    [DllImport("kernel32.dll", SetLastError=true)] public static extern uint ResumeThread(IntPtr thread);
    [DllImport("kernel32.dll", SetLastError=true)] public static extern bool CloseHandle(IntPtr handle);
}
'@
$heldThreads=[Collections.Generic.List[IntPtr]]::new()

function Wait-FaultCommand([int]$BudgetSeconds) {
    $inputTask=[Console]::In.ReadLineAsync()
    $deadline=[DateTime]::UtcNow.AddSeconds($BudgetSeconds)
    while(-not $inputTask.IsCompleted -and [DateTime]::UtcNow -lt $deadline){
        try { if([Diagnostics.Process]::GetProcessById($ParentProcessId).HasExited){return $null} }
        catch { return $null }
        Start-Sleep -Milliseconds 10
    }
    if($inputTask.IsCompleted){return $inputTask.GetAwaiter().GetResult()}
    return $null
}

try {
    # 先准备原生适配器，再由实际业务等待点发BLOCK，避免编译时间越过HTTP原预算。
    Write-Output '{"state":"PREPARED"}'
    if((Wait-FaultCommand 10) -ne 'BLOCK'){return}
    foreach($thread in $heldProcess.Threads){
        if($heldProcess.HasExited){throw 'OWNED_PROCESS_EXITED'}
        $handle=[OwnedThreadFault]::OpenThread(0x0802,$false,[uint32]$thread.Id)
        if($handle -eq [IntPtr]::Zero){throw 'THREAD_HANDLE_UNAVAILABLE'}
        if([OwnedThreadFault]::GetProcessIdOfThread($handle) -ne [uint32]$ownedNode.pid){
            $null=[OwnedThreadFault]::CloseHandle($handle)
            throw 'THREAD_OWNER_MISMATCH'
        }
        $prior=[OwnedThreadFault]::SuspendThread($handle)
        if($prior -eq [uint32]::MaxValue){
            $null=[OwnedThreadFault]::CloseHandle($handle)
            throw 'THREAD_SUSPEND_FAILED'
        }
        $heldThreads.Add($handle)
        if($prior -ne 0){throw 'THREAD_ALREADY_SUSPENDED'}
    }
    if($heldThreads.Count -eq 0){throw 'ACTUAL_THREADS_REQUIRED'}
    Write-Output ('{"state":"BLOCKED","threadCount":'+$heldThreads.Count+'}')
    # 辅助进程独立负责自动恢复：测试进程退出、输入断开或15秒上限都进入finally。
    $null=Wait-FaultCommand 15
} finally {
    $resumeFailures=0
    foreach($handle in $heldThreads){
        if([OwnedThreadFault]::ResumeThread($handle) -eq [uint32]::MaxValue){$resumeFailures++}
        $null=[OwnedThreadFault]::CloseHandle($handle)
    }
    $heldProcess.Dispose()
    if($resumeFailures -ne 0){throw 'THREAD_RESUME_FAILED'}
    Write-Output '{"state":"RESUMED"}'
}
