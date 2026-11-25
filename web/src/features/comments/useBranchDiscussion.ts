import { UserFacingError, api, explain, transportEpoch } from '../../lib/http';
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import type { FixedContent, ReviewRound } from '../../../../contracts/http/generated';
import { readRequests, readRounds } from '../review/api';

type DiscussionTarget = {
  document: string;
  unit: string;
  boundary: { branch: string; version: string; manifest: string; lineage: string };
};

export function useBranchDiscussion(props: DiscussionTarget, enabled: () => boolean) {
  const baseline = ref<FixedContent | null>(null);
  const rounds = ref<ReviewRound[]>([]);
  const selected = ref('baseline');
  const loading = ref(false);
  const error = ref('');
  let generation = 0;
  let active = true;
  const round = computed(() => rounds.value.find((value) => value.id === selected.value));
  const content = computed(() =>
    selected.value === 'baseline' ? baseline.value : round.value?.content,
  );

  async function load() {
    const current = ++generation;
    const target = { document: props.document, unit: props.unit, ...props.boundary };
    const valid = () => active && current === generation;
    baseline.value = null;
    rounds.value = [];
    selected.value = 'baseline';
    error.value = '';
    loading.value = true;
    try {
      const fixed = await api<FixedContent>(
        '/v1/content-versions/' + encodeURIComponent(target.version),
      );
      if (!valid()) return;
      if (
        fixed.id !== target.version ||
        fixed.document_id !== target.document ||
        fixed.unit_id !== target.unit
      )
        throw new UserFacingError('支线基线正文归属不一致，请重新打开编辑器');
      baseline.value = fixed;
      // 文档列表与每个请求的轮次都必须走完真实游标；第一页无匹配项不代表没有历史。
      const candidates: ReviewRound[] = [];
      let cursor: string | null = null;
      const requestCursors = new Set<string>();
      do {
        const requests = await readRequests(target.document, cursor);
        if (!valid()) return;
        for (const request of requests.items) {
          if (
            request.document_id !== target.document ||
            request.unit_id !== target.unit ||
            request.branch_id !== target.branch
          )
            continue;
          let roundCursor: string | null = null;
          const roundCursors = new Set<string>();
          do {
            const page = await readRounds(request.id, roundCursor);
            if (!valid()) return;
            for (const entry of page.items) {
              if (
                entry.request_id !== request.id ||
                entry.content.id !== entry.content_version_id ||
                entry.content.document_id !== target.document ||
                entry.content.unit_id !== target.unit
              )
                throw new UserFacingError('送审轮次正文归属不一致，请刷新讨论版本');
              if (!candidates.some((old) => old.id === entry.id)) candidates.push(entry);
            }
            roundCursor = page.next_cursor;
            if ((page.has_more && !roundCursor) || (roundCursor && roundCursors.has(roundCursor)))
              throw new UserFacingError('送审轮次分页尚未完成，请刷新讨论版本');
            if (roundCursor) roundCursors.add(roundCursor);
          } while (roundCursor);
        }
        cursor = requests.next_cursor;
        if ((requests.has_more && !cursor) || (cursor && requestCursors.has(cursor)))
          throw new UserFacingError('审核请求分页尚未完成，请刷新讨论版本');
        if (cursor) requestCursors.add(cursor);
      } while (cursor);
      if (valid()) rounds.value = candidates;
    } catch (reason) {
      if (valid()) error.value = explain(reason);
    } finally {
      if (valid()) loading.value = false;
    }
  }
  watch(
    // SAVED等状态更新会生成新的boundary对象；只有实际固定边界变化才丢弃讨论输入。
    [
      enabled,
      () => props.document,
      () => props.unit,
      () => props.boundary.branch,
      () => props.boundary.version,
      () => props.boundary.manifest,
      () => props.boundary.lineage,
      () => transportEpoch.value,
    ],
    () => {
      if (enabled()) void load();
      else {
        // 收起入口即放弃在途读取，长分页结果不能重新填回已关闭的讨论。
        generation++;
        baseline.value = null;
        rounds.value = [];
        loading.value = false;
      }
    },
    { immediate: true },
  );
  onBeforeUnmount(() => {
    active = false;
    generation++;
  });
  return { content, round, rounds, selected, loading, error, load };
}
