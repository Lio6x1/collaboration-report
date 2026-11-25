# C01｜正文契约、隔离候选与连续日志恢复

## 这节课要实现什么

多人编辑后关闭页面，再次进入应能恢复已保存的正文；非法更新不能污染大家正在编辑的内容。本课实现协作服务的正文转换、更新候选检查和日志恢复。

## 做完后能看到什么

本课可离线验证：合法修改得到新的正文，非法更新不会装入原文档；恢复必须按完整日志顺序重放，缺一条或摘要不符就拒绝继续。此时还没有联网编辑房间。

## 先想清楚怎样实现

1. 先把编辑器结构转换为受限正文。
2. 在独立候选上应用并检查更新。
3. 再从检查点和连续日志恢复指定保存位置，同一房间的操作按序执行。

## 怎样判断这一课完成

本课末可运行正文、隔离候选和Markdown导入测试；真实数据库恢复及多人连接在C07写齐后进入06验证。

## 开始写之前

从[03 文档服务](../../03-document.md)继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `packages/content/index.mjs` | 定义ProseMirror schema、语义树转换、规范化和隔离Yjs候选；非法节点不能污染host。 | [逐段带写](../../write/360-index.md) |
| 2 | `packages/content/markdown.mjs` | 把Markdown转为受限正文，生成新块ID；外部图片只有READY资源映射才成为图片节点。 | [逐段带写](../../write/361-markdown.md) |
| 3 | `services/collaboration/src/domain/shared_protocol.mjs` | 统一SHA-256、规范业务ID和有界RPCFailure；将内部异常限制为稳定业务码。 | [逐段带写](../../write/301-shared-protocol.md) |
| 4 | `services/collaboration/src/domain/room_replay.mjs` | 校验检查点并逐条核连续seq与payload_hash，重放到调用方指定cut。 | [逐段带写](../../write/300-room-replay.md) |
| 5 | `services/collaboration/src/repository/sql.mjs` | 提供三秒SQL预算及同一连接的Transactions.run；仓储不得自行提交。 | [逐段带写](../../write/323-sql.md) |
| 6 | `services/collaboration/src/repository/room_snapshots.mjs` | 读取不越过cut的最新有效checkpoint及后续日志，保存新检查点并读取完整BIGINT身份。 | [逐段带写](../../write/321-room-snapshots.md) |
| 7 | `services/collaboration/src/application/room_snapshots.mjs` | 组合恢复、正文检查和检查点写入；启动核实每个持久房间而非只测试连通。 | [逐段带写](../../write/341-room-snapshots.md) |
| 8 | `services/collaboration/src/application/serialized_rooms.mjs` | 为同room排一条Promise链，使UPDATE、FLUSH和capture共享顺序。 | [逐段带写](../../write/343-serialized-rooms.md) |

`packages/content/validate.mjs` 是支持包内的生成物，不创建空文件、不手抄其几千行校验分支。它由 `scripts/generate-content.mjs` 将冻结的 `contracts/content/content.schema.json` 与受限的段落格式扩展交给锁定 AJV 生成；运行时直接导入已生成函数，浏览器无需动态编译字符串。输入契约改动后重新生成并检查，不能直接编辑 validate.mjs 放宽规则：

```powershell
# 工作目录：自己的 lessonRoot；00 已安装锁定依赖。
node scripts/generate-content.mjs --check
if ($LASTEXITCODE -ne 0) { throw '生成校验器与契约不一致；核对材料，不手改生成物' }
```

预期退出 0。开发有意修改允许扩展的契约时，先运行 `node scripts/generate-content.mjs`，再 `--check` 与本课正文测试；冻结契约不能随意变更。

## 2. 先写正文模型，再碰Yjs状态

先在 `packages/content/index.mjs` 写节点、mark与 `schema` 的声明，再依次写 `atomFields → toPM → fromPM → canonical → normalizeContent → resourceIds → inspectDocument → prepareCandidate`。schema 是ProseMirror的结构规则；`validate.mjs` 处理content-v1 JSON字段约束，两道检查都要保留。`markdown.mjs` 最后写，调用这些已定义的模型。

