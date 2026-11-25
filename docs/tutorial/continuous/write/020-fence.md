# 分步编写 `packages/platform/fence.go`

先按 [F04 · 原操作、资格来源与跨服务身份](../lessons/foundation/04-original-operation.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/020-fence.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义来源关闭范围、目标摘要、参与者ACK比对和空底座的Fence实现。

确认必须绑定原撤销号、scope、旧epoch、协议generation和参与者角色，避免拿别的关闭回执替代。

写之前先按这条执行路线理解各段的连接：

1. ValidFence区分Account来源和doc/kb/publication来源；BootstrapDigest把所有绑定及role纳入SHA256。
2. MatchesBootstrapAck要求ACKNOWLEDGED、同范围、同generation、正确摘要且无MissingInstances。
3. PrepareBootstrapFence仅在admission/delivery/output全空时可采用底座路径，Finish保存真实collaboration ACK。

本文件需要守住的边界：

- 已有业务输出必须用真实排空器，不能空ACK。
- 协议generation固定1，不等于worker租约代次。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/fence.go') -and (Get-Item -LiteralPath 'packages/platform/fence.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/fence.go')) { New-Item -ItemType File -Path 'packages/platform/fence.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:254 -->
````go
package platform

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"regexp"
	"strconv"
	"strings"
)

````

## 第 2 步：fenceID

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`var fenceID = regexp.MustCompile(\`^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$\`)`。

