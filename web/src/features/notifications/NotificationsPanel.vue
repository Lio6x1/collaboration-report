<script setup lang="ts">
import type { Notification } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import PaginationControls from '../../components/PaginationControls.vue';
import AppIcon from '../../components/AppIcon.vue';
import { useNotifications } from './useNotifications';

const emit = defineEmits<{ open: [notification: Notification] }>();
const {
  action,
  notifications,
  cursor,
  loading,
  readingError,
  blocked,
  refresh,
  pageNav,
  turn,
  markRead,
} = useNotifications();

function open(notification: Notification) {
  if (
    blocked.value ||
    !notification.target_id ||
    !notifications.value.some((item) => item.id === notification.id)
  )
    return;
  // 这里只交给宿主导航，不猜目标路由或读取文档正文；目标页仍须重新核验当前资格。
  emit('open', notification);
}
</script>

<template>
  <section class="notifications-panel panel" aria-label="通知">
    <div class="page-heading">
      <div>
        <span class="eyebrow">协作动态 / INBOX</span>
        <h1>通知</h1>
        <p>跟进讨论、审核与交接，回到需要你关注的内容。</p>
      </div>
      <button type="button" :disabled="blocked" @click="refresh">刷新通知</button>
    </div>
    <ActionStatus
      :busy="action.busy.value"
      :error="action.error.value"
      :notice="action.notice.value"
      :pending="action.pending.value"
      :retry="action.retry.value"
    />
    <p v-if="readingError" class="error" role="alert">{{ readingError }}</p>
    <p v-if="loading" role="status" aria-live="polite">正在读取通知…</p>
    <div v-if="!loading && !notifications.length && !readingError" class="empty-state">
      <span class="empty-icon"><AppIcon name="bell" /></span>
      <h3>{{ cursor ? '本页暂无通知，可继续加载。' : '暂时没有新动态' }}</h3>
      <p>与你相关的讨论、审核和交接通知会出现在这里。</p>
    </div>
    <ul class="notification-list" aria-label="通知列表">
      <li
        v-for="notification in notifications"
        :key="notification.id"
        :class="{ unread: !notification.read_at }"
      >
        <div class="notification-title">
          <strong>{{ notification.title }}</strong>
          <span class="read-state">{{ notification.read_at ? '已读' : '未读' }}</span>
        </div>
        <p class="hint">
          类型：{{ notification.kind
          }}<template v-if="notification.target_id"> · 目标：{{ notification.target_id }}</template>
        </p>
        <div class="button-row">
          <button
            v-if="notification.target_id"
            type="button"
            :disabled="blocked"
            @click="open(notification)"
          >
            打开相关内容
          </button>
          <button
            v-if="!notification.read_at"
            type="button"
            :disabled="blocked"
            @click="markRead(notification)"
          >
            标记已读
          </button>
        </div>
      </li>
    </ul>
    <PaginationControls
      label="通知"
      :page="pageNav.index + 1"
      :has-next="!!pageNav.next"
      :busy="blocked"
      @change="turn"
    />
  </section>
</template>

<style scoped>
.notification-list {
  list-style: none;
  padding: 0;
}
.notification-list li {
  position: relative;
  padding: 22px 24px;
  margin-bottom: 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #fafaf5;
  overflow-wrap: anywhere;
}
.notification-list li.unread {
  background: #eff3e7;
  border-left: 3px solid #7e9459;
}
.notification-title {
  display: flex;
  justify-content: space-between;
  align-items: start;
  gap: 15px;
}
.read-state {
  flex-shrink: 0;
  font-size: var(--ui-font-meta);
  color: var(--muted);
}
.unread .read-state {
  color: var(--accent);
  font-weight: 600;
}
@media (max-width: 560px) {
  .notification-list li {
    padding: 18px 15px;
  }
}
</style>
