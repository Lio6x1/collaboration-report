<script setup lang="ts">
import {
  UserFacingError,
  actionsBlocked,
  api,
  ApiError,
  OperationRejected,
  useAction,
} from '../../lib/http';
import { computed, onMounted, ref, watch } from 'vue';
import type { Operation, Organization } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import AppDialog from '../../components/AppDialog.vue';

const props = defineProps<{ organization: Organization | null; allowed: boolean }>();
const emit = defineEmits<{ closed: [] }>();
const action = useAction(),
  confirmationName = ref(''),
  dialogOpen = ref(false),
  result = ref<Operation | null>(null);
const original = ref(
  new URLSearchParams(location.search).get('organization_close_operation') ?? '',
);
const terminal = computed(
  () => result.value && ['SUCCEEDED', 'REJECTED', 'CANCELLED'].includes(result.value.state),
);
const confirmed = computed(
  () => !!props.organization && confirmationName.value === props.organization.name,
);
watch(
  () => props.organization?.id,
  () => {
    confirmationName.value = '';
    // 切换组织清空确认；已受理的关闭操作仍按原号恢复，不跟随新组织重新提交。
    if (!original.value) dialogOpen.value = false;
  },
);
watch(dialogOpen, (open) => {
  if (!open) confirmationName.value = '';
});
function requestClose() {
  if (!props.allowed || !props.organization || actionsBlocked.value) return;
  confirmationName.value = '';
  dialogOpen.value = true;
}
function open() {
  if (original.value) dialogOpen.value = true;
  else requestClose();
}
// 入口由组织页标题栏摆放；恢复状态独立常驻，组织关闭后不再出现在目录也能继续查原操作。
defineExpose({ original, open });
function clearOriginal(closeDialog = true) {
  original.value = '';
  result.value = null;
  confirmationName.value = '';
  if (closeDialog) dialogOpen.value = false;
  const url = new URL(location.href);
  url.searchParams.delete('organization_close_operation');
  history.replaceState(null, '', url);
}
function retainOriginal(operation: string) {
  original.value = operation;
  const url = new URL(location.href);
  url.pathname = '/organizations';
  url.searchParams.set('organization_close_operation', operation);
  history.replaceState(null, '', url);
}
function showTerminal(value: Operation) {
  result.value = value;
  if (value.state === 'SUCCEEDED') emit('closed');
}
async function close() {
  if (!props.allowed || !props.organization || !confirmed.value) return;
  const organization = props.organization.id,
    revision = props.organization.revision;
  const operation = crypto.randomUUID();
  // 原号在提交前记录于当前地址；刷新只查询它，不以重新点击启动另一份清理。
  retainOriginal(operation);
  await action.run(async () => {
    try {
      const final = await action.mutate(`/v1/organizations/${organization}/closures`, {
        operation_id: operation,
        expected_revision: revision,
      });
      if ('scope_key' in final) throw new UserFacingError('组织关闭状态暂时无法确认，请稍后刷新。');
      showTerminal(final);
    } catch (error) {
      if (error instanceof OperationRejected) showTerminal(error.operation);
      // 只释放后端在登记前明确拒绝的输入。网络失败、进度404和关闭中冲突
      // 不能证明原请求未受理，仍保留原号供查询，避免重复启动清理。
      else if (
        error instanceof ApiError &&
        [
          'ORGANIZATION_OWNER_HANDOFF_REQUIRED',
          'REVISION_CHANGED',
          'ORGANIZATION_CLOSE_INPUT_INVALID',
        ].includes(error.code)
      )
        clearOriginal(false);
      throw error;
    }
  }, '组织已关闭，账号与个人空间继续有效');
}
async function inspect() {
  if (!original.value) return;
  const operation = original.value;
  await action.run(
    async () =>
      showTerminal(await api<Operation>(`/v1/account/operations/${encodeURIComponent(operation)}`)),
    '',
  );
}
onMounted(() => {
  if (original.value) {
    dialogOpen.value = true;
    void inspect();
  }
});
</script>

<template>
  <AppDialog
    v-model="dialogOpen"
    :title="original ? '组织关闭进度' : '关闭组织'"
    :blocked="actionsBlocked"
  >
    <section aria-label="独立关闭组织">
      <p class="hint">
        仅所有者作为最后一名存续成员时可关闭。当前公开正式版固定留存，组织私有内容永久清理；你的账号和个人空间继续有效。存在其他成员时请先完成所有者交接。
      </p>
      <form
        v-if="allowed && organization?.lifecycle === 'ACTIVE' && !original"
        @submit.prevent="close"
      >
        <p>即将关闭「{{ organization.name }}」。此操作无法撤销。</p>
        <label
          >输入组织名称以确认关闭<input
            v-model="confirmationName"
            type="text"
            :disabled="actionsBlocked"
            autocomplete="off"
            required
        /></label>
        <div class="button-row">
          <button type="button" :disabled="actionsBlocked" @click="dialogOpen = false">取消</button>
          <button type="submit" class="danger" :disabled="actionsBlocked || !confirmed">
            关闭组织并清理
          </button>
        </div>
      </form>
      <template v-if="original">
        <p v-if="result" role="status">
          {{
            result.state === 'SUCCEEDED'
              ? '组织已关闭，个人账号继续有效'
              : result.state === 'REJECTED'
                ? '原关闭操作已被拒绝'
                : '原关闭操作仍在处理'
          }}
        </p>
        <button type="button" :disabled="actionsBlocked" @click="inspect">
          查询原组织关闭进度
        </button>
        <button v-if="terminal" type="button" :disabled="actionsBlocked" @click="clearOriginal()">
          结束查看此操作
        </button>
      </template>
      <ActionStatus
        :busy="action.busy.value"
        :error="action.error.value"
        :notice="action.notice.value"
        :pending="action.pending.value"
        :retry="action.retry.value"
      />
    </section>
  </AppDialog>
</template>
