<script setup lang="ts">
import { onMounted, onUnmounted, watch, ref, toRaw } from 'vue';
import { EditorState } from 'prosemirror-state';
import { Decoration, DecorationSet, EditorView } from 'prosemirror-view';
import { schema, toPM } from '../../../../packages/content/index.mjs';
import { formulaNodeView } from './formula';
import { imageNodeView, type ResourceContext } from './image';
import { codeNodeView, codeHighlightPlugin } from './code';
import type { Anchor, FixedContent, StyleRevision } from '../../../../contracts/http/generated';
import { locateAnchor, selectionAnchor } from '../../features/comments/anchors';

const props = defineProps<{
  content: FixedContent;
  style?: StyleRevision;
  label: string;
  resourceContext?: ResourceContext;
  selectable?: boolean;
  anchor?: Anchor | null;
  commentMarkers?: { threadId: string; anchor: Anchor; label: string; number?: number }[];
  activeComment?: string;
}>();
const emit = defineEmits<{
  selection: [anchor: Anchor | null];
  'open-comment': [thread: string];
}>();
const host = ref<HTMLElement | null>(null);
const selectionHint = ref('');
const anchorHint = ref('');
let view: EditorView | undefined;

function updateHighlight() {
  if (!view) return;
  const location = locateAnchor(view.state.doc, props.content.id, props.anchor);
  anchorHint.value = location.kind === 'orphan' ? location.hint : '';
  const decorations: Decoration[] = [];
  if (location.kind === 'block' || location.kind === 'range') {
    const attributes = { class: 'comment-anchor-highlight' };
    const decoration =
      location.kind === 'block'
        ? Decoration.node(location.from, location.to, attributes)
        : location.from === location.to
          ? Decoration.widget(location.from, () => {
              const marker = document.createElement('span');
              marker.className = 'comment-anchor-caret';
              marker.setAttribute('aria-label', '批注位置');
              return marker;
            })
          : Decoration.inline(location.from, location.to, attributes);
    decorations.push(decoration);
  }
  for (const marker of props.commentMarkers ?? []) {
    const position = locateAnchor(view.state.doc, props.content.id, marker.anchor);
    // 历史版本或已失效块不能猜一个新位置；仍在评论模块保留原文核验入口。
    if (position.kind !== 'block' && position.kind !== 'range') continue;
    // 所有可定位引用都留有浅色标识，当前选中的引用再用强高亮；正文和卡片共用编号。
    if (position.from < position.to)
      decorations.push(
        position.kind === 'block'
          ? Decoration.node(position.from, position.to, { class: 'comment-location-range' })
          : Decoration.inline(position.from, position.to, { class: 'comment-location-range' }),
      );
    decorations.push(
      Decoration.widget(
        position.to,
        () => {
          const button = document.createElement('button');
          button.type = 'button';
          button.className =
            'comment-anchor-marker' + (props.activeComment === marker.threadId ? ' active' : '');
          button.textContent = marker.number ? String(marker.number) : '评论';
          button.setAttribute('aria-label', marker.label);
          button.setAttribute('title', marker.label);
          button.setAttribute('data-comment-thread', marker.threadId);
          button.setAttribute('aria-pressed', String(props.activeComment === marker.threadId));
          button.addEventListener('mousedown', (event) => event.preventDefault());
          button.addEventListener('click', (event) => {
            event.preventDefault();
            event.stopPropagation();
            emit('open-comment', marker.threadId);
          });
          return button;
        },
        {
          key: [
            marker.threadId,
            marker.number,
            marker.label,
            props.activeComment === marker.threadId,
          ].join(':'),
          side: 1,
          stopEvent: () => true,
        },
      ),
    );
  }
  const marked = DecorationSet.create(view.state.doc, decorations);
  view.setProps({ decorations: () => marked });
}