<!-- write-step: 2 bytes=254:324 -->
````go
var fenceID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$`)
````

## 第 3 步：sessionScope

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`var sessionScope = regexp.MustCompile(\`^session:[a-f0-9]{64}$\`)`。

<!-- write-step: 3 bytes=324:388 -->
````go
var sessionScope = regexp.MustCompile(`^session:[a-f0-9]{64}$`)
````

## 第 4 步：credentialScope

插入位置：第 3 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`var credentialScope = regexp.MustCompile(\`^credential:[1-9][0-9]{0,19}$\`)`。

<!-- write-step: 4 bytes=388:463 -->
````go
var credentialScope = regexp.MustCompile(`^credential:[1-9][0-9]{0,19}$`)

````

## 第 5 步：ValidAccountOutputScope

插入位置：第 4 步代码之后，紧接追加；完成稿约第 22 行。

输出依赖包含会话哈希；组织/账号变更来源本身沿用ValidAccountScope的十进制身份规则。

本步声明：`func ValidAccountOutputScope(key string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>sessionScope.MatchString(key)</code>；<code>ValidAccountScope(key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>sessionScope.MatchString(key) &#124;&#124; ValidAccountScope(key)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return sessionScope.MatchString(key) &#124;&#124; ValidAccountScope(key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=463:696 -->
````go
// 输出依赖包含会话哈希；组织/账号变更来源本身沿用ValidAccountScope的十进制身份规则。
func ValidAccountOutputScope(key string) bool {
	return sessionScope.MatchString(key) || ValidAccountScope(key)
}

````

## 第 6 步：ValidAccountScope

插入位置：第 5 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func ValidAccountScope(key string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>strings.Split(key, &quot;:&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>len(parts) != count+1</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; value == 0 &#124;&#124; strconv.FormatUint(value, 10) != part</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=696:1205 -->
````go
func ValidAccountScope(key string) bool {
	parts := strings.Split(key, ":")
	count := 0
	switch parts[0] {
	case "identity", "credential", "platform-admin":
		count = 1
	case "org-member", "org-admin", "org-owner", "org-create":
		count = 2
	default:
		return false
	}
	if len(parts) != count+1 {
		return false
	}
	for _, part := range parts[1:] {
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil || value == 0 || strconv.FormatUint(value, 10) != part {
			return false
		}
	}
	return true
}

````

## 第 7 步：ValidDocumentScope

插入位置：第 6 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`func ValidDocumentScope(key string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>strings.Split(key, &quot;:&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil &amp;&amp; id &gt; 0 &amp;&amp; strconv.FormatUint(id, 10) == parts&#91;1&#93;</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(parts) != 2 &#124;&#124; (parts&#91;0&#93; != &quot;doc-acl&quot; &amp;&amp; parts&#91;0&#93; != &quot;kb-acl&quot; &amp;&amp; parts&#91;0&#93; != &quot;publication&quot;)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil &amp;&amp; id &gt; 0 &amp;&amp; strconv.FormatUint(id, 10) == parts&#91;1&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1205:1614 -->
````go
func ValidDocumentScope(key string) bool {
	parts := strings.Split(key, ":")
	// 正式版切换由Doc关闭旧publication水位；它不是账号身份准入来源。
	if len(parts) != 2 || (parts[0] != "doc-acl" && parts[0] != "kb-acl" && parts[0] != "publication") {
		return false
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	return err == nil && id > 0 && strconv.FormatUint(id, 10) == parts[1]
}
````

## 第 8 步：ValidFence

插入位置：第 7 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func ValidFence(req *pb.FenceRequest) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>req</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>ValidBootstrapFence(req)</code>；<code>req != nil</code>；<code>req.RevocationId</code>；<code>req.Generation</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>true</code>；<code>req != nil &amp;&amp; fenceID.MatchString(req.RevocationId) &amp;&amp; req.Generation == 1 &amp;&amp; len(req.ClosedScopes) == 1 &amp;&amp; req.ClosedScopes&#91;0&#93; != nil &amp;&amp; req.ClosedScopes&#91;0&#93;.Epoch &gt; 0 &amp;&amp; ValidDocu …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>ValidBootstrapFence(req)</code>。条件成立时执行 <code>return true</code>。当前函数在这里结束。
- 此处通过 <code>return req != nil &amp;&amp; fenceID.MatchString(req.RevocationId) &amp;&amp; req.Generation == 1 &amp;&amp; len(req.ClosedScopes) == 1 &amp;&amp; req.ClosedScopes&#91;0&#93; != nil &amp;&amp; req.ClosedScopes&#91;0&#93;.Epoch &gt; 0 &amp;&amp; Va …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1614:2049 -->
````go
func ValidFence(req *pb.FenceRequest) bool {
	if ValidBootstrapFence(req) {
		return true
	}
	return req != nil && fenceID.MatchString(req.RevocationId) && req.Generation == 1 && len(req.ClosedScopes) == 1 && req.ClosedScopes[0] != nil && req.ClosedScopes[0].Epoch > 0 && ValidDocumentScope(req.ClosedScopes[0].ScopeKey) && bytes.Equal(req.TargetDigest, SourceClosureDigest(req.ClosedScopes[0].ScopeKey, req.ClosedScopes[0].Epoch))
}

````

## 第 9 步：SourceClosureDigest

插入位置：第 8 步代码之后，紧接追加；完成稿约第 66 行。

本步声明：`func SourceClosureDigest(scope string, epoch uint64) []byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;AUTHORIZATION_CHANGE\n&quot; + scope</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>strconv.FormatUint(epoch, 10)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>value&#91;:&#93;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return value&#91;:&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2049:2236 -->
````go
func SourceClosureDigest(scope string, epoch uint64) []byte {
	value := sha256.Sum256([]byte("AUTHORIZATION_CHANGE\n" + scope + "\n" + strconv.FormatUint(epoch, 10)))
	return value[:]
}

````

## 第 10 步：LogoutTargetDigest

插入位置：第 9 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func LogoutTargetDigest(hash string) []byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>hash</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;SESSION_LOGOUT\n&quot; + hash</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>value&#91;:&#93;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return value&#91;:&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2236:2360 -->
````go
func LogoutTargetDigest(hash string) []byte {
	value := sha256.Sum256([]byte("SESSION_LOGOUT\n" + hash))
	return value[:]
}
````

## 第 11 步：ValidBootstrapFence

插入位置：第 10 步代码之后，紧接追加；完成稿约第 75 行。

本步声明：`func ValidBootstrapFence(req *pb.FenceRequest) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>req</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>req == nil</code>；<code>req.RevocationId</code>；<code>req.Generation</code>；<code>req.ClosedScopes</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>bytes.Equal(req.TargetDigest, LogoutTargetDigest(s.ScopeKey&#91;len(&quot;session:&quot;):&#93;))</code>；<code>bytes.Equal(req.TargetDigest, SourceClosureDigest(s.ScopeKey, s.Epoch))</code> |

**跟着本段代码走一遍：**

- 检查 <code>req == nil &#124;&#124; !fenceID.MatchString(req.RevocationId) &#124;&#124; req.Generation != 1 &#124;&#124; len(req.ClosedScopes) != 1 &#124;&#124; req.ClosedScopes&#91;0&#93; == nil &#124;&#124; req.ClosedScopes&#91;0&#93;.Epoch == 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>sessionScope.MatchString(s.ScopeKey)</code>。条件成立时执行 <code>return bytes.Equal(req.TargetDigest, LogoutTargetDigest(s.ScopeKey&#91;len(&quot;session:&quot;):&#93;))</code>。当前函数在这里结束。
- 检查 <code>ValidAccountScope(s.ScopeKey)</code>。条件成立时执行 <code>return bytes.Equal(req.TargetDigest, SourceClosureDigest(s.ScopeKey, s.Epoch))</code>。当前函数在这里结束。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=2360:2905 -->
````go
func ValidBootstrapFence(req *pb.FenceRequest) bool {
	if req == nil || !fenceID.MatchString(req.RevocationId) || req.Generation != 1 || len(req.ClosedScopes) != 1 || req.ClosedScopes[0] == nil || req.ClosedScopes[0].Epoch == 0 {
		return false
	}
	s := req.ClosedScopes[0]
	if sessionScope.MatchString(s.ScopeKey) {
		return bytes.Equal(req.TargetDigest, LogoutTargetDigest(s.ScopeKey[len("session:"):]))
	}
	if ValidAccountScope(s.ScopeKey) {
		return bytes.Equal(req.TargetDigest, SourceClosureDigest(s.ScopeKey, s.Epoch))
	}
	return false
}
````

## 第 12 步：BootstrapDigest

插入位置：第 11 步代码之后，紧接追加；完成稿约第 88 行。

本步声明：`func BootstrapDigest(req *pb.FenceRequest, role string) []byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>req</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>req.ClosedScopes</code>；<code>req.RevocationId</code>；<code>req.Generation</code>；<code>req.TargetDigest</code> |
| <code>role</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;bootstrap-fence-v1\n&quot; + role</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>value&#91;:&#93;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return value&#91;:&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=2905:3259 -->
````go
func BootstrapDigest(req *pb.FenceRequest, role string) []byte {
	s := req.ClosedScopes[0]
	value := sha256.Sum256([]byte("bootstrap-fence-v1\n" + role + "\n" + req.RevocationId + "\n" + s.ScopeKey + "\n" + strconv.FormatUint(s.Epoch, 10) + "\n" + strconv.FormatUint(req.Generation, 10) + "\n" + hex.EncodeToString(req.TargetDigest)))
	return value[:]
}
````

## 第 13 步：BootstrapAck

插入位置：第 12 步代码之后，紧接追加；完成稿约第 93 行。

本步声明：`func BootstrapAck(req *pb.FenceRequest, role string) *pb.FenceResult`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>req</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>req.RevocationId</code>；<code>req.ClosedScopes</code>；<code>req.Generation</code>；<code>BootstrapDigest(req, role)</code> |
| <code>role</code> | <code>string</code> | 在此声明中的实际使用：<code>BootstrapDigest(req, role)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceResult</code> | 实际返回表达式：<code>&amp;pb.FenceResult{State: pb.FenceResult_ACKNOWLEDGED, RevocationId: req.RevocationId, ClosedScopes: req.ClosedScopes, Generation: req.Generation, ResultDigest: BootstrapDigest(req, role)}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;pb.FenceResult{State: pb.FenceResult_ACKNOWLEDGED, RevocationId: req.RevocationId, ClosedScopes: req.ClosedScopes, Generation: req.Generation, ResultDigest: BootstrapDigest(req, role)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=3259:3526 -->
````go
func BootstrapAck(req *pb.FenceRequest, role string) *pb.FenceResult {
	return &pb.FenceResult{State: pb.FenceResult_ACKNOWLEDGED, RevocationId: req.RevocationId, ClosedScopes: req.ClosedScopes, Generation: req.Generation, ResultDigest: BootstrapDigest(req, role)}
}
````

## 第 14 步：MatchesBootstrapAck

插入位置：第 13 步代码之后，紧接追加；完成稿约第 96 行。

本步声明：`func MatchesBootstrapAck(req *pb.FenceRequest, reply *pb.FenceResult, role string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>req</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>ValidFence(req)</code>；<code>req.RevocationId</code>；<code>req.Generation</code>；<code>req.ClosedScopes</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>reply</code> | <code>*pb.FenceResult</code> | 在此声明中的实际使用：<code>reply != nil</code>；<code>reply.State</code>；<code>reply.RevocationId</code>；<code>reply.Generation</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>role</code> | <code>string</code> | 在此声明中的实际使用：<code>BootstrapDigest(req, role)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>ValidFence(req) &amp;&amp; reply != nil &amp;&amp; reply.State == pb.FenceResult_ACKNOWLEDGED &amp;&amp; reply.RevocationId == req.RevocationId &amp;&amp; reply.Generation == req.Generation &amp;&amp; len(reply.ClosedSco …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return ValidFence(req) &amp;&amp; reply != nil &amp;&amp; reply.State == pb.FenceResult_ACKNOWLEDGED &amp;&amp; reply.RevocationId == req.RevocationId &amp;&amp; reply.Generation == req.Generation &amp;&amp; len(reply.Cl …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=3526:4074 -->
````go
func MatchesBootstrapAck(req *pb.FenceRequest, reply *pb.FenceResult, role string) bool {
	return ValidFence(req) && reply != nil && reply.State == pb.FenceResult_ACKNOWLEDGED && reply.RevocationId == req.RevocationId && reply.Generation == req.Generation && len(reply.ClosedScopes) == 1 && reply.ClosedScopes[0] != nil && reply.ClosedScopes[0].ScopeKey == req.ClosedScopes[0].ScopeKey && reply.ClosedScopes[0].Epoch == req.ClosedScopes[0].Epoch && bytes.Equal(reply.ResultDigest, BootstrapDigest(req, role)) && len(reply.MissingInstances) == 0
}

````

## 第 15 步：bootstrapBinding

插入位置：第 14 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`type bootstrapBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Digest</code> | <code>string</code> | 原FenceRequest.TargetDigest的hex，重试必须一致。<br>序列化/校验标记：<code>`json:&quot;bootstrap_target_digest&quot;`</code> |
| <code>Generation</code> | <code>string</code> | 原协议generation的十进制字符串，当前固定为1。<br>序列化/校验标记：<code>`json:&quot;generation&quot;`</code> |

<!-- write-step: 15 bytes=4074:4199 -->
````go
type bootstrapBinding struct {
	Digest     string `json:"bootstrap_target_digest"`
	Generation string `json:"generation"`
}

````

## 第 16 步：PrepareBootstrapFence

插入位置：第 15 步代码之后，紧接追加；完成稿约第 105 行。

PrepareBootstrapFence只支持尚无业务输出的底座。后续引入admission/output时必须换真正排空器。

本步声明：`func PrepareBootstrapFence(ctx context.Context, store *Store, req *pb.FenceRequest) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>store.DB.BeginTx(ctx, nil)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE&quot;, req.RevocationId)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM &quot;+table)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;\&quot;collaboration\&quot;&#93;&#x27;)&quot;, req. …</code>（长表达式，按本步完整代码继续）<br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>store</code> | <code>*Store</code> | 使用 <code>Store</code> 类型；其定义处的职责：加载服务自身配置，创建有界数据库池，并以迁移和恢复事实判定就绪。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>req</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>ValidBootstrapFence(req)</code>；<code>req.ClosedScopes</code>；<code>req.RevocationId</code>；<code>req.TargetDigest</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>status.Error(codes.InvalidArgument, &quot;BOOTSTRAP_FENCE_INVALID&quot;)</code>；<code>status.Error(codes.Unavailable, &quot;FENCE_UNAVAILABLE&quot;)</code>；<code>status.Error(codes.AlreadyExists, &quot;FENCE_ID_CONFLICT&quot;)</code>；<code>nil</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!ValidBootstrapFence(req)</code>。条件成立时执行 <code>return false, status.Error(codes.InvalidArgument, &quot;BOOTSTRAP_FENCE_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>store.DB.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, status.Error(codes.Unavailable, &quot;FENCE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE&quot;, req.RevocationId).Scan(&amp;previousScope, &amp;epoch, &amp;state, &amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE&quot;, req.RevocationId)</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>json.Unmarshal(raw, &amp;prior) != nil &#124;&#124; prior != binding &#124;&#124; previousScope != scope.ScopeKey &#124;&#124; epoch != scope.Epoch</code>。条件成立时执行 <code>return false, status.Error(codes.AlreadyExists, &quot;FENCE_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>state == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return true, nil</code>。当前函数在这里结束。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return false, status.Error(codes.Unavailable, &quot;FENCE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM &quot;+table).Scan(&amp;count) != nil</code>。条件成立时执行 <code>return false, status.Error(codes.Unavailable, &quot;FENCE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM &quot;+table).Scan(&amp;count)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM &quot;+table)</code>。
- 检查 <code>count != 0</code>。条件成立时执行 <code>return false, status.Error(codes.FailedPrecondition, &quot;FULL_OUTPUT_DRAIN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;\&quot;collaboration\&quot;&#93;&#x27;)&quot;, req. …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, status.Error(codes.Unavailable, &quot;FENCE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_e …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 16 bytes=4199:6827 -->
````go
// PrepareBootstrapFence只支持尚无业务输出的底座。后续引入admission/output时必须换真正排空器。
func PrepareBootstrapFence(ctx context.Context, store *Store, req *pb.FenceRequest) (bool, error) {
	if !ValidBootstrapFence(req) {
		return false, status.Error(codes.InvalidArgument, "BOOTSTRAP_FENCE_INVALID")
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
	}
	defer tx.Rollback()
	scope := req.ClosedScopes[0]
	var previousScope, state string
	var epoch uint64
	var raw []byte
	err = tx.QueryRowContext(ctx, "SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE", req.RevocationId).Scan(&previousScope, &epoch, &state, &raw)
	binding := bootstrapBinding{Digest: hex.EncodeToString(req.TargetDigest), Generation: strconv.FormatUint(req.Generation, 10)}
	if err == nil {
		var prior bootstrapBinding
		if json.Unmarshal(raw, &prior) != nil || prior != binding || previousScope != scope.ScopeKey || epoch != scope.Epoch {
			return false, status.Error(codes.AlreadyExists, "FENCE_ID_CONFLICT")
		}
		if state == "SUCCEEDED" {
			return true, nil
		}
	} else if err != sql.ErrNoRows {
		return false, status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
	}
	// 没有正文/下载handler也没有输出登记，才能证明无需排空。非空记录绝不空ACK。
	for _, table := range []string{"document_admission", "delivery_session", "output_instance"} {
		var count int
		if tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count) != nil {
			return false, status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
		}
		if count != 0 {
			return false, status.Error(codes.FailedPrecondition, "FULL_OUTPUT_DRAIN_REQUIRED")
		}
	}
	if err == sql.ErrNoRows {
		encoded, _ := json.Marshal(binding)
		_, err = tx.ExecContext(ctx, "INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,'FENCING','[\"collaboration\"]')", req.RevocationId, scope.ScopeKey, scope.Epoch, string(encoded))
		if err != nil {
			return false, status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
		}
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state=IF(VALUES(max_epoch)>=max_epoch,'BLOCKED',state),last_revocation_id=IF(VALUES(max_epoch)>=max_epoch,VALUES(last_revocation_id),last_revocation_id),revision=revision+1", scope.ScopeKey, scope.Epoch, req.RevocationId)
````

## 第 17 步：PrepareBootstrapFence（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 150 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; tx.Commit() != nil</code>。条件成立时执行 <code>return false, status.Error(codes.Unavailable, &quot;FENCE_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return false, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=6827:6962 -->
````go
	if err != nil || tx.Commit() != nil {
		return false, status.Error(codes.Unavailable, "FENCE_COMMIT_UNKNOWN")
	}
	return false, nil
}
````

## 第 18 步：FinishBootstrapFence

插入位置：第 17 步代码之后，紧接追加；完成稿约第 155 行。

本步声明：`func FinishBootstrapFence(ctx context.Context, store *Store, req *pb.FenceRequest, node *pb.FenceResult) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>store.DB.BeginTx(ctx, nil)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;collaboration&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_T …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE authorization_change SET state=&#x27;SUCCEEDED&#x27; WHERE revocation_id=?&quot;, req.RevocationId)</code> |
| <code>store</code> | <code>*Store</code> | 使用 <code>Store</code> 类型；其定义处的职责：加载服务自身配置，创建有界数据库池，并以迁移和恢复事实判定就绪。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>req</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>MatchesBootstrapAck(req, node, &quot;collaboration&quot;)</code>；<code>req.ClosedScopes</code>；<code>req.RevocationId</code> |
| <code>node</code> | <code>*pb.FenceResult</code> | 在此声明中的实际使用：<code>MatchesBootstrapAck(req, node, &quot;collaboration&quot;)</code>；<code>node.ResultDigest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>status.Error(codes.Unavailable, &quot;NODE_FENCE_NOT_CONFIRMED&quot;)</code>；<code>status.Error(codes.Unavailable, &quot;FENCE_UNAVAILABLE&quot;)</code>；<code>status.Error(codes.Unavailable, &quot;FENCE_COMMIT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!MatchesBootstrapAck(req, node, &quot;collaboration&quot;)</code>。条件成立时执行 <code>return status.Error(codes.Unavailable, &quot;NODE_FENCE_NOT_CONFIRMED&quot;)</code>。当前函数在这里结束。
- 执行 <code>store.DB.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return status.Error(codes.Unavailable, &quot;FENCE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;collaboration&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_T …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return status.Error(codes.Unavailable, &quot;FENCE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE authorization_change SET state=&#x27;SUCCEEDED&#x27; WHERE revocation_id=?&quot;, req.RevocationId)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil &#124;&#124; tx.Commit() != nil</code>。条件成立时执行 <code>return status.Error(codes.Unavailable, &quot;FENCE_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=6962:8031 -->
````go
func FinishBootstrapFence(ctx context.Context, store *Store, req *pb.FenceRequest, node *pb.FenceResult) error {
	if !MatchesBootstrapAck(req, node, "collaboration") {
		return status.Error(codes.Unavailable, "NODE_FENCE_NOT_CONFIRMED")
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
	}
	defer tx.Rollback()
	scope := req.ClosedScopes[0]
	_, err = tx.ExecContext(ctx, "INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,'collaboration',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE result_digest=VALUES(result_digest)", req.RevocationId, scope.ScopeKey, scope.Epoch, node.ResultDigest)
	if err != nil {
		return status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
	}
	_, err = tx.ExecContext(ctx, "UPDATE authorization_change SET state='SUCCEEDED' WHERE revocation_id=?", req.RevocationId)
	if err != nil || tx.Commit() != nil {
		return status.Error(codes.Unavailable, "FENCE_COMMIT_UNKNOWN")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/fence.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/04-original-operation.md)。
