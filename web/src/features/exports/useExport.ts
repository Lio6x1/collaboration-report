import {
  UserFacingError,
  api,
  ApiError,
  actionsBlocked,
  explain,
  transportEpoch,
  useAction,
} from '../../lib/http';
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import type { ExportJob, FixedContent, MainView } from '../../../../contracts/http/generated';
import { user } from '../account/session';

type Context = {
  document: string;
  target: 'MAIN' | 'PUBLIC';
  reference: string;
  mainView?: MainView;
  mainBodies?: Record<string, FixedContent>;
};
type Request = {
  operation_id: string;
  target: 'MAIN' | 'PUBLIC';
  format: string;
  expected_manifest_id?: string;
  expected_release_id?: string;
};

// 登记成功只表示QUEUED；转换、查询与下载分别由真实服务端检查，页面不推断成功。
export function useExport(context: () => Context) {
  const action = useAction();
  const job = ref<ExportJob | null>(null);
  const format = ref<'DOCX' | 'MARKDOWN' | 'PDF'>('DOCX');
  const readingError = ref('');
  const refreshing = ref(false);
  const original = ref<Request | null>(null);
  const unfinished = computed(() => !!job.value && ['QUEUED', 'RUNNING'].includes(job.value.state));
  let generation = 0;
  let active = true;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let polls = 0;
  const storageKey = () =>
    `collaboration-report.export:${user.value?.id ?? 'anonymous'}:${context().document}:${context().target}`;

  function save() {
    try {
      sessionStorage.setItem(
        storageKey(),
        JSON.stringify({ input: original.value, job: job.value?.id ?? null }),
      );
    } catch {
      /* 存储不可用不改变服务器结果。 */
    }
  }
  function clearSaved() {
    try {
      sessionStorage.removeItem(storageKey());
    } catch {
      /* 浏览器存储是便利恢复入口，不是授权。 */
    }
  }
  async function mainReference(scope: Context) {
    const fresh = await api<MainView>(`/v1/documents/${scope.document}/main`);
    const old = scope.mainView;
    const changed = () =>
      new UserFacingError('主线内容、标题或样式已变化，请先点击“刷新阅读”核对，再重新导出');
    if (!old) {
      if (fresh.manifest_id !== scope.reference) throw changed();
      return fresh.manifest_id;
    }
    if (
      fresh.document.id !== scope.document ||
      fresh.document.title !== old.document.title ||
      fresh.structure_id !== old.structure_id ||
      fresh.style_id !== old.style_id ||
      fresh.units.length !== old.units.length
    )
      throw changed();
    for (let index = 0; index < fresh.units.length; index++) {
      const unit = fresh.units[index],
        previous = old.units[index];
      if (unit.unit_id !== previous.unit_id) throw changed();
      if (unit.content_version_id === previous.content_version_id) continue;
      const originalBody = scope.mainBodies?.[unit.unit_id];
      if (!originalBody || originalBody.id !== previous.content_version_id) throw changed();
      // 未锁定MAIN每次捕获可能产生新的版本编号。比较正文摘要，不能把正常续期误判为编辑。
      const body = await api<FixedContent>(
        `/v1/documents/${scope.document}/read-views/${fresh.read_view_id}/units/${unit.unit_id}`,
      );
      if (body.id !== unit.content_version_id || body.body_hash !== originalBody.body_hash)
        throw changed();
    }
    return fresh.manifest_id;
  }
  async function refresh() {
    if (!job.value || refreshing.value) return;
    const current = generation,
      id = job.value.id;
    refreshing.value = true;
    readingError.value = '';
    clearTimeout(timer);
    try {
      const value = await api<ExportJob>(`/v1/export-jobs/${id}`);
      if (!active || generation !== current) return;
      job.value = value;
      save();
      // 有限自动观察结束后保留原任务供手动查询，不把超时转换成失败或另发POST。
      if (['QUEUED', 'RUNNING'].includes(value.state) && ++polls < 90)
        timer = setTimeout(refresh, 1000);
    } catch (failure) {
      if (!active || generation !== current) return;
      readingError.value = explain(failure);
      if (failure instanceof ApiError && [401, 403, 404].includes(failure.status)) {
        job.value = null;
        original.value = null;
        clearSaved();
      }
    } finally {
      if (active && generation === current) refreshing.value = false;
    }
  }
  async function create(recover = false) {
    if (actionsBlocked.value || action.busy.value || action.pending.value) return;
    const current = generation;
    const scope = { ...context() };
    let fixed = recover ? original.value : null;
    await action.run(async () => {
      if (!recover && !fixed) {
        // 只在新建前更新阅读准入；原号未知时必须恢复相同输入，不能偷偷换成另一个快照。
        const reference = scope.target === 'MAIN' ? await mainReference(scope) : scope.reference;
        if (!active || current !== generation) return;
        original.value = {
          operation_id: crypto.randomUUID(),
          target: scope.target,
          format: format.value,
          ...(scope.target === 'MAIN'
            ? { expected_manifest_id: reference }
            : { expected_release_id: reference }),
        };
        fixed = original.value;
        job.value = null;
        save();
      }
      if (!fixed) return;
      const value = await action.write<ExportJob>(`/v1/documents/${scope.document}/exports`, fixed);
      if (!active || current !== generation) return;
      job.value = value;
      save();
      polls = 0;
      void refresh();
    }, '导出任务已登记，请等待转换完成');
    if (current === generation && action.error.value && !action.pending.value) {
      original.value = null;
      clearSaved();
    }
  }

  watch(
    // 多来源分别比较值。同一账号的后台会话查询会替换user对象，
    // 不能因数组引用每次不同而重置已登记任务；实际换账号/会话仍立即清空。
    [() => context().document, () => context().target, () => user.value?.id, transportEpoch],
    () => {
      generation++;
      clearTimeout(timer);
      job.value = null;
      original.value = null;
      refreshing.value = false;
      readingError.value = '';
      polls = 0;
      try {
        const saved = JSON.parse(sessionStorage.getItem(storageKey()) ?? 'null');
        const reference =
          context().target === 'MAIN'
            ? saved?.input?.expected_manifest_id
            : saved?.input?.expected_release_id;
        if (
          saved?.input?.operation_id &&
          (context().target === 'MAIN' || reference === context().reference) &&
          saved.input.target === context().target
        ) {
          original.value = saved.input;
          if (saved.job && /^[1-9][0-9]*$/.test(saved.job)) {
            job.value = {
              id: saved.job,
              document_id: context().document,
              manifest_id: '',
              format: saved.input.format,
              state: 'QUEUED',
            };
            void refresh();
          }
        }
      } catch {
        clearSaved();
      }
    },
    { immediate: true },
  );
  onBeforeUnmount(() => {
    active = false;
    generation++;
    clearTimeout(timer);
  });
  return {
    action,
    job,
    format,
    readingError,
    refreshing,
    original,
    unfinished,
    actionsBlocked,
    create,
    refresh,
  };
}
