<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, defineAsyncComponent } from 'vue';
import type { Notification } from '../../contracts/http/generated';
const AccountPanel = defineAsyncComponent(() => import('./features/account/AccountPanel.vue'));
const OrganizationPanel = defineAsyncComponent(
  () => import('./features/organization/OrganizationPanel.vue'),
);
const WorkspacePanel = defineAsyncComponent(
  () => import('./features/workspace/WorkspacePanel.vue'),
);
const PublicReader = defineAsyncComponent(
  () => import('./features/public-reading/PublicReader.vue'),
);
const SearchPanel = defineAsyncComponent(() => import('./features/search/SearchPanel.vue'));
const RetainedGovernancePanel = defineAsyncComponent(
  () => import('./features/governance/RetainedGovernancePanel.vue'),
);
const PersonalTransfersPanel = defineAsyncComponent(
  () => import('./features/transfers/PersonalTransfersPanel.vue'),
);
const NotificationsPanel = defineAsyncComponent(
  () => import('./features/notifications/NotificationsPanel.vue'),
);
const DocumentReader = defineAsyncComponent(
  () => import('./features/workspace/DocumentReader.vue'),
);
const CommentDestination = defineAsyncComponent(
  () => import('./features/notifications/CommentDestination.vue'),
);
const ReviewDestination = defineAsyncComponent(
  () => import('./features/notifications/ReviewDestination.vue'),
);
import { publicDocumentFromPath } from './features/public-reading/api';
import { canonicalID } from './features/search/rules';
import { documentRoute, documentPath, rememberWorkspace } from './features/workspace/navigation';
const currentDocument = ref(documentRoute(location.pathname));
const publicDocument = publicDocumentFromPath(location.pathname);
import ActionStatus from './components/ActionStatus.vue';
import AppIcon from './components/AppIcon.vue';
import {
  user,
  sessionReady,
  identityUnavailable,
  refreshSession,
  logout,
} from './features/account/session';
import {
  useAction,
  actionsBlocked,
  recoveryAction,
  transportEpoch,
  sessionContextID,
} from './lib/http';
import { closureVisible } from './features/account/accountClosure';
const closureOrganization = ref('');
const closureKnowledgeBase = ref('');
const moreMenu = ref<HTMLDetailsElement | null>(null);
function closeMenu(event?: Event) {
  if (event?.type === 'pointerdown' && moreMenu.value?.contains(event.target as Node)) return;
  if (event instanceof KeyboardEvent && event.key !== 'Escape') return;
  // 原生 details 保留键盘展开能力；离开菜单或导航后关闭，避免悬浮层遮挡新页面。
  if (moreMenu.value?.open) {
    moreMenu.value.open = false;
    if (!event || event.type === 'keydown') moreMenu.value.querySelector('summary')?.focus();
  }
}
function focusContent() {
  // 跳过导航只改变焦点与滚动，不写入 hash 历史，避免触发路由恢复并清空当前搜索。
  const content = document.getElementById('main-content');
  content?.focus({ preventScroll: true });
  content?.scrollIntoView({ block: 'start' });
}
function openSuccession(kind: string, target: string) {
  navigate(kind === 'KNOWLEDGE_BASE_ADMIN' ? 'workspace' : 'organization');
  closureOrganization.value = kind === 'KNOWLEDGE_BASE_ADMIN' ? '' : target;
  closureKnowledgeBase.value = kind === 'KNOWLEDGE_BASE_ADMIN' ? target : '';
}
const view = ref(
    location.pathname === '/account'
      ? 'account'
      : location.pathname === '/transfers'
        ? 'transfers'
        : location.pathname === '/governance'
          ? 'governance'
          : location.pathname === '/search'
            ? 'search'
            : location.pathname === '/notifications'
              ? 'notifications'
              : ['/join', '/organizations'].includes(location.pathname)
                ? 'organization'
                : 'workspace',
  ),
  action = useAction();
