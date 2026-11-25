<script setup lang="ts">
import { computed, ref } from 'vue';
import type { MainView, StructureEntry } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import AppDialog from '../../components/AppDialog.vue';
import AppPopover from '../../components/AppPopover.vue';
import { actionsBlocked } from '../../lib/http';
import { useStructure } from './useStructure';
import { chapterGroups, unitTitle } from './reading';
import StructurePreviewPanel from './StructurePreviewPanel.vue';
const props = defineProps<{
  view: MainView;
  reload: () => Promise<void>;
  prepare: () => Promise<unknown>;
}>();
const dialogOpen = ref(false);
const {
  action,
  chapters,
  newTitle,
  addAfter,
  chapter,
  title,
  moveAfter,
  addChapter,
  changeChapter,
} = useStructure(
  () => props.view,
  props.reload,
  props.prepare,
  () => (dialogOpen.value = false),
);
const { busy, error, notice, pending, retry } = action;
const groups = computed(() => chapterGroups(props.view.structure.entries));
const command = ref<'ADD' | 'RENAME' | 'MOVE' | 'RANGE'>('ADD');
const rangeType = ref<'SPLIT_LEVEL2' | 'JOIN_CHAPTER' | 'REMOVE'>('SPLIT_LEVEL2');
const commandTitle = computed(
  () =>
    ({
      ADD: '新增章节',
      RENAME: '重命名章节',
      MOVE: '移动章节',
      RANGE: { SPLIT_LEVEL2: '按二级标题拆分', JOIN_CHAPTER: '合章', REMOVE: '删除章节' }[
        rangeType.value
      ],
    })[command.value],
);
function openCommand(
  entry: StructureEntry | null,
  next: typeof command.value,
  close?: () => void,
  range?: typeof rangeType.value,
) {
  close?.();
  command.value = next;
  if (entry) {
    chapter.value = entry.chapter_id;
    title.value = entry.title;
  }
  if (next === 'ADD') {
    newTitle.value = '';
    addAfter.value = entry?.chapter_id ?? '';
  }
  if (next === 'MOVE') moveAfter.value = '';
  if (range) rangeType.value = range;
  dialogOpen.value = true;
}
async function save() {
  if (command.value === 'ADD') await addChapter();
  else if (command.value === 'RENAME' || command.value === 'MOVE')
    await changeChapter(command.value);
  if (!error.value && !pending.value) dialogOpen.value = false;
}
</script>

