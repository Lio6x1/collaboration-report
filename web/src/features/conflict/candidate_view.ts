import { EditorState } from 'prosemirror-state';
import { EditorView } from 'prosemirror-view';
import { keymap } from 'prosemirror-keymap';
import { baseKeymap } from 'prosemirror-commands';
import { tableEditing } from 'prosemirror-tables';
import { schema, toPM } from '../../../../packages/content/index.mjs';
import { createStableBlocks } from '../editor/plugins/stable_blocks';
import { pasteHTML } from '../editor/plugins/paste';
import { formulaNodeView } from '../../lib/content/formula';
import { imageNodeView, type ResourceContextSource } from '../../lib/content/image';
import { toolbarSelectionPlugin } from '../editor/commands';
import { codeNodeView, codeHighlightPlugin, codeKeymap } from '../../lib/content/code';

// 人工候选只是一份独立PM状态；没有协作控制器、Y.Doc、WSS或共同支线队列。
// 原块ID进入原样副本，新块沿同一稳定身份插件生成，不能为修复便利重建全部身份。
export function createCandidateView(
  host: HTMLElement,
  body: object,
  locked: () => boolean,
  composing: (value: boolean) => void,
  loss: (value: string) => void,
  resourceContext: ResourceContextSource,
  selectionChanged: (state: EditorState) => void = () => {},
) {
  return new EditorView(host, {
    state: EditorState.create({
      schema,
      doc: schema.nodeFromJSON(toPM(body)),
      plugins: [
        createStableBlocks(),
        codeHighlightPlugin(),
        keymap(codeKeymap),
        keymap(baseKeymap),
        tableEditing(),
        toolbarSelectionPlugin(selectionChanged),
      ],
    }),
    editable: () => !locked(),
    attributes: { role: 'textbox', 'aria-label': '人工修复候选编辑器', 'aria-multiline': 'true' },
    nodeViews: {
      formula: formulaNodeView,
      formula_block: formulaNodeView,
      image: imageNodeView(resourceContext),
      code_block: codeNodeView,
    },
    handleDOMEvents: {
      compositionstart: () => {
        composing(true);
        return false;
      },
      compositionend: () => {
        composing(false);
        return false;
      },
    },
    handlePaste: (editor, event) => pasteHTML(editor, event, loss),
  });
}
