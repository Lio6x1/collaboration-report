<script setup lang="ts">
import { UserFacingError, api, explain, transportEpoch } from '../../lib/http';
import { ref, watch, onBeforeUnmount } from 'vue';
import type { ReviewRound, ReviewRequest } from '../../../../contracts/http/generated';
import ContentView from '../../lib/content/ContentView.vue';

const props = defineProps<{ notification: string }>();
const round = ref<ReviewRound | null>(null);
const document = ref('');
const loading = ref(false);
const error = ref('');
let active = true;
let generation = 0;
async function load() {
  const current = ++generation;
  round.value = null;
  document.value = '';
  error.value = '';
  loading.value = true;
  try {
    // 唯一来源是通知保存的原轮次，不读取request.current_round作为替代内容。
    const fixed = await api<ReviewRound>(
      '/v1/notifications/' + encodeURIComponent(props.notification) + '/review-round',
    );
    if (!active || current !== generation) return;
    const request = await api<ReviewRequest>(
      '/v1/review-requests/' + encodeURIComponent(fixed.request_id),
    );
    if (!active || current !== generation) return;
    if (request.id !== fixed.request_id || fixed.content.id !== fixed.content_version_id)
      throw new UserFacingError('原审核通知的固定目标不一致，请重新打开');
    round.value = fixed;
    document.value = request.document_id;
  } catch (reason) {
    if (active && current === generation) error.value = explain(reason);
  } finally {
    if (current === generation) loading.value = false;
  }
}
watch(
  () => [props.notification, transportEpoch.value],
  () => void load(),
  { immediate: true },
);
onBeforeUnmount(() => {
  active = false;
  generation++;
});
</script>

<template>
  <section aria-label="通知原始审核轮次">
    <p v-if="loading" role="status">正在核验并读取原审核轮次…</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <button v-if="error" type="button" :disabled="loading" @click="load">重新打开原审核轮次</button>
    <template v-if="round && document">
      <h2>通知对应的原审核轮次 · 第 {{ round.round_no }} 轮</h2>
      <p class="hint">
        原审核请求 {{ round.request_id }} · 原轮次 {{ round.id }} · 固定正文版本
        {{ round.content_version_id }}
      </p>
      <p class="hint">下方展示通知产生时的固定正文、目录及样式；之后的重新提交不会替换此轮次。</p>
      <p class="hint">固定目录 {{ round.structure.id }} · 固定样式 {{ round.style.id }}</p>
      <nav aria-label="原审核固定目录">
        <ol>
          <li v-for="entry in round.structure.entries" :key="entry.node_id">{{ entry.title }}</li>
        </ol>
      </nav>
      <ContentView
        class="paper"
        :content="round.content"
        :style="round.style"
        :resource-context="{ document, kind: 'REVIEW', id: round.id }"
        label="通知原审核固定正文"
      />
    </template>
  </section>
</template>
