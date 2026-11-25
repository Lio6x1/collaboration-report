# services/collaboration/src/repository/structure_workspace.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/324-structure-workspace.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：完整定义见下方源码。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { query } from './sql.mjs';

// 来源 structure_preparations.mjs：原业务声明与函数体保留。

export const branchGate = (connection, room, epoch) =>
  query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='STRUCTURE_BRANCH_GATE' AND business_key=? FOR UPDATE",
    [room + ':' + epoch],
  );
export const prepareBranchGate = (connection, branch, binding) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('STRUCTURE_BRANCH_GATE',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE operation_id=VALUES(operation_id),payload=VALUES(payload)",
    [
      branch.room_id + ':' + branch.workspace_epoch,
      binding.context.operation_id,
      JSON.stringify({
        document_id: binding.context.target.document_id,
        operation_id: binding.context.operation_id,
        generation: binding.barrier_generation,
        state: 'PREPARED',
      }),
    ],
  );
export const finishBranchGate = (connection, branch, binding, state) =>
  query(
    connection,
    "UPDATE durable_task SET payload=JSON_SET(payload,'$.state',?) WHERE kind='STRUCTURE_BRANCH_GATE' AND business_key=? AND operation_id=? AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.generation'))=? AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.state'))='PREPARED'",
    [
      state,
      branch.room_id + ':' + branch.workspace_epoch,
      binding.context.operation_id,
      binding.barrier_generation,
    ],
  );

// 来源 structure_previews.mjs：原业务声明与函数体保留。

export const lockBranchRooms = (connection, document) =>
  query(
    connection,
    "SELECT * FROM room WHERE document_id=? AND workspace_kind='BRANCH' AND state<>'RETIRED' ORDER BY id FOR UPDATE",
    [document],
  );
````

UTF-8/LF全文SHA-256：`761b631ecefee031ceb3910ab0b8fa36a23ea471c1abe13dd0496b89f956c6b9`。[参考实现](../../../../services/collaboration/src/repository/structure_workspace.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
