<script setup lang="ts">
import type { PullMain } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import { actionsBlocked } from '../../lib/http';
import { usePullMain } from './usePullMain';

const props = defineProps<{
  document: string;
  unit: string;
  disabled: boolean;
  prepare: () => Promise<Omit<PullMain, 'operation_id'>>;
  reload: () => Promise<void>;
}>();
const { action, pull } = usePullMain(props.document, props.unit, props.prepare, props.reload);
const { busy, error, notice, pending, retry } = action;
defineExpose({ pull, error, pending });
</script>
<template>
  <section class="draft-control" aria-label="拉取主线">
    <div class="section-title">
      <h4>更新共同草稿</h4>
      <button type="button" :disabled="disabled || actionsBlocked" @click="pull">
        {{ busy ? '正在合入主线…' : '拉取主线到草稿' }}
      </button>
    </div>
    <p class="hint">先确认本机输入已保存，再合入可靠主线；兼容的草稿修改会保留。</p>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
  </section>
</template>
