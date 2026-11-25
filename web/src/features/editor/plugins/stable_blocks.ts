import { Plugin } from 'prosemirror-state';
import { ySyncPluginKey } from 'y-prosemirror';
// 每个视图拥有独立插件；仅处理本地新块，远端非法身份必须继续由服务端拒绝。
export function createStableBlocks() {
  return new Plugin({
    appendTransaction(transactions, _old, state) {
      if (
        !transactions.some((t) => t.docChanged) ||
        transactions.some((t) => t.getMeta(ySyncPluginKey)?.isChangeOrigin)
      )
        return;
      // Enter/粘贴会复制父块属性。只为本地新块分配身份，服务端仍拒绝恶意重复，不能修复远端正文。
      const used = new Set<string>(),
        tr = state.tr;
      state.doc.descendants((node, pos) => {
        if (node.isBlock) {
          const id = node.attrs.id as string | null;
          if (!id || used.has(id)) {
            const fresh = crypto.randomUUID();
            tr.setNodeMarkup(pos, undefined, { ...node.attrs, id: fresh });
            used.add(fresh);
          } else used.add(id);
        }
      });
      return tr.docChanged ? tr : undefined;
    },
  });
}
