# 分步编写 `services/collaboration/src/application/room_engine.mjs`

先按 [C07 · 协作完整依赖装配、传输与就绪](../lessons/collaboration/07-wiring-readiness.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/339-room-engine.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

给传输层提供统一用例调用面，并衔接UPDATE与结构保存握手。

WSS/RPC无需知道每个用例内部依赖，但共享队列、授权和输出对象不能拆成多份状态。

写之前先按这条执行路线理解各段的连接：

1. 多数方法仅委派对应用例
2. update先assertStructureWrite，再清该delivery.lastFlush，防止新增编辑仍借旧回执READY
3. flush完成后recordFlush保存当前连接真实回执
4. initialize/recovered委派OutputRecovery，stopScope委派真实OutputSessions

本文件需要守住的边界：

- 不能在这里再做一份SQL/正文规则造成双实现
- 旧FLUSH在UPDATE后失效，漏清lastFlush会错误确认未保存编辑

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/room_engine.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/room_engine.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/room_engine.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/room_engine.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：RoomEngine

插入位置：空文件第 1 行。

RoomEngine保留服务调用面，协调已分工的用例；这里不解释正文、执行SQL或直接发送socket。

本步声明：`export class RoomEngine `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.store = store</code> |
| <code>cfg</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.cfg = cfg</code> |
| <code>peers</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |
| <code>instance</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.instance = instance</code> |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>queue</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.queue = queue</code> |
| <code>snapshots</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>admissions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.admissions = admissions</code> |
| <code>updates</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.updates = updates</code> |
| <code>reading</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.reading = reading</code> |
| <code>awarenessUpdates</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.awarenessUpdates = awareness</code> |
| <code>captures</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.captures = captures</code> |
| <code>mergeCandidates</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.mergeCandidates = mergeCandidates</code> |
| <code>mainCommits</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.mainCommits = mainCommits</code> |
| <code>mainCaptures</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.mainCaptures = mainCaptures</code> |
| <code>mainInstallations</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.mainInstallations = mainInstallations</code> |
| <code>draftPreviews</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.draftPreviews = draftPreviews</code> |
| <code>structurePreviews</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.structurePreviews = structurePreviews</code> |
| <code>draftTransitions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.draftTransitions = draftTransitions</code> |
| <code>deliveryOutputs</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.deliveryOutputs = outputs</code> |
| <code>confirmations</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.confirmations = confirmations</code> |
| <code>recovery</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.recovery = recovery</code> |
| <code>accountDocumentCleanup</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.accountDocumentCleanup = accountDocumentCleanup</code> |
| <code>personalTransferStorage</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.personalTransferStorage = personalTransferStorage</code> |
| <code>candidateCleanup</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.candidateCleanup = candidateCleanup</code> |

**本段进入 RoomEngine.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>{     store,     cfg,     peers,     instance,     transactions,     queue,     snapshots,     admissions,     updates,     reading,     awareness,     captures,     mergeCandidate …</code>（长表达式，按本步完整代码继续） | <code>解构对象</code> | 在此声明中的实际使用：<code>this.store = store</code>；<code>this.cfg = cfg</code>；<code>this.peers = peers</code>；<code>this.instance = instance</code><br>另有 21 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

<!-- write-step: 1 bytes=0:1116 -->
````javascript
// RoomEngine保留服务调用面，协调已分工的用例；这里不解释正文、执行SQL或直接发送socket。
export class RoomEngine {
  constructor({
    store,
    cfg,
    peers,
    instance,
    transactions,
    queue,
    snapshots,
    admissions,
    updates,
    reading,
    awareness,
    captures,
    mergeCandidates,
    mainCommits,
    mainCaptures,
    mainInstallations,
    draftPreviews,
    structurePreviews,
    draftTransitions,
    outputs,
    confirmations,
    recovery,
    accountDocumentCleanup,
    personalTransferStorage,
    candidateCleanup,
  }) {
    this.store = store;
    this.cfg = cfg;
    this.peers = peers;
    this.instance = instance;
    this.transactions = transactions;
    this.queue = queue;
    this.snapshots = snapshots;
    this.admissions = admissions;
    this.updates = updates;
    this.reading = reading;
    this.awarenessUpdates = awareness;
    this.captures = captures;
    this.mergeCandidates = mergeCandidates;
    this.mainCommits = mainCommits;
    this.mainCaptures = mainCaptures;
    this.mainInstallations = mainInstallations;
````

## 第 2 步：RoomEngine（接着实现）

插入位置：第 1 步代码之后，紧接追加；完成稿约第 46 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 RoomEngine.outputs：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>this.deliveryOutputs.outputs</code> |

**本段进入 RoomEngine.roomLocks：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>this.queue.locks</code> |

**本段进入 RoomEngine.recovered：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>this.recovery.recovered</code> |

**本段进入 RoomEngine.transaction：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>body</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions.run(body)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.transactions.run(body)</code> |

**本段进入 RoomEngine.serialized：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.queue.run(room, body)</code> |
| <code>body</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.queue.run(room, body)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.queue.run(room, body)</code> |

**本段进入 RoomEngine.initialize：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.recovery.initialize()</code> |

**本段进入 RoomEngine.verifyRooms：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.snapshots.verifyRooms()</code> |

**本段进入 RoomEngine.restore：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots.restore(connection, room, cut)</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots.restore(connection, room, cut)</code> |
| <code>cut = room.last_durable_seq</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>this.snapshots.restore(connection, room, cut)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.snapshots.restore(connection, room, cut)</code> |

**本段进入 RoomEngine.checkpoint：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots.checkpoint(connection, room, doc, cut)</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots.checkpoint(connection, room, doc, cut)</code> |
| <code>doc</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots.checkpoint(connection, room, doc, cut)</code> |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots.checkpoint(connection, room, doc, cut)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.snapshots.checkpoint(connection, room, doc, cut)</code> |

**本段进入 RoomEngine.open：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.admissions.open(input, handler)</code> |
| <code>handler</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.admissions.open(input, handler)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.admissions.open(input, handler)</code> |

**本段进入 RoomEngine.authorized：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.admissions.authorized(connection, binding, input)</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.admissions.authorized(connection, binding, input)</code> |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.admissions.authorized(connection, binding, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.admissions.authorized(connection, binding, input)</code> |

**本段进入 RoomEngine.update：**

**这一段为什么这样写：** 在真正保存前维护结构握手的保存证据有效性。

1. ready参与者禁止继续写
2. 清除该连接旧FLUSH证据
3. 委派RoomUpdates


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 实际连接准入绑定<br>在此声明中的实际使用：<code>this.deliveryOutputs.assertStructureWrite(binding)</code>；<code>binding.delivery</code>；<code>this.updates.update(binding, input)</code> |
| <code>input</code> | <code>JavaScript 参数</code> | 客户端UPDATE帧<br>在此声明中的实际使用：<code>this.updates.update(binding, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.updates.update(binding, input)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return this.deliveryOutputs.outputs;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.queue.locks;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.recovery.recovered;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.transactions.run(body);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.queue.run(room, body);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.recovery.initialize();</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.snapshots.verifyRooms();</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.snapshots.restore(connection, room, cut);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.snapshots.checkpoint(connection, room, doc, cut);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.admissions.open(input, handler);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.admissions.authorized(connection, binding, input);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=1116:2522 -->
````javascript
    this.draftPreviews = draftPreviews;
    this.structurePreviews = structurePreviews;
    this.draftTransitions = draftTransitions;
    this.deliveryOutputs = outputs;
    this.confirmations = confirmations;
    this.recovery = recovery;
    this.accountDocumentCleanup = accountDocumentCleanup;
    this.personalTransferStorage = personalTransferStorage;
    this.candidateCleanup = candidateCleanup;
  }
  get outputs() {
    return this.deliveryOutputs.outputs;
  }
  get roomLocks() {
    return this.queue.locks;
  }
  get recovered() {
    return this.recovery.recovered;
  }
  async transaction(body) {
    return this.transactions.run(body);
  }
  async serialized(room, body) {
    return this.queue.run(room, body);
  }
  async initialize() {
    return this.recovery.initialize();
  }
  async verifyRooms() {
    return this.snapshots.verifyRooms();
  }
  async restore(connection, room, cut = room.last_durable_seq) {
    return this.snapshots.restore(connection, room, cut);
  }
  async checkpoint(connection, room, doc, cut) {
    return this.snapshots.checkpoint(connection, room, doc, cut);
  }
  async open(input, handler) {
    return this.admissions.open(input, handler);
  }
  async authorized(connection, binding, input) {
    return this.admissions.authorized(connection, binding, input);
  }
  async update(binding, input) {
    this.deliveryOutputs.assertStructureWrite(binding);
````

## 第 3 步：RoomEngine（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 91 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 RoomEngine.flush：**

**这一段为什么这样写：** 将持久FLUSH回执登记到同一实际连接供结构握手核验。

1. 调用RoomReading.flush
2. 记录成功FLUSHED
3. 返回原结果


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 实际连接准入绑定<br>在此声明中的实际使用：<code>this.reading.flush(binding, input)</code>；<code>this.deliveryOutputs.recordFlush(binding, result)</code> |
| <code>input</code> | <code>JavaScript 参数</code> | 原操作ID和依赖集<br>在此声明中的实际使用：<code>this.reading.flush(binding, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>result</code> |

**本段进入 RoomEngine.structureReady：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.deliveryOutputs.structureReady(binding, input)</code> |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.deliveryOutputs.structureReady(binding, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。 |

**本段进入 RoomEngine.sync：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.reading.sync(binding, input)</code> |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.reading.sync(binding, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.reading.sync(binding, input)</code> |

**本段进入 RoomEngine.awareness：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.awarenessUpdates.publish(binding, input)</code> |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.awarenessUpdates.publish(binding, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.awarenessUpdates.publish(binding, input)</code> |

**本段进入 RoomEngine.attach：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.deliveryOutputs.attach(binding, socket)</code> |
| <code>socket</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.deliveryOutputs.attach(binding, socket)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>this.deliveryOutputs.attach(binding, socket)</code> |

**本段进入 RoomEngine.send：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.deliveryOutputs.send(binding, message)</code> |
| <code>message</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.deliveryOutputs.send(binding, message)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.deliveryOutputs.send(binding, message)</code> |

**本段进入 RoomEngine.broadcast：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.deliveryOutputs.broadcast(room, message)</code> |
| <code>message</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.deliveryOutputs.broadcast(room, message)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.deliveryOutputs.broadcast(room, message)</code> |

**本段进入 RoomEngine.settle：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.deliveryOutputs.settle(binding, reason)</code> |
| <code>reason</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.deliveryOutputs.settle(binding, reason)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.deliveryOutputs.settle(binding, reason)</code> |

**本段进入 RoomEngine.recoverResources：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.confirmations.recover(operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.confirmations.recover(operation)</code> |

**本段进入 RoomEngine.stopScope：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.deliveryOutputs.stopScope(scope)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.deliveryOutputs.stopScope(scope)</code> |

**跟着本段代码走一遍：**

- 检查 <code>output</code>。
- 此处通过 <code>return this.updates.update(binding, input);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>this.reading.flush(binding, input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.reading.sync(binding, input);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.awarenessUpdates.publish(binding, input);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.deliveryOutputs.attach(binding, socket);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.deliveryOutputs.send(binding, message);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.deliveryOutputs.broadcast(room, message);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.deliveryOutputs.settle(binding, reason);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.confirmations.recover(operation);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.deliveryOutputs.stopScope(scope);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2522:3665 -->
````javascript
    const output = this.outputs.get(binding.delivery);
    if (output) output.lastFlush = null;
    return this.updates.update(binding, input);
  }
  async flush(binding, input) {
    const result = await this.reading.flush(binding, input);
    this.deliveryOutputs.recordFlush(binding, result);
    return result;
  }
  structureReady(binding, input) {
    this.deliveryOutputs.structureReady(binding, input);
  }
  async sync(binding, input) {
    return this.reading.sync(binding, input);
  }
  async awareness(binding, input) {
    return this.awarenessUpdates.publish(binding, input);
  }
  attach(binding, socket) {
    return this.deliveryOutputs.attach(binding, socket);
  }
  async send(binding, message) {
    return this.deliveryOutputs.send(binding, message);
  }
  async broadcast(room, message) {
    return this.deliveryOutputs.broadcast(room, message);
  }
  async settle(binding, reason) {
    return this.deliveryOutputs.settle(binding, reason);
  }
  async recoverResources(operation) {
    return this.confirmations.recover(operation);
  }
  async stopScope(scope) {
    return this.deliveryOutputs.stopScope(scope);
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/room_engine.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/07-wiring-readiness.md)。
