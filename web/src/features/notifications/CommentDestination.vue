<script setup lang="ts">
import { UserFacingError, api, explain, transportEpoch } from '../../lib/http';
import { ref, watch, onBeforeUnmount } from 'vue';
import type { CommentThread, Document, FixedContent } from '../../../../contracts/http/generated';
import { user } from '../account/session';
import ContentView from '../../lib/content/ContentView.vue';
import CommentsPanel from '../comments/CommentsPanel.vue';

const props = defineProps<{ thread: string }>();
const target = ref<CommentThread | null>(null);
const document = ref<Document | null>(null);
const content = ref<FixedContent | null>(null);
const actions = ref<string[]>([]);
const loading = ref(false);
const error = ref('');
let active = true;
let generation = 0;

async function load() {
  const current = ++generation;
  target.value = null;
  document.value = null;
  content.value = null;
  actions.value = [];
  error.value = '';
  loading.value = true;
  try {
    // 提醒只携线程ID。归属和锚点从当前授权的详情接口取得，不由客户端猜测。
    const thread = await api<CommentThread>(
      '/v1/comment-threads/' + encodeURIComponent(props.thread),
    );
    if (!active || current !== generation) return;
    let available: string[] = [];
    const doc = await api<Document>(
      '/v1/documents/' + thread.document_id,
      'GET',
      undefined,
      (headers) => {
        const value: unknown = JSON.parse(headers.get('X-Available-Actions') ?? '[]');
        if (Array.isArray(value) && value.every((item) => typeof item === 'string'))
          available = value;
      },
    );
    const fixed = await api<FixedContent>(
      '/v1/content-versions/' + thread.anchor.content_version_id,
    );
    if (!active || current !== generation) return;
    // 固定版本读取另走正文授权，旧提醒本身不构成读取正文的凭据。
    if (fixed.id !== thread.anchor.content_version_id)
      throw new UserFacingError('原批注正文版本不一致，请重新打开');
    target.value = thread;
    document.value = doc;
    content.value = fixed;
    actions.value = available;
  } catch (reason) {
    if (active && current === generation) error.value = explain(reason);
  } finally {
    if (current === generation) loading.value = false;
  }
}

watch(
  () => [props.thread, transportEpoch.value],
  () => void load(),
  { immediate: true },
);
onBeforeUnmount(() => {
  active = false;
  generation++;
});
</script>

<template>
  <section aria-label="通知原始讨论">
    <p v-if="loading" role="status">正在核验并读取原讨论…</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <button v-if="error" type="button" :disabled="loading" @click="load">重新打开原讨论</button>
    <template v-if="target && document && content && user">
      <h2>{{ document.title }} · 原始讨论</h2>
      <p class="hint">
        固定正文版本 {{ content.id }} · {{ target.context_kind }} {{ target.context_id }}
      </p>
      <p v-if="target.anchor.block_id" class="hint">原始块 {{ target.anchor.block_id }}</p>
      <blockquote v-if="target.anchor.quote">{{ target.anchor.quote }}</blockquote>
      <ContentView
        class="paper"
        :content="content"
        :anchor="target.anchor"
        :resource-context="{
          document: target.document_id,
          kind: target.context_kind,
          id: target.context_id,
        }"
        label="批注原始固定正文"
      />
      <CommentsPanel
        :key="target.id"
        :document="target.document_id"
        :space="document.space_id"
        :context-kind="target.context_kind"
        :context-id="target.context_id"
        :version="target.anchor.content_version_id"
        :actor="user.id"
        :can-comment="actions.includes('COMMENT')"
        :can-manage="actions.includes('CHANGE_AUTHORIZATION')"
        :initial-thread="target"
        thread-only
      />
    </template>
  </section>
</template>
