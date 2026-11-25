# 分步编写 `services/search/internal/repository/generation/active.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/387-active.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取并锁定别名对应的SQL物理索引代次，阻止切换期间继续写。

普通消费与重建切换必须竞争同一代次行，不能在一条Canal记录处理中间换索引。

写之前先按这条执行路线理解各段的连接：

1. Find根据exclusive选择FOR SHARE/UPDATE，核physical/generation/revision
2. CheckSwitching拒绝任意SWITCHING未恢复作业
3. LockCurrent组合当前代次存在性和切换检查
4. InsertInitial只登记经过ES归属核验的初代绑定

本文件需要守住的边界：

- 锁顺序先index_generation再event/application，重建也相同
- alias不能等于physical，缺代次不是自动沿配置继续写

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/repository/generation' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/repository/generation/active.go') -and (Get-Item -LiteralPath 'services/search/internal/repository/generation/active.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/repository/generation/active.go')) { New-Item -ItemType File -Path 'services/search/internal/repository/generation/active.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:79 -->
````go
package generation

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
)

````

## 第 2 步：Target

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`type Target struct {`。

**这一段为什么这样写：** 一次记录处理期间固定的写入目标。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Physical</code> | <code>string</code> | 具体ES物理索引名 |
| <code>Generation</code> | <code>uint64</code> | 逻辑索引代次，重建切换后递增 |
| <code>Revision</code> | <code>uint64</code> | SQL绑定修订号，用于比较原代次 |

<!-- write-step: 2 bytes=79:160 -->
````go
type Target struct {
	Physical   string
	Generation uint64
	Revision   uint64
}

````

## 第 3 步：indexName

插入位置：第 2 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`var indexName = regexp.MustCompile(\`^[a-z0-9][a-z0-9._-]{0,159}$\`)`。

<!-- write-step: 3 bytes=160:228 -->
````go
var indexName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)

````

## 第 4 步：ValidateNames

