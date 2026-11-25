import { UserFacingError, useAction } from '../../lib/http';
import { computed, onUnmounted, ref } from 'vue';
import type {
  ConflictContext,
  ConflictResolution,
  ReviewRequest,
} from '../../../../contracts/http/generated';
import { readConflict, resolutionRequest, resolutionPath } from './api';

export function useConflictWorkbench(
  document: string,
  canEdit: (unit: string) => boolean,
  prepare: () => Promise<void>,
  reload: () => Promise<void>,
) {
  const action = useAction(),
    reference = ref(''),
    context = ref<ConflictContext | null>(null);
  const request = ref<ReviewRequest | null>(null),
    handled = ref<Record<string, boolean>>({});
  const acknowledged = ref(false),
    now = ref(Date.now());
  const timer = setInterval(() => {
    now.value = Date.now();
  }, 1000);
  onUnmounted(() => clearInterval(timer));
  const expired = computed(
    () =>
      !context.value ||
      !Number.isFinite(Date.parse(context.value.expires_at)) ||
      Date.parse(context.value.expires_at) <= now.value,
  );
  const editable = computed(() => !!context.value && canEdit(context.value.unit_id));
  const withdrawn = computed(
    () =>
      request.value?.status === 'WITHDRAWN' &&
      request.value.current_round_id === context.value?.round_id,
  );
  const canSave = computed(
    () =>
      editable.value &&
      withdrawn.value &&
      !expired.value &&
      acknowledged.value &&
      !!context.value?.conflicts.length &&
      context.value.conflicts.every((item) => handled.value[item.id]),
  );

  async function install(value: ConflictContext) {
    if (value.document_id !== document || value.conflict_ref !== value.operation_id)
      throw new UserFacingError('原冲突不属于当前文档，请核对编号');
    if (
      context.value?.conflict_ref === value.conflict_ref &&
      (context.value.attempt_id !== value.attempt_id ||
        context.value.input_digest !== value.input_digest)
    )
      throw new UserFacingError('原固定冲突绑定已变化，请保留候选并核对');
    if (context.value?.conflict_ref !== value.conflict_ref) {
      handled.value = {};
      acknowledged.value = false;
    }
    context.value = value;
    reference.value = value.conflict_ref;
    request.value = null;
    request.value = await resolutionRequest(value);
  }

  async function open() {
    const original = reference.value.trim();
    await action.run(async () => {
      if (!/^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$/.test(original))
        throw new UserFacingError('请填写完整原冲突操作编号');
      const value = await readConflict(original);
      if (value.conflict_ref !== original) throw new UserFacingError('原冲突编号尚未核验');
      await install(value);
    }, '固定输入已读取，请处理候选并核对撤回状态');
  }
  async function accept(value: ConflictContext) {
    // 已核验的原上下文可从批准面板带入；工作台自身仍核验文档归属和当前轮次。
    await action.run(() => install(value), '固定输入已在独立工作台打开');
  }
  async function refreshRequest() {
    const original = context.value;
    if (!original) return;
    request.value = null;
    await action.run(async () => {
      request.value = await resolutionRequest(original);
    }, '当前审核状态已核验');
  }

  async function save(readCandidate: () => Promise<object>) {
    const original = context.value;
    if (!original || !canSave.value) return;
    let input: Omit<ConflictResolution, 'operation_id'> | undefined,
      prepared = false;
    await action.run(async () => {
      // 首次完成IME后固定整个候选和原attempt/digest/round；未知续接绝不读取后来候选。
      input ??= {
        expected_attempt_id: original.attempt_id,
        expected_input_digest: original.input_digest,
        expected_round_id: original.round_id,
        resolution_content: await readCandidate(),
        resolved_conflict_ids: original.conflicts.map((item) => item.id),
      };
      if (!prepared) {
        await prepare();
        prepared = true;
      }
      await action.mutate(resolutionPath(original.conflict_ref), input);
      // 实际安装READY后旧BRANCH已经失效；父级按受控草稿生命周期关闭旧视图，不再FLUSH旧epoch。
      context.value = null;
      request.value = null;
      await reload();
    }, '修复已保存为新共同草稿，请核对后重新送审');
  }
  function close() {
    context.value = null;
    request.value = null;
    handled.value = {};
    acknowledged.value = false;
  }
  return {
    action,
    reference,
    context,
    request,
    editable,
    withdrawn,
    expired,
    handled,
    acknowledged,
    canSave,
    open,
    accept,
    refreshRequest,
    save,
    close,
  };
}
