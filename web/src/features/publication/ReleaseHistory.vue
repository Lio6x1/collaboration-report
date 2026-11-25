<script setup lang="ts">
import type { PublicDocument, Release } from '../../../../contracts/http/generated';
import { actionsBlocked } from '../../lib/http';
import PaginationControls from '../../components/PaginationControls.vue';
import PublicationContent from '../public-reading/PublicationContent.vue';
import { releaseLabel } from './usePublication';
defineProps<{
  releases: Release[];
  current?: string;
  managing: boolean;
  page: number;
  hasNext: boolean;
  busy: boolean;
  loading: boolean;
  readingError: string;
  previewLoading: boolean;
  previewError: string;
  fixed: PublicDocument | null;
  fixedRelease: Release | null;
}>();
defineEmits<{ refresh: []; change: [number]; preview: [Release]; switch: [Release]; close: [] }>();
</script>
<template>
  <section class="release-history" role="region" aria-label="正式版本历史">
    <div class="section-title">
      <h3>正式版本历史</h3>
      <button :disabled="busy" @click="$emit('refresh')">刷新版本历史</button>
    </div>
    <p class="hint">按发布时间区分版本，时间均为北京时间。历史版供内部协作者核对。</p>
    <p v-if="loading" role="status">正在读取正式版本历史…</p>
    <p v-if="readingError" role="alert">{{ readingError }}</p>
    <p v-if="!loading && !readingError && !releases.length" class="empty">本页没有正式版本。</p>
    <ol class="entity-list">
      <li v-for="release in releases" :key="release.id" :data-release-id="release.id">
        <time :datetime="release.created_at">{{ releaseLabel(release.created_at) }}</time>
        <span v-if="release.id === current" class="ready">当前正式版</span>
        <span v-else class="hint">历史正式版</span>
        <div class="release-actions">
          <button :disabled="busy" @click="$emit('preview', release)">查看固定版本</button>
          <button
            v-if="managing && release.id !== current"
            :disabled="busy"
            @click="$emit('switch', release)"
          >
            设为当前正式版
          </button>
        </div>
        <details class="release-details">
          <summary>版本详情</summary>
          <p>
            版本标识：<code>{{ release.id }}</code>
          </p>
          <p>
            文档标识：<code>{{ release.document_id }}</code>
          </p>
        </details>
      </li>
    </ol>
    <PaginationControls
      label="正式版本历史"
      :page="page"
      :has-next="hasNext"
      :busy="busy"
      @change="$emit('change', $event)"
    />
    <p v-if="previewLoading" role="status">正在读取固定正式版本…</p>
    <p v-if="previewError" role="alert">{{ previewError }}</p>
    <section v-if="fixed" role="region" aria-label="历史正式版阅读" class="fixed-release">
      <div class="section-title">
        <h3>历史版本预览</h3>
        <button :disabled="actionsBlocked" @click="$emit('close')">关闭历史阅读</button>
      </div>
      <p class="hint">
        {{ releaseLabel(fixedRelease?.created_at) }}
        · 北京时间
      </p>
      <PublicationContent :publication="fixed" label="历史正式正文" />
    </section>
  </section>
</template>
<style scoped>
.release-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.release-history {
  margin-top: 28px;
  border-top: 1px solid #e0e5db;
  padding-top: 10px;
}
.release-history ol {
  padding: 0;
}
.release-details {
  font-size: 0.875em;
  color: var(--muted);
}
.release-details p {
  margin: 6px 0;
  overflow-wrap: anywhere;
}
.fixed-release {
  margin-top: 22px;
}
@media (max-width: 760px) {
  .entity-list li {
    flex-wrap: wrap;
  }
}
</style>
