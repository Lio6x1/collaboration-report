<script setup lang="ts">
import {
  computed,
  defineComponent,
  h,
  onMounted,
  ref,
  watch,
  type PropType,
  type VNode,
} from 'vue';
import katex from 'katex';
import 'katex/dist/katex.min.css';
import { transportEpoch } from '../../lib/http';
import { resourceContentURL, type ResourceContext } from '../../lib/content/image';
import { compareReviewBodies, type DiffBlock, type DiffSpan, type RichNode } from './reviewDiff';
import type { FixedContent } from '../../../../contracts/http/generated';

const props = defineProps<{
  base: FixedContent;
  submitted: FixedContent;
  resourceContext: ResourceContext;
}>();
const comparison = computed(() =>
  compareReviewBodies(props.base.body_json, props.submitted.body_json),
);
const openGroups = ref(new Set<string>());
watch(
  () => props.submitted.id,
  () => {
    openGroups.value = new Set();
  },
);
const groups = computed(() => {
  const result: { key: string; unchanged: boolean; blocks: DiffBlock[] }[] = [];
  for (const block of comparison.value.blocks) {
    const unchanged = block.kind === 'same' && !block.moved;
    const previous = result.at(-1);
    if (unchanged && previous?.unchanged) previous.blocks.push(block);
    else result.push({ key: block.key, unchanged, blocks: [block] });
  }
  return result;
});
const counts = computed(() => {
  const blocks = comparison.value.blocks;
  return {
    add: blocks.filter((block) => block.kind === 'add').length,
    delete: blocks.filter((block) => block.kind === 'delete').length,
    change: blocks.filter((block) => block.kind === 'change').length,
    format: blocks.filter((block) => block.kind === 'format').length,
    move: blocks.filter((block) => block.moved).length,
  };
});
function toggleGroup(key: string, event: Event) {
  if ((event.currentTarget as HTMLDetailsElement).open) openGroups.value.add(key);
  else openGroups.value.delete(key);
}
const labels: Record<string, string> = {
  same: '未改动',
  add: '新增',
  delete: '删除',
  change: '修改',
  format: '格式变化',
};

// 图片继续走本轮REVIEW引用鉴权，旧稿图片也由服务端校验base_version_id；不取任意外链src。
const ReviewImage = defineComponent({
  props: { node: { type: Object as PropType<RichNode>, required: true } },
  setup(imageProps) {
    const error = ref(false),
      active = ref(true),
      retry = ref(0);
    watch(transportEpoch, () => {
      active.value = false;
    });
    return () =>
      h('span', { class: 'diff-image' }, [
        active.value &&
          h('img', {
            key: retry.value,
            src: resourceContentURL(
              String(imageProps.node.attrs?.resourceId ?? ''),
              props.resourceContext,
            ),
            alt: String(imageProps.node.attrs?.alt ?? '正文图片'),
            title: String(imageProps.node.attrs?.title ?? ''),
            width: imageProps.node.attrs?.width ?? undefined,
            height: imageProps.node.attrs?.height ?? undefined,
            loading: 'lazy',
            onLoad: () => {
              error.value = false;
            },
            onError: () => {
              error.value = true;
            },
          }),
        (!active.value || error.value) &&
          h('span', { role: 'status', class: 'diff-image-error' }, [
            active.value ? '图片暂不可用。' : '会话已变化，请重新打开审核正文。',
            active.value &&
              h(
                'button',
                {
                  type: 'button',
                  onClick: () => {
                    error.value = false;
                    retry.value++;
                  },
                },
                '重新加载图片',
              ),
          ]),
      ]);
  },
});
const ReviewFormula = defineComponent({
  props: { node: { type: Object as PropType<RichNode>, required: true } },
  setup(formulaProps) {
    const host = ref<HTMLElement | null>(null);
    function render() {
      if (host.value)
        katex.render(String(formulaProps.node.attrs?.latex ?? ''), host.value, {
          trust: false,
          throwOnError: false,
          displayMode: formulaProps.node.type === 'formula_block',
          maxExpand: 1000,
        });
    }
    onMounted(render);
    watch(() => formulaProps.node, render);
    return () =>
      h('span', {
        ref: host,
        class: 'diff-formula',
        title: String(formulaProps.node.attrs?.latex ?? ''),
      });
  },
});

