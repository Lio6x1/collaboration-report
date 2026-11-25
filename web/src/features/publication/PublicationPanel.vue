<script setup lang="ts">
import type { Document, StructureRevision } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import { actionsBlocked } from '../../lib/http';
import { publicDocumentPath } from '../public-reading/api';
import ReleaseHistory from './ReleaseHistory.vue';
import PublicationConfirmation from './PublicationConfirmation.vue';
import { usePublication } from './usePublication';
const props = defineProps<{
  document: Document;
  structure: StructureRevision;
  managing: boolean;
  reload: () => Promise<void>;
  prepare: () => Promise<unknown>;
}>();
const {
  action,
  releases,
  pagination,
  fixed,
  fixedRelease,
  choice,
  blocked,
  historyLoading,
  previewLoading,
  readingError,
  previewError,
  choose,
  confirm,
  refresh,
  loadPage,
  preview,
  closePreview,
} = usePublication(() => props.document, props.reload, props.prepare);
const { busy, error, notice, pending, retry } = action;
</script>
<template>
  <section class="management publication-panel" role="region" aria-label="正式发布">
    <div class="section-title">
      <div>
        <span class="eyebrow">主线与正式版独立</span>
        <h3>
          {{ document.publication_state === 'PUBLISHED' ? '已发布正式版' : '尚未发布正式版' }}
        </h3>
      </div>
      <button v-if="managing" class="primary" :disabled="actionsBlocked" @click="choose('CREATE')">
        发布可靠主线
      </button>
    </div>
    <p class="hint">发布冻结整篇主线；之后仍可继续编辑，旧正式版保持自己的正文、标题与样式。</p>
    <div v-if="document.publication_state === 'PUBLISHED'" class="publication-links">
      <a :href="publicDocumentPath(document.id)" target="_blank" rel="noopener">打开当前正式版</a>
      <button
        v-if="managing"
        class="danger"
        :disabled="actionsBlocked"
        @click="choose('UNPUBLISH')"
      >
        撤下当前正式版
      </button>
    </div>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
    <ReleaseHistory
      :releases="releases"
      :current="document.current_release_id"
      :managing="managing"
      :page="pagination.index + 1"
      :has-next="!!pagination.next"
      :busy="blocked"
      :loading="historyLoading"
      :reading-error="readingError"
      :preview-loading="previewLoading"
      :preview-error="previewError"
      :fixed="fixed"
      :fixed-release="fixedRelease"
      @refresh="refresh()"
      @change="loadPage"
      @preview="preview"
      @switch="choose('SWITCH', $event)"
      @close="closePreview"
    />
    <PublicationConfirmation
      v-if="choice"
      :choice="choice"
      :structure="structure"
      :error="error"
      @confirm="confirm"
      @close="choice = null"
    />
  </section>
</template>
<style scoped>
.publication-links {
  display: flex;
  align-items: center;
  gap: 18px;
  flex-wrap: wrap;
}
.publication-links a {
  color: #285d50;
  text-underline-offset: 4px;
}
.publication-panel > .section-title {
  align-items: center;
}
.publication-panel h3 {
  margin-bottom: 0;
}
</style>
