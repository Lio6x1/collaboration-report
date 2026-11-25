import {
  UserFacingError,
  actionsBlocked,
  api,
  explain,
  transportEpoch,
  useAction,
} from '../../lib/http';
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue';
import { acceptPage, createCursorPage, pageCursor, resetPage } from '../../lib/pagination';
import type {
  KnowledgeBase,
  KnowledgeBasePage,
  Operation,
  PersonalTransferMetadata,
  PersonalTransferPage,
  Quota,
  SpacePage,
} from '../../../../contracts/http/generated';

export function usePersonalTransfers() {
  const action = useAction();
  const direction = ref<'incoming' | 'outgoing'>('incoming');
  const offers = ref<PersonalTransferMetadata[]>([]);
  const pagination = reactive(createCursorPage());
  const cursor = computed(() => pagination.next);
  const knowledgeBases = ref<KnowledgeBase[]>([]);
  const knowledgeBaseCursor = ref<string | null>(null);
  const knowledgeBase = ref('');
  const quota = ref<Quota | null>(null);
  const loading = ref(false);
  const targetsLoading = ref(false);
  const readingError = ref('');
  const targetError = ref('');
  const progress = ref<Operation | null>(null);
  const blocked = computed(() => actionsBlocked.value || loading.value || targetsLoading.value);
  let generation = 0,
    listRequest = 0,
    targetRequest = 0;
  let space = '';
  let active = true;

  async function load(index = pagination.index) {
    const current = generation,
      sequence = ++listRequest;
    const next = pageCursor(pagination, index);
    loading.value = true;
    readingError.value = '';
    offers.value = [];
    try {
      const page = await api<PersonalTransferPage>(
        `/v1/personal-transfers?direction=${direction.value}&limit=20` +
          (next ? '&cursor=' + encodeURIComponent(next) : ''),
      );
      if (!active || current !== generation || sequence !== listRequest) return;
      offers.value = page.items;
      acceptPage(pagination, index, next, page.next_cursor);
    } catch (error) {
      if (active && current === generation && sequence === listRequest)
        readingError.value = explain(error);
    } finally {
      if (active && current === generation && sequence === listRequest) loading.value = false;
    }
  }
  function refresh() {
    // 用户主动刷新从无游标的首页重新读取，避免停留过久后只能反复请求失效游标。
    resetPage(pagination);
    return load(0);
  }

  async function loadTargets(append = false) {
    const current = generation,
      sequence = ++targetRequest;
    targetsLoading.value = true;
    targetError.value = '';
    try {
      // 个人空间在当前目录首位；使用现有limit=1真实物化入口，不猜空间编号或其他人的库。
      if (!space) {
        const page = await api<SpacePage>('/v1/me/spaces?limit=1');
        const personal = page.items.find((s) => s.kind === 'PERSONAL' && s.lifecycle === 'ACTIVE');
        if (!personal) throw new UserFacingError('当前没有可接收文档的个人空间');
        if (!active || current !== generation || sequence !== targetRequest) return;
        space = personal.id;
      }
      const next = append ? knowledgeBaseCursor.value : null;
      const [page, currentQuota] = await Promise.all([
        api<KnowledgeBasePage>(
          `/v1/spaces/${space}/knowledge-bases?limit=20` +
            (next ? '&cursor=' + encodeURIComponent(next) : ''),
        ),
        api<Quota>(`/v1/spaces/${space}/quota`),
      ]);
      if (!active || current !== generation || sequence !== targetRequest) return;
      const allowed = page.items.filter((kb) => kb.lifecycle === 'ACTIVE');
      knowledgeBases.value = append
        ? [
            ...knowledgeBases.value,
            ...allowed.filter((v) => !knowledgeBases.value.some((old) => old.id === v.id)),
          ]
        : allowed;
      knowledgeBaseCursor.value = page.next_cursor;
      quota.value = currentQuota;
      if (!knowledgeBases.value.some((kb) => kb.id === knowledgeBase.value))
        knowledgeBase.value = knowledgeBases.value[0]?.id ?? '';
    } catch (error) {
      if (active && current === generation && sequence === targetRequest)
        targetError.value = explain(error);
    } finally {
      if (active && current === generation && sequence === targetRequest)
        targetsLoading.value = false;
    }
  }

  async function transition(offer: PersonalTransferMetadata, kind: 'accept' | 'reject' | 'cancel') {
    const fixed = { ...offer },
      current = generation;
    const input =
      kind === 'accept'
        ? {
            expected_revision: fixed.revision,
            target_knowledge_base_id: knowledgeBase.value,
            expected_quota_revision: quota.value?.revision ?? '',
          }
        : { expected_revision: fixed.revision };
    if (kind === 'accept' && (!knowledgeBase.value || !quota.value)) return;
    await action.run(
      async () => {
        // 原号闭包固定offer、目的库与配额修订；未知响应不采用刷新后的新输入。
        await action.mutate(`/v1/personal-transfers/${fixed.id}/${kind}`, input);
        if (!active || current !== generation) return;
        await load();
        if (kind === 'accept') await loadTargets();
      },
      kind === 'accept'
        ? '文档已转入你的个人知识库，旧访问已关闭'
        : kind === 'cancel'
          ? '转让申请已取消，归属未改变'
          : '转让申请已拒绝，归属未改变',
    );
  }

  async function inspect(offer: PersonalTransferMetadata) {
    if (!offer.accept_operation_id) return;
    const current = generation;
    loading.value = true;
    readingError.value = '';
    try {
      const value = await api<Operation>(`/v1/operations/${offer.accept_operation_id}`);
      if (!active || current !== generation) return;
      progress.value = value;
      if (['SUCCEEDED', 'REJECTED', 'CANCELLED'].includes(value.state)) await load();
    } catch (error) {
      if (active && current === generation) readingError.value = explain(error);
    } finally {
      if (active && current === generation) loading.value = false;
    }
  }

  watch(
    [direction, transportEpoch],
    () => {
      generation++;
      offers.value = [];
      resetPage(pagination);
      progress.value = null;
      quota.value = null;
      space = '';
      knowledgeBases.value = [];
      knowledgeBaseCursor.value = null;
      loading.value = false;
      targetsLoading.value = false;
      void load();
      if (direction.value === 'incoming') void loadTargets();
    },
    { immediate: true, flush: 'sync' },
  );
  onBeforeUnmount(() => {
    active = false;
    generation++;
  });
  return {
    action,
    direction,
    offers,
    pagination,
    cursor,
    knowledgeBases,
    knowledgeBaseCursor,
    knowledgeBase,
    quota,
    loading,
    targetsLoading,
    readingError,
    targetError,
    progress,
    blocked,
    load,
    refresh,
    loadTargets,
    transition,
    inspect,
  };
}