function marked(node: RichNode, child: VNode | string): VNode | string {
  let result = child;
  for (const mark of [...(node.marks ?? [])].reverse()) {
    const tags: Record<string, string> = { strong: 'strong', em: 'em', code: 'code', strike: 's' };
    if (tags[mark.type]) result = h(tags[mark.type], null, [result]);
    // 差异阅读不跳转链接，保留文字和地址，避免点选旧稿触发外部导航。
    else if (mark.type === 'link')
      result = h('span', { class: 'diff-link', title: String(mark.attrs?.href ?? '') }, [result]);
    else if (mark.type === 'font')
      result = h('span', { style: { fontFamily: String(mark.attrs?.family ?? '') } }, [result]);
  }
  return result;
}
function inline(node: RichNode): VNode | string {
  if (node.type === 'text') return marked(node, node.text ?? '');
  if (node.type === 'hard_break') return h('br');
  const child = node.type === 'image' ? h(ReviewImage, { node }) : h(ReviewFormula, { node });
  return h(
    'span',
    {
      class: 'diff-atom',
      style: {
        fontWeight: node.attrs?.atomStrong ? '700' : undefined,
        fontStyle: node.attrs?.atomEm ? 'italic' : undefined,
        textDecoration: node.attrs?.atomStrike ? 'line-through' : undefined,
      },
      title: node.attrs?.atomLink
        ? '链接：' + String((node.attrs.atomLink as { href?: string }).href ?? '')
        : undefined,
    },
    [child],
  );
}
function span(span: DiffSpan) {
  const tag = span.kind === 'add' ? 'ins' : span.kind === 'delete' ? 'del' : 'span';
  const title = span.format.join('；');
  return h(
    tag,
    {
      class: ['diff-span', 'diff-' + span.kind],
      'data-diff-kind': span.kind,
      title: title || undefined,
      'aria-label': title || undefined,
    },
    [inline(span.node)],
  );
}
function nodeTag(node: RichNode) {
  const tags: Record<string, string> = {
    paragraph: 'p',
    blockquote: 'blockquote',
    bullet_list: 'ul',
    ordered_list: 'ol',
    list_item: 'li',
    table: 'table',
    table_row: 'tr',
    table_cell: 'td',
    table_header: 'th',
    code_block: 'pre',
    horizontal_rule: 'hr',
  };
  return node.type === 'heading'
    ? 'h' + Math.min(6, Math.max(1, Number(node.attrs?.level ?? 1)))
    : (tags[node.type] ?? 'div');
}
function nodeAttributes(node: RichNode) {
  return {
    ...(node.type === 'ordered_list' ? { start: Number(node.attrs?.order ?? 1) } : {}),
    ...(['table_cell', 'table_header'].includes(node.type)
      ? { colspan: Number(node.attrs?.colspan ?? 1), rowspan: Number(node.attrs?.rowspan ?? 1) }
      : {}),
    style: {
      ...(node.type === 'paragraph' || node.type === 'heading'
        ? {
            textAlign: ['left', 'center', 'right'].includes(String(node.attrs?.align))
              ? String(node.attrs?.align)
              : 'left',
            textIndent: node.attrs?.firstLineIndent ? '2em' : '0',
          }
        : {}),
    },
  };
}
function plain(node: RichNode, kind: 'add' | 'delete' | 'same' = 'same'): VNode | string {
  if (['text', 'image', 'formula', 'hard_break'].includes(node.type))
    return span({ kind, node, format: [] });
  if (node.type === 'formula_block') return h('div', { class: 'diff-' + kind }, [inline(node)]);
  const children = (node.content ?? []).map((child) => plain(child, kind));
  return h(
    nodeTag(node),
    { ...nodeAttributes(node), class: ['diff-rich-node', 'diff-' + kind] },
    node.type === 'table' ? [h('tbody', null, children)] : children,
  );
}
function rich(block: DiffBlock): VNode | string {
  const node = block.after ?? block.before!;
  if (block.kind === 'add' || block.kind === 'delete') return plain(node, block.kind);
  if (['image', 'formula', 'formula_block', 'hard_break'].includes(node.type)) {
    return block.kind === 'change'
      ? h('div', { class: 'diff-atom-replacement' }, [
          span({ kind: 'delete', node: block.before!, format: [] }),
          span({ kind: 'add', node, format: [] }),
        ])
      : span({
          kind: block.kind === 'format' ? 'format' : 'same',
          node,
          before: block.before,
          format: block.format,
        });
  }
  const children = block.spans ? block.spans.map(span) : (block.children ?? []).map(rich);
  const annotation = [
    ...block.format,
    ...(block.moved ? ['位置：第 ' + block.moved.from + ' 块 → 第 ' + block.moved.to + ' 块'] : []),
  ].join('；');
  const metadata =
    block.format.length &&
    !['table', 'table_row', 'ordered_list', 'bullet_list'].includes(node.type)
      ? [h('span', { class: 'diff-format-note' }, block.format.join('；'))]
      : [];
  return h(
    nodeTag(node),
    {
      ...nodeAttributes(node),
      key: block.key,
      class: [
        'diff-rich-node',
        block.kind === 'format' ? 'diff-format-block' : '',
        block.moved ? 'diff-moved-block' : '',
      ],
      'data-diff-kind': block.kind,
      title: annotation || undefined,
    },
    node.type === 'table' ? [h('tbody', null, children)] : [...metadata, ...children],
  );
}
const RichBlock = defineComponent({
  props: { block: { type: Object as PropType<DiffBlock>, required: true } },
  setup(blockProps) {
    return () => rich(blockProps.block);
  },
});
</script>

