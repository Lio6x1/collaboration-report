import { createHash } from 'node:crypto';
import {
  transferStorageRequest,
  transferStorageMeasurement,
} from '../domain/personal_transfer_storage.mjs';
import { bad, json } from '../domain/shared_protocol.mjs';
import { lockedClosure } from '../repository/fences.mjs';
import { lockDocument } from '../repository/main_workspace.mjs';
import * as repository from '../repository/personal_transfer_storage.mjs';

export class PersonalTransferStorage {
  constructor(transactions) {
    this.transactions = transactions;
  }

  async measure(input) {
    const request = transferStorageRequest(input);
    return this.transactions.run(async (c) => {
      // 只接受真实终结的两条原关闭事实。mTLS限制调用方为Doc；此接口返回计量，
      // 不发grant、不重新开放来源，也不能用另一个文档的ACK换取存储证明。
      for (const scope of request.closures) {
        const source = await repository.lockStorageSource(c, scope.scope_key);
        const closed = await lockedClosure(c, scope.revocation_id);
        const digest = createHash('sha256')
          .update('AUTHORIZATION_CHANGE\n' + scope.scope_key + '\n' + scope.epoch)
          .digest('hex');
        const binding = closed && json(closed.target_permission);
        if (
          !source ||
          source.state !== 'BLOCKED' ||
          BigInt(source.max_epoch) < BigInt(scope.epoch) ||
          !closed ||
          closed.state !== 'SUCCEEDED' ||
          closed.scope_key !== scope.scope_key ||
          String(closed.epoch) !== scope.epoch ||
          binding?.generation !== '1' ||
          binding?.bootstrap_target_digest !== digest
        )
          throw bad('PERSONAL_TRANSFER_STORAGE_DRAIN_PENDING');
      }
      await lockDocument(c, request.document_id);
      const original = await repository.storageReceipt(c, request.operation_id);
      if (original) {
        const saved = json(original.payload);
        // MySQL JSON 会规范化对象字段顺序；重放比较严格解析后的绑定，
        // 而不是数据库读回对象的序列化顺序，否则真实原回执永远被误判冲突。
        if (
          original.state !== 'SUCCEEDED' ||
          JSON.stringify(transferStorageRequest(saved.request)) !== JSON.stringify(request)
        )
          throw bad('PERSONAL_TRANSFER_STORAGE_BINDING_CHANGED');
        return saved.result;
      }
      const facts = await repository.storageFacts(c, request.document_id);
      const result = transferStorageMeasurement(request, facts);
      await repository.saveStorageReceipt(c, request, result);
      return result;
    });
  }
}
