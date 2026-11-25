<script setup lang="ts">
import { computed, ref, watch, onUnmounted } from 'vue';
import type {
  Document,
  DocumentInvitation,
  DocumentInvitationCreated,
  DocumentInvitationPage,
  DocumentApplication,
  DocumentApplicationPage,
  InvitationGrant,
  StructureEntry,
} from '../../../../contracts/http/generated';
import { api, useAction, actionsBlocked, ApiError } from '../../lib/http';
import ActionStatus from '../../components/ActionStatus.vue';
import PaginationControls from '../../components/PaginationControls.vue';
import AppDialog from '../../components/AppDialog.vue';
import { accountNumber } from '../account/identity';

const props = defineProps<{
  document: Document;
  entries: StructureEntry[];
  pane: string;
  reload: () => Promise<void>;
}>();
const action = useAction();
const { busy, error, notice, pending, retry } = action;
const invitationStatus = ref('ACTIVE'),
  applicationStatus = ref('PENDING');
const invitations = ref<DocumentInvitation[]>([]),
  applications = ref<DocumentApplication[]>([]);
const invitePages = ref(['']),
  appPages = ref(['']);
const inviteIndex = ref(0),
  appIndex = ref(0);
const inviteNext = ref<string | null>(null),
  appNext = ref<string | null>(null);
const hours = ref(48),
  share = ref(''),
  grantChoice = ref('COMMENT');
