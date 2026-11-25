# 分步编写 `services/search/internal/domain/cursor/digest.go`

先按 [S03 · 签名批锚点、PIT与当前权限二次过滤](../lessons/search/03-candidates-cursor.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/367-digest.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

为完整有序候选批和分页边界计算稳定摘要。

重放时PIT句柄和调用nonce可能变化，但返回候选身份/顺序必须与原批完全相同。

写之前先按这条执行路线理解各段的连接：

1. validateDigestCandidates核候选doc/release/seq/physical、sort绑定doc且无重复
2. 要求所有候选同物理索引，after等于最后sort
3. DigestCandidates写用途标签、长度前缀、before/after、count/more，再按序写每候选身份与原sort

本文件需要守住的边界：

- 不能只hash文档ID而遗漏release/sequence/排序位置
- 摘要不包含PIT/nonce/时间，否则合法重放无法匹配

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/domain/cursor' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/domain/cursor/digest.go') -and (Get-Item -LiteralPath 'services/search/internal/domain/cursor/digest.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/domain/cursor/digest.go')) { New-Item -ItemType File -Path 'services/search/internal/domain/cursor/digest.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:170 -->
````go
package cursor

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash"
	"regexp"
	"strconv"
)

````

## 第 2 步：candidateDigestPurpose

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`const candidateDigestPurpose = "search-candidates-batch-digest-v1\x00"`。

<!-- write-step: 2 bytes=170:242 -->
````go
const candidateDigestPurpose = "search-candidates-batch-digest-v1\x00"

````

## 第 3 步：candidatePhysical

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`var candidatePhysical = regexp.MustCompile(\`^[a-z0-9][a-z0-9._-]{0,159}$\`)`。

<!-- write-step: 3 bytes=242:318 -->
````go
var candidatePhysical = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)

````

## 第 4 步：digestField

插入位置：第 3 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func digestField(digest hash.Hash, value []byte)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>digest</code> | <code>hash.Hash</code> | 在此声明中的实际使用：<code>digest.Write</code> |
| <code>value</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(value)</code>；<code>digest.Write(value)</code> |

<!-- write-step: 4 bytes=318:497 -->
````go
func digestField(digest hash.Hash, value []byte) {
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(value)))
	digest.Write(length[:])
	digest.Write(value)
}

````

## 第 5 步：validateDigestCandidates

