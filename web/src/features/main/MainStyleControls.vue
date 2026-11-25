<script setup lang="ts">
import type { MainView } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import { actionsBlocked } from '../../lib/http';
import { useMainStyle } from './useMainStyle';
const props = defineProps<{
  view: MainView;
  reload: () => Promise<void>;
  prepare: () => Promise<unknown>;
}>();
const { action, font, applyStyle } = useMainStyle(() => props.view, props.reload, props.prepare);
const { busy, error, notice, pending, retry } = action;
</script>

<template>
  <section class="management" role="region" aria-label="正文样式">
    <h3>整篇正文样式</h3>
    <p class="hint">正文中文字体可选择宋体或仿宋；选中文字的粗体、斜体等格式在独立编辑页中调整。</p>
    <p class="hint">固定排版：A4 · 正文小四（12磅） · 页边距2.54厘米 · 西文 Times New Roman</p>
    <p v-if="view.document.main_locked" class="hint">请先解锁主线，再调整正文样式。</p>
    <form class="inline-form" @submit.prevent="applyStyle">
      <fieldset :disabled="actionsBlocked || view.document.main_locked">
        <label
          >正文中文字体<select v-model="font" aria-label="正文中文字体">
            <option value="SimSun">宋体</option>
            <option value="FangSong">仿宋</option>
          </select></label
        >
        <button :disabled="font === view.style.style.body_font">应用正文样式</button>
      </fieldset>
    </form>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
  </section>
</template>
