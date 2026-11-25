# F05 写可核对的重建制品：元数据、签名和原字节

## 这节课要实现什么

搜索索引需要重建时，会接收文档服务导出的清单。我们必须确认清单来源可信、内容完整，而且属于这次重建。本课实现清单编码、签名核验与安全落盘。

## 做完后能看到什么

合法的原清单可以被读取；摘要、签名或操作绑定不符时拒绝使用；写到一半的文件不会被当成完整制品。这里还不会开始重建搜索索引。

## 先想清楚怎样实现

1. 先定义清单元数据。
2. 再逐行编解码并检查数量与摘要。
3. 最后绑定原请求、验证签名并原子发布完整文件。

## 怎样判断这一课完成

本课运行离线制品与签名测试；真实快照导入、日志追赶和索引切换在S04、S05及06完成。

## 开始写之前

搜索重建会把 Document 的固定元数据交给 Search。一个文件能被打开，不代表它来自正确服务；一个签名有效，也不代表它绑定这次原操作。本课先写共用的离线规则，S04/S05 再接到真实 Kafka 水位和别名切换。

## 1. 先创建元数据目录和绑定模型

```powershell
# 工作目录：自己的同一个学习工程。
New-Item -ItemType Directory -Path 'packages/platform/rebuildmeta' -Force | Out-Null
New-Item -ItemType Directory -Path 'packages/platform/rebuildartifact' -Force | Out-Null
```

先按 [binding.go](../../write/025-binding.md) 写 Prepare、Seal、正则及 Decimal/Hex，再逐段写 Bytes/Parse 方法。

<!-- lesson-code: packages/platform/rebuildmeta/binding.go :: Prepare -->
````go
// Prepare的整数采用字符串。原L与源/目标代次一起签名，不能靠两台机器的时间戳证明先后。
type Prepare struct {
	Version          int       `json:"version"`
	Operation        string    `json:"operation_id"`
	Owner            string    `json:"owner"`
	Alias            string    `json:"alias"`
	Source           string    `json:"source_index"`
	SourceGeneration string    `json:"source_generation"`
	SourceRevision   string    `json:"source_revision"`
	Target           string    `json:"target_index"`
	TargetGeneration string    `json:"target_generation"`
	Cluster          string    `json:"cluster_id"`
	Topic            string    `json:"topic"`
	TopicID          string    `json:"topic_id"`
	Starts           [3]string `json:"starts"`
	Nonce            string    `json:"nonce"`
}
````
<!-- /lesson-code -->

| Prepare 字段 | 作用 |
| --- | --- |
| Version、Operation | 制品格式版本和原重建号，不能仅凭一个可猜操作号认可任何文件。 |
| Owner、Alias | 独立实例与服务使用的搜索别名，防止别的实验环境接入。 |
| Source、SourceGeneration、SourceRevision | 重建开始时的物理源索引、代次及修订事实。 |
| Target、TargetGeneration | 新物理索引和严格 source+1 的目标代次，不能覆盖源索引。 |
| Cluster、Topic、TopicID | 原 Kafka 集群和 topic 的实体身份，同名重建 topic 也不能冒充原日志。 |
| Starts | 分区 0、1、2 的原起始 offset，各为规范十进制字符串。 |
| Nonce | 首次生成的随机绑定事实；恢复原任务时保留，不每次更换。 |

Decimal 先 ParseUint 再 FormatUint 比较，所以 `01`、`+1` 都不是规范输入；positive=true 时还拒绝 0。Prepare.Bytes 验证物理名、互不相同的 alias/source/target、代次不溢出、目标严格下一代、三分区 offset 不超过 MaxInt64，然后才 Marshal。

Seal.Version 是封存格式；PrepareRef 是整份原 prepare 的内容地址；Count 是元数据行数；EntriesDigest 是整个规范行流摘要。它不复制一份容易漂移的 owner/代次字段，而是通过内容地址绑定原 Prepare。

## 2. 一行一行写清单编码和扫描器

接着按 [entry.go](../../write/027-entry.md) 写 Entry、Line、Scan。

<!-- lesson-code: packages/platform/rebuildmeta/entry.go :: Entry.Line -->
````go
// Line使用规范十进制字符串保留全uint64；无正式指针用null，不用字符串"0"冒充引用。
func (e Entry) Line() ([]byte, error) {
	if e.Document == 0 {
		return nil, errors.New("REBUILD_ENTRY_INVALID")
	}
	release := "null"
	if e.Release != 0 {
		release = strconv.Quote(strconv.FormatUint(e.Release, 10))
	}
	return []byte(`{"document_id":"` + strconv.FormatUint(e.Document, 10) + `","minimum_seq":"` + strconv.FormatUint(e.Minimum, 10) + `","release_hint":` + release + "}\n"), nil
}
````
<!-- /lesson-code -->

Document 是正数文档 ID，Minimum 是最低事件序号，Release 为 0 表示没有正式指针。输出数字用字符串，缺正式版用 JSON null，且每条以 LF 结束；字符串 "0" 不能伪装成一个正式版本。

