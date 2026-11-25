<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import type { Anchor, Comment, CommentThread } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import PaginationControls from '../../components/PaginationControls.vue';
import { useComments, type CommentsTarget } from './useComments';
import MentionPicker from './MentionPicker.vue';
import CommentAnchorPreview from './CommentAnchorPreview.vue';
import { user } from '../account/session';
import { accountNumber } from '../account/identity';

const props = defineProps<
  CommentsTarget & {
    locatable?: boolean;
    compact?: boolean;
    visible?: boolean;
    locations?: Record<
      string,
      { label: string; kind: 'current' | 'matched' | 'historical' | 'unplaced' }
    >;
  }
>();
const emit = defineEmits<{
  'open-anchor': [anchor: Anchor, thread?: string];
  'locate-anchor': [anchor: Anchor];
  'clear-anchor': [];
  'threads-changed': [threads: CommentThread[]];
}>();
const {
  action,
  threads,
  selected,
  comments,
  threadCursor,
  threadPage,
  turnThreads,
  commentCursor,
  loadingThreads,
  loadingComments,
  readingError,
  previews,
  loadPreview,
  newBody,
  newMentions,
  replyMentions,
  replyBody,
  replyTo,
  editing,
  editBody,
  busy,
  canReply,
  canModerate,
  canEdit,
  canDelete,
  refresh,
  moreComments,
  open,
  create,
  reply,
  beginEdit,
  saveEdit,
  remove,
  setState,
} = useComments(props);
watch(selected, (thread) => {
  if (thread) emit('open-anchor', thread.anchor, thread.id);
});
// 只把已经过列表接口授权的线程交给阅读页画位置标记；标记本身不读取评论正文或历史正文。
watch(threads, (value) => emit('threads-changed', value), { immediate: true });
const stateLabel = (state: string) =>
  state === 'OPEN' ? '进行中' : state === 'RESOLVED' ? '已解决' : '已删除';
const threadDetail = ref<HTMLElement | null>(null);
const panel = ref<HTMLElement | null>(null);
const composingComment = ref(false);
const repliesExpanded = ref(false);
const replyComposerOpen = ref(false);
const replyInput = ref<HTMLTextAreaElement | null>(null);
// 线程首条是原评论，其余按服务端原顺序作为同一讨论下的回复；不伪造接口未返回的楼中楼关系。
const visibleComments = computed(() =>
  repliesExpanded.value ? comments.value : comments.value.slice(0, 1),
);
const authorName = (id: string) =>
  id === props.actor ? user.value?.display_name || '我' : '协作者 · ' + accountNumber(id);
const avatarText = (id: string) => (id === props.actor ? Array.from(authorName(id))[0] : '协');
watch(
  () => selected.value?.id,
  () => {
    repliesExpanded.value = false;
    replyComposerOpen.value = false;
  },
);
async function startReply(comment?: Comment) {
  if (busy.value || !canReply.value) return;
  replyTo.value = comment ?? null;
  replyComposerOpen.value = true;
  await nextTick();
  replyInput.value?.focus();
}
async function submitReply() {
  await reply();
  // 只有原写入确认并清空草稿后才收起输入，未知结果或失败时继续保留恢复入口。
  if (!replyBody.value && !action.pending.value && !action.error.value) {
    repliesExpanded.value = true;
    replyComposerOpen.value = false;
  }
}
const detailFallback = ref<HTMLElement | null>(null);
const detailPrefix = 'comment-detail-' + crypto.randomUUID() + '-';
let observer: IntersectionObserver | undefined;
watch(
  () => props.anchor,
  (anchor) => {
    if (anchor?.block_id) composingComment.value = true;
  },
);
watch(
  [threads, () => props.visible],
  async (_value, _old, cleanup) => {
    let cancelled = false;
    cleanup(() => {
      cancelled = true;
      observer?.disconnect();
    });
    observer?.disconnect();
    await nextTick();
    if (cancelled || !panel.value || props.visible === false) return;
    observer = new IntersectionObserver((entries) => {
      for (const entry of entries)
        if (entry.isIntersecting) {
          const thread = threads.value.find(
            (item) => item.id === (entry.target as HTMLElement).dataset.threadId,
          );
          if (thread) loadPreview(thread);
          observer?.unobserve(entry.target);
        }
    });
    panel.value.querySelectorAll('[data-thread-id]').forEach((card) => observer!.observe(card));
  },
  { flush: 'post' },
);
onBeforeUnmount(() => observer?.disconnect());
const positionLabel = (thread: CommentThread) =>
  props.locations?.[thread.id]?.label ?? (thread.anchor.quote ? '引用原文' : '整篇讨论');
