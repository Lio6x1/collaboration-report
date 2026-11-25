# F04 固定原操作：回执、资格来源、关闭代次和跨服务身份

## 这节课要实现什么

用户点击一次退出或撤权，服务器已经处理，但网络断开导致浏览器没收到结果。本课定义恢复原操作的共同规则，避免用户重试时重复操作或误认成功。

## 做完后能看到什么

后续业务可以凭同一操作号查询原结果，并区分处理中、成功和确定失败；权限关闭绑定原来的资格范围与代次，不误关新权限。此处提供共用协议，实际退出和撤权由后续课程完成。

## 先想清楚怎样实现

1. 先定义操作回执。
2. 再明确权限来自哪里以及关闭哪一代权限。
3. 最后把这些事实绑定进跨服务请求与身份检查，让恢复沿用原目标。

## 怎样判断这一课完成

F04完成后运行平台回执与关闭绑定反例；跨服务实际停止输出和恢复分别在A07、D11、C03及06验收。

## 开始写之前

用户按一次“退出”“撤权”或“确认”，网络却在服务器提交后断开。此时客户端只能说结果未知，不能换个操作号再做一遍。本课先写跨服务共用的绑定规则，后续 A07、D11 和 C03 再接上真实排空和持久回执。

## 1. 创建操作回执文件，先定义返回模型

沿用 F03 的 `packages/platform` 目录，按 [operation_receipt.go](../../write/021-operation-receipt.md) 创建文件。先写 EncodedOperationReceipt，再写 EncodeOperationReceipt。

<!-- lesson-code: packages/platform/operation_receipt.go :: EncodedOperationReceipt -->
````go
// EncodedOperationReceipt 是写入回执所需的已校验状态和固定 JSON，不是另一套 HTTP DTO。
// 业务结果可以携带生成契约允许的不同字段，但原号、状态与可编码性必须在持久化前确认。
type EncodedOperationReceipt struct {
	State string
	JSON  json.RawMessage
}
````
<!-- /lesson-code -->

State 是要写入回执表的已校验状态，JSON 是已固定的原结果字节。json.RawMessage 是 JSON 字节，不再套一层 JSON 字符串；它也不是新造的一份 HTTP DTO。

<!-- lesson-code: packages/platform/operation_receipt.go :: EncodeOperationReceipt -->
````go
func EncodeOperationReceipt(operation string, value map[string]any) (EncodedOperationReceipt, error) {
	id, ok := value["operation_id"].(string)
	if !ok || operation == "" || id != operation {
		return EncodedOperationReceipt{}, errors.New("OPERATION_RESULT_BINDING_INVALID")
	}
	state, ok := value["state"].(string)
	if !ok {
		return EncodedOperationReceipt{}, errors.New("OPERATION_RESULT_STATE_INVALID")
	}
	switch state {
	case "PREPARING", "SUCCEEDED", "REJECTED", "CANCELLED":
	default:
		return EncodedOperationReceipt{}, errors.New("OPERATION_RESULT_STATE_INVALID")
	}
	// 继续使用旧 map 的确定性 JSON 编码，避免固定回执/摘要因字段顺序变化而改变。
	raw, err := json.Marshal(value)
	if err != nil {
		return EncodedOperationReceipt{}, fmt.Errorf("OPERATION_RESULT_ENCODING_FAILED: %w", err)
	}
	return EncodedOperationReceipt{State: state, JSON: raw}, nil
}
````
<!-- /lesson-code -->

`operation` 是调用者手上原操作号，`value` 是用例产生的结果。先验证结果中的 operation_id 类型和值完全相等，再检查状态只允许 PREPARING/SUCCEEDED/REJECTED/CANCELLED，最后 Marshal。返回 error 时 State/JSON 均为空，仓储不能持久化一个默认“成功”。

推演 `operation-123456789`：对应结果 state=PREPARING 且带 poll_url，编码保留这些扩展字段；把结果号改成 another-operation 立即失败；把 state 改为 RUNNING 也失败；result 放入 channel 导致 Marshal 失败并包装原错误。这里拒绝“RUNNING”不表示整个系统没有运行状态，而是这个固定操作回执契约没有该状态。

## 2. 写资格来源与摘要规则

新建 [fence.go](../../write/020-fence.md)。先写三个正则，再写 ValidAccountOutputScope、ValidAccountScope、ValidDocumentScope，之后写请求和 ACK 的绑定函数。编号必须规范十进制：`publication:01` 与 `publication:1` 不允许成为两个看似不同的同一对象。

<!-- lesson-code: packages/platform/fence.go :: SourceClosureDigest -->
````go
func SourceClosureDigest(scope string, epoch uint64) []byte {
	value := sha256.Sum256([]byte("AUTHORIZATION_CHANGE\n" + scope + "\n" + strconv.FormatUint(epoch, 10)))
	return value[:]
}
````
<!-- /lesson-code -->

scope 区分 `credential:123`、`org-member:组织:用户`、`doc-acl:文档`、`publication:文档`；epoch 是被关闭资格的代次。摘要中带固定用途标签和分隔符，把来源与代次一起绑定。它不是正文摘要，也不等于文档修订号。

以 credential:123、epoch=7 为例：构造原 digest 后把 epoch 改成 8，ValidBootstrapFence 应拒绝。撤销的是第 7 代资格，不能挪用回执声称第 8 代也已关闭。MatchesBootstrapAck 还比较 revocation_id、generation、scope、epoch、角色摘要和 MissingInstances，不能只检查 State==ACKNOWLEDGED。

