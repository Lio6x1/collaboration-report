# services/collaboration/src/domain/room_replay.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/300-room-replay.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`openCheckpoint`、`applyDurableLog`、`encodeCheckpoint`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`45796d40259d7149946f84726b1bce727fc0c96cb566771b223062a041f0b3d2`。[参考实现](../../../../services/collaboration/src/domain/room_replay.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
