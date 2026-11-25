import { UserFacingError, explain, transportEpoch, useAction } from '../../lib/http';
import { ref, computed, reactive, watch, onScopeDispose } from 'vue';
import type {
  ReviewRequest,
  ReviewRound,
  ReviewRoundPage,
  ChangeRequest,
  FixedContent,
} from '../../../../contracts/http/generated';
import { acceptPage, createCursorPage, pageCursor, resetPage } from '../../lib/pagination';
import {
  readRequest,
  readRequests,
  readRounds,
  readChanges,
  readReviewBase,
  findDefaultRequest,
  requestPath,
} from './api';
import { validateReviewBase } from './reviewDiff';

export function useReviews(
  document: string | (() => string),
  actor: () => string,
  managing: () => boolean,
  initialRequest: () => string = () => '',
) {
  const documentID = () => (typeof document === 'string' ? document : document());
  const requests = ref<ReviewRequest[]>([]),
    request = ref<ReviewRequest | null>(null),
    rounds = ref<ReviewRound[]>([]),
    changes = ref<ChangeRequest[]>([]),
    fixed = ref<ReviewRound | null>(null);
  const requestPagination = reactive(createCursorPage()),
    roundPagination = reactive(createCursorPage()),
    changePagination = reactive(createCursorPage());
  const selectedRound = ref(''),
    reason = ref(''),
    resolution = ref<Record<string, string>>({}),
    listLoading = ref(false),
    detailLoading = ref(false),
    roundsLoading = ref(false),
    changesLoading = ref(false),
    locatingDefault = ref(false),
    readingError = ref('');
  const loading = computed(
    () =>
      listLoading.value ||
      detailLoading.value ||
      roundsLoading.value ||
      changesLoading.value ||
      locatingDefault.value,
  );
  const action = useAction();
  let generation = 0,
    listRequest = 0,
    detailRequest = 0,
    roundRequest = 0,
    changeRequest = 0,
    selectionRequest = 0,
    active = true,
    lastRequestedFocus = '';
  const valid = (started: number) => active && started === generation;
  const comparisonBase = ref<FixedContent | null>(null),
    comparisonLoading = ref(false),
    comparisonError = ref('');
  let comparisonGeneration = 0;

  // 固定阅读与展示页分离；翻到另一页不能卸载本轮正文、讨论草稿或改变当前审核对象。
  const displayedRounds = computed(() => rounds.value);
  watch(
    selectedRound,
    (id) => {
      if (fixed.value?.id === id) return;
      const value =
        rounds.value.find((round) => round.id === id) ??
        (request.value?.current_round?.id === id ? request.value.current_round : null);
      if (value && value.request_id === request.value?.id) fixed.value = value;
    },
    { flush: 'sync' },
  );
  async function loadComparison() {
    const round = fixed.value,
      document = documentID(),
      context = generation,
      started = ++comparisonGeneration;
    comparisonBase.value = null;
    comparisonError.value = '';
    comparisonLoading.value = !!round;
    if (!round) return;
    try {
      if (
        round.request_id !== request.value?.id ||
        round.content.unit_id !== request.value?.unit_id
      )
        throw new UserFacingError('审核轮次归属已变化，请刷新审核记录');
      const base = await readReviewBase(round.base_version_id);
      if (!valid(context) || started !== comparisonGeneration) return;
      comparisonBase.value = validateReviewBase(round, base, document);
    } catch (error) {
      if (valid(context) && started === comparisonGeneration)
        comparisonError.value = explain(error, '审核基线暂时无法读取，请重试或切换到完整送审稿。');
    } finally {
      if (valid(context) && started === comparisonGeneration) comparisonLoading.value = false;
    }
  }
  watch(fixed, () => void loadComparison(), { flush: 'sync' });
  const fixedEntries = computed(() =>
    fixed.value
      ? fixed.value.structure.entries.filter(
          (entry) => entry.unit_id === fixed.value!.content.unit_id,
        )
      : [],
  );

  // null表示完整历史尚未核验；资格事实不能随展示页从有变无。
  const participated = ref<boolean | null>(null);
  const canResolve = (change: ChangeRequest) =>
    !!request.value &&
    (managing() ||
      (request.value.actions.includes('REQUEST_CHANGES') &&
        change.author_id === actor() &&
        participated.value === false));
  async function participation(
    id: string,
    first: ReviewRoundPage,
    started: number,
    sequence: number,
  ) {
    let page = first;
    const visited = new Set<string>();
    do {
      if (!valid(started) || sequence !== detailRequest) return null;
      if (page.items.some((round) => round.submitter_id === actor())) return true;
      const cursor = page.next_cursor;
      if ((page.has_more && !cursor) || (cursor && visited.has(cursor)))
        throw new UserFacingError('审核历史分页尚未完成，请刷新后重新核验资格');
      if (!cursor) return false;
      visited.add(cursor);
      page = await readRounds(id, cursor);
    } while (true);
  }
  function clearBody() {
    request.value = null;
    rounds.value = [];
    changes.value = [];
    fixed.value = null;
    selectedRound.value = '';
    participated.value = null;
    resetPage(roundPagination);
    resetPage(changePagination);
  }

  async function open(id: string, exact?: ReviewRequest) {
    const started = generation,
      sequence = ++detailRequest,
      previous = request.value?.id === id ? fixed.value : null;
    roundRequest++;
    changeRequest++;
    if (request.value?.id !== id) {
      clearBody();
      reason.value = '';
      resolution.value = {};
    }
    participated.value = null;
    readingError.value = '';
    detailLoading.value = true;
    roundsLoading.value = false;
    changesLoading.value = false;
    try {
      const [current, history, blocking] = await Promise.all([
        exact ?? readRequest(id),
        readRounds(id),
        readChanges(id),
      ]);
      if (!valid(started) || sequence !== detailRequest) return;
      if (current.document_id !== documentID()) throw new UserFacingError('审核请求不属于当前文档');
      const submitted =
        current.current_round?.submitter_id === actor()
          ? true
          : await participation(id, history, started, sequence);
      if (!valid(started) || sequence !== detailRequest) return;
      request.value = current;
      rounds.value = history.items;
      changes.value = blocking.items;
      acceptPage(roundPagination, 0, null, history.next_cursor);
      acceptPage(changePagination, 0, null, blocking.next_cursor);
      participated.value = submitted;
      const chosen =
        previous ??
        current.current_round ??
        history.items.find((round) => round.id === current.current_round_id);
      fixed.value = chosen ?? null;
      selectedRound.value = chosen?.id ?? '';
    } catch (error) {
      if (valid(started) && sequence === detailRequest) {
        clearBody();
        readingError.value = explain(error);
      }
    } finally {
      if (valid(started) && sequence === detailRequest) detailLoading.value = false;
    }
  }

  async function loadRequests(index = requestPagination.index) {
    const started = generation,
      sequence = ++listRequest,
      cursor = pageCursor(requestPagination, index);
    listLoading.value = true;
    readingError.value = '';
    requests.value = [];
    try {
      const page = await readRequests(documentID(), cursor);
      if (!valid(started) || sequence !== listRequest) return null;
      requests.value = page.items;
      acceptPage(requestPagination, index, cursor, page.next_cursor);
      return page;
    } catch (error) {
      if (valid(started) && sequence === listRequest) readingError.value = explain(error);
      return null;
    } finally {
      if (valid(started) && sequence === listRequest) listLoading.value = false;
    }
  }
  async function load() {
    const focus = initialRequest(),
      explicitFocus = !!focus && focus !== lastRequestedFocus,
      previous = explicitFocus ? focus : (request.value?.id ?? focus),
      started = generation,
      detail = detailRequest,
      selection = ++selectionRequest;
    lastRequestedFocus = focus;
    const page = await loadRequests();
    if (!page || !valid(started) || detail !== detailRequest || selection !== selectionRequest)
      return;
    try {
      let selected = page.items.find((value) => value.id === previous && !value.archived_at),
        exact: ReviewRequest | undefined;
      // 精确选中结果可以落在别页；只读取它的详情，不能把它塞入20条展示页。
      if (!selected && previous) {
        exact = await readRequest(previous);
        if (!valid(started) || detail !== detailRequest || selection !== selectionRequest) return;
        if (exact.document_id !== documentID()) throw new UserFacingError('审核请求不属于当前文档');
        if (!exact.archived_at) selected = exact;
      }
      // 丢弃后归档的旧请求不再作为当前工作；已确认的新审核仍按原有入口选择。
      if (!selected) {
        locatingDefault.value = true;
        selected =
          (await findDefaultRequest(
            documentID(),
            () => valid(started) && detail === detailRequest && selection === selectionRequest,
          )) ?? undefined;
      }
      if (!valid(started) || detail !== detailRequest || selection !== selectionRequest) return;
      if (selected) await open(selected.id, exact?.id === selected.id ? exact : undefined);
    } catch (error) {
      if (valid(started) && detail === detailRequest && selection === selectionRequest) {
        clearBody();
        readingError.value = explain(error);
      }
    } finally {
      if (valid(started) && selection === selectionRequest) locatingDefault.value = false;
    }
  }
  function refresh() {
    // 显式刷新只重建目录游标；选中请求、固定轮次和本机草稿继续按原对象核对。
    resetPage(requestPagination);
    return load();
  }
  async function loadRounds(index = roundPagination.index) {
    const id = request.value?.id;
    if (!id) return;
    const started = generation,
      detail = detailRequest,
      sequence = ++roundRequest,
      cursor = pageCursor(roundPagination, index);
    roundsLoading.value = true;
    readingError.value = '';
    rounds.value = [];
    try {
      const page = await readRounds(id, cursor);
      if (!valid(started) || detail !== detailRequest || sequence !== roundRequest) return;
      rounds.value = page.items;
      acceptPage(roundPagination, index, cursor, page.next_cursor);
    } catch (error) {
      if (valid(started) && detail === detailRequest && sequence === roundRequest)
        readingError.value = explain(error);
    } finally {
      if (valid(started) && detail === detailRequest && sequence === roundRequest)
        roundsLoading.value = false;
    }
  }
  async function loadChanges(index = changePagination.index) {
    const id = request.value?.id;
    if (!id) return;
    const started = generation,
      detail = detailRequest,
      sequence = ++changeRequest,
      cursor = pageCursor(changePagination, index);
    changesLoading.value = true;
    readingError.value = '';
    changes.value = [];
    try {
      const page = await readChanges(id, cursor);
      if (!valid(started) || detail !== detailRequest || sequence !== changeRequest) return;
      changes.value = page.items;
      acceptPage(changePagination, index, cursor, page.next_cursor);
    } catch (error) {
      if (valid(started) && detail === detailRequest && sequence === changeRequest)
        readingError.value = explain(error);
    } finally {
      if (valid(started) && detail === detailRequest && sequence === changeRequest)
        changesLoading.value = false;
    }
  }
  function refreshRounds() {
    resetPage(roundPagination);
    return loadRounds(0);
  }
  function refreshChanges() {
    resetPage(changePagination);
    return loadChanges(0);
  }

  async function requestChanges() {
    const current = request.value,
      started = generation,
      inputReason = reason.value;
    if (!current?.current_round) return;
    await action.run(async () => {
      await action.mutate(requestPath(current.id) + '/actions', {
        round_id: current.current_round_id,
        content_version_id: current.current_round!.content_version_id,
        type: 'REQUEST_CHANGES',
        reason: inputReason,
      });
      if (!valid(started) || request.value?.id !== current.id) return;
      if (reason.value === inputReason) reason.value = '';
      await load();
    }, '要求修改已记录，重新送审也会保留');
  }
  async function withdraw() {
    const current = request.value,
      started = generation;
    if (!current) return;
    await action.run(async () => {
      await action.mutate(requestPath(current.id) + '/withdrawals', {
        expected_round_id: current.current_round_id,
      });
      if (valid(started) && request.value?.id === current.id) await load();
    }, '已撤回，可编辑后重新送审');
  }
  async function resolve(change: ChangeRequest) {
    const current = request.value,
      started = generation,
      inputReason = resolution.value[change.id] ?? '';
    if (!current) return;
    await action.run(async () => {
      await action.mutate(requestPath(current.id) + '/change-requests/' + change.id + '/resolve', {
        expected_round_id: current.current_round_id,
        expected_change_revision: change.revision,
        reason: inputReason,
      });
      if (!valid(started) || request.value?.id !== current.id) return;
      if (resolution.value[change.id] === inputReason) delete resolution.value[change.id];
      await load();
    }, '修改要求已解除');
  }
  watch(
    [documentID, actor, managing, transportEpoch],
    () => {
      generation++;
      detailRequest++;
      comparisonGeneration++;
      requests.value = [];
      resetPage(requestPagination);
      clearBody();
      reason.value = '';
      resolution.value = {};
      lastRequestedFocus = '';
      listLoading.value = false;
      detailLoading.value = false;
      roundsLoading.value = false;
      changesLoading.value = false;
      comparisonBase.value = null;
      comparisonError.value = '';
      comparisonLoading.value = false;
      locatingDefault.value = false;
      void load();
    },
    { flush: 'sync' },
  );
  onScopeDispose(() => {
    active = false;
    generation++;
    comparisonGeneration++;
  });
  return {
    requests,
    request,
    displayedRounds,
    fixed,
    fixedEntries,
    comparisonBase,
    comparisonLoading,
    comparisonError,
    loadComparison,
    changes,
    selectedRound,
    reason,
    resolution,
    loading,
    readingError,
    action,
    requestPagination,
    roundPagination,
    changePagination,
    canResolve,
    load,
    refresh,
    open,
    loadRequests,
    loadRounds,
    loadChanges,
    refreshRounds,
    refreshChanges,
    requestChanges,
    withdraw,
    resolve,
  };
}