输入正文可以是 `{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p-001"},"content":[{"type":"text","text":"共同编辑"}]}]}`。`id` 是稳定块身份，不是数组下标；删除、移动和后面的三方合并依赖它。文本不带块ID，图片 `resourceId` 必须是规范十进制字符串，能保存 `9007199254740993`。`workspaceKind='BRANCH'` 禁止字体mark；`MAIN` 可保存主线字体。不能把分支字体静默删掉后当合法输入。

`normalizeContent(value, {workspaceKind})` 返回独立的受限语义树：默认属性补齐、块ID唯一、表格几何合法、深度不超过64、节点不超过100000，完整规范字节不超过16MiB；失败抛 `code='INVALID_CONTENT'`。它不会trim正文空格。`canonical(value)` 按对象键排序、保留数组和文本、固定Unicode分隔符转义；Go和Node只有得到相同字节才可能得到相同SHA-256。

<!-- lesson-code: packages/content/index.mjs :: normalizeContent -->
````javascript
export function normalizeContent(value, { workspaceKind = 'BRANCH' } = {}) {
  try {
    inspectValue(value);
    const body = structuredClone(value);
    let count = 0;
    const ids = new Set();
    function inspect(n, depth = 0) {
      if (
        depth > 64 ||
        ++count > 100000 ||
        !n ||
        typeof n !== 'object' ||
        typeof n.type !== 'string'
      )
        throw invalid();
      if (workspaceKind === 'BRANCH' && (n.marks ?? []).some((m) => m.type === 'font'))
        throw invalid();
      if (atoms.has(n.type))
        n.attrs = {
          ...Object.fromEntries(Object.entries(atomAttributes).map(([k, v]) => [k, v.default])),
          ...n.attrs,
        };
      if (n.type === 'code_block') n.attrs = { language: null, ...n.attrs };
      if (n.type === 'table_cell' || n.type === 'table_header')
        n.attrs = { colwidth: null, ...n.attrs };
      if (n.type === 'image') {
        n.attrs = { alt: null, title: null, width: null, height: null, ...n.attrs };
        if (
          !/^[1-9][0-9]{0,19}$/.test(n.attrs.resourceId) ||
          BigInt(n.attrs.resourceId) > 18446744073709551615n
        )
          throw invalid();
      }
      const spec = schema.nodes[n.type];
      if (!spec) throw invalid();
      if (n.type !== 'doc' && n.type !== 'text' && !spec.isInline) {
        const id = n.attrs?.id;
        if (typeof id !== 'string' || ids.has(id)) throw invalid();
        ids.add(id);
      }
      for (const c of n.content ?? []) inspect(c, depth + 1);
    }
    inspect(body);
    if (!validateSchema(body)) throw invalid();
    const parsed = schema.nodeFromJSON(toPM(body));
    parsed.check();
    parsed.descendants((n) => {
      if (n.type.name === 'table' && TableMap.get(n).problems?.length) throw invalid();
    });
    if (new TextEncoder().encode(canonical(body)).length > 16 * 1024 * 1024) throw invalid();
    return body;
  } catch {
    throw invalid();
  }
}
````
<!-- /lesson-code -->

写完该函数，再写 `resourceIds`。一篇正文重复引用资源 `20` 两次，只计一个资源ID；输出按BigInt数值排序，所以 `2` 在 `10` 前面。Markdown导入的 `newId` 用来给新块发新身份，`resources` 映射把外部URL对应到已READY的内部资源；没有映射就保留显式待处理文本与 `externalImages`，原始HTML不能执行。`importMarkdown` 返回 `body/losses/externalImages`，不是把外链直接变成可读取的存储资格。

## 3. UPDATE先在隔离文档验证

