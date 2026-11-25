// 兼容既有测试/服务调用面；规则、SQL及真实排空已经分别归属领域、仓储和应用用例。
export { RPCFailure } from './domain/shared_protocol.mjs';
export { binding, ack } from './domain/fence_protocol.mjs';
export { fenceRooms } from './application/fences.mjs';
export { bootstrapFence } from './application/bootstrap_fence.mjs';
