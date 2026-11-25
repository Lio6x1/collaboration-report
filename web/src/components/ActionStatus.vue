<script setup lang="ts">
defineProps<{
  error: string;
  notice: string;
  pending: string;
  busy: boolean;
  retry: null | (() => Promise<void>);
}>();
</script>
<template>
  <div class="feedback" aria-live="polite">
    <p v-if="busy" class="busy-feedback">
      <span class="activity-dot" aria-hidden="true"></span>正在处理，请稍候…
    </p>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success" role="status">{{ notice }}</p>
    <template v-if="pending"
      ><details class="hint">
        <summary>查看操作凭证</summary>
        <code>{{ pending }}</code>
      </details>
      <button v-if="retry" type="button" :disabled="busy" @click="retry">
        确认操作结果
      </button></template
    >
  </div>
</template>
