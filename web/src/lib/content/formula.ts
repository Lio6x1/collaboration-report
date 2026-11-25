import katex from 'katex';
import 'katex/dist/katex.min.css';
// 阅读和编辑共享原子公式视图；渲染不解释权限，禁用trust并限制展开，独立格式字段各自呈现。
export function formulaNodeView(node: any) {
  const element = document.createElement(node.type.name === 'formula_block' ? 'div' : 'span');
  element.className = 'formula-atom';
  element.dataset.formula = node.attrs.latex;
  function render(n: any) {
    element.style.fontWeight = n.attrs.atomStrong ? '700' : '';
    element.style.fontStyle = n.attrs.atomEm ? 'italic' : '';
    element.style.textDecoration = n.attrs.atomStrike ? 'line-through' : '';
    katex.render(n.attrs.latex, element, {
      throwOnError: false,
      trust: false,
      displayMode: n.type.name === 'formula_block',
      maxExpand: 1000,
    });
    element.title = n.attrs.latex;
  }
  render(node);
  return {
    dom: element,
    update(next: any) {
      if (next.type !== node.type) return false;
      node = next;
      render(next);
      return true;
    },
  };
}
