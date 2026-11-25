# 当前契约与生成物

00支持包已经带好生成的DTO、RPC、描述符及正文校验器，照课程亲写业务不要求先重生成。生成物来自公开契约，不能为了让业务代码编译通过在生成文件中补字段。以下仅在需要核对或修改允许扩展的契约时使用，始终在自己的lessonRoot。

HTTP的权威输入是contracts/http下的OpenAPI及生成器读取的扩展，RPC输入是contracts/proto/internal.proto。generate-contracts.mjs输出contracts/http/generated.ts、contracts/httpdto/generated.go、contracts/http/routes.json，以及contracts/proto/caller-policy.json和packages/platform/rpc-policy.json两份相同调用策略。它不生成Go RPC stub，也没有--check参数。

```powershell
Set-Location -LiteralPath $lessonRoot
node scripts/generate-contracts.mjs
if ($LASTEXITCODE -ne 0) { throw 'HTTP契约生成失败' }
gofmt -w contracts/httpdto/generated.go
if ($LASTEXITCODE -ne 0) { throw '生成DTO格式化失败' }
node scripts/generate-content.mjs --check
if ($LASTEXITCODE -ne 0) { throw '正文生成物与契约不一致' }
```

HTTP生成器输出httpOperations/tsModels/rpcMethods统计，数字来自当前契约，不照抄历史课程数量。正文生成器的--check只核一致性；确实修改允许扩展的契约后，先运行 `node scripts/generate-content.mjs` 再--check与F01/C01规则。不要修改冻结的基础正文定义绕开规则。

如需重生成RPC，当前文件头记录protoc 5.29.3、protoc-gen-go 1.28.1、protoc-gen-go-grpc 1.2.0。它们是生成工具版本，与go.mod的运行库版本不同。下载[Protocol Buffers v29.3官方发行](https://github.com/protocolbuffers/protobuf/releases/tag/v29.3)的Windows工具包，将解出的bin目录加入当前PATH，运行protoc --version核对发行身份（命令行与生成文件头可能采用29.3/5.29.3不同表示）。Go插件可安装到本工程缓存：

```powershell
Set-Location -LiteralPath $lessonRoot
$env:GOBIN = Join-Path $lessonRoot 'artifacts/proto-tools'
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
if ($LASTEXITCODE -ne 0) { throw 'Go proto插件安装失败' }
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0
if ($LASTEXITCODE -ne 0) { throw 'Go gRPC插件安装失败' }
$env:PATH = $env:GOBIN + ';' + $env:PATH
protoc --version
protoc-gen-go --version
protoc-gen-go-grpc --version
protoc --proto_path=contracts/proto --go_out=contracts --go_opt=module=collaboration/contracts --go-grpc_out=contracts --go-grpc_opt=module=collaboration/contracts --descriptor_set_out=contracts/proto/internal.descriptor.pb contracts/proto/internal.proto
if ($LASTEXITCODE -ne 0) { throw 'RPC生成失败' }
```

预期产生contracts/internalv1/internal.pb.go、internal_grpc.pb.go和contracts/proto/internal.descriptor.pb。Go健康服务使用gRPC运行库内类型；Node读取已有health.proto。用自己的Git或编辑器比较生成前后变化，保留差异原因。找不到插件先核GOBIN/PATH，找不到proto文件先核工作目录；没有安装工具时保留支持包产物，将“重生成”记为未执行即可继续业务学习。
