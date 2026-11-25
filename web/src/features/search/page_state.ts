import type { SearchResult, SearchResultPage } from '../../../../contracts/http/generated';
import { checkedPage, searchFailure, searchPath } from './rules.ts';
import type { SearchQuery } from './rules.ts';

export type SearchPageState = {
  items: readonly SearchResult[];
  nextCursor: string | null;
  page: number;
  loading: boolean;
  searched: boolean;
  error: string;
  retryable: boolean;
};
export type SearchLoader = (query: SearchQuery, cursor: string | null) => Promise<SearchResultPage>;
export function createSearchPager(
  load: SearchLoader,
  identity: () => string,
  changed: (state: SearchPageState) => void,
) {
  type Request = { query: SearchQuery; cursor: string | null; page: number; identity: string };
  const empty = (): SearchPageState => ({
    items: [],
    nextCursor: null,
    page: 0,
    loading: false,
    searched: false,
    error: '',
    retryable: false,
  });
  let state = empty();
  let version = 0;
  let disposed = false;
  let pending: Request | null = null;
  let completed: Request | null = null;
  function publish(next: SearchPageState) {
    state = next;
    if (!disposed) changed(next);
  }
  function reset(message = '') {
    version++;
    pending = completed = null;
    publish({ ...empty(), error: message });
  }
  async function run(request: Request) {
    if (disposed) return;
    const original = ++version;
    pending = request;
    completed = null;
    // 翻页也重新授权整页；不累计过去交付的摘要，在失败后继续冒充当前可读结果。
    publish({ ...empty(), loading: true, searched: true });
    try {
      searchPath(request.query, request.cursor);
      const page = checkedPage(await load(request.query, request.cursor), request.query.limit);
      if (original !== version || disposed) return;
      if (identity() !== request.identity) {
        reset('登录身份已变化，请重新搜索。');
        return;
      }
      pending = null;
      completed = request;
      publish({
        ...empty(),
        items: [...page.items],
        nextCursor: page.next_cursor,
        page: request.page,
        searched: true,
      });
    } catch (error) {
      if (original !== version || disposed) return;
      if (identity() !== request.identity) {
        reset('登录身份已变化，请重新搜索。');
        return;
      }
      const failure = searchFailure(error);
      if (!failure.retryable) pending = null;
      publish({ ...empty(), searched: true, error: failure.message, retryable: failure.retryable });
    }
  }
  async function search(query: SearchQuery) {
    // 捕获原输入对象；后来v-model变化不能修改已经发出或待重试的原请求。
    await run({ query: Object.freeze({ ...query }), cursor: null, page: 1, identity: identity() });
  }
  async function next() {
    if (!completed || state.loading || !state.nextCursor) return;
    if (completed.identity !== identity()) return reset('登录身份已变化，请重新搜索。');
    await run({ ...completed, cursor: state.nextCursor, page: state.page + 1 });
  }
  async function retry() {
    if (!pending || state.loading || !state.retryable) return;
    if (pending.identity !== identity()) return reset('登录身份已变化，请重新搜索。');
    await run(pending);
  }
  function dispose() {
    disposed = true;
    version++;
    pending = completed = null;
  }
  publish(state);
  return { search, next, retry, reset, dispose };
}
