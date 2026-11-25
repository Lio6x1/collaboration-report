import type { SearchResult, SearchResultPage } from '../../../../contracts/http/generated';

// 这里只保存查询输入，不复制HTTP结果DTO；scope与session证明仍由服务端从真实Cookie生成。
export type SearchQuery = { text: string; limit: number; space: string };
export class SearchInputError extends Error {
  readonly code: string;
  constructor(code: string) {
    super(code);
    this.code = code;
  }
}
export function canonicalID(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    /^[1-9][0-9]{0,19}$/.test(value) &&
    BigInt(value) <= 18446744073709551615n
  );
}
function unicodeLength(value: string): number {
  let count = 0;
  for (const rune of value) {
    const point = rune.codePointAt(0)!;
    // URLSearchParams会静默替换孤立代理项，必须在传输前拒绝，不能改变原查询词法。
    if (point >= 0xd800 && point <= 0xdfff) return -1;
    count++;
  }
  return count;
}
function validCursor(value: unknown): value is string {
  if (typeof value !== 'string' || value.length === 0 || value.length > 2048) return false;
  if (!/^[A-Za-z0-9_-]+$/.test(value) || value.length % 4 === 1) return false;
  const tail = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_'.indexOf(
    value.at(-1)!,
  );
  // 只核规范base64url尾位，不解密/解释内容，也不从cursor推算任何权限或期限。
  return value.length % 4 === 2 ? tail % 16 === 0 : value.length % 4 === 3 ? tail % 4 === 0 : true;
}
export function searchPath(query: SearchQuery, cursor: string | null): string {
  const size = unicodeLength(query.text);
  const whitespace =
    /^[\u0009-\u000d\u0020\u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+$/u;
  if (size < 1 || size > 500 || whitespace.test(query.text))
    throw new SearchInputError('SEARCH_QUERY_INVALID');
  if (!Number.isInteger(query.limit) || query.limit < 1 || query.limit > 100)
    throw new SearchInputError('SEARCH_PARAMETERS_INVALID');
  if (query.space !== '' && !canonicalID(query.space))
    throw new SearchInputError('SEARCH_PARAMETERS_INVALID');
  if (cursor !== null && !validCursor(cursor)) throw new SearchInputError('CURSOR_INVALID');
  const parameters = new URLSearchParams({ q: query.text, limit: String(query.limit) });
  if (query.space) parameters.set('space_id', query.space);
  if (cursor !== null) parameters.set('cursor', cursor);
  return '/v1/search?' + parameters;
}
export function searchResultPath(result: SearchResult): string {
  if (!canonicalID(result.document_id) || !canonicalID(result.release_id))
    throw new SearchInputError('SEARCH_RESPONSE_INVALID');
  const path = '/public/documents/' + result.document_id;
  if (result.chapter_id === undefined) return path;
  if (!canonicalID(result.chapter_id)) throw new SearchInputError('SEARCH_RESPONSE_INVALID');
  // 正式链接只携带章节身份，打开时重新取得当前正式版，不能选择搜索时的历史release。
  return path + '#chapter-' + result.chapter_id;
}

export function searchExcerpt(snippet: string, query: string): string {
  // 只裁剪Doc已经授权的摘要，不从ES或未发布主线补正文；Vue仍按普通文本展示。
  const text = snippet.replace(/\s+/gu, ' ').trim();
  const needle = query.replace(/\s+/gu, ' ').trim().toLowerCase();
  const match = needle ? text.toLowerCase().indexOf(needle) : -1;
  const characters = Array.from(text);
  const start = match < 0 ? 0 : Math.max(0, Array.from(text.slice(0, match)).length - 48);
  const end = Math.min(characters.length, start + 240);
  // 按Unicode字符截取，避免将中文或emoji拆成损坏字符；空白行也不能撑满搜索列表。
  return (
    (start ? '…' : '') +
    characters.slice(start, end).join('') +
    (end < characters.length ? '…' : '')
  );
}

export function checkedPage(value: unknown, limit: number): SearchResultPage {
  const invalid = () => new SearchInputError('SEARCH_RESPONSE_INVALID');
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw invalid();
  const page = value as SearchResultPage;
  if (
    Object.keys(page).sort().join(',') !== 'has_more,items,next_cursor' ||
    !Array.isArray(page.items) ||
    page.items.length > limit ||
    typeof page.has_more !== 'boolean' ||
    (page.has_more ? !validCursor(page.next_cursor) : page.next_cursor !== null)
  )
    throw invalid();
  const seen = new Set<string>();
  for (const item of page.items) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) throw invalid();
    const keys = Object.keys(item).sort().join(',');
    if (
      ![
        'document_id,release_id,snippet,title',
        'chapter_id,document_id,release_id,snippet,title',
      ].includes(keys) ||
      !canonicalID(item.document_id) ||
      !canonicalID(item.release_id) ||
      seen.has(item.document_id) ||
      typeof item.title !== 'string' ||
      typeof item.snippet !== 'string' ||
      unicodeLength(item.title) < 0 ||
      unicodeLength(item.title) > 240 ||
      unicodeLength(item.snippet) < 0 ||
      unicodeLength(item.snippet) > 2000
    )
      throw invalid();
    searchResultPath(item);
    seen.add(item.document_id);
  }
  return page;
}
export function searchFailure(error: unknown): { message: string; retryable: boolean } {
  const code = error && typeof error === 'object' && 'code' in error ? String(error.code) : '';
  const status = error && typeof error === 'object' && 'status' in error ? Number(error.status) : 0;
  const fixed: Record<string, string> = {
    SEARCH_QUERY_INVALID: '请输入1～500个字符的查询词，不能只有空白。',
    SEARCH_PARAMETERS_INVALID: '查询范围或每页数量无效，请重新选择。',
    CURSOR_INVALID: '翻页凭据无效，请重新搜索。',
    CURSOR_EXPIRED: '翻页已到期，请重新搜索。',
    CURSOR_STALE: '查询范围、会话或索引已变化，请重新搜索。',
    SEARCH_CURSOR_EXPIRED: '翻页已到期，请重新搜索。',
    SEARCH_CURSOR_STALE: '索引或翻页状态已变化，请重新搜索。',
    PAGE_CHANGED_RETRY: '正式版或可见范围已变化，请重新搜索。',
    SESSION_CHANGED: '登录身份已变化，请重新搜索。',
    SEARCH_RESPONSE_INVALID: '搜索返回无效，未展示部分结果，请重新搜索。',
  };
  if (fixed[code]) return { message: fixed[code], retryable: false };
  if (status === 401 || code === 'SESSION_INVALID' || code === 'SESSION_OR_CSRF_INVALID')
    return { message: '会话已失效，请登录后重新搜索。', retryable: false };
  if (status === 403)
    return { message: '当前身份无法读取此范围，请重新核验资格。', retryable: false };
  if (status >= 400 && status < 500)
    return { message: '查询条件或翻页状态已失效，请重新搜索。', retryable: false };
  // 超时/503不能当成空页或成功；显式重试仍绑定原输入，不自动改成匿名或第一页。
  return { message: '搜索未完成，未展示部分结果。可以重试本次查询。', retryable: true };
}