const initialSpace = new URLSearchParams(location.search).get('space_id') ?? '';
const searchSpace = ref(canonicalID(initialSpace) ? initialSpace : '');
const searchVisit = ref(0);
const notificationDocument = ref('');
const notificationThread = ref('');
const notificationReview = ref('');
const notificationOrganization = ref('');
const notificationNotice = ref('');
function returnToNotifications() {
  notificationOrganization.value = '';
  notificationReview.value = '';
  notificationDocument.value = '';
  notificationThread.value = '';
}
function openNotification(notification: Notification) {
  if (!user.value || actionsBlocked.value) return;
  notificationNotice.value = '';
  if (notification.kind === 'ORG_OWNER_TRANSFER') {
    if (!notification.target_id || !canonicalID(notification.target_id)) {
      notificationNotice.value = '通知中的组织编号无效。';
      return;
    }
    notificationOrganization.value = notification.target_id;
    return;
  }
  if (
    ['REVIEW_SUBMITTED', 'REVIEW_RESUBMITTED', 'REVIEW_ACTION', 'REVIEW_MERGED'].includes(
      notification.kind,
    )
  ) {
    if (!canonicalID(notification.id)) {
      notificationNotice.value = '通知编号无效，无法打开原审核轮次。';
      return;
    }
    notificationReview.value = notification.id;
    return;
  }
  if (['COMMENT_MENTION', 'COMMENT_REPLY'].includes(notification.kind)) {
    if (!notification.target_id || !canonicalID(notification.target_id)) {
      notificationNotice.value = '通知目标无效，无法打开原讨论。';
      return;
    }
    notificationThread.value = notification.target_id;
    return;
  }
  // 各事件的目标类型不同；线程由详情接口解析归属，不能误当documentId。
  if (!['RESPONSIBLE_TRANSFER', 'RESPONSIBLE_DEPARTURE'].includes(notification.kind)) {
    notificationNotice.value = '该类型通知暂不支持直接打开目标（' + notification.kind + '）';
    return;
  }
  if (!notification.target_id || !canonicalID(notification.target_id)) {
    notificationNotice.value = '通知目标无效，无法打开相关内容。';
    return;
  }
  notificationDocument.value = notification.target_id;
}
watch(transportEpoch, () => {
  notificationOrganization.value = '';
  notificationReview.value = '';
  notificationDocument.value = '';
  notificationThread.value = '';
  notificationNotice.value = '';
});
function navigate(next: string) {
  closeMenu();
  currentDocument.value = null;
  closureOrganization.value = '';
  closureKnowledgeBase.value = '';
  notificationOrganization.value = '';
  notificationReview.value = '';
  notificationDocument.value = '';
  notificationThread.value = '';
  notificationNotice.value = '';
  view.value = next;
  history.pushState(
    null,
    '',
    next === 'account'
      ? '/account'
      : next === 'transfers'
        ? '/transfers'
        : next === 'organization'
          ? '/organizations'
          : next === 'governance'
            ? '/governance'
            : next === 'search'
              ? '/search'
              : next === 'notifications'
                ? '/notifications'
                : '/',
  );
}
function openSearch(space = '') {
  closeMenu();
  currentDocument.value = null;
  searchSpace.value = space;
  searchVisit.value++;
  view.value = 'search';
  history.pushState(null, '', '/search' + (space ? '?space_id=' + encodeURIComponent(space) : ''));
}
function restoreView() {
  closeMenu();
  currentDocument.value = documentRoute(location.pathname);
  notificationOrganization.value = '';
  notificationReview.value = '';
  notificationDocument.value = '';
  notificationThread.value = '';
  notificationNotice.value = '';
  view.value =
    location.pathname === '/account'
      ? 'account'
      : location.pathname === '/transfers'
        ? 'transfers'
        : location.pathname === '/governance'
          ? 'governance'
          : location.pathname === '/search'
            ? 'search'
            : location.pathname === '/notifications'
              ? 'notifications'
              : ['/join', '/organizations'].includes(location.pathname)
                ? 'organization'
                : 'workspace';
  const space = new URLSearchParams(location.search).get('space_id') ?? '';
  searchSpace.value = canonicalID(space) ? space : '';
  searchVisit.value++;
}
const { busy, error, notice, pending, retry } = action;
function openDocument(id: string) {
  if (!canonicalID(id)) return;
  if (user.value) rememberWorkspace(user.value.id, sessionContextID());
  currentDocument.value = { id, editor: undefined };
  view.value = 'workspace';
  history.pushState(null, '', documentPath(id));
  window.scrollTo({ top: 0, behavior: 'instant' });
}
let timer: ReturnType<typeof setInterval>;
function focus() {
  void refreshSession();
}
function signOut() {
  const operationId = crypto.randomUUID();
  void action.run(() => logout(operationId), '旧会话与输出已关闭');
}
onMounted(() => {
  void refreshSession();
  timer = setInterval(() => {
    // 用户界面只续验会话。服务运维检查留给本机/服务器管理工具，不为每位访问者轮询。
    if (user.value) void refreshSession();
  }, 10000);
  window.addEventListener('focus', focus);
  window.addEventListener('popstate', restoreView);
  window.addEventListener('pointerdown', closeMenu);
  window.addEventListener('keydown', closeMenu);
});
onUnmounted(() => {
  clearInterval(timer);
  window.removeEventListener('focus', focus);
  window.removeEventListener('popstate', restoreView);
  window.removeEventListener('pointerdown', closeMenu);
  window.removeEventListener('keydown', closeMenu);
});
</script>
<template>
  <div class="app-shell">
    <a class="skip-link" href="#main-content" @click.prevent="focusContent">跳到页面内容</a>
    <header class="app-header">
      <a href="/" @click="actionsBlocked && $event.preventDefault()" class="brand"
        ><span class="brand-mark" aria-hidden="true"><AppIcon name="page" /></span
        ><span>协作报告<small>COLLABORATION REPORT</small></span></a
      >
      <nav v-if="!publicDocument" aria-label="主导航">
        <button
          v-if="user"
          :disabled="actionsBlocked"
          @click="navigate('workspace')"
          :class="{ active: view === 'workspace' }"
        >
          <AppIcon name="page" />工作台</button
        ><button
          :disabled="actionsBlocked"
          @click="openSearch()"
          :class="{ active: view === 'search' }"
        >
          <AppIcon name="search" />正式版搜索</button
        ><button
          v-if="user"
          :disabled="actionsBlocked"
          @click="navigate('organization')"
          :class="{ active: view === 'organization' }"
        >
          <AppIcon name="people" />组织</button
        ><button
          v-if="user"
          :disabled="actionsBlocked"
          @click="navigate('notifications')"
          :class="{ active: view === 'notifications' }"
        >
          <AppIcon name="bell" />通知
        </button>
        <details v-if="user" ref="moreMenu" class="nav-more">
          <summary>更多</summary>
          <div>
            <button
              v-if="user"
              :disabled="actionsBlocked"
              @click="navigate('account')"
              :class="{ active: view === 'account' }"
            >
              账号设置
            </button>
            <button
              v-if="user"
              :disabled="actionsBlocked"
              @click="navigate('governance')"
              :class="{ active: view === 'governance' }"
            >
              留存治理
            </button>
            <button
              v-if="user"
              :disabled="actionsBlocked"
              @click="navigate('transfers')"
              :class="{ active: view === 'transfers' }"
            >
              个人转让
            </button>
          </div>
        </details>
      </nav>
      <div v-if="user" class="profile">
        <span class="profile-avatar" aria-hidden="true">{{ [...user.display_name][0] }}</span>
        <span class="profile-name">{{ user.display_name }}</span
        ><button :disabled="actionsBlocked" @click="signOut">退出登录</button>
      </div>
    </header>
    <main id="main-content" tabindex="-1">
      <div v-if="actionsBlocked" class="activity-banner" role="status" aria-live="polite">
        <span class="activity-dot" aria-hidden="true"></span>
        {{ recoveryAction ? '操作结果待确认，请续接原操作' : '正在处理，请稍候…' }}
      </div>
      <p v-if="closureVisible && user && view !== 'account' && !publicDocument" class="hint">
        本账号已有注销申请，交接后可继续查询原操作。
        <span v-if="closureKnowledgeBase">需交接的知识库编号：{{ closureKnowledgeBase }}。</span>
        <button type="button" :disabled="actionsBlocked" @click="navigate('account')">
          返回注销进度
        </button>
      </p>
      <p v-if="user && identityUnavailable" class="pending" role="status">
        连接不可用：当前资格待重新核验，离线输入仅保留在本机。
      </p>
      <ActionStatus
        v-if="recoveryAction && recoveryAction !== action"
        :busy="recoveryAction.busy.value"
        :error="recoveryAction.error.value"
        :notice="recoveryAction.notice.value"
        :pending="recoveryAction.pending.value"
        :retry="recoveryAction.retry.value"
      /><ActionStatus
        :busy="busy"
        :error="error"
        :notice="notice"
        :pending="pending"
        :retry="retry"
      />
      <PublicReader v-if="publicDocument" :document="publicDocument" />
      <p v-else-if="!sessionReady" class="empty">正在核验会话…</p>
      <SearchPanel v-else-if="view === 'search'" :key="searchVisit" :initial-space="searchSpace" />
      <AccountPanel v-else-if="!user" @succession="openSuccession" /><template v-else
        ><DocumentReader
          v-if="currentDocument"
          :key="
            user.id +
            ':' +
            currentDocument.id +
            ':' +
            (currentDocument.editor?.kind ?? '') +
            ':' +
            (currentDocument.editor?.unit ?? '')
          "
          :id="currentDocument.id"
          :editor-target="currentDocument.editor"
          @deleted="navigate('workspace')" /><WorkspacePanel
          v-else-if="view === 'workspace'"
          :key="user.id"
          @search="openSearch"
          @open="openDocument" /><OrganizationPanel
          v-else-if="view === 'organization'"
          :key="user.id + ':' + closureOrganization"
          :initial-organization="closureOrganization"
          @open="openDocument"
          @search="openSearch" />
        <section v-else-if="view === 'notifications'" aria-label="通知中心">
          <template v-if="notificationOrganization">
            <button type="button" :disabled="actionsBlocked" @click="returnToNotifications">
              返回通知
            </button>
            <OrganizationPanel
              :key="user.id + ':' + notificationOrganization"
              :initial-organization="notificationOrganization"
              @open="openDocument"
              @search="openSearch"
            />
          </template>
          <template v-else-if="notificationReview">
            <button type="button" :disabled="actionsBlocked" @click="returnToNotifications">
              返回通知
            </button>
            <ReviewDestination
              :key="user.id + ':' + notificationReview"
              :notification="notificationReview"
            />
          </template>
          <template v-else-if="notificationThread">
            <button type="button" :disabled="actionsBlocked" @click="returnToNotifications">
              返回通知
            </button>
            <CommentDestination
              :key="user.id + ':' + notificationThread"
              :thread="notificationThread"
            />
          </template>
          <template v-else-if="notificationDocument">
            <button type="button" :disabled="actionsBlocked" @click="returnToNotifications">
              返回通知
            </button>
            <DocumentReader
              :key="user.id + ':' + notificationDocument"
              :id="notificationDocument"
              @changed="returnToNotifications"
              @deleted="returnToNotifications"
            />
          </template>
          <template v-else>
            <p v-if="notificationNotice" class="hint" role="status">{{ notificationNotice }}</p>
            <NotificationsPanel :key="user.id" @open="openNotification" />
          </template>
        </section>
        <RetainedGovernancePanel v-else-if="view === 'governance'" :key="user.id" />
        <PersonalTransfersPanel v-else-if="view === 'transfers'" :key="user.id" />
        <AccountPanel v-else :key="user.id" @succession="openSuccession"
      /></template>
    </main>
    <footer>协作报告 · 让每一次共同创作都有迹可循</footer>
  </div>
</template>
