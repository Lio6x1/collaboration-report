import { EditorState } from 'prosemirror-state';
import { EditorView } from 'prosemirror-view';
import { keymap } from 'prosemirror-keymap';
import { baseKeymap } from 'prosemirror-commands';
import { tableEditing } from 'prosemirror-tables';
import { ySyncPlugin, yUndoPlugin, undo, redo, yDocToProsemirrorJSON } from 'y-prosemirror';
import { schema } from '../../../../packages/content/index.mjs';
import type { DraftController } from './controller';
import { createStableBlocks } from './plugins/stable_blocks';
import { pasteHTML } from './plugins/paste';
import { formulaNodeView } from '../../lib/content/formula';
import { imageNodeView, type ResourceContextSource } from '../../lib/content/image';
import { toolbarSelectionPlugin } from './commands';
import { codeNodeView, codeHighlightPlugin, codeKeymap } from '../../lib/content/code';

// 视图只绑定当前Y.Doc、编辑命令与DOM事件，持久队列和新epoch安装由控制器管理。
export function createDraftView(
  host: HTMLElement,
  controller: DraftController,
  reportLoss: (value: string) => void,
  resourceContext: ResourceContextSource,
  resourceSaved: EventTarget,
  uploadImage: (file: File, editor: EditorView) => void,
  editingBlocked: () => boolean = () => false,
  selectionChanged: (state: EditorState) => void = () => {},
) {
  return new EditorView(host, {
    state: EditorState.create({
      schema,
      doc: schema.nodeFromJSON(yDocToProsemirrorJSON(controller.doc)),
      plugins: [
        ySyncPlugin(controller.doc.getXmlFragment('prosemirror')),
        yUndoPlugin(),
        createStableBlocks(),
        codeHighlightPlugin(),
        keymap({ 'Mod-z': undo, 'Mod-y': redo, 'Mod-Shift-z': redo }),
        keymap(codeKeymap),
        keymap(baseKeymap),
        tableEditing(),
        toolbarSelectionPlugin(selectionChanged),
      ],
    }),
    editable: () =>
      !controller.stopped &&
      !controller.stale &&
      !controller.loading &&
      (!controller.structurePause || controller.composing) &&
      !editingBlocked(),
    attributes: {
      role: 'textbox',
      'aria-label': controller.owner.kind === 'MAIN' ? '主线编辑器' : '共同支线编辑器',
      'aria-multiline': 'true',
    },
    nodeViews: {
      formula: formulaNodeView,
      formula_block: formulaNodeView,
      image: imageNodeView(resourceContext, resourceSaved, true),
      code_block: codeNodeView,
    },
    handleDOMEvents: {
      compositionstart: () => {
        controller.setComposing(true);
        return false;
      },
      compositionend: () => {
        controller.setComposing(false);
        return false;
      },
    },
    handlePaste: (editor, event) => pasteHTML(editor, event, reportLoss, uploadImage),
  });
}
