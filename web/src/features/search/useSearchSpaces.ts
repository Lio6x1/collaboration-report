import { computed, onUnmounted, ref, watch } from 'vue';
import type { Organization, Space, SpacePage } from '../../../../contracts/http/generated';
import { api, explain, transportEpoch } from '../../lib/http';
import { identityUnavailable, user } from '../account/session';

export function useSearchSpaces() {
  const spaces = ref<Space[]>([]);
  const names = ref<Record<string, string>>({});
  const cursor = ref<string | null>(null);
  const loading = ref(false);
  const error = ref('');
  const identity = computed(() => `${transportEpoch.value}:${user.value?.id ?? 'anonymous'}`);
  let request = 0;
  let disposed = false;

  async function load(append = false) {
    if (!user.value || identityUnavailable.value || (append && (!cursor.value || loading.value)))
      return;
    const original = ++request;
    const originalIdentity = identity.value;
    const oldCursor = append ? cursor.value : null;
    const active = () => !disposed && original === request && originalIdentity === identity.value;
    loading.value = true;
    error.value = '';
    if (!append) {
      spaces.value = [];
      cursor.value = null;
      names.value = {};
    }
    try {
      const page = await api<SpacePage>(
        '/v1/me/spaces?limit=20' + (oldCursor ? '&cursor=' + encodeURIComponent(oldCursor) : ''),
      );
      if (!active()) return;
      const merged = append ? [...spaces.value, ...page.items] : page.items;
      spaces.value = [...new Map(merged.map((space) => [space.id, space])).values()];
      cursor.value = page.next_cursor;
      // 组织名称仅供选范围，资格由搜索HTTP重新核验；最多四个名称请求并行，目录不截断。
      const ids = [
        ...new Set(
          page.items
            .map((space) => space.organization_id)
            .filter((id): id is string => !!id && !names.value[id]),
        ),
      ];
      for (let offset = 0; offset < ids.length && active(); offset += 4) {
        await Promise.all(
          ids.slice(offset, offset + 4).map(async (id) => {
            try {
              const organization = await api<Organization>('/v1/organizations/' + id);
              if (active()) names.value[id] = organization.name;
            } catch {
              // 名称未知不能变成授权，也不能删除已确认的空间范围；仍用可区分的空间编号。
            }
          }),
        );
      }
    } catch (failure) {
      if (!active()) return;
      error.value = explain(failure);
    } finally {
      if (active()) loading.value = false;
    }
  }
  watch(
    [identity, identityUnavailable],
    () => {
      request++;
      spaces.value = [];
      names.value = {};
      cursor.value = null;
      loading.value = false;
      error.value = '';
      if (user.value && !identityUnavailable.value) void load();
    },
    { immediate: true, flush: 'sync' },
  );
  onUnmounted(() => {
    disposed = true;
    request++;
  });
  const spaceName = (space: Space) =>
    space.kind === 'PERSONAL'
      ? '个人空间'
      : (names.value[space.organization_id ?? ''] ?? '组织空间 ' + space.id);
  return { spaces, cursor, loading, error, load, spaceName };
}
