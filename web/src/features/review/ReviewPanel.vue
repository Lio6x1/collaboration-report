<script setup lang="ts">
import { onMounted, watch, ref, computed, nextTick } from 'vue';
import ActionStatus from '../../components/ActionStatus.vue';
import { actionsBlocked } from '../../lib/http';
import { accountNumber } from '../account/identity';
import { useReviews } from './useReviews';
import PaginationControls from '../../components/PaginationControls.vue';
import FixedContentView from './FixedContentView.vue';
import ReviewDiffView from './ReviewDiffView.vue';
import CommentsPanel from '../comments/CommentsPanel.vue';
import MergeControl from '../merge/MergeControl.vue';
import ReverseControl from '../reverse/ReverseControl.vue';
import type { Anchor, ConflictContext, ReviewRequest } from '../../../../contracts/http/generated';

const props = defineProps<{
  document: string;
  actor: string;
  managing: boolean;
  canComment: boolean;
  space?: string;
  initialRequest?: string;
  canReverse: (unit: string) => boolean;
  refresh: number;
  prepareMerge: () => Promise<void>;
  reloadMerge: () => Promise<void>;
  reloadReverse: (request: string) => Promise<void>;
  openConflict: (context: ConflictContext) => Promise<void>;
}>();
const state = useReviews(
  () => props.document,
  () => props.actor,
  () => props.managing,
  () => props.initialRequest ?? '',
);
const {
  requests,
  request,
  displayedRounds,
  fixed,
  fixedEntries,
  comparisonBase,
  comparisonLoading,
  comparisonError,
  loadComparison,
  changes,
  selectedRound,
  reason,
  resolution,
  loading,
  readingError,
  requestPagination,
  roundPagination,
  changePagination,
  canResolve,
  load,
  refresh,
  open,
  loadRequests,
  loadRounds,
  loadChanges,
  refreshRounds,
  refreshChanges,
  requestChanges,
  withdraw,
  resolve,
} = state;
const { busy, error, notice, pending, retry } = state.action;
const commentAnchor = ref<Anchor | null>(null);
const focusedAnchor = ref<Anchor | null>(null);
const bodyExpanded = ref(false);
const bodyMode = ref<'diff' | 'full'>('diff');
const discussionExpanded = ref(false);
const discussionVisited = ref(false);
const bodyViewport = ref<HTMLElement | null>(null);
const fixedTitle = computed(
  () =>
    fixedEntries.value.map((entry) => entry.title).join(' / ') ||
    request.value?.summary?.title ||
    '送审内容',
);
const statusLabel = (status: ReviewRequest['status']) =>
  status === 'REVIEWING' ? '审核中' : status === 'WITHDRAWN' ? '已撤回' : '已合并';
const submitterLabel = (id: string) =>
  id === props.actor ? '我' : '协作者（编号 ' + accountNumber(id) + '）';
function submittedTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? ''
    : new Intl.DateTimeFormat('zh-CN', {
        month: 'numeric',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
      }).format(date);
}
function toggleBody(event: Event) {
  bodyExpanded.value = (event.currentTarget as HTMLDetailsElement).open;
}
function toggleDiscussion(event: Event) {
  discussionExpanded.value = (event.currentTarget as HTMLDetailsElement).open;
  if (discussionExpanded.value) discussionVisited.value = true;
}
function selectRequest(id: string) {
  // 当前卡片再次点击不重新加载，保留用户正在阅读的位置和未提交讨论。
  if (request.value?.id !== id) void open(id);
}
async function focusComment(anchor: Anchor) {
  focusedAnchor.value = anchor;
  bodyMode.value = 'full';
  bodyExpanded.value = true;
  await nextTick();
  bodyViewport.value
    ?.querySelector('.comment-anchor-highlight')
    ?.scrollIntoView({ block: 'nearest' });
}
watch(
  () => fixed.value?.id,
  () => {
    commentAnchor.value = null;
    focusedAnchor.value = null;
    // 每个固定轮次单独展开，切换请求或历史轮次后不能沿用上一份长文的展开状态。
    bodyExpanded.value = false;
    bodyMode.value = 'diff';
    discussionExpanded.value = false;
    discussionVisited.value = false;
  },
);
onMounted(() => void load());
watch(
  () => props.refresh,
  () => void load(),
);
</script>