const archiveTarget = ref<{
  kind: 'INVITATION' | 'APPLICATION';
  id: string;
  revision: string;
} | null>(null);
const archiveOpen = ref(false);
let generation = 0;
onUnmounted(() => generation++);
const date = (value?: string) => (value ? new Date(value).toLocaleString('zh-CN') : '');
const invitationNames: Record<string, string> = {
  OPEN: '有效',
  CLOSED: '已关闭',
  EXPIRED: '已过期',
};
const applicationNames: Record<string, string> = {
  PENDING: '待处理',
  ACCEPTED: '已同意',
  REJECTED: '已拒绝',
  CANCELLED: '已撤回',
};
const grantOptions = computed(() => [
  {
    value: 'COMMENT',
    label: '整个文档 · 评论',
    grant: {
      scope_kind: 'DOCUMENT',
      scope_id: props.document.id,
      capability: 'COMMENT',
    } as InvitationGrant,
  },
  {
    value: 'REVIEW',
    label: '整个文档 · 审核与评论',
    grant: {
      scope_kind: 'DOCUMENT',
      scope_id: props.document.id,
      capability: 'REVIEW',
    } as InvitationGrant,
  },
  ...props.entries
    .filter((e) => e.kind === 'CHAPTER')
    .map((e) => ({
      value: 'CHAPTER:' + e.chapter_id,
      label: e.title + ' · 整章编辑',
      grant: {
        scope_kind: 'CHAPTER',
        scope_id: e.chapter_id,
        capability: 'EDIT',
      } as InvitationGrant,
    })),
  ...props.entries
    .filter((e) => e.kind !== 'CHAPTER' && e.unit_id)
    .map((e) => ({
      value: 'UNIT:' + e.unit_id,
      label: (e.title || '章前内容') + ' · 小节编辑',
      grant: { scope_kind: 'UNIT', scope_id: e.unit_id!, capability: 'EDIT' } as InvitationGrant,
    })),
]);
function grantText(grants: InvitationGrant[]) {
  return grants
    .map((g) => {
      if (g.scope_kind === 'DOCUMENT') return g.capability === 'REVIEW' ? '审核与评论' : '文档评论';
      const entry = props.entries.find((e) =>
        g.scope_kind === 'CHAPTER'
          ? e.kind === 'CHAPTER' && e.chapter_id === g.scope_id
          : e.unit_id === g.scope_id,
      );
      return (
        (entry?.title || (entry?.kind === 'INTRODUCTION' ? '章前内容' : '原授权范围')) + ' · 编辑'
      );
    })
    .join('、');
}
function clearDeniedPage(failure: unknown, started: number) {
  if (
    started !== generation ||
    !(failure instanceof ApiError) ||
    ![401, 403, 404, 410].includes(failure.status)
  )
    return;
  // 撤权后不能保留旧邀请/申请及分享链接；原未知写入仍由useAction保管，不改其原号或输入。
  invitations.value = [];
  applications.value = [];
  inviteNext.value = appNext.value = null;
  inviteIndex.value = appIndex.value = 0;
  invitePages.value = appPages.value = [''];
  share.value = '';
}
async function loadInvitations(index = 0) {
  const started = ++generation;
  invitations.value = [];
  try {
    const page = await api<DocumentInvitationPage>(
      '/v1/documents/' +
        props.document.id +
        '/invitations?limit=20&status=' +
        invitationStatus.value +
        '&cursor=' +
        encodeURIComponent(invitePages.value[index] ?? ''),
    );
    if (started !== generation) return;
    invitations.value = page.items;
    inviteIndex.value = index;
    inviteNext.value = page.next_cursor;
  } catch (failure) {
    if (started !== generation) return;
    clearDeniedPage(failure, started);
    throw failure;
  }
}
async function loadApplications(index = 0) {
  const started = ++generation;
  applications.value = [];
  try {
    const page = await api<DocumentApplicationPage>(
      '/v1/documents/' +
        props.document.id +
        '/applications?limit=20&status=' +
        applicationStatus.value +
        '&cursor=' +
        encodeURIComponent(appPages.value[index] ?? ''),
    );
    if (started !== generation) return;
    applications.value = page.items;
    appIndex.value = index;
    appNext.value = page.next_cursor;
  } catch (failure) {
    if (started !== generation) return;
    clearDeniedPage(failure, started);
    throw failure;
  }
}
async function refreshCurrent() {
  if (props.pane === 'invitations') {
    invitePages.value = [''];
    await loadInvitations();
  }
  if (props.pane === 'applications') {
    appPages.value = [''];
    await loadApplications();
  }
}
watch(
  [() => props.pane, invitationStatus, applicationStatus],
  () => {
    invitations.value = [];
    applications.value = [];
    inviteNext.value = appNext.value = null;
    inviteIndex.value = appIndex.value = 0;
    void action.run(refreshCurrent, '');
  },
  { immediate: true },
);
function turn(kind: 'invites' | 'apps', direction: number) {
  void action.run(async () => {
    if (kind === 'invites') {
      if (direction > 0 && inviteNext.value)
        invitePages.value[inviteIndex.value + 1] = inviteNext.value;
      await loadInvitations(inviteIndex.value + direction);
    } else {
      if (direction > 0 && appNext.value) appPages.value[appIndex.value + 1] = appNext.value;
      await loadApplications(appIndex.value + direction);
    }
  }, '');
}
async function invite() {
  const grant = grantOptions.value.find((item) => item.value === grantChoice.value)?.grant;
  if (!grant) return;
  await action.run(async () => {
    const result = await action.write<DocumentInvitationCreated>(
      '/v1/documents/' + props.document.id + '/invitations',
      {
        expected_acl_revision: props.document.acl_revision,
        expires_in_hours: Number(hours.value),
        requested_grants: [grant],
      },
    );
    share.value = result.share_url;
    await refreshCurrent();
  }, '邀请已生成，收到链接的人仍须申请并经审批');
}
async function closeInvitation(item: DocumentInvitation) {
  await action.run(async () => {
    await action.mutate('/v1/document-invitations/' + item.id + '/cancel', {
      expected_revision: item.revision,
    });
    await refreshCurrent();
  }, '链接已关闭，已提交的申请仍可处理');
}
async function decide(item: DocumentApplication, decision: 'ACCEPT' | 'REJECT') {
  await action.run(async () => {
    await action.mutate('/v1/document-applications/' + item.id + '/decisions', {
      expected_revision: item.revision,
      decision,
      reason: decision === 'ACCEPT' ? '同意协作' : '本次申请未通过',
    });
    await props.reload();
  }, '申请已处理');
}
function requestArchive(
  kind: 'INVITATION' | 'APPLICATION',
  item: DocumentInvitation | DocumentApplication,
) {
  archiveTarget.value = { kind, id: item.id, revision: item.revision };
  archiveOpen.value = true;
}
async function archive() {
  const target = archiveTarget.value;
  if (!target) return;
  await action.run(async () => {
    await action.mutate(
      '/v1/' +
        (target.kind === 'INVITATION' ? 'document-invitations' : 'document-applications') +
        '/' +
        target.id +
        '/archive',
      { expected_revision: target.revision },
    );
    await refreshCurrent();
    archiveOpen.value = false;
  }, '记录已从管理列表移除，已有授权保持不变');
}
</script>

