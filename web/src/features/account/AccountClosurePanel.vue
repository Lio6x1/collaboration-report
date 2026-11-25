<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';
import { actionsBlocked } from '../../lib/http';
import { user } from './session';
import {
  closure,
  closurePlan,
  closureError,
  closureReading,
  closureSending,
  closureVisible,
  closureTerminal,
  canReplayClosure,
  startClosure,
  readClosure,
  replayClosure,
  dismissClosure,
} from './accountClosure';
const emit = defineEmits<{ succession: [kind: string, target: string] }>();
const confirming = ref(false);
const confirmed = ref(false);
const password = ref('');
const replayPassword = ref('');
const labels = {
  PREPARING: '正在核验交接要求',
  FROZEN: '账号已冻结，正在准备清理',
  PLANNED: '清理计划已固定',
  CLEANING: '正在清理账号数据',
  SUCCEEDED: '账号已注销',
  REJECTED: '注销申请已拒绝',
};
const successionLabels = {
  ORGANIZATION_OWNER: '组织所有者',
  ORGANIZATION_ADMIN: '组织管理员',
  KNOWLEDGE_BASE_ADMIN: '知识库管理员',
};
async function submit() {
  if (!confirmed.value || actionsBlocked.value) return;
  const input = password.value;
  password.value = '';
  await startClosure(input);
}
async function replay() {
  const input = replayPassword.value;
  replayPassword.value = '';
  await replayClosure(input);
}
let timer: ReturnType<typeof setInterval>;
onMounted(() => {
  if (closureVisible.value) void readClosure();
  timer = setInterval(() => {
    if (
      closureVisible.value &&
      !closureTerminal.value &&
      !closureError.value &&
      !actionsBlocked.value
    )
      void readClosure();
  }, 3000);
});
onUnmounted(() => {
  clearInterval(timer);
  password.value = '';
  replayPassword.value = '';
});
</script>

<template>
  <section v-if="user || closureVisible" class="panel" aria-label="账号注销">
    <h2>注销账号</h2>
    <template v-if="closureVisible && closure">
      <p class="hint">原操作编号：{{ closure.operation }}</p>
      <p role="status">{{ closurePlan ? labels[closurePlan.state] : '注销结果尚未确认' }}</p>
      <p v-if="closurePlan?.state === 'SUCCEEDED'">
        按留存规则保留 {{ closurePlan.retained_release_ids.length }} 个正式版本。
      </p>
      <template
        v-if="closurePlan?.state === 'PREPARING' && closurePlan.required_successions.length"
      >
        <p>
          请先完成以下角色交接。准备阶段仍可使用账号，交接后返回这里查询原申请；后台会继续推进。
        </p>
        <ul>
          <li
            v-for="item in closurePlan.required_successions"
            :key="item.kind + ':' + item.target_id"
          >
            {{ successionLabels[item.kind] }} · {{ item.target_id }}
            <button
              type="button"
              :disabled="actionsBlocked"
              @click="emit('succession', item.kind, item.target_id)"
            >
              {{
                item.kind === 'KNOWLEDGE_BASE_ADMIN' ? '前往工作台交接知识库' : '前往组织管理交接'
              }}
            </button>
          </li>
        </ul>
      </template>
      <p
        v-if="closurePlan && !['PREPARING', 'SUCCEEDED', 'REJECTED'].includes(closurePlan.state)"
        class="hint"
      >
        账号已进入关闭流程。此页只查询原注销进度，完成前请保留操作编号。
      </p>
      <button type="button" :disabled="closureReading || closureSending" @click="readClosure">
        {{ closureReading ? '正在查询注销进度…' : '查询注销进度' }}
      </button>
      <form v-if="canReplayClosure" @submit.prevent="replay">
        <p class="hint">
          响应丢失时先查询。若仍需重放，请使用原申请密码；原操作号和身份代次保持不变。
        </p>
        <label
          >原申请确认密码<input
            v-model="replayPassword"
            type="password"
            autocomplete="current-password"
        /></label>
        <button type="submit" :disabled="actionsBlocked || closureReading">重试原注销申请</button>
      </form>
      <button v-if="closureTerminal" type="button" @click="dismissClosure">关闭注销结果</button>
    </template>
    <template v-else-if="user && !closure">
      <p>
        注销会关闭登录及协作资格并清理账号数据，部分正式版本按留存规则保留。请先完成组织和知识库管理角色交接。
      </p>
      <button
        v-if="!confirming"
        type="button"
        :disabled="actionsBlocked"
        @click="confirming = true"
      >
        申请注销账号
      </button>
      <form v-else @submit.prevent="submit">
        <fieldset :disabled="actionsBlocked || closureSending">
          <label
            >注销确认密码<input
              v-model="password"
              type="password"
              required
              autocomplete="current-password"
          /></label>
          <label
            ><input
              v-model="confirmed"
              type="checkbox"
              required
            />我理解账号冻结后将无法继续使用，确认申请注销</label
          >
          <button type="submit">确认申请注销</button>
          <button
            type="button"
            @click="
              confirming = false;
              confirmed = false;
              password = '';
            "
          >
            取消
          </button>
        </fieldset>
      </form>
    </template>
    <p v-else class="hint">本标签保留着另一个账号的原注销编号，请使用原账号恢复查询。</p>
    <p v-if="closureSending" role="status">正在提交原注销申请…</p>
    <p v-if="closureError && (!closure || closureVisible)" class="error" role="alert">
      {{ closureError }}
    </p>
  </section>
</template>