插入位置：第 4 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func validateDigestCandidates(before, after json.RawMessage, candidates []*pb.SearchCandidate, more bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>before</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>在此声明中的实际使用：<code>Before: before</code> |
| <code>after</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>在此声明中的实际使用：<code>After: after</code>；<code>bytes.Equal(after, candidates&#91;len(candidates)-1&#93;.SortJsonUtf8)</code> |
| <code>candidates</code> | <code>&#91;&#93;*pb.SearchCandidate</code> | 在此声明中的实际使用：<code>len(candidates)</code>；<code>candidates&#91;len(candidates)-1&#93;.SortJsonUtf8</code> |
| <code>more</code> | <code>bool</code> | 在此声明中的实际使用：<code>More: more</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(candidates) &gt; 100 &#124;&#124; validateBatch(Batch{Chain: &#91;32&#93;byte{1}, Before: before, After: after, 		Count: uint8(len(candidates)), More: more, Digest: &#91;32&#93;byte{1}}) != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>candidate == nil &#124;&#124; candidate.DocumentId == 0 &#124;&#124; candidate.ReleaseId == 0 &#124;&#124; candidate.AggregateSeq == 0 &#124;&#124; 			!candidatePhysical.MatchString(candidate.PhysicalIndex) &#124;&#124; ValidateSo …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(candidate.SortJsonUtf8, &amp;tuple) != nil &#124;&#124; json.Unmarshal(tuple&#91;1&#93;, &amp;document) != nil &#124;&#124; 			document != strconv.FormatUint(candidate.DocumentId, 10)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>len(candidates) != 0 &amp;&amp; !bytes.Equal(after, candidates&#91;len(candidates)-1&#93;.SortJsonUtf8)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=497:1749 -->
````go
func validateDigestCandidates(before, after json.RawMessage, candidates []*pb.SearchCandidate, more bool) error {
	invalid := errors.New("SEARCH_CURSOR_DIGEST_INVALID")
	if len(candidates) > 100 || validateBatch(Batch{Chain: [32]byte{1}, Before: before, After: after,
		Count: uint8(len(candidates)), More: more, Digest: [32]byte{1}}) != nil {
		return invalid
	}
	seen := make(map[uint64]bool, len(candidates))
	physical := ""
	for _, candidate := range candidates {
		if candidate == nil || candidate.DocumentId == 0 || candidate.ReleaseId == 0 || candidate.AggregateSeq == 0 ||
			!candidatePhysical.MatchString(candidate.PhysicalIndex) || ValidateSort(candidate.SortJsonUtf8) != nil || seen[candidate.DocumentId] ||
			(physical != "" && physical != candidate.PhysicalIndex) {
			return invalid
		}
		var tuple []json.RawMessage
		var document string
		if json.Unmarshal(candidate.SortJsonUtf8, &tuple) != nil || json.Unmarshal(tuple[1], &document) != nil ||
			document != strconv.FormatUint(candidate.DocumentId, 10) {
			return invalid
		}
		physical = candidate.PhysicalIndex
		seen[candidate.DocumentId] = true
	}
	if len(candidates) != 0 && !bytes.Equal(after, candidates[len(candidates)-1].SortJsonUtf8) {
		return invalid
	}
	return nil
}

````

## 第 6 步：DigestCandidates

插入位置：第 5 步代码之后，紧接追加；完成稿约第 55 行。

DigestCandidates绑定完整有序候选与批边界，不依赖ES客户端，也不引入另一份RPC DTO。
PIT/nonce/时间会在重放时改变，不能参与摘要；用途标签与长度字段防止跨用途或拼接歧义。

本步声明：`func DigestCandidates(before, after json.RawMessage, candidates []*pb.SearchCandidate, more bool) ([32]byte, error)`。

**这一段为什么这样写：** 把分页重放的结果一致性压缩成可签名证据。

1. 验证整批及边界
2. 用长度前缀防拼接歧义
3. 按原顺序散列完整候选


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>before</code> | <code>json.RawMessage</code> | 本批起点原sort或nil<br>在此声明中的实际使用：<code>validateDigestCandidates(before, after, candidates, more)</code>；<code>digestField(digest, before)</code> |
| <code>after</code> | <code>json.RawMessage</code> | 本批终点原sort<br>在此声明中的实际使用：<code>validateDigestCandidates(before, after, candidates, more)</code>；<code>digestField(digest, after)</code> |
| <code>candidates</code> | <code>&#91;&#93;*pb.SearchCandidate</code> | 完整有序最小候选列表<br>在此声明中的实际使用：<code>validateDigestCandidates(before, after, candidates, more)</code>；<code>len(candidates)</code> |
| <code>more</code> | <code>bool</code> | 是否存在已验证探测项<br>在此声明中的实际使用：<code>validateDigestCandidates(before, after, candidates, more)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 32字节批摘要<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 候选或边界不一致错误<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>more</code>。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1749:2934 -->
````go
// DigestCandidates绑定完整有序候选与批边界，不依赖ES客户端，也不引入另一份RPC DTO。
// PIT/nonce/时间会在重放时改变，不能参与摘要；用途标签与长度字段防止跨用途或拼接歧义。
func DigestCandidates(before, after json.RawMessage, candidates []*pb.SearchCandidate, more bool) ([32]byte, error) {
	var result [32]byte
	if err := validateDigestCandidates(before, after, candidates, more); err != nil {
		return result, err
	}
	digest := sha256.New()
	digest.Write([]byte(candidateDigestPurpose))
	digestField(digest, before)
	digestField(digest, after)
	moreByte := byte(0)
	if more {
		moreByte = 1
	}
	digest.Write([]byte{byte(len(candidates)), moreByte})
	for _, candidate := range candidates {
		digestField(digest, []byte(candidate.PhysicalIndex))
		var identifiers [24]byte
		binary.BigEndian.PutUint64(identifiers[0:8], candidate.DocumentId)
		binary.BigEndian.PutUint64(identifiers[8:16], candidate.ReleaseId)
		binary.BigEndian.PutUint64(identifiers[16:24], candidate.AggregateSeq)
		digest.Write(identifiers[:])
		digestField(digest, candidate.SortJsonUtf8)
	}
	copy(result[:], digest.Sum(nil))
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/domain/cursor/digest.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/03-candidates-cursor.md)。
