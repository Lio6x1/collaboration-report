# services/collaboration/src/repository/personal_transfer_storage.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/316-personal-transfer-storage.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`storageReceipt`、`lockStorageSource`、`storageFacts`、`saveStorageReceipt`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { query } from './sql.mjs';

const kind = 'PERSONAL_TRANSFER_STORAGE_MEASUREMENT';
export async function storageReceipt(c, operation) {
  const [rows] = await query(
    c,
    'SELECT payload,state FROM durable_task WHERE kind=? AND business_key=? AND operation_id=? FOR UPDATE',
    [kind, operation, operation],
  );
  return rows[0];
}

export async function lockStorageSource(c, scope) {
  const [rows] = await query(
    c,
    'SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE',
    [scope],
  );
  return rows[0];
}

export async function storageFacts(c, document) {
  // 来源ACK之后仍锁MAIN入口和全部room，和真实保存/捕获/检查点使用同一顺序。
  // 聚合按表批量查询，避免每个room单独往返，也不把正文搬到Doc或内存中再数长度。
  const [rooms] = await query(
    c,
    'SELECT id,epoch,last_durable_seq,checkpoint_id,state FROM room WHERE document_id=? ORDER BY id FOR UPDATE',
    [document],
  );
  const [checkpoints] = await query(
    c,
    `SELECT x.room_id,COUNT(*) AS records,COALESCE(SUM(OCTET_LENGTH(x.yjs_state)),0) AS bytes
    FROM checkpoint x JOIN room r ON r.id=x.room_id WHERE r.document_id=? GROUP BY x.room_id ORDER BY x.room_id`,
    [document],
  );
  const [updates] = await query(
    c,
    `SELECT x.room_id,COUNT(*) AS records,MAX(x.seq) AS sequence,COALESCE(SUM(OCTET_LENGTH(x.payload)),0) AS bytes
    FROM update_log x JOIN room r ON r.id=x.room_id WHERE r.document_id=? GROUP BY x.room_id ORDER BY x.room_id`,
    [document],
  );
  const [captures] = await query(
    c,
    `SELECT x.room_id,COUNT(*) AS records,COALESCE(SUM(COALESCE(OCTET_LENGTH(x.canonical_body),0)+COALESCE(OCTET_LENGTH(CAST(x.candidate_json AS CHAR)),0)),0) AS bytes
    FROM capture_operation x JOIN room r ON r.id=x.room_id WHERE r.document_id=? GROUP BY x.room_id ORDER BY x.room_id`,
    [document],
  );
  return { rooms, checkpoints, updates, captures };
}

export async function saveStorageReceipt(c, request, result) {
  await query(
    c,
    `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
    VALUES(?,?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))`,
    [kind, request.operation_id, request.operation_id, JSON.stringify({ request, result })],
  );
}
````

UTF-8/LF全文SHA-256：`266e463f3133b1b8121b77794d708bce4741f1ae292f5d447a0bc91400c437af`。[参考实现](../../../../services/collaboration/src/repository/personal_transfer_storage.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
