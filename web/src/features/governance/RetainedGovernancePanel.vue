<script setup lang="ts">
import { ref, watch } from 'vue';
import ActionStatus from '../../components/ActionStatus.vue';
import { transportEpoch } from '../../lib/http';
import { useRetainedGovernance } from './useRetainedGovernance';

const { action, documentID, document, loading, readingError, blocked, load, govern } =
  useRetainedGovernance();
const deleting = ref(false);
const confirmed = ref(false);
watch([document, transportEpoch], () => {
  deleting.value = false;
  confirmed.value = false;
});
</script>

<template>
  <section class="panel" aria-label="公开留存治理">
    <div class="section-title"><h2>公开留存治理</h2></div>
    <p class="hint">
      此入口供已获平台治理资格的管理员处理注销后孤立的公开文章。治理资格由部署运维授予，组织管理员角色不包含此权限。
    </p>
    <form class="toolbar" @submit.prevent="load">
      <label
        >留存文档编号 <input v-model="documentID" inputmode="numeric" :disabled="blocked"
      /></label>
      <button type="submit" :disabled="blocked">查询留存文章</button>
    </form>
    <ActionStatus
      v-if="!action.pending.value"
      :busy="action.busy.value"
      :error="action.error.value"
      :notice="action.notice.value"
      :pending="action.pending.value"
      :retry="action.retry.value"
    />
    <p v-if="loading" role="status">正在核验资格与留存状态…</p>
    <p v-if="readingError" role="alert">{{ readingError }}</p>
    <article v-if="document" class="retained-card">
      <h3>{{ document.title }}</h3>
      <p>
        文档 {{ document.id }} ·
        {{ document.publication_state === 'PUBLISHED' ? '仍在正式发布' : '已撤下正式发布' }}
      </p>
      <p class="hint">此处仅提供留存治理，不提供修改正文或重新开启协作的操作。</p>
      <div class="toolbar">
        <button
          v-if="document.publication_state === 'PUBLISHED'"
          type="button"
          :disabled="blocked"
          @click="govern('UNPUBLISH')"
        >
          撤下正式发布
        </button>
        <button
          type="button"
          class="danger"
          :disabled="blocked"
          @click="
            deleting = true;
            confirmed = false;
          "
        >
          永久删除留存
        </button>
      </div>
      <div v-if="deleting" class="purge-confirmation">
        <p>永久删除「{{ document.title }}」会清除正文、内容历史及可回收文件，完成后无法恢复。</p>
        <label
          ><input
            v-model="confirmed"
            type="checkbox"
            :disabled="blocked"
          />我确认永久删除这篇留存文章及其内容历史</label
        >
        <div class="toolbar">
          <button
            type="button"
            class="danger"
            :disabled="blocked || !confirmed"
            @click="govern('PURGE')"
          >
            确认永久删除留存
          </button>
          <button
            type="button"
            :disabled="blocked"
            @click="
              deleting = false;
              confirmed = false;
            "
          >
            取消
          </button>
        </div>
      </div>
    </article>
  </section>
</template>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}
.toolbar label {
  display: flex;
  gap: 8px;
  align-items: center;
}
.retained-card {
  margin-top: 20px;
}
.purge-confirmation {
  margin-top: 16px;
  padding: 16px;
  border: 1px solid var(--line);
  border-radius: 12px;
}
.purge-confirmation label {
  display: flex;
  gap: 8px;
  align-items: center;
}
.purge-confirmation .toolbar {
  margin-top: 12px;
}
</style>