<template>
  <section class="review-diff" role="region" aria-label="审核正文差异">
    <p class="diff-context">本轮固定基线 → 本轮送审稿</p>
    <p class="diff-legend" aria-label="差异图例">
      <span class="diff-add">新增 {{ counts.add }} 块</span>
      <span class="diff-delete">删除 {{ counts.delete }} 块</span>
      <span>修改 {{ counts.change }} 块</span>
      <span class="diff-format">格式变化 {{ counts.format }} 块</span>
      <span v-if="counts.move">移动 {{ counts.move }} 块</span>
    </p>
    <p class="diff-instruction">
      绿色为新增，红色删除线为原文，蓝色为格式变化。要选择文字发表评论，请切换到完整送审稿。
    </p>
    <p v-if="comparison.limited" class="diff-limit" role="status">
      部分段落改动较大，已保留旧文与新文，未逐字细分。未改动部分仍可展开核对。
    </p>
    <p v-if="!Object.values(counts).some(Boolean)" class="diff-empty">本轮正文与固定基线一致。</p>
    <template v-for="group in groups" :key="group.key">
      <details
        v-if="group.unchanged"
        class="diff-unchanged"
        @toggle="toggleGroup(group.key, $event)"
      >
        <summary>未改动内容（{{ group.blocks.length }} 块）</summary>
        <div v-if="openGroups.has(group.key)" class="diff-unchanged-body">
          <RichBlock v-for="block in group.blocks" :key="block.key" :block="block" />
        </div>
      </details>
      <article
        v-else
        v-for="block in group.blocks"
        :key="block.key"
        class="diff-block"
        :data-diff-kind="block.kind"
      >
        <div class="diff-block-label">
          <span :class="'diff-label-' + block.kind">{{ labels[block.kind] }}</span>
          <span v-if="block.moved" class="diff-move"
            >移动：第 {{ block.moved.from }} 块 → 第 {{ block.moved.to }} 块</span
          >
          <span v-if="block.format.length" class="diff-format">{{ block.format.join('；') }}</span>
          <span v-if="block.limited">该段按旧文/新文展示</span>
        </div>
        <RichBlock :block="block" />
      </article>
    </template>
  </section>
