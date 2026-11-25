# services/collaboration/src/domain/merge/inline.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/294-inline.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`mergeInline`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { diff } from 'lib0/diff/patience';
import { clone, equal, semantic, same } from './values.mjs';

const segmenter = new Intl.Segmenter('zh', { granularity: 'grapheme' });
const tokenKey = (token) => (token.type === 'text' ? 'text:' + token.text : 'atom:' + token.type);
function tokens(node) {
  return (node.content ?? []).flatMap((child) =>
    child.type === 'text'
      ? [...segmenter.segment(child.text)].map(({ segment }) => ({
          ...clone(child),
          text: segment,
        }))
      : [clone(child)],
  );
}
function changes(base, side) {
  const mapping = new Map();
  let bi = 0,
    si = 0;
  const hunks = diff(base.map(tokenKey), side.map(tokenKey)).map((hunk) => {
    while (bi < hunk.index) mapping.set(bi++, si++);
    const result = {
      start: bi,
      end: bi + hunk.remove.length,
      insert: side.slice(si, si + hunk.insert.length),
    };
    bi += hunk.remove.length;
    si += hunk.insert.length;
    return result;
  });
  while (bi < base.length) mapping.set(bi++, si++);
  return { mapping, hunks };
}
function overlap(left, right) {
  if (left.start === left.end && right.start === right.end) return left.start === right.start;
  if (left.start === left.end) return left.start >= right.start && left.start <= right.end;
  if (right.start === right.end) return right.start >= left.start && right.start <= left.end;
  return left.start < right.end && right.start < left.end;
}
function untokenize(list) {
  const result = [];
  for (const token of list) {
    const node = clone(token);
    if (!node.marks?.length) delete node.marks;
    const previous = result.at(-1);
    if (node.type === 'text' && previous?.type === 'text' && equal(previous.marks, node.marks))
      previous.text += node.text;
    else result.push(node);
  }
  return result;
}

export function mergeInline(values, baseNode, submittedNode, mainNode, block, path) {
  if (same(baseNode.content, submittedNode.content)) return clone(mainNode.content ?? []);
  // 先按UTF-16长度保守估算，再分字素。否则仅为检查预算先分词也会阻塞事件循环。
  const estimate = [baseNode, submittedNode, mainNode].reduce(
    (total, node) =>
      total +
      (node.content ?? []).reduce(
        (size, child) => size + (child.type === 'text' ? child.text.length : 1),
        0,
      ),
    0,
  );
  if (estimate > 200000) {
    values.report(
      'MERGE_COMPLEXITY_LIMIT',
      block,
      path,
      '本段双侧改动超出自动合并计算预算，请分段处理。',
    );
    return [];
  }
  // diff API是锁定版本的实验接口；以字素作差异单位，不切断emoji或组合字符。
  const base = tokens(baseNode),
    submitted = tokens(submittedNode),
    main = tokens(mainNode);
  const sc = changes(base, submitted),
    mc = changes(base, main),
    duplicates = new Set();
  for (const [index, sh] of sc.hunks.entries()) {
    for (const mh of mc.hunks) {
      if (
        sh.start === mh.start &&
        sh.end === mh.end &&
        equal(sh.insert.map(tokenKey), mh.insert.map(tokenKey))
      ) {
        duplicates.add(index);
        mh.insert = mh.insert.map((token, i) =>
          values.token(undefined, sh.insert[i], token, block, `${path}@${sh.start}+${i}`),
        );
      } else if (overlap(sh, mh)) {
        values.report('TEXT_OVERLAP', block, path, '同一字素范围被不同修改或同位置插入占用。');
        return [];
      }
    }
  }
  for (let i = 0; i < base.length; i++) {
    const si = sc.mapping.get(i),
      mi = mc.mapping.get(i);
    if (
      (si === undefined && mi !== undefined && !same(base[i], main[mi])) ||
      (mi === undefined && si !== undefined && !same(base[i], submitted[si]))
    ) {
      values.report(
        'DELETE_FORMAT',
        block,
        `${path}@${i}`,
        '删除或替换会丢弃另一侧保留内容上的格式或原子字段修改。',
      );
      return [];
    }
  }
  const fontAt = (index) => {
    const markAt = (i) => main[mc.mapping.get(i)]?.marks?.find((mark) => mark.type === 'font');
    if (mc.mapping.has(index)) return markAt(index);
    for (let i = index - 1; i >= 0; i--) if (mc.mapping.has(i)) return markAt(i);
    for (let i = index + 1; i < base.length; i++) if (mc.mapping.has(i)) return markAt(i);
    return undefined;
  };
  const hunks = [
    ...mc.hunks.map((h) => ({ ...h, side: 'MAIN' })),
    ...sc.hunks.filter((_, i) => !duplicates.has(i)).map((h) => ({ ...h, side: 'SUBMITTED' })),
  ].sort((a, b) => a.start - b.start);
  const result = [];
  let position = 0;
  const keep = (i) =>
    values.token(
      base[i],
      submitted[sc.mapping.get(i)],
      main[mc.mapping.get(i)],
      block,
      `${path}@${i}`,
    );
  for (const hunk of hunks) {
    while (position < hunk.start) result.push(keep(position++));
    const font = fontAt(hunk.start);
    result.push(
      ...hunk.insert.map((token) => {
        if (hunk.side === 'MAIN' || token.type !== 'text') return clone(token);
        return {
          ...clone(token),
          marks: [...(semantic(token).marks ?? []), ...(font ? [clone(font)] : [])],
        };
      }),
    );
    position = hunk.end;
  }
  while (position < base.length) result.push(keep(position++));
  return untokenize(result);
}
````

UTF-8/LF全文SHA-256：`b1e834fe98d909967fcfeb4eb3603925495a6f2e546564d362b2e1eac30d47a5`。[参考实现](../../../../services/collaboration/src/domain/merge/inline.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