const canLocate = (thread: CommentThread) =>
  props.locations
    ? ['current', 'matched'].includes(props.locations[thread.id]?.kind ?? '')
    : !!thread.anchor.block_id;
async function chooseThread(thread: CommentThread) {
  const alreadySelected = selected.value?.id === thread.id;
  await open(thread);
  // 重选当前卡片也要恢复正文定位，但不能重读线程并清空尚未提交的回复。
  if (alreadySelected && selected.value?.id === thread.id)
    emit('open-anchor', thread.anchor, thread.id);
}
// 父阅读页只需知道是否有未提交输入，不读取评论正文，也不为自动焦点刷新销毁草稿。
defineExpose({
  beginComment: () => {
    composingComment.value = true;
  },
  hasDraft: computed(() =>
    Boolean(newBody.value.trim() || replyBody.value.trim() || editing.value),
  ),
  // 正文标记只打开本组件已经读到的线程，仍由原来的评论读取、权限及草稿流程处理。
  openThread: async (id: string) => {
    const thread = threads.value.find((item) => item.id === id);
    if (!thread) return;
    await chooseThread(thread);
    await nextTick();
    // 从正文标记进入时直接带到讨论内容，不能让侧栏顶部的新建表单挡住已经选中的评论。
    threadDetail.value?.scrollIntoView({ block: 'start' });
  },
});
</script>

