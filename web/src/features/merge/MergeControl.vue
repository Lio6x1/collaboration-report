<script setup lang="ts">
import type { ConflictContext, ReviewRequest } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import { actionsBlocked } from '../../lib/http';
import { useApproveMerge } from './useApproveMerge';
import ConflictInputs from './ConflictInputs.vue';
const props = defineProps<{
  request: ReviewRequest;
  actor: string;
  prepare: () => Promise<void>;
  reload: () => Promise<void>;
  openConflict: (context: ConflictContext) => Promise<void>;
}>();
const {
  action,
  fixed,
  acknowledged,
  included,
  rejected,
  conflict,
  canConfirm,
  inspect,
  confirm,
  inspectConflict,
  close,
} = useApproveMerge(
  () => props.request,
  () => props.actor,
  props.prepare,
  props.reload,
);
const { busy, error, notice, pending, retry } = action;
</script>
<template>
  <section class="merge-control" aria-label="通过并合并">
    <button v-if="!fixed" type="button" :disabled="actionsBlocked" @click="inspect">
      通过并合并
    </button>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
    <section v-if="fixed && !rejected" role="region" aria-label="合并确认">
      <h4>确认整个固定送审版本</h4>
      <p class="hint">轮次 {{ fixed.round }} · 正文版本 {{ fixed.version }}</p>
      <p>确认上方整个固定正文；共同草稿之后的工作继续保留。</p>
      <div v-for="change in fixed.changes" :key="change.id" class="merge-opinion">
        <p>{{ change.reason }}</p>
        <small>本人要求 {{ change.id }} · 修订 {{ change.revision }}</small>
        <label
          ><input
            v-model="included[change.id]"
            type="checkbox"
            :disabled="actionsBlocked"
            :aria-label="'一并解除本人要求 ' + change.id"
          />本次合并成功时一并解除这一项本人要求</label
        >
      </div>
      <label
        ><input
          v-model="acknowledged"
          type="checkbox"
          :disabled="actionsBlocked"
        />我已核对整个固定送审版本，确认通过并合并</label
      >
      <div class="button-row spaced">
        <button type="button" :disabled="actionsBlocked || !canConfirm" @click="confirm">
          确认通过并合并
        </button>
        <button type="button" :disabled="actionsBlocked" @click="close">关闭本次确认</button>
      </div>
    </section>
    <template v-if="rejected">
      <p>本次操作 {{ rejected.operation_id }} 没有批准。请先查看原冲突并处理。</p>
      <button type="button" :disabled="actionsBlocked" @click="inspectConflict">
        查看本次固定冲突
      </button>
      <ConflictInputs v-if="conflict" :context="conflict" />
      <button
        v-if="conflict"
        type="button"
        :disabled="actionsBlocked"
        @click="openConflict(conflict)"
      >
        打开修复工作台
      </button>
      <button type="button" :disabled="actionsBlocked" @click="close">关闭本次合并结果</button>
    </template>
  </section>
</template>