</template>

<style scoped>
.review-diff {
  padding: 4px 0 18px;
  color: var(--ink);
  overflow-wrap: anywhere;
}
.diff-context {
  margin: 0 0 9px;
  font-weight: 600;
}
.diff-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  font-size: var(--ui-font-meta);
}
.diff-instruction,
.diff-limit,
.diff-empty {
  color: var(--muted);
  font-size: var(--ui-font-meta);
}
.diff-limit {
  padding: 10px;
  background: #fff4d9;
  color: #715013;
}
.diff-block {
  margin: 16px 0;
  padding: 12px 14px;
  border: 1px solid var(--line);
  border-radius: 6px;
}
.diff-block-label {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  margin-bottom: 8px;
  font-size: var(--ui-font-meta);
  color: var(--muted);
}
.diff-unchanged {
  margin: 14px 0;
  border: 1px dashed var(--line);
  border-radius: 6px;
}
.diff-unchanged > summary {
  padding: 11px 13px;
  cursor: pointer;
  color: var(--muted);
}
.diff-unchanged-body {
  padding: 0 14px 12px;
}
.review-diff :deep(.diff-add),
.diff-add,
.diff-label-add {
  color: #176337;
  background: #e1f4e7;
  text-decoration: none;
}
.review-diff :deep(.diff-delete),
.diff-delete,
.diff-label-delete {
  color: #a3262c;
  background: #fce6e6;
  text-decoration: line-through;
}
.review-diff :deep(.diff-format),
.diff-format,
.diff-label-format {
  color: #17599a;
  background: #e5f0ff;
}
.review-diff :deep(.diff-format-block) {
  box-shadow: inset 3px 0 #477eb7;
  padding-left: 10px;
}
.review-diff :deep(.diff-format-note) {
  display: block;
  margin-bottom: 6px;
  color: #17599a;
  font: var(--ui-font-meta)/1.6 sans-serif;
}
.review-diff :deep(.diff-span) {
  white-space: pre-wrap;
}
.review-diff :deep(.diff-rich-node) {
  margin-top: 8px;
  margin-bottom: 8px;
}
.review-diff :deep(p) {
  line-height: 1.8;
}
.review-diff :deep(h1),
.review-diff :deep(h2),
.review-diff :deep(h3) {
  line-height: 1.5;
}
.review-diff :deep(table) {
  display: block;
  max-width: 100%;
  overflow-x: auto;
  border-collapse: collapse;
}
.review-diff :deep(td),
.review-diff :deep(th) {
  min-width: 80px;
  border: 1px solid var(--line);
  padding: 8px;
  vertical-align: top;
}
.review-diff :deep(th) {
  background: #f2f3ee;
}
.review-diff :deep(pre) {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  background: #f2f3ee;
  padding: 10px;
}
.review-diff :deep(blockquote) {
  margin-left: 0;
  padding-left: 14px;
  border-left: 3px solid var(--line);
}
.review-diff :deep(.diff-image) {
  display: inline-flex;
  flex-direction: column;
  max-width: 100%;
  padding: 3px;
}
.review-diff :deep(img) {
  max-width: 100%;
  height: auto;
  object-fit: contain;
}
.review-diff :deep(.diff-image-error) {
  font-size: var(--ui-font-meta);
}
.review-diff :deep(.diff-atom-replacement) {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.review-diff :deep(.diff-atom) {
  display: inline-block;
  max-width: 100%;
  vertical-align: middle;
}
.review-diff :deep(.diff-link) {
  text-decoration: underline;
  text-underline-offset: 3px;
}
.review-diff :deep(.katex-display) {
  max-width: 100%;
  overflow-x: auto;
}
.diff-move {
  color: #705412;
}
@media (max-width: 520px) {
  .diff-block {
    padding: 10px;
  }
}
</style>
