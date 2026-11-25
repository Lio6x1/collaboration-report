<script setup lang="ts">
import { ref, watch } from 'vue';
import type { Anchor } from '../../../../contracts/http/generated';
import ContentView from '../../lib/content/ContentView.vue';
import CommentsPanel from './CommentsPanel.vue';
import { useBranchDiscussion } from './useBranchDiscussion';

const props = defineProps<{
  document: string;
  unit: string;
  space: string;
  actor: string;
  managing: boolean;
  commenting: boolean;
  boundary: { branch: string; version: string; manifest: string; lineage: string };
}>();
const expanded = ref(false);
const selection = ref<Anchor | null>(null);
const focusedAnchor = ref<Anchor | null>(null);
const { content, round, rounds, selected, loading, error, load } = useBranchDiscussion(
  props,
  () => expanded.value,
);
function select(anchor: Anchor | null) {
  selection.value = anchor;
  focusedAnchor.value = anchor;
}
function focus(anchor: Anchor) {
  // 历史线程保留原版本，不能把历史位置绘制到另一个版本；原文可在讨论面板独立展开。
  focusedAnchor.value = anchor.content_version_id === content.value?.id ? anchor : null;
}
watch(
  () => content.value?.id,
  () => select(null),
);
</script>

<template>
  <section class="branch-discussion" aria-label="共同草稿固定版本讨论">
    <button type="button" :aria-expanded="expanded" @click="expanded = !expanded">
      {{ expanded ? '收起共同草稿讨论' : '共同草稿讨论' }}
    </button>
    <div v-if="expanded">
      <p class="hint">
        上方草稿继续实时编辑。此处讨论绑定已固定的支线基线或送审版本，当前实时输入不会自动进入讨论正文。
      </p>
      <label>
        讨论依据
        <select v-model="selected">
          <option value="baseline">当前支线基线</option>
          <option v-for="entry in rounds" :key="entry.id" :value="entry.id">
            送审第 {{ entry.round_no }} 轮 · {{ entry.id }}
          </option>
        </select>
      </label>
      <button type="button" :disabled="loading" @click="load">刷新可讨论版本</button>
      <p v-if="loading" role="status">正在读取固定正文并逐页核验送审历史…</p>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <template v-if="content">
        <p class="hint">支线：{{ boundary.branch }} · 固定正文：{{ content.id }}</p>
        <p v-if="selected === 'baseline'" class="hint">基线清单：{{ boundary.manifest }}</p>
        <p class="hint">
          可选择正文文字创建选区讨论；不选择文字时创建整份固定正文讨论。下方也包含本支线其他版本的讨论。
        </p>
        <ContentView
          :content="content"
          :style="round?.style"
          :resource-context="{
            document,
            kind: round ? 'REVIEW' : 'BRANCH',
            id: round?.id ?? boundary.branch,
          }"
          :selectable="commenting"
          :anchor="focusedAnchor"
          label="共同草稿讨论固定正文"
          @selection="select"
        />
        <CommentsPanel
          :document="document"
          context-kind="BRANCH"
          :context-id="boundary.branch"
          :version="content.id"
          :actor="actor"
          :space="space"
          :can-comment="commenting"
          :can-manage="managing"
          :browse-branch-history="true"
          :anchor="selection"
          @open-anchor="focus"
          @clear-anchor="select(null)"
        />
      </template>
    </div>
  </section>
</template>