<template>
  <section v-show="pane === 'invitations'" aria-label="邀请链接管理" class="sharing-panel">
    <div class="section-title">
      <div>
        <h3>邀请链接</h3>
        <p class="hint">指定协作范围，生成链接后分享给对方。</p>
      </div>
      <button :disabled="actionsBlocked" @click="action.run(refreshCurrent, '列表已刷新')">
        刷新邀请
      </button>
    </div>
    <form class="invitation-form" @submit.prevent="invite">
      <fieldset :disabled="actionsBlocked">
        <label
          >邀请授权范围<select v-model="grantChoice" aria-label="邀请授权范围">
            <option v-for="item in grantOptions" :key="item.value" :value="item.value">
              {{ item.label }}
            </option>
          </select></label
        >
        <label>有效小时数<input v-model="hours" type="number" min="1" max="720" required /></label>
        <button>生成邀请链接</button>
      </fieldset>
    </form>
    <label v-if="share" class="share-result"
      >邀请链接<input :value="share" readonly @focus="($event.target as HTMLInputElement).select()"
    /></label>
    <div class="sharing-toolbar">
      <label
        >邀请状态<select
          v-model="invitationStatus"
          aria-label="邀请状态"
          :disabled="actionsBlocked"
        >
          <option value="ACTIVE">有效</option>
          <option value="INACTIVE">已失效</option>
          <option value="ALL">全部</option>
        </select></label
      ><span class="hint">每页最多20条 · 最新在前</span>
    </div>
    <p v-if="!busy && !invitations.length" class="empty">当前筛选下没有邀请链接。</p>
    <ul class="sharing-list">
      <li v-for="item in invitations" :key="item.id">
        <div>
          <strong>{{ grantText(item.requested_grants) }}</strong>
          <p class="hint">
            {{ item.created_at ? '创建于 ' + date(item.created_at) + ' · ' : '' }}到期
            {{ date(item.expires_at) }}
          </p>
        </div>
        <span class="status-pill" :class="{ inactive: item.state !== 'OPEN' }">{{
          invitationNames[item.state]
        }}</span>
        <button
          v-if="item.state === 'OPEN'"
          :disabled="actionsBlocked"
          @click="closeInvitation(item)"
        >
          关闭链接
        </button>
        <button
          v-else
          class="quiet"
          :disabled="actionsBlocked"
          @click="requestArchive('INVITATION', item)"
        >
          删除记录
        </button>
      </li>
    </ul>
    <PaginationControls
      label="邀请"
      :page="inviteIndex + 1"
      :has-next="!!inviteNext"
      :busy="actionsBlocked"
      @change="(index) => turn('invites', index - inviteIndex)"
    />
  </section>
  <section v-show="pane === 'applications'" aria-label="协作申请管理" class="sharing-panel">
    <div class="section-title">
      <div>
        <h3>加入申请</h3>
        <p class="hint">处理新的协作申请，或查看已处理记录。</p>
      </div>
      <button :disabled="actionsBlocked" @click="action.run(refreshCurrent, '列表已刷新')">
        刷新申请
      </button>
    </div>
    <div class="sharing-toolbar">
      <div class="document-tabs" aria-label="申请处理状态">
        <button
          :aria-current="applicationStatus === 'PENDING' ? 'page' : undefined"
          :disabled="actionsBlocked"
          @click="applicationStatus = 'PENDING'"
        >
          待处理</button
        ><button
          :aria-current="applicationStatus === 'PROCESSED' ? 'page' : undefined"
          :disabled="actionsBlocked"
          @click="applicationStatus = 'PROCESSED'"
        >
          已处理
        </button>
      </div>
      <span class="hint">每页最多20条 · 最新在前</span>
    </div>
    <p v-if="!busy && !applications.length" class="empty">
      {{ applicationStatus === 'PENDING' ? '暂无待处理的协作申请。' : '暂无已处理记录。' }}
    </p>
    <ul class="sharing-list">
      <li v-for="item in applications" :key="item.id" class="application-row">
        <span class="member-avatar" aria-hidden="true">{{
          (item.applicant_display_name || '用').slice(0, 1)
        }}</span>
        <div class="application-detail">
          <strong>{{ item.applicant_display_name || '申请人' }}</strong
          ><small class="hint"
            >账号 {{ accountNumber(item.applicant_id)
            }}{{ item.created_at ? ' · ' + date(item.created_at) : '' }}</small
          >
          <p class="application-reason">{{ item.reason }}</p>
          <small class="hint">申请范围：{{ grantText(item.requested_grants) }}</small>
        </div>
        <span class="status-pill" :class="{ inactive: item.state !== 'PENDING' }">{{
          applicationNames[item.state]
        }}</span>
        <div v-if="item.state === 'PENDING'" class="button-row">
          <button :disabled="actionsBlocked" @click="decide(item, 'ACCEPT')">同意</button
          ><button class="quiet" :disabled="actionsBlocked" @click="decide(item, 'REJECT')">
            拒绝
          </button>
        </div>
        <button
          v-else
          class="quiet"
          :disabled="actionsBlocked"
          @click="requestArchive('APPLICATION', item)"
        >
          删除记录
        </button>
      </li>
    </ul>
    <PaginationControls
      label="申请"
      :page="appIndex + 1"
      :has-next="!!appNext"
      :busy="actionsBlocked"
      @change="(index) => turn('apps', index - appIndex)"
    />
  </section>
  <ActionStatus :busy="busy" :error="error" :notice="notice" :pending="pending" :retry="retry" />
  <AppDialog v-model="archiveOpen" title="删除这条记录？" :blocked="busy || !!pending"
    ><p>这条记录将从管理列表移除，审批历史仍保留，已获得的协作权限不会撤销。</p>
    <div class="button-row">
      <button class="quiet" :disabled="actionsBlocked" @click="archiveOpen = false">取消</button
      ><button class="danger" :disabled="actionsBlocked" @click="archive">确认删除记录</button>
    </div>
    <ActionStatus :busy="busy" :error="error" :notice="notice" :pending="pending" :retry="retry"
  /></AppDialog>