<template>
  <section ref="panel" class="comments-panel" :class="{ compact }" aria-label="普通批注与讨论">
    <div class="section-title">
      <h3>
        {{ compact ? '评论' : '普通批注与讨论' }}
        <span v-if="!threadOnly" class="thread-count">本页 {{ threads.length }} 条</span>
      </h3>
      <button type="button" :disabled="busy" @click="refresh">刷新讨论</button>
    </div>
    <details v-if="!compact" class="comment-help">
      <summary>关于评论与原文</summary>
      <p class="hint">
        {{ threadOnly ? '原讨论' : '新讨论' }}对应固定正文版本
        {{ version }}，不会作为正式修改要求阻止审核通过。
      </p>
      <p v-if="browseMainHistory && contextKind === 'MAIN'" class="hint">
        这里汇总本文档历次主线讨论。刷新正文不会改变旧讨论的原始锚点；上方单元选择只决定新讨论的位置。
      </p>
    </details>
    <details
      v-if="canComment && !threadOnly"
      class="comment-composer"
      :open="!compact || composingComment"
    >
      <summary
        v-show="compact"
        role="button"
        :aria-expanded="composingComment"
        @click.prevent="composingComment = !composingComment"
      >
        {{ composingComment ? '收起新评论' : '新增评论' }}
      </summary>
      <form @submit.prevent="create">
        <slot name="creation-context"></slot>
        <p v-if="anchor?.block_id" class="hint" aria-label="新讨论选区锚点">
          评论引用
          <q v-if="anchor.quote">{{ anchor.quote }}</q>
          <button type="button" :disabled="busy" @click="emit('clear-anchor')">清除选区锚点</button>
        </p>
        <p v-else class="hint">可先在正文中选择同一段文字，将新讨论定位到该选区。</p>
        <label
          >对这份正文发起讨论<textarea
            v-model="newBody"
            :disabled="busy"
            maxlength="8000"
            rows="3"
            placeholder="写下问题或建议"
          ></textarea>
        </label>
        <MentionPicker v-model="newMentions" :document="document" :actor="actor" :disabled="busy" />
        <button type="submit" :disabled="busy || !newBody.trim() || !version">发起讨论</button>
      </form>
    </details>
    <p v-else-if="!canComment" class="hint">当前仅可阅读讨论。</p>
    <ActionStatus
      :busy="action.busy.value"
      :error="action.error.value"
      :notice="action.notice.value"
      :pending="action.pending.value"
      :retry="action.retry.value"
    />
    <p v-if="readingError" class="error" role="alert">{{ readingError }}</p>
    <p v-if="loadingThreads || loadingComments" role="status">正在读取讨论…</p>
    <p v-if="!threadOnly && !threads.length && !loadingThreads" class="hint">
      {{ threadCursor ? '本页没有当前上下文的讨论，可继续加载。' : '当前上下文暂无讨论。' }}
    </p>
    <div ref="detailFallback"></div>
    <ul v-if="!threadOnly" class="thread-list" aria-label="讨论列表">
      <li
        v-for="(thread, index) in threads"
        :key="thread.id"
        :data-thread-id="thread.id"
        :class="{ 'selected-thread': selected?.id === thread.id }"
      >
        <button
          type="button"
          class="thread-card"
          :aria-label="
            compact
              ? '打开评论 ' + thread.id
              : '讨论 ' +
                thread.id +
                ' · ' +
                stateLabel(thread.status) +
                ' · ' +
                (thread.author_id === actor ? '我' : '用户 ' + accountNumber(thread.author_id))
          "
          :disabled="busy"
          :aria-pressed="selected?.id === thread.id"
          @click="chooseThread(thread)"
        >
          <span v-if="selected?.id !== thread.id" class="thread-card-top"
            ><span class="comment-avatar" aria-hidden="true">{{
              avatarText(thread.author_id)
            }}</span
            ><span class="comment-author">{{ authorName(thread.author_id) }}</span
            ><span class="thread-state" :class="{ resolved: thread.status === 'RESOLVED' }">{{
              stateLabel(thread.status)
            }}</span></span
          >
          <span class="thread-position"
            ><span class="thread-number">{{ index + 1 }}</span
            >{{ positionLabel(thread) }}</span
          >
          <q v-if="thread.anchor.quote" class="thread-quote">{{ thread.anchor.quote }}</q>
          <span v-if="selected?.id !== thread.id" class="thread-excerpt">{{
            thread.status === 'DELETED'
              ? '此讨论已删除'
              : previews[thread.id]?.error
                ? '暂时无法读取，点击重试'
                : previews[thread.id]?.loading || !previews[thread.id]
                  ? '正在读取评论…'
                  : previews[thread.id]?.body || '此评论已删除或暂无内容'
          }}</span>
          <span v-if="selected?.id !== thread.id" class="thread-card-footer">查看回复</span>
        </button>
        <div :id="detailPrefix + thread.id"></div>
      </li>
    </ul>
    <PaginationControls
      v-if="!threadOnly"
      label="评论"
      :page="threadPage.index + 1"
      :has-next="!!threadPage.next"
      :busy="busy"
      @change="turnThreads"
    />
    <p
      v-if="selected && !threadOnly && !threads.some((thread) => thread.id === selected?.id)"
      class="hint"
    >
      正在查看之前选中的评论，未提交的回复继续保留在原讨论中。
    </p>

    <Teleport
      v-if="selected && detailFallback"
      :to="
        threads.some((thread) => thread.id === selected?.id)
          ? '#' + detailPrefix + selected.id
          : detailFallback
      "
    >
      <section ref="threadDetail" class="thread-detail" aria-label="当前讨论">
        <h4 class="accessible-title">讨论 {{ selected.id }} · {{ stateLabel(selected.status) }}</h4>
        <div class="thread-tools">
          <button
            v-if="locatable && canLocate(selected)"
            class="text-action"
            type="button"
            :disabled="busy"
            @click="emit('locate-anchor', selected.anchor)"
          >
            定位原文
          </button>
          <CommentAnchorPreview :key="selected.id" :thread="selected" :show-quote="false" />
          <details class="comment-menu thread-menu">
            <summary role="button" aria-label="讨论更多操作">···</summary>
            <div class="comment-menu-content">
              <template v-if="canModerate && selected.status !== 'DELETED'">
                <button
                  type="button"
                  :disabled="busy"
                  @click="setState(selected.status === 'OPEN' ? 'RESOLVED' : 'OPEN')"
                >
                  {{ selected.status === 'OPEN' ? '标记已解决' : '重新打开讨论' }}
                </button>
                <button
                  class="destructive-action"
                  type="button"
                  :disabled="busy"
                  @click="setState('DELETED')"
                >
                  删除整个讨论
                </button>
              </template>
              <details class="comment-help">
                <summary>版本信息</summary>
                <p>
                  原始正文版本 {{ selected.anchor.content_version_id }} · 原始{{
                    selected.context_kind === 'MAIN'
                      ? '主线清单'
                      : selected.context_kind === 'REVIEW'
                        ? '审核轮次'
                        : '草稿'
                  }}
                  {{ selected.context_id }}
                </p>
              </details>
            </div>
          </details>
        </div>
        <p v-if="selected.status === 'DELETED'" class="hint">此讨论已删除。</p>
        <template v-else>
          <ol class="comment-list" aria-label="讨论内容">
            <li
              v-for="(comment, index) in visibleComments"
              :key="comment.id"
              :data-comment-id="comment.id"
              class="comment-item"
              :class="{ 'reply-item': index > 0 }"
            >
              <span class="comment-avatar" aria-hidden="true">{{
                avatarText(comment.author_id)
              }}</span>
              <div class="comment-content">
                <div class="comment-byline">
                  <span class="comment-author">{{ authorName(comment.author_id) }}</span
                  ><span v-if="index === 0" class="thread-state">{{
                    stateLabel(selected.status)
                  }}</span>
                </div>
                <p v-if="comment.body === ''" class="hint">此批注已删除。</p>
                <form v-else-if="editing?.id === comment.id" @submit.prevent="saveEdit">
                  <label
                    >修改批注<textarea
                      v-model="editBody"
                      :disabled="busy"
                      maxlength="8000"
                      rows="3"
                    ></textarea>
                  </label>
                  <button type="submit" :disabled="busy || !editBody.trim()">保存修改</button>
                  <button type="button" :disabled="busy" @click="editing = null">取消修改</button>
                </form>
                <p v-else class="comment-body">{{ comment.body }}</p>
                <div
                  v-if="comment.body !== '' && editing?.id !== comment.id"
                  class="comment-actions"
                >
                  <button
                    v-if="canReply"
                    class="text-action"
                    type="button"
                    :disabled="busy"
                    aria-label="回复这条批注"
                    @click="startReply(comment)"
                  >
                    回复
                  </button>
                  <details v-if="canEdit(comment) || canDelete(comment)" class="comment-menu">
                    <summary role="button" aria-label="评论更多操作">···</summary>
                    <div class="comment-menu-content">
                      <button
                        v-if="canEdit(comment)"
                        type="button"
                        :disabled="busy"
                        @click="beginEdit(comment)"
                      >
                        修改
                      </button>
                      <button
                        v-if="canDelete(comment)"
                        class="destructive-action"
                        type="button"
                        :disabled="busy"
                        @click="remove(comment)"
                      >
                        删除批注
                      </button>
                    </div>
                  </details>
                </div>
              </div>
            </li>
          </ol>
          <button
            v-if="comments.length > 1 || commentCursor"
            class="text-action replies-toggle"
            type="button"
            :aria-expanded="repliesExpanded"
            @click="repliesExpanded = !repliesExpanded"
          >
            {{
              repliesExpanded
                ? '收起回复'
                : '查看回复（' +
                  Math.max(0, comments.length - 1) +
                  (commentCursor ? '+' : '') +
                  '）'
            }}
          </button>
          <button
            v-if="repliesExpanded && commentCursor"
            class="text-action replies-toggle"
            type="button"
            :disabled="busy"
            @click="
              repliesExpanded = true;
              moreComments();
            "
          >
            加载更多回复
          </button>
          <button
            v-if="canReply && !replyComposerOpen && !replyBody.trim()"
            type="button"
            class="reply-placeholder"
            :disabled="busy"
            @click="startReply()"
          >
            写下你的回复…
          </button>
          <form
            v-if="canReply && (replyComposerOpen || replyBody.trim())"
            class="reply-composer"
            @submit.prevent="submitReply"
          >
            <p v-if="replyTo" class="hint">
              回复 {{ authorName(replyTo.author_id)
              }}<q class="reply-context">{{ replyTo.body.slice(0, 80) }}</q>
              <button type="button" :disabled="busy" @click="replyTo = null">取消指定回复</button>
            </p>
            <label
              >发表批注<textarea
                ref="replyInput"
                v-model="replyBody"
                :disabled="busy"
                maxlength="8000"
                rows="3"
              ></textarea>
            </label>
            <details class="comment-help">
              <summary>@ 提及协作者</summary>
              <MentionPicker
                v-model="replyMentions"
                :key="selected.id"
                :document="document"
                :actor="actor"
                :disabled="busy"
              />
            </details>
            <div class="composer-actions">
              <button type="submit" :disabled="busy || !replyBody.trim()">发表批注</button>
              <button
                type="button"
                :disabled="busy"
                @click="
                  replyComposerOpen = false;
                  replyTo = null;
                "
              >
                {{ replyBody.trim() ? '收起输入' : '取消' }}
              </button>
            </div>
          </form>
          <p v-else-if="selected.status === 'RESOLVED'" class="hint">
            讨论已解决，重新打开后可继续回复。
          </p>
        </template>
      </section>
    </Teleport>
  </section>
