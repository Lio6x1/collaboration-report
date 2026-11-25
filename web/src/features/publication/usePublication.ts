import { computed, onScopeDispose, reactive, ref, watch } from 'vue';
import type { Document, PublicDocument, Release } from '../../../../contracts/http/generated';
import { actionsBlocked, explain, transportEpoch, useAction } from '../../lib/http';
import { acceptPage, createCursorPage, pageCursor, resetPage } from '../../lib/pagination';
import {
  publicationInput,
  publicationPath,
  readFixedRelease,
  readReleases,
  releaseInput,
  releasesPath,
} from './api';

export type PublicationChoice = {
  kind: 'CREATE' | 'SWITCH' | 'UNPUBLISH';
  document: Document;
  release?: Release;
};
const releaseTime = new Intl.DateTimeFormat('zh-CN', {
  dateStyle: 'long',
  timeStyle: 'medium',
  timeZone: 'Asia/Shanghai',
  hourCycle: 'h23',
});
export function releaseLabel(createdAt?: string) {
  // 用不可变的发布时间识别版本，不把数据库ID或分页位置伪装成连续的v1/v2。
  const date = createdAt ? new Date(createdAt) : null;
  return date && Number.isFinite(date.getTime())
    ? releaseTime.format(date) + ' 发布'
    : '发布时间待确认';
}
export function usePublication(
  current: () => Document,
  reload: () => Promise<void>,
  prepare: () => Promise<unknown>,
) {
  const action = useAction(),
    releases = ref<Release[]>([]),
    pagination = reactive(createCursorPage()),
    nextCursor = computed(() => pagination.next),
    fixed = ref<PublicDocument | null>(null),
    fixedRelease = ref<Release | null>(null),
    choice = ref<PublicationChoice | null>(null);
  const historyLoading = ref(false),
    previewLoading = ref(false),
    readingError = ref(''),
    previewError = ref('');
  const blocked = computed(
    () => actionsBlocked.value || historyLoading.value || previewLoading.value,
  );
  let generation = 0,
    historyRequest = 0,
    previewRequest = 0,
    active = true;
  const valid = (started: number) => active && started === generation;
  async function loadHistory(index = pagination.index) {
    const started = generation,
      sequence = ++historyRequest,
      document = current().id,
      cursor = pageCursor(pagination, index);
    historyLoading.value = true;
    releases.value = [];
    readingError.value = '';
    try {
      const page = await readReleases(document, cursor);
      if (!valid(started) || sequence !== historyRequest) return;
      releases.value = page.items;
      acceptPage(pagination, index, cursor, page.next_cursor);
    } catch (error) {
      if (valid(started) && sequence === historyRequest) {
        readingError.value = explain(error);
        // 已确认写入后的刷新失败仍交给原操作恢复器，不能丢失后续读取上下文。
        throw error;
      }
    } finally {
      if (valid(started) && sequence === historyRequest) historyLoading.value = false;
    }
  }
  function choose(kind: PublicationChoice['kind'], release?: Release) {
    // 对话框固定首次看到的期待值；之后服务器变化会明确冲突，不静默改成后来修订再写。
    choice.value = {
      kind,
      document: { ...current() },
      ...(release ? { release: { ...release } } : {}),
    };
  }
  async function confirm() {
    const selected = choice.value;
    if (!selected) return;
    const started = generation;
    let prepared = false;
    await action.run(
      async () => {
        if (selected.kind === 'CREATE' && !prepared) {
          // MAIN本机IME/待发队列先沿原可靠FLUSH保存；未知POST或进度重试不再次捕获本机边界。
          await prepare();
          prepared = true;
        }
        await action.mutate(
          selected.kind === 'CREATE'
            ? releasesPath(selected.document.id)
            : publicationPath(selected.document.id),
          selected.kind === 'CREATE'
            ? releaseInput(selected.document)
            : publicationInput(selected.document, selected.release),
        );
        if (!valid(started)) return;
        await reload();
        if (!valid(started)) return;
        await loadHistory();
        choice.value = null;
      },
      selected.kind === 'CREATE'
        ? '正式版已发布；查询结果可能需要稍后刷新'
        : selected.kind === 'SWITCH'
          ? '当前正式版已切换'
          : '当前正式版已撤下',
    );
    // 原号由应用级恢复器保留。未知时先退出遮罩，让唯一原操作恢复入口可以实际点击。
    if (action.pending.value) choice.value = null;
  }
  async function loadPage(index = pagination.index) {
    try {
      await loadHistory(index);
    } catch {
      // 只读失败在历史区单独展示；不能覆盖未确认写入的原号与恢复入口。
    }
  }
  function refresh() {
    // 手动刷新清除过期入口链；历史预览、确认修订和未知写入闭包不随列表重建。
    resetPage(pagination);
    return loadPage(0);
  }
  function closePreview() {
    previewRequest++;
    fixed.value = null;
    fixedRelease.value = null;
    previewLoading.value = false;
    previewError.value = '';
  }
  async function preview(release: Release) {
    const started = generation,
      sequence = ++previewRequest,
      document = current().id,
      selected = { ...release };
    fixed.value = null;
    fixedRelease.value = selected;
    previewLoading.value = true;
    previewError.value = '';
    try {
      const value = await readFixedRelease(document, selected.id);
      if (!valid(started) || sequence !== previewRequest) return;
      fixed.value = value;
    } catch (error) {
      if (valid(started) && sequence === previewRequest) previewError.value = explain(error);
    } finally {
      if (valid(started) && sequence === previewRequest) previewLoading.value = false;
    }
  }
  watch(
    [() => current().id, transportEpoch],
    () => {
      generation++;
      resetPage(pagination);
      releases.value = [];
      choice.value = null;
      historyLoading.value = false;
      closePreview();
      void refresh();
    },
    { immediate: true, flush: 'sync' },
  );
  onScopeDispose(() => {
    active = false;
    generation++;
  });
  return {
    action,
    releases,
    pagination,
    nextCursor,
    fixed,
    fixedRelease,
    choice,
    blocked,
    historyLoading,
    previewLoading,
    readingError,
    previewError,
    choose,
    confirm,
    refresh,
    loadPage,
    preview,
    closePreview,
  };
}
