param([Parameter(Mandatory=$true)][string]$Path)
$ErrorActionPreference='Stop'
try {
  if(-not [IO.Path]::IsPathRooted($Path) -or $Path.StartsWith('\\')){throw 'SMTP_CREDENTIAL_PATH_INVALID'}
  $smtpFile=Get-Item -LiteralPath $Path -Force
  if($smtpFile.PSIsContainer){throw 'SMTP_CREDENTIAL_FILE_INVALID'}
  $smtpCurrent=$smtpFile
  while($null -ne $smtpCurrent){
    if($smtpCurrent.Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'SMTP_CREDENTIAL_LINK_REJECTED'}
    $smtpCurrent=if($smtpCurrent.PSIsContainer){$smtpCurrent.Parent}else{$smtpCurrent.Directory}
  }
  # Windows FileMode不是ACL。替换整个DACL，包含旧的显式授权，而非只关继承。
  # 只读取文件元数据，不读取或输出JSON里的SMTP授权码。
  Import-Module (Join-Path $PSHOME 'Modules/Microsoft.PowerShell.Security/Microsoft.PowerShell.Security.psd1') -ErrorAction Stop
  $smtpIdentity=[Security.Principal.WindowsIdentity]::GetCurrent().User
  $smtpSecurity=New-Object Security.AccessControl.FileSecurity
  $smtpSecurity.SetAccessRuleProtection($true,$false)
  $smtpRule=New-Object Security.AccessControl.FileSystemAccessRule($smtpIdentity,'FullControl','Allow')
  $smtpSecurity.AddAccessRule($smtpRule)
  if($PSVersionTable.PSEdition -eq 'Core'){
    [IO.FileSystemAclExtensions]::SetAccessControl($smtpFile,$smtpSecurity)
  }else{
    [IO.File]::SetAccessControl($smtpFile.FullName,$smtpSecurity)
  }
  Write-Output '{"state":"PROTECTED"}'
}catch{
  Write-Output '{"state":"FAILED","code":"SMTP_CREDENTIAL_PROTECTION_FAILED"}'
  exit 1
}
