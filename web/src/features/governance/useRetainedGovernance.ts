import { computed, onBeforeUnmount, ref, watch } from 'vue';
import type { Document } from '../../../../contracts/http/generated';
import { actionsBlocked, api, explain, transportEpoch, useAction } from '../../lib/http';
import { canonicalID } from '../search/rules';

// 治理只消费专用元数据接口，不借工作台正文权限。角色是否有效由服务端每次核验。
export function useRetainedGovernance() {
  const action = useAction();
  const documentID = ref('');
  const document = ref<Document | null>(null);
  const loading = ref(false);
  const readingError = ref('');
  const blocked = computed(() => actionsBlocked.value || loading.value);
  let generation = 0;
  let request = 0;
  let active = true;

  async function load() {
    const id = documentID.value.trim();
    const current = generation;
    const sequence = ++request;
    document.value = null;
    readingError.value = '';
    if (!canonicalID(id)) {
      readingError.value = '请输入有效的留存文档编号。';
      return;
    }
    loading.value = true;
    try {
      const value = await api<Document>(`/v1/retained-public/documents/${id}`);
      if (!active || generation !== current || request !== sequence) return;
      document.value = value;
    } catch (error) {
      if (active && generation === current && request === sequence)
        readingError.value = explain(error);
    } finally {
      if (active && generation === current && request === sequence) loading.value = false;
    }
  }

  async function govern(kind: 'UNPUBLISH' | 'PURGE') {
    if (!document.value || blocked.value) return;
    // 在启动动作之前固定目标和修订；未知回复后的恢复不能取另一篇文章的新输入。
    const fixed = { ...document.value };
    const current = generation;
    const path =
      `/v1/retained-public/documents/${fixed.id}` + (kind === 'UNPUBLISH' ? '/unpublish' : '');
    await action.run(
      async () => {
        await action.mutate(
          path,
          { expected_revision: fixed.control_revision },
          kind === 'PURGE' ? 'DELETE' : 'POST',
        );
        if (!active || current !== generation) return;
        if (kind === 'UNPUBLISH') await load();
        else document.value = null;
      },
      kind === 'UNPUBLISH' ? '留存文章已撤下，正式阅读已关闭' : '留存文章及其内容历史已永久删除',
    );
  }

  watch([documentID, transportEpoch], () => {
    generation++;
    document.value = null;
    readingError.value = '';
    loading.value = false;
  });
  onBeforeUnmount(() => {
    active = false;
    generation++;
  });
  return { action, documentID, document, loading, readingError, blocked, load, govern };
}
