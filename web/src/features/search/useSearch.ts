import { computed, onUnmounted, ref, shallowRef, watch } from 'vue';
import { identityUnavailable, sessionReady, user } from '../account/session';
import { actionsBlocked, transportEpoch } from '../../lib/http';
import { readSearchPage } from './api';
import { createSearchPager } from './page_state';
import type { SearchPageState } from './page_state';

export function useSearch(initialSpace: () => string) {
  const text = ref('');
  const limit = ref(20);
  const space = ref(initialSpace());
  const state = shallowRef<SearchPageState>();
  const identity = computed(() => `${transportEpoch.value}:${user.value?.id ?? 'anonymous'}`);
  const ready = computed(
    () => sessionReady.value && !identityUnavailable.value && !actionsBlocked.value,
  );
  const pager = createSearchPager(
    readSearchPage,
    () => identity.value,
    (value) => (state.value = value),
  );

  // 同一个账号重新登录也会改变transportEpoch。同步失效先清旧页，晚到的200不能重新显示。
  watch(
    identity,
    () => {
      pager.reset('登录身份已变化，请重新搜索。');
      space.value = '';
    },
    { flush: 'sync' },
  );
  watch(
    identityUnavailable,
    (unavailable) => {
      if (unavailable) pager.reset('当前身份资格待核验，请连接恢复后重新搜索。');
    },
    { flush: 'sync' },
  );
  watch([text, limit, space], () => pager.reset(), { flush: 'sync' });
  watch(initialSpace, (value) => {
    pager.reset();
    space.value = value;
  });
  function available() {
    if (ready.value) return true;
    pager.reset('当前身份或操作尚待核验，请稍后重新搜索。');
    return false;
  }
  const search = () =>
    available()
      ? pager.search({ text: text.value, limit: limit.value, space: space.value })
      : undefined;
  const next = () => (available() ? pager.next() : undefined);
  const retry = () => (available() ? pager.retry() : undefined);
  onUnmounted(pager.dispose);
  return { text, limit, space, state, ready, search, next, retry };
}
