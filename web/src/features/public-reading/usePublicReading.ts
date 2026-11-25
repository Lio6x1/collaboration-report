import { onMounted, onUnmounted, ref, watch } from 'vue';
import type { PublicDocument } from '../../../../contracts/http/generated';
import { user } from '../account/session';
import { ApiError, explain, transportEpoch } from '../../lib/http';
import { readCurrentPublication } from './api';

export function usePublicReading(document: () => string) {
  const current = ref<PublicDocument | null>(null),
    loading = ref(false),
    error = ref('');
  let request = 0;
  async function refresh() {
    const original = ++request;
    // 重新核验时清空旧正文；404、撤下或资格变化都不能让旧版本继续冒充当前正式版。
    current.value = null;
    error.value = '';
    loading.value = true;
    try {
      const value = await readCurrentPublication(document());
      if (original === request) current.value = value;
    } catch (failure) {
      if (original !== request) return;
      error.value =
        failure instanceof ApiError && failure.status === 404
          ? '当前没有可阅读的正式版'
          : failure instanceof ApiError && failure.status === 401
            ? '此正式版需要有效登录身份，请登录后重新打开'
            : failure instanceof ApiError && failure.status === 403
              ? '当前身份不在此正式版的可见范围内'
              : explain(failure);
    } finally {
      if (original === request) loading.value = false;
    }
  }
  // 会话切换后重新走真实Cookie授权；页面不会通过删除凭据伪装成匿名资格。
  watch(() => `${document()}:${user.value?.id ?? 'anonymous'}:${transportEpoch.value}`, refresh);
  onMounted(refresh);
  onUnmounted(() => {
    request++;
  });
  return { current, loading, error, refresh };
}
