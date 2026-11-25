<script setup lang="ts">
import { ref, computed, watch, nextTick, onUnmounted } from 'vue';
import type { FixedContent } from '../../../../contracts/http/generated';
import { UserFacingError, explain } from '../../lib/http';
import { readFixedContent } from '../draft/api';
import { useUnitEditor } from './useUnitEditor';
import EditorToolbar from './views/EditorToolbar.vue';
import MarkdownImport from './views/MarkdownImport.vue';
import DraftRecovery from './views/DraftRecovery.vue';
import SubmissionButton from '../review/SubmissionButton.vue';
import PullMainControl from '../draft/PullMainControl.vue';
import DiscardDraftControl from '../draft/DiscardDraftControl.vue';
import ResourceUpload from '../resources/ResourceUpload.vue';
import AppPopover from '../../components/AppPopover.vue';
import BranchDiscussion from '../comments/BranchDiscussion.vue';
const props = defineProps<{
  actor: string;
  document: string;
  space: string;
  unit: string;
  structure: string;
  kind?: 'BRANCH' | 'MAIN';
  reloadDraft: () => Promise<void>;
  managing: boolean;
  commenting: boolean;
  mainBody?: FixedContent;
  autoPullMain?: boolean;
}>();
const emit = defineEmits<{ close: []; flushed: [unknown]; submitted: [request: string] }>();
const resourceUpload = ref<InstanceType<typeof ResourceUpload> | null>(null);
const pullControl = ref<InstanceType<typeof PullMainControl> | null>(null);
const checkingMain = ref(Boolean(props.autoPullMain)),
  syncMessage = ref(''),
  imageTools = ref(false);
const openingBlocked = computed(() => checkingMain.value || Boolean(pullControl.value?.pending));
let active = true,
  checkedMain = false;
