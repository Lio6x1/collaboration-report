<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue';
import PublicationContent from './PublicationContent.vue';
import ExportPanel from '../exports/ExportPanel.vue';
import { usePublicReading } from './usePublicReading';
import { chapterAnchor, chapterFromHash, firstChapterUnit } from './anchors';
const props = defineProps<{ document: string }>();
const { current, loading, error, refresh } = usePublicReading(() => props.document);
const chapterNotice = ref('');
let locationRequest = 0;
async function locateChapter() {
  const original = ++locationRequest;
  chapterNotice.value = '';
  const chapter = chapterFromHash(location.hash);
  if (!chapter || !current.value) return;
  // 必须等当前正式版正文挂载后定位；旧章节不存在则保留整篇，不用旧release补读。
  if (!firstChapterUnit(current.value.structure.entries, chapter)) {
    chapterNotice.value = '原章节不在当前正式版中，已打开当前整篇内容。';
    return;
  }
  await nextTick();
  if (original !== locationRequest || !current.value) return;
  document.getElementById(chapterAnchor(chapter))?.scrollIntoView({ block: 'start' });
}
watch(current, locateChapter, { flush: 'post' });
onMounted(() => window.addEventListener('hashchange', locateChapter));
onUnmounted(() => {
  locationRequest++;
  window.removeEventListener('hashchange', locateChapter);
});
</script>
<template>
  <section class="panel public-reader" aria-label="当前正式版阅读">
    <div class="section-title">
      <div>
        <span class="eyebrow">正式阅读</span>
        <h2>当前正式版</h2>
      </div>
      <button :disabled="loading" @click="refresh">刷新正式版</button>
    </div>
    <p class="hint">这里展示当前发布的整篇内容。正式版更新后，请刷新阅读。</p>
    <p v-if="loading" role="status">正在核验当前正式版…</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="chapterNotice" class="hint" role="status">{{ chapterNotice }}</p>
    <PublicationContent v-if="current" :publication="current" label="正式正文" />
    <ExportPanel
      v-if="current"
      :document="props.document"
      target="PUBLIC"
      :reference="current.release_id"
    />
    <a href="/search">返回正式版搜索</a>
    <a v-if="error" href="/">返回工作台或登录</a>
  </section>
</template>
