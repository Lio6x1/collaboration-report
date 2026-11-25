# 分步编写 `services/search/internal/logic/generation/guard.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/393-guard.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

用完整记录期间的共享锁固定ES/SQL索引代次，并恢复初代别名建立。

ES建立成功而SQL提交未知时只能重核原绑定；Do不能假装ES/SQL/Kafka是一个事务。

写之前先按这条执行路线理解各段的连接：

1. Initialize以独占代次锁查SQL，已有则ResolveAlias一致；没有则EnsureInitialAlias后InsertInitial
2. Do持FOR SHARE代次锁并核真实alias，使用30秒子context执行work
3. 回调期间包含投影、SQL回执、Kafka确认，结束释放共享锁

本文件需要守住的边界：

- 回调必须用传入context，不可取消后另起后台继续提交
- ES/SQL别名不符或SWITCHING时拒绝继续写

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/logic/generation' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/logic/generation/guard.go') -and (Get-Item -LiteralPath 'services/search/internal/logic/generation/guard.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/logic/generation/guard.go')) { New-Item -ItemType File -Path 'services/search/internal/logic/generation/guard.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:143 -->
````go
package generation

import (
	repository "collaboration/search/internal/repository/generation"
	"context"
	"database/sql"
	"errors"
	"time"
)

````

## 第 2 步：Indexes

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

本步声明：`type Indexes interface {`。

**这一段为什么这样写：** 代次用例需要的ES别名能力。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>EnsureInitialAlias</code> | <code>(context.Context, string, string) error</code> | 输入context、alias、initial physical；仅确保本owner原初代绑定，返回错误 |
| <code>ResolveAlias</code> | <code>(context.Context, string) (string, bool, error)</code> | 输入context和alias；返回唯一physical、是否存在及错误 |

<!-- write-step: 2 bytes=143:291 -->
````go
type Indexes interface {
	EnsureInitialAlias(context.Context, string, string) error
	ResolveAlias(context.Context, string) (string, bool, error)
}

````

## 第 3 步：Guard

插入位置：第 2 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`type Guard struct {`。

**这一段为什么这样写：** 跨整条记录保持物理目标稳定。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>database</code> | <code>*sql.DB</code> | Search私有SQL库 |
| <code>indexes</code> | <code>Indexes</code> | 真实ES别名客户端 |
| <code>alias</code> | <code>string</code> | 逻辑读取/写入别名 |
| <code>initial</code> | <code>string</code> | 首次初始化配置物理名 |

<!-- write-step: 3 bytes=291:384 -->
````go
type Guard struct {
	database *sql.DB
	indexes  Indexes
	alias    string
	initial  string
}

````

## 第 4 步：New

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func New(database *sql.DB, indexes Indexes, alias, initial string) (*Guard, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>database</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>database == nil</code>；<code>database: database</code> |
| <code>indexes</code> | <code>Indexes</code> | 使用 <code>Indexes</code> 类型；其定义处的职责：代次用例需要的ES别名能力。<br>在此声明中的实际使用：<code>indexes == nil</code>；<code>indexes: indexes</code> |
| <code>alias</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.ValidateNames(alias, initial)</code>；<code>alias: alias</code> |
| <code>initial</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.ValidateNames(alias, initial)</code>；<code>initial: initial</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Guard</code> | 使用 <code>Guard</code> 类型；其定义处的职责：跨整条记录保持物理目标稳定。<br>实际返回表达式：<code>nil</code>；<code>&amp;Guard{database: database, indexes: indexes, alias: alias, initial: initial}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_GENERATION_DEPENDENCIES_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>database == nil &#124;&#124; indexes == nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_GENERATION_DEPENDENCIES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Guard{database: database, indexes: indexes, alias: alias, initial: initial}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=384:763 -->
````go
func New(database *sql.DB, indexes Indexes, alias, initial string) (*Guard, error) {
	if database == nil || indexes == nil {
		return nil, errors.New("SEARCH_GENERATION_DEPENDENCIES_UNAVAILABLE")
	}
	if err := repository.ValidateNames(alias, initial); err != nil {
		return nil, err
	}
	return &Guard{database: database, indexes: indexes, alias: alias, initial: initial}, nil
}

````

## 第 5 步：Guard.Initialize

插入位置：第 4 步代码之后，紧接追加；完成稿约第 33 行。

初代创建是可核验的两系统恢复流程：先建立确切的owned ES绑定，再提交SQL。
ES已成功而SQL未知时，重启只能核验原配置的同一绑定，不允许覆盖任意alias。

本步声明：`func (g *Guard) Initialize(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 10*time.Second)</code>；<code>context.WithTimeout(ctx, 10*time.Second)</code>；<code>g.database.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.Find(ctx, tx, g.alias, true)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_GENERATION_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_ALIAS_GENERATION_MISMATCH&quot;)</code>；<code>errors.New(&quot;SEARCH_GENERATION_COMMIT_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 10*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>g.database.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_GENERATION_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>exists</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!present &#124;&#124; actual != target.Physical</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ALIAS_GENERATION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_GENERATION_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=763:1996 -->
````go
// 初代创建是可核验的两系统恢复流程：先建立确切的owned ES绑定，再提交SQL。
// ES已成功而SQL未知时，重启只能核验原配置的同一绑定，不允许覆盖任意alias。
func (g *Guard) Initialize(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := g.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_GENERATION_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	target, exists, err := repository.Find(ctx, tx, g.alias, true)
	if err != nil {
		return err
	}
	if err := repository.CheckSwitching(ctx, tx); err != nil {
		return err
	}
	if exists {
		actual, present, err := g.indexes.ResolveAlias(ctx, g.alias)
		if err != nil {
			return err
		}
		if !present || actual != target.Physical {
			return errors.New("SEARCH_ALIAS_GENERATION_MISMATCH")
		}
	} else {
		if err := g.indexes.EnsureInitialAlias(ctx, g.alias, g.initial); err != nil {
			return err
		}
		if err := repository.InsertInitial(ctx, tx, g.alias, g.initial); err != nil {
			return err
		}
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_GENERATION_COMMIT_UNKNOWN")
	}
	return nil
}

````

## 第 6 步：Guard.Do

插入位置：第 5 步代码之后，紧接追加；完成稿约第 72 行。

Do的事务只拥有代次共享锁，不写事件账本。事件事务仍各自明确提交，
这个跨完整记录的共享锁阻止重建在中途切代次；它不伪装成ES/SQL/Kafka原子事务。
回调必须使用传入的有界context，不能在锁已因取消释放后另起后台继续提交。

本步声明：`func (g *Guard) Do(ctx context.Context, work func(context.Context, repository.Target) error) error`。

**这一段为什么这样写：** 在回调全过程持有当前代次共享锁。

1. 创建有限context与事务
2. 锁当前代次并检查SWITCHING
3. 核真实ES别名
4. 执行work，退出回滚只读锁事务


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 上层处理生命周期<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 30*time.Second)</code>；<code>context.WithTimeout(ctx, 30*time.Second)</code>；<code>g.database.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.LockCurrent(ctx, tx, g.alias)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>work</code> | <code>func(context.Context, repository.Target) error</code> | 接收有界ctx和固定Target，完成一整条记录及确认<br>在此声明中的实际使用：<code>work == nil</code>；<code>work(ctx, target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 回调或代次核验错误<br>实际返回表达式：<code>errors.New(&quot;SEARCH_GENERATION_WORK_REQUIRED&quot;)</code>；<code>errors.New(&quot;SEARCH_GENERATION_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_ALIAS_GENERATION_MISMATCH&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>work == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_GENERATION_WORK_REQUIRED&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 30*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>g.database.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_GENERATION_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!exists &#124;&#124; actual != target.Physical</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ALIAS_GENERATION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return work(ctx, target)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1996:3072 -->
````go
// Do的事务只拥有代次共享锁，不写事件账本。事件事务仍各自明确提交，
// 这个跨完整记录的共享锁阻止重建在中途切代次；它不伪装成ES/SQL/Kafka原子事务。
// 回调必须使用传入的有界context，不能在锁已因取消释放后另起后台继续提交。
func (g *Guard) Do(ctx context.Context, work func(context.Context, repository.Target) error) error {
	if work == nil {
		return errors.New("SEARCH_GENERATION_WORK_REQUIRED")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := g.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_GENERATION_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	target, err := repository.LockCurrent(ctx, tx, g.alias)
	if err != nil {
		return err
	}
	actual, exists, err := g.indexes.ResolveAlias(ctx, g.alias)
	if err != nil {
		return err
	}
	if !exists || actual != target.Physical {
		return errors.New("SEARCH_ALIAS_GENERATION_MISMATCH")
	}
	return work(ctx, target)
}

````

## 第 7 步：Guard.Ready

插入位置：第 6 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`func (g *Guard) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>g.Do(ctx, func(context.Context, repository.Target) error { return nil })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>g.Do(ctx, func(context.Context, repository.Target) error { return nil })</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return g.Do(ctx, func(context.Context, repository.Target) error { return nil })</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3072:3206 -->
````go
func (g *Guard) Ready(ctx context.Context) error {
	return g.Do(ctx, func(context.Context, repository.Target) error { return nil })
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/logic/generation/guard.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
