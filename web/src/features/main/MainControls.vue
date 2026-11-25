<script setup lang="ts">
import type { Document } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import { actionsBlocked } from '../../lib/http';
import { useMainControls } from './useMainControls';
const props = defineProps<{
  document: Document;
  reload: () => Promise<void>;
  prepareLock: () => Promise<unknown>;
}>();
const { action, changeLock } = useMainControls(
  () => props.document,
  props.reload,
  props.prepareLock,
);
const { busy, error, notice, pending, retry } = action;
</script>

<template>
  <section class="management" role="region" aria-label="主线控制">
    <div class="section-title">
      <h3>{{ document.main_locked ? '主线已锁定' : '主线可编辑' }}</h3>
      <button :disabled="actionsBlocked" @click="changeLock">
        {{ document.main_locked ? '解锁主线' : '锁定主线' }}
      </button>
    </div>
    <p class="hint">锁定会固定当前可靠主线；共同支线可继续编辑。</p>
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
