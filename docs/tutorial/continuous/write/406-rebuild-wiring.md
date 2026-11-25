# 分步编写 `services/search/internal/svc/rebuild_wiring.go`

先按 [S05 · 固定窗口重放、完整核验与别名恢复切换](../lessons/search/05-cutover-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/406-rebuild-wiring.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将Doc权威读取、ES客户端、Kafka日志读取和重建阶段用例装配为一个worker。

装配集中在边界，使业务只依赖小接口；播种不使用事件账本，重放则必须按job创建专用consumer处理器。

写之前先按这条执行路线理解各段的连接：

1. NewRebuild验证search配置并固定accepted目录，供RPC开始/查询原任务。
2. NewRebuildWorker先建Doc、ES、Kafka依赖，失败时关闭已创建客户端，再装配Seeder、Replayer及Coordinator。
3. Step按播种→重放→协调执行；RecoveryReady只检查SQL持久SWITCHING，普通消费者还要自行做日志恢复。

本文件需要守住的边界：

- RecoveryReady通过不等于普通消费者已初始化完成。
- 任一阶段失败当步返回，不能跳过失败阶段继续切换；Close释放日志和ES客户端。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/svc' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/svc/rebuild_wiring.go') -and (Get-Item -LiteralPath 'services/search/internal/svc/rebuild_wiring.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/svc/rebuild_wiring.go')) { New-Item -ItemType File -Path 'services/search/internal/svc/rebuild_wiring.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:504 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	"collaboration/platform"
	"collaboration/search/internal/client/document"
	"collaboration/search/internal/client/elastic"
	"collaboration/search/internal/client/logsource"
	"collaboration/search/internal/logic/indexing"
	"collaboration/search/internal/logic/rebuild"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
)

// rebuild.go：保留原实现，按业务合并文件。

````

