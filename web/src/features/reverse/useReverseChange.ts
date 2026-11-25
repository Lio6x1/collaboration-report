import { UserFacingError, useAction } from '../../lib/http';
import { computed, ref } from 'vue';
import type {
  ReviewAction,
  ReviewRequest,
  ReverseChange,
} from '../../../../contracts/http/generated';
import { reversePath, reverseSource } from './api';

type Confirmation = { document: string; approval: ReviewAction; manifest: string };

export function useReverseChange(
  current: () => ReviewRequest,
  prepare: () => Promise<void>,
  reload: (request: string) => Promise<void>,
) {
  const action = useAction(),
    fixed = ref<Confirmation | null>(null);
  const reason = ref(''),
    acknowledged = ref(false);
  const canConfirm = computed(() => !!fixed.value && !!reason.value.trim() && acknowledged.value);

  async function inspect() {
    acknowledged.value = false;
    await action.run(async () => {
      const expected = current(),
        source = await reverseSource(expected);
      fixed.value = { document: expected.document_id, ...source };
    }, '原批准来源已核验，请确认新审核的理由');
  }
  async function confirm() {
    if (!fixed.value || !canConfirm.value) return;
    const original = fixed.value;
    const input: Omit<ReverseChange, 'operation_id'> = {
      expected_manifest_id: original.manifest,
      merge_record_id: original.approval.merge_record_id!,
      reason: reason.value.trim(),
    };
    let prepared = false;
    await action.run(async () => {
      // 原号重试固定同一来源、正式清单和理由；只在首次登记前IME/FLUSH，不采后来草稿替换原输入。
      if (!prepared) {
        await prepare();
        prepared = true;
      }
      const result = await action.mutate(reversePath(original.document), input);
      const request = 'result' in result ? result.result?.request_id : undefined;
      if (!request) throw new UserFacingError('新审核编号尚未核验，请查询原操作');
      // settle已等待实际新BRANCH安装。旧代次不能再FLUSH；父级刷新并定位真实新请求。
      await reload(request);
    }, '已创建新的固定审核，请核对后正常通过并合并');
  }
  function close() {
    fixed.value = null;
    acknowledged.value = false;
    reason.value = '';
  }
  return { action, fixed, reason, acknowledged, canConfirm, inspect, confirm, close };
}
