<script setup lang="ts">
import type { DraftQueue } from '../domain/model';
defineProps<{ retained: DraftQueue[]; stale: boolean; text: string }>();
const emit = defineEmits<{ select: [string]; recover: []; keep: [] }>();
</script>
<template>
  <div v-if="retained.length" class="retained-inputs">
    <h4>本机保留的其他输入</h4>
    <div v-for="q in retained" :key="q.key">
      <span>{{ new Date(q.updatedAt).toLocaleString() }}{{ q.archive ? '（保留副本）' : '' }}</span>
      <button :disabled="stale" @click="emit('select', q.key)">查看该标签的输入</button>
    </div>
  </div>
  <div v-if="stale" class="recovery-box">
    <h4>旧工作区输入仍在本机</h4>
    <pre>{{ text }}</pre>
    <p class="hint">请先核对当前草稿。复制会替换当前看到的单元正文，旧输入会另外保留一份。</p>
    <button @click="emit('recover')">将旧输入复制到当前草稿</button>
    <button @click="emit('keep')">保留旧输入，继续当前草稿</button>
  </div>
</template>
