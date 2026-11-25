<script setup lang="ts">
import AppPopover from '../../../components/AppPopover.vue';
const markdown = defineModel<string>({ required: true });
const emit = defineEmits<{ import: [] }>();
</script>
<template>
  <AppPopover label="导入 Markdown">
    <label>Markdown 正文<textarea v-model="markdown" rows="6" maxlength="65536"></textarea></label>
    <p class="hint">加入光标所在位置。单次规范正文最多256KiB，外部图片会明确保留为待转存文本。</p>
    <button @click="emit('import')">将 Markdown 加入当前草稿</button>
  </AppPopover>
</template>