</template>

<style scoped>
.sharing-panel {
  padding-top: 24px;
}
.invitation-form fieldset {
  display: grid;
  grid-template-columns: minmax(200px, 1fr) 160px auto;
  align-items: end;
  gap: 16px;
}
.sharing-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 28px 0 8px;
}
.sharing-toolbar label {
  display: flex;
  align-items: center;
  gap: 12px;
}
.sharing-toolbar select {
  width: auto;
  min-width: 130px;
}
.sharing-list {
  padding: 0;
  margin: 0;
  list-style: none;
}
.sharing-list > li {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 22px 0;
  border-bottom: 1px solid var(--line);
}
.sharing-list > li > div:first-child,
.application-detail {
  flex: 1;
  min-width: 0;
}
.sharing-list p {
  margin: 6px 0 0;
  overflow-wrap: anywhere;
}
.application-detail small {
  display: block;
  margin-top: 6px;
}
.application-reason {
  white-space: pre-wrap;
}
.member-avatar {
  display: grid;
  place-items: center;
  flex: 0 0 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--soft);
  color: var(--accent);
}
.status-pill {
  white-space: nowrap;
}
.status-pill.inactive {
  background: var(--soft);
  color: var(--muted);
}
.sharing-pagination {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 16px;
  margin-top: 20px;
}
@media (max-width: 760px) {
  .invitation-form fieldset {
    grid-template-columns: 1fr;
  }
  .sharing-list > li {
    flex-wrap: wrap;
    gap: 12px;
  }
  .sharing-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
  .application-detail {
    flex-basis: calc(100% - 60px);
  }
  .sharing-pagination {
    justify-content: center;
  }
}
</style>
