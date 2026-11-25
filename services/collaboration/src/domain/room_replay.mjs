import * as Y from 'yjs';
import { timingSafeEqual } from 'node:crypto';
import { inspectDocument } from '../../../../packages/content/index.mjs';
import { hash } from './shared_protocol.mjs';

// 检查点先验证再应用；损坏字节不能被后续日志掩盖，也不能合成一个空房间。
export function openCheckpoint(checkpoint) {
  if (
    !checkpoint ||
    checkpoint.format_version !== 'collab-yjs-v1' ||
    !timingSafeEqual(hash(checkpoint.yjs_state), checkpoint.checksum)
  )
    throw Error('CHECKPOINT_INVALID');
  const doc = new Y.Doc();
  try {
    Y.applyUpdate(doc, checkpoint.yjs_state);
    return doc;
  } catch (error) {
    doc.destroy();
    throw error;
  }
}

// 纯规则只应用指定cut的连续持久日志；Y.Doc由调用用例负责归还或释放。
export function applyDurableLog(doc, covered, updates, room, cut) {
  let sequence = BigInt(covered);
  for (const update of updates) {
    if (
      BigInt(update.seq) !== ++sequence ||
      !timingSafeEqual(hash(update.payload), update.payload_hash)
    )
      throw Error('DURABLE_LOG_INVALID');
    Y.applyUpdate(doc, update.payload);
  }
  if (sequence !== BigInt(cut)) throw Error('DURABLE_LOG_GAP');
  // 恢复文档本就独立于在线房间；完整检查沿共享规则直接只读执行，避免再次克隆正文。
  inspectDocument(doc, {
    workspaceKind: room.workspace_kind,
  });
  return doc;
}

export function encodeCheckpoint(doc) {
  const bytes = Buffer.from(Y.encodeStateAsUpdate(doc));
  return { bytes, checksum: hash(bytes) };
}
