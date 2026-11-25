import { DOMParser as PMParser } from 'prosemirror-model';
import type { EditorView } from 'prosemirror-view';
import { schema } from '../../../../../packages/content/index.mjs';
// 外部HTML在浏览器规范化；未READY图片明确保留为文本，不能绕过Doc资源保护。
export function pasteHTML(
  editor: EditorView,
  event: ClipboardEvent,
  reportLoss: (value: string) => void,
  uploadImage?: (file: File, editor: EditorView) => void,
) {
  if (!editor.editable) return true;
  // 代码块按PM原生代码粘贴接收纯文本，保留缩进/换行；富HTML不能把代码变成正文节点。
  if (editor.state.selection.$from.parent.type.name === 'code_block') return false;
  const html = event.clipboardData?.getData('text/html');
  if (html && html.length > 8 * 1024 * 1024) {
    reportLoss('粘贴内容过大，请拆分后导入');
    return true;
  }
  const template = document.createElement('template');
  template.innerHTML = html ?? '';
  const files = Array.from(event.clipboardData?.files ?? []);
  // Word等来源可能同时提供完整图文HTML和一个图片文件；必须保留文字，不能误当成纯图片吞掉。
  const mixedHTML = Boolean(template.content.textContent?.trim());
  if (!mixedHTML && files.some((file) => file.type.startsWith('image/'))) {
    if (files.length !== 1) {
      reportLoss('请一次粘贴一张图片，上一张完成后可继续粘贴');
      return true;
    }
    if (!uploadImage) {
      reportLoss('此编辑器暂不支持上传新图片，请先在正文编辑器中上传');
      return true;
    }
    // 剪贴板字节必须先走资源上传/完整性确认，不能把blob或data URL直接保存为正文。
    uploadImage(files[0], editor);
    return true;
  }
  if (!html) return false;
  let changed = false;
  for (const node of Array.from(
    template.content.querySelectorAll('script,iframe,object,embed,style'),
  )) {
    node.remove();
    changed = true;
  }
  for (const image of Array.from(template.content.querySelectorAll('img'))) {
    if (!/^[1-9][0-9]{0,19}$/.test(image.getAttribute('data-resource-id') ?? '')) {
      image.replaceWith(
        document.createTextNode(
          '[图片待转存：' +
            (image.getAttribute('alt') ?? image.getAttribute('src') ?? '图片') +
            ']',
        ),
      );
      changed = true;
    }
  }
  for (const link of Array.from(template.content.querySelectorAll('a')))
    if (!/^(https?:\/\/|mailto:)/i.test(link.getAttribute('href') ?? '')) {
      link.removeAttribute('href');
      changed = true;
    }
  if (template.content.querySelector('[style*="font"],[style*="margin"],[style*="page"]'))
    changed = true;
  if (changed) reportLoss('粘贴已规范化：外部图片保留为待转存文本，字体和页面样式不写入共同支线');
  editor.dispatch(
    editor.state.tr
      .replaceSelection(PMParser.fromSchema(schema).parseSlice(template.content))
      .scrollIntoView(),
  );
  return true;
}
