import * as Y from 'yjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import { fromPM, normalizeContent } from '../../../../../packages/content/index.mjs';
import { decode } from './bytes';
import type { DraftQueue } from './model';

// 旧输入只解析为独立副本供查看；不能把旧Y.Doc直接接到当前epoch。
export function reconstructStaleBody(stale: DraftQueue | null, kind: DraftQueue['kind']) {
  if (!stale) return null;
  const old = new Y.Doc();
  try {
    Y.applyUpdate(old, decode(stale.snapshot));
    return normalizeContent(fromPM(yDocToProsemirrorJSON(old)), { workspaceKind: kind });
  } finally {
    old.destroy();
  }
}
export function bodyText(body: any): string {
  return (
    body.text ??
    body.attrs?.latex ??
    (body.content ?? []).map(bodyText).join(body.type === 'doc' ? '\n' : '')
  );
}
