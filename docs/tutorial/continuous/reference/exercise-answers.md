# 五项练习：当前答案与验证

本页对应[07练习](../07-exercises.md)。实现都已在39课及write页给全，练习不承载下一课必需代码。下面的规则命令以已完成05、回到自己的lessonRoot、GOWORK=off为前置；真实接口测试必须等06完整READY。各课提供完整命令、具体测试名和失败定位。

## 1. 新密码与原密码

`abcdefgh`、`AbCdEfGh`都只有英文一类，大小写不算两类；不能作为新密码。`1234567!`有数字和特殊字符两类，且长度至少8，满足新密码组成要求。原密码保持大小写敏感，不trim；bcrypt输入仍有72字节上限，不能按72个汉字理解。

ValidPassword用于原登录/当前密码验证，ValidNewPassword用于新注册、重置和修改的新密码。把前者改为新强度规则，会让旧弱密码用户在核哈希前被拒绝。sql.ErrNoRows表示没这条身份；超时/断连表示未知，不能统一401或“密码错误”。验证码校验成功不等于一次消费，注册用户、消费记录与原回执必须同一SQL事务。

按[A01第7节](../lessons/account/01-authentication.md#7-验证先登记检查项包齐后执行)执行密码/认证/验证码分类测试。预期TestNewPasswordPolicyKeepsLegacyInputSeparate通过；真实改密还要证明旧资格关闭、新密码登录和原operation恢复，不只看页面提示。

## 2. 目录、落后支线与在线保存

第一章H2以前的段落和表格归第一章的章前单元，两个顶层H2划出两个小节；第二章保留自己的正文。chapter_id/unit_id是后端业务归属，node_id/parent_node_id是目录树关系，不能按数组相邻或排序数字猜父子关系。章前单元显示“章前内容”，不需要伪造一个H2。

落后支线用原base、当前可靠MAIN及支线修改三方合并，兼容修改保留后再按主线边界拆分。清空支线再塞MAIN会丢掉支线输入。支线改了不兼容H2边界或正文有冲突时，拒绝旧预览并走原冲突流程。

在线协调先让每个实际连接暂停输入、完成UPDATE及FLUSH，再核同room/epoch/client与原协调号的真实回执。客户端自报“保存了”或序号不算持久证明。最后一次保存可能改变原cut；即使所有人暂停，旧预览仍可能失效。新出现的零尾房间也改变固定依赖集合。临时READY不等于SQL PREPARED。

按[D04](../lessons/document/04-structure.md)和[C05](../lessons/collaboration/05-structure-online.md)的规则与真实在线测试执行，再按[页面验收表](page-acceptance.md)亲自检查章节归属、输入保留与重新预览。

## 3. 固定审核与原号重试

送审先用原FLUSH回执固定S1，Node固定capture，Document保存不可变version与round上下文；审核只能读该round的版本。之后写入S2只改变共同支线，不能改原轮次。合并使用原base、固定submitted S1和当时MAIN，安装还须保留S1之后的支线修改。

同operation绑定原actor、target、请求指纹及固定输入；再次请求读取原回执/任务，不再创建round。同号换输入应冲突。HTTP响应丢失不证明提交失败，不能换号制造第二轮。普通REVIEW不能审批自己曾提交的请求，MANAGE例外沿当前规则。

按[D05](../lessons/document/05-submission-review.md)和[D06](../lessons/document/06-merge-conflict.md)执行已有测试。实际检查S1的version/body hash→写S2→重读原round仍同hash→合并后的MAIN只纳入固定S1、共同支线仍保留S2；仅看SUCCEEDED无法发现丢字或偷换版本。

## 4. 搜索隐藏水位

索引已有seq=20的隐藏墓碑，迟到seq=19的发布应no-op；seq=20但摘要不同必须冲突；合法seq=21才可推进。若撤下时删掉整个ES记录，迟到旧发布可能把它当无历史文档重新插入。隐藏记录保留水位并清除正文；Doc当前权限与正式指针仍负责最终输出，不能用ES visible代替授权。

```powershell
Set-Location -LiteralPath $lessonRoot
go -C services/search test -mod=readonly ./internal/domain/index -run '^TestSameSequenceDifferentAuthorityCannotOverwriteAndOldCannotRevive$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '隐藏水位反例失败' }
```

应实际RUN/PASS。该纯规则不证明Canal/Kafka/ES链路；06后按[S01](../lessons/search/01-event-authority.md)的indexing真实用例核切版、撤下与墓碑，按S02核ES→SQL→Kafka顺序，未知保留同一原记录。

## 5. 固定导出与进程退出

HTTP登记固定manifest和原operation，返回任务不代表制品已完成；固定worker领取原任务，转换器生成真实Word/PDF/Markdown；下载还要重验原用户当前资格与输出来源。两个用户的多个任务共用有界工作池和预算，worker数不保证等比例吞吐。

Windows只结束直接子进程不能保证后代已退出；Job Object负责进程树，取消回调与句柄释放有顺序。取消转换不自动证明SQL、文件和预算全部回滚，原任务需要恢复落定。当前Start到绑定Job的窗口仍是边界，不能声称绝对无遗留进程。

按[D10](../lessons/document/10-exports.md)执行池配置/并发取消规则；06的formats与pool测试核真实产物、并发领取、中断原号恢复和预算归还。每份下载都检查内容、图片、长代码和摘要，202或浏览器开始下载都不是完成证据。

以上是答案和预期，不是学习者的运行记录。保存实际命令、退出码、RUN/PASS或TAP fail/skip，以及未执行项；不为练习再造一套镜像实现测试。
