<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue';
import type { KnowledgeBase, Member, MemberPage } from '../../../../contracts/http/generated';
import { api, useAction, actionsBlocked, ApiError, explain, transportEpoch } from '../../lib/http';
import ActionStatus from '../../components/ActionStatus.vue';
import MemberPicker from '../../components/MemberPicker.vue';
import PaginationControls from '../../components/PaginationControls.vue';
import { createCursorPage, pageCursor, acceptPage, resetPage } from '../../lib/pagination';
import { accountNumber } from '../account/identity';
const props = defineProps<{
    id: string;
    organization?: string;
  }>(),
  emit = defineEmits<{
    changed: [];
    deleted: [];
    createDocument: [];
  }>(),
  kb = ref<KnowledgeBase | null>(null),
  actions = ref<string[]>([]),
  members = ref<Member[]>([]),
  name = ref(''),
  target = ref(''),
  role = ref<'ADMIN' | 'MEMBER' | 'REMOVE'>('MEMBER'),
  allowed = ref(true),
  policy = ref('COLLABORATORS'),
  action = useAction();
const { busy, error, notice, pending, retry, mutate } = action;
const managementTab = ref<'basic' | 'members' | 'permissions'>('basic');
const memberPagination = reactive(createCursorPage()),
  memberQuery = ref(''),
  memberFilter = ref(''),
  memberBusy = ref(false);
const candidates = ref<Member[]>([]),
  candidateCursor = ref<string | null>(null),
  candidateQuery = ref(''),
  candidateBusy = ref(false),
  candidateError = ref('');
let active = true,
  generation = 0,
  memberRequest = 0;
