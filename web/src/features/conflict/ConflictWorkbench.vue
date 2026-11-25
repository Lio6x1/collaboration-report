<script setup lang="ts">
import { UserFacingError, actionsBlocked } from '../../lib/http';
import { ref } from 'vue';
import type { ConflictContext } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import ConflictInputs from '../merge/ConflictInputs.vue';
import CandidateEditor from './CandidateEditor.vue';
import { useConflictWorkbench } from './useConflictWorkbench';

const props = defineProps<{
  document: string;
  canEdit: (unit: string) => boolean;
  prepare: () => Promise<void>;
  reload: () => Promise<void>;
}>();
const {
  action,
  reference,
  context,
  request,
  editable,
  withdrawn,
  expired,
  handled,
  acknowledged,
  canSave,
  open,
  accept,
  refreshRequest,
  save,
  close,
} = useConflictWorkbench(props.document, props.canEdit, props.prepare, props.reload);
const candidate = ref<{ read: () => Promise<object> } | null>(null);
const { busy, error, notice, pending, retry } = action;
defineExpose({ accept: (value: ConflictContext) => accept(value) });
async function confirm() {
  await save(async () => {
    if (!candidate.value) throw new UserFacingError('人工候选尚未建立，请保留原上下文');
    return candidate.value.read();
  });
}
</script>
<template>
  <section class="conflict-workbench" role="region" aria-label="人工修复工作台">
    <h3>固定冲突人工修复</h3>
    <p class="hint">
      填写原冲突操作编号，或从本次合并结果打开。先明确撤回，再把完整修复保存为新共同草稿。
    </p>
    <div class="inline-form">
      <label
        >原冲突操作编号<input
          v-model="reference"
          :disabled="actionsBlocked || !!context"
          maxlength="96"
      /></label>
      <button v-if="!context" type="button" :disabled="actionsBlocked" @click="open">
        打开固定冲突工作台
      </button>
    </div>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
    <template v-if="context">
      <ConflictInputs :context="context" />
      <template v-if="editable">
        <h4>完整人工修复候选</h4>
        <p class="hint">从原固定送审正文开始修改，保存成功前不会改变共同草稿。</p>
        <CandidateEditor
          ref="candidate"
          :key="context.conflict_ref"
          :initial="context.submitted"
          :resource-context="{
            document: context.document_id,
            kind: 'REVIEW',
            id: context.round_id,
          }"
          :disabled="actionsBlocked || expired"
        />
        <p v-if="request?.status === 'REVIEWING'" class="error">
          请先在审核记录撤回本轮，再核对撤回状态。
        </p>
        <p v-if="withdrawn" class="notice">已核验原轮次撤回，可以保存修复为新共同草稿。</p>
        <button type="button" :disabled="actionsBlocked" @click="refreshRequest">
          核对撤回状态
        </button>
        <div v-for="item in context.conflicts" :key="item.id" class="resolution-item">
          <p>{{ item.reason }}</p>
          <label
            ><input
              v-model="handled[item.id]"
              type="checkbox"
              :aria-label="'已处理冲突 ' + item.id"
              :disabled="actionsBlocked || expired"
            />我已在完整候选中处理这一项冲突</label
          >
        </div>
        <label
          ><input
            v-model="acknowledged"
            type="checkbox"
            :disabled="actionsBlocked || expired"
          />我已核对完整修复正文，保存为新共同草稿后重新送审</label
        >
        <button type="button" :disabled="actionsBlocked || !canSave" @click="confirm">
          保存修复为新草稿
        </button>
      </template>
      <p v-else class="hint">当前范围没有完整编辑资格。可以核对固定输入，由有权编辑者处理修复。</p>
      <p v-if="expired" class="error">原冲突上下文已经到期，请保留候选并核对当前审核。</p>
      <p class="hint">关闭工作台会移除本次候选；共同草稿只在保存动作成功后更新。</p>
      <button type="button" :disabled="actionsBlocked" @click="close">关闭工作台</button>
    </template>
  </section>
</template>
