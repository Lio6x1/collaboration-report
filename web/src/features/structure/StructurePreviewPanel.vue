<script setup lang="ts">
import type { MainView } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import { actionsBlocked } from '../../lib/http';
import { rangeNames, useStructurePreview } from './useStructurePreview';
const props = defineProps<{
  view: MainView;
  reload: () => Promise<void>;
  prepare: () => Promise<unknown>;
  chapterId: string;
  rangeType: 'SPLIT_LEVEL2' | 'JOIN_CHAPTER' | 'REMOVE';
}>();
const emit = defineEmits<{ completed: [] }>();
const { action, chapters, chapter, type, fixed, acknowledged, preview, confirm, discardPreview } =
  useStructurePreview(
    () => props.view,
    props.reload,
    props.prepare,
    () => emit('completed'),
  );
const { busy, error, notice, pending, retry } = action;
chapter.value = props.chapterId;
type.value = props.rangeType;
</script>

<template>
  <section role="region" aria-label="目录范围调整">
    <p class="hint">
      请先在主线中把小节标题设为“二级标题”并保存。预览会自动合入兼容的主线更新，保留共同支线的未审核修改；有冲突时需先处理。
      确认时会通知正在编辑的人保存并短暂停止输入，完成后恢复。预览后有新修改时需重新生成预览。
      拆分后可按小节协作与审核；合章恢复整章编辑，删除会移除整章范围。先生成预览，再确认。
    </p>
    <p v-if="view.document.main_locked" class="hint">请先解锁主线，再调整目录范围。</p>
    <form class="inline-form" @submit.prevent="preview">
      <fieldset
        :disabled="actionsBlocked || view.document.main_locked || !chapters.length || !!fixed"
      >
        <button>生成固定目录预览</button>
      </fieldset>
    </form>
    <section v-if="fixed" class="recovery-box" role="region" aria-label="固定目录预览">
      <h4>{{ rangeNames[fixed.type] }}：{{ fixed.title }}</h4>
      <p>当前目录保持原样；确认后追加新目录并安装新的共同支线，旧版本和审核历史保留。</p>
      <p class="hint">
        预览引用：{{ fixed.response.preview_cut_ref }} · 原目录：{{ fixed.response.structure_id }}
      </p>
      <p class="hint">有效至 {{ new Date(fixed.response.expires_at).toLocaleString() }}</p>
      <p class="hint">
        固定摘要：<code aria-label="固定预览摘要">{{ fixed.response.cut_digest }}</code>
      </p>
      <ul v-if="fixed.response.blocking_units.length">
        <li v-for="unit in fixed.response.blocking_units" :key="unit.unit_id">
          单元 {{ unit.unit_id }}：{{ unit.reason }}
        </li>
      </ul>
      <p v-else class="hint">预览时没有阻止单元；确认仍须核验当前资格及原可靠切点。</p>
      <label
        ><input
          v-model="acknowledged"
          type="checkbox"
          aria-label="我已确认目录范围变化"
          :disabled="actionsBlocked"
        />我已确认目录范围变化</label
      >
      <div class="review-actions">
        <button
          :disabled="actionsBlocked || !acknowledged || !!fixed.response.blocking_units.length"
          @click="confirm"
        >
          确认目录范围调整
        </button>
        <button class="quiet" :disabled="actionsBlocked" @click="discardPreview">关闭此预览</button>
      </div>
    </section>
    <ActionStatus :busy="busy" :error="error" :notice="notice" :pending="pending" :retry="retry" />
  </section>
</template>