插入位置：第 3 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func ValidateNames(alias, physical string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>alias</code> | <code>string</code> | 在此声明中的实际使用：<code>indexName.MatchString(alias)</code>；<code>alias == physical</code> |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>indexName.MatchString(physical)</code>；<code>alias == physical</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_GENERATION_CONFIG_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!indexName.MatchString(alias) &#124;&#124; !indexName.MatchString(physical) &#124;&#124; alias == physical</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_GENERATION_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=228:446 -->
````go
func ValidateNames(alias, physical string) error {
	if !indexName.MatchString(alias) || !indexName.MatchString(physical) || alias == physical {
		return errors.New("SEARCH_GENERATION_CONFIG_INVALID")
	}
	return nil
}

````

## 第 5 步：Find

插入位置：第 4 步代码之后，紧接追加；完成稿约第 25 行。

Find由调用方决定共享/独占锁；锁顺序始终先index_generation，再原事件和应用水位。
后续重建进入SWITCHING以及切SQL代次也必须先拿这行的独占锁。

本步声明：`func Find(ctx context.Context, tx *sql.Tx, alias string, exclusive bool) (Target, bool, error)`。

**这一段为什么这样写：** 提供代次的共享读取或独占切换锁。

1. 校验alias
2. 选择对应锁SQL
3. 核读出绑定


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 事务期限<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, query, alias)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 外层事务连接<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>alias</code> | <code>string</code> | 逻辑索引别名<br>在此声明中的实际使用：<code>indexName.MatchString(alias)</code>；<code>tx.QueryRowContext(ctx, query, alias)</code>；<code>ValidateNames(alias, target.Physical)</code> |
| <code>exclusive</code> | <code>bool</code> | true供初始化/切换独占，false供消费共享 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Target</code> | Target绑定<br>实际返回表达式：<code>target</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 是否存在代次行<br>实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | SQL或绑定错误<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ALIAS_INVALID&quot;)</code>；<code>nil</code>；<code>errors.New(&quot;SEARCH_GENERATION_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_GENERATION_UNBOUND&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!indexName.MatchString(alias)</code>。条件成立时执行 <code>return target, false, errors.New(&quot;SEARCH_ALIAS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>exclusive</code>。
- 执行 <code>tx.QueryRowContext(ctx, query, alias).Scan(&amp;target.Physical, &amp;target.Generation, &amp;target.Revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, query, alias)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return target, false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return target, false, errors.New(&quot;SEARCH_GENERATION_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>ValidateNames(alias, target.Physical) != nil &#124;&#124; target.Generation == 0 &#124;&#124; target.Revision == 0</code>。条件成立时执行 <code>return target, false, errors.New(&quot;SEARCH_GENERATION_UNBOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return target, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=446:1537 -->
````go
// Find由调用方决定共享/独占锁；锁顺序始终先index_generation，再原事件和应用水位。
// 后续重建进入SWITCHING以及切SQL代次也必须先拿这行的独占锁。
func Find(ctx context.Context, tx *sql.Tx, alias string, exclusive bool) (Target, bool, error) {
	var target Target
	if !indexName.MatchString(alias) {
		return target, false, errors.New("SEARCH_ALIAS_INVALID")
	}
	query := "SELECT physical_index,generation,revision FROM index_generation WHERE alias_name=? FOR SHARE"
	if exclusive {
		query = "SELECT physical_index,generation,revision FROM index_generation WHERE alias_name=? FOR UPDATE"
	}
	err := tx.QueryRowContext(ctx, query, alias).Scan(&target.Physical, &target.Generation, &target.Revision)
	if err == sql.ErrNoRows {
		return target, false, nil
	}
	if err != nil {
		return target, false, errors.New("SEARCH_GENERATION_READ_FAILED")
	}
	if ValidateNames(alias, target.Physical) != nil || target.Generation == 0 || target.Revision == 0 {
		return target, false, errors.New("SEARCH_GENERATION_UNBOUND")
	}
	return target, true, nil
}

````

## 第 6 步：CheckSwitching

插入位置：第 5 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`func CheckSwitching(ctx context.Context, tx *sql.Tx) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM rebuild_job WHERE state=&#x27;SWITCHING&#x27; ORDER BY id LIMIT 1 FOR SHARE&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;SEARCH_GENERATION_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_GENERATION_RECOVERY_REQUIRED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM rebuild_job WHERE state=&#x27;SWITCHING&#x27; ORDER BY id LIMIT 1 FOR SHARE&quot;).Scan(&amp;job)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM rebuild_job WHERE state=&#x27;SWITCHING&#x27; ORDER BY id LIMIT 1 FOR SHARE&quot;)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_GENERATION_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;SEARCH_GENERATION_RECOVERY_REQUIRED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1537:1918 -->
````go
func CheckSwitching(ctx context.Context, tx *sql.Tx) error {
	var job uint64
	err := tx.QueryRowContext(ctx, "SELECT id FROM rebuild_job WHERE state='SWITCHING' ORDER BY id LIMIT 1 FOR SHARE").Scan(&job)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_GENERATION_READ_FAILED")
	}
	return errors.New("SEARCH_GENERATION_RECOVERY_REQUIRED")
}

````

## 第 7 步：LockCurrent

插入位置：第 6 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`func LockCurrent(ctx context.Context, tx *sql.Tx, alias string) (Target, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>Find(ctx, tx, alias, false)</code>；<code>CheckSwitching(ctx, tx)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>Find(ctx, tx, alias, false)</code>；<code>CheckSwitching(ctx, tx)</code> |
| <code>alias</code> | <code>string</code> | 在此声明中的实际使用：<code>Find(ctx, tx, alias, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Target</code> | 使用 <code>Target</code> 类型；其定义处的职责：一次记录处理期间固定的写入目标。<br>实际返回表达式：<code>target</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>；<code>CheckSwitching(ctx, tx)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return target, err</code>。当前函数在这里结束。
- 检查 <code>!exists</code>。条件成立时执行 <code>return target, errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return target, CheckSwitching(ctx, tx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1918:2210 -->
````go
func LockCurrent(ctx context.Context, tx *sql.Tx, alias string) (Target, error) {
	target, exists, err := Find(ctx, tx, alias, false)
	if err != nil {
		return target, err
	}
	if !exists {
		return target, errors.New("SEARCH_GENERATION_MISSING")
	}
	return target, CheckSwitching(ctx, tx)
}

````

## 第 8 步：InsertInitial

插入位置：第 7 步代码之后，紧接追加；完成稿约第 72 行。

本步声明：`func InsertInitial(ctx context.Context, tx *sql.Tx, alias, physical string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO index_generation(alias_name,physical_index) VALUES(?,?)&quot;, alias, physical)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>alias</code> | <code>string</code> | 在此声明中的实际使用：<code>ValidateNames(alias, physical)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO index_generation(alias_name,physical_index) VALUES(?,?)&quot;, alias, physical)</code> |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>ValidateNames(alias, physical)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO index_generation(alias_name,physical_index) VALUES(?,?)&quot;, alias, physical)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_GENERATION_WRITE_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_GENERATION_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO index_generation(alias_name,physical_index) VALUES(?,?)&quot;, alias, physical)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2210:2573 -->
````go
func InsertInitial(ctx context.Context, tx *sql.Tx, alias, physical string) error {
	if err := ValidateNames(alias, physical); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO index_generation(alias_name,physical_index) VALUES(?,?)", alias, physical); err != nil {
		return errors.New("SEARCH_GENERATION_WRITE_FAILED")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/repository/generation/active.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
