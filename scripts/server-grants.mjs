// 初始化按原计划核验权限；解析只归一SHOW GRANTS的表现差异，不能忽略转授权能力。
export function canonicalGrant(statement) {
  // 转授权是额外能力；不能在抹掉TO后缀后把它当成原计划权限，也不能过滤USAGE时漏掉。
  // 发现原账号漂移只拒绝初始化，保留现场，由操作者明确修复，不自动撤权或改密码。
  if (/\bWITH\s+GRANT\s+OPTION\b/i.test(statement))
    throw Error('SERVER_ACCOUNT_GRANT_OPTION_NOT_ALLOWED');
  const match = /^GRANT (.+) ON (.+) TO /i.exec(statement.trim());
  if (!match) throw Error('SERVER_GRANT_FORMAT_INVALID');
  // SHOW GRANTS会调整权限顺序并给列名加反引号；只消除这些表现差异，不放宽权限。
  const privileges = match[1]
    .replaceAll('`', '')
    .replaceAll(/\s/g, '')
    .toUpperCase()
    .split(/,(?![^()]*\))/)
    .sort()
    .join(',');
  return privileges + ' ON ' + match[2].replaceAll('`', '').replaceAll(/\s/g, '');
}
