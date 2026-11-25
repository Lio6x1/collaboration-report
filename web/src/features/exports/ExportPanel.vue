<script setup lang="ts">
import { ApiError, explain, sessionContentURL } from '../../lib/http';
import ActionStatus from '../../components/ActionStatus.vue';
import { useExport } from './useExport';
import type { FixedContent, MainView } from '../../../../contracts/http/generated';
const props = defineProps<{
  document: string;
  target: 'MAIN' | 'PUBLIC';
  reference: string;
  mainView?: MainView;
  mainBodies?: Record<string, FixedContent>;
}>();
const {
  action,
  job,
  format,
  readingError,
  refreshing,
  original,
  unfinished,
  actionsBlocked,
  create,
  refresh,
} = useExport(() => props);
const { busy, error, notice, pending, retry } = action;
const states: Record<string, string> = {
  QUEUED: '等待转换',
  RUNNING: '正在转换',
  SUCCEEDED: '转换完成',
  REJECTED: '转换未完成',
  CANCELLED: '任务已取消',
};
</script>
<template>
  <section class="panel export-panel" aria-label="导出固定版本">
    <div class="section-title">
      <h3>导出{{ target === 'MAIN' ? '主线稿' : '当前正式版' }}</h3>
    </div>
    <p class="hint">
      {{
        target === 'MAIN'
          ? '导出前自动重新核验阅读快照；正文未变化时直接导出，文件注明未正式发布。'
          : '按当前正式版导出，后续下载仍需满足当时的阅读权限。'
      }}
      Word和PDF采用A4版式；Markdown包包含离线图片。
    </p>
    <div class="toolbar">
      <label
        >文件格式
        <select v-model="format" :disabled="busy || unfinished">
          <option value="DOCX">Word 文档</option>
          <option value="PDF">PDF</option>
          <option value="MARKDOWN">Markdown 资源包</option>
        </select></label
      >
      <button :disabled="actionsBlocked || unfinished || (!!original && !job)" @click="create()">
        创建导出
      </button>
      <button v-if="original && !job && !pending" :disabled="actionsBlocked" @click="create(true)">
        恢复原导出请求
      </button>
    </div>
    <ActionStatus :busy="busy" :error="error" :notice="notice" :pending="pending" :retry="retry" />
    <p v-if="readingError" class="error" role="alert">{{ readingError }}</p>
    <div v-if="job" class="export-job" aria-live="polite">
      <p>{{ states[job.state] ?? '正在确认导出结果' }}</p>
      <p v-if="job.last_error" class="error">
        {{
          explain(
            new ApiError(job.last_error.code, 0),
            '文件生成失败，请稍后重新导出；若仍失败，请联系管理员。',
          )
        }}
      </p>
      <button :disabled="refreshing || busy" @click="refresh">刷新任务状态</button>
      <a
        v-if="job.state === 'SUCCEEDED'"
        :href="sessionContentURL(`/v1/export-jobs/${job.id}/download`)"
        download
        >下载导出文件</a
      >
    </div>
  </section>
</template>
<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
.toolbar label {
  display: flex;
  align-items: center;
  gap: 8px;
}
.export-job {
  margin-top: 16px;
}
.export-job a {
  margin-left: 12px;
}
</style>