文件后半部的 PrepareBootstrapFence/FinishBootstrapFence 是兼容底座边界：只有相关输出表确实为空才允许无业务输出的关闭；有记录会返回 FULL_OUTPUT_DRAIN_REQUIRED。后续完整业务使用真实输出排空器，不能为了让退出成功给活跃房间制造一个空 ACK。

## 3. 写跨服务请求模型，分清谁提供事实

依次创建 [account_closure.go](../../write/018-account-closure.md)、[authority_rpc_protocol.go](../../write/019-authority-rpc-protocol.md)。按各自操作页先写结构体，再写 Valid/Matches/构造函数。账号关闭绑定由 Account 在锁内固定，不接受用户自行提交“要删除哪些人的文档”作为权威范围。

<!-- lesson-code: packages/platform/authority_rpc_protocol.go :: AccountDocumentCleanupContext -->
````go
type AccountDocumentCleanupContext struct {
	AccountDocumentCleanupRequest
	Scope      string `json:"scope_key"`
	Epoch      string `json:"closed_epoch"`
	Revocation string `json:"revocation_id"`
	Digest     string `json:"plan_digest"`
}
````
<!-- /lesson-code -->

嵌入的 Request 带原 operation_id 和 document_id；Scope 为该文档 doc-acl 来源，Epoch 是关闭代次，Revocation 是关闭操作号，Digest 绑定整组固定计划。Node 边界编号用 string 是防止 JavaScript Number 改变 uint64，不是把它们变成昵称。构造 Context 时算出 digest，Valid 时重建同样的绑定并逐字段比较，收到任意不同字段就不能继续清理。

同文件中的通知目录只给候选人；接收资格仍要投递前重新检查。NotificationRecipient 的 Eligible 是本次判断，IdentityEpoch/CredentialEpoch/MemberEpoch 及 Sources 是判断所依赖的来源；OrgOwner 与 Role 区分所有者和成员角色，ExpiresUnixMS 限制资格时效。目录查到了一个人不代表永远有权看到这篇文档。

## 4. 把网络身份接到协议上

最后创建 [peer_clients_protocol.go](../../write/022-peer-clients-protocol.md)，先写 DialPeer，再写 HTTPPeer 和 HTTPCaller。cfg 提供本服务证书、CA 和目标地址，role 指定要连接的目标服务，返回可复用客户端或配置错误。

<!-- lesson-code: packages/platform/peer_clients_protocol.go :: DialPeer -->
````go
// DialPeer同时验证CA/hostname和目标服务URI，localhost证书不能互相冒充目标角色。
func DialPeer(cfg Config, role string) (*grpc.ClientConn, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, errors.New("RPC_TLS_INVALID")
	}
	data, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, errors.New("RPC_TLS_INVALID")
	}
	ca := x509.NewCertPool()
	if !ca.AppendCertsFromPEM(data) {
		return nil, errors.New("RPC_TLS_INVALID")
	}
	config := &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: ca, ServerName: "localhost", MinVersion: tls.VersionTLS13, VerifyConnection: func(state tls.ConnectionState) error {
		for _, uri := range state.PeerCertificates[0].URIs {
			if uri.String() == "spiffe://collaboration-report/"+role {
				return nil
			}
		}
		return errors.New("RPC_PEER_IDENTITY_MISMATCH")
	}}
	target := cfg.Peers[role]
	if target == "" {
		return nil, errors.New("RPC_PEER_MISSING")
	}
	// 服务实际绑定IPv4 loopback。本机localhost的DNS/IPv6解析曾让有限RPC超时，
	// 直接连已知绑定地址；TLS仍验证localhost和目标服务URI，不降低证书校验。
	if host, port, err := net.SplitHostPort(target); err == nil && host == "localhost" {
		target = net.JoinHostPort("127.0.0.1", port)
	}
	return grpc.NewClient(target, grpc.WithTransportCredentials(credentials.NewTLS(config)))
}
````
<!-- /lesson-code -->

连接时先验证 CA/localhost，再核对精确 `spiffe://collaboration-report/{role}` URI。四个服务在同一台电脑上，不意味着拿 document 证书就能冒充 account。把 localhost 目标换成已知绑定的 127.0.0.1 只优化本机解析，TLS 的 ServerName 与角色验证仍保留。HTTPPeer 同样核验身份，使用有限超时且不自动跟随重定向，避免认证请求被带到另一个地址。

接线关系为：业务 svc 构造客户端→客户端调用内部契约→对端平台拦截器验证服务角色→对端 handler/server 进入实际用例→仓储持久化→返回原绑定回执。浏览器不得直接调用这条内部链路。

## 5. 运行关闭绑定与回执反例

F03/F04 文件写齐后，运行 F03 的平台规则，再运行：

```powershell
# 工作目录：自己的学习工程。
go -C packages/platform test -mod=readonly . -run '^(TestNilScopeRejectedWithoutPanic|TestCredentialScopeFenceIsBoundToExactEpoch|TestPublicationScopeFenceRequiresOriginalBinding|TestEncodeOperationReceipt|TestOrganizationClosurePageProtectsOriginalCursor)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '原操作绑定规则失败' }
```

五个顶层 Test 应 RUN/PASS，无 SKIP。nil scope 必须返回拒绝而非 panic；凭据代次改变拒绝；正式版来源保持 Document 权威；回执错误号/未知状态/不可编码载荷不能落库；组织清理分页不能换原游标。它们没有真实断开一个用户 WSS，真实撤权关口到 06 做。

报 FENCE_ID_CONFLICT，先比较原请求是否被重新生成，而不是改新号重试；报 unknown 或提交不确定，保留原号去查持久进度；报 RPC_PEER_IDENTITY_MISMATCH，查本实例目标地址/证书角色，不去掉 VerifyConnection。

继续 [F05 离线重建制品](05-rebuild-artifacts.md)。
