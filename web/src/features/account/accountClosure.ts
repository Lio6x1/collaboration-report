import { UserFacingError, api, ApiError, explain, useAction } from '../../lib/http';
import { computed, ref, watch } from 'vue';
import type { AccountClose, AccountClosePlan } from '../../../../contracts/http/generated';
import { user, afterAccountClosure } from './session';

type SavedClosure = { operation: string; actor: string; epoch: string };
const storageKey = 'collaboration-report.account-closure';
function restore(): SavedClosure | null {
  try {
    const value = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null');
    return value &&
      typeof value.operation === 'string' &&
      /^[0-9a-f-]{36}$/i.test(value.operation) &&
      typeof value.actor === 'string' &&
      typeof value.epoch === 'string'
      ? value
      : null;
  } catch {
    return null;
  }
}

// 注销跨越正常会话失效边界，不能交给依赖正常会话的 /operations 恢复器。
// 持久化只有原号及所属身份；密码只用于原请求内存重放，绝不进入浏览器存储。
export const closure = ref<SavedClosure | null>(restore());
export const closurePlan = ref<AccountClosePlan | null>(null);
export const closureError = ref('');
export const closureReading = ref(false);
const action = useAction();
export const closureSending = action.busy;
let originalInput: AccountClose | null = null;
watch(
  () => user.value?.id,
  (actor) => {
    // 身份离开原账号即释放密码，保留最小原号仍可走恢复Cookie读取进度。
    if (actor !== closure.value?.actor) originalInput = null;
  },
);
export const closureVisible = computed(
  () => !!closure.value && (!user.value || user.value.id === closure.value.actor),
);
export const closureTerminal = computed(
  () => closurePlan.value?.state === 'SUCCEEDED' || closurePlan.value?.state === 'REJECTED',
);
export const canReplayClosure = computed(
  () => !!closure.value && !closurePlan.value && user.value?.id === closure.value.actor,
);

function accept(plan: AccountClosePlan, saved: SavedClosure) {
  if (closure.value?.operation !== saved.operation) return;
  if (
    plan.operation_id !== saved.operation ||
    !['PREPARING', 'FROZEN', 'PLANNED', 'CLEANING', 'SUCCEEDED', 'REJECTED'].includes(plan.state)
  )
    throw new UserFacingError('注销结果与原操作不一致，请重新查询原操作');
  closurePlan.value = plan;
  originalInput = null;
  if (['FROZEN', 'PLANNED', 'CLEANING', 'SUCCEEDED'].includes(plan.state)) {
    // 服务端明确确认冻结才清登录状态；404、超时或旧Cookie拒绝都不能推导注销成功。
    afterAccountClosure(saved.actor);
  }
}

export async function readClosure() {
  const saved = closure.value;
  if (!saved || !closureVisible.value || closureReading.value || closureSending.value) return;
  closureReading.value = true;
  closureError.value = '';
  try {
    const plan = await api<AccountClosePlan>(
      '/v1/account/closures/' + encodeURIComponent(saved.operation),
    );
    if (closureVisible.value) accept(plan, saved);
  } catch (reason) {
    if (closure.value?.operation === saved.operation)
      closureError.value = '注销结果尚未确认，请保留原操作编号并重新查询。' + explain(reason);
  } finally {
    closureReading.value = false;
  }
}

async function sendClosure(input: AccountClose, saved: SavedClosure) {
  await action.run(async () => {
    closureError.value = '';
    try {
      // 复用现有8秒HTTP预算、Origin与CSRF；202只代表已接收，以专用计划状态显示进度。
      accept(await api<AccountClosePlan>('/v1/account/closures', 'POST', input), saved);
    } catch (reason) {
      if (reason instanceof ApiError && reason.code === 'CURRENT_PASSWORD_INVALID') {
        // 密码校验明确拒绝发生在登记前，可重新确认；其他冲突/未知结果必须保留原号。
        closure.value = null;
        originalInput = null;
        sessionStorage.removeItem(storageKey);
        closureError.value = explain(reason);
      } else {
        closureError.value =
          '注销申请结果尚未确认；请查询或重试原申请，不要另建操作。' + explain(reason);
      }
    }
  }, '');
}

export async function startClosure(password: string) {
  if (!user.value || closure.value || closureSending.value) return;
  if (!password) {
    closureError.value = '请输入当前密码';
    return;
  }
  const saved = {
    operation: crypto.randomUUID(),
    actor: user.value.id,
    epoch: user.value.identity_epoch,
  };
  if (!saved.epoch) {
    closureError.value = '身份代次尚未确认，请刷新账号信息';
    return;
  }
  try {
    sessionStorage.setItem(storageKey, JSON.stringify(saved));
  } catch {
    closureError.value = '浏览器无法保留原操作编号，请允许会话存储后再申请注销';
    return;
  }
  closure.value = saved;
  closurePlan.value = null;
  originalInput = {
    operation_id: saved.operation,
    expected_identity_epoch: saved.epoch,
    current_password: password,
  };
  await sendClosure(originalInput, saved);
}

export async function replayClosure(password: string) {
  const saved = closure.value;
  if (!saved || !canReplayClosure.value || closureSending.value || closureReading.value) return;
  if (!originalInput && !password) {
    closureError.value = '请重新输入原申请使用的密码';
    return;
  }
  originalInput ??= {
    operation_id: saved.operation,
    expected_identity_epoch: saved.epoch,
    current_password: password,
  };
  await sendClosure(originalInput, saved);
}

export function dismissClosure() {
  if (!closureTerminal.value) return;
  closure.value = null;
  closurePlan.value = null;
  originalInput = null;
  closureError.value = '';
  sessionStorage.removeItem(storageKey);
}