</template>

<style scoped>
.comments-panel {
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid var(--line);
}
.compact {
  border-top: 0;
  padding-top: 0;
  margin-top: 12px;
}
.comments-panel label {
  display: block;
  margin: 12px 0;
}
.comments-panel textarea {
  display: block;
  width: 100%;
  margin-top: 6px;
}
.thread-list,
.comment-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.thread-list > li {
  padding: 20px 0;
  border-bottom: 1px solid var(--line);
}
.thread-card {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
  width: 100%;
  padding: 6px;
  text-align: left;
  border: 0;
  border-radius: 8px;
  background: transparent;
  box-shadow: none;
  color: var(--ink);
  font-size: var(--ui-font-control);
  line-height: 1.65;
}
.thread-card:hover {
  background: var(--soft);
  transform: none;
}
.selected-thread > .thread-card {
  background: #f3f6ef;
}
.thread-card-top,
.comment-byline,
.thread-position,
.comment-actions,
.thread-tools,
.composer-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}
.comment-avatar {
  display: inline-grid;
  place-items: center;
  flex: 0 0 34px;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: #e4ece3;
  color: var(--accent);
  font-size: 16px;
  font-weight: 600;
}
.comment-author {
  color: var(--accent);
  font-size: var(--ui-font-meta);
  font-weight: 600;
  overflow-wrap: anywhere;
}
.thread-number {
  display: inline-grid;
  place-items: center;
  flex: 0 0 22px;
  height: 22px;
  border-radius: 6px;
  background: #fff2c3;
  color: #675018;
  font-size: 13px;
  font-weight: 600;
}
.thread-state,
.thread-count {
  font-size: var(--ui-font-meta);
  font-weight: 400;
  color: var(--muted);
}
.thread-state {
  margin-left: auto;
  flex-shrink: 0;
}
.thread-position,
.thread-card-footer {
  color: var(--muted);
  font-size: var(--ui-font-meta);
}
.thread-card-footer {
  color: var(--accent);
}
.thread-quote {
  border-left: 2px solid #d9ae42;
  padding: 3px 10px;
  background: #fff9e9;
  color: #675018;
  quotes: none;
  font-size: var(--ui-font-meta);
}
.thread-quote,
.thread-excerpt {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 4;
  overflow: hidden;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}