<template>
  <section class="review-panel" role="region" aria-label="审核记录">
    <div class="section-title">
      <div>
        <span class="eyebrow">固定轮次</span>
        <h3>送审与审核记录</h3>
      </div>
      <button :disabled="actionsBlocked || loading" @click="refresh">刷新审核记录</button>
    </div>
    <p class="hint">送审正文固定保留；之后的共同草稿不会改变这份审核内容。</p>
    <p class="hint">
      {{
        managing
          ? '你具有文档管理权，可以通过自己提交的内容；仍须先处理修改要求和合并冲突。'
          : '普通审核者不能通过自己参与提交的请求，请由文档管理员或其他审核者处理。'
      }}
    </p>
    <p v-if="loading" aria-live="polite">正在读取审核记录…</p>
    <p v-if="readingError" class="error" role="alert">{{ readingError }}</p>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
    <p v-if="!loading && !requests.length && !readingError" class="empty">本页没有可读审核记录。</p>
    <ul v-if="requests.length" class="review-records" aria-label="审核请求">
      <li v-for="r in requests" :key="r.id">
        <button
          type="button"
          class="review-record"
          :data-request-id="r.id"
          :aria-pressed="request?.id === r.id"
          :aria-label="
            '查看审核：' + (r.summary?.title || '送审内容') + '，' + statusLabel(r.status)
          "
          :disabled="actionsBlocked || loading"
          @click="selectRequest(r.id)"
        >
          <span class="review-record-heading">
            <span class="review-record-title">{{ r.summary?.title || '送审内容' }}</span>
            <span class="review-record-state" :data-status="r.status">{{
              statusLabel(r.status)
            }}</span>
          </span>
          <span class="review-record-excerpt">{{
            r.summary?.excerpt || '本轮暂无文字摘要，展开正文查看。'
          }}</span>
          <span class="review-record-meta">
            <template v-if="r.summary">
              <span>第 {{ r.summary.round_no }} 轮</span>
              <span>{{ r.summary.submitter_id === actor ? '我提交' : '协作者提交' }}</span>
              <time :datetime="r.summary.submitted_at">{{
                submittedTime(r.summary.submitted_at)
              }}</time>
            </template>
            <span v-if="r.blocking_change_count">{{ r.blocking_change_count }} 项待处理要求</span>
            <span v-if="r.archived_at">已归档</span>
          </span>
        </button>
      </li>
    </ul>
    <PaginationControls
      label="审核请求"
      :page="requestPagination.index + 1"
      :has-next="!!requestPagination.next"
      :busy="actionsBlocked || loading"
      @change="loadRequests"
    />
    <article v-if="request && fixed" class="review-detail" aria-label="当前审核详情">
      <h4 class="review-detail-title">{{ fixedTitle }}</h4>
      <div class="review-summary">
        <span class="review-state">{{
          request.status === 'REVIEWING'
            ? '审核中'
            : request.status === 'WITHDRAWN'
              ? '已撤回，可编辑后重新送审'
              : '已合并'
        }}</span
        ><span>未解除要求 {{ request.blocking_change_count ?? 0 }} 项</span>
      </div>
      <label
        >审核轮次<select v-model="selectedRound" :disabled="loading">
          <optgroup v-if="!displayedRounds.some((r) => r.id === fixed?.id)" label="当前固定阅读">
            <option :value="fixed.id">
              第 {{ fixed.round_no }} 轮 · {{ submitterLabel(fixed.submitter_id) }}提交（选中）
            </option>
          </optgroup>
          <optgroup label="本页审核轮次">
            <option v-for="r in displayedRounds" :key="r.id" :value="r.id">
              第 {{ r.round_no }} 轮{{
                r.id === request.current_round_id ? '（当前）' : '（历史）'
              }}
              · {{ submitterLabel(r.submitter_id) }}提交
            </option>
          </optgroup>
        </select></label
      >
      <button type="button" :disabled="actionsBlocked || loading" @click="refreshRounds">
        刷新审核轮次
      </button>
      <PaginationControls
        label="审核轮次"
        :page="roundPagination.index + 1"
        :has-next="!!roundPagination.next"
        :busy="actionsBlocked || loading"
        @change="loadRounds"
      />
      <details class="review-disclosure" :open="bodyExpanded" @toggle="toggleBody">
        <summary>
          <span>{{ bodyExpanded ? '收起送审正文' : '展开送审正文' }}</span>
          <span class="hint">第 {{ fixed.round_no }} 轮 · 固定送审版本</span>
        </summary>
        <div
          v-if="bodyExpanded"
          class="review-body-mode"
          role="group"
          aria-label="送审正文显示方式"
        >
          <button type="button" :aria-pressed="bodyMode === 'diff'" @click="bodyMode = 'diff'">
            查看改动
          </button>
          <button type="button" :aria-pressed="bodyMode === 'full'" @click="bodyMode = 'full'">
            完整送审稿
          </button>
          <span class="hint">{{
            bodyMode === 'diff' ? '比较本轮固定基线与送审稿' : '选择正文文字可关联本轮批注'
          }}</span>
        </div>
        <div
          ref="bodyViewport"
          class="review-paper-scroll"
          role="region"
          aria-label="送审正文阅读区"
          tabindex="0"
        >
          <template v-if="bodyExpanded && bodyMode === 'diff'">
            <p v-if="comparisonLoading" role="status">正在读取本轮固定基线…</p>
            <div v-else-if="comparisonError" class="review-comparison-error" role="alert">
              <p>{{ comparisonError }}</p>
              <button type="button" @click="loadComparison">重新读取基线</button>
            </div>
            <ReviewDiffView
              v-else-if="comparisonBase"
              :key="fixed.id + ':' + comparisonBase.id"
              :base="comparisonBase"
              :submitted="fixed.content"
              :resource-context="{ document, kind: 'REVIEW', id: fixed.id }"
            />
          </template>
          <FixedContentView
            v-if="bodyExpanded && bodyMode === 'full'"
            :key="fixed.id"
            :content="fixed.content"
            :style="fixed.style"
            :resource-context="{ document, kind: 'REVIEW', id: fixed.id }"
            :selectable="canComment"
            :anchor="focusedAnchor"
            @selection="commentAnchor = $event"
          />
        </div>
      </details>
      <details class="review-version-info">
        <summary>版本信息</summary>
        <p class="hint">
          审核请求 {{ request.id }} · 提交人 {{ submitterLabel(fixed.submitter_id) }}
        </p>
        <p class="hint">
          固定目录 {{ fixed.structure.id }} · 样式 {{ fixed.style.id }} · 正文版本
          {{ fixed.content_version_id }}
        </p>
        <p class="hint">
          比较基线版本 {{ fixed.base_version_id }} · 固定基线清单 {{ fixed.base_manifest_id }}
        </p>
      </details>
      <details class="review-disclosure" :open="discussionExpanded" @toggle="toggleDiscussion">
        <summary>
          <span>本轮讨论</span
          ><span class="hint">{{ discussionExpanded ? '收起' : '展开查看或发表评论' }}</span>
        </summary>
        <CommentsPanel
          v-if="discussionVisited"
          :key="'comments:' + fixed.id"
          :document="document"
          context-kind="REVIEW"
          :context-id="fixed.id"
          :version="fixed.content.id"
          :anchor="commentAnchor"
          @clear-anchor="commentAnchor = null"
          @open-anchor="focusComment"
          :actor="actor"
          :can-comment="canComment"
          :can-manage="managing"
          :space="space"
        />
      </details>
      <div v-if="selectedRound === request.current_round_id" class="review-actions">
        <p v-if="!bodyExpanded" class="review-reading-hint hint">
          摘要用于区分请求，请展开送审正文核对完整内容后再审核。
        </p>
        <ReverseControl
          v-if="
            managing &&
            request.status === 'MERGED' &&
            !request.archived_at &&
            canReverse(request.unit_id)
          "
          :key="request.id + ':reverse'"
          :request="request"
          :prepare="prepareMerge"
          :reload="reloadReverse"
        />
        <MergeControl
          v-if="request.actions.includes('APPROVE_AND_MERGE')"
          :key="request.id + ':' + request.current_round_id"
          :request="request"
          :actor="actor"
          :prepare="prepareMerge"
          :reload="reloadMerge"
          :open-conflict="openConflict"
        />
        <form v-if="request.actions.includes('REQUEST_CHANGES')" @submit.prevent="requestChanges">
          <fieldset :disabled="actionsBlocked">
            <label>审核理由<textarea v-model="reason" required maxlength="2000"></textarea></label
            ><button>要求修改</button>
          </fieldset>
        </form>
        <button
          v-if="request.actions.includes('WITHDRAW')"
          :disabled="actionsBlocked"
          @click="withdraw"
        >
          撤回当前轮次
        </button>
      </div>
      <div class="section-title">
        <h4>修改要求</h4>
        <button type="button" :disabled="actionsBlocked || loading" @click="refreshChanges">
          刷新修改要求
        </button>
      </div>
      <p v-if="!loading && !readingError && !changes.length" class="empty">本页没有修改要求。</p>
      <ul class="review-changes">
        <li v-for="change in changes" :key="change.id">
          <div class="section-title">
            <p>{{ change.reason }}</p>
            <span>{{ change.status === 'OPEN' ? '待解除' : '已解除' }}</span>
          </div>
          <small
            >原轮次 {{ change.round_id }} · 提出者 {{ submitterLabel(change.author_id) }}</small
          >
          <p v-if="change.resolution_reason">处理结论：{{ change.resolution_reason }}</p>
          <form
            v-if="change.status === 'OPEN' && canResolve(change)"
            @submit.prevent="resolve(change)"
          >
            <fieldset :disabled="actionsBlocked">
              <label
                >解除理由<textarea
                  v-model="resolution[change.id]"
                  required
                  maxlength="2000"
                ></textarea></label
              ><button>解除修改要求</button>
            </fieldset>
          </form>
        </li>
      </ul>
      <PaginationControls
        label="修改要求"
        :page="changePagination.index + 1"
        :has-next="!!changePagination.next"
        :busy="actionsBlocked || loading"
        @change="loadChanges"
      />
    </article>
  </section>