onUnmounted(() => {
  active = false;
});
const {
  host,
  toolbar,
  formula,
  markdown,
  loss,
  busy,
  failure,
  status,
  discussionBoundary,
  format,
  heading,
  paragraph,
  codeBlock,
  paragraphAlignment,
  firstLineIndent,
  table,
  tableAction,
  insertFormula,
  insertImage,
  insertMarkdown,
  flush,
  prepareSubmission,
  preparePull,
  prepareDraft,
  prepareSave,
  oldText,
  recover,
  selectRetained,
  keepCurrent,
  refreshEditability,
} = useUnitEditor(
  props,
  (receipt) => emit('flushed', receipt),
  (file, insert) => {
    imageTools.value = true;
    return resourceUpload.value?.paste(file, insert) ?? false;
  },
  () => openingBlocked.value,
);
watch(openingBlocked, refreshEditability);
watch(
  [discussionBoundary, () => status.value.stale, () => status.value.retained.length],
  async ([boundary]) => {
    if (!props.autoPullMain || checkedMain) return;
    if (status.value.stale || status.value.retained.length) {
      checkedMain = true;
      checkingMain.value = false;
      syncMessage.value = '发现本机保留输入，请先处理恢复内容，再手动拉取主线。';
      return;
    }
    if (!boundary || !props.mainBody) return;
    checkedMain = true;
    syncMessage.value = '正在检查主线是否有更新…';
    try {
      const base = await readFixedContent(boundary.version);
      if (!active) return;
      if (status.value.stale || status.value.retained.length)
        throw new UserFacingError('发现本机保留输入，请先恢复，再手动拉取主线。');
      if (base.body_hash === props.mainBody.body_hash) {
        syncMessage.value = '打开时已核对：支线基线与当前主线一致。';
        return;
      }
      // 复用可靠保存→三方合并→新代次安装，不能把MAIN正文直接填进支线并覆盖他人修改。
      syncMessage.value = '主线已有更新，正在合入共同支线并保留支线修改…';
      await nextTick();
      if (!pullControl.value) throw new UserFacingError('主线同步入口尚未就绪，请手动拉取。');
      await pullControl.value.pull();
      if (pullControl.value?.error) throw new UserFacingError(pullControl.value.error);
      if (active) syncMessage.value = '已合入打开时的主线更新，兼容的支线修改已保留。';
    } catch (error) {
      if (active) syncMessage.value = '主线尚未自动合入：' + explain(error);
    } finally {
      if (active) checkingMain.value = false;
    }
  },
  { immediate: true },
);
defineExpose({ confirmSave: prepareSave });
</script>
<template>
  <section
    class="unit-editor"
    :aria-label="kind === 'MAIN' ? '主线编辑区' : '共同支线编辑区'"
    :aria-busy="openingBlocked || status.loading"
  >
    <div class="section-title">
      <h3>{{ kind === 'MAIN' ? '主线' : '共同支线' }}</h3>
      <button :disabled="busy || openingBlocked || status.structurePaused" @click="emit('close')">
        保存并返回阅读
      </button>
    </div>
    <p class="save-state" aria-live="polite">{{ status.notice }}</p>
    <p v-if="syncMessage" class="sync-state" role="status">{{ syncMessage }}</p>
    <p v-if="status.problem || failure" class="error">{{ failure || status.problem }}</p>
    <DraftRecovery
      :retained="status.retained"
      :stale="!!status.stale"
      :text="oldText()"
      @select="selectRetained"
      @recover="recover"
      @keep="keepCurrent"
    />
    <EditorToolbar
      v-model:formula="formula"
      :state="toolbar"
      :disabled="
        busy || openingBlocked || status.structurePaused || status.loading || !!status.stale
      "
      @format="format"
      @heading="heading"
      @paragraph="paragraph"
      @code-block="codeBlock"
      @paragraph-alignment="paragraphAlignment"
      @first-line-indent="firstLineIndent"
      @table="table"
      @table-action="tableAction"
      @insert-formula="insertFormula"
    >
      <template #inserts>
        <MarkdownImport v-model="markdown" @import="insertMarkdown" />
        <AppPopover v-model="imageTools" label="图片与上传">
          <ResourceUpload
            ref="resourceUpload"
            :document="document"
            :space="space"
            :disabled="
              busy || openingBlocked || status.structurePaused || status.loading || !!status.stale
            "
            @ready="insertImage"
          />
        </AppPopover>
      </template>
    </EditorToolbar>
    <p v-if="loss" class="hint">{{ loss }}</p>
    <div ref="host" class="editor-paper" :aria-busy="status.loading"></div>
    <div class="button-row spaced">
      <button
        :disabled="busy || openingBlocked || status.structurePaused || status.loading"
        @click="flush"
      >
        {{ busy ? '正在保存…' : '保存当前修改' }}
      </button>
      <p v-if="status.receipt" class="success">当前修改已保存</p>
    </div>
    <SubmissionButton
      v-if="kind !== 'MAIN'"
      :document="document"
      :unit="unit"
      :disabled="
        busy || openingBlocked || status.structurePaused || status.loading || !!status.stale
      "
      :prepare="prepareSubmission"
      @submitted="emit('submitted', $event)"
    />
    <PullMainControl
      ref="pullControl"
      v-if="kind !== 'MAIN'"
      :document="document"
      :unit="unit"
      :disabled="
        busy || openingBlocked || status.structurePaused || status.loading || !!status.stale
      "
      :prepare="preparePull"
      :reload="reloadDraft"
    />
    <p class="hint">修改会自动保存。“保存当前修改”会等待服务端确认；保存后可以继续编辑。</p>
    <BranchDiscussion
      v-if="kind !== 'MAIN' && commenting && discussionBoundary"
      :key="
        discussionBoundary.branch +
        ':' +
        discussionBoundary.lineage +
        ':' +
        discussionBoundary.version
      "
      :document="document"
      :unit="unit"
      :space="space"
      :actor="actor"
      :managing="managing"
      :commenting="commenting"
      :boundary="discussionBoundary"
    />
    <p v-else-if="kind !== 'MAIN' && commenting" class="hint">
      共同草稿讨论将在当前工作区连接确认后开放；旧工作区输入不作为讨论正文。
    </p>
    <DiscardDraftControl
      v-if="kind !== 'MAIN' && managing"
      :document="document"
      :unit="unit"
      :disabled="
        busy || openingBlocked || status.structurePaused || status.loading || !!status.stale
      "
      :prepare="prepareDraft"
      :reload="reloadDraft"
    />
  </section>
</template>
