# C06｜固定候选清理、账号清理与转移计量

## 这节课要实现什么

文档永久清理或转移空间时，协作服务还保存着日志、检查点和候选数据。本课让这些数据按原计划清理或计量，避免只删文档目录却遗留正文或算错容量。

## 做完后能看到什么

接通后，只有已授权且完成排空的原清理计划能删除对应协作内容；重试得到原清理证明。转移计量绑定固定范围与真实字节，不用估算数字直接结算。

## 先想清楚怎样实现

1. 先验证文档服务的原计划和关闭证据。
2. 在事务中清理正文并留下完成凭据。
3. 另按固定关闭范围统计转移字节，候选清理也核原任务终态。

## 怎样判断这一课完成

C07后检查清理协议，06配合D12、D13执行真实存储清理和转移，分别验证元数据、协作数据与对象存储结果。

## 开始写之前

从上一节已经写入的文件继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `services/collaboration/src/domain/account_document_cleanup.mjs` | 校验Doc原清理authority与plan_digest，PURGE内部派发保持原CLEANED回执形状。 | [逐段带写](../../write/285-account-document-cleanup.md) |
| 2 | `services/collaboration/src/repository/account_document_cleanup.mjs` | 锁原清理任务及房间，核排空事实，删除该文档的协作正文并保存清理证明。 | [逐段带写](../../write/309-account-document-cleanup.md) |
| 3 | `services/collaboration/src/application/account_document_cleanup.mjs` | 先取Doc原authority和关闭事实，确认无活动输出后在同事务清理，绝不按当前权限猜授权。 | [逐段带写](../../write/325-account-document-cleanup.md) |
| 4 | `services/collaboration/src/domain/personal_transfer_storage.mjs` | 绑定doc-acl与publication两份关闭计划，按实际持久字节BigInt求和并固定摘要。 | [逐段带写](../../write/297-personal-transfer-storage.md) |
| 5 | `services/collaboration/src/repository/personal_transfer_storage.mjs` | 锁转移与两份撤权事实，读取原checkpoint/update/capture字节计量。 | [逐段带写](../../write/316-personal-transfer-storage.md) |
| 6 | `services/collaboration/src/application/personal_transfer_storage.mjs` | 只有两类来源都关闭并排空才能固定转移storage_bytes，原号重复返回原测量。 | [逐段带写](../../write/336-personal-transfer-storage.md) |
| 7 | `services/collaboration/src/application/candidate_cleanup.mjs` | 沿原capture/merge authority清理候选资源与正文；保留绑定和终态避免同号复活。 | [逐段带写](../../write/328-candidate-cleanup.md) |

## 2. 清理输入只描述原计划，不自行授予删除权

先写account_document_cleanup领域规则。`cleanupRequest` 只接受operation_id/document_id两个字段，document_id是完整uint64字符串。`cleanupAuthority(input,request)` 核Doc提供的原scope=`doc-acl:<document>`、closed_epoch、revocation_id与plan_digest，摘要由固定用途/操作/文档/来源/代次/撤权号产生。`cleanup_kind='PURGE'` 只在内部派发使用；`cleanupReply` 去掉该字段，保持历史CLEANED回执字节形状。未知字段或另外文档的authority不能拿来清理本地正文。

<!-- lesson-code: services/collaboration/src/domain/account_document_cleanup.mjs :: cleanupAuthority -->
````javascript
export function cleanupAuthority(input, request) {
  const expected = createHash('sha256')
    .update(
      [
        'ACCOUNT_DOCUMENT_CLEANUP',
        request.operation_id,
        request.document_id,
        input?.scope_key,
        input?.closed_epoch,
        input?.revocation_id,
      ].join('\n'),
    )
    .digest('hex');
  if (
    (input?.cleanup_kind !== undefined && input.cleanup_kind !== 'PURGE') ||
    input?.operation_id !== request.operation_id ||
    input?.document_id !== request.document_id ||
    input?.scope_key !== 'doc-acl:' + request.document_id ||
    !canonicalID(input?.closed_epoch) ||
    !validID(input?.revocation_id) ||
    input?.plan_digest !== expected
  )
    throw bad('ACCOUNT_DOCUMENT_CLEANUP_AUTHORITY_INVALID');
  return input;
}
````
<!-- /lesson-code -->

## 3. 先验证排空，正文删除与墓碑同事务