</template>

<style scoped>
.review-records {
  display: grid;
  gap: 10px;
  list-style: none;
  padding: 2px;
  margin: 22px 0;
  max-height: 440px;
  overflow-y: auto;
}
.review-record {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  justify-content: stretch;
  align-items: stretch;
  gap: 9px;
  width: 100%;
  padding: 16px 18px;
  text-align: left;
  background: var(--paper);
  color: var(--ink);
}
.review-record[aria-pressed='true'] {
  border-color: var(--accent);
  background: #f0f4ea;
}
.review-record-heading {
  display: flex;
  align-items: start;
  gap: 16px;
  justify-content: space-between;
}
.review-record-title {
  min-width: 0;
  overflow-wrap: anywhere;
  font-weight: 600;
  font-size: var(--ui-font-body);
}
.review-record-state {
  flex: 0 0 auto;
  padding: 3px 9px;
  font-size: var(--ui-font-meta);
  color: var(--accent);
  border-radius: 4px;
  background: #e8efdf;
}
.review-record-state[data-status='WITHDRAWN'] {
  color: #786a4b;
  background: #f2eee4;
}
.review-record-excerpt {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  overflow-wrap: anywhere;
  color: var(--muted);
  line-height: 1.65;
  font-size: var(--ui-font-control);
}
.review-record-meta {
  display: flex;
  flex-wrap: wrap;
  column-gap: 14px;
  row-gap: 4px;
  color: var(--muted);
  font-size: var(--ui-font-meta);
}
.review-detail {
  padding-top: 22px;
  margin-top: 24px;
  border-top: 1px solid var(--line);
}
.review-detail-title {
  font-size: var(--ui-font-section);
  margin: 0;
  overflow-wrap: anywhere;
}
.review-disclosure {
  border: 1px solid var(--line);
  border-radius: 8px;
  margin: 18px 0;
  padding: 0 16px;
}
.review-disclosure > summary {
  padding: 15px 0;
  cursor: pointer;
  font-weight: 600;
}
.review-disclosure > summary .hint {
  margin-left: 12px;
  font-weight: 400;
}
.review-paper-scroll {
  max-height: min(65vh, 640px);
  overflow: auto;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
}
.review-paper-scroll :deep(.fixed-review-body) {
  margin: 0 0 16px;
}
.review-body-mode {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 0 0 14px;
}
.review-body-mode button[aria-pressed='true'] {
  border-color: var(--accent);
  background: #f0f4ea;
}
.review-body-mode .hint {
  margin: 0;
}
.review-comparison-error {
  color: #a3262c;
  padding-bottom: 16px;
}
.review-version-info {
  margin: 16px 0;
  color: var(--muted);
  font-size: var(--ui-font-meta);
  overflow-wrap: anywhere;
}
.review-version-info > summary {
  cursor: pointer;
}
.review-reading-hint {
  flex-basis: 100%;
  margin: 0;
}
@media (max-width: 520px) {
  .review-record {
    padding: 13px;
  }
  .review-disclosure {
    padding: 0 12px;
  }
  .review-disclosure > summary .hint {
    display: block;
    margin: 4px 0 0;
  }
}
</style>