## 第 2 步：NewRebuild

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func NewRebuild(store *platform.Store, cfg platform.Config) (*rebuild.JobService, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | search角色的私有Store，提供同一search数据库。<br>在此声明中的实际使用：<code>store == nil</code>；<code>store.DB</code>；<code>store.Role</code> |
| <code>cfg</code> | <code>platform.Config</code> | 本服务配置，Root必须是绝对路径且Search配置完整。<br>在此声明中的实际使用：<code>cfg.Role</code>；<code>cfg.Search</code>；<code>cfg.Root</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*rebuild.JobService</code> | 使用 <code>rebuild.JobService</code> 类型；其定义处的职责：组织重建准备、任务接受、封存切换屏障、两阶段完整验证及未知切换恢复。<br>实际返回表达式：<code>nil</code>；<code>&amp;rebuild.JobService{DB: store.DB, Owner: cfg.Search.Owner, Alias: cfg.Search.Alias, AcceptedDir: filepath.Join(cfg.Root, &quot;.runtime&quot;, &quot;rebuild&quot;, &quot;search&quot;, &quot;accepted&quot;)}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_CONFIG_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>store == nil &#124;&#124; store.DB == nil &#124;&#124; store.Role != &quot;search&quot; &#124;&#124; cfg.Role != &quot;search&quot; &#124;&#124; cfg.Search == nil &#124;&#124; !filepath.IsAbs(cfg.Root) &#124;&#124; cfg.Search.Owner == &quot;&quot; &#124;&#124; cfg.Search.Alias == &quot;&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;rebuild.JobService{DB: store.DB, Owner: cfg.Search.Owner, Alias: cfg.Search.Alias, AcceptedDir: filepath.Join(cfg.Root, &quot;.runtime&quot;, &quot;rebuild&quot;, &quot;search&quot;, &quot;accepted&quot;)}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=504:1096 -->
````go
func NewRebuild(store *platform.Store, cfg platform.Config) (*rebuild.JobService, error) {
	if store == nil || store.DB == nil || store.Role != "search" || cfg.Role != "search" || cfg.Search == nil || !filepath.IsAbs(cfg.Root) || cfg.Search.Owner == "" || cfg.Search.Alias == "" {
		return nil, errors.New("SEARCH_REBUILD_CONFIG_INVALID")
	}
	return &rebuild.JobService{DB: store.DB, Owner: cfg.Search.Owner, Alias: cfg.Search.Alias, AcceptedDir: filepath.Join(cfg.Root, ".runtime", "rebuild", "search", "accepted")}, nil
}

// rebuild_worker.go：保留原实现，按业务合并文件。

````

## 第 3 步：RebuildWorker

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`type RebuildWorker struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>logic</code> | <code>*rebuild.Seeder</code> | 分批播种已接受Doc快照的Seeder。 |
| <code>replay</code> | <code>*rebuild.Replayer</code> | 按原L至H重放日志的Replayer。 |
| <code>index</code> | <code>*elastic.Client</code> | worker持有的ES客户端，退出时关闭。 |
| <code>logs</code> | <code>*logsource.Client</code> | worker持有的Kafka原日志读取客户端。 |
| <code>coordinator</code> | <code>*rebuild.Coordinator</code> | 封H、验证并完成原切换的协调器。 |

<!-- write-step: 3 bytes=1096:1281 -->
````go
type RebuildWorker struct {
	logic       *rebuild.Seeder
	replay      *rebuild.Replayer
	index       *elastic.Client
	logs        *logsource.Client
	coordinator *rebuild.Coordinator
}

````

## 第 4 步：NewRebuildWorker

插入位置：第 3 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`func NewRebuildWorker(store *platform.Store, cfg platform.Config) (*RebuildWorker, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | search角色的私有Store，提供同一search数据库。<br>在此声明中的实际使用：<code>store == nil</code>；<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 本服务配置，Root必须是绝对路径且Search配置完整。<br>在此声明中的实际使用：<code>cfg.Role</code>；<code>cfg.Search</code>；<code>cfg.Root</code>；<code>document.NewSnapshots(cfg)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*RebuildWorker</code> | 实际返回表达式：<code>nil</code>；<code>&amp;RebuildWorker{logic: seed, replay: replay, index: sink, logs: logs, coordinator: coordinator}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_CONFIG_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>store == nil &#124;&#124; store.DB == nil &#124;&#124; cfg.Role != &quot;search&quot; &#124;&#124; cfg.Search == nil &#124;&#124; !filepath.IsAbs(cfg.Root)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return indexing.New(store.DB, source, sink, consumer)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &amp;RebuildWorker{logic: seed, replay: replay, index: sink, logs: logs, coordinator: coordinator}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1281:2604 -->
````go
func NewRebuildWorker(store *platform.Store, cfg platform.Config) (*RebuildWorker, error) {
	if store == nil || store.DB == nil || cfg.Role != "search" || cfg.Search == nil || !filepath.IsAbs(cfg.Root) {
		return nil, errors.New("SEARCH_REBUILD_CONFIG_INVALID")
	}
	source, err := document.NewSnapshots(cfg)
	if err != nil {
		return nil, err
	}
	sink, err := elastic.New(cfg.Search)
	if err != nil {
		return nil, err
	}
	logs, err := logsource.New(cfg.Search.Brokers, cfg.Search.Topic)
	if err != nil {
		sink.Close()
		return nil, err
	}
	// 种子不写事件账本，Processor的consumer字段不会参与此入口。
	writer := indexing.New(store.DB, source, sink, "")
	seed := &rebuild.Seeder{DB: store.DB, Owner: cfg.Search.Owner, Alias: cfg.Search.Alias,
		AcceptedDir: filepath.Join(cfg.Root, ".runtime", "rebuild", "search", "accepted"), Writer: writer, Indexes: sink, Logs: logs}
	replay := &rebuild.Replayer{Seed: seed, Logs: logs, Processor: func(consumer string) rebuild.ReplayProcessor {
		return indexing.New(store.DB, source, sink, consumer)
	}}
	coordinator := &rebuild.Coordinator{Barrier: &rebuild.Barrier{Seed: seed, Indexes: sink}, Validator: &rebuild.Validator{Seed: seed, Indexes: sink}}
	return &RebuildWorker{logic: seed, replay: replay, index: sink, logs: logs, coordinator: coordinator}, nil
}
````

## 第 5 步：RebuildWorker.Step

插入位置：第 4 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`func (s *RebuildWorker) Step(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.logic.Step(ctx)</code>；<code>s.replay.Step(ctx)</code>；<code>s.coordinator.Step(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>s.coordinator.Step(ctx)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return s.coordinator.Step(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2604:2816 -->
````go
func (s *RebuildWorker) Step(ctx context.Context) error {
	if err := s.logic.Step(ctx); err != nil {
		return err
	}
	if err := s.replay.Step(ctx); err != nil {
		return err
	}
	return s.coordinator.Step(ctx)
}

````

## 第 6 步：RebuildWorker.RecoveryReady

插入位置：第 5 步代码之后，紧接追加；完成稿约第 73 行。

RecoveryReady只判定持久切换屏障；普通consumer自己的Initialize仍负责日志恢复。

本步声明：`func (s *RebuildWorker) RecoveryReady(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.logic.DB.QueryRowContext(ctx, `SELECT id FROM rebuild_job WHERE state=&#x27;SWITCHING&#x27; LIMIT 1`)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;SEARCH_REBUILD_JOB_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_GENERATION_RECOVERY_REQUIRED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.logic.DB.QueryRowContext(ctx, `SELECT id FROM rebuild_job WHERE state=&#x27;SWITCHING&#x27; LIMIT 1`).Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>s.logic.DB.QueryRowContext(ctx, `SELECT id FROM rebuild_job WHERE state=&#x27;SWITCHING&#x27; LIMIT 1`)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_JOB_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;SEARCH_GENERATION_RECOVERY_REQUIRED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2816:3291 -->
````go
// RecoveryReady只判定持久切换屏障；普通consumer自己的Initialize仍负责日志恢复。
func (s *RebuildWorker) RecoveryReady(ctx context.Context) error {
	var id uint64
	err := s.logic.DB.QueryRowContext(ctx, `SELECT id FROM rebuild_job WHERE state='SWITCHING' LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_REBUILD_JOB_READ_FAILED")
	}
	return errors.New("SEARCH_GENERATION_RECOVERY_REQUIRED")
}
````

## 第 7 步：RebuildWorker.Close

插入位置：第 6 步代码之后，紧接追加；完成稿约第 85 行。

本步声明：`func (s *RebuildWorker) Close()`。

<!-- write-step: 7 bytes=3291:3359 -->
````go
func (s *RebuildWorker) Close() { s.logs.Close(); s.index.Close() }
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/svc/rebuild_wiring.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/05-cutover-wiring.md)。
