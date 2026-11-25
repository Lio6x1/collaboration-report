// 只记每页入口游标，不缓存列表内容；上一页同样重新经过服务端当前授权。
export type CursorPage = { index: number; next: string | null; cursors: (string | null)[] };

export function createCursorPage(): CursorPage {
  return { index: 0, next: null, cursors: [null] };
}

export function pageCursor(state: CursorPage, index: number): string | null {
  if (!Number.isInteger(index) || index < 0) throw new RangeError('分页目标无效，请刷新列表。');
  if (index === 0) return null;
  if (index === state.index + 1 && state.next) return state.next;
  if (index <= state.index && state.cursors[index]) return state.cursors[index];
  throw new RangeError('该页尚不可用，请从当前页继续。');
}

export function acceptPage(
  state: CursorPage,
  index: number,
  cursor: string | null,
  next: string | null,
) {
  // 仅在原请求与范围仍有效时调用；重读前页会使原先更后面的游标链失效。
  state.cursors = [...state.cursors.slice(0, index), cursor];
  state.index = index;
  state.next = next;
}

export function resetPage(state: CursorPage) {
  Object.assign(state, createCursorPage());
}