.thread-detail {
  position: relative;
  padding: 0 6px;
  margin-top: 12px;
}
.accessible-title {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}
.thread-tools {
  flex-wrap: wrap;
  justify-content: space-between;
  margin-bottom: 14px;
}
.thread-tools > .anchor-preview {
  flex: 1 1 130px;
  min-width: 0;
}
.thread-tools > .anchor-preview:has(.ProseMirror) {
  flex-basis: 100%;
  order: 3;
}
.thread-tools :deep(.anchor-preview > button),
.text-action {
  padding: 3px 0;
  min-height: 28px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  box-shadow: none;
  color: var(--muted);
  font-size: var(--ui-font-meta);
  font-weight: 400;
}
.thread-tools :deep(.anchor-preview > button:hover),
.text-action:hover {
  color: var(--accent);
  text-decoration: underline;
  background: transparent;
  transform: none;
}
.comment-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 0;
}
.comment-content {
  min-width: 0;
  flex: 1;
}
.comment-byline {
  min-height: 30px;
}
.comment-body {
  margin: 5px 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  line-height: 1.75;
}
.reply-item {
  margin-left: 22px;
  padding: 10px;
  background: #f4f6f0;
}
.reply-item .comment-avatar {
  flex-basis: 28px;
  width: 28px;
  height: 28px;
  font-size: 14px;
}
.comment-actions {
  justify-content: space-between;
}
.comment-menu {
  position: relative;
  flex-shrink: 0;
}
.comment-menu > summary {
  list-style: none;
  padding: 0 8px;
  margin: 0;
  border-radius: 5px;
  color: var(--muted);
  font-size: 22px;
  line-height: 28px;
  cursor: pointer;
}
.comment-menu > summary::-webkit-details-marker {
  display: none;
}
.comment-menu > summary:hover,
.comment-menu[open] > summary {
  background: #e6ece3;
  color: var(--accent);
}
.comment-menu-content {
  position: absolute;
  right: 0;
  top: 100%;
  z-index: 3;
  min-width: 180px;
  width: max-content;
  max-width: min(260px, calc(100vw - 96px));
  padding: 8px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--paper);
  box-shadow: 0 5px 18px #183e3220;
  overflow-wrap: anywhere;
}
.comment-menu-content > button {
  display: block;
  width: 100%;
  min-height: 36px;
  padding: 5px 8px;
  border: 0;
  text-align: left;
  background: transparent;
  box-shadow: none;
  font-size: var(--ui-font-meta);
}
.comment-menu-content > button:hover {
  background: var(--soft);
  transform: none;
}
.comment-menu-content > .destructive-action {
  color: #a54435;
}
.comment-composer {
  margin: 16px 0;
}
.comment-composer > summary {
  color: var(--accent);
  cursor: pointer;
  font-weight: 600;
}
.comment-help {
  font-size: var(--ui-font-meta);
  margin: 10px 0;
  color: var(--muted);
}
.replies-toggle {
  display: block;
  margin: 8px 0 8px 32px;
  color: var(--accent);
}
.reply-placeholder {
  width: 100%;
  margin-top: 12px;
  padding: 9px 12px;
  border: 1px solid var(--line);
  background: #f4f6f0;
  text-align: left;
  color: var(--muted);
  font-size: var(--ui-font-meta);
  box-shadow: none;
}
.reply-composer {
  margin-top: 12px;
}
.reply-context {
  display: block;
  margin: 6px 0;
  padding-left: 10px;
  border-left: 2px solid var(--line);
  overflow-wrap: anywhere;
}
.composer-actions {
  justify-content: flex-end;
  flex-wrap: wrap;
}
@media (prefers-reduced-motion: reduce) {
  .comments-panel button {
    transition: none;
  }
}
</style>
