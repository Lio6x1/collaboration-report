# services/collaboration/src/repository/room_snapshots.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/321-room-snapshots.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`RoomSnapshots`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { query } from './sql.mjs';

// 启动失败只给出阶段和稳定码；不能打印包含正文或配置的SQL/驱动原文。
async function snapshotQuery(stage, connection, sql, args = []) {
  try {
    return await query(connection, sql, args);
  } catch (error) {
    if (process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === '1') {
      const code =
        typeof error?.code === 'string' && /^[A-Z][A-Z0-9_]{1,95}$/.test(error.code)
          ? error.code
          : 'SNAPSHOT_READ_UNKNOWN';
      console.error(JSON.stringify({ event: 'room_snapshot_read_failed', stage, code }));
    }
    throw error;
  }
}

export class RoomSnapshots {
  constructor(pool) {
    this.pool = pool;
  }

  listRooms() {
    // 普通退休房间仍检查持久性；仅原清理回执与退休状态同事务提交的房间没有正文。
    return snapshotQuery(
      'ROOM_LIST',
      this.pool,
      `SELECT r.* FROM room r WHERE NOT (r.state='RETIRED' AND EXISTS (
        SELECT 1 FROM durable_task t WHERE t.state='SUCCEEDED' AND (
          (t.kind='ACCOUNT_DOCUMENT_CLEANUP' AND t.business_key=CAST(r.document_id AS CHAR)) OR
          (t.kind='PURGE_DOCUMENT_CLEANUP' AND t.business_key LIKE CONCAT(CAST(r.document_id AS CHAR),':%'))
        )
      )) ORDER BY r.id`,
    );
  }

  latestCheckpoint(connection, room, epoch, cut) {
    // 恢复只使用不越过cut的最新已验证检查点；旧检查点可含多份MiB正文，
    // 全部返回会挤占三秒SQL预算及启动内存，却不参与任何持久性判断。
    return snapshotQuery(
      'ROOM_CHECKPOINT',
      connection,
      'SELECT * FROM checkpoint WHERE room_id=? AND epoch=? AND covered_seq<=? AND validated_at IS NOT NULL ORDER BY covered_seq DESC,id DESC LIMIT 1',
      [room, epoch, cut],
    );
  }

  updatesBetween(connection, room, epoch, after, cut) {
    return snapshotQuery(
      'ROOM_UPDATE_LOG',
      connection,
      'SELECT seq,payload,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND seq>? AND seq<=? ORDER BY seq',
      [room, epoch, after, cut],
    );
  }

  insertCheckpoint(connection, room, epoch, cut, bytes, checksum) {
    return query(
      connection,
      "INSERT INTO checkpoint(room_id,epoch,covered_seq,yjs_state,checksum,format_version,validated_at) VALUES(?,?,?,?,?,'collab-yjs-v1',UTC_TIMESTAMP(6))",
      [room, epoch, cut, bytes, checksum],
    );
  }

  // 同一连接读取BIGINT原值，不能把mysql2浮点insertId当作checkpoint身份。
  lastInsertedID(connection) {
    return query(connection, 'SELECT CAST(LAST_INSERT_ID() AS CHAR) AS id');
  }

  attachCheckpoint(connection, id, room, epoch) {
    return query(connection, 'UPDATE room SET checkpoint_id=? WHERE id=? AND epoch=?', [
      id,
      room,
      epoch,
    ]);
  }
}
````

UTF-8/LF全文SHA-256：`c89f2af66f9c310ead4f4d1d629331b0cb560c05f6a32d6f07d86f46a37ca6cd`。[参考实现](../../../../services/collaboration/src/repository/room_snapshots.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
