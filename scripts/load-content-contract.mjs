import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { isDeepStrictEqual } from 'node:util';

// 冻结正文契约只能追加这两个已确认的段落属性；扩展不能修改类型、required、
// 其它节点或additionalProperties。约束与Go加载器一致，schema定义来自同一扩展文件。
export function mergeParagraphFormattingExtension(baseline, extension) {
  const allowed = {
    version: 'paragraph-formatting-v1',
    nodes: ['paragraph', 'heading'],
    attributes: {
      align: { enum: ['left', 'center', 'right'] },
      firstLineIndent: { type: 'boolean' },
    },
  };
  if (!isDeepStrictEqual(extension, allowed)) throw Error('CONTENT_PARAGRAPH_EXTENSION_INVALID');
  const merged = structuredClone(baseline);
  for (const name of extension.nodes) {
    const attrs = merged?.$defs?.[name]?.properties?.attrs;
    if (
      attrs?.type !== 'object' ||
      attrs.additionalProperties !== false ||
      !attrs.properties ||
      Object.keys(extension.attributes).some((key) => Object.hasOwn(attrs.properties, key))
    )
      throw Error('CONTENT_PARAGRAPH_EXTENSION_OVERRIDE');
    Object.assign(attrs.properties, structuredClone(extension.attributes));
  }
  return merged;
}

export function loadContentContract(root) {
  const read = (name) => JSON.parse(readFileSync(resolve(root, 'contracts/content', name), 'utf8'));
  return mergeParagraphFormattingExtension(
    read('content.schema.json'),
    read('paragraph-formatting.extension.json'),
  );
}
