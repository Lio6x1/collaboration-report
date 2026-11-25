# services/collaboration/src/application/admission_gates.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/326-admission-gates.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`requireMainOpen`、`requireBranchOpen`、`requireDraftMainOpen`、`lockSources`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { bad } from '../domain/shared_protocol.mjs';
import { branchGate } from '../repository/structure_workspace.mjs';
import { deny } from '../domain/shared_protocol.mjs';
import * as drafts from '../repository/draft_transitions.mjs';
import { json } from '../domain/shared_protocol.mjs';
import * as repository from '../repository/main_workspace.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import * as sourceGates from '../repository/source_gates.mjs';

// 来源 main_gate.mjs：原业务声明与函数体保留。

export async function requireMainOpen(connection, document) {
  // 先锁文档级入口，房间尚未存在也不能绕过屏障。调用方仍在原来源/日志事务内。
  await repository.lockDocument(connection, document);
  await requireDraftMainOpen(connection, document);
  const [[barrier]] = await repository.lockBarrier(connection, document);
  if (barrier && !['INSTALLED', 'ABORTED'].includes(barrier.state))
    throw new RPCFailure(14, 'MAIN_BARRIER_PENDING');
}

export async function requireBranchOpen(connection, document, room, epoch) {
  // 与结构prepare共用来源→文档→支线入口→房间顺序；尚无room行的晚到票据也不能绕过准备。
  await repository.lockDocument(connection, document);
  const [[gate]] = await branchGate(connection, room, epoch);
  if (!gate) return;
  const fixed = json(gate.payload);
  if (fixed.document_id !== document || !['PREPARED', 'INSTALLED', 'ABORTED'].includes(fixed.state))
    throw new RPCFailure(14, 'STRUCTURE_BRANCH_GATE_UNKNOWN');
  if (fixed.state === 'PREPARED') throw new RPCFailure(14, 'STRUCTURE_BRANCH_GATE_PENDING');
  if (fixed.state === 'INSTALLED') throw new RPCFailure(9, 'WORKSPACE_CHANGED');
}

// 来源 draft_main_gate.mjs：原业务声明与函数体保留。

// 调用方已经取得统一Doc互斥。此持久入口覆盖全篇MAIN，既有和晚到房间都不能越过。
export async function requireDraftMainOpen(connection, document) {
  const [[row]] = await drafts.artifact(connection, 'DRAFT_MAIN_GATE', document);
  if (!row) return;
  const gate = json(row.payload);
  if (gate.document_id !== document || !['PREPARED', 'INSTALLED', 'ABORTED'].includes(gate.state))
    throw new RPCFailure(14, 'DRAFT_MAIN_GATE_UNKNOWN');
  if (gate.state === 'PREPARED') throw new RPCFailure(14, 'DRAFT_MAIN_GATE_PENDING');
}

// 来源 source_access.mjs：原业务声明与函数体保留。

// 来源统一按ASCII顺序取得排他锁，与关闭事务共享线性化点；只有更高已核验代次可重开。
export async function lockSources(c, sources) {
  if (!Array.isArray(sources) || !sources.length || sources.length > 512)
    throw deny('SOURCES_REQUIRED');
  const list = [...sources].sort((a, b) =>
    Buffer.compare(Buffer.from(a.scope_key), Buffer.from(b.scope_key)),
  );
  if (new Set(list.map((x) => x.scope_key)).size !== list.length) throw bad('SOURCE_DUPLICATE');
  await sourceGates.ensureSourceGates(c, list);
  const [rows] = await sourceGates.lockSourceGates(c, list);
  const gates = new Map(rows.map((gate) => [gate.scope_key, gate]));
  for (const s of list) {
    const gate = gates.get(s.scope_key);
    if (!gate) throw deny('SOURCE_CLOSED');
    if (
      BigInt(gate.max_epoch) > BigInt(s.epoch) ||
      (String(gate.max_epoch) === s.epoch && gate.state === 'BLOCKED')
    )
      throw deny('SOURCE_CLOSED');
    if (BigInt(gate.max_epoch) < BigInt(s.epoch))
      await sourceGates.reopenSourceGate(c, s.epoch, s.scope_key);
  }
}
````

UTF-8/LF全文SHA-256：`1a5610c10ce6824483e037a72e797ec9097f3ba4b882cf2a0420984f05a9da81`。[参考实现](../../../../services/collaboration/src/application/admission_gates.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