把 `inspectDocument` 写在 `prepareCandidate` 前面。其参数 `doc` 是调用方拥有的Y.Doc；返回 `body/resources/canonical`，不销毁、不修改该doc。若Yjs还有 `pendingStructs` 或 `pendingDs`，返回 `DEPENDENCIES_MISSING`，这表示原更新缺少前置依赖，不能伪造为空正文继续保存。

`prepareCandidate(host, update, options)` 新建一份doc，复制host，再应用update。成功返回拥有生命周期的 `doc` 和正文事实；调用方必须在finally中destroy这份候选。未知节点、重复 `p-001`、非法表格或BRANCH font全部拒绝，host仍保持输入前字节。请特别推演纯删除：Yjs删除集可以改变正文却不改变state vector，所以保存不能只比较state vector，C02使用更新身份与hash证明删除已经落库。

<!-- lesson-code: packages/content/index.mjs :: prepareCandidate -->
````javascript
export function prepareCandidate(host, update, options) {
  // 推测候选独立于公共房间；校验、资源保护、事务成功之前不能向host applyUpdate。
  const doc = new Y.Doc();
  try {
    Y.applyUpdate(doc, Y.encodeStateAsUpdate(host));
    Y.applyUpdate(doc, update);
    return { doc, ...inspectDocument(doc, options) };
  } catch (err) {
    doc.destroy();
    if (err.code === 'DEPENDENCIES_MISSING') throw err;
    throw invalid();
  }
}
````
<!-- /lesson-code -->

## 4. 从检查点恢复精确cut，数据库不替业务提交

现在依次写 `shared_protocol.mjs`、`sql.mjs`，再写 `room_replay.mjs`，最后两份 `room_snapshots.mjs` 和队列。`RPCFailure(code,details)` 的details必须是稳定有界码，避免将SQL、DSN或网络原文跨到客户端。

`Transactions.run(body)` 向回调交同一 `connection`，START→body→COMMIT；finally尝试ROLLBACK并释放。`body` 的返回值只有COMMIT返回后才交给调用方。COMMIT抛错可能是提交未知，不能推断未落库，也不能发送SAVED。仓储函数接受这份connection而不是另取pool，从而把日志、房间水位和确认任务放进一个事务。

<!-- lesson-code: services/collaboration/src/repository/sql.mjs :: Transactions -->
````javascript
export class Transactions {
  constructor(pool) {
    this.pool = pool;
  }

  async run(body) {
    const connection = await this.pool.getConnection();
    try {
      await query(connection, 'START TRANSACTION');
      const result = await body(connection);
      await query(connection, 'COMMIT');
      return result;
    } finally {
      try {
        await query(connection, 'ROLLBACK');
      } catch {}
      connection.release();
    }
  }
}
````
<!-- /lesson-code -->

先写仓储的 `latestCheckpoint(connection,room,epoch,cut)`：只取 `covered_seq<=cut` 的最新已验证行，再写 `updatesBetween(...,after,cut)` 按seq升序读尾日志。`openCheckpoint` 必须先核format/checksum再apply，不能损坏就从空Y.Doc开始。`applyDurableLog(doc,covered,updates,room,cut)` 每条核hash与连续seq，最终必须等于cut，然后执行与在线UPDATE一致的正文检查。

<!-- lesson-code: services/collaboration/src/domain/room_replay.mjs :: applyDurableLog -->
````javascript
// 纯规则只应用指定cut的连续持久日志；Y.Doc由调用用例负责归还或释放。
export function applyDurableLog(doc, covered, updates, room, cut) {
  let sequence = BigInt(covered);
  for (const update of updates) {
    if (
      BigInt(update.seq) !== ++sequence ||
      !timingSafeEqual(hash(update.payload), update.payload_hash)
    )
      throw Error('DURABLE_LOG_INVALID');
    Y.applyUpdate(doc, update.payload);
  }
  if (sequence !== BigInt(cut)) throw Error('DURABLE_LOG_GAP');
  // 恢复文档本就独立于在线房间；完整检查沿共享规则直接只读执行，避免再次克隆正文。
  inspectDocument(doc, {
    workspaceKind: room.workspace_kind,
  });
  return doc;
}
````
<!-- /lesson-code -->

