import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue';
import { createCursorPage, pageCursor, acceptPage, resetPage } from '../../lib/pagination';
import type { Action, Notification } from '../../../../contracts/http/generated';
import { actionsBlocked, ApiError, explain, transportEpoch, useAction } from '../../lib/http';
import { notificationReadPath, readNotifications } from './api';

export function useNotifications() {
  const action = useAction();
  const notifications = ref<Notification[]>([]);
  const cursor = ref<string | null>(null);
  const pageNav = reactive(createCursorPage());
  const loading = ref(false);
  const readingError = ref('');
  const blocked = computed(() => actionsBlocked.value || loading.value);
  let active = true;
  let generation = 0;
  let request = 0;

  async function fetchPage(index = 0) {
    const started = generation;
    const sequence = ++request;
    const next = pageCursor(pageNav, index);
    notifications.value = [];
    loading.value = true;
    try {
      const page = await readNotifications(next);
      if (!active || started !== generation || sequence !== request) return;
      notifications.value = page.items;
      acceptPage(pageNav, index, next, page.next_cursor);
      // 游标来自服务端已授权结果；空页不等于结束，也不把当前页未读数冒充全部未读数。
      cursor.value = page.next_cursor;
    } catch (error) {
      if (
        active &&
        started === generation &&
        sequence === request &&
        error instanceof ApiError &&
        [401, 403].includes(error.status)
      ) {
        notifications.value = [];
        cursor.value = null;
        resetPage(pageNav);
      }
      throw error;
    } finally {
      if (sequence === request) loading.value = false;
    }
  }

  async function read(index = 0) {
    if (loading.value) return;
    const started = generation;
    readingError.value = '';
    try {
      await fetchPage(index);
    } catch (error) {
      if (active && started === generation) readingError.value = explain(error);
    }
  }

  async function markRead(notification: Notification) {
    if (
      blocked.value ||
      notification.read_at ||
      !notifications.value.some((item) => item.id === notification.id)
    )
      return;
    const started = generation;
    const path = notificationReadPath(notification.id);
    const input: Omit<Action, 'operation_id'> = {};
    readingError.value = '';
    await action.run(async () => {
      // 同号恢复原已读操作；只有服务端回执确认成功后才重新读取真实read_at，不自行写客户端时间。
      await action.mutate(path, input);
      if (active && started === generation) await fetchPage(pageNav.index);
    }, '通知已标记为已读');
  }

  watch(
    transportEpoch,
    () => {
      generation++;
      request++;
      resetPage(pageNav);
      notifications.value = [];
      cursor.value = null;
      readingError.value = '';
      loading.value = false;
      if (!action.pending.value && !action.busy.value) {
        action.error.value = '';
        action.notice.value = '';
      }
      // 旧请求和未知写入继续由各自身份边界约束，新会话只能读取自己的列表。
      void read();
    },
    { immediate: true },
  );
  onBeforeUnmount(() => {
    active = false;
    generation++;
    request++;
  });

  return {
    action,
    notifications,
    cursor,
    loading,
    readingError,
    blocked,
    refresh: () => read(),
    pageNav,
    turn: (index: number) => read(index),
    more: () => read(pageNav.index + 1),
    markRead,
  };
}
