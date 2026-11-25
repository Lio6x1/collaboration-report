# D10｜固定导出、转换器与进程退出

## 这节课要实现什么

用户点击导出Word、PDF或Markdown，等待完成后下载文件；多人同时导出时服务也应有控制。本课实现固定内容导出与后台转换任务。

## 做完后能看到什么

接通后，导出内容来自已确认的固定版本，任务可查询进度；下载时仍要检查权限。转换失败或超时有明确状态，同时运行的转换数量受限，不会每次点击就无限启动进程。

## 先想清楚怎样实现

1. 先固定正文、资源和导出参数。
2. 保存作业并预留预算。
3. 后台领取任务、调用真实转换器并管理子进程退出。
4. 保存产物后通过受保护下载入口交付。

## 怎样判断这一课完成

D14后分别运行规则与转换器检查，06实际导出并打开三种产物核对正文、图片和分页；任务标记完成不能替代打开文件验证。

## 开始写之前

继续[D09](./09-comments-notifications.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。导出输入先固定清单与正式版，转换器消费已校验的正文，而不自行找当前草稿。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/export.go**，用途是导出输入先固定清单与正式版，转换器消费已校验的正文，而不自行找当前草稿。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 先规定固定导出的输入、输出及小接口

export.go先写ExportInput/ParseExportInput、ExportSnapshot、ExportRecord、ExportDocument、ExportUnit/ExportAsset。MAIN请求绑定expected_manifest_id，PUBLIC请求绑定expected_release_id；严格union只有四个字段，format只接受DOCX/MARKDOWN/PDF。Snapshot记录document/space/manifest/structure/style、release/publication、标题、抓取时间及Sources；Record另记录requester、operation、fingerprint、generation、artifact key/size/checksum/expiry。generation用于区分原worker的实际处理代次，不改变业务operation。

在exports_service.go先写ExportRenderer：

<!-- lesson-code: services/document/internal/logic/document/exports_service.go :: ExportRenderer -->
````go
type ExportRenderer interface {
	Render(context.Context, domain.ExportRecord, domain.ExportDocument) (string, uint64, [32]byte, error)
	Cleanup(context.Context, domain.ExportRecord) error
	Files() ResourceFiles
}
````
<!-- /lesson-code -->

Render(ctx,record,document)返回真实制品key、字节数、SHA-256及error；Cleanup清原代次临时物，Files返回实际ResourceFiles下载适配器。exporter/rendering_export.go真实依赖logic.ResourceFiles，**不能在logic未齐时声称exporter可独立编译**。先建立本接口，再写实现，禁止为了提前编译另外复制接口类型或写空renderer。

## 3. 写持久作业和预算，再写领取/恢复

exports_queries.go把job、原输入指纹、固定snapshot、任务generation、暂存预算及完成产物一起存储。Create从当前MAIN真实read view或锁定MAIN取得资格；PUBLIC只允许当前正式版与当前受众。事务内重核资格与固定指针、预留预算、写作业及原回执；HTTP202只是已登记。

在NewExports位置输入完整构造：

<!-- lesson-code: services/document/internal/logic/document/exports_service.go :: NewExports -->
````go
func NewExports(db *sql.DB, identity *Identity, access *Access, public *PublicReading, outputs *Outputs, resources *Resources, renderer ExportRenderer, workers int) (*Exports, error) {
	if workers == 0 {
		workers = 1
	}
	if workers < 1 || workers > 8 {
		return nil, errors.New("EXPORT_WORKERS_INVALID")
	}
	// 这是持久预留的磁盘预算，不代表转换器的内存上限。
	// 每worker留256MiB，沿用单任务192MiB预留和32MiB制品限制。
	return &Exports{db: db, identity: identity, access: access, public: public, outputs: outputs, resources: resources, renderer: renderer,
		workers: workers, temporaryLimit: uint64(workers) * repository.ExportTemporaryLimit}, nil
}
````
<!-- /lesson-code -->

workers缺省为1，仅允许1～8；temporaryLimit随worker数乘持久256MiB预算，每任务保留192MiB，实际制品上限32MiB。预算是磁盘/中间产物边界，不宣称限制转换器所有内存。超过预算应排队或明确失败，不能给每次点击开无界goroutine。

接着输入Create、Recover及作业完成/失败/下载全部段。领取短事务持久标RUNNING及generation，Commit后才转换；完成事务带原代次核验并记key/size/checksum、释放预算。进程崩溃从原job恢复，旧worker迟到不能覆盖新generation。未知Commit不能直接删制品或退款，必须查询原事实。

例子：创建MAIN manifest M1后主线继续M2，导出仍是M1；当前正式R1切换R2后，原公开下载重新核当前资格，不因文件已生成就永久开放。取消时必须等转换进程退出、文件IO结束才释放暂存，不能看到context取消便立即退款。

## 4. 写真实转换器与进程生命周期

按表完成exporter八文件。content_export.go将已校验固定正文转Pandoc AST，按structure parent关系递归输出章/节，不按unit自增ID拼篇；assets.go只写清单内实际图片，不抓任意URL。docx.go设置真实Word排版，formats.go处理三格式转换，rendering_export.go处理临时目录、60秒共享转换预算、48MiB AST、32MiB制品及真实校验和。

<!-- lesson-code: services/document/internal/exporter/rendering_export.go :: Renderer.Render -->
````go
func (r *Renderer) Render(ctx context.Context, value domain.ExportRecord, document domain.ExportDocument) (string, uint64, [32]byte, error) {
	var empty [32]byte
	if err := r.Cleanup(ctx, value); err != nil {
		return "", 0, empty, err
	}
	directory := r.directory(value)
	if err := os.Mkdir(directory, 0700); err != nil {
		return "", 0, empty, errors.New("EXPORT_TEMPORARY_UNAVAILABLE")
	}
	defer os.RemoveAll(directory)
	assets, err := writeAssets(directory, document.Assets)
	if err != nil {
		return "", 0, empty, err
	}
	ast, err := buildAST(document, assets)
	if err != nil {
		return "", 0, empty, err
	}
	if len(ast) > 48<<20 {
		return "", 0, empty, domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
	}
	input := filepath.Join(directory, "source.json")
	if err = os.WriteFile(input, ast, 0600); err != nil {
		return "", 0, empty, err
	}
	// 每次转换共享一个60秒预算，不给每个步骤分别续满时限。
	attempt, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	path, err := r.convert(attempt, directory, input, document)
	if err != nil {
		return "", 0, empty, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", 0, empty, errors.New("EXPORT_ARTIFACT_MISSING")
	}
	if info.Size() <= 0 || info.Size() > 32<<20 {
		return "", 0, empty, domain.Violation(domain.StateConflict, "EXPORT_ARTIFACT_LIMIT")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, empty, err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err = io.Copy(digest, file); err != nil {
		return "", 0, empty, err
	}
	var checksum [32]byte
	copy(checksum[:], digest.Sum(nil))
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return "", 0, empty, err
	}
	key := artifactKey(value)
	unlock, err := r.files.Acquire(ctx, key)
	if err != nil {
		return "", 0, empty, err
	}
	defer unlock()
	if err = r.files.Put(ctx, key, file, uint64(info.Size()), checksum); err != nil {
		return "", 0, empty, err
	}
	return key, uint64(info.Size()), checksum, nil
}
````
<!-- /lesson-code -->

Render先清原临时目录，写真实资源与AST，调用受控转换器，检查结果是普通非空文件、算SHA-256、持真实存储锁并Put，再返回元数据。转换失败不返回固定成功。公式转换警告不能静默丢内容；输出stderr可能含用户正文，只记录稳定错误码。

process_windows.go用隐藏窗口/Job对象管理后代进程；process_linux.go处理进程组；other明确不支持。client/process_*用于Edge/输出实例的进程归属判断，属于D11，不能把两个平台组删为一份。取消一个Pandoc/浏览器任务必须连仍持stdout的后代一起终止并等退出，避免worker永远不归还。

用户点击导出→handler/exports.go→Exports.Create→持久队列→D14的RunExports固定池→Recover→Renderer.Render→完成作业；下载→重新检查当前MAIN/正式资格→登记两段输出→Files.Open。D14的resources_outputs_wiring.go注入真实Pandoc/Node/Edge路径和Renderer，publication_outputs_workers.go启动固定池。

## 5. 三类检查分开

包齐后离线：

~~~powershell
go -C services/document test -mod=readonly ./internal/logic/document -run '^TestExportPoolConfiguration$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '上一组文档规则检查失败' }
go -C services/document test -mod=readonly ./internal/worker -run '^TestExportPoolBoundsConcurrencyAndJoinsCancellation$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '导出池规则失败' }
~~~

前者在logic/document/exports_pool_test.go，后者在worker/exports_pool_test.go；核合法并发及等待取消完成，不调用实际Pandoc。

Windows进程包齐后可另跑 TestRunProgramDoesNotCreateConsole、TestRunProgramCancelsDescendantsHoldingOutput（exporter/process_windows_test.go）；它们会启动测试子进程，属于真实本机进程检查。TestParagraphFormattingPandocExports 需要本机Pandoc，缺工具不能记PASS。

06 READY、Pandoc3.8/Edge/字体按运行课准备后：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/exports/formats.test.mjs tests/exports/pool.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实导出或恢复失败' }
~~~

fixed MAIN and anonymous PUBLIC export actual DOCX Markdown and PDF with current authorization 核真实三格式/图片/下载；bounded export pool resumes original concurrent jobs after downscaling 会修改并恢复本学习实例配置并重启本owner，必须串行且勿指向体验实例。202查job状态；conversion failed查工具/字体/公式；预算未退先查实际进程和原完成事务，不删历史或伪造成功。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/export.go | [逐声明/函数带写](../../write/093-export.md) |
| 2 | services/document/internal/repository/document/exports_queries.go | [逐声明/函数带写](../../write/154-exports-queries.md) |
| 3 | services/document/internal/logic/document/exports_service.go | [逐声明/函数带写](../../write/205-exports-service.md) |
| 4 | services/document/internal/exporter/assets.go | [逐声明/函数带写](../../write/118-assets.md) |
| 5 | services/document/internal/exporter/content_export.go | [逐声明/函数带写](../../write/119-content-export.md) |
| 6 | services/document/internal/exporter/docx.go | [逐声明/函数带写](../../write/120-docx.md) |
| 7 | services/document/internal/exporter/formats.go | [逐声明/函数带写](../../write/121-formats.md) |
| 8 | services/document/internal/exporter/process_linux.go | [逐声明/函数带写](../../write/122-process-linux.md) |
| 9 | services/document/internal/exporter/process_other.go | [逐声明/函数带写](../../write/123-process-other.md) |
| 10 | services/document/internal/exporter/process_windows.go | [逐声明/函数带写](../../write/124-process-windows.md) |
| 11 | services/document/internal/exporter/rendering_export.go | [逐声明/函数带写](../../write/125-rendering-export.md) |
| 12 | services/document/internal/handler/document/exports.go | [逐声明/函数带写](../../write/253-exports.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D11](./11-output-fence.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/export.go',
    'services/document/internal/repository/document/exports_queries.go',
    'services/document/internal/logic/document/exports_service.go',
    'services/document/internal/exporter/assets.go',
    'services/document/internal/exporter/content_export.go',
    'services/document/internal/exporter/docx.go',
    'services/document/internal/exporter/formats.go',
    'services/document/internal/exporter/process_linux.go',
    'services/document/internal/exporter/process_other.go',
    'services/document/internal/exporter/process_windows.go',
    'services/document/internal/exporter/rendering_export.go',
    'services/document/internal/handler/document/exports.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
