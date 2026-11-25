# services/collaboration/src/rooms.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/365-rooms.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`RoomEngine`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { RoomEngine as ApplicationEngine } from './application/room_engine.mjs';
import { createRoomComponents } from './bootstrap/room_components.mjs';
export { hash } from './domain/shared_protocol.mjs';
export { query } from './repository/sql.mjs';
export { lockSources } from './application/admission_gates.mjs';

// 兼容既有构造调用；实现已经按用例、领域、仓储、运输和恢复分别归属。
export class RoomEngine extends ApplicationEngine {
  constructor(store, cfg, peers) {
    super(createRoomComponents(store, cfg, peers));
  }
}
````

UTF-8/LF全文SHA-256：`147262173eb71f87d40aefee5941ea7b66ad511a60bac2f6bc3fc0578ef18659`。[参考实现](../../../../services/collaboration/src/rooms.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
