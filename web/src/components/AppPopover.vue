<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, useId } from 'vue';
const props = defineProps<{ label: string; buttonText?: string; disabled?: boolean }>();
const expanded = defineModel<boolean>({ default: false });
const root = ref<HTMLElement | null>(null),
  trigger = ref<HTMLButtonElement | null>(null);
const panelID = useId();
function close(focus = false) {
  expanded.value = false;
  if (focus) trigger.value?.focus({ preventScroll: true });
}
function open() {
  if (!props.disabled) expanded.value = true;
}
function outside(event: Event) {
  if (!root.value?.contains(event.target as Node)) close();
}
function keyboard(event: KeyboardEvent) {
  if (event.key === 'Escape' && expanded.value) {
    event.stopPropagation();
    close(true);
  }
}
function anotherOpened(event: Event) {
  if ((event as CustomEvent<string>).detail !== panelID) close();
}
watch(expanded, (value) => {
  // 浮层互斥只隐藏呈现，不卸载slot：上传任务、输入和编辑器选区继续保留。
  if (value) window.dispatchEvent(new CustomEvent('collab-popover-open', { detail: panelID }));
});
onMounted(() => {
  window.addEventListener('pointerdown', outside);
  window.addEventListener('keydown', keyboard);
  window.addEventListener('collab-popover-open', anotherOpened);
});
onUnmounted(() => {
  window.removeEventListener('pointerdown', outside);
  window.removeEventListener('keydown', keyboard);
  window.removeEventListener('collab-popover-open', anotherOpened);
});
defineExpose({ open, close });
</script>

<template>
  <div ref="root" class="app-popover">
    <button
      ref="trigger"
      type="button"
      :disabled="disabled"
      :aria-label="label"
      :aria-expanded="expanded"
      :aria-controls="panelID"
      @mousedown.prevent
      @click="expanded ? close() : open()"
    >
      {{ buttonText ?? label
      }}<span v-if="!buttonText" aria-hidden="true" class="popover-chevron">⌄</span>
    </button>
    <div v-show="expanded" :id="panelID" class="popover-panel" role="dialog" :aria-label="label">
      <slot :close="close" />
    </div>
  </div>
</template>

<style scoped>
.app-popover {
  position: relative;
  min-width: 0;
}
.popover-chevron {
  color: var(--muted);
}
.popover-panel {
  position: absolute;
  z-index: 35;
  top: calc(100% + 8px);
  left: 0;
  width: min(440px, calc(100vw - 64px));
  max-height: min(520px, 65dvh);
  overflow: auto;
  padding: 20px;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
  box-shadow: 0 12px 36px #172e312b;
}
@media (max-width: 760px) {
  .popover-panel {
    position: fixed;
    top: auto;
    bottom: 16px;
    left: 16px;
    right: 16px;
    width: auto;
    max-height: 70dvh;
  }
}
</style>