<template>
  <section class="management" role="region" aria-label="目录管理">
    <div class="section-title">
      <div>
        <h3>目录管理</h3>
        <p class="hint">在对应章节右侧选择操作，小节归属于原章节。</p>
      </div>
      <button :disabled="actionsBlocked" @click="openCommand(null, 'ADD')">新增章节</button>
    </div>
    <p v-if="view.document.main_locked" class="hint">
      主线已锁定；解锁后可重命名、移动或调整章节范围。
    </p>
    <p v-if="!groups.length" class="empty">还没有章节，新增一章开始写作。</p>
    <ol class="chapter-management-list">
      <li v-for="group in groups" :key="group.chapter.node_id" class="chapter-management-row">
        <div class="chapter-management-heading">
          <div>
            <strong>{{ group.chapter.title }}</strong>
            <p class="hint">
              {{
                group.chapter.unit_id
                  ? '整章编辑'
                  : '按小节编辑 · ' + group.units.length + ' 个内容区'
              }}
            </p>
          </div>
          <AppPopover
            :label="'章节操作：' + group.chapter.title"
            button-text="⋯"
            :disabled="actionsBlocked"
            ><template #default="{ close }"
              ><div class="chapter-menu">
                <button
                  :disabled="view.document.main_locked"
                  @click="openCommand(group.chapter, 'RENAME', close)"
                >
                  重命名
                </button>
                <button
                  :disabled="view.document.main_locked"
                  @click="openCommand(group.chapter, 'MOVE', close)"
                >
                  移动章节
                </button>
                <button @click="openCommand(group.chapter, 'ADD', close)">在此章后新增</button>
                <button
                  v-if="group.chapter.unit_id"
                  :disabled="view.document.main_locked"
                  @click="openCommand(group.chapter, 'RANGE', close, 'SPLIT_LEVEL2')"
                >
                  按二级标题拆分
                </button>
                <button
                  v-else
                  :disabled="view.document.main_locked"
                  @click="openCommand(group.chapter, 'RANGE', close, 'JOIN_CHAPTER')"
                >
                  合章
                </button>
                <button
                  class="danger"
                  :disabled="view.document.main_locked"
                  @click="openCommand(group.chapter, 'RANGE', close, 'REMOVE')"
                >
                  删除章节
                </button>
              </div></template
            ></AppPopover
          >
        </div>
        <ul v-if="!group.chapter.unit_id" class="chapter-children">
          <li v-for="unit in group.units" :key="unit.node_id">{{ unitTitle(unit) }}</li>
        </ul>
      </li>
    </ol>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
    <AppDialog v-model="dialogOpen" :title="commandTitle" :blocked="actionsBlocked">
      <p v-if="command !== 'ADD'" class="hint">
        操作章节：{{ chapters.find((e) => e.chapter_id === chapter)?.title }}
      </p>
      <form v-if="command !== 'RANGE'" @submit.prevent="save">
        <fieldset :disabled="actionsBlocked">
          <template v-if="command === 'ADD'"
            ><label>新增章节标题<input v-model="newTitle" required maxlength="240" /></label
            ><label
              >新增章节位置<select v-model="addAfter">
                <option value="">目录末尾</option>
                <option v-for="entry in chapters" :key="entry.chapter_id" :value="entry.chapter_id">
                  {{ entry.title }}之后
                </option>
              </select></label
            ></template
          >
          <label v-else-if="command === 'RENAME'"
            >章节标题<input v-model="title" required maxlength="240"
          /></label>
          <label v-else
            >移动到章节之后<select v-model="moveAfter" aria-label="移动到章节之后">
              <option value="">目录开头</option>
              <option
                v-for="entry in chapters.filter((e) => e.chapter_id !== chapter)"
                :key="entry.chapter_id"
                :value="entry.chapter_id"
              >
                {{ entry.title }}
              </option>
            </select></label
          >
          <div class="button-row">
            <button class="quiet" type="button" @click="dialogOpen = false">取消</button
            ><button
              :disabled="
                command === 'RENAME' ? !title.trim() : command === 'ADD' ? !newTitle.trim() : false
              "
            >
              {{
                command === 'ADD' ? '新增章节' : command === 'RENAME' ? '更新章节标题' : '移动章节'
              }}
            </button>
          </div>
        </fieldset>
      </form>
      <StructurePreviewPanel
        v-if="command === 'RANGE' && dialogOpen"
        :key="chapter + ':' + rangeType"
        :view="view"
        :reload="reload"
        :prepare="prepare"
        :chapter-id="chapter"
        :range-type="rangeType"
        @completed="dialogOpen = false"
      />
      <ActionStatus
        v-if="command !== 'RANGE'"
        :busy="busy"
        :error="error"
        :notice="notice"
        :pending="pending"
        :retry="retry"
      />
    </AppDialog>
  </section>
</template>

<style scoped>
.chapter-management-list {
  padding: 0;
  margin: 12px 0;
  list-style: none;
}
.chapter-management-row {
  padding: 22px 0;
  border-bottom: 1px solid var(--line);
}
.chapter-management-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
}
.chapter-management-heading p {
  margin: 6px 0 0;
}
.chapter-children {
  margin: 12px 0 0 14px;
  padding-left: 22px;
  border-left: 2px solid var(--line);
  list-style: none;
  color: var(--muted);
}
.chapter-children li {
  padding: 7px 0;
}
.chapter-menu {
  display: grid;
  gap: 4px;
}
.chapter-menu button {
  border-color: transparent;
  text-align: left;
}
.chapter-management-heading :deep(.popover-panel) {
  left: auto;
  right: 0;
  width: min(260px, calc(100vw - 64px));
}
@media (max-width: 560px) {
  .chapter-management-heading {
    align-items: flex-start;
  }
  .chapter-management-heading :deep(.app-popover > button) {
    font-size: 0;
  }
  .chapter-management-heading :deep(.app-popover > button::before) {
    content: '⋯';
    font-size: 24px;
  }
}
</style>
