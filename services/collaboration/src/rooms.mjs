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
