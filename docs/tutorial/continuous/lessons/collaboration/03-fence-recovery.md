# C03｜撤权门闩、真实输出排空与进程恢复

## 这节课要实现什么

一个正在共同编辑的人被撤销权限，他的旧连接不能继续写入或接收正文；服务重启也不能重新放行该连接。本课实现协作侧撤权与恢复。

## 做完后能看到什么

接通后，旧权限先被封住，实际连接随后停止，确认输出排空后才返回关闭结果。进程重启先恢复未完成关闭，再允许服务进入可用状态。

## 先想清楚怎样实现

1. 先记录原撤权范围并提交准入关闭。
2. 停止关联的真实连接与输出。
3. 核对排空后保存确认，重启时继续同一份持久任务。

## 怎样判断这一课完成

本课先检查关闭协议与恢复规则；C07及06在真实多人连接中撤权并重启，验证旧资格不能重新准入。

## 开始写之前

从上一节已经写入的文件继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `services/collaboration/src/domain/fence_protocol.mjs` | 绑定revocation、scope、完整uint64 epoch、generation和target_digest，产生确定ACK摘要。 | [逐段带写](../../write/290-fence-protocol.md) |
| 2 | `services/collaboration/src/repository/fences.mjs` | 保存原撤权目标、关闭来源和资格，读取待恢复计划，终态前检查输出已排空。 | [逐段带写](../../write/312-fences.md) |
| 3 | `services/collaboration/src/application/fences.mjs` | 先提交门闩，再停止实际socket，最后提交撤权终态并返回ACK。 | [逐段带写](../../write/331-fences.md) |
| 4 | `services/collaboration/src/application/bootstrap_fence.mjs` | 保留历史空库启动协议；任意业务表非空均拒绝空ACK，不用于真实排空。 | [逐段带写](../../write/327-bootstrap-fence.md) |
| 5 | `services/collaboration/src/worker/fence_recovery.mjs` | 监听前逐项消费持久原目标，恢复未知撤权而不重算当前权限。 | [逐段带写](../../write/353-fence-recovery.md) |
| 6 | `services/collaboration/src/repository/output_instances.mjs` | 登记运行实例、原delivery退出证据和已退出实例的结算。 | [逐段带写](../../write/314-output-instances.md) |
| 7 | `services/collaboration/src/client/process_probe_linux.mjs` | 读取Linux procfs的NUL分隔参数，区分原PID消失、实际参数存在及不可证明的未知状态。 | [逐段带写](../../write/308-process-probe-linux.md) |
| 8 | `services/collaboration/src/client/process_owner.mjs` | 跨Windows/Linux核验本owner进程确已退出；PID存在或重用不能当作原实例。 | [逐段带写](../../write/307-process-owner.md) |
| 9 | `services/collaboration/src/worker/output_recovery.mjs` | 启动时核原实例归属并结算可证明退出的输出，核房间持久性后才恢复资源任务。 | [逐段带写](../../write/355-output-recovery.md) |
| 10 | `services/collaboration/src/fence.mjs` | 保留现有导入入口，重导出真实领域、仓储之上的应用排空器，不另写第二套实现。 | [逐段带写](../../write/363-fence.md) |

## 2. 模型先固定原撤权计划

先写 `fence_protocol.mjs` 的binding、ack、assertSameBinding。请求真实字段是 `revocation_id/closed_scopes/generation/target_digest`；v1只允许一项closed_scope和generation='1'。scope_key可以是session摘要或doc-acl/org-member/publication等确切来源，epoch使用规范十进制uint64字符串。`publication:9007199254740993`、epoch=`18446744073709551615` 都必须原样保留，不能转Number。

`binding(req)` 校验目标摘要：session使用 `SESSION_LOGOUT\n<session摘要>`，其他来源使用 `AUTHORIZATION_CHANGE\n<scope>\n<epoch>`。成功只返回固定digest/generation；格式/范围/摘要失败为gRPC 3 `BOOTSTRAP_FENCE_INVALID`。同revocation_id换scope、epoch或digest由assertSameBinding返回6 `FENCE_ID_CONFLICT`。`ack(req)` 对原参与者、原epoch和原目标再做确定摘要，不能用新权限状态重算。

