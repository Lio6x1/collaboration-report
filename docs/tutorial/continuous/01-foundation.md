# 01 公共底座、正文契约和SQL：功能带写

这一阶段为后续保存正文、注册账号和跨服务操作建立基础：同一正文得到相同字节，业务数据有持久归属，服务能安全启动，断网后的操作有恢复依据。这里还没有完整用户页面。

先用具体正文样例理解校验，再设计数据表、启动与错误出口、原操作协议和重建文件。F01可独立测试；F03引用F04的同包声明，写齐后运行平台规则；F05两个子包独立检查。SQL到06自己的实例才安装。

先按下表进入功能课，跟随具体输入、字段和函数逐段落笔。[文件与完整答案索引](01-files.md)只用于查找，表内文件序号不是教学顺序。

| 课次 | 按功能一步一步实现 | 所属文件 |
| --- | --- | --- |
| F01 | [严格正文、规范字节与格式契约](lessons/foundation/01-fixed-content.md) | 3 |
| F02 | [私有库、业务约束与连续迁移](lessons/foundation/02-private-databases.md) | 15 |
| F03 | [配置、连接池、就绪与安全出口](lessons/foundation/03-runtime-and-security.md) | 5 |
| F04 | [原操作、资格来源与跨服务身份](lessons/foundation/04-original-operation.md) | 5 |
| F05 | [重建元数据、签名与原子制品](lessons/foundation/05-rebuild-artifacts.md) | 5 |

## 阶段检查

先完成各课规定的依赖；下列为本阶段的收口命令。工作目录仍是自己的lessonRoot，GOWORK/GOCACHE按00配置；coursePrepare/lessonRoot在00设置，新开终端须重新设置。

```powershell
# 工作目录：自己的同一个lessonRoot；GOWORK=off，GOCACHE沿00设置。
go -C contracts test -mod=readonly ./content -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '正文契约失败' }
go -C packages/platform test -mod=readonly . -run '^(TestNilScopeRejectedWithoutPanic|TestCredentialScopeFenceIsBoundToExactEpoch|TestPublicationScopeFenceRequiresOriginalBinding|TestEncodeOperationReceipt|TestProtectedAccountAndDocumentOutputsDoNotUseDetachedTimeoutBuffer)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '公共边界规则失败' }
python $coursePrepare --target $lessonRoot --check 01
if ($LASTEXITCODE -ne 0) { throw '阶段01文件未齐' }
```

正文六项和平台具名用例应实际RUN/PASS，无SKIP。各课还给出就绪、安全及重建制品反例。这里没有安装真实数据库；摘要核对不替代06的迁移、授权与恢复验收。

缺声明先查本阶段分步是否漏写；重复声明检查是否把完整答案又追加一遍。不可用、超时或未知结果按各课说明保留原操作，不能以删断言、跳过测试或另造新号制造成功。真实功能和页面按[06完整装配](06-integration.md)继续。
