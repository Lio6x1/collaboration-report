# 05 正式版搜索与索引恢复：功能带写

这一阶段实现正式版文档搜索：发布后进入索引，用户按关键词查询和翻页，撤下或失去权限后不再暴露正文。服务重启能继续消费变更，索引需要重建时可追赶期间变化并安全切换。

先理解发布事件如何变成可搜索内容，再保存消费进度、实现候选分页，最后准备快照、回填、追赶和切换。搜索索引是可重建副本，当前阅读资格由文档服务复核；三个存储系统不是一个事务，未知结果必须恢复原事件。S05完成全包接线，06验证真实搜索与重建。

先按下表进入功能课，跟随具体输入、字段和函数逐段落笔。[文件与完整答案索引](05-files.md)只用于查找，表内文件序号不是教学顺序。

| 课次 | 按功能一步一步实现 | 所属文件 |
| --- | --- | --- |
| S01 | [Canal事件与Doc当前正式投影](lessons/search/01-event-authority.md) | 7 |
| S02 | [持久消费、日志身份与代次锁](lessons/search/02-consumer-ledger.md) | 15 |
| S03 | [签名批锚点、PIT与当前权限二次过滤](lessons/search/03-candidates-cursor.md) | 9 |
| S04 | [先封存L，再导入签名快照并种子回填](lessons/search/04-rebuild-snapshot.md) | 6 |
| S05 | [固定窗口重放、完整核验与别名恢复切换](lessons/search/05-cutover-wiring.md) | 8 |

## 阶段检查

先完成各课规定的依赖；下列为本阶段的收口命令。工作目录仍是自己的lessonRoot，GOWORK/GOCACHE按00配置；coursePrepare/lessonRoot在00设置，新开终端须重新设置。

```powershell
# 工作目录：原lessonRoot。
go -C services/search build -mod=readonly ./...
if ($LASTEXITCODE -ne 0) { throw '搜索服务构建失败' }
go -C services/search test -mod=readonly ./internal/domain/... -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '事件、游标或索引规则失败' }
go -C services/search test -mod=readonly ./internal/logic/consumer -run '^TestWindowRejectsLostHistoryAndBrokerPositionAheadOfSQL$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '连续消费水位失败' }
python $coursePrepare --target $lessonRoot --check 05
if ($LASTEXITCODE -ne 0) { throw '完整后端文件未齐' }
```

构建、domain规则和consumer具名水位测试应PASS、无SKIP。大整数身份不经float；游标核用途/精确帧/摘要；旧事件不能复活隐藏内容。此处不代表真实Canal/Kafka/ES消费完成，实际投影和搜索页面在06验证。

缺声明先查本阶段分步是否漏写；重复声明检查是否把完整答案又追加一遍。不可用、超时或未知结果按各课说明保留原操作，不能以删断言、跳过测试或另造新号制造成功。真实功能和页面按[06完整装配](06-integration.md)继续。