<!-- lesson-code: services/collaboration/src/domain/fence_protocol.mjs :: binding -->
````javascript
// 固定目标摘要绑定撤权号、来源和代次；重试必须消费原计划，不能替换成当前可变权限。
export function binding(req) {
  const scope = req.closed_scopes?.[0],
    epoch = scope?.epoch;
  const key = scope?.scope_key ?? '',
    isSession = /^session:[a-f0-9]{64}$/.test(key),
    isAuthorizationSource =
      /^(?:(?:identity|credential|platform-admin|doc-acl|kb-acl|publication):[1-9][0-9]{0,19}|(?:org-member|org-admin|org-owner|org-create):[1-9][0-9]{0,19}:[1-9][0-9]{0,19})$/.test(
        key,
      );
  if (
    !/^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$/.test(req.revocation_id ?? '') ||
    String(req.generation) !== '1' ||
    req.closed_scopes?.length !== 1 ||
    (!isSession && !isAuthorizationSource) ||
    (isAuthorizationSource &&
      key
        .split(':')
        .slice(1)
        .some((x) => BigInt(x) > 18446744073709551615n)) ||
    // protobuf固定以十进制字符串传uint64；不能接纳已被Number舍入的代次或多种绑定拼法。
    (typeof epoch === 'number' && !Number.isSafeInteger(epoch)) ||
    !/^[1-9][0-9]{0,19}$/.test(String(epoch)) ||
    BigInt(epoch) < 1n ||
    BigInt(epoch) > 18446744073709551615n
  )
    throw new RPCFailure(3, 'BOOTSTRAP_FENCE_INVALID');
  const expected = createHash('sha256')
    .update(
      isSession
        ? 'SESSION_LOGOUT\n' + key.slice(8)
        : 'AUTHORIZATION_CHANGE\n' + key + '\n' + String(epoch),
    )
    .digest();
  const actual = Buffer.from(req.target_digest ?? []);
  if (actual.length !== 32 || !timingSafeEqual(actual, expected))
    throw new RPCFailure(3, 'BOOTSTRAP_FENCE_INVALID');
  return { bootstrap_target_digest: actual.toString('hex'), generation: '1' };
}
````
<!-- /lesson-code -->

## 3. 按门闩→真实停止→终态三步写入

先在repository/fences写 `lockedClosure → insertClosure → blockSource → closeGrants → closeReceipts`。锁内读原计划；新请求落持久目标并把source_gate从OPEN收紧，使C02 `authorized` 的新UPDATE/SYNC/输出立即被拒。事务一完成以后，才调用 `engine.stopScope(scope)`。第三步再以同revocation写 `completeClosure`，满足输出排空证明后才ACK。

<!-- lesson-code: services/collaboration/src/application/fences.mjs :: fenceRooms -->
````javascript
// 撤权分成已提交门闩、真实输出排空、终态三个阶段；ACK永远在socket停止之后持久登记。
export async function fenceRooms(engine, req) {
  const current = binding(req),
    scope = req.closed_scopes[0];
  const done = await engine.transaction(async (connection) => {
    const prior = await repository.lockedClosure(connection, req.revocation_id);
    if (prior) {
      assertSameBinding(prior, scope, current);
      if (prior.state === 'SUCCEEDED') return true;
    } else await repository.insertClosure(connection, req.revocation_id, scope, current);
    await repository.blockSource(connection, req.revocation_id, scope);
    await repository.closeGrants(connection, scope);
    await repository.closeReceipts(connection, scope);
    return false;
  });
  if (done) return ack(req);
  await engine.stopScope(scope);
  const result = ack(req);
  await engine.transaction((connection) =>
    repository.completeClosure(connection, req.revocation_id, scope, result.result_digest),
  );
  return result;
}
````
<!-- /lesson-code -->

输入推演：member的doc-acl:10旧epoch=7正在编辑。Doc发送原撤权F后，第一事务关闭epoch=7，第二步终止该来源的实际socket并等待close，第三事务才保存SUCCEEDED和result_digest。如果进程在第二步前退出，库中原门闩仍闭，重启继续F的原目标，不能开放旧epoch。后来新的合法epoch=8是另一份资格，恢复不能拿8替换7来“成功”。

C05将写 `OutputSessions.stopScope` 及实际socket生命周期；它处理b.stopping和真实close Promise。TCP/TLS仍在CLOSING时可能还有字节排出，所以只有CLOSED或close事件证明停止。远端/退出实例delivery不能靠本地Map空就结算，repository检查持久delivery和所属instance的退出事实。

`bootstrapFence` 是现有历史空库边界，先数业务表，任何非空返回 `FULL_OUTPUT_DRAIN_REQUIRED`；C07的RPC接的始终是fenceRooms。保留这份真实代码是为了旧测试/入口兼容，不可把它当简化线上撤权实现。

## 4. 启动恢复为什么先于监听

依次写 output_instances仓储、process_probe_linux、process_owner，再写OutputRecovery。`ProcessOwner` 在Windows调用既有process-owner.ps1读取指定PID命令行，核绝对入口与原 --run-owner；Linux读取/proc/self及指定PID的NUL分隔argv，按参数边界核入口/owner。发现同原入口/owner仍运行就报OLD_OUTPUT_PROCESS_STILL_RUNNING；PID已消失或可证明已属于其他命令可结算原实例，探测权限/格式未知则OUTPUT_EXIT_PROOF_UNAVAILABLE。`OutputRecovery.initialize()` 核旧instance，结算可证明已退出的delivery，恢复并验证持久房间，然后续接资源任务；只有这些事实成立才设置recovered。

