import { openCheckpoint, applyDurableLog, encodeCheckpoint } from '../domain/room_replay.mjs';

export class RoomSnapshots {
  constructor(repository, store) {
    this.repository = repository;
    this.store = store;
  }

  async verifyRooms() {
    const [rooms] = await this.repository.listRooms();
    let cursor = 0,
      failed = false,
      failure;
    const recover = async () => {
      while (!failed && cursor < rooms.length) {
        const room = rooms[cursor++];
        try {
          const doc = await this.restore(this.store.pool, room);
          doc.destroy();
        } catch (error) {
          if (!failed) failure = error;
          failed = true;
        }
      }
    };
    // 旧 owner 已经核验退出，监听尚未开放；不同房间的只读恢复可重叠 I/O。
    // 固定四个I/O通道消化大量保留房间，并限制同时载入的正文数量；
    // 保留每房间 checkpoint→连续日志→内容校验顺序，不把全部房间一起载入内存。
    // 任一路失败后不启动新房间，并等待已开始的恢复完成/释放后才返回失败；绝不提前 READY。
    await Promise.all(Array.from({ length: 4 }, () => recover()));
    if (failed) throw failure;
    this.store.roomsRecovered = true;
  }

  async restore(connection, room, cut = room.last_durable_seq) {
    const [checkpoints] = await this.repository.latestCheckpoint(
      connection,
      room.id,
      room.epoch,
      cut,
    );
    const checkpoint = checkpoints[0],
      doc = openCheckpoint(checkpoint);
    try {
      // 保持原顺序：损坏checkpoint/Yjs先失败，然后才读取固定范围内的实际日志。
      // 检查点已覆盖精确cut时区间为空，省去每个已保存房间的额外往返。
      // Yjs/checksum已验证，正文规则仍在applyDurableLog检查；未覆盖的尾部照常读连续日志。
      const updates =
        BigInt(checkpoint.covered_seq) === BigInt(cut)
          ? []
          : (
              await this.repository.updatesBetween(
                connection,
                room.id,
                room.epoch,
                checkpoint.covered_seq,
                cut,
              )
            )[0];
      return applyDurableLog(doc, checkpoint.covered_seq, updates, room, cut);
    } catch (error) {
      doc.destroy();
      throw error;
    }
  }

  async checkpoint(connection, room, doc, cut) {
    const { bytes, checksum } = encodeCheckpoint(doc);
    const [result] = await this.repository.insertCheckpoint(
      connection,
      room.id,
      room.epoch,
      cut,
      bytes,
      checksum,
    );
    const [[id]] = await this.repository.lastInsertedID(connection);
    await this.repository.attachCheckpoint(connection, id.id, room.id, room.epoch);
    return String(result.affectedRows);
  }
}
