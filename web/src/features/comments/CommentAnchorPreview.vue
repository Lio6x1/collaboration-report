<script setup lang="ts">
import { UserFacingError, api, explain, transportEpoch } from '../../lib/http';
import { ref, watch, onBeforeUnmount, nextTick } from 'vue';
import type { CommentThread, FixedContent } from '../../../../contracts/http/generated';
import ContentView from '../../lib/content/ContentView.vue';

const props = withDefaults(defineProps<{ thread: CommentThread; showQuote?: boolean }>(), {
  showQuote: true,
});
const content = ref<FixedContent | null>(null);
const loading = ref(false);
const error = ref('');
const preview = ref<HTMLElement | null>(null);
let active = true;
let generation = 0;
function clear() {
  generation++;
  content.value = null;
  loading.value = false;
  error.value = '';
}
async function open() {
  const current = ++generation;
  content.value = null;
  error.value = '';
  loading.value = true;
  try {
    // 原线程详情和原正文分别重新授权；不把列表里留存的锚点当成永久访问凭据。
    const thread = await api<CommentThread>('/v1/comment-threads/' + props.thread.id);
    if (!active || current !== generation) return;
    if (
      thread.document_id !== props.thread.document_id ||
      thread.context_kind !== props.thread.context_kind ||
      thread.context_id !== props.thread.context_id ||
      JSON.stringify(thread.anchor) !== JSON.stringify(props.thread.anchor)
    )
      throw new UserFacingError('讨论原始上下文已变化，请刷新讨论');
    const fixed = await api<FixedContent>(
      '/v1/content-versions/' + thread.anchor.content_version_id,
    );
    if (!active || current !== generation) return;
    if (fixed.id !== thread.anchor.content_version_id || fixed.document_id !== thread.document_id)
      throw new UserFacingError('原正文版本不一致，请刷新讨论');
    content.value = fixed;
    await nextTick();
    if (!active || current !== generation) return;
    // 原文可能很长，只滚动预览内部到锚点，避免把当前评论和整页一起推走。
    const container = preview.value;
    const anchor = container?.querySelector('.comment-anchor-highlight, .comment-anchor-caret');
    if (container && anchor)
      container.scrollTop +=
        anchor.getBoundingClientRect().top - container.getBoundingClientRect().top - 24;
  } catch (reason) {
    if (active && current === generation) error.value = explain(reason);
  } finally {
    if (current === generation) loading.value = false;
  }
}
watch(() => [props.thread.id, props.thread.revision, transportEpoch.value], clear);
onBeforeUnmount(() => {
  active = false;
  clear();
});
</script>

<template>
  <section class="anchor-preview" aria-label="讨论原文位置">
    <blockquote v-if="showQuote && thread.anchor.quote">{{ thread.anchor.quote }}</blockquote>
    <button v-if="!content" type="button" :disabled="loading" @click="open">
      {{ loading ? '正在核验原正文…' : '查看原文位置' }}
    </button>
    <button v-else type="button" @click="clear">收起原文位置</button>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <div
      v-if="content"
      ref="preview"
      class="original-preview"
      role="region"
      aria-label="历史原文预览"
      tabindex="0"
    >
      <ContentView
        :content="content"
        :anchor="thread.anchor"
        :resource-context="{
          document: thread.document_id,
          kind: thread.context_kind,
          id: thread.context_id,
        }"
        label="讨论原始锚点正文"
      />
    </div>
  </section>
</template>

<style scoped>
.original-preview {
  max-height: min(320px, 45vh);
  overflow: auto;
  overscroll-behavior: contain;
  margin-top: 0.75rem;
  padding: 0.75rem;
  border: 1px solid var(--line);
  border-radius: 0.5rem;
}
</style>
