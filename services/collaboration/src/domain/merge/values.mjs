import { canonical } from '../../../../../packages/content/index.mjs';

export const clone = (value) => (value === undefined ? undefined : structuredClone(value));
export const equal = (left, right) =>
  left === undefined || right === undefined ? left === right : canonical(left) === canonical(right);

// 分支没有正文font权限；主线font不构成送审语义差异，但最终结果必须保留它。
export function semantic(value) {
  if (Array.isArray(value)) return value.map(semantic);
  if (!value || typeof value !== 'object') return value;
  const result = {};
  for (const [key, child] of Object.entries(value)) {
    const next =
      key === 'marks'
        ? child.filter((mark) => mark.type !== 'font').map(semantic)
        : semantic(child);
    if (key !== 'marks' || next.length) result[key] = next;
  }
  return result;
}
export const same = (left, right) => equal(semantic(left), semantic(right));
export function hasFont(node) {
  return (
    (node.marks ?? []).some((mark) => mark.type === 'font') || (node.content ?? []).some(hasFont)
  );
}

export class MergeValues {
  constructor() {
    this.conflicts = [];
  }

  report(kind, block, path, reason) {
    this.conflicts.push({ block_id: block, node_path: path, kind, reason });
  }

  scalar(base, submitted, main, block, path) {
    if (equal(main, submitted) || equal(base, submitted)) return clone(main);
    if (equal(base, main)) return clone(submitted);
    this.report('ATTRIBUTE_CONFLICT', block, path, '同一字段在主线和送审中发生了不同修改。');
    return clone(main);
  }

  attributes(base = {}, submitted = {}, main = {}, block, path) {
    const result = {};
    for (const key of new Set([
      ...Object.keys(base),
      ...Object.keys(submitted),
      ...Object.keys(main),
    ])) {
      const value = this.scalar(base[key], submitted[key], main[key], block, `${path}.${key}`);
      if (value !== undefined) result[key] = value;
    }
    return result;
  }

  marks(base, submitted, main, block, path) {
    const map = (token) =>
      Object.fromEntries((token?.marks ?? []).map((mark) => [mark.type, mark]));
    const b = map(base),
      s = map(submitted),
      m = map(main),
      result = [];
    for (const kind of [
      ...new Set([...Object.keys(b), ...Object.keys(s), ...Object.keys(m)]),
    ].sort()) {
      let mark;
      if (kind === 'font') mark = clone(m.font);
      else if (b[kind]?.attrs && s[kind]?.attrs && m[kind]?.attrs)
        mark = {
          type: kind,
          attrs: this.attributes(
            b[kind].attrs,
            s[kind].attrs,
            m[kind].attrs,
            block,
            `${path}.marks.${kind}.attrs`,
          ),
        };
      else mark = this.scalar(b[kind], s[kind], m[kind], block, `${path}.marks.${kind}`);
      if (mark) result.push(mark);
    }
    return result;
  }

  token(base, submitted, main, block, path) {
    if (main.type === 'text')
      return { ...clone(main), marks: this.marks(base, submitted, main, block, path) };
    // hard_break没有attrs；不能凭合并层的通用对象形状添加schema不允许的空字段。
    if (!base?.attrs && !submitted.attrs && !main.attrs) return clone(main);
    return {
      ...clone(main),
      attrs: this.attributes(base?.attrs, submitted.attrs, main.attrs, block, `${path}.attrs`),
    };
  }
}
