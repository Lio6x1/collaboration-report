# services/collaboration/src/domain/main_protocol.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/291-main-protocol.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`storedCut`、`prepareBinding`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { canonical } from '../../../../packages/content/index.mjs';
import { validID, bad } from './shared_protocol.mjs';
import { hash } from './shared_protocol.mjs';

const positive = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;
export const cutDigest = (rooms) => hash(Buffer.from(canonical(rooms))).toString('base64');
export function storedCut(cut) {
  return {
    document_id: cut.document_id,
    structure_id: cut.structure_id,
    style_id: cut.style_id,
    control_revision: cut.control_revision,
    rooms: [...cut.rooms].sort((a, b) =>
      BigInt(a.room_id) < BigInt(b.room_id) ? -1 : BigInt(a.room_id) > BigInt(b.room_id) ? 1 : 0,
    ),
    dependency_digest: Buffer.from(cut.dependency_digest ?? []).toString('base64'),
  };
}
export const wireCut = (cut) => ({
  ...cut,
  dependency_digest: Buffer.from(cut.dependency_digest, 'base64'),
});

export function prepareBinding(input) {
  const context = input.context,
    target = context?.target,
    cut = input.expected_cut;
  if (
    !validID(context?.operation_id) ||
    !positive(context?.actor_id) ||
    !positive(input.barrier_generation) ||
    !target ||
    !positive(target.document_id) ||
    !positive(target.structure_id) ||
    typeof target.purpose !== 'string' ||
    !/^[A-Z_]{1,32}$/.test(target.purpose) ||
    !context.document_admission_ref ||
    !context.identity_admission_ref ||
    Buffer.from(context.request_fingerprint ?? []).length !== 32 ||
    !Array.isArray(context.sources) ||
    !context.sources.length ||
    !cut ||
    cut.document_id !== target.document_id ||
    cut.structure_id !== target.structure_id ||
    !positive(cut.style_id) ||
    !positive(cut.control_revision) ||
    !Array.isArray(cut.rooms) ||
    cut.rooms.length > 10000 ||
    Buffer.from(cut.dependency_digest ?? []).length !== 32
  )
    throw bad('MAIN_PREPARE_INVALID');
  for (const room of cut.rooms)
    if (
      !positive(room.room_id) ||
      !positive(room.workspace_epoch) ||
      !(room.durable_seq === '0' || positive(room.durable_seq))
    )
      throw bad('MAIN_CUT_INVALID');
  if (new Set(cut.rooms.map((room) => room.room_id)).size !== cut.rooms.length)
    throw bad('MAIN_CUT_INVALID');
  return {
    context: {
      ...context,
      request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
    },
    barrier_generation: input.barrier_generation,
    expected_cut: storedCut(cut),
  };
}
````

UTF-8/LF全文SHA-256：`846c55af1eadbe7e60fea95ea581b9dea1af40b80e003f92331d7d7cf313d56a`。[参考实现](../../../../services/collaboration/src/domain/main_protocol.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
