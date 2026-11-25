<script setup lang="ts">
import { computed, useId, watch, ref, onUnmounted } from 'vue';
import { matchesAccount } from '../features/account/identity';
import AppPopover from './AppPopover.vue';

// 目录读取、分页及权限失效仍由业务页负责；这里只负责把有权候选明确展示并选择。
const props = defineProps<{
  modelValue: string;
  query: string;
  members: { user_id: string; display_name: string; label: string }[];
  searchLabel: string;
  optionsLabel: string;
  emptyText: string;
  moreLabel: string;
  hasMore: boolean;
  cursor?: string | null;
  disabled?: boolean;
  loading?: boolean;
}>();
const emit = defineEmits<{
  'update:modelValue': [id: string];
  'update:query': [query: string];
  more: [];
}>();
const radioName = useId();
const expanded = ref(false);
const chooseLabel = computed(
  () => (selected.value ? '更换' : '选择') + props.optionsLabel.replace(/^选择/, ''),
);
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let requested = '';
watch(
  [expanded, () => props.query, () => props.cursor, () => props.loading, () => props.disabled],
  () => {
    clearTimeout(searchTimer);
    if (!expanded.value || !props.query.trim() || !props.hasMore || props.loading || props.disabled)
      return;
    const key = props.query + ':' + props.cursor;
    if (key === requested) return;
    // 逐页搜索全部有权候选；关闭浮层即停止，不把当前页无匹配当成全目录无结果。
    searchTimer = setTimeout(() => {
      requested = key;
      emit('more');
    }, 250);
  },
);
onUnmounted(() => clearTimeout(searchTimer));
const visible = computed(() =>
  props.members.filter((member) => matchesAccount(member, props.query)),
);
const selected = computed(() =>
  props.members.find((member) => member.user_id === props.modelValue),
);
watch(
  () => props.query,
  () => {
    // 改查找条件后必须重新确认对象，不能带着已隐藏的旧选择提交授权。
    emit('update:modelValue', '');
  },
);
watch(
  () => props.members,
  () => {
    if (props.modelValue && !selected.value) emit('update:modelValue', '');
  },
);
function select(id: string) {
  if (!props.disabled && !props.loading && visible.value.some((member) => member.user_id === id)) {
    emit('update:modelValue', id);
    expanded.value = false;
  }
}
</script>

<template>
  <div class="member-directory" :aria-busy="loading">
    <div class="member-choice">
      <span v-if="selected" class="member-selected" role="status">
        <span class="member-avatar" aria-hidden="true">{{
          selected.display_name.slice(0, 1)
        }}</span>
        已选：{{ selected.label }}
      </span>
      <span v-else class="hint">尚未选择成员</span>
      <AppPopover v-model="expanded" :label="chooseLabel" :disabled="disabled">
        <label>
          {{ searchLabel }}
          <input
            :value="query"
            type="search"
            placeholder="输入姓名或账号编号"
            autocomplete="off"
            :disabled="disabled"
            @input="emit('update:query', ($event.target as HTMLInputElement).value)"
          />
        </label>
        <p class="directory-summary hint" role="status" aria-live="polite">
          <template v-if="loading">正在加载成员…</template>
          <template v-else>
            {{ query.trim() ? '匹配' : '可选' }} {{ visible.length }} 位成员
            <span v-if="hasMore">
              · {{ query.trim() ? '正在继续查找后续成员' : '还有更多成员' }}</span
            >
          </template>
        </p>
        <div v-if="visible.length" class="member-options" role="group" :aria-label="optionsLabel">
          <label
            v-for="member in visible"
            :key="member.user_id"
            class="member-option"
            :class="{ chosen: modelValue === member.user_id }"
          >
            <input
              type="radio"
              :name="radioName"
              :value="member.user_id"
              :aria-label="member.label"
              :checked="modelValue === member.user_id"
              :disabled="disabled || loading"
              @change="select(member.user_id)"
            />
            <span class="member-avatar" aria-hidden="true">{{
              member.display_name.slice(0, 1)
            }}</span>
            <span class="member-name">{{ member.label }}</span>
            <span v-if="modelValue === member.user_id" class="member-selection">已选择</span>
          </label>
        </div>
        <p v-else-if="!loading" class="empty directory-empty" role="status">
          {{
            query.trim()
              ? hasMore
                ? '正在查找其他成员…'
                : '没有匹配的成员，请核对姓名或账号编号。'
              : emptyText
          }}
        </p>
        <button v-if="hasMore" type="button" :disabled="disabled || loading" @click="emit('more')">
          {{ loading ? '正在加载…' : moreLabel }}
        </button>
      </AppPopover>
    </div>
  </div>
</template>

<style scoped>
.member-directory {
  margin: 0 0 20px;
  min-width: 0;
}
.member-choice {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--paper);
}
.member-selected {
  display: flex;
  align-items: center;
  gap: 8px;
}
.member-choice :deep(.popover-panel) {
  left: auto;
  right: 0;
}
.directory-summary {
  margin: 8px 0;
  font-size: var(--ui-font-meta);
}
.member-options {
  max-height: 260px;
  overflow: auto;
  overscroll-behavior: contain;
  border: 1px solid var(--line);
  border-radius: 8px;
}
.member-option {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0;
  padding: 12px;
  cursor: pointer;
  background: var(--paper);
}
.member-option + .member-option {
  border-top: 1px solid var(--line);
}
.member-option:hover,
.member-option:focus-within {
  background: var(--soft);
}
.member-option.chosen {
  background: var(--soft);
  box-shadow: inset 3px 0 var(--accent);
}
.member-option input {
  flex: 0 0 auto;
  width: 18px;
  height: 18px;
  min-height: 0;
  padding: 0;
  margin: 0;
  border-radius: 50%;
  accent-color: var(--accent);
}
.member-option input:focus {
  box-shadow: none;
}
.member-avatar {
  display: grid;
  place-items: center;
  flex: 0 0 32px;
  height: 32px;
  border-radius: 50%;
  background: #e5ece5;
  color: var(--accent);
}
.member-name {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}
.member-selection {
  color: var(--accent);
  white-space: nowrap;
  font-size: var(--ui-font-meta);
}
.directory-empty {
  padding: 20px 12px;
  margin: 8px 0;
}
.member-selected {
  margin: 10px 0 0;
  color: var(--accent);
  font-size: var(--ui-font-meta);
  overflow-wrap: anywhere;
}
</style>
