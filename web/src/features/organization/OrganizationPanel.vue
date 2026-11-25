<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, defineAsyncComponent } from 'vue';
import type {
  Organization,
  OrganizationPage,
  MemberPage,
  Member,
  ApplicationPage,
  Application,
  InvitationPage,
  Invitation,
  InvitationCreated,
} from '../../../../contracts/http/generated';
import {
  api,
  useAction,
  actionsBlocked,
  UserFacingError,
  ApiError,
  transportGeneration,
} from '../../lib/http';
import { createCursorPage, pageCursor, acceptPage, resetPage } from '../../lib/pagination';
import { user } from '../account/session';
import { accountNumber, publicNumber, idFromPublicNumber } from '../account/identity';
import ActionStatus from '../../components/ActionStatus.vue';
import AppDialog from '../../components/AppDialog.vue';
import AppPopover from '../../components/AppPopover.vue';
import PaginationControls from '../../components/PaginationControls.vue';
import { canonicalID } from '../search/rules';
import OrganizationClosePanel from './OrganizationClosePanel.vue';
const WorkspacePanel = defineAsyncComponent(() => import('../workspace/WorkspacePanel.vue'));
const props = defineProps<{ initialOrganization?: string }>();
const emit = defineEmits<{ search: [space: string]; open: [document: string] }>();
const closePanel = ref<InstanceType<typeof OrganizationClosePanel> | null>(null);
// 通知/交接及原关闭操作需要直接回到管理；日常进入组织则先看到有权内容。
const detailPane = ref<'content' | 'management'>(
  props.initialOrganization ||
    new URLSearchParams(location.search).has('organization_close_operation')
    ? 'management'
    : 'content',
);
const contentVisited = ref(detailPane.value === 'content');
const managementLoaded = ref(false);
let active = true,
  organizationGeneration = 0,
  orgRequest = 0,
  memberRequest = 0,
  appRequest = 0,
  inviteRequest = 0,
  ownRequest = 0,
  managementRequest = 0;
const current = (generation: number) => active && generation === organizationGeneration;
const organizations = ref<Organization[]>([]),
  selected = ref<Organization | null>(null),
  members = ref<Member[]>([]),
  applications = ref<Application[]>([]),
  invites = ref<Invitation[]>([]),
  ownApplications = ref<Application[]>([]);
const orgPage = reactive(createCursorPage()),
  memberPage = reactive(createCursorPage()),
  appPage = reactive(createCursorPage()),
  invitePage = reactive(createCursorPage()),
  ownPage = reactive(createCursorPage());
const memberSearch = ref(''),
  memberQuery = ref('');
const name = ref(''),
  rename = ref(''),
  joinId = ref(''),
  joinReason = ref(''),
  inviteToken = ref(new URLSearchParams(location.search).get('invitation_token') ?? ''),
  hours = ref(48),
  share = ref(''),
  target = ref(''),
  action = useAction();
// 创建/加入是独立短任务，关闭仅隐藏界面，表单输入和申请进度继续保留。
const membershipOpen = ref(location.pathname === '/join' || !!inviteToken.value);
function openMembership() {
  membershipOpen.value = true;
}
const membershipKind = ref<'create' | 'join'>(
  location.pathname === '/join' || inviteToken.value ? 'join' : 'create',
);
const ownApplicationsVisible = ref(false);
const joinOrganizationID = computed(() => idFromPublicNumber(joinId.value));
const copyingNumber = ref(false),
  numberNotice = ref('');
async function copyOrganizationNumber() {
  if (!selected.value || copyingNumber.value) return;
  const organization = selected.value.id,
    generation = organizationGeneration;
  copyingNumber.value = true;
  numberNotice.value = '';
  try {
    // 复制与展示使用同一格式；申请入口再还原原ID，不改变组织身份或权限关系。
    await navigator.clipboard.writeText(publicNumber(organization));
    if (current(generation) && selected.value?.id === organization)
      numberNotice.value = '组织编号已复制';
  } catch {
    if (current(generation) && selected.value?.id === organization)
      numberNotice.value = '暂时无法复制，请选中组织编号手动复制。';
  } finally {
    copyingNumber.value = false;
  }
}
const { busy, error, notice, pending, retry, write, mutate } = action;
// 这里只控制按钮是否展示；服务器仍以当前成员/所有者代次重新准入，页面不能自证角色。
const mine = computed(() => {
  const member = selected.value?.current_member;
  return member?.user_id === user.value?.id ? member : undefined;
});
const owner = computed(() => selected.value?.owner_id === user.value?.id),
  admin = computed(
    () => owner.value || (mine.value?.role === 'ADMIN' && mine.value.state === 'ACTIVE'),
  );
