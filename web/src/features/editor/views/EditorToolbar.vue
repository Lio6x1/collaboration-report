<script setup lang="ts">
import type { FormatName, TableAction, ParagraphAlignment, ToolbarState } from '../commands';
import AppPopover from '../../../components/AppPopover.vue';
defineProps<{ disabled?: boolean; state: ToolbarState }>();
const formula = defineModel<string>('formula', { required: true });
const emit = defineEmits<{
  format: [FormatName];
  heading: [];
  paragraph: [];
  codeBlock: [];
  paragraphAlignment: [ParagraphAlignment];
  firstLineIndent: [boolean];
  table: [];
  tableAction: [TableAction];
  insertFormula: [];
}>();
</script>
<template>
  <fieldset :disabled="disabled">
    <p class="hint">
      正文小四（12 磅）。选中文字设置强调；段落格式作用于光标所在段落或选中的多段。
    </p>
    <div class="editor-toolbar" role="toolbar" aria-label="正文格式">
      <button
        :aria-pressed="state.marks.strong"
        @mousedown.prevent
        @click="emit('format', 'strong')"
      >
        粗体
      </button>
      <button :aria-pressed="state.marks.em" @mousedown.prevent @click="emit('format', 'em')">
        斜体
      </button>
      <button
        :aria-pressed="state.marks.strike"
        @mousedown.prevent
        @click="emit('format', 'strike')"
      >
        删除线
      </button>
      <button :aria-pressed="state.marks.code" @mousedown.prevent @click="emit('format', 'code')">
        行内代码
      </button>
      <button
        :aria-pressed="state.block === 'code_block'"
        @mousedown.prevent
        @click="emit('codeBlock')"
      >
        代码块
      </button>
      <button
        :aria-pressed="state.block === 'paragraph'"
        :disabled="state.block === null"
        @mousedown.prevent
        @click="emit('paragraph')"
      >
        正文段落
      </button>
      <button
        :aria-pressed="state.block === 'heading'"
        :disabled="state.block === null"
        @mousedown.prevent
        @click="emit('heading')"
      >
        二级标题
      </button>
    </div>
    <div class="editor-paragraph-tools" role="group" aria-label="段落排版">
      <span class="toolbar-label">段落</span>
      <button
        :aria-pressed="state.alignment === 'left'"
        :disabled="state.alignment === null"
        @mousedown.prevent
        @click="emit('paragraphAlignment', 'left')"
      >
        左对齐
      </button>
      <button
        :aria-pressed="state.alignment === 'center'"
        :disabled="state.alignment === null"
        @mousedown.prevent
        @click="emit('paragraphAlignment', 'center')"
      >
        居中对齐
      </button>
      <button
        :aria-pressed="state.alignment === 'right'"
        :disabled="state.alignment === null"
        @mousedown.prevent
        @click="emit('paragraphAlignment', 'right')"
      >
        右对齐
      </button>
      <button
        :aria-pressed="state.indent === true"
        :disabled="state.indent === null || state.indent === true"
        @mousedown.prevent
        @click="emit('firstLineIndent', true)"
      >
        首行缩进
      </button>
      <button
        :aria-pressed="state.indent === false"
        :disabled="state.indent === null || state.indent === false"
        @mousedown.prevent
        @click="emit('firstLineIndent', false)"
      >
        取消缩进
      </button>
    </div>
    <p
      v-if="state.block === 'mixed' || state.alignment === 'mixed' || state.indent === 'mixed'"
      class="hint"
    >
      所选段落格式不一致，点击排版按钮可统一设置。
    </p>
    <div class="editor-inserts">
      <AppPopover v-slot="{ close }" label="表格工具" :disabled="disabled">
        <div class="editor-toolbar">
          <button
            @mousedown.prevent
            @click="
              emit('table');
              close();
            "
          >
            插入表格
          </button>
          <button @mousedown.prevent @click="emit('tableAction', 'addRowAfter')">增行</button>
          <button @mousedown.prevent @click="emit('tableAction', 'addColumnAfter')">增列</button>
          <button @mousedown.prevent @click="emit('tableAction', 'deleteRow')">删行</button>
          <button @mousedown.prevent @click="emit('tableAction', 'deleteColumn')">删列</button>
          <button @mousedown.prevent @click="emit('tableAction', 'mergeCells')">合并格</button>
          <button @mousedown.prevent @click="emit('tableAction', 'splitCell')">拆分格</button>
        </div>
      </AppPopover>
      <AppPopover v-slot="{ close }" label="插入公式" :disabled="disabled">
        <div class="inline-form">
          <label>公式 LaTeX<input v-model="formula" maxlength="4096" /></label>
          <button
            @mousedown.prevent
            @click="
              emit('insertFormula');
              close();
            "
          >
            插入行内公式
          </button>
        </div>
      </AppPopover>
      <slot name="inserts" />
    </div>
  </fieldset>
</template>
