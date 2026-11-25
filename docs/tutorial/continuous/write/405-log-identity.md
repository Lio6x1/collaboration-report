# 分步编写 `services/search/internal/svc/log_identity.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/405-log-identity.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

首次持久绑定普通消费者的物理日志身份，并保守处理已有旧checkpoint。

当前同名主题的UUID不能证明旧offset曾属于它；有历史但无身份时必须有本owner先前prepare证据。

写之前先按这条执行路线理解各段的连接：

1. bindConsumerLog锁当前generation
2. 无consumer_log_identity时检查是否有consumer_checkpoint
3. 存在历史则取最早rebuild_binding并核owner/alias/topic/cluster/TopicID
4. BindLogIdentity只建立不覆盖原身份，COMMIT未知保留原值重查

本文件需要守住的边界：

- 不能用今天Capture的身份覆盖旧历史
- SEARCH_LOG_IDENTITY_BOOTSTRAP_REQUIRED需要补正确迁移依据，不可删除旧checkpoint绕过

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/svc' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/svc/log_identity.go') -and (Get-Item -LiteralPath 'services/search/internal/svc/log_identity.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/svc/log_identity.go')) { New-Item -ItemType File -Path 'services/search/internal/svc/log_identity.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:336 -->
````go
package svc

import (
	"collaboration/platform"
	"collaboration/search/internal/client/logsource"
	event "collaboration/search/internal/repository/event"
	generation "collaboration/search/internal/repository/generation"
	rebuild "collaboration/search/internal/repository/rebuild"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
)

````

## 第 2 步：bindConsumerLog

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

bindConsumerLog在首次持久绑定时保守处理旧检查点。已有消费历史必须有本owner
先前签名prepare作为身份依据；不能仅凭今天的同名主题把历史offset重新解释。

本步声明：`func bindConsumerLog(ctx context.Context, db *sql.DB, cfg *platform.SearchConfig, identity logsource.Identity) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>generation.Find(ctx, tx, cfg.Alias, false)</code>；<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_log_identity WHERE consumer_name=?`, cfg.Group)</code>；<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_checkpoint WHERE consumer_name=?`, cfg.Group)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.BeginTx</code> |
| <code>cfg</code> | <code>*platform.SearchConfig</code> | 使用 <code>platform.SearchConfig</code> 类型；其定义处的职责：加载服务自身配置，创建有界数据库池，并以迁移和恢复事实判定就绪。<br>在此声明中的实际使用：<code>cfg.Alias</code>；<code>cfg.Group</code>；<code>cfg.Owner</code> |
| <code>identity</code> | <code>logsource.Identity</code> | 使用 <code>logsource.Identity</code> 类型；其定义处的职责：不可替换的物理日志身份。<br>在此声明中的实际使用：<code>identity.Topic</code>；<code>identity.Cluster</code>；<code>identity.TopicID</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_LOG_IDENTITY_TRANSACTION_FAILED&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>；<code>errors.New(&quot;SEARCH_LOG_IDENTITY_READ_FAILED&quot;)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_IDENTITY_TRANSACTION_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>。当前函数在这里结束。
- 检查 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_log_identity WHERE consumer_name=?`, cfg.Group).Scan(&amp;exists) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_IDENTITY_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_log_identity WHERE consumer_name=?`, cfg.Group).Scan(&amp;exists)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_log_identity WHERE consumer_name=?`, cfg.Group)</code>。
- 检查 <code>exists == 0</code>。
- 检查 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_checkpoint WHERE consumer_name=?`, cfg.Group).Scan(&amp;history) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_checkpoint WHERE consumer_name=?`, cfg.Group).Scan(&amp;history)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_checkpoint WHERE consumer_name=?`, cfg.Group)</code>。
- 检查 <code>history &gt; 0</code>。
- 检查 <code>tx.QueryRowContext(ctx, `SELECT operation_id FROM rebuild_binding WHERE alias_name=? ORDER BY created_at,operation_id LIMIT 1`, cfg.Alias).Scan(&amp;operation) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_IDENTITY_BOOTSTRAP_REQUIRED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT operation_id FROM rebuild_binding WHERE alias_name=? ORDER BY created_at,operation_id LIMIT 1`, cfg.Alias).Scan(&amp;operation)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT operation_id FROM rebuild_binding WHERE alias_name=? ORDER BY created_at,operation_id LIMIT 1`, cfg.Alias)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!found &#124;&#124; p.Owner != cfg.Owner &#124;&#124; p.Alias != cfg.Alias &#124;&#124; p.Topic != identity.Topic &#124;&#124; p.Cluster != identity.Cluster &#124;&#124; p.TopicID != hex.EncodeToString(identity.TopicID&#91;:&#93;)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_IDENTITY_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=336:2332 -->
````go
// bindConsumerLog在首次持久绑定时保守处理旧检查点。已有消费历史必须有本owner
// 先前签名prepare作为身份依据；不能仅凭今天的同名主题把历史offset重新解释。
func bindConsumerLog(ctx context.Context, db *sql.DB, cfg *platform.SearchConfig, identity logsource.Identity) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_LOG_IDENTITY_TRANSACTION_FAILED")
	}
	defer tx.Rollback()
	if _, found, err := generation.Find(ctx, tx, cfg.Alias, false); err != nil {
		return err
	} else if !found {
		return errors.New("SEARCH_GENERATION_MISSING")
	}
	var exists uint64
	if tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_log_identity WHERE consumer_name=?`, cfg.Group).Scan(&exists) != nil {
		return errors.New("SEARCH_LOG_IDENTITY_READ_FAILED")
	}
	if exists == 0 {
		var history uint64
		if tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_checkpoint WHERE consumer_name=?`, cfg.Group).Scan(&history) != nil {
			return errors.New("SEARCH_CHECKPOINT_READ_FAILED")
		}
		if history > 0 {
			var operation string
			if tx.QueryRowContext(ctx, `SELECT operation_id FROM rebuild_binding WHERE alias_name=? ORDER BY created_at,operation_id LIMIT 1`, cfg.Alias).Scan(&operation) != nil {
				return errors.New("SEARCH_LOG_IDENTITY_BOOTSTRAP_REQUIRED")
			}
			binding, found, err := rebuild.Find(ctx, tx, operation)
			if err != nil {
				return err
			}
			p := binding.Prepare
			if !found || p.Owner != cfg.Owner || p.Alias != cfg.Alias || p.Topic != identity.Topic || p.Cluster != identity.Cluster || p.TopicID != hex.EncodeToString(identity.TopicID[:]) {
				return errors.New("SEARCH_LOG_IDENTITY_CHANGED")
			}
		}
	}
	if err := event.BindLogIdentity(ctx, tx, cfg.Group, identity.Topic, identity.Cluster, identity.TopicID); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_LOG_IDENTITY_COMMIT_UNKNOWN")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/svc/log_identity.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