写repository/account_document_cleanup的receipt/source/documentArtifacts/rooms/deliveries查询，再写删除与recordCleanup。应用 `AccountDocumentCleanup.clean(input)` 先读原本地成功回执；若已完成，重复请求即使后来Doc留存代次变化也返回原结果。未完成才在SQL锁外调Doc `/infra/lifecycle/document-close-context` 核原authority。

事务锁序是source→MAIN文档入口→原receipt→barrier/artifacts→room；`requireDrain` 要求source BLOCKED、原max_epoch/revocation吻合、closure SUCCEEDED，并核 `authorization_ack(participant='node-runtime',phase='OUTPUT_STOPPED')` 的原ACK digest。非终态MAIN/草稿/结构安装、UNKNOWN capture、未结算delivery都阻止清理。不能先删正文再等待远端安装或输出停止。

<!-- lesson-code: services/collaboration/src/application/account_document_cleanup.mjs :: AccountDocumentCleanup -->
````javascript
export class AccountDocumentCleanup {
  constructor(transactions, peers) {
    Object.assign(this, { transactions, peers });
  }

  replay(row, request) {
    if (!row) return null;
    if (row.state !== 'SUCCEEDED' || row.operation_id !== request.operation_id)
      throw new RPCFailure(6, 'ACCOUNT_DOCUMENT_CLEANUP_CONFLICT');
    return cleanupReply(cleanupAuthority(json(row.payload), request));
  }

  async clean(input) {
    const started = performance.now();
    const stage = (name) => {
      if (process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === '1')
        console.error(
          JSON.stringify({
            event: 'account_document_cleanup',
            stage: name,
            elapsed_ms: Math.round(performance.now() - started),
          }),
        );
    };
    const request = cleanupRequest(input);
    // 提交回复丢失后先读取原本地回执；后续Doc留存代次变化不改变已完成原清理。
    const prior = await this.transactions.run(async (c) =>
      this.replay(
        await repository.cleanupReceipt(c, request.document_id, request.operation_id),
        request,
      ),
    );
    if (prior) return prior;
    stage('ORIGINAL_RECEIPT_CHECKED');
    const authority = cleanupAuthority(
      await this.peers.http('/infra/lifecycle/document-close-context', request),
      request,
    );
    stage('DOC_AUTHORITY_CONFIRMED');
    return this.transactions.run(async (c) => {
      // 与捕获/UPDATE相同：来源→MAIN入口→房间。系统安装也取得MAIN入口。
      const [[source]] = await repository.lockCleanupSource(c, authority.scope_key);
      await barriers.lockDocument(c, request.document_id);
      const repeated = this.replay(
        await repository.cleanupReceipt(c, request.document_id, request.operation_id),
        request,
      );
      if (repeated) return repeated;
      await this.requireDrain(c, authority, source);
      stage('LOCAL_DRAIN_CONFIRMED');
      const [[barrier]] = await barriers.lockBarrier(c, request.document_id);
      if (barrier && !['INSTALLED', 'ABORTED'].includes(barrier.state))
        throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_INSTALL_PENDING');
      const [fixed] = await repository.documentArtifacts(c, request.document_id);
      stage('ARTIFACTS_READ');
      for (const row of fixed) {
        const value = json(row.payload);
        if (!['SUCCEEDED', 'REJECTED'].includes(row.state))
          throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_ARTIFACT_UNKNOWN');
        if (
          ['DRAFT_MAIN_GATE', 'STRUCTURE_BRANCH_GATE'].includes(row.kind) &&
          !['INSTALLED', 'ABORTED'].includes(value.state)
        )
          throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_INSTALL_PENDING');
        if (row.kind === 'DRAFT_TRANSITION_PREPARED') {
          const installed = await repository.draftInstallation(c, row.business_key);
          if (
            !installed ||
            installed.state !== 'SUCCEEDED' ||
            installed.operation_id !== row.operation_id ||
            json(installed.payload).query?.target?.document_id !== request.document_id
          )
            throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_DRAFT_PENDING');
        }
      }
      const [rooms] = await repository.lockRooms(c, request.document_id);
      stage('ROOMS_LOCKED');
      for (const room of rooms) {
        const [pending] = await repository.pendingDeliveries(c, room.id);
        if (pending.length) throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_OUTPUT_PENDING');
        const [captures] = await repository.roomCaptures(c, room.id);
        if (captures.some((capture) => capture.state === 'UNKNOWN'))
          throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_CAPTURE_UNKNOWN');
      }
      // Doc固定计划证明原写入/安装已终结；CUT_FIXED不再猜测其用户操作结果。
      // 所有正文清除和成功墓碑在同一TX，网络/COMMIT未知只重送原号。
      for (const room of rooms) await repository.clearRoomBody(c, room.id);
      for (const row of fixed) {
        if (row.kind === 'RESOURCE_CONFIRM') continue; // 保留资源确认审计，Doc另行释放保护。
        if (row.kind === 'MERGE_ATTEMPT_CONTEXT')
          await repository.removeMergeResult(c, row.business_key);
        await repository.removeArtifact(c, row.id);
      }
      await repository.clearBarrierBody(c, request.document_id);
      await repository.recordCleanup(c, authority);
      return cleanupReply(authority);
    });
  }