function install() {
  if (!host.value) return;
  view?.destroy();
  emit('selection', null);
  selectionHint.value = '';
  // MAIN阅读和历史审核使用同一正文模型，仅呈现传入的可靠版本。
  // 不绑定Yjs或持久队列，不接收活跃草稿，组件复用也不能取得编辑资格。
  view = new EditorView(host.value, {
    // Vue Proxy不能structuredClone；只在视图边界解包，领域转换仍处理普通JSON。
    state: EditorState.create({
      schema,
      doc: schema.nodeFromJSON(toPM(toRaw(props.content.body_json))),
      plugins: [codeHighlightPlugin()],
    }),
    editable: () => false,
    dispatchTransaction(transaction) {
      if (!view || transaction.docChanged) return;
      // 只读仍需接收PM鼠标/键盘选区事务；明确拒绝正文变化，不能因批注获得编辑能力。
      view.updateState(view.state.apply(transaction));
      if (props.selectable && transaction.selectionSet) {
        const selected = selectionAnchor(view.state.selection, props.content.id);
        selectionHint.value = selected.hint;
        emit('selection', selected.anchor);
      }
    },
    attributes: {
      role: 'document',
      'aria-label': props.label,
      ...(props.selectable ? { tabindex: '0' } : {}),
      // 草稿预览契约没有固定样式，不用当前样式冒充其历史样式；MAIN/审核仍传入真实修订。
      ...(props.style
        ? {
            style: `font-family:${props.style.style.body_font},'Times New Roman',serif;font-size:${props.style.style.body_size_pt}pt`,
          }
        : {}),
    },
    nodeViews: {
      formula: formulaNodeView,
      formula_block: formulaNodeView,
      image: imageNodeView(() => props.resourceContext ?? null),
      code_block: codeNodeView,
    },
  });
  updateHighlight();
}
onMounted(install);
watch(
  // 逐项比较真实边界。父组件更新选区时会重建上下文对象，不能因此销毁编辑器并清空选区。
  [
    () => props.content.id,
    () => props.content.body_json,
    () => props.selectable,
    () => props.style?.id,
    () => props.resourceContext?.document,
    () => props.resourceContext?.kind,
    () => props.resourceContext?.id,
  ],
  install,
);
watch(() => props.anchor, updateHighlight, { deep: true });
watch(() => [props.commentMarkers, props.activeComment], updateHighlight, { deep: true });
onUnmounted(() => view?.destroy());
</script>
<template>
  <div class="fixed-content-view">
    <p v-if="anchorHint" class="anchor-hint" role="status">{{ anchorHint }}</p>
    <div ref="host" class="content-view"></div>
    <p v-if="selectable && selectionHint" class="anchor-hint" role="status">{{ selectionHint }}</p>
  </div>
</template>
<style scoped>
.content-view :deep(.ProseMirror) {
  min-height: 0;
}
.content-view :deep(.comment-anchor-highlight) {
  background: #ffe49a;
  outline: 2px solid #bf8d12;
  border-radius: 2px;
  scroll-margin-top: 120px;
}
.content-view :deep(.comment-location-range) {
  background: #fff7df;
  border-bottom: 2px solid #d9ae42;
}
.content-view :deep(.comment-location-range.comment-anchor-highlight) {
  background: #ffe49a;
}
.content-view :deep(.comment-anchor-caret) {
  border-left: 3px solid #c79b26;
}
.content-view :deep(.comment-anchor-marker) {
  display: inline-flex;
  margin: 2px 0 2px 8px;
  padding: 2px 8px;
  border: 1px solid var(--line);
  border-radius: 12px;
  background: #fff2bd;
  color: var(--accent);
  font-family: var(--ui-font-family, sans-serif);
  font-size: 14px;
  font-weight: 700;
  line-height: 1.5;
  vertical-align: middle;
  cursor: pointer;
}
.content-view :deep(.comment-anchor-marker.active) {
  background: var(--accent);
  color: #fff;
  border-color: var(--accent);
}
.anchor-hint {
  color: #735419;
  font-size: var(--ui-font-meta);
}
</style>
