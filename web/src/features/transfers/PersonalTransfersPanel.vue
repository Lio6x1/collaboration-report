<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { ApiError, explain, transportEpoch } from '../../lib/http';
import ActionStatus from '../../components/ActionStatus.vue';
import PaginationControls from '../../components/PaginationControls.vue';
import { usePersonalTransfers } from './usePersonalTransfers';
import { accountNumber } from '../account/identity';
import type { PersonalTransferMetadata } from '../../../../contracts/http/generated';

const {
  action,
  direction,
  offers,
  pagination,
  knowledgeBases,
  knowledgeBaseCursor,
  knowledgeBase,
  quota,
  loading,
  targetsLoading,
  readingError,
  targetError,
  progress,
  blocked,
  load,
  refresh,
  loadTargets,
  transition,
  inspect,
} = usePersonalTransfers();
const accepting = ref<PersonalTransferMetadata | null>(null),
  confirmed = ref(false);
const available = computed(() => {
  if (!quota.value) return '';
  const bytes =
    BigInt(quota.value.limit_bytes) -
    BigInt(quota.value.used_bytes) -
    BigInt(quota.value.reserved_bytes);
  return (bytes > 0n ? bytes : 0n).toString();
});
const labels: Record<PersonalTransferMetadata['state'], string> = {
  PENDING: '等待接收',
  ACCEPTING: '原接受操作处理中',
  ACCEPTED: '已接受',
  CANCELLED: '已取消',
  REJECTED: '已拒绝',
  EXPIRED: '已过期',
};
// 翻页不取消已固定的接收确认；切换身份或方向才清空旧范围的输入。
watch([direction, transportEpoch], () => {
  accepting.value = null;
  confirmed.value = false;
});
async function confirmAcceptance() {
  if (!accepting.value) return;
  await transition(accepting.value, 'accept');
  if (!action.error.value && !action.pending.value) {
    accepting.value = null;
    confirmed.value = false;
  }
}
</script>

<template>
  <section class="panel" aria-label="个人文档转让">
    <div class="section-title">
      <h2>个人文档转让</h2>
      <button type="button" :disabled="blocked" @click="refresh">刷新转让申请</button>
    </div>
    <p class="hint">
      只有接收方确认且容量足够，文档才转入接收方个人空间。接受前仍归原负责人；处理中沿原操作号恢复。
    </p>
    <label
      >转让方向<select v-model="direction" :disabled="blocked">
        <option value="incoming">发给我的申请</option>
        <option value="outgoing">我发出的申请</option>
      </select></label
    >
    <ActionStatus
      v-if="!action.pending.value"
      :busy="action.busy.value"
      :error="action.error.value"
      :notice="action.notice.value"
      :pending="action.pending.value"
      :retry="action.retry.value"
    />
    <p v-if="readingError" role="alert">{{ readingError }}</p>
    <p v-if="loading" role="status">正在读取当前转让申请…</p>
    <p v-if="progress" role="status">
      原接受操作：{{ progress.operation_id }} · {{ progress.state
      }}<span v-if="progress.error">
        ·
        {{
          explain(new ApiError(progress.error.code, 0), '转让未完成，请刷新申请状态后重新确认。')
        }}</span
      >
    </p>
    <div v-if="direction === 'incoming'" class="target-settings">
      <label
        >接收知识库<select v-model="knowledgeBase" :disabled="blocked">
          <option v-for="kb in knowledgeBases" :key="kb.id" :value="kb.id">
            {{ kb.name }} · {{ kb.id }}
          </option>
        </select></label
      >
      <button type="button" :disabled="blocked" @click="loadTargets()">刷新接收容量</button>
      <button
        v-if="knowledgeBaseCursor"
        type="button"
        :disabled="blocked"
        @click="loadTargets(true)"
      >
        更多个人知识库
      </button>
      <p v-if="targetsLoading" role="status">正在读取个人知识库与容量…</p>
      <p v-if="targetError" role="alert">{{ targetError }}</p>
      <p v-if="quota" class="hint">
        当前可用 {{ available }} 字节。实际接受还会核对完整历史、资源与持久草稿容量。
      </p>
      <p v-if="!knowledgeBases.length && !targetsLoading && !targetError" class="hint">
        请先在工作台创建个人知识库，再刷新此页。
      </p>
    </div>
    <ul class="entity-list" aria-label="转让申请列表">
      <li v-for="offer in offers" :key="offer.id">
        <div class="offer-details">
          <strong>{{ offer.document_title }}</strong>
          <p>文档 {{ offer.document_id }} · {{ labels[offer.state] }} · 申请 {{ offer.id }}</p>
          <small
            >原负责人 {{ accountNumber(offer.source_user_id) }} → 接收人
            {{ accountNumber(offer.target_user_id) }} · 有效至
            {{ new Date(offer.expires_at).toLocaleString() }}</small
          >
        </div>
        <template v-if="offer.state === 'PENDING'">
          <template v-if="direction === 'incoming'">
            <button
              type="button"
              :disabled="blocked || !knowledgeBase || !quota"
              @click="
                accepting = { ...offer };
                confirmed = false;
              "
            >
              接受文档转入
            </button>
            <button type="button" :disabled="blocked" @click="transition(offer, 'reject')">
              拒绝转让
            </button>
          </template>
          <button v-else type="button" :disabled="blocked" @click="transition(offer, 'cancel')">
            取消转让申请
          </button>
        </template>
        <button
          v-if="offer.state === 'ACCEPTING' && offer.accept_operation_id"
          type="button"
          :disabled="blocked"
          @click="inspect(offer)"
        >
          查询原接受操作
        </button>
      </li>
    </ul>
    <PaginationControls
      label="转让申请"
      :page="pagination.index + 1"
      :has-next="!!pagination.next"
      :busy="blocked"
      @change="load"
    />
    <div v-if="accepting" class="accept-confirmation" aria-label="接受转让确认">
      <p>
        将「{{
          accepting.document_title
        }}」转入你选择的个人知识库。旧访问和正式发布会关闭，内容与历史保留。
      </p>
      <label
        ><input
          v-model="confirmed"
          type="checkbox"
          :disabled="blocked"
        />我确认接收这篇文档并承担对应存储容量</label
      >
      <div class="toolbar">
        <button
          type="button"
          :disabled="blocked || !confirmed || !knowledgeBase || !quota"
          @click="confirmAcceptance"
        >
          确认接受转让</button
        ><button
          type="button"
          :disabled="blocked"
          @click="
            accepting = null;
            confirmed = false;
          "
        >
          取消
        </button>
      </div>
    </div>
    <p v-if="!loading && !readingError && !offers.length" class="empty">
      本页没有当前可见的转让申请。
    </p>
  </section>
</template>

<style scoped>
.target-settings {
  margin: 20px 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
.target-settings p {
  width: 100%;
  margin: 0;
}
.target-settings label {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  max-width: 100%;
  white-space: nowrap;
  gap: 8px;
}
.target-settings select {
  min-width: min(320px, 100%);
  max-width: 100%;
}
.offer-details {
  flex: 1;
  min-width: 240px;
}
.offer-details p {
  margin: 6px 0;
}
.accept-confirmation {
  width: 100%;
  margin-top: 12px;
  padding: 16px;
  border: 1px solid var(--line);
  border-radius: 12px;
}
.accept-confirmation label,
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
}
.toolbar {
  margin-top: 12px;
}
</style>
