# services/collaboration/src/application/serialized_rooms.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/343-serialized-rooms.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`SerializedRooms`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`4613c693c74a722e107942378cdaa8b11b822dcac0eb3f1c1e457d22478e8235`。[参考实现](../../../../services/collaboration/src/application/serialized_rooms.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