具体输入：checkpoint覆盖32，room.last_durable_seq为34，日志必须为33和34。只有34会在首条触发 `DURABLE_LOG_INVALID`；只有33会最终触发 `DURABLE_LOG_GAP`；两条齐全但34的hash错也必须失败。如果要固定送审cut=33，只重放到33，后来的34不能进入这份固定内容。`RoomSnapshots.restore` 成功返回doc，调用方负责destroy；失败自己销毁临时doc。`checkpoint` 先写字节/checksum，再用同连接 `CAST(LAST_INSERT_ID() AS CHAR)` 取完整ID，不使用可能精度丢失的mysql2数字insertId。

`SerializedRooms.run(room,fn)` 对同room等待原Promise并在finally释放latch；上次fn失败不会永久堵住后续请求。不同room仍可并行。它是单进程顺序，真实跨进程事实仍由SQL锁保护。后续UPDATE、FLUSH、送审必须共用同一个实例，不能每个用例new一份队列。

## 5. 接线位置与本课检查

本课尚未监听网络。C02的 `RoomAdmissions` 用快照生成READY，`RoomUpdates` 用它恢复host；C07的 `createRoomComponents` 只创建一份queue/transactions/snapshots。此时只运行正文模块的离线测试，不运行依赖还没写齐的RoomEngine。

```powershell
# 工作目录：自己的lessonRoot；00已安装锁定Node依赖，无Docker要求。
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 services/collaboration/tests/content.test.mjs services/collaboration/tests/inspect-document.test.mjs services/collaboration/tests/markdown.test.mjs
if ($LASTEXITCODE -ne 0) { throw '正文候选或导入规则失败' }
```

精确断言包括 `a pure deletion update changes accepted content despite an unchanged state vector`、`共享校验后的prepareCandidate仍隔离真实非法更新，不安装到host`、`external images stay explicit pending text until a READY resource mapping exists and raw HTML never executes`。预期TAP fail=0、skip=0且进程退出0。出现 `INVALID_CONTENT` 先核块ID、默认原子属性、MAIN/BRANCH和表格几何；`ERR_MODULE_NOT_FOUND` 先核00的workspace依赖和表中三个packages/content路径。离线检查还没有证明MySQL持久性；在06跑 `tests/collaboration/recovery.test.mjs` 的 `actual owned process restart restores durable checkpoints and logs and replays a lost acknowledgement as the original sequence` 才核重启后的原日志。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `packages/content/index.mjs`：[本课完成文件](../../source/360-index.md)。
- `packages/content/markdown.mjs`：[本课完成文件](../../source/361-markdown.md)。
- `services/collaboration/src/domain/shared_protocol.mjs`：[本课完成文件](../../source/301-shared-protocol.md)。
- `services/collaboration/src/domain/room_replay.mjs`：[本课完成文件](../../source/300-room-replay.md)。
- `services/collaboration/src/repository/sql.mjs`：[本课完成文件](../../source/323-sql.md)。
- `services/collaboration/src/repository/room_snapshots.mjs`：[本课完成文件](../../source/321-room-snapshots.md)。
- `services/collaboration/src/application/room_snapshots.mjs`：[本课完成文件](../../source/341-room-snapshots.md)。
- `services/collaboration/src/application/serialized_rooms.mjs`：[本课完成文件](../../source/343-serialized-rooms.md)。

下一节：[ C02 真实准入、UPDATE持久确认与FLUSH ](02-admission-save.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'packages/content/index.mjs',
    'packages/content/markdown.mjs',
    'services/collaboration/src/domain/shared_protocol.mjs',
    'services/collaboration/src/domain/room_replay.mjs',
    'services/collaboration/src/repository/sql.mjs',
    'services/collaboration/src/repository/room_snapshots.mjs',
    'services/collaboration/src/application/room_snapshots.mjs',
    'services/collaboration/src/application/serialized_rooms.mjs'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
