# 04 Node协作与可靠保存：功能带写

这一阶段让多人真正共同编辑：输入的内容得到可靠保存，重连后可恢复；送审和合并使用固定内容，拆章时保护在线编辑者的输入，撤权后旧连接停止工作。正文与权限的权威结果仍由文档服务配合确认。

先理解一次正文更新如何检查和恢复，再连接准入、落库后的保存确认与保存证明；随后实现撤权、固定工作流、在线结构协调和清理，C07完成网络与进程装配。语法检查不等于多人保存成功，全部依赖齐备后跑规则，真实WSS与数据库在06验证。

先按下表进入功能课，跟随具体输入、字段和函数逐段落笔。[文件与完整答案索引](04-files.md)只用于查找，表内文件序号不是教学顺序。

| 课次 | 按功能一步一步实现 | 所属文件 |
| --- | --- | --- |
| C01 | [正文契约、隔离候选与连续日志恢复](lessons/collaboration/01-content-replay.md) | 8 |
| C02 | [真实准入、UPDATE持久确认与FLUSH](lessons/collaboration/02-admission-save.md) | 11 |
| C03 | [撤权门闩、真实输出排空与进程恢复](lessons/collaboration/03-fence-recovery.md) | 10 |
| C04 | [固定送审、三方合并与草稿变换](lessons/collaboration/04-fixed-workflows.md) | 22 |
| C05 | [结构拆并与每个在线编辑器保存握手](lessons/collaboration/05-structure-online.md) | 9 |
| C06 | [固定候选清理、账号清理与转移计量](lessons/collaboration/06-cleanup-storage.md) | 7 |
| C07 | [协作完整依赖装配、传输与就绪](lessons/collaboration/07-wiring-readiness.md) | 13 |

## 阶段检查

先完成各课规定的依赖；下列为本阶段的收口命令。工作目录仍是自己的lessonRoot，GOWORK/GOCACHE按00配置；coursePrepare/lessonRoot在00设置，新开终端须重新设置。

```powershell
# 工作目录：原lessonRoot；先完成本阶段所有源码，00已安装锁定依赖。
$lessonRuleFiles = @(Get-ChildItem -LiteralPath 'services/collaboration/tests' -Filter '*.test.mjs' |
    Where-Object { $_.Name -ne 'readiness.test.mjs' } | ForEach-Object { $_.FullName })
node --no-maglev --test --test-reporter=tap --test-concurrency=1 @lessonRuleFiles
if ($LASTEXITCODE -ne 0) { throw '协作规则或用例失败' }
python $coursePrepare --target $lessonRoot --check 04
if ($LASTEXITCODE -ne 0) { throw '协作文件未齐' }
```

规则组应PASS、没有失败或跳过。readiness.test.mjs明确要求06的真实配置和数据库，移到真实关口执行。规则替身不能证明MySQL持久保存；真实WSS须确认落库后SAVED、去重、FLUSH和断线恢复。使用锁定Node24.19及显式no-maglev，核对进程退出码。

缺声明先查本阶段分步是否漏写；重复声明检查是否把完整答案又追加一遍。不可用、超时或未知结果按各课说明保留原操作，不能以删断言、跳过测试或另造新号制造成功。真实功能和页面按[06完整装配](06-integration.md)继续。
