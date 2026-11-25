# services/collaboration/src/domain/merge/values.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/295-values.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`semantic`、`hasFont`、`MergeValues`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`2f90858ca4f3ac101944b4da0ee392f072ac3b8e687da5df59f445432483c276`。[参考实现](../../../../services/collaboration/src/domain/merge/values.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
