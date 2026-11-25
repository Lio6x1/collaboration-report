import { ref, computed, reactive, watch, onBeforeUnmount } from 'vue';
import type { Document, DocumentPage } from '../../../../contracts/http/generated';
import { api, useAction, actionsBlocked, transportEpoch, explain } from '../../lib/http';
import { acceptPage, createCursorPage, pageCursor, resetPage } from '../../lib/pagination';

// 当前空间/会话变化立即丢弃旧列表；迟到响应不能把另一账号的回收站显示回来。
export function useTrash(space: () => string, restored: (id: string) => void) {
  const action = useAction();
  const documents = ref<Document[]>([]);
  const pagination = reactive(createCursorPage());
  const cursor = computed(() => pagination.next);
  const loading = ref(false);
  const readingError = ref('');
  const blocked = computed(() => actionsBlocked.value || loading.value);
  let generation = 0,
    request = 0,
    active = true;

  async function load(index = pagination.index) {
    const id = space(),
      current = generation,
      sequence = ++request;
    if (!id) return;
    const next = pageCursor(pagination, index);
    // 每次翻页重新核验授权，只保留入口游标；确认删除使用视图中的独立固定快照。
    documents.value = [];
    loading.value = true;
    readingError.value = '';
    try {
      const page = await api<DocumentPage>(
        `/v1/spaces/${id}/trash?limit=20` + (next ? '&cursor=' + encodeURIComponent(next) : ''),
      );
      if (!active || current !== generation || sequence !== request) return;
      documents.value = page.items;
      acceptPage(pagination, index, next, page.next_cursor);
    } catch (error) {
      if (active && current === generation && sequence === request) {
        documents.value = [];
        readingError.value = explain(error);
      }
    } finally {
      if (active && current === generation && sequence === request) loading.value = false;
    }
  }
  function refresh() {
    // Doc游标有有效期；手动刷新重新建链，固定确认对象和原写入恢复器保持独立。
    resetPage(pagination);
    return load(0);
  }

  async function restore(document: Document) {
    const current = generation;
    await action.run(async () => {
      // 输入由useAction先固定原号及revision，回复未知沿原POST/GET恢复。
      // 只有服务端真实SUCCEEDED才移除回收项或刷新正常目录。
      await action.mutate(
        () => `/v1/documents/${document.id}/restore`,
        () => ({ expected_control_revision: document.control_revision }),
      );
      if (!active || current !== generation) return;
      restored(document.id);
      await load();
    }, '文档已恢复，正式阅读保持关闭；请重新核对当前权限');
  }

  async function purge(document: Document) {
    const current = generation;
    await action.run(async () => {
      // 二次确认由视图提供；固定同一修订和原号，直到实际Node清理及Doc终态成功。
      await action.mutate(
        () => `/v1/documents/${document.id}/purge`,
        () => ({ expected_control_revision: document.control_revision, confirmed: true }),
      );
      if (!active || current !== generation) return;
      await load();
    }, '文档及其内容历史已永久删除');
  }

  watch(
    [space, transportEpoch],
    () => {
      generation++;
      documents.value = [];
      resetPage(pagination);
      loading.value = false;
      readingError.value = '';
      void load();
    },
    { immediate: true, flush: 'sync' },
  );
  onBeforeUnmount(() => {
    active = false;
    generation++;
  });
  return {
    action,
    documents,
    pagination,
    cursor,
    loading,
    readingError,
    blocked,
    load,
    refresh,
    restore,
    purge,
  };
}
