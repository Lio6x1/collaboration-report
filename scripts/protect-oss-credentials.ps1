param([Parameter(Mandatory=$true)][string]$Path)
$ErrorActionPreference='Stop'
$taskStage='PATH_CHECK'
try {
 if(-not [IO.Path]::IsPathRooted($Path)){throw 'OSS_CREDENTIAL_PATH_INVALID'}
 $taskStage='READ_FILE_METADATA'
 $taskFile=Get-Item -LiteralPath $Path -Force
 if($taskFile.PSIsContainer -or ($taskFile.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'OSS_CREDENTIAL_FILE_INVALID'}
 $taskStage='READ_ANCESTORS'
 $taskCurrent=$taskFile
 while($null -ne $taskCurrent){
  if($taskCurrent.Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'OSS_CREDENTIAL_LINK_REJECTED'}
  $taskCurrent=if($taskCurrent.PSIsContainer){$taskCurrent.Parent}else{$taskCurrent.Directory}
 }
 # Replace the DACL, including explicit old grants; changing inheritance alone is insufficient.
 $taskStage='BUILD_DACL'
 # Node can inherit PowerShell 7's PSModulePath while launching Windows PowerShell 5.
 # Load this host's own module explicitly instead of autoloading an incompatible one.
 Import-Module (Join-Path $PSHOME 'Modules/Microsoft.PowerShell.Security/Microsoft.PowerShell.Security.psd1') -ErrorAction Stop
 $taskIdentity=[Security.Principal.WindowsIdentity]::GetCurrent().User
 # Only persist the DACL. Set-Acl also attempts owner/group operations and can
 # require SeRestorePrivilege for an existing file even when ownership is unchanged.
 $taskExistingSecurity=Get-Acl -LiteralPath $taskFile.FullName
 $taskExistingRules=@($taskExistingSecurity.GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier]))
 if($taskExistingSecurity.AreAccessRulesProtected -and $taskExistingRules.Count -eq 1 -and
    $taskExistingRules[0].IdentityReference -eq $taskIdentity -and
    $taskExistingRules[0].AccessControlType -eq 'Allow' -and
    ($taskExistingRules[0].FileSystemRights -band [Security.AccessControl.FileSystemRights]::FullControl) -eq [Security.AccessControl.FileSystemRights]::FullControl){
  Write-Output '{"state":"PROTECTED"}'
  return
 }
 $taskSecurity=New-Object Security.AccessControl.FileSecurity
 $taskSecurity.SetAccessRuleProtection($true,$false)
 $taskRule=New-Object Security.AccessControl.FileSystemAccessRule($taskIdentity,'FullControl','Allow')
 $taskSecurity.AddAccessRule($taskRule)
 $taskStage='SET_DACL'
 if($PSVersionTable.PSEdition -eq 'Core'){
  [IO.FileSystemAclExtensions]::SetAccessControl($taskFile,$taskSecurity)
 }else{
  [IO.File]::SetAccessControl($taskFile.FullName,$taskSecurity)
 }
 Write-Output '{"state":"PROTECTED"}'
}catch{
 @{state='FAILED';code='OSS_CREDENTIAL_PROTECTION_FAILED';stage=$taskStage;
   category=[string]$_.CategoryInfo.Category;errorId=$_.FullyQualifiedErrorId} | ConvertTo-Json -Compress
 exit 1
}
