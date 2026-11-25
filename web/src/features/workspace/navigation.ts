import { canonicalID } from '../search/rules';

// 页面地址只保存对象编号与工作区类型；编辑票据、会话和正文均由服务端重新核验。
export function documentPath(document: string) {
  return '/documents/' + encodeURIComponent(document);
}

export function editorPath(document: string, unit: string, kind: 'MAIN' | 'BRANCH') {
  return documentPath(document) + '/edit/' + kind.toLowerCase() + '/' + encodeURIComponent(unit);
}

export function documentRoute(path: string) {
  const match =
    /^\/documents\/([1-9][0-9]{0,19})(?:\/edit\/(main|branch)\/([1-9][0-9]{0,19}))?\/?$/.exec(path);
  if (!match || !canonicalID(match[1]) || (match[3] && !canonicalID(match[3]))) return null;
  return {
    id: match[1],
    editor: match[3]
      ? { unit: match[3], kind: match[2] === 'main' ? ('MAIN' as const) : ('BRANCH' as const) }
      : undefined,
  };
}

export type WorkspaceSection = 'libraries' | 'documents' | 'applications' | 'trash' | 'settings';
export interface WorkspaceLocation {
  section: WorkspaceSection;
  space: string;
  knowledgeBase: string;
}

// 地址只描述界面位置；刷新后仍必须从有权空间目录和知识库详情重新核验归属。
export function workspaceRoute(search: string, organization?: string): WorkspaceLocation {
  const query = new URLSearchParams(search);
  if ((query.get('organization_id') ?? '') !== (organization ?? ''))
    return { section: 'libraries', space: '', knowledgeBase: '' };
  const value = query.get('workspace') ?? 'libraries';
  const sections: WorkspaceSection[] = [
    'libraries',
    'documents',
    'applications',
    'trash',
    'settings',
  ];
  let section: WorkspaceSection = sections.includes(value as WorkspaceSection)
    ? (value as WorkspaceSection)
    : 'libraries';
  const space = query.get('space_id') ?? '',
    knowledgeBase = query.get('knowledge_base_id') ?? '';
  const kb = canonicalID(knowledgeBase) ? knowledgeBase : '';
  if ((organization && section === 'applications') || (section === 'settings' && !kb))
    section = 'libraries';
  return {
    section,
    space: canonicalID(space) ? space : '',
    knowledgeBase: ['documents', 'settings'].includes(section) ? kb : '',
  };
}

export function workspacePath(location: WorkspaceLocation, organization?: string) {
  const query = new URLSearchParams();
  if (organization && canonicalID(organization)) query.set('organization_id', organization);
  if (location.section !== 'libraries') query.set('workspace', location.section);
  if (canonicalID(location.space)) query.set('space_id', location.space);
  if (['documents', 'settings'].includes(location.section) && canonicalID(location.knowledgeBase))
    query.set('knowledge_base_id', location.knowledgeBase);
  const search = query.toString();
  return (organization ? '/organizations' : '/') + (search ? '?' + search : '');
}

// 只记录同源目录位置，按账号与标签会话隔离；返回后仍重新核验权限，不缓存正文或授权。
export function rememberWorkspace(actor: string, context: string) {
  if (!['/', '/organizations'].includes(location.pathname)) return;
  const organization = new URLSearchParams(location.search).get('organization_id') ?? '';
  if (location.pathname === '/organizations' && !canonicalID(organization)) return;
  const path = workspacePath(
    workspaceRoute(location.search, organization || undefined),
    organization || undefined,
  );
  try {
    sessionStorage.setItem('collaboration-report.workspace-return:' + actor + ':' + context, path);
  } catch {
    /* 存储受限时仍可回到默认工作台。 */
  }
}

export function workspaceReturn(actor: string, context: string) {
  try {
    const path =
      sessionStorage.getItem('collaboration-report.workspace-return:' + actor + ':' + context) ??
      '/';
    const url = new URL(path, location.origin);
    if (url.origin !== location.origin || !['/', '/organizations'].includes(url.pathname))
      return '/';
    const organization = new URLSearchParams(url.search).get('organization_id') ?? '';
    if (url.pathname === '/organizations' && !canonicalID(organization)) return '/';
    return workspacePath(
      workspaceRoute(url.search, organization || undefined),
      organization || undefined,
    );
  } catch {
    return '/';
  }
}