<!-- lesson-code: packages/platform/rebuildmeta/entry.go :: Scan -->
````go
// Scan只保留一行，严格检查升序、规范原字节和完整LF尾部。回调可写入调用者自己的事务，
// 返回的摘要/总数只有整个流成功才有效，调用者不能在封存前提交部分导入。
func Scan(reader io.Reader, visit func(Entry) error) (uint64, [32]byte, error) {
	hash := sha256.New()
	input := bufio.NewReaderSize(io.TeeReader(reader, hash), 4096)
	var count, previous uint64
	fail := func() (uint64, [32]byte, error) { return 0, [32]byte{}, errors.New("REBUILD_ENTRIES_INVALID") }
	for {
		line, err := input.ReadSlice('\n')
		if err == io.EOF && len(line) == 0 {
			break
		}
		if err != nil {
			return fail()
		}
		var wire struct {
			Document string  `json:"document_id"`
			Minimum  string  `json:"minimum_seq"`
			Release  *string `json:"release_hint"`
		}
		if json.Unmarshal(line, &wire) != nil {
			return fail()
		}
		document, err := strconv.ParseUint(wire.Document, 10, 64)
		if err != nil || document <= previous {
			return fail()
		}
		minimum, err := strconv.ParseUint(wire.Minimum, 10, 64)
		if err != nil {
			return fail()
		}
		var release uint64
		if wire.Release != nil {
			release, err = strconv.ParseUint(*wire.Release, 10, 64)
			if err != nil || release == 0 {
				return fail()
			}
		}
		entry := Entry{document, minimum, release}
		canonical, err := entry.Line()
		// 原字节比较同时拒绝重复/未知字段、非规范数字、重排和缺字段。
		if err != nil || !bytes.Equal(canonical, line) || count == ^uint64(0) {
			return fail()
		}
		if visit != nil {
			if err := visit(entry); err != nil {
				return 0, [32]byte{}, err
			}
		}
		previous, count = document, count+1
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return count, digest, nil
}
````
<!-- /lesson-code -->

`reader` 可以是大文件流，`visit` 每次只收到一条已验证 Entry；回调返回 error 会立即停止并返回零计数/零摘要。过程依次读完整 LF 行、解码、检查文档严格递增、校验整数、重新 Line 编码并与原行逐字节比较、调用回调、累积前一 ID 和数量。最后全部成功才返回 count/digest。回调若写数据库，应写调用者的同一事务，不能每读一行就提交后宣称整批封存成功。

反例：把两行文档 ID 42、43 换成 43、42，会在第二行拒绝；加一个未知字段或调换 JSON 字段顺序，即使反序列化后值相似，也会在原字节比较拒绝。读到半行 EOF 同样失败，不能把截断文件当作较小的完整清单。

## 3. 把原请求绑定到描述符

新建 [descriptor.go](../../write/026-descriptor.md)。先写 Descriptor、ErrDescriptor，再写引用校验、Bytes/Parse、DescriptorFor 和 RequestFingerprint。

Descriptor 的 Version/PrepareRef/SnapshotRef 固定格式和两份制品；Cluster/Topic/TopicID/Starts 固定日志身份及原起始窗口，不含正文或私钥。DescriptorFor 接收已验证 Prepare 和两个地址，返回完整描述符；RequestFingerprint 再将 operation、generation、snapshotRef、watermark 原字节做带用途标签和长度前缀的摘要，消除单纯字符串拼接的歧义。

如果两次请求号一样但 watermark 变化，摘要不同，应拒绝作为原任务恢复。Get 不能只拿 operation_id 就把另一份快照导入到当前任务。

## 4. 最后写签名和原子发布

按 [artifact.go](../../write/023-artifact.md) 先写用途/大小常量、错误、envelope、Signer/Verifier/Verified，再写构造、签名、验签和解码辅助函数；按 [files.go](../../write/024-files.md) 写内容地址、文件名/读取及原子发布。

envelope 包含 Purpose、Certificates、Payload、Signature；证书链是验证输入，不是自带信任根。Signer 持有本服务私钥、证书链、role 和 verifier；Verifier 只持有受信 CA 根。Verified.Payload 是经过验证的原载荷，Reference 是制品内容地址，SignerFingerprint 是签名身份摘要，Role 是经证书核对的服务角色。

NewSigner 只允许 search/document，读取明确配置的证书/私钥/CA 并核对身份；Sign 按用途生成签名；Verify 核对用途、身份、大小与签名后才返回 Payload。RSA-PSS 带随机盐，两次签同一载荷会产生不同 envelope，因此首次制品必须保存；重试不能重新签一个“内容差不多”的文件替代原地址。

PublishFile 的 write 回调写临时文件，全部成功并同步后才发布目标名，失败不留下看似完整的产物。Reference/FileName 只接受规定内容地址，不能把用户字符串直接拼接成任意文件路径。恢复已接受制品时的 RestoreAccepted 使用原持久 envelope/signature 绑定；不是跳过首次信任验证的新入口。

## 5. 接线和测试

这两组库本身不注册用户路由。Search prepare CLI 创建原 Prepare 制品→Document 快照 CLI 核对后封存元数据→Search 导入/完成用同一 descriptor 和 fingerprint，D14/S04/S05 会将这些库接到实际仓储和内部 RPC。

```powershell
# 工作目录：自己的学习工程；这两个包已经在本课写齐。
go -C packages/platform test -mod=readonly ./rebuildmeta ./rebuildartifact -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '重建制品规则失败' }
```

预期 `TestCanonicalSnapshotEntries`、`TestPrepareAndSealBindExactOriginalFacts`、`TestDescriptorBindsOriginalStartRequest`、`TestArtifactIdentityAndOriginalBytes` 均 RUN/PASS，无 SKIP。这证明本机纯规则和测试证书下的签名绑定，不证明 ES 已重建或别名已切换。真实关口仍在 06。

报 REBUILD_NUMBER_INVALID，查是否把编号格式化成补零展示值或 JS 数字；报 identity/signature 错误，查自己的服务证书角色与原制品，不改用文件内证书当 CA；缺 LF 或 digest 不符，保留原文件定位截断，不重新排序后冒充原制品。

F01—F05 写齐后可运行 [01 阶段检查](../../01-foundation.md)，随后进入账号课。完整答案仍只供写完后核对。
