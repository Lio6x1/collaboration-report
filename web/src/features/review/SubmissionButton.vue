<script setup lang="ts">
import { UserFacingError, actionsBlocked, useAction } from '../../lib/http';
import type { SubmissionBoundary } from '../editor/domain/model';
import ActionStatus from '../../components/ActionStatus.vue';
import { submissionMode } from './api';

const props = defineProps<{
  document: string;
  unit: string;
  disabled: boolean;
  prepare: () => Promise<SubmissionBoundary>;
}>();
const emit = defineEmits<{ submitted: [request: string] }>();
const action = useAction();
const { busy, error, notice, pending, retry } = action;

async function submit() {
  let fixed: Record<string, unknown> | undefined;
  await action.run(async () => {
    // 续接闭包保存首次意图和 FLUSH；丢响应后继续编辑不能让同编号换成后来的正文。
    if (!fixed) {
      const mode = await submissionMode(props.document, props.unit);
      fixed = { ...mode, ...(await props.prepare()) };
    }
    const result = await action.mutate(
      `/v1/documents/${props.document}/units/${props.unit}/submissions`,
      fixed,
    );
    const request = 'result' in result ? result.result?.request_id : undefined;
    if (!request) throw new UserFacingError('已固定送审，但新请求编号尚未核验，请查询原操作');
    // 同一单元上一项可以已经MERGED；把已确认结果原号传给阅读面板，不能只刷新后保留旧选择。
    emit('submitted', request);
  }, '固定送审已完成，后续草稿可继续编辑');
}
</script>

<template>
  <div class="submission-control">
    <button type="button" :disabled="disabled || actionsBlocked" @click="submit">
      {{ busy ? '正在保存并送审…' : '送审当前草稿' }}
    </button>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
  </div>
</template>
