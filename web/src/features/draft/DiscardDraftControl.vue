<script setup lang="ts">
import { defineAsyncComponent } from 'vue';
import type { DiscardPreviewCreate } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import { actionsBlocked } from '../../lib/http';
import { accountNumber } from '../account/identity';
import { useDiscardDraft } from './useDiscardDraft';

const ContentView = defineAsyncComponent(() => import('../../lib/content/ContentView.vue'));
const props = defineProps<{
  document: string;
  unit: string;
  disabled: boolean;
  prepare: () => Promise<Omit<DiscardPreviewCreate, 'operation_id'>>;
  reload: () => Promise<void>;
}>();
const { action, fixed, reasons, acknowledged, expired, canConfirm, preview, confirm, close } =
  useDiscardDraft(props.document, props.unit, props.prepare, props.reload);
const { busy, error, notice, pending, retry } = action;
</script>

<template>
  <section class="draft-control discard-control" aria-label="丢弃草稿">
    <div class="section-title">
      <h4>结束当前草稿工作</h4>
      <button v-if="!fixed" type="button" :disabled="disabled || actionsBlocked" @click="preview">
        生成丢弃预览
      </button>
    </div>
    <p class="hint">先保留并核对旧稿。正在审核的工作需先撤回；每条未解除意见都需说明处理理由。</p>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
    <section v-if="fixed" role="region" aria-label="丢弃确认" class="discard-preview">
      <h4>将保留的旧稿</h4>
      <p class="hint">
        正文版本 {{ fixed.content.id }} · 目录 {{ fixed.response.structure_id }} · 支线
        {{ fixed.response.branch_id }}
      </p>
      <p class="hint">
        主线清单 {{ fixed.response.main_manifest_id }} · 原预览 {{ fixed.response.preview_cut_ref }}
      </p>
      <p>预览有效至 {{ new Date(fixed.response.expires_at).toLocaleString() }}</p>
      <p v-if="fixed.round" class="hint">
        将归档已撤回轮次 {{ fixed.round }}，旧稿和意见记录继续保留。
      </p>
      <ContentView
        :content="fixed.content"
        :resource-context="{ document, kind: 'BRANCH', id: fixed.response.branch_id }"
        label="待保留旧草稿"
      />
      <p v-if="!fixed.response.blocking_changes.length" class="hint">
        此预览没有未解除的修改要求。
      </p>
      <div
        v-for="change in fixed.response.blocking_changes"
        :key="change.id"
        class="discard-opinion"
      >
        <p>{{ change.reason }}</p>
        <small
          >意见 {{ change.id }} · 修订 {{ change.revision }} · 提出者
          {{ accountNumber(change.author_id) }}</small
        >
        <label :for="'discard-reason-' + change.id">意见 {{ change.id }} 的处理理由</label>
        <textarea
          :id="'discard-reason-' + change.id"
          v-model="reasons[change.id]"
          :disabled="actionsBlocked"
          maxlength="2000"
          required
        ></textarea>
      </div>
      <label
        ><input
          v-model="acknowledged"
          type="checkbox"
          :disabled="actionsBlocked || expired"
        />我确认保留所示旧稿，恢复共同草稿并归档已撤回工作</label
      >
      <p v-if="expired" class="error">预览已到期，请关闭后重新核对。</p>
      <div class="button-row spaced">
        <button
          type="button"
          :disabled="disabled || actionsBlocked || !canConfirm"
          @click="confirm"
        >
          确认丢弃并恢复主线
        </button>
        <button type="button" :disabled="actionsBlocked" @click="close">关闭此预览</button>
      </div>
    </section>
  </section>
</template>
