# services/collaboration/src/fence.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/363-fence.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：完整定义见下方源码。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
// 兼容既有测试/服务调用面；规则、SQL及真实排空已经分别归属领域、仓储和应用用例。
export { RPCFailure } from './domain/shared_protocol.mjs';
export { binding, ack } from './domain/fence_protocol.mjs';
export { fenceRooms } from './application/fences.mjs';
export { bootstrapFence } from './application/bootstrap_fence.mjs';
````

UTF-8/LF全文SHA-256：`c0f63d4dd00fb28919bc3e5f6892f2052aff63653fadd620c45d79e82e950aa6`。[参考实现](../../../../services/collaboration/src/fence.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