  async requireDrain(connection, authority, source) {
    if (
      !source ||
      source.state !== 'BLOCKED' ||
      String(source.max_epoch) !== authority.closed_epoch ||
      source.last_revocation_id !== authority.revocation_id
    )
      throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_SOURCE_CHANGED');
    const request = {
      revocation_id: authority.revocation_id,
      closed_scopes: [{ scope_key: authority.scope_key, epoch: authority.closed_epoch }],
      generation: '1',
      target_digest: createHash('sha256')
        .update('AUTHORIZATION_CHANGE\n' + authority.scope_key + '\n' + authority.closed_epoch)
        .digest(),
    };
    const closure = await lockedClosure(connection, authority.revocation_id);
    if (!closure || closure.state !== 'SUCCEEDED')
      throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_DRAIN_PENDING');
    assertSameBinding(closure, request.closed_scopes[0], binding(request));
    const [[proof]] = await query(
      connection,
      "SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant='node-runtime' AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'",
      [authority.revocation_id, authority.scope_key, authority.closed_epoch],
    );
    if (!proof || !Buffer.from(proof.result_digest).equals(ack(request).result_digest))
      throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_ACK_CHANGED');
  }
}
````
<!-- /lesson-code -->

具体推演：Doc原cleanup-K关闭doc-acl:10 epoch=7，Node还有一个未settle delivery。clean必须 `ACCOUNT_DOCUMENT_OUTPUT_PENDING`，房间字节全在；实际close和ACK完成后才能同事务清room正文、固定artifact及barrier正文并写成功墓碑。COMMIT回执丢失，仍重送K；不能换K2删除第二遍。RESOURCE_CONFIRM审计保留由Doc另行释放保护。`requireDocumentBodyPresent(connection,document)` 供C04的迟到系统安装核墓碑，已CLEANED则拒绝，旧候选不能复活正文。

## 4. 个人转移计量要固定两种来源关闭与真实字节

再写transfer领域、仓储、应用。`transferStorageRequest(input)` 限定generation='1'，closures恰好包含不同revocation的doc-acl和publication两项、都是当前document；不能漏公开资格。规范后按scope排序。`transferStorageMeasurement(request,facts)` 只从真实checkpoint/update/capture行的bytes求和，使用BigInt并拒绝uint64溢出；返回storage_bytes十进制字符串、snapshot_digest、source_closed=true。

<!-- lesson-code: services/collaboration/src/domain/personal_transfer_storage.mjs :: transferStorageMeasurement -->
````javascript
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
<!-- /lesson-code -->

`PersonalTransferStorage.measure(input)` 先核每个source已BLOCKED、closed原epoch及SUCCEEDED原closure的目标摘要，再锁文档。原operation有成功receipt则严格解析原request并返回saved.result；不以数据库JSON对象键顺序重新字符串化比较业务绑定。新操作在同事务读字节facts并保存结果。它只证明计量，既不发grant也不重新开放来源。

例如checkpoint=100字节、两条update=20和30、capture=40，结果190；大于2^53的bytes仍要精确相加。只关doc-acl而publication仍开放，返回 `PERSONAL_TRANSFER_STORAGE_DRAIN_PENDING`；闭另一文档也不能替代。原operation更换closures是 `PERSONAL_TRANSFER_STORAGE_BINDING_CHANGED`，不是重新测一个余额。

## 5. 候选清理保持可证明的原终态

最后写candidate_cleanup。`cleanupBinding(input)` 绑定operation/generation/candidate_ref/candidate_digest与CLEANABLE permit_digest；ref只允许 `capture:<operation>` 或 `merge:<operation>:<generation>`。调用方传来的permit仍不授予删除权：应用锁外调用Doc `AuthorizeCandidateCleanup` 重核原持久许可，RETAIN/UNKNOWN/超时全部保留副本。

