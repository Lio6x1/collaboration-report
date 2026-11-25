# 分步编写 `services/search/internal/client/elastic/aliases_client.go`

先按 [S05 · 固定窗口重放、完整核验与别名恢复切换](../lessons/search/05-cutover-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/373-aliases-client.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

核验并切换 Elasticsearch 唯一写别名，区分仍在来源索引与已经到达目标的恢复事实。

ES 与 SQL 无法组成同一个事务；必须先确认真实 ES 绑定，再由原重建任务完成 SQL 代次，不能把 HTTP 200 当成全部完成。

写之前先按这条执行路线理解各段的连接：

1. ResolveAlias 拒绝多个目标、filter/routing及非本owner/schema索引，只接受唯一 is_write_index=true 的绑定。
2. EnsureInitialAlias 仅安装配置指定的初代绑定；已有不同绑定直接冲突，不擅自覆盖。
3. InspectAliasSwitch 验证来源、目标及目标所有权；SwitchAlias 使用同一次 _aliases 请求精确 remove+add，随后重新读取绑定。
4. validSwitchAcknowledgement 除顶层 acknowledged 外还核验可选 action_results 中两个原动作的目标及状态。

本文件需要守住的边界：

- TARGET 只代表 ES 事实，仍需原 SWITCHING 任务提交 SQL。
- 恢复核验不能创建缺失的空目标索引；超时/未知回执不得另选目标。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/client/elastic' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/client/elastic/aliases_client.go') -and (Get-Item -LiteralPath 'services/search/internal/client/elastic/aliases_client.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/client/elastic/aliases_client.go')) { New-Item -ItemType File -Path 'services/search/internal/client/elastic/aliases_client.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:222 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package elastic

import (
	"context"
	"encoding/json"
	"errors"
)

// alias.go：保留原实现，按业务合并文件。

````

## 第 2 步：Client.ResolveAlias

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

ResolveAlias拒绝多目标、额外filter/routing和未知owner/schema。
后台消费仍直接写解析得到的物理索引，不能让一次alias变化改写在途请求的目标。

本步声明：`func (c *Client) ResolveAlias(ctx context.Context, alias string) (string, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.request(ctx, &quot;GET&quot;, &quot;/_alias/&quot;+alias, nil, 1&lt;&lt;20)</code>；<code>c.verifyMapping(ctx, physical)</code> |
| <code>alias</code> | <code>string</code> | 稳定对外别名，必须为合法物理名形式且与source/target不同。<br>在此声明中的实际使用：<code>physicalName.MatchString(alias)</code>；<code>&quot;/_alias/&quot;+alias</code>；<code>metadata, exists := binding.Aliases&#91;alias&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 唯一绑定的物理索引名称；不存在时为空。<br>实际返回表达式：<code>&quot;&quot;</code>；<code>physical</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 是否找到了合法绑定；false且error=nil表示404未创建。<br>实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 网络故障、格式不符、绑定不唯一或owner/schema错误。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ALIAS_INVALID&quot;)</code>；<code>err</code>；<code>nil</code>；<code>errors.New(&quot;SEARCH_ALIAS_UNBOUND&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(alias)</code>。条件成立时执行 <code>return &quot;&quot;, false, errors.New(&quot;SEARCH_ALIAS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, false, err</code>。当前函数在这里结束。
- 检查 <code>status == 404</code>。条件成立时执行 <code>return &quot;&quot;, false, nil</code>。当前函数在这里结束。
- 检查 <code>status != 200 &#124;&#124; json.Unmarshal(data, &amp;bindings) != nil &#124;&#124; len(bindings) != 1</code>。条件成立时执行 <code>return &quot;&quot;, false, errors.New(&quot;SEARCH_ALIAS_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>!exists &#124;&#124; len(binding.Aliases) != 1 &#124;&#124; len(metadata) != 1 &#124;&#124; json.Unmarshal(metadata&#91;&quot;is_write_index&quot;&#93;, &amp;writable) != nil &#124;&#124; !writable &#124;&#124; !physicalName.MatchString(physical)</code>。条件成立时执行 <code>return &quot;&quot;, false, errors.New(&quot;SEARCH_ALIAS_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, false, err</code>。当前函数在这里结束。
- 检查 <code>!owned</code>。条件成立时执行 <code>return &quot;&quot;, false, errors.New(&quot;SEARCH_ALIAS_UNBOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return physical, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=222:1616 -->
````go
// ResolveAlias拒绝多目标、额外filter/routing和未知owner/schema。
// 后台消费仍直接写解析得到的物理索引，不能让一次alias变化改写在途请求的目标。
func (c *Client) ResolveAlias(ctx context.Context, alias string) (string, bool, error) {
	if !physicalName.MatchString(alias) {
		return "", false, errors.New("SEARCH_ALIAS_INVALID")
	}
	status, data, err := c.request(ctx, "GET", "/_alias/"+alias, nil, 1<<20)
	if err != nil {
		return "", false, err
	}
	if status == 404 {
		return "", false, nil
	}
	var bindings map[string]struct {
		Aliases map[string]map[string]json.RawMessage `json:"aliases"`
	}
	if status != 200 || json.Unmarshal(data, &bindings) != nil || len(bindings) != 1 {
		return "", false, errors.New("SEARCH_ALIAS_UNBOUND")
	}
	for physical, binding := range bindings {
		metadata, exists := binding.Aliases[alias]
		var writable bool
		if !exists || len(binding.Aliases) != 1 || len(metadata) != 1 || json.Unmarshal(metadata["is_write_index"], &writable) != nil || !writable || !physicalName.MatchString(physical) {
			return "", false, errors.New("SEARCH_ALIAS_UNBOUND")
		}
		owned, err := c.verifyMapping(ctx, physical)
		if err != nil {
			return "", false, err
		}
		if !owned {
			return "", false, errors.New("SEARCH_ALIAS_UNBOUND")
		}
		return physical, true, nil
	}
	return "", false, errors.New("SEARCH_ALIAS_UNBOUND")
}

````

## 第 3 步：Client.EnsureInitialAlias

插入位置：第 2 步代码之后，紧接追加；完成稿约第 49 行。

EnsureInitialAlias只创建配置中明确指定的初代绑定。重启时可核验同一绑定，
不能因SQL缺行就覆盖已指向其他索引的alias，也不能冒称ES和SQL原子提交。

本步声明：`func (c *Client) EnsureInitialAlias(ctx context.Context, alias, physical string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.ResolveAlias(ctx, alias)</code>；<code>c.EnsureIndex(ctx, physical)</code>；<code>c.request(ctx, &quot;POST&quot;, &quot;/_aliases?master_timeout=1s&amp;timeout=1s&quot;, map&#91;string&#93;any{ 		&quot;actions&quot;: &#91;&#93;any{map&#91;string&#93;any{&quot;add&quot;: map&#91;string&#93;any{&quot;index&quot;: physical, &quot;alias&quot;: alias, &quot;is_write_index&quot;: true}}}, 	}, 1&lt;&lt;20)</code> |
| <code>alias</code> | <code>string</code> | 稳定对外别名，必须为合法物理名形式且与source/target不同。<br>在此声明中的实际使用：<code>physicalName.MatchString(alias)</code>；<code>alias == physical</code>；<code>c.ResolveAlias(ctx, alias)</code>；<code>&quot;alias&quot;: alias</code> |
| <code>physical</code> | <code>string</code> | 配置或查询得到的实际索引名，不是别名。<br>在此声明中的实际使用：<code>physicalName.MatchString(physical)</code>；<code>alias == physical</code>；<code>actual != physical</code>；<code>c.EnsureIndex(ctx, physical)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ALIAS_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_INITIAL_ALIAS_CONFLICT&quot;)</code>；<code>nil</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(alias) &#124;&#124; !physicalName.MatchString(physical) &#124;&#124; alias == physical</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ALIAS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>exists</code>。
- 检查 <code>actual != physical</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_INITIAL_ALIAS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>status != 200 &#124;&#124; json.Unmarshal(data, &amp;response) != nil &#124;&#124; !response.Acknowledged &#124;&#124; response.Errors</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ALIAS_CREATE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!exists &#124;&#124; actual != physical</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ALIAS_CREATE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1616:3105 -->
````go
// EnsureInitialAlias只创建配置中明确指定的初代绑定。重启时可核验同一绑定，
// 不能因SQL缺行就覆盖已指向其他索引的alias，也不能冒称ES和SQL原子提交。
func (c *Client) EnsureInitialAlias(ctx context.Context, alias, physical string) error {
	if !physicalName.MatchString(alias) || !physicalName.MatchString(physical) || alias == physical {
		return errors.New("SEARCH_ALIAS_INVALID")
	}
	actual, exists, err := c.ResolveAlias(ctx, alias)
	if err != nil {
		return err
	}
	if exists {
		if actual != physical {
			return errors.New("SEARCH_INITIAL_ALIAS_CONFLICT")
		}
		return nil
	}
	if err := c.EnsureIndex(ctx, physical); err != nil {
		return err
	}
	status, data, err := c.request(ctx, "POST", "/_aliases?master_timeout=1s&timeout=1s", map[string]any{
		"actions": []any{map[string]any{"add": map[string]any{"index": physical, "alias": alias, "is_write_index": true}}},
	}, 1<<20)
	if err != nil {
		return err
	}
	var response struct {
		Acknowledged bool `json:"acknowledged"`
		Errors       bool `json:"errors"`
	}
	if status != 200 || json.Unmarshal(data, &response) != nil || !response.Acknowledged || response.Errors {
		return errors.New("SEARCH_ALIAS_CREATE_UNKNOWN")
	}
	actual, exists, err = c.ResolveAlias(ctx, alias)
	if err != nil {
		return err
	}
	if !exists || actual != physical {
		return errors.New("SEARCH_ALIAS_CREATE_UNKNOWN")
	}
	return nil
}

// alias_switch.go：保留原实现，按业务合并文件。

````

## 第 4 步：AliasSwitchState

插入位置：第 3 步代码之后，紧接追加；完成稿约第 93 行。

AliasSwitchState仅描述ES事实；AtTarget不代表SQL代次和原job已经提交。

本步声明：`type AliasSwitchState string`。

<!-- write-step: 4 bytes=3105:3225 -->
````go
// AliasSwitchState仅描述ES事实；AtTarget不代表SQL代次和原job已经提交。
type AliasSwitchState string

````

## 第 5 步：AliasAtSource、AliasAtTarget

插入位置：第 4 步代码之后，紧接追加；完成稿约第 96 行。

本步声明：`const (`。

<!-- write-step: 5 bytes=3225:3322 -->
````go
const (
	AliasAtSource AliasSwitchState = "SOURCE"
	AliasAtTarget AliasSwitchState = "TARGET"
)

````

## 第 6 步：Client.InspectAliasSwitch

插入位置：第 5 步代码之后，紧接追加；完成稿约第 101 行。

InspectAliasSwitch供持久SWITCHING恢复使用。调用方须核验原job的封存验证事实，
再按SOURCE重试原切换或按TARGET完成原SQL CAS，不能另选目标或解除屏障。

本步声明：`func (c *Client) InspectAliasSwitch(ctx context.Context, alias, source, target string) (AliasSwitchState, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.ResolveAlias(ctx, alias)</code>；<code>c.verifyMapping(ctx, target)</code> |
| <code>alias</code> | <code>string</code> | 稳定对外别名，必须为合法物理名形式且与source/target不同。<br>在此声明中的实际使用：<code>physicalName.MatchString(alias)</code>；<code>alias == source</code>；<code>alias == target</code>；<code>c.ResolveAlias(ctx, alias)</code> |
| <code>source</code> | <code>string</code> | 原准备中固定的来源物理索引。<br>在此声明中的实际使用：<code>physicalName.MatchString(source)</code>；<code>alias == source</code>；<code>source == target</code>；<code>actual != source</code> |
| <code>target</code> | <code>string</code> | 原准备中固定的目标物理索引，已构建并属于同一owner。<br>在此声明中的实际使用：<code>physicalName.MatchString(target)</code>；<code>alias == target</code>；<code>source == target</code>；<code>actual != target</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AliasSwitchState</code> | 使用 <code>AliasSwitchState</code> 类型；其定义处的职责：AliasSwitchState仅描述ES事实；AtTarget不代表SQL代次和原job已经提交。<br>实际返回表达式：<code>&quot;&quot;</code>；<code>AliasAtTarget</code>；<code>AliasAtSource</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ALIAS_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_ALIAS_SWITCH_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(alias) &#124;&#124; !physicalName.MatchString(source) &#124;&#124; !physicalName.MatchString(target) &#124;&#124; 		alias == source &#124;&#124; alias == target &#124;&#124; source == target</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;SEARCH_ALIAS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>!exists &#124;&#124; (actual != source &amp;&amp; actual != target)</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;SEARCH_ALIAS_SWITCH_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>!owned</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;SEARCH_ALIAS_SWITCH_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>actual == target</code>。条件成立时执行 <code>return AliasAtTarget, nil</code>。当前函数在这里结束。
- 此处通过 <code>return AliasAtSource, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3322:4478 -->
````go
// InspectAliasSwitch供持久SWITCHING恢复使用。调用方须核验原job的封存验证事实，
// 再按SOURCE重试原切换或按TARGET完成原SQL CAS，不能另选目标或解除屏障。
func (c *Client) InspectAliasSwitch(ctx context.Context, alias, source, target string) (AliasSwitchState, error) {
	if !physicalName.MatchString(alias) || !physicalName.MatchString(source) || !physicalName.MatchString(target) ||
		alias == source || alias == target || source == target {
		return "", errors.New("SEARCH_ALIAS_INVALID")
	}
	actual, exists, err := c.ResolveAlias(ctx, alias)
	if err != nil {
		return "", err
	}
	if !exists || (actual != source && actual != target) {
		return "", errors.New("SEARCH_ALIAS_SWITCH_CONFLICT")
	}
	// 即使当前alias仍在source，也必须确认目标是已经构建的本owner索引；
	// 恢复检查不创建缺失索引，避免把空索引当作已验证目标。
	owned, err := c.verifyMapping(ctx, target)
	if err != nil {
		return "", err
	}
	if !owned {
		return "", errors.New("SEARCH_ALIAS_SWITCH_CONFLICT")
	}
	if actual == target {
		return AliasAtTarget, nil
	}
	return AliasAtSource, nil
}

````

## 第 7 步：Client.SwitchAlias

插入位置：第 6 步代码之后，紧接追加；完成稿约第 130 行。

SwitchAlias只执行一次精确切换。调用方必须已持久化SWITCHING屏障及完整验证。
ES与SQL不构成分布式事务：请求/回执未知直接返回错误，留给原job重读恢复。

本步声明：`func (c *Client) SwitchAlias(ctx context.Context, alias, source, target string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.InspectAliasSwitch(ctx, alias, source, target)</code>；<code>c.request(ctx, &quot;POST&quot;, &quot;/_aliases?master_timeout=1s&amp;timeout=1s&quot;, map&#91;string&#93;any{ 		&quot;actions&quot;: &#91;&#93;any{ 			map&#91;string&#93;any{&quot;remove&quot;: map&#91;string&#93;any{&quot;index&quot;: source, &quot;alias&quot;: alias, &quot;mu …</code>（长表达式，按本步完整代码继续）；<code>c.ResolveAlias(ctx, alias)</code> |
| <code>alias</code> | <code>string</code> | 稳定对外别名，必须为合法物理名形式且与source/target不同。<br>在此声明中的实际使用：<code>c.InspectAliasSwitch(ctx, alias, source, target)</code>；<code>&quot;alias&quot;: alias</code>；<code>validSwitchAcknowledgement(data, alias, source, target)</code>；<code>c.ResolveAlias(ctx, alias)</code> |
| <code>source</code> | <code>string</code> | 原准备中固定的来源物理索引。<br>在此声明中的实际使用：<code>c.InspectAliasSwitch(ctx, alias, source, target)</code>；<code>&quot;index&quot;: source</code>；<code>validSwitchAcknowledgement(data, alias, source, target)</code> |
| <code>target</code> | <code>string</code> | 原准备中固定的目标物理索引，已构建并属于同一owner。<br>在此声明中的实际使用：<code>c.InspectAliasSwitch(ctx, alias, source, target)</code>；<code>&quot;index&quot;: target</code>；<code>validSwitchAcknowledgement(data, alias, source, target)</code>；<code>actual != target</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_ALIAS_SWITCH_RECONCILE_REQUIRED&quot;)</code>；<code>errors.New(&quot;SEARCH_ALIAS_SWITCH_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state != AliasAtSource</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ALIAS_SWITCH_RECONCILE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>status != 200 &#124;&#124; !validSwitchAcknowledgement(data, alias, source, target)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ALIAS_SWITCH_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!exists &#124;&#124; actual != target</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ALIAS_SWITCH_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4478:5658 -->
````go
// SwitchAlias只执行一次精确切换。调用方必须已持久化SWITCHING屏障及完整验证。
// ES与SQL不构成分布式事务：请求/回执未知直接返回错误，留给原job重读恢复。
func (c *Client) SwitchAlias(ctx context.Context, alias, source, target string) error {
	state, err := c.InspectAliasSwitch(ctx, alias, source, target)
	if err != nil {
		return err
	}
	if state != AliasAtSource {
		return errors.New("SEARCH_ALIAS_SWITCH_RECONCILE_REQUIRED")
	}
	status, data, err := c.request(ctx, "POST", "/_aliases?master_timeout=1s&timeout=1s", map[string]any{
		"actions": []any{
			map[string]any{"remove": map[string]any{"index": source, "alias": alias, "must_exist": true}},
			map[string]any{"add": map[string]any{"index": target, "alias": alias, "is_write_index": true}},
		},
	}, 1<<20)
	if err != nil {
		return err
	}
	if status != 200 || !validSwitchAcknowledgement(data, alias, source, target) {
		return errors.New("SEARCH_ALIAS_SWITCH_UNKNOWN")
	}
	actual, exists, err := c.ResolveAlias(ctx, alias)
	if err != nil {
		return err
	}
	if !exists || actual != target {
		return errors.New("SEARCH_ALIAS_SWITCH_UNKNOWN")
	}
	return nil
}

````

## 第 8 步：validSwitchAcknowledgement

插入位置：第 7 步代码之后，紧接追加；完成稿约第 162 行。

本步声明：`func validSwitchAcknowledgement(data []byte, alias, source, target string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>data</code> | <code>&#91;&#93;byte</code> | ES响应原始JSON字节，必须核验实际动作而不只检查HTTP状态。<br>在此声明中的实际使用：<code>json.Unmarshal(data, &amp;reply)</code> |
| <code>alias</code> | <code>string</code> | 稳定对外别名，必须为合法物理名形式且与source/target不同。<br>在此声明中的实际使用：<code>result.Action.Aliases&#91;0&#93; != alias</code> |
| <code>source</code> | <code>string</code> | 原准备中固定的来源物理索引。<br>在此声明中的实际使用：<code>kind, physical := &quot;remove&quot;, source</code> |
| <code>target</code> | <code>string</code> | 原准备中固定的目标物理索引，已构建并属于同一owner。<br>在此声明中的实际使用：<code>kind, physical = &quot;add&quot;, target</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(data, &amp;reply) != nil &#124;&#124; reply.Acknowledged == nil &#124;&#124; !*reply.Acknowledged &#124;&#124; reply.Errors &#124;&#124; len(reply.Error) != 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>len(reply.Results) == 0</code>。条件成立时执行 <code>return true</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(reply.Results, &amp;results) != nil &#124;&#124; len(results) != 2</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>i == 1</code>。
- 检查 <code>result.Status != 200 &#124;&#124; len(result.Error) != 0 &#124;&#124; result.Action.Type != kind &#124;&#124; 			len(result.Action.Indices) != 1 &#124;&#124; result.Action.Indices&#91;0&#93; != physical &#124;&#124; 			len(result.Action.A …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=5658:7105 -->
````go
func validSwitchAcknowledgement(data []byte, alias, source, target string) bool {
	var reply struct {
		Acknowledged *bool           `json:"acknowledged"`
		Errors       bool            `json:"errors"`
		Error        json.RawMessage `json:"error"`
		Results      json.RawMessage `json:"action_results"`
	}
	if json.Unmarshal(data, &reply) != nil || reply.Acknowledged == nil || !*reply.Acknowledged || reply.Errors || len(reply.Error) != 0 {
		return false
	}
	// ES成功时允许省略action_results；一旦返回逐动作结果就必须完整对应两个原动作，
	// 不能只看HTTP 200或顶层errors=false而忽略部分失败或错误目标。
	if len(reply.Results) == 0 {
		return true
	}
	var results []struct {
		Action struct {
			Type    string   `json:"type"`
			Indices []string `json:"indices"`
			Aliases []string `json:"aliases"`
		} `json:"action"`
		Status int             `json:"status"`
		Error  json.RawMessage `json:"error"`
	}
	if json.Unmarshal(reply.Results, &results) != nil || len(results) != 2 {
		return false
	}
	for i, result := range results {
		kind, physical := "remove", source
		if i == 1 {
			kind, physical = "add", target
		}
		if result.Status != 200 || len(result.Error) != 0 || result.Action.Type != kind ||
			len(result.Action.Indices) != 1 || result.Action.Indices[0] != physical ||
			len(result.Action.Aliases) != 1 || result.Action.Aliases[0] != alias {
			return false
		}
	}
	return true
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/client/elastic/aliases_client.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/05-cutover-wiring.md)。
