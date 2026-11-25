<script setup lang="ts">
import ActionStatus from '../../components/ActionStatus.vue';
import PaginationControls from '../../components/PaginationControls.vue';
import { useTrash } from './useTrash';
import { ref, watch } from 'vue';
import type { Document } from '../../../../contracts/http/generated';
import { transportEpoch } from '../../lib/http';
const props = defineProps<{ space: string }>();
const emit = defineEmits<{ restored: [id: string] }>();
const {
  action,
  documents,
  pagination,
  loading,
  readingError,
  blocked,
  load,
  refresh,
  restore,
  purge,
} = useTrash(
  () => props.space,
  (id) => emit('restored', id),
);
const deleting = ref<Document | null>(null),
  confirmed = ref(false);
watch([() => props.space, transportEpoch], () => {
  deleting.value = null;
  confirmed.value = false;
});
function confirmDeletion(document: Document) {
  deleting.value = { ...document };
  confirmed.value = false;
}
async function confirmPurge() {
  if (!deleting.value) return;
  await purge(deleting.value);
  if (!action.error.value && !action.pending.value) {
    deleting.value = null;
    confirmed.value = false;
  }
}
</script>

<template>
  <section class="panel" aria-label="文档回收站">
    <div class="section-title">
      <h2>文档回收站</h2>
      <button type="button" :disabled="blocked" @click="refresh">刷新回收站</button>
    </div>
    <p class="hint">
      删除后保留 30 天。恢复保留内容与历史，正式阅读仍然关闭，原直接授权不会自动恢复。
    </p>
    <ActionStatus
      v-if="!action.pending.value"
      :busy="action.busy.value"
      :error="action.error.value"
      :notice="action.notice.value"
      :pending="action.pending.value"
      :retry="action.retry.value"
    />
    <p v-if="readingError" role="alert">{{ readingError }}</p>
    <p v-if="loading" role="status">正在读取回收站…</p>
    <ul class="entity-list" aria-label="回收文档列表">
      <li v-for="document in documents" :key="document.id">
        <span
          >{{ document.title }} <small>ID {{ document.id }}</small></span
        >
        <button type="button" :disabled="blocked" @click="restore(document)">恢复文档</button>
        <button type="button" class="danger" :disabled="blocked" @click="confirmDeletion(document)">
          永久删除
        </button>
      </li>
    </ul>
    <PaginationControls
      label="回收文档"
      :page="pagination.index + 1"
      :has-next="!!pagination.next"
      :busy="blocked"
      @change="load"
    />
    <div v-if="deleting" class="purge-confirmation" aria-label="永久删除确认">
      <p>永久删除「{{ deleting.title }}」将清除正文和内容历史，完成后无法恢复。</p>
      <label
        ><input
          v-model="confirmed"
          type="checkbox"
          :disabled="blocked"
        />我确认永久删除这篇文档及其内容历史</label
      >
      <div class="toolbar">
        <button
          type="button"
          class="danger"
          :disabled="blocked || !confirmed"
          @click="confirmPurge"
        >
          确认永久删除
        </button>
        <button
          type="button"
          :disabled="blocked"
          @click="
            deleting = null;
            confirmed = false;
          "
        >
          取消
        </button>
      </div>
    </div>
    <p v-if="!loading && !readingError && !documents.length" class="empty">
      本页没有当前可管理的回收文档。
    </p>
  </section>
</template>
<style scoped>
.purge-confirmation {
  width: 100%;
  padding: 12px 0;
}
.purge-confirmation label {
  display: flex;
  gap: 8px;
  align-items: center;
}
.purge-confirmation .toolbar {
  display: flex;
  gap: 12px;
  margin-top: 12px;
}
</style>
