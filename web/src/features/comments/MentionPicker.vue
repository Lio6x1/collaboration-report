<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import type { DocumentCollaborator } from '../../../../contracts/http/generated';
import { explain, transportEpoch } from '../../lib/http';
import { mentionCandidates } from './api';
import { accountNumber, matchesAccount } from '../account/identity';

const props = defineProps<{
  modelValue: string[];
  document: string;
  actor: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{ 'update:modelValue': [ids: string[]] }>();
const members = ref<DocumentCollaborator[]>([]),
  cursor = ref<string | null>(null),
  loaded = ref(false),
  expanded = ref(false),
  loading = ref(false),
  error = ref(''),
  query = ref('');
let generation = 0;
let active = true;
const visible = computed(() =>
  members.value.filter((member) => matchesAccount(member, query.value)),
);
const emptyHint = computed(() => {
  if (query.value.trim())
    return cursor.value
      ? '暂未找到这位协作者。可以换个姓名或编号，或继续加载更多协作者。'
      : '没有找到这位协作者，请核对姓名或编号。';
  return cursor.value
    ? '本页暂无其他协作者，可以继续加载。'
    : '当前没有其他可提及的协作者；邀请伙伴加入后即可选择。';
});
function name(member: DocumentCollaborator) {
  // 展示姓名，提交仍使用真实ID；只有重名时附编号，避免把两个人误认成同一位。
  return members.value.some(
    (other) => other.user_id !== member.user_id && other.display_name === member.display_name,
  )
    ? member.display_name + '（编号 ' + accountNumber(member.user_id) + '）'
    : member.display_name;
}
function selectedName(id: string) {
  const member = members.value.find((item) => item.user_id === id);
  return member ? name(member) : '已选协作者';
}
function select(member: DocumentCollaborator) {
  if (props.disabled || member.user_id === props.actor || props.modelValue.includes(member.user_id))
    return;
  // 只能选择本次有权目录返回的候选，不把输入框里的文字或手填编号当成权限目标。
  if (!members.value.some((item) => item.user_id === member.user_id)) return;
  if (props.modelValue.length >= 100) {
    error.value = '每次最多提及 100 位协作者';
    return;
  }
  emit('update:modelValue', [...props.modelValue, member.user_id]);
  query.value = '';
  error.value = '';
}
function remove(id: string) {
  if (!props.disabled)
    emit(
      'update:modelValue',
      props.modelValue.filter((value) => value !== id),
    );
}
async function open() {
  if (props.disabled) return;
  expanded.value = true;
  if (!loaded.value) await load();
}
async function load() {
  if (loading.value || props.disabled) return;
  const started = generation;
  loading.value = true;
  error.value = '';
  try {
    const page = await mentionCandidates(props.document, cursor.value);
    if (!active || started !== generation) return;
    const candidates = page.items.filter((member) => member.user_id !== props.actor);
    members.value = [
      ...members.value,
      ...candidates.filter(
        (member) => !members.value.some((old) => old.user_id === member.user_id),
      ),
    ];
    cursor.value = page.next_cursor ?? null;
    loaded.value = true;
  } catch (failure) {
    if (active && started === generation) error.value = explain(failure);
  } finally {
    if (started === generation) loading.value = false;
  }
}
watch([() => props.document, () => props.actor, transportEpoch], () => {
  // 换文档或会话时清空旧姓名及选中值；晚到的目录响应不能进入新评论上下文。
  generation++;
  members.value = [];
  cursor.value = null;
  loaded.value = false;
  expanded.value = false;
  loading.value = false;
  error.value = '';
  query.value = '';
  emit('update:modelValue', []);
});
onBeforeUnmount(() => {
  active = false;
  generation++;
});
</script>

<template>
  <fieldset class="mention-picker" :disabled="disabled">
    <legend>@ 提及协作者（可选）</legend>
    <p class="hint">按姓名或账号编号选择要提醒的人；发表后，有权查看这条评论的协作者会收到通知。</p>
    <label class="mention-search">
      查找要提及的协作者
      <input
        v-model="query"
        placeholder="输入姓名或账号编号，或从候选中选择"
        autocomplete="off"
        @focus="open"
        @input="open"
      />
    </label>
    <button v-if="!expanded" type="button" @click="open">选择要 @ 的协作者</button>
    <div v-if="expanded" class="mention-directory">
      <p v-if="loading" class="hint" role="status">正在查找协作者…</p>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <button v-if="error" type="button" :disabled="loading" @click="load">重新加载协作者</button>
      <ul v-if="visible.length" class="mention-options" aria-label="可提及协作者">
        <li v-for="member in visible" :key="member.user_id">
          <button
            type="button"
            :data-user-id="member.user_id"
            :aria-label="'提及 ' + name(member)"
            :disabled="modelValue.includes(member.user_id) || modelValue.length >= 100"
            @click="select(member)"
          >
            <span class="mention-avatar" aria-hidden="true">{{
              member.display_name.slice(0, 1)
            }}</span>
            <span class="mention-name">{{ name(member) }}</span>
            <span class="mention-choice-state">{{
              modelValue.includes(member.user_id) ? '已选择' : '添加'
            }}</span>
          </button>
        </li>
      </ul>
      <p v-else-if="loaded && !loading && !error" class="hint" role="status">
        {{ emptyHint }}
      </p>
      <button v-if="cursor" type="button" :disabled="loading" @click="load">
        {{ loading ? '正在加载…' : '加载更多协作者' }}
      </button>
    </div>
    <ul v-if="modelValue.length" class="mention-selected" aria-label="已选提及对象">
      <li v-for="id in modelValue" :key="id">
        <span>@{{ selectedName(id) }}</span>
        <button type="button" :aria-label="'移除 ' + selectedName(id)" @click="remove(id)">
          移除
        </button>
      </li>
    </ul>
  </fieldset>
</template>
<style scoped>
.mention-picker {
  margin: 20px 0;
  padding: 18px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #f8f9f3;
  min-width: 0;
}
.mention-picker legend {
  padding: 0 6px;
  font-weight: 600;
}
.mention-picker > .hint {
  margin-top: 0;
}
.mention-search {
  margin-bottom: 12px;
}
.mention-directory {
  margin-top: 12px;
}
.mention-options,
.mention-selected {
  list-style: none;
  padding: 0;
  margin: 10px 0 0;
}
.mention-options {
  max-height: 240px;
  overflow: auto;
  overscroll-behavior: contain;
}
.mention-options li + li {
  margin-top: 6px;
}
.mention-options button {
  display: flex;
  gap: 10px;
  align-items: center;
  width: 100%;
  padding: 10px 12px;
  text-align: left;
}
.mention-avatar {
  display: grid;
  place-items: center;
  flex: 0 0 28px;
  height: 28px;
  background: #e8eddd;
  color: var(--accent);
  border-radius: 50%;
}
.mention-name {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}
.mention-choice-state {
  color: var(--muted);
  font-size: var(--ui-font-meta);
  white-space: nowrap;
}
.mention-selected {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.mention-selected li {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  max-width: 100%;
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 5px 8px;
  background: var(--paper);
}
.mention-selected li > span {
  min-width: 0;
  overflow-wrap: anywhere;
}
.mention-selected button {
  padding: 4px 6px;
  min-height: 28px;
  font-size: var(--ui-font-meta);
}
@media (max-width: 480px) {
  .mention-picker {
    padding: 12px;
  }
  .mention-options button {
    padding: 10px 8px;
  }
}
</style>