const knownOwnerName = ref('');
const ownerName = computed(() => {
  if (!selected.value) return '';
  // 只用当前登录身份或已授权成员目录中的昵称；未知姓名不能用账号编号冒充。
  if (owner.value) return user.value?.display_name ?? '';
  return knownOwnerName.value;
});
function clearDeniedOrganization(failure: unknown) {
  if (!(failure instanceof ApiError) || ![401, 403, 404, 410].includes(failure.status)) return;
  // 调用方先核对组织、身份和本次请求代次；明确撤权才清资格，短暂503不证明成员已退出。
  // 递增组织代次也使其它并行目录响应失效，防止它们重新装回已撤权内容。
  organizationGeneration++;
  selected.value = null;
  members.value = [];
  applications.value = [];
  invites.value = [];
  resetPage(memberPage);
  resetPage(appPage);
  resetPage(invitePage);
  managementLoaded.value = false;
  knownOwnerName.value = '';
  memberSearch.value = '';
  memberQuery.value = '';
  share.value = '';
  target.value = '';
  numberNotice.value = '';
  // action持有的未知写入编号、固定输入和重试闭包继续保留，读目录撤权不等于写操作终态。
}
async function listOrg(index = 0) {
  const cursor = pageCursor(orgPage, index),
    request = ++orgRequest;
  const p = await api<OrganizationPage>(
    '/v1/organizations?limit=20' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
  );
  if (!active || request !== orgRequest) return;
  organizations.value = p.items;
  acceptPage(orgPage, index, cursor, p.next_cursor);
}
async function loadMembers(index = 0) {
  if (!selected.value) return;
  const generation = organizationGeneration,
    organization = selected.value.id,
    identity = transportGeneration(),
    query = memberQuery.value,
    cursor = pageCursor(memberPage, index),
    request = ++memberRequest;
  const valid = () =>
    current(generation) &&
    selected.value?.id === organization &&
    identity === transportGeneration() &&
    request === memberRequest &&
    query === memberQuery.value;
  // 先卸下当前可见行，失败时也不能用旧页冒充这次经过服务端准入的结果。
  members.value = [];
  target.value = '';
  try {
    const p = await api<MemberPage>(
      '/v1/organizations/' +
        organization +
        '/members?limit=20' +
        (query ? '&q=' + encodeURIComponent(query) : '') +
        (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
    );
    if (!valid()) return;
    members.value = p.items;
    // 组织内已授权读取的所有者昵称独立保留，翻页隐藏其行时标题不应闪回匿名状态。
    const listedOwner = p.items.find((member) => member.user_id === selected.value?.owner_id);
    if (listedOwner) knownOwnerName.value = listedOwner.display_name;
    acceptPage(memberPage, index, cursor, p.next_cursor);
  } catch (failure) {
    if (valid()) clearDeniedOrganization(failure);
    throw failure;
  }
}
async function searchMembers() {
  await action.run(async () => {
    // 昵称或八位账号都交由服务端筛选；新筛选必须从新游标链首页开始。
    memberQuery.value = memberSearch.value.trim();
    memberRequest++;
    resetPage(memberPage);
    members.value = [];
    target.value = '';
    await loadMembers();
  }, '');
}
async function loadApps(index = 0) {
  if (!selected.value || !admin.value) return;
  const generation = organizationGeneration,
    organization = selected.value.id,
    identity = transportGeneration(),
    cursor = pageCursor(appPage, index),
    request = ++appRequest;
  const valid = () =>
    current(generation) &&
    selected.value?.id === organization &&
    identity === transportGeneration() &&
    request === appRequest;
  applications.value = [];
  try {
    const p = await api<ApplicationPage>(
      '/v1/organizations/' +
        organization +
        '/applications?limit=20' +
        (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
    );
    if (!valid()) return;
    applications.value = p.items;
    acceptPage(appPage, index, cursor, p.next_cursor);
  } catch (failure) {
    if (valid()) clearDeniedOrganization(failure);
    throw failure;
  }
}
async function loadInvites(index = 0) {
  if (!selected.value || !admin.value) return;
  share.value = '';
  const generation = organizationGeneration,
    organization = selected.value.id,
    identity = transportGeneration(),
    cursor = pageCursor(invitePage, index),
    request = ++inviteRequest;
  const valid = () =>
    current(generation) &&
    selected.value?.id === organization &&
    identity === transportGeneration() &&
    request === inviteRequest;
  invites.value = [];
  try {
    const p = await api<InvitationPage>(
      '/v1/organizations/' +
        organization +
        '/invitations?limit=20' +
        (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
    );
    if (!valid()) return;
    invites.value = p.items;
    acceptPage(invitePage, index, cursor, p.next_cursor);
  } catch (failure) {
    if (valid()) clearDeniedOrganization(failure);
    throw failure;
  }
}
async function ownApps(index = 0) {
  const cursor = pageCursor(ownPage, index),
    request = ++ownRequest;
  const p = await api<ApplicationPage>(
    '/v1/account/applications?limit=20' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
  );
  if (!active || request !== ownRequest) return;
  ownApplications.value = p.items;
  acceptPage(ownPage, index, cursor, p.next_cursor);
}
async function openOrg(id: string) {
  const generation = ++organizationGeneration;
  // 切换先卸载上一组织的目录及权限表单；晚到响应不得把旧成员或文档装到新组织下。
  selected.value = null;
  members.value = [];
  resetPage(memberPage);
  memberSearch.value = '';
  memberQuery.value = '';
  knownOwnerName.value = '';
  applications.value = [];
  resetPage(appPage);
  invites.value = [];
  resetPage(invitePage);
  managementLoaded.value = false;
  share.value = '';
  target.value = '';
  numberNotice.value = '';
  const o = await api<Organization>('/v1/organizations/' + id);
  if (!current(generation)) return;
  selected.value = o;
  rename.value = o.name;
  if (detailPane.value === 'management') await loadManagement();
}
async function loadManagement() {
  const generation = organizationGeneration,
    request = ++managementRequest;
  await loadMembers();
  if (!current(generation) || request !== managementRequest) return;
  // 当前成员资格来自组织详情，与可见目录页独立；翻页和搜索不能让管理入口消失。
  if (admin.value) {
    await Promise.all([loadApps(), loadInvites()]);
  }
  if (current(generation) && request === managementRequest) managementLoaded.value = true;
}
function selectOrganization(id: string) {
  ownApplicationsVisible.value = false;
  detailPane.value = 'content';
  contentVisited.value = true;
  if (new URLSearchParams(location.search).get('organization_id') !== id)
    history.pushState(null, '', '/organizations?organization_id=' + encodeURIComponent(id));
  void action.run(() => openOrg(id), '');
}
function restoreOrganization() {
  if (location.pathname !== '/organizations' || actionsBlocked.value) return;
  const id = new URLSearchParams(location.search).get('organization_id') ?? '';
  if (canonicalID(id) && selected.value?.id !== id) selectOrganization(id);
}
function selectDetailPane(next: 'content' | 'management') {
  detailPane.value = next;
  if (next === 'content') contentVisited.value = true;
  else if (!managementLoaded.value) void action.run(loadManagement, '');
}
async function refresh() {
  await Promise.all([listOrg(), ownApps()]);
  if (selected.value) await openOrg(selected.value.id);
}
function onRefresh() {
  void action.run(refresh, '状态已刷新');
}
onMounted(() => {
  void action.run(async () => {
    await refresh();
    // 通知只提供目标编号；详情和管理能力仍通过Account当前资格接口重新读取。
    if (props.initialOrganization) await openOrg(props.initialOrganization);
    else {
      const target = new URLSearchParams(location.search).get('organization_id') ?? '';
      if (canonicalID(target)) await openOrg(target);
      else if (!selected.value && organizations.value[0]) await openOrg(organizations.value[0].id);
    }
    if (location.pathname === '/join' || inviteToken.value) await openMembership();
  }, '');
  window.addEventListener('collab-refresh', onRefresh);
  window.addEventListener('popstate', restoreOrganization);
});
onUnmounted(() => {
  active = false;
  organizationGeneration++;
  window.removeEventListener('collab-refresh', onRefresh);
  window.removeEventListener('popstate', restoreOrganization);
});
async function createOrg() {
  await action.run(async () => {
    await mutate(
      () => '/v1/organizations',
      () => ({ name: name.value }),
    );
    name.value = '';
    await listOrg();
    membershipOpen.value = false;
  }, '组织已创建');
}
async function applyOrg() {
  await action.run(async () => {
    if (inviteToken.value) {
      await mutate(
        () => '/v1/invitations/accept',
        () => ({ invitation_token: inviteToken.value, reason: joinReason.value }),
      );
      history.replaceState(null, '', '/organizations');
      inviteToken.value = '';
    } else {
      const organization = joinOrganizationID.value;
      if (!organization) throw new UserFacingError('请输入有效的组织编号。');
      await mutate(
        () => '/v1/organizations/' + organization + '/applications',
        () => ({ reason: joinReason.value }),
      );
    }
    await ownApps();
    membershipOpen.value = false;
    ownApplicationsVisible.value = true;
  }, '申请已提交，等待管理员审批');
}
// expected_revision来自最后读取的权威事实。任免、创建资格是独立动作，不合并成万能角色。
async function memberAction(m: Member, kind: string) {
  if (!selected.value) return;
  // 详情刷新会先置空selected，续接必须持有第一次操作的组织身份。
  const organizationId = selected.value.id,
    base = '/v1/organizations/' + organizationId,
    memberRevision = selected.value.member_revision;
  await action.run(async () => {
    if (kind === 'role')
      await mutate(
        () => base + '/members/' + m.user_id + '/role',
        () => ({ expected_revision: m.revision, role: m.role === 'ADMIN' ? 'MEMBER' : 'ADMIN' }),
      );
    else if (kind === 'remove')
      await mutate(
        () => base + '/members/' + m.user_id,
        () => ({ expected_revision: m.revision }),
        'DELETE',
      );
    else
      await mutate(
        () => base + '/creation-grants',
        () => ({
          expected_revision: memberRevision,
          user_id: m.user_id,
          allowed: kind === 'allow',
        }),
      );
    await openOrg(organizationId);
  }, '成员权限已更新');
}
async function review(a: Application, decision: 'ACCEPT' | 'REJECT') {
  const organizationId = selected.value?.id;
  if (!organizationId) return;
  await action.run(async () => {
    await mutate(
      () => '/v1/applications/' + a.id + '/decisions',
      () => ({
        expected_revision: a.revision,
        decision,
        reason: decision === 'ACCEPT' ? '同意加入' : '本次申请未通过',
      }),
    );
    await openOrg(organizationId);
  }, '申请已处理');
}
async function createInvite() {
  if (!selected.value) return;
  await action.run(async () => {
    const r = await write<InvitationCreated>(
      () => '/v1/organizations/' + selected.value!.id + '/invitations',
      () => ({
        expected_member_revision: selected.value!.member_revision,
        expires_in_hours: Number(hours.value),
      }),
    );
    await loadInvites();
    share.value = r.share_url;
  }, '邀请链接已生成，收到链接者仍须登录申请');
}
async function cancelInvite(i: Invitation) {
  await action.run(async () => {
    await mutate(
      () => '/v1/invitations/' + i.id + '/cancel',
      () => ({ expected_revision: i.revision }),
    );
    await loadInvites();
  }, '邀请已关闭，已受理申请仍可审批');
}
async function renameOrg() {
  await action.run(async () => {
    await mutate(
      () => '/v1/organizations/' + selected.value!.id,
      () => ({ expected_revision: selected.value!.revision, name: rename.value }),
      'PATCH',
    );
    await refresh();
  }, '组织名称已更新');
}
async function transferOwner() {
  await action.run(async () => {
    await mutate(
      () => '/v1/organizations/' + selected.value!.id + '/owner-transfers',
      () => ({ expected_revision: selected.value!.revision, target_user_id: target.value }),
    );
    await refresh();
  }, '所有者已转让');
}
async function leave() {
  if (!selected.value || !mine.value) return;
  const organization = selected.value.id,
    revision = mine.value.revision;
  await action.run(async () => {
    await mutate(
      () => '/v1/organizations/' + organization + '/leave',
      () => ({ expected_revision: revision }),
    );
    selected.value = null;
    await refresh();
  }, '已退出组织');
}
</script>
<template>
  <section class="page-heading organization-page-heading">
    <div>
      <h1>组织</h1>
      <p>选择组织浏览知识库与文档，或管理成员和协作资格。</p>
    </div>
    <div class="button-row">
      <button
        v-if="closePanel?.original && (!selected || ownApplicationsVisible)"
        type="button"
        @click="closePanel.open"
      >
        查看组织关闭进度
      </button>
      <button :disabled="actionsBlocked" @click="onRefresh">刷新组织</button>
    </div>
  </section>
  <ActionStatus
    v-if="!pending"
    :busy="busy"
    :error="error"
    :notice="notice"
    :pending="pending"
    :retry="retry"
  />
  <nav class="document-tabs" aria-label="组织页面">
    <button
      :aria-current="!ownApplicationsVisible ? 'page' : undefined"
      :disabled="actionsBlocked"
      @click="ownApplicationsVisible = false"
    >
      组织内容
    </button>
    <button
      :aria-current="ownApplicationsVisible ? 'page' : undefined"
      :disabled="actionsBlocked"
      @click="ownApplicationsVisible = true"
    >
      我的组织申请
    </button>
    <button
      :class="{ primary: !organizations.length }"
      :disabled="actionsBlocked"
      @click="openMembership"
    >
      创建或加入组织
    </button>
  </nav>
  <section
    v-if="!ownApplicationsVisible"
    class="panel organization-selector"
    aria-labelledby="organization-list-title"
  >
    <div class="section-title">
      <h2 id="organization-list-title">我的组织</h2>
      <ul class="organization-list">
        <li v-for="o in organizations" :key="o.id">
          <button
            class="organization-choice"
            :disabled="actionsBlocked"
            :data-organization-id="o.id"
            :aria-label="o.name"
            :aria-pressed="selected?.id === o.id"
            @click="selectOrganization(o.id)"
          >
            <span>{{ o.name }}</span>
            <small>{{ o.owner_id === user?.id ? '所有者' : '成员' }}</small>
          </button>
        </li>
      </ul>
    </div>
    <p v-if="!busy && !error && !organizations.length" class="empty">
      还没有组织，创建新组织或申请加入后，即可在这里浏览组织内容。
    </p>
    <PaginationControls
      label="我的组织"
      :page="orgPage.index + 1"
      :has-next="!!orgPage.next"
      :busy="actionsBlocked"
      @change="(index) => action.run(() => listOrg(index), '')"
    />
  </section>
  <section
    v-if="selected && !ownApplicationsVisible"
    class="panel detail-panel organization-detail"
  >
    <div class="section-title organization-detail-heading">
      <div>
        <h2>{{ selected.name }}</h2>
        <p class="hint">
          <template v-if="ownerName"
            >所有者：{{ ownerName }}<span v-if="owner">（你）</span></template
          >
          <template v-else>{{ admin ? '你是此组织的管理员' : '组织协作空间' }}</template>
        </p>
        <div class="organization-number">
          <span
            >组织编号：<code>{{ publicNumber(selected.id) }}</code></span
          >
          <button type="button" :disabled="copyingNumber" @click="copyOrganizationNumber">
            {{ copyingNumber ? '复制中…' : '复制组织编号' }}
          </button>
          <small v-if="numberNotice" role="status">{{ numberNotice }}</small>
        </div>
      </div>
      <div class="organization-header-actions">
        <button v-if="closePanel?.original" type="button" @click="closePanel.open">
          查看组织关闭进度
        </button>
        <AppPopover
          v-else-if="owner && selected.lifecycle === 'ACTIVE'"
          label="更多操作"
          :disabled="actionsBlocked"
        >
          <template #default="{ close: closeMenu }">
            <button
              type="button"
              class="danger close-entry"
              @click="
                closeMenu();
                closePanel?.open();
              "
            >
              关闭组织…
            </button>
          </template>
        </AppPopover>
        <button
          v-if="detailPane === 'management' && mine && mine.user_id !== selected.owner_id"
          :disabled="actionsBlocked"
          @click="leave"
        >
          退出组织
        </button>
      </div>
    </div>
    <nav class="document-tabs organization-detail-tabs" role="tablist" aria-label="组织详情">
      <button
        id="organization-content-tab"
        role="tab"
        aria-controls="organization-content"
        :aria-selected="detailPane === 'content'"
        :class="{ selected: detailPane === 'content' }"
        :disabled="actionsBlocked"
        @click="selectDetailPane('content')"
      >
        内容
      </button>
      <button
        id="organization-management-tab"
        role="tab"
        aria-controls="organization-management"
        :aria-selected="detailPane === 'management'"
        :class="{ selected: detailPane === 'management' }"
        :disabled="actionsBlocked"
        @click="selectDetailPane('management')"
      >
        {{ admin ? '成员与管理' : '组织成员' }}
      </button>
    </nav>
    <div
      v-if="contentVisited"
      v-show="detailPane === 'content'"
      id="organization-content"
      role="tabpanel"
      aria-labelledby="organization-content-tab"
    >
      <!-- 复用工作台目录及权限读法，仅绑定当前组织；切换入口时保留已选库与分页位置。 -->
      <WorkspacePanel
        :key="selected.id"
        :organization="selected"
        @open="emit('open', $event)"
        @search="emit('search', $event)"
      />
    </div>
    <div
      v-show="detailPane === 'management'"
      id="organization-management"
      role="tabpanel"
      aria-labelledby="organization-management-tab"
    >
      <p class="hint">组织管理与知识库管理分别授权，创建资格单独分配。</p>
      <p class="hint">管理员退出前须先交接管理角色；所有者须先转让组织。</p>
      <form class="inline-form" @submit.prevent="searchMembers">
        <fieldset :disabled="actionsBlocked">
          <label
            >搜索组织成员<input
              v-model="memberSearch"
              type="search"
              maxlength="160"
              placeholder="昵称或8位账号"
          /></label>
          <button :disabled="actionsBlocked">搜索成员</button>
        </fieldset>
      </form>
      <p v-if="managementLoaded && !busy && !error && !members.length" class="empty">
        {{ memberQuery ? '没有找到匹配的组织成员。' : '暂无组织成员。' }}
      </p>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>成员</th>
              <th>角色</th>
              <th>状态</th>
              <th>建库资格</th>
              <th v-if="admin">管理</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="m in members" :key="m.user_id">
              <td>
                {{ m.display_name
                }}<small
                  v-if="
                    members.some(
                      (other) =>
                        other.user_id !== m.user_id && other.display_name === m.display_name,
                    )
                  "
                  >{{ accountNumber(m.user_id) }}</small
                >
              </td>
              <td>
                {{
                  m.user_id === selected.owner_id
                    ? '所有者'
                    : m.role === 'ADMIN'
                      ? '组织管理员'
                      : '普通成员'
                }}
              </td>
              <td>
                {{
                  m.state === 'ACTIVE' ? '正常' : m.state === 'REVOKING' ? '权限更新中' : '已退出'
                }}
              </td>
              <td>
                <span
                  :class="['status-pill', { 'status-draft': m.knowledge_base_creation === 'NONE' }]"
                >
                  {{
                    m.knowledge_base_creation === 'ROLE'
                      ? '角色自带'
                      : m.knowledge_base_creation === 'GRANTED'
                        ? '已授予'
                        : m.knowledge_base_creation === 'NONE'
                          ? '未授予'
                          : '状态待刷新'
                  }}
                </span>
                <small v-if="m.knowledge_base_creation === 'ROLE'">随所有者／管理员角色生效</small>
              </td>
              <td v-if="admin">
                <div class="button-row">
                  <button
                    v-if="owner && m.user_id !== selected.owner_id && m.state === 'ACTIVE'"
                    :disabled="actionsBlocked"
                    @click="memberAction(m, 'role')"
                  >
                    {{ m.role === 'ADMIN' ? '降为成员' : '任命管理员' }}</button
                  ><button
                    v-if="m.state === 'ACTIVE' && m.knowledge_base_creation === 'NONE'"
                    :disabled="actionsBlocked"
                    @click="memberAction(m, 'allow')"
                  >
                    授予建库资格</button
                  ><button
                    v-else-if="m.state === 'ACTIVE' && m.knowledge_base_creation === 'GRANTED'"
                    :disabled="actionsBlocked"
                    @click="memberAction(m, 'deny')"
                  >
                    撤销建库资格</button
                  ><button
                    v-if="m.user_id !== selected.owner_id && m.state === 'ACTIVE'"
                    class="danger"
                    :disabled="actionsBlocked"
                    @click="memberAction(m, 'remove')"
                  >
                    移除成员
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <PaginationControls
        label="组织成员"
        :page="memberPage.index + 1"
        :has-next="!!memberPage.next"
        :busy="actionsBlocked"
        @change="(index) => action.run(() => loadMembers(index), '')"
      />
      <div v-if="admin" class="two-column spaced">
        <section>
          <h3>待审批申请</h3>
          <ul class="entity-list">
            <li v-for="a in applications" :key="a.id">
              <span>账号 {{ accountNumber(a.applicant_id) }} · {{ a.state }}</span
              ><small>{{ a.reason }}</small>
              <div v-if="a.state === 'PENDING'" class="button-row">
                <button :disabled="actionsBlocked" @click="review(a, 'ACCEPT')">同意加入</button
                ><button :disabled="actionsBlocked" @click="review(a, 'REJECT')">拒绝申请</button>
              </div>
            </li>
          </ul>
          <PaginationControls
            label="待审批申请"
            :page="appPage.index + 1"
            :has-next="!!appPage.next"
            :busy="actionsBlocked"
            @change="(index) => action.run(() => loadApps(index), '')"
          />
        </section>
        <section>
          <h3>邀请与名称</h3>
          <form class="inline-form" @submit.prevent="createInvite">
            <fieldset :disabled="actionsBlocked">
              <label
                >邀请有效小时数<input
                  v-model="hours"
                  type="number"
                  min="1"
                  max="720"
                  required /></label
              ><button :disabled="actionsBlocked">生成组织邀请</button>
            </fieldset>
          </form>
          <label v-if="share"
            >可分享链接<input
              :value="share"
              readonly
              @focus="($event.target as HTMLInputElement).select()"
          /></label>
          <ul class="entity-list">
            <li v-for="i in invites" :key="i.id">
              <span>{{ i.state }} · {{ new Date(i.expires_at).toLocaleString('zh-CN') }}</span
              ><button
                v-if="i.state === 'OPEN'"
                :disabled="actionsBlocked"
                @click="cancelInvite(i)"
              >
                关闭邀请
              </button>
            </li>
          </ul>
          <PaginationControls
            label="组织邀请"
            :page="invitePage.index + 1"
            :has-next="!!invitePage.next"
            :busy="actionsBlocked"
            @change="(index) => action.run(() => loadInvites(index), '')"
          />
          <form class="inline-form" @submit.prevent="renameOrg">
            <fieldset :disabled="actionsBlocked">
              <label>新的组织名称<input v-model="rename" required maxlength="160" /></label
              ><button :disabled="actionsBlocked">更新组织名称</button>
            </fieldset>
          </form>
        </section>
      </div>
      <form v-if="owner" class="inline-form spaced" @submit.prevent="transferOwner">
        <fieldset :disabled="actionsBlocked">
          <label
            >接任所有者<select v-model="target" required>
              <option value="">选择当前成员</option>
              <option
                v-for="m in members.filter(
                  (x) => x.user_id !== selected?.owner_id && x.state === 'ACTIVE',
                )"
                :key="m.user_id"
                :value="m.user_id"
              >
                {{ m.display_name
                }}<template
                  v-if="
                    members.some(
                      (other) =>
                        other.user_id !== m.user_id && other.display_name === m.display_name,
                    )
                  "
                >
                  · 账号 {{ accountNumber(m.user_id) }}</template
                >
              </option>
            </select></label
          ><button :disabled="actionsBlocked" class="danger">转让组织所有者</button>
        </fieldset>
      </form>
    </div>
  </section>
  <OrganizationClosePanel
    ref="closePanel"
    :organization="selected"
    :allowed="owner"
    @closed="
      selected = null;
      action.run(() => listOrg(), '');
    "
  />
  <section v-if="ownApplicationsVisible" class="panel" aria-label="组织申请进度">
    <h3>我的申请进度</h3>
    <ul class="entity-list">
      <li v-for="a in ownApplications" :key="a.id">
        <span
          >组织 {{ publicNumber(a.organization_id) }} ·
          {{
            a.state === 'PENDING'
              ? '待审批'
              : a.state === 'ACCEPTED'
                ? '已加入'
                : a.state === 'REJECTED'
                  ? '未通过'
                  : '已取消'
          }}</span
        ><small>{{ a.reason }}</small>
      </li>
    </ul>
    <PaginationControls
      label="我的组织申请"
      :page="ownPage.index + 1"
      :has-next="!!ownPage.next"
      :busy="actionsBlocked"
      @change="(index) => action.run(() => ownApps(index), '')"
    />
  </section>
  <AppDialog v-model="membershipOpen" title="创建或加入组织" :blocked="actionsBlocked">
    <nav class="document-tabs" aria-label="组织办理类型">
      <button
        :aria-pressed="membershipKind === 'create'"
        :disabled="actionsBlocked"
        @click="membershipKind = 'create'"
      >
        创建组织
      </button>
      <button
        :aria-pressed="membershipKind === 'join'"
        :disabled="actionsBlocked"
        @click="membershipKind = 'join'"
      >
        申请加入
      </button>
    </nav>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
    <div>
      <section v-show="membershipKind === 'create'">
        <h2 class="visually-hidden">创建组织</h2>
        <form @submit.prevent="createOrg">
          <fieldset :disabled="actionsBlocked">
            <label>组织名称<input v-model="name" required maxlength="160" /></label>
            <button class="primary" :disabled="actionsBlocked">创建组织</button>
          </fieldset>
        </form>
      </section>
      <section v-show="membershipKind === 'join'">
        <h2 class="visually-hidden">申请加入</h2>
        <form @submit.prevent="applyOrg">
          <fieldset :disabled="actionsBlocked">
            <label
              >组织邀请 token<input
                v-model="inviteToken"
                autocomplete="off"
                placeholder="收到邀请链接后填写，可留空"
            /></label>
            <label v-if="!inviteToken"
              >组织编号<input
                v-model="joinId"
                required
                pattern="[0-9]{1,20}"
                maxlength="20"
                inputmode="numeric"
                placeholder="例如 00004483"
            /></label>
            <label
              >申请说明<textarea v-model="joinReason" required maxlength="2000"></textarea>
            </label>
            <button :disabled="actionsBlocked || (!inviteToken && !joinOrganizationID)">
              提交加入申请
            </button>
          </fieldset>
        </form>
      </section>
    </div>
  </AppDialog>
</template>

<style scoped>
.organization-page-heading {
  margin-bottom: 16px;
}
.organization-page-heading h1 {
  margin: 0 0 4px;
}
.organization-page-heading p {
  margin: 0;
}
.organization-page-heading > .button-row {
  flex-shrink: 0;
}
.organization-page-heading button {
  white-space: nowrap;
}
.organization-selector {
  padding: 18px 24px;
}
.organization-selector .section-title {
  margin-bottom: 0;
}
.organization-selector h2 {
  margin: 0;
  flex-shrink: 0;
}
.organization-selector .section-title > button {
  flex-shrink: 0;
}
/* 组织选择横向续看，组织较多时也不给首屏内容增加整列高度；手机可直接横滑选择。 */
.organization-list {
  display: flex;
  flex: 1;
  min-width: 0;
  gap: 12px;
  list-style: none;
  overflow-x: auto;
  margin: 0;
  padding: 0 0 4px;
}
.organization-list li {
  flex: 0 0 auto;
  max-width: min(280px, 75vw);
}
.organization-choice {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  width: 100%;
  min-width: 130px;
  padding: 9px 14px;
  text-align: left;
  overflow-wrap: anywhere;
}
.organization-choice small {
  color: var(--muted);
  font-size: var(--ui-font-meta);
}
.organization-choice[aria-pressed='true'] {
  color: var(--accent);
  background: var(--soft);
  border-color: var(--accent);
  box-shadow: inset 0 0 0 1px var(--accent);
}
.organization-choice[aria-pressed='true'] span {
  font-weight: 600;
}
/* 组织页已有标题与身份信息，嵌入目录时收紧重复的标题留白，让文档入口留在首屏。 */
.organization-detail-heading {
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 12px;
}
.organization-detail-heading > div:first-child {
  min-width: 0;
  overflow-wrap: anywhere;
}
.organization-header-actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
  flex-shrink: 0;
}
.organization-header-actions :deep(.popover-panel) {
  left: auto;
  right: 0;
  width: min(260px, calc(100vw - 64px));
}
.close-entry {
  width: 100%;
  text-align: left;
}
.organization-detail-heading h2 {
  margin-bottom: 4px;
}
.organization-detail-heading p {
  margin: 0;
}
.organization-number {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px 12px;
  margin-top: 6px;
  color: var(--muted);
  font-size: var(--ui-font-meta);
}
.organization-number button {
  padding: 4px 8px;
  border-color: transparent;
  font-size: inherit;
}
.organization-number code {
  user-select: all;
}
.organization-detail-tabs {
  margin: 0 0 24px;
}
#organization-content > :deep(.section-title) {
  margin-bottom: 16px;
}
#organization-content > :deep(.section-title h3) {
  margin: 0 0 4px;
}
#organization-content > :deep(.section-title p) {
  margin: 0;
}
#organization-content > :deep(.scope-bar) {
  padding: 16px 20px;
  margin-bottom: 16px;
}
.organization-membership {
  scroll-margin-top: 24px;
}
@media (max-width: 700px) {
  .organization-selector {
    padding: 16px;
  }
  .organization-selector .section-title {
    flex-wrap: wrap;
    gap: 12px;
  }
  .organization-list {
    order: 1;
    flex-basis: 100%;
  }
  .organization-detail-heading {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px 12px;
    align-items: center;
  }
  .organization-detail-heading > div:first-child {
    display: contents;
  }
  .organization-detail-heading h2 {
    grid-column: 1;
    grid-row: 1;
    margin: 0;
  }
  .organization-detail-heading p {
    grid-column: 1 / -1;
    grid-row: 2;
  }
  .organization-number {
    grid-column: 1 / -1;
    grid-row: 3;
    margin-top: 0;
  }
  .organization-header-actions {
    grid-column: 2;
    grid-row: 1;
    max-width: 180px;
  }
  .organization-header-actions :deep(.popover-panel) {
    left: 16px;
    right: 16px;
    width: auto;
  }
}
</style>