`recoverFences(pool,engine)` 从原 `pendingClosures` 读取target_permission，使用保存的scope/epoch/generation/bootstrap_target_digest再次调用fenceRooms。它不重新询问“当前用户还有什么权限”。C07的runtime先await engine.initialize，再await recoverFences，之后才bind RPC和listen HTTPS。

<!-- lesson-code: services/collaboration/src/worker/fence_recovery.mjs :: recoverFences -->
````javascript
// 启动前消费原持久目标；监听开放前完成所有未知撤权，不能根据当前权限重写旧计划。
export async function recoverFences(pool, engine) {
  for (const pending of await pendingClosures(pool)) {
    const p =
      typeof pending.target_permission === 'string'
        ? JSON.parse(pending.target_permission)
        : pending.target_permission;
    await fenceRooms(engine, {
      revocation_id: pending.revocation_id,
      closed_scopes: [{ scope_key: pending.scope_key, epoch: String(pending.epoch) }],
      generation: p.generation,
      target_digest: Buffer.from(p.bootstrap_target_digest, 'hex'),
    });
  }
}
````
<!-- /lesson-code -->

如果撤权ACK提交后回执丢失，原请求重试从SUCCEEDED读取同一ACK；如果commit未知，返回失败并保持原号恢复。`fence.mjs` 只重导出这些规则和用例，不写重复逻辑。`output_recovery.mjs` 依赖本章后续C06清理标记与C07装配，C07以前只运行纯fence规则测试。

## 5. 精确检查与排错

```powershell
# 离线规则：已有C01 shared_protocol，当前不读实际配置、不启动服务。
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 services/collaboration/tests/publication-fence.test.mjs
if ($LASTEXITCODE -ne 0) { throw '撤权原目标规则失败' }
```

精确测试 `publication fence preserves original full uint64 scope and epoch` 核最大epoch与大ID；`publication fence rejects changed original digest, generation and participant scope` 核原目标不能置换；`publication fence rejects ambiguous or overflowing scope and epoch` 核舍入/溢出拒绝。预期3项PASS、fail/skip=0。若第一项失败，检查String/BigInt、正则范围和摘要换行，不能降低为安全整数。

06独立完整环境下执行 `node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/collaboration/wss.test.mjs tests/collaboration/recovery.test.mjs`，精确测试 `a completed real edit revocation has already closed the member browser socket and a fresh higher epoch still opens` 核ACK前真实close，恢复测试核原SAVED内容与原seq。纯协议测试不能证明这些行为。启动报 `OLD_OUTPUT_PROCESS_STILL_RUNNING` 或 `OUTPUT_EXIT_PROOF_UNAVAILABLE` 先查指定PID/原入口/owner及探测权限；`FENCE_COMMIT_UNKNOWN` 保留原revocation重试；`FULL_OUTPUT_DRAIN_REQUIRED` 说明错误调用空库边界，应核RPC接线，不能删除数据让它通过。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `services/collaboration/src/domain/fence_protocol.mjs`：[本课完成文件](../../source/290-fence-protocol.md)。
- `services/collaboration/src/repository/fences.mjs`：[本课完成文件](../../source/312-fences.md)。
- `services/collaboration/src/application/fences.mjs`：[本课完成文件](../../source/331-fences.md)。
- `services/collaboration/src/application/bootstrap_fence.mjs`：[本课完成文件](../../source/327-bootstrap-fence.md)。
- `services/collaboration/src/worker/fence_recovery.mjs`：[本课完成文件](../../source/353-fence-recovery.md)。
- `services/collaboration/src/repository/output_instances.mjs`：[本课完成文件](../../source/314-output-instances.md)。
- `services/collaboration/src/client/process_probe_linux.mjs`：[本课完成文件](../../source/308-process-probe-linux.md)。
- `services/collaboration/src/client/process_owner.mjs`：[本课完成文件](../../source/307-process-owner.md)。
- `services/collaboration/src/worker/output_recovery.mjs`：[本课完成文件](../../source/355-output-recovery.md)。
- `services/collaboration/src/fence.mjs`：[本课完成文件](../../source/363-fence.md)。

下一节：[ C04 固定送审、三方合并与草稿变换 ](04-fixed-workflows.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'services/collaboration/src/domain/fence_protocol.mjs',
    'services/collaboration/src/repository/fences.mjs',
    'services/collaboration/src/application/fences.mjs',
    'services/collaboration/src/application/bootstrap_fence.mjs',
    'services/collaboration/src/worker/fence_recovery.mjs',
    'services/collaboration/src/repository/output_instances.mjs',
    'services/collaboration/src/client/process_probe_linux.mjs',
    'services/collaboration/src/client/process_owner.mjs',
    'services/collaboration/src/worker/output_recovery.mjs',
    'services/collaboration/src/fence.mjs'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
