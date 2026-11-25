<script setup lang="ts">
import { UserFacingError } from '../../lib/http';
import { onMounted, onUnmounted, ref, toRaw, watch } from 'vue';
import type { EditorView } from 'prosemirror-view';
import type { FixedContent } from '../../../../contracts/http/generated';
import { fromPM, normalizeContent } from '../../../../packages/content/index.mjs';
import * as commands from '../editor/commands';
import EditorToolbar from '../editor/views/EditorToolbar.vue';
import { createCandidateView } from './candidate_view';
import type { ResourceContext } from '../../lib/content/image';

const props = defineProps<{
  initial: FixedContent;
  disabled: boolean;
  resourceContext: ResourceContext;
}>();
const host = ref<HTMLElement | null>(null),
  formula = ref(''),
  toolbar = ref(commands.toolbarState()),
  loss = ref('');
let view: EditorView | undefined,
  composing = false;
function install() {
  if (!host.value) return;
  view?.destroy();
  view = createCandidateView(
    host.value,
    toRaw(props.initial.body_json),
    () => props.disabled,
    (value) => {
      composing = value;
    },
    (value) => {
      loss.value = value;
    },
    () => props.resourceContext,
    (state) => {
      toolbar.value = commands.toolbarState(state);
    },
  );
}
async function read() {
  view?.dom.blur();
  await new Promise((resolve) => setTimeout(resolve, 0));
  await new Promise((resolve) => setTimeout(resolve, 0));
  if (!view || composing) throw new UserFacingError('请完成中文选词后再保存完整修复候选');
  // 转换完整正文且补齐既有原子/表格规范字段，不经innerText丢掉表格、公式或稳定身份。
  return normalizeContent(fromPM(view.state.doc.toJSON()), { workspaceKind: 'BRANCH' });
}
defineExpose({ read });
onMounted(install);
watch(() => props.initial.id, install);
watch(
  () => props.disabled,
  () => view?.setProps({ editable: () => !props.disabled }),
);
onUnmounted(() => view?.destroy());
</script>
<template>
  <div class="resolution-candidate">
    <fieldset :disabled="disabled">
      <EditorToolbar
        v-model:formula="formula"
        :state="toolbar"
        :disabled="disabled"
        @format="(name) => commands.format(view, name)"
        @heading="commands.heading(view)"
        @paragraph="commands.paragraph(view)"
        @code-block="commands.codeBlock(view)"
        @paragraph-alignment="(align) => commands.paragraphAlignment(view, align)"
        @first-line-indent="(enabled) => commands.firstLineIndent(view, enabled)"
        @table="commands.table(view)"
        @table-action="(name) => commands.tableAction(view, commands.tableActions[name])"
        @insert-formula="commands.insertFormula(view, formula)"
      />
    </fieldset>
    <p v-if="loss" class="notice">{{ loss }}</p>
    <div ref="host" class="paper editor-host"></div>
  </div>
</template>
