# 03 文档、权限与完整工作流：功能带写

这一阶段实现文档从创建到协作、审核、发布和清理的完整业务。接通后，用户可管理知识库、阅读和编辑文档、拆分章节、上传图片、发表评论与导出文件；每一步都受当前权限和已确认版本约束，不能为了方便覆盖别人工作。

先明确谁能操作哪份文档，再依次完成工作区、主线与支线、章节、审核、发布、图片、评论和导出，最后处理撤权、回收转移与搜索接线。D14是文档整包构建关口；转换器也依赖尚待写齐的业务声明，真实功能在06验证。

先按下表进入功能课，跟随具体输入、字段和函数逐段落笔。[文件与完整答案索引](03-files.md)只用于查找，表内文件序号不是教学顺序。

| 课次 | 按功能一步一步实现 | 所属文件 |
| --- | --- | --- |
| D01 | [身份、权限与共享准入](lessons/document/01-identity-access.md) | 14 |
| D02 | [创建工作区与可靠正文阅读](lessons/document/02-workspace-content.md) | 13 |
| D03 | [固定主线操作与支线更新](lessons/document/03-main-draft.md) | 13 |
| D04 | [结构预览、拆章、合章与历史范围](lessons/document/04-structure.md) | 13 |
| D05 | [可靠送审与固定轮次](lessons/document/05-submission-review.md) | 6 |
| D06 | [审核合并、冲突修复与反向更改](lessons/document/06-merge-conflict.md) | 14 |
| D07 | [正式发布与当前受众](lessons/document/07-publication.md) | 7 |
| D08 | [资源上传、配额、保护与真实对象存储](lessons/document/08-resources-storage.md) | 23 |
| D09 | [固定批注、提及与可靠通知](lessons/document/09-comments-notifications.md) | 10 |
| D10 | [固定导出、转换器与进程退出](lessons/document/10-exports.md) | 12 |
| D11 | [输出登记、撤权与真实排空](lessons/document/11-output-fence.md) | 13 |
| D12 | [回收恢复、永久清理与个人转移](lessons/document/12-trash-transfer.md) | 18 |
| D13 | [账号注销、组织关闭与公开留存](lessons/document/13-account-retirement.md) | 17 |
| D14 | [搜索资格、快照重建与文档服务总装](lessons/document/14-search-assembly.md) | 23 |

## 阶段检查

先完成各课规定的依赖；下列为本阶段的收口命令。工作目录仍是自己的lessonRoot，GOWORK/GOCACHE按00配置；coursePrepare/lessonRoot在00设置，新开终端须重新设置。

```powershell
go -C services/document test -mod=readonly ./internal/domain/document -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '文档模型与纯规则失败' }
```

```powershell
go -C services/document build -mod=readonly ./...
if ($LASTEXITCODE -ne 0) { throw '文档服务构建失败' }
go -C services/document test -mod=readonly ./internal/logic/document -run '^(TestResolutionBodyRejectsDuplicateStableBlockIdentities|TestResolutionBodyPreservesRepeatedInlineResourceReferences|TestResolutionRejectsOnlyDefiniteProposalOrTransferFailures)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '冲突内容与未知结果边界失败' }
python $coursePrepare --target $lessonRoot --check 03
if ($LASTEXITCODE -ne 0) { throw '文档文件未齐' }
```

domain涉及多个功能模型，须按各课把相关声明写齐后执行纯规则；D14全服务装配后再build和三个logic反例。重复块ID拒绝，合法重复图片引用保留，确定拒绝与网络未知区分。真实数据库、跨服务和转换文件在06验收。

缺声明先查本阶段分步是否漏写；重复声明检查是否把完整答案又追加一遍。不可用、超时或未知结果按各课说明保留原操作，不能以删断言、跳过测试或另造新号制造成功。真实功能和页面按[06完整装配](06-integration.md)继续。
