param([Parameter(Mandatory=$true)][int]$ProcessId)
$ErrorActionPreference='Stop'
if($ProcessId -le 0){throw 'PROCESS_ID_INVALID'}
# 已退出实例只需指定PID的内核缺失证据；避免为每个不存在PID加载慢CIM提供器。
# 只接收GetProcessById明确的ArgumentException，权限/系统异常继续失败，不能猜测退出。
try {
  $taskProcess=[System.Diagnostics.Process]::GetProcessById($ProcessId)
  $taskProcess.Dispose()
} catch [System.ArgumentException] {
  '{"state":"ABSENT"}';exit 0
}
# 只读取指定PID；权限不足必须报错，不能当作进程不存在跳过归属核验。
$p=Get-CimInstance Win32_Process -Filter "ProcessId = $ProcessId"
if($null -eq $p){ '{"state":"ABSENT"}';exit 0 }
if([string]::IsNullOrEmpty($p.CommandLine)){throw 'PROCESS_IDENTITY_UNAVAILABLE'}
@{state='PRESENT';pid=$p.ProcessId;commandLine=$p.CommandLine;executablePath=$p.ExecutablePath}|ConvertTo-Json -Compress
