import { UserFacingError, useAction } from '../../lib/http';
import { computed, onUnmounted, ref } from 'vue';
import type {
  DiscardDraft,
  DiscardPreview,
  DiscardPreviewCreate,
  FixedContent,
} from '../../../../contracts/http/generated';
import { currentUnitRequest } from '../review/api';
import { discardPreviewPath, discardDraftPath, readFixedContent } from './api';

type Boundary = Omit<DiscardPreviewCreate, 'operation_id'>;
type FixedPreview = { response: DiscardPreview; content: FixedContent; round?: string };

export function useDiscardDraft(
  document: string,
  unit: string,
  prepare: () => Promise<Boundary>,
  reload: () => Promise<void>,
) {
  const action = useAction();
  const fixed = ref<FixedPreview | null>(null);
  const reasons = ref<Record<string, string>>({});
  const acknowledged = ref(false);
  const now = ref(Date.now());
  // 本机到期提示只禁用新确认，原号恢复由全局状态继续；服务端仍独立核验期限。
  const timer = setInterval(() => {
    now.value = Date.now();
  }, 1000);
  onUnmounted(() => clearInterval(timer));
  const expired = computed(() => {
    if (!fixed.value) return true;
    const expires = Date.parse(fixed.value.response.expires_at);
    return !Number.isFinite(expires) || expires <= now.value;
  });
  const canConfirm = computed(
    () =>
      !!fixed.value &&
      !expired.value &&
      acknowledged.value &&
      fixed.value.response.blocking_changes.every((change) => !!reasons.value[change.id]?.trim()),
  );

  async function preview() {
    let boundary: Boundary | undefined;
    let checked = false,
      round: string | undefined;
    acknowledged.value = false;
    await action.run(async () => {
      boundary ??= await prepare();
      if (!checked) {
        const request = await currentUnitRequest(document, unit, boundary.expected_branch_id);
        if (request?.status === 'REVIEWING')
          throw new UserFacingError('请先撤回当前审核，再核对丢弃预览');
        round = request?.current_round_id;
        checked = true;
      }
      // POST未知仍重取原预览；版本GET失败沿原业务续接，不另截当前更晚草稿。
      const response = await action.write<DiscardPreview>(
        discardPreviewPath(document, unit),
        boundary,
      );
      const content = await readFixedContent(response.draft_content_version_id);
      if (
        response.document_id !== document ||
        response.unit_id !== unit ||
        response.branch_id !== boundary.expected_branch_id ||
        response.structure_id !== boundary.expected_structure_id ||
        content.id !== response.draft_content_version_id ||
        content.document_id !== document ||
        content.unit_id !== unit ||
        content.structure_id !== response.structure_id
      )
        throw new UserFacingError('固定预览归属尚未确认，请保留原操作并重试');
      fixed.value = { response, content, round };
      reasons.value = {};
      now.value = Date.now();
    }, '旧稿已固定保留，请逐条核对并明确确认');
  }

  async function confirm() {
    const original = fixed.value;
    if (!original || !canConfirm.value) return;
    const input: Omit<DiscardDraft, 'operation_id'> = {
      expected_branch_id: original.response.branch_id,
      expected_structure_id: original.response.structure_id,
      ...(original.round ? { expected_round_id: original.round } : {}),
      preview_cut_ref: original.response.preview_cut_ref,
      changes_to_resolve: original.response.blocking_changes.map((change) => ({
        id: change.id,
        revision: change.revision,
        reason: reasons.value[change.id].trim(),
      })),
    };
    let prepared = false;
    await action.run(async () => {
      // 预览后的输入必须先可靠保存，让原预览明确CHANGED；不能悄悄重采并丢掉新工作。
      if (!prepared) {
        await prepare();
        prepared = true;
      }
      await action.mutate(discardDraftPath(document, unit), input);
      // READY包含实际恢复和撤回工作归档；刷新失败只续接刷新，绝不重复恢复新epoch。
      await reload();
    }, '共同草稿已恢复，旧稿和处理理由已保留');
  }

  function close() {
    fixed.value = null;
    reasons.value = {};
    acknowledged.value = false;
  }
  return { action, fixed, reasons, acknowledged, expired, canConfirm, preview, confirm, close };
}
