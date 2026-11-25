# Windows本机体验：先使用现成完整产品

2026-10-11再次健康检查返回`SERVER_READY`，入口：**https://localhost:20609**。原产品目录为`D:/job/golang/projects/collaboration-report`，原owner、数据库、对象存储绑定和数据保留；MySQL、Redis、Kafka、Elasticsearch、Canal全部是原项目的Docker实例。四业务服务和边缘入口在Windows运行，用户自己的后端学习目录另外创建。近期修改与教程同步见[交付核对](../delivery/CURRENT-REVIEW.md)。

## 登录并开始

1. 用Edge或Chrome打开上述地址。本机使用开发自签证书，首次浏览器可能显示证书提示；确认地址确实是localhost:20609后进入本机页面。服务器受信证书在后续正式部署时设置。
2. 用编辑器打开本机`.runtime/private/local-experience-account.json`，将其中`email`和`password`分别填入“登录邮箱”“登录密码”。这是专门创建的本机体验账号，口令没有写进公开教程或运行日志。
3. 工作台选择“个人空间”，可以看到“我的协作报告体验”（文档ID19291）。点击标题打开，也可以输入自己的标题创建新文档。这个样本保留给你使用，不会在验收后自动删除。
4. 若要用自己的邮箱注册，切到“注册”、发送验证码，并按页面提示完成确认密码和注册。真实SMTP模式下到收件邮箱查收；只有自己的实例配置为开发收件器时，才在项目目录执行`./scripts/show-mail.ps1 -Email '你填写的邮箱' -Limit 1`读取开发邮件。是否真实投递取决于该实例的邮件配置，不能把开发收件器的方式套到已绑定真实发信邮箱的实例。

主控已实际通过HTTP登录、本人接口及真实Edge登录，等个人空间与样本文档加载后截图，0个页面脚本错误。这是直接浏览器检查，不将Node输出的“0 tests”冒称额外测试数量。实际记录在`artifacts/batches/09/local-experience.json`及`local-experience.png`。

## 建议的体验顺序

先创建个人报告，打开共同支线，写一小段文字并观察服务端保存状态，再刷新确认正文。编辑在当前标签切换工作区，保存后返回阅读。继续体验个人支线、拉主线、固定送审、审核合并与批注。多人体验可以在同一浏览器的不同标签分别主动登录不同账号；当前前端按标签会话上下文选择HttpOnly Cookie。复制标签可能复制会话上下文，下一次主动登录会轮换；也可使用独立浏览器配置或匿名窗口。具体边界见[登录态接入说明](../tutorial/frontend-integration.md)。

团队功能从顶部“组织”创建自己的体验组织和知识库，再通过申请/邀请加入另一个账号并分配合法编辑、审核范围。只看到某个按钮不表示永久有权操作；可选范围来自当前权限。正式版搜索需要先由管理者发布有权公开的固定版本，消息消费后才能搜索到。

固定主线或正式版可以导出DOCX、Markdown资源包和PDF；等待原作业完成再下载。回收后从回收站恢复，恢复语义为私有、未发布。永久删除、账号注销、组织关闭只对自己的专用样本执行，它们会真实清理数据。完整演示步骤见[演示指南](../tutorial/demo.md)。

## 电脑重启后的重新启动

保持Docker Desktop运行，在原项目根目录复用原owner和配置，不删除.runtime、不重新初始化数据库。先沿00章选择Go1.25.10、Node24.19.0、OpenSSL、Pandoc3.8的实际PATH；既有前端已经构建。

```powershell
Set-Location -LiteralPath 'D:/job/golang/projects/collaboration-report'
# 核发行配置，保留原OSS/账号/证书绑定。
node --no-maglev scripts/release.mjs check
if ($LASTEXITCODE -ne 0) { throw '先根据原错误排查，不清空数据' }
node --no-maglev --input-type=module -e "import { ownedContainer, compose } from './scripts/runtime-lib.mjs'; for (const role of ['mysql','redis','kafka','es','canal']) ownedContainer(role); compose('up','-d','mysql','redis','kafka','es','canal');"
if ($LASTEXITCODE -ne 0) { throw '原Docker依赖未启动' }
node --no-maglev scripts/start.mjs
if ($LASTEXITCODE -ne 0) { throw '整站未确认READY，保留原日志和进程现场' }
```

以实际READY输出URL为准。这个步骤不改原服务配置，不重跑首次dev/login/search配置生成。需要暂停应用时用`node scripts/start.mjs stop`，卷和账号仍保留；常规操作不要`down -v`。

体验之后，从[完整后端教程入口](../tutorial/continuous/README.md)学习。你先按教程从零写自己的后端、复用现成前端，在本机测试自己的版本；全部正常后再按[Linux正常完整Docker方案](LINUX.md)部署Ubuntu24.04服务器。当前现成产品和教师学习副本的验收都不能代替你自己的学习版本验收。
