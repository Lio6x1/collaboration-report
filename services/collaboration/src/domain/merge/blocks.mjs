import { clone, equal, same, semantic, hasFont } from './values.mjs';
import { mergeInline } from './inline.mjs';

// 编辑正文以平铺heading保存；计算时临时成树，删除标题才会检查整个子树，而不是遗漏子段落。
export function headingTree(document) {
  const root = { id: '$root', children: [] },
    stack = [root];
  for (const value of document.content) {
    const entity = { id: value.attrs.id, value };
    if (value.type === 'heading') {
      entity.children = [];
      while (stack.length > 1 && stack.at(-1).value.attrs.level >= value.attrs.level) stack.pop();
      stack.at(-1).children.push(entity);
      stack.push(entity);
    } else stack.at(-1).children.push(entity);
  }
  return root;
}
export const flattenHeadings = (entities) =>
  entities.flatMap((entity) => [entity.value, ...flattenHeadings(entity.children ?? [])]);
function parentMap(entity, parent = '$root', result = new Map()) {
  if (entity.value) result.set(entity.id, parent);
  for (const child of entity.children ?? []) parentMap(child, entity.id, result);
  return result;
}

export class MergeBlocks {
  constructor(values) {
    this.values = values;
  }

  roots(base, submitted, main) {
    const b = headingTree(base),
      s = headingTree(submitted),
      m = headingTree(main);
    const bp = parentMap(b),
      sp = parentMap(s),
      mp = parentMap(m);
    for (const [id, parent] of bp)
      if ((sp.has(id) && sp.get(id) !== parent) || (mp.has(id) && mp.get(id) !== parent))
        this.values.report(
          'CROSS_PARENT_STRUCTURE',
          id,
          id,
          '章节层级或跨父范围移动需要明确结构处理。',
        );
    return flattenHeadings(this.children(b.children, s.children, m.children, '$root'));
  }

  node(base, submitted, main, path) {
    const block = base.attrs?.id ?? main.attrs?.id ?? submitted.attrs?.id ?? '';
    if (same(base, submitted)) return clone(main);
    if (same(main, submitted)) return clone(main);
    if (equal(base, main) && !hasFont(main)) return semantic(submitted);
    if (base.type !== main.type || base.type !== submitted.type) {
      this.values.report('NODE_TYPE', block, path, '同一块发生了不能安全组合的节点类型修改。');
      return clone(main);
    }
    const attrs = this.values.attributes(
      base.attrs,
      submitted.attrs,
      main.attrs,
      block,
      `${path}.attrs`,
    );
    if (['paragraph', 'heading', 'code_block'].includes(base.type))
      return {
        type: base.type,
        attrs,
        content: mergeInline(this.values, base, submitted, main, block, path),
      };
    if (
      [
        'table',
        'table_row',
        'table_cell',
        'table_header',
        'bullet_list',
        'ordered_list',
        'list_item',
        'blockquote',
      ].includes(base.type)
    ) {
      const wrap = (nodes) => (nodes ?? []).map((value) => ({ id: value.attrs.id, value }));
      return {
        type: base.type,
        attrs,
        content: this.children(
          wrap(base.content),
          wrap(submitted.content),
          wrap(main.content),
          path,
        ).map((entity) => entity.value),
      };
    }
    return { ...clone(main), attrs };
  }

  entity(base, submitted, main, path) {
    if (!base) {
      if (!main) return semantic(submitted);
      if (!submitted || same(main, submitted)) return clone(main);
      this.values.report('NEW_ID_COLLISION', main.id, path, '同一新块身份在两侧产生不同内容。');
      return clone(main);
    }
    if (!main || !submitted) {
      if (!main && !submitted) return null;
      if (same(base, main ?? submitted)) return null;
      this.values.report('DELETE_EDIT', base.id, path, '已删除的块或章节子树在另一侧仍有修改。');
      return main ? clone(main) : null;
    }
    return {
      id: base.id,
      value: this.node(base.value, submitted.value, main.value, path),
      ...(base.children
        ? {
            children: this.children(
              base.children,
              submitted.children ?? [],
              main.children ?? [],
              path,
            ),
          }
        : {}),
    };
  }

  children(base, submitted, main, path) {
    const map = (nodes) => new Map(nodes.map((node) => [node.id, node]));
    const b = map(base),
      s = map(submitted),
      m = map(main),
      merged = new Map();
    for (const id of new Set([...b.keys(), ...s.keys(), ...m.keys()])) {
      const entity = this.entity(b.get(id), s.get(id), m.get(id), `${path}/${id}`);
      if (entity) merged.set(id, entity);
    }
    const original = (nodes) =>
      nodes.map((node) => node.id).filter((id) => b.has(id) && merged.has(id));
    const bo = original(base),
      so = original(submitted),
      mo = original(main);
    if (bo.length !== mo.length || bo.length !== so.length)
      return main.map((node) => merged.get(node.id)).filter(Boolean);
    let order = equal(bo, mo) ? so : mo;
    if (!equal(bo, mo) && !equal(bo, so) && !equal(mo, so)) {
      this.values.report(
        'ORDER_CONFLICT',
        b.get(bo[0])?.id ?? '',
        path,
        '双方对原有块提出不同排序。',
      );
      order = mo;
    }
    const additions = new Map(),
      orderPosition = new Map(order.map((id, i) => [id, i]));
    for (const [side, nodes] of [
      ['MAIN', main],
      ['SUBMITTED', submitted],
    ]) {
      const nextAnchor = new Array(nodes.length);
      let after, before;
      for (let i = nodes.length - 1; i >= 0; i--) {
        nextAnchor[i] = after;
        if (b.has(nodes[i].id) && merged.has(nodes[i].id)) after = nodes[i].id;
      }
      for (let i = 0; i < nodes.length; i++) {
        const id = nodes[i].id;
        if (b.has(id) && merged.has(id)) before = id;
        if (b.has(id) || !merged.has(id)) continue;
        const next = nextAnchor[i];
        if (before && next && orderPosition.get(before) >= orderPosition.get(next))
          this.values.report('INSERT_ANCHOR', id, path, '新增块的前后锚点因移序互相矛盾。');
        const slot = before ? orderPosition.get(before) + 1 : next ? orderPosition.get(next) : 0;
        if (!additions.has(slot)) additions.set(slot, { MAIN: [], SUBMITTED: [] });
        additions.get(slot)[side].push(id);
      }
    }
    const result = [],
      emitted = new Set();
    const emit = (id) => {
      if (!emitted.has(id)) {
        emitted.add(id);
        result.push(merged.get(id));
      }
    };
    for (let i = 0; i <= order.length; i++) {
      const added = additions.get(i);
      if (added) {
        if (added.MAIN.length && added.SUBMITTED.length && !equal(added.MAIN, added.SUBMITTED))
          this.values.report(
            'INSERT_ORDER',
            added.MAIN[0],
            `${path}@${i}`,
            '同一结构位置的两组新增块没有可确认顺序。',
          );
        [...added.MAIN, ...added.SUBMITTED].forEach(emit);
      }
      if (i < order.length) emit(order[i]);
    }
    return result;
  }
}
