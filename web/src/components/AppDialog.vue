<script setup lang="ts">
import { ref, watch, onMounted, useId } from 'vue';
const props = defineProps<{ title: string; blocked?: boolean }>();
const open = defineModel<boolean>({ required: true });
const dialog = ref<HTMLDialogElement | null>(null);
const titleID = useId();
function dismiss() {
  // 未知写入必须保留原请求及恢复界面，不能用关闭弹窗伪装取消。
  if (!props.blocked) open.value = false;
}
function synchronize() {
  const element = dialog.value;
  if (!element) return;
  if (open.value && !element.open) element.showModal();
  if (!open.value && element.open) element.close();
}
watch(open, synchronize, { flush: 'post' });
onMounted(synchronize);
</script>

<template>
  <Teleport to="body">
    <dialog
      ref="dialog"
      class="app-dialog"
      :aria-labelledby="titleID"
      @cancel.prevent="dismiss"
      @click="$event.target === dialog && dismiss()"
    >
      <div class="dialog-heading">
        <h2 :id="titleID">{{ title }}</h2>
        <button
          type="button"
          class="dialog-close"
          :disabled="blocked"
          aria-label="关闭弹窗"
          @click="dismiss"
        >
          ×
        </button>
      </div>
      <!-- dialog关闭时保留slot，重新打开不会清空尚未提交的输入。 -->
      <slot />
    </dialog>
  </Teleport>
</template>

<style scoped>
.app-dialog {
  width: min(580px, calc(100vw - 32px));
  max-height: calc(100dvh - 40px);
  overflow: auto;
  margin: auto;
  padding: 28px;
  border: 1px solid var(--line);
  border-radius: 16px;
  color: var(--ink);
  background: var(--paper);
  box-shadow: 0 24px 90px #152a3540;
}
.app-dialog::backdrop {
  background: #1d302b66;
  backdrop-filter: blur(3px);
}
.dialog-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}
.dialog-heading h2 {
  margin: 0;
  font-size: var(--ui-font-section);
}
.dialog-close {
  min-width: 44px;
  padding: 2px 12px;
  border-color: transparent;
  font-size: 26px;
}
@media (max-width: 560px) {
  .app-dialog {
    padding: 20px;
  }
}
</style>
