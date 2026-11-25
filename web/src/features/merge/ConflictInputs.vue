<script setup lang="ts">
import { ref, computed, defineAsyncComponent } from 'vue';
import type { ConflictContext } from '../../../../contracts/http/generated';
const ContentView = defineAsyncComponent(() => import('../../lib/content/ContentView.vue'));
const props = defineProps<{ context: ConflictContext }>();
const selected = ref<'base' | 'submitted' | 'main'>('submitted');
const inputs = [
  { key: 'base' as const, title: '原基线', label: '冲突原基线正文' },
  { key: 'submitted' as const, title: '原送审', label: '冲突原送审正文' },
  { key: 'main' as const, title: '原主线', label: '冲突原主线正文' },
];
const current = computed(() => inputs.find((item) => item.key === selected.value)!);
</script>
<template>
  <section role="region" aria-label="本次固定冲突" class="conflict-inputs">
    <h4>本次固定冲突</h4>
    <p class="hint">原尝试 {{ context.attempt_id }} · 固定轮次 {{ context.round_id }}</p>
    <p class="hint">读取有效至 {{ new Date(context.expires_at).toLocaleString() }}</p>
    <p>请依据这些固定输入处理冲突；之后的主线和共同草稿仍单独保留。</p>
    <ul>
      <li v-for="item in context.conflicts" :key="item.id">
        {{ item.reason }}
        <small>块 {{ item.block_id }} · 位置 {{ item.node_path }} · {{ item.kind }}</small>
      </li>
    </ul>
    <!-- 样式是原尝试的实际排版上下文，三个正文分别来自真实不可变版本，不声称是各自历史排版。 -->
    <nav class="document-tabs" aria-label="固定冲突输入">
      <button
        v-for="item in inputs"
        :key="item.key"
        :aria-current="selected === item.key ? 'page' : undefined"
        @click="selected = item.key"
      >
        {{ item.title }}
      </button>
    </nav>
    <div class="conflict-input-body">
      <ContentView
        :key="current.key"
        :content="props.context[current.key]"
        :style="context.style"
        :resource-context="{ document: context.document_id, kind: 'REVIEW', id: context.round_id }"
        :label="current.label"
      />
    </div>
  </section>
</template>

<style scoped>
.conflict-input-body {
  max-height: 48dvh;
  overflow: auto;
  padding: 16px;
  border: 1px solid var(--line);
  border-radius: 10px;
}
</style>
