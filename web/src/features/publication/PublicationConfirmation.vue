<script setup lang="ts">
import { computed } from 'vue';
import type { StructureRevision } from '../../../../contracts/http/generated';
import type { PublicationChoice } from './usePublication';
import { releaseLabel } from './usePublication';
import { actionsBlocked } from '../../lib/http';
import { chapterGroups, unitTitle } from '../structure/reading';
const props = defineProps<{
  choice: PublicationChoice;
  structure: StructureRevision;
  error: string;
}>();
defineEmits<{ confirm: []; close: [] }>();
const chapters = computed(() => chapterGroups(props.structure.entries));
const wording = computed(() =>
  props.choice.kind === 'CREATE'
    ? {
        title: '确认发布主线',
        button: '确认发布',
        note: '发布当前整篇可靠主线，形成新的固定正式版。',
      }
    : props.choice.kind === 'SWITCH'
      ? {
          title: '确认切换正式版',
          button: '确认切换',
          note: '普通读者将看到选中的历史正式版，正在编写的主线继续保留。',
        }
      : {
          title: '确认撤下正式版',
          button: '确认撤下',
          note: '正式阅读入口将关闭，内部主线和版本历史继续保留。',
        },
);
</script>
<template>
  <div class="publication-overlay">
    <section
      class="panel publication-confirm"
      role="dialog"
      aria-modal="true"
      :aria-label="wording.title"
    >
      <span class="eyebrow">{{ choice.document.title }}</span>
      <h2>{{ wording.title }}</h2>
      <p>{{ wording.note }}</p>
      <template v-if="choice.kind === 'CREATE'">
        <p class="publication-reminder">
          未合入的共同支线不会进入正式版。请先检查这些支线，或确认只发布当前主线。
        </p>
        <h3>本次整篇目录</h3>
        <ol class="publication-chapters">
          <li v-for="group in chapters" :key="group.chapter.node_id">
            {{ group.chapter.title }}
            <ol v-if="group.chapter.split_level2">
              <li v-for="entry in group.units" :key="entry.node_id">{{ unitTitle(entry) }}</li>
            </ol>
          </li>
        </ol>
      </template>
      <p v-if="choice.release">
        选择的版本：{{ releaseLabel(choice.release.created_at) }}（北京时间）
      </p>
      <details v-if="choice.release" class="hint">
        <summary>版本详情</summary>
        <p>
          版本标识：<code>{{ choice.release.id }}</code>
        </p>
      </details>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <div class="publication-buttons">
        <button :disabled="actionsBlocked" @click="$emit('close')">取消</button>
        <button
          :disabled="actionsBlocked"
          :class="choice.kind === 'UNPUBLISH' ? 'danger' : 'primary'"
          @click="$emit('confirm')"
        >
          {{ wording.button }}
        </button>
      </div>
    </section>
  </div>
</template>
<style scoped>
.publication-overlay {
  position: fixed;
  inset: 0;
  z-index: 100;
  background: #1c332d66;
  display: grid;
  place-items: center;
  padding: 24px;
}
.publication-confirm {
  width: min(560px, 100%);
  max-height: 85vh;
  overflow: auto;
  box-shadow: 0 20px 70px #142b2940;
}
.publication-confirm h2 {
  margin-top: 10px;
}
.publication-reminder {
  padding: 14px 16px;
  border-left: 3px solid #b9954d;
  background: #fbf6e9;
  color: #78603e;
}
.publication-chapters {
  max-height: 180px;
  overflow: auto;
  color: #52645b;
  padding-left: 22px;
}
.publication-buttons {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  margin-top: 24px;
}
</style>
