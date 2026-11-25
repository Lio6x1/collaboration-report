# services/collaboration/src/domain/personal_transfer_storage.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/297-personal-transfer-storage.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`transferStorageRequest`、`transferStorageMeasurement`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { createHash } from 'node:crypto';
import { bad, validID } from './shared_protocol.mjs';

const uint64 = (value, zero = false) =>
  typeof value === 'string' &&
  (zero ? /^(0|[1-9][0-9]{0,19})$/ : /^[1-9][0-9]{0,19}$/).test(value) &&
  BigInt(value) <= 18446744073709551615n;

export function transferStorageRequest(input) {
  if (
    !input ||
    !validID(input.operation_id) ||
    !uint64(input.document_id) ||
    input.generation !== '1' ||
    !Array.isArray(input.closures) ||
    input.closures.length !== 2 ||
    Object.keys(input).some(
      (key) => !['operation_id', 'document_id', 'generation', 'closures'].includes(key),
    )
  )
    throw bad('PERSONAL_TRANSFER_STORAGE_INVALID');
  const closures = input.closures
    .map((s) => {
      if (
        !s ||
        !validID(s.revocation_id) ||
        !uint64(s.epoch) ||
        !['doc-acl:' + input.document_id, 'publication:' + input.document_id].includes(
          s.scope_key,
        ) ||
        Object.keys(s).some((key) => !['revocation_id', 'scope_key', 'epoch'].includes(key))
      )
        throw bad('PERSONAL_TRANSFER_STORAGE_INVALID');
      return { revocation_id: s.revocation_id, scope_key: s.scope_key, epoch: s.epoch };
    })
    .sort((a, b) => a.scope_key.localeCompare(b.scope_key));
  if (
    closures[0].scope_key === closures[1].scope_key ||
    closures[0].revocation_id === closures[1].revocation_id
  )
    throw bad('PERSONAL_TRANSFER_STORAGE_INVALID');
  return {
    operation_id: input.operation_id,
    document_id: input.document_id,
    generation: '1',
    closures,
  };
}

export function transferStorageMeasurement(request, facts) {
  let bytes = 0n;
  // 只累加真实持久行的字节；编号/容量均保持uint64字符串，不把SUM或room ID转Number。
  for (const group of [facts.checkpoints, facts.updates, facts.captures])
    for (const row of group) {
      if (!uint64(String(row.bytes), true)) throw bad('PERSONAL_TRANSFER_STORAGE_OVERFLOW');
      bytes += BigInt(row.bytes);
    }
  if (bytes > 18446744073709551615n) throw bad('PERSONAL_TRANSFER_STORAGE_OVERFLOW');
  return {
    operation_id: request.operation_id,
    document_id: request.document_id,
    generation: '1',
    storage_bytes: bytes.toString(),
    snapshot_digest: createHash('sha256').update(JSON.stringify({ request, facts })).digest('hex'),
    source_closed: true,
  };
}
````

UTF-8/LF全文SHA-256：`1f92e8e1b5052b7c5c2d3bc04ae1de449d19d3689fe767b0001b57b11560007a`。[参考实现](../../../../services/collaboration/src/domain/personal_transfer_storage.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
