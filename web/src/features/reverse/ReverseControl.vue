<script setup lang="ts">
import type { ReviewRequest } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import { actionsBlocked } from '../../lib/http';
import { useReverseChange } from './useReverseChange';
const props = defineProps<{
  request: ReviewRequest;
  prepare: () => Promise<void>;
  reload: (request: string) => Promise<void>;
}>();
const { action, fixed, reason, acknowledged, canConfirm, inspect, confirm, close } =
  useReverseChange(() => props.request, props.prepare, props.reload);
const { busy, error, notice, pending, retry } = action;
</script>
<template>
  <section class="reverse-control" aria-label="反向修改">
    <button v-if="!fixed" type="button" :disabled="actionsBlocked" @click="inspect">
      反向创建新审核
    </button>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
    <form v-if="fixed" role="region" aria-label="反向修改确认" @submit.prevent="confirm">
      <h4>从原合并记录创建新审核</h4>
      <p>这会保留原审核和批准历史。主线在新审核正常通过并合并后才更新；独立草稿继续保留。</p>
      <p class="hint">
        原批准者 {{ fixed.approval.actor_id }} ·
        {{ new Date(fixed.approval.created_at).toLocaleString() }}
      </p>
      <p class="hint">
        原轮次 {{ fixed.approval.round_id }} · 送审版本 {{ fixed.approval.content_version_id }} ·
        合并记录 {{ fixed.approval.merge_record_id }}
      </p>
      <p class="hint">本次期待的正式主线清单 {{ fixed.manifest }}</p>
      <fieldset :disabled="actionsBlocked">
        <label>反向修改理由<textarea v-model="reason" required maxlength="2000"></textarea></label>
        <label
          ><input
            v-model="acknowledged"
            type="checkbox"
          />我确认创建新审核，再次批准后才更新主线</label
        >
        <div class="button-row">
          <button :disabled="!canConfirm">确认创建反向审核</button
          ><button type="button" @click="close">取消反向修改</button>
        </div>
      </fieldset>
    </form>
  </section>
</template>