const current = (started: number) => active && started === generation;
const candidateOptions = computed(() =>
  candidates.value.map((member) => ({ ...member, label: memberLabel(member) })),
);
const selectedCandidate = computed(() =>
  candidates.value.find((member) => member.user_id === target.value),
);
function memberLabel(member: Member) {
  const existing = members.value.find((item) => item.user_id === member.user_id);
  // 名字重名时展示完整账号编号，尾号也可能相同；操作提交服务器返回的真实账号 ID。
  const duplicates = candidates.value.filter((item) => item.display_name === member.display_name);
  return (
    member.display_name +
    (duplicates.length > 1 ? ' · 账号 ' + accountNumber(member.user_id) : '') +
    (existing
      ? existing.role === 'ADMIN'
        ? ' · 本库管理员'
        : ' · 本库成员'
      : // 当前成员页可能只展示一部分；没有出现在此页不能推断其尚未加入知识库。
        ' · 组织成员')
  );
}
function clearCandidates() {
  candidates.value = [];
  candidateCursor.value = null;
  candidateQuery.value = '';
  candidateError.value = '';
  candidateBusy.value = false;
  target.value = '';
}
async function candidatePage(append = false, started = generation) {
  if (!props.organization || !actions.value.includes('CHANGE_CREATION_GRANTS')) return;
  if (candidateBusy.value || (append && !candidateCursor.value)) return;
  if (!append) clearCandidates();
  candidateBusy.value = true;
  candidateError.value = '';
  try {
    // 组织成员目录允许当前有效组织成员读取，并不要求组织管理员身份。
    // 仍使用真实授权接口；本库成员只是子集，不能充当可加入本库的完整候选。
    const page = await api<MemberPage>(
      '/v1/organizations/' +
        props.organization +
        '/members?limit=50' +
        (append ? '&cursor=' + encodeURIComponent(candidateCursor.value!) : ''),
    );
    if (!current(started)) return;
    const unique = new Map(
      (append ? candidates.value : []).map((member) => [member.user_id, member]),
    );
    for (const member of page.items) {
      if (member.state === 'ACTIVE') unique.set(member.user_id, member);
    }
    candidates.value = [...unique.values()];
    candidateCursor.value = page.next_cursor;
  } catch (failure) {
    if (!current(started)) return;
    if (failure instanceof ApiError && [401, 403, 404, 410].includes(failure.status)) {
      // 撤权后不能继续展示旧候选，也不能保留一个看似仍可授权的旧目标。
      clearCandidates();
      kb.value = null;
      clearMembers();
      actions.value = [];
    }
    candidateError.value = explain(failure);
  } finally {
    if (current(started)) candidateBusy.value = false;
  }
}
function clearMembers() {
  memberRequest++;
  members.value = [];
  memberBusy.value = false;
  resetPage(memberPagination);
}
async function memberPage(index = 0) {
  const started = generation,
    id = props.id,
    filter = memberFilter.value,
    cursor = pageCursor(memberPagination, index),
    request = ++memberRequest;
  const valid = () =>
    current(started) &&
    request === memberRequest &&
    id === props.id &&
    filter === memberFilter.value;
  memberBusy.value = true;
  members.value = [];
  try {
    const p = await api<MemberPage>(
      '/v1/knowledge-bases/' +
        id +
        '/members?limit=20' +
        (cursor ? '&cursor=' + encodeURIComponent(cursor) : '') +
        (filter ? '&q=' + encodeURIComponent(filter) : ''),
    );
    if (!valid()) return;
    members.value = p.items;
    acceptPage(memberPagination, index, cursor, p.next_cursor);
  } catch (failure) {
    if (!valid()) return;
    if (failure instanceof ApiError && [401, 403, 404, 410].includes(failure.status)) {
      // 翻页同样核验当前资格；撤权后同时关闭库详情与可授权候选，不能保留旧目标。
      generation++;
      kb.value = null;
      actions.value = [];
      clearMembers();
      clearCandidates();
    }
    throw failure;
  } finally {
    if (request === memberRequest) memberBusy.value = false;
  }
}
async function searchMembers() {
  memberFilter.value = memberQuery.value.trim();
  clearMembers();
  await memberPage();
}
async function load() {
  const started = ++generation;
  kb.value = null;
  clearMembers();
  actions.value = [];
  clearCandidates();
  let available: string[] = [];
  const reading = await api<KnowledgeBase>(
    '/v1/knowledge-bases/' + props.id,
    'GET',
    undefined,
    (h) => {
      try {
        available = JSON.parse(h.get('X-Available-Actions') ?? '[]');
      } catch {
        available = [];
      }
    },
  );
  if (!current(started)) return;
  actions.value = available;
  kb.value = reading;
  name.value = reading.name;
  policy.value = reading.visibility_policy;
  if (actions.value.includes('VIEW_MEMBERS')) await memberPage();
  if (current(started)) await candidatePage(false, started);
}
function refresh() {
  void action.run(load, '知识库状态已刷新');
}
onMounted(() => {
  void action.run(load, '');
  window.addEventListener('collab-refresh', refresh);
});
watch(
  [() => props.id, () => props.organization, transportEpoch],
  () => {
    // 切库、换账号与刷新都使旧请求失效；迟到目录不能安装到新的授权上下文。
    generation++;
    kb.value = null;
    clearMembers();
    actions.value = [];
    memberQuery.value = '';
    memberFilter.value = '';
    clearCandidates();
    refresh();
  },
  { flush: 'sync' },
);
onUnmounted(() => {
  active = false;
  generation++;
  window.removeEventListener('collab-refresh', refresh);
});
async function rename() {
  await action.run(async () => {
    await mutate(
      () => '/v1/knowledge-bases/' + props.id,
      () => ({ expected_revision: kb.value!.acl_revision, name: name.value }),
      'PATCH',
    );
    await load();
    emit('changed');
  }, '知识库名称已更新');
}
// 库管理员任免由组织上级决定，建文档资格另行分配；动作摘要只显示当前可执行入口。
async function changeRole() {
  if (!selectedCandidate.value) {
    error.value = '请先从列表选择目标成员。';
    return;
  }
  await action.run(async () => {
    await mutate(
      () => '/v1/knowledge-bases/' + props.id + '/roles',
      () => ({
        expected_acl_revision: kb.value!.acl_revision,
        user_id: target.value,
        role: role.value,
      }),
    );
    await load();
    emit('changed');
  }, '知识库成员角色已更新');
}
async function changeGrant() {
  if (!selectedCandidate.value) {
    error.value = '请先从列表选择目标成员。';
    return;
  }
  await action.run(async () => {
    await mutate(
      () => '/v1/knowledge-bases/' + props.id + '/creation-grants',
      () => ({
        expected_revision: kb.value!.acl_revision,
        user_id: target.value,
        allowed: allowed.value,
      }),
    );
    await load();
    emit('changed');
  }, '本库创建资格已更新');
}
async function changeVisibility() {
  await action.run(async () => {
    await mutate(
      () => '/v1/knowledge-bases/' + props.id + '/visibility',
      () => ({ expected_visibility_revision: kb.value!.visibility_revision, policy: policy.value }),
    );
    await load();
    emit('changed');
  }, '知识库受众范围已更新');
}
async function remove() {
  await action.run(async () => {
    await mutate(
      () => '/v1/knowledge-bases/' + props.id,
      () => ({ expected_revision: kb.value!.acl_revision }),
      'DELETE',
    );
    emit('deleted');
  }, '知识库及未发布文档已关闭');
}
</script>
<template>
  <section v-if="kb" class="panel" aria-label="当前知识库">
    <div class="section-title">
      <h2>{{ kb.name }}</h2>
      <button
        v-if="actions.includes('CREATE_DOCUMENT')"
        :disabled="actionsBlocked"
        @click="emit('createDocument')"
      >
        在本库新增文档
      </button>
    </div>
    <p class="hint">这些设置仅作用于当前知识库；成员身份和可执行动作仍由服务端核验。</p>
  </section>
  <ActionStatus
    v-if="!pending"
    :busy="busy"
    :error="error"
    :notice="notice"
    :pending="pending"
    :retry="retry"
  />
  <p v-if="candidateError" class="error" role="alert">{{ candidateError }}</p>
  <section
    v-if="
      actions.some((action) =>
        [
          'RENAME',
          'DELETE',
          'CHANGE_VISIBILITY',
          'CHANGE_CREATION_GRANTS',
          'CHANGE_ROLES',
        ].includes(action),
      )
    "
    class="panel kb-management"
  >
    <nav class="document-tabs" role="tablist" aria-label="知识库设置分区">
      <button
        role="tab"
        :aria-selected="managementTab === 'basic'"
        :class="{ selected: managementTab === 'basic' }"
        :disabled="actionsBlocked"
        @click="managementTab = 'basic'"
      >
        基本信息
      </button>
      <button
        v-if="actions.includes('VIEW_MEMBERS')"
        role="tab"
        :aria-selected="managementTab === 'members'"
        :class="{ selected: managementTab === 'members' }"
        :disabled="actionsBlocked"
        @click="managementTab = 'members'"
      >
        成员
      </button>
      <button
        v-if="
          actions.some((value) =>
            ['CHANGE_VISIBILITY', 'CHANGE_CREATION_GRANTS', 'CHANGE_ROLES'].includes(value),
          )
        "
        role="tab"
        :aria-selected="managementTab === 'permissions'"
        :class="{ selected: managementTab === 'permissions' }"
        :disabled="actionsBlocked"
        @click="managementTab = 'permissions'"
      >
        权限
      </button>
    </nav>
    <template v-if="kb"
      ><p class="hint">管理此知识库的成员、创建权限与正式版范围。</p>
      <div
        v-if="actions.includes('VIEW_MEMBERS')"
        v-show="managementTab === 'members'"
        class="table-wrap"
        :aria-busy="memberBusy"
      >
        <form class="inline-form" role="search" @submit.prevent="action.run(searchMembers, '')">
          <label
            >搜索库成员
            <input
              v-model="memberQuery"
              type="search"
              placeholder="输入昵称或账号"
              maxlength="100"
              :disabled="actionsBlocked || memberBusy"
            />
          </label>
          <button :disabled="actionsBlocked || memberBusy">搜索</button>
        </form>
        <table>
          <thead>
            <tr>
              <th>库成员</th>
              <th>库角色</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="m in members" :key="m.user_id">
              <td>
                {{ m.display_name }}<small>账号 {{ accountNumber(m.user_id) }}</small>
              </td>
              <td>{{ m.role === 'ADMIN' ? '知识库管理员' : '库成员' }}</td>
              <td>
                {{
                  m.state === 'ACTIVE' ? '正常' : m.state === 'REVOKING' ? '权限更新中' : '已退出'
                }}
              </td>
            </tr>
          </tbody>
        </table>
        <p v-if="!members.length && !memberBusy && !busy" class="empty">
          {{ memberFilter ? '没有匹配的库成员。' : '本页没有可显示的库成员。' }}
        </p>
        <PaginationControls
          label="知识库成员"
          :page="memberPagination.index + 1"
          :has-next="!!memberPagination.next"
          :busy="actionsBlocked || memberBusy"
          @change="(index) => action.run(() => memberPage(index), '')"
        />
      </div>
      <div class="kb-settings-sections">
        <section
          v-show="managementTab === 'basic'"
          v-if="
            actions.some((action) => ['RENAME', 'DELETE', 'CHANGE_VISIBILITY'].includes(action))
          "
        >
          <form v-if="actions.includes('RENAME')" class="inline-form" @submit.prevent="rename">
            <fieldset :disabled="actionsBlocked">
              <label>新的知识库名称<input v-model="name" required maxlength="160" /></label
              ><button :disabled="actionsBlocked">更新知识库名称</button>
            </fieldset>
          </form>

          <button
            v-if="actions.includes('DELETE')"
            class="danger spaced"
            :disabled="actionsBlocked"
            @click="remove"
          >
            删除知识库与未发布文档
          </button>
        </section>
        <section v-show="managementTab === 'permissions'">
          <form v-if="actions.includes('CHANGE_VISIBILITY')" @submit.prevent="changeVisibility">
            <fieldset :disabled="actionsBlocked">
              <label
                >知识库正式版受众上限<select v-model="policy">
                  <option value="COLLABORATORS">协作者</option>
                  <option value="SPACE_MEMBERS">空间成员</option>
                  <option value="PUBLIC">公开</option>
                </select></label
              ><button :disabled="actionsBlocked">更新知识库范围</button>
            </fieldset>
          </form>
        </section>
        <section
          v-if="actions.includes('CHANGE_CREATION_GRANTS')"
          v-show="managementTab === 'permissions'"
        >
          <div class="member-picker" :aria-busy="candidateBusy">
            <div class="section-title">
              <h3>选择目标成员</h3>
              <button
                type="button"
                :disabled="candidateBusy || actionsBlocked"
                @click="candidatePage()"
              >
                刷新成员列表
              </button>
            </div>
            <MemberPicker
              v-model="target"
              v-model:query="candidateQuery"
              :members="candidateOptions"
              search-label="按成员姓名或编号筛选"
              options-label="目标成员"
              empty-text="暂时没有可选择的组织成员。"
              more-label="加载更多可选成员"
              :has-more="!!candidateCursor"
              :cursor="candidateCursor"
              :loading="candidateBusy"
              :disabled="actionsBlocked"
              @more="candidatePage(true)"
            />
          </div>
          <form v-if="actions.includes('CHANGE_ROLES')" @submit.prevent="changeRole">
            <fieldset :disabled="actionsBlocked">
              <label
                >知识库角色<select v-model="role">
                  <option value="MEMBER">库成员</option>
                  <option value="ADMIN">库管理员</option>
                  <option value="REMOVE">移除库成员角色</option>
                </select></label
              ><button :disabled="busy || !selectedCandidate">更新库成员角色</button>
            </fieldset>
          </form>
          <form @submit.prevent="changeGrant">
            <fieldset :disabled="actionsBlocked">
              <label
                >文档创建资格<select v-model="allowed">
                  <option :value="true">允许在本库创建文档</option>
                  <option :value="false">撤销本库文档创建资格</option>
                </select></label
              ><button :disabled="busy || !selectedCandidate">更新本库创建资格</button>
            </fieldset>
          </form>
          <p class="hint">
            任免库管理员需要组织上级管理资格；库管理员只能分配本库创建资格，不能任命同级。
          </p>
        </section>
      </div></template
    >
  </section>
</template>
