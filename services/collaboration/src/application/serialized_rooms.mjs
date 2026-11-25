// 所有读写共用一个房间队列；失败只释放自己的latch，不让后续动作越过尚在提交的动作。
export class SerializedRooms {
  constructor() {
    this.locks = new Map();
    this.pending = 0;
    this.active = 0;
    this.peakPending = 0;
  }

  async run(room, fn) {
    this.pending++;
    this.peakPending = Math.max(this.peakPending, this.pending);
    const prior = this.locks.get(room) ?? Promise.resolve();
    let release;
    const latch = new Promise((r) => {
      release = r;
    });
    const tail = prior.catch(() => {}).then(() => latch);
    this.locks.set(room, tail);
    await prior.catch(() => {});
    this.active++;
    try {
      return await fn();
    } finally {
      this.active--;
      this.pending--;
      release();
      if (this.locks.get(room) === tail) this.locks.delete(room);
    }
  }

  metrics() {
    return {
      pending: this.pending,
      active: this.active,
      waiting: this.pending - this.active,
      peakPending: this.peakPending,
      rooms: this.locks.size,
    };
  }
}