锁内复用capture operation gate或merge attempt gate，再核原body/结果摘要与Go结果引用，删除候选正文与写固定清理回执同事务。CPU合并期间不占SQL锁，所以迟到的合并第二事务必须看到墓碑，不能重新生成候选。返回的fixed_result_json_utf8与result_digest可重复读取并核SHA-256；未知回执保持原号恢复。

## 6. 传输装配与检查

C07私有HTTPS `/infra/lifecycle/document-cleanup` 接AccountDocumentCleanup，`/infra/lifecycle/personal-transfer-storage` 接PersonalTransferStorage，都只接受Doc证书；RPC `CleanupCandidate` 接CandidateCleanup。`createRoomComponents` 显式创建三者，C04的草稿/MAIN安装直接引用 `requireDocumentBodyPresent`，不能漏掉防复活入口。

```powershell
# 当前离线检查冻结回执规则，不操作真实数据。
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 services/collaboration/tests/purge_document_cleanup.test.mjs
if ($LASTEXITCODE -ne 0) { throw '清理authority或冻结回执形状失败' }
node --check services/collaboration/src/application/personal_transfer_storage.mjs
if ($LASTEXITCODE -ne 0) { throw '转移计量语法错误' }
```

精确测试 `PURGE uses original document authority and keeps the frozen CLEANED reply shape` 核PURGE派发不改变旧回执。预期PASS、fail/skip=0；语法检查没有证明真实存储测量。06完整READY、开发SMTP以及本学习独立身份/文档前置成立后，执行真实生命周期组；该组包含退出、恢复和自身到期夹具，不与其他组并行：

```powershell
foreach ($lessonLifecycleFile in @('tests/lifecycle/account-close.test.mjs','tests/lifecycle/personal-transfer.test.mjs','tests/lifecycle/purge.test.mjs')) {
    node --no-maglev --test --test-reporter=tap --test-concurrency=1 $lessonLifecycleFile
    if ($LASTEXITCODE -ne 0) { throw ('原生命周期任务未完成：' + $lessonLifecycleFile) }
}
```

精确测试 `account closure persists and completes original public/private cleanup before deidentification` 核清理先于去标识化；`personal transfer retains ownership until actual acceptance, preserves fixed data and rejects insufficient persistent storage` 核真实Node计量与容量不足；`purge recovers its original task and automatic expiry physically removes only the fixed documents` 核原PURGE恢复与目标范围。预期全部PASS、fail/skip=0。任何DRAIN_PENDING查原source/closure/OUTPUT_STOPPED和delivery；INSTALL_PENDING查原安装恢复；不要清空表或手改SUCCEEDED消除阻塞。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `services/collaboration/src/domain/account_document_cleanup.mjs`：[本课完成文件](../../source/285-account-document-cleanup.md)。
- `services/collaboration/src/repository/account_document_cleanup.mjs`：[本课完成文件](../../source/309-account-document-cleanup.md)。
- `services/collaboration/src/application/account_document_cleanup.mjs`：[本课完成文件](../../source/325-account-document-cleanup.md)。
- `services/collaboration/src/domain/personal_transfer_storage.mjs`：[本课完成文件](../../source/297-personal-transfer-storage.md)。
- `services/collaboration/src/repository/personal_transfer_storage.mjs`：[本课完成文件](../../source/316-personal-transfer-storage.md)。
- `services/collaboration/src/application/personal_transfer_storage.mjs`：[本课完成文件](../../source/336-personal-transfer-storage.md)。
- `services/collaboration/src/application/candidate_cleanup.mjs`：[本课完成文件](../../source/328-candidate-cleanup.md)。

下一节：[ C07 协作完整依赖装配、传输与就绪 ](07-wiring-readiness.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'services/collaboration/src/domain/account_document_cleanup.mjs',
    'services/collaboration/src/repository/account_document_cleanup.mjs',
    'services/collaboration/src/application/account_document_cleanup.mjs',
    'services/collaboration/src/domain/personal_transfer_storage.mjs',
    'services/collaboration/src/repository/personal_transfer_storage.mjs',
    'services/collaboration/src/application/personal_transfer_storage.mjs',
    'services/collaboration/src/application/candidate_cleanup.mjs'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
