import { UserFacingError, OperationRejected, useAction } from '../../lib/http';
import { computed, ref } from 'vue';
import type {
  ApproveMerge,
  ChangeRequest,
  ConflictContext,
  Operation,
  ReviewRequest,
} from '../../../../contracts/http/generated';
import { approvePath, ownMergeOpinions, readConflict } from './api';

type Confirmation = { request: string; round: string; version: string; changes: ChangeRequest[] };

export function useApproveMerge(
  current: () => ReviewRequest,
  actor: () => string,
  prepare: () => Promise<void>,
  reload: () => Promise<void>,
) {
  const action = useAction(),
    fixed = ref<Confirmation | null>(null);
  const acknowledged = ref(false),
    included = ref<Record<string, boolean>>({});
  const rejected = ref<Operation | null>(null),
    conflict = ref<ConflictContext | null>(null);
  const canConfirm = computed(
    () =>
      !!fixed.value &&
      !rejected.value &&
      acknowledged.value &&
      fixed.value.changes.every((change) => included.value[change.id]),
  );

  async function inspect() {
    acknowledged.value = false;
    included.value = {};
    await action.run(async () => {
      const expected = current();
      if (!expected.current_round || !expected.actions.includes('APPROVE_AND_MERGE'))
        throw new UserFacingError('当前固定轮次尚无通过并合并资格，请刷新');
      const changes = await ownMergeOpinions(expected, actor());
      fixed.value = {
        request: expected.id,
        round: expected.current_round_id,
        version: expected.current_round.content_version_id,
        changes,
      };
    }, '请核对整个固定送审版本及所有拟解除本人要求');
  }

  async function confirm() {
    if (!fixed.value || !canConfirm.value) return;
    const original = fixed.value;
    const input: Omit<ApproveMerge, 'operation_id'> = {
      round_id: original.round,
      content_version_id: original.version,
      own_changes_to_resolve: original.changes.map((change) => ({
        id: change.id,
        revision: change.revision,
      })),
    };
    let prepared = false;
    await action.run(async () => {
      // 保存本机后续输入保护刷新生命周期；它不成为本次批准的正文，未知续接也不重新FLUSH。
      if (!prepared) {
        await prepare();
        prepared = true;
      }
      try {
        await action.mutate(approvePath(original.request), input);
      } catch (error) {
        if (error instanceof OperationRejected && error.operation.result?.conflict_context_ref)
          rejected.value = error.operation;
        throw error;
      }
      // settle只在安装READY结束。刷新前另一次保存保护计算期间的新输入，随后由父级关闭旧视图。
      await reload();
    }, '固定版本已通过并合并，后续草稿继续保留');
  }

  async function inspectConflict() {
    const original = rejected.value,
      ref = original?.result?.conflict_context_ref;
    if (!original || !ref) return;
    await action.run(async () => {
      const value = await readConflict(ref);
      if (
        value.operation_id !== original.operation_id ||
        value.conflict_ref !== ref ||
        value.request_id !== fixed.value?.request ||
        value.round_id !== fixed.value?.round ||
        (original.result?.attempt_id && value.attempt_id !== original.result.attempt_id)
      )
        throw new UserFacingError('本次固定冲突归属尚未确认，请保留原操作并重新读取');
      conflict.value = value;
    }, '显示的是本次原计算的固定输入');
  }

  function close() {
    fixed.value = null;
    acknowledged.value = false;
    included.value = {};
    rejected.value = null;
    conflict.value = null;
  }
  return {
    action,
    fixed,
    acknowledged,
    included,
    rejected,
    conflict,
    canConfirm,
    inspect,
    confirm,
    inspectConflict,
    close,
  };
}
