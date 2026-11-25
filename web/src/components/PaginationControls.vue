<script setup lang="ts">
defineProps<{ label: string; page: number; hasNext: boolean; busy?: boolean }>();
defineEmits<{ change: [index: number] }>();
</script>

<template>
  <nav v-if="page > 1 || hasNext" class="pagination-controls" :aria-label="label + '分页'">
    <button type="button" :disabled="busy || page <= 1" @click="$emit('change', page - 2)">
      上一页
    </button>
    <span aria-live="polite" aria-atomic="true">第 {{ page }} 页</span>
    <button type="button" :disabled="busy || !hasNext" @click="$emit('change', page)">
      下一页
    </button>
  </nav>
</template>

<style scoped>
.pagination-controls {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--line);
}
.pagination-controls span {
  color: var(--muted);
  white-space: nowrap;
}
.pagination-controls button {
  min-height: 40px;
}
</style>
