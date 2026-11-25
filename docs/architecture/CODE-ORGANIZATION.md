# 当前代码组织与维护边界

## 四服务与职责

账号、文档、搜索为Go模块；协作为Node服务。各自持有私有数据库，跨服务通过明确HTTP/RPC调用，不直接读取其他服务的库。浏览器经同源HTTPS入口访问。

Go调用方向为 handler/server → logic → domain及repository/client；bootstrap、svc组合依赖。handler解析输入和映射错误，logic决定权限及事务，repository集中SQL，domain保存业务模型与纯规则。接口由实际消费者定义，避免为每个struct建立空接口。main只接启动。HTTP DTO和RPC以contracts生成物为准。

Node按transport、application、domain、repository、worker、client、bootstrap组织。Vue按features组织页面、composable、API适配和纯规则。配置示例在configs，真实实例配置与凭据在忽略的.runtime。

## 不可在重构中改变的行为

- 同一业务流程保持内聚，不为每个小函数新增文件。目录和接口只为真实职责服务。
- 用例控制事务；repository接受同一连接/事务，不能偷偷提交或另开池完成半个事务。
- 真实身份、权限来源、锁顺序、固定版本、幂等结果与未知结果恢复保持；不能靠超时猜成功。
- UPDATE按候选隔离验证、文档保护、Node同事务日志/确认任务、提交、公共内存/广播、SAVED的顺序执行。Go不解析Yjs二进制。
- FLUSH确认原输入已持久；审核首次Capture固定送审内容，后续输入属于之后的内容。
- worker只调度并复用原业务恢复逻辑；导出池限制并发，外部进程取消与预算归还沿原作业。
- errors.Is/As保留可判断原因；对外只返回稳定契约和面向用户的提示，不能泄露驱动、SQL或凭据。
- Vue迟到响应需核当前账号、组织或文档代次；分页首页不能被误当完整权限目录。

## 可读性与检查

关键事务、权限、幂等、保存确认与跨服务恢复用中文说明设计原因；不逐行翻译赋值。正常缩进、明确命名，Go用gofmt，JS/TS/Vue用锁定Prettier。生成物由生成器维护。

在项目根执行 `./scripts/verify-quality.ps1 -Tutorial` 检查静态分析、格式、离线规则、前端构建及教材。真实接口/存储/WSS测试按[课程06](../tutorial/continuous/06-integration.md)在独立实例串行执行，不能改体验数据制造通过。

增改业务时同步[当前全量课程](../tutorial/continuous/README.md)。先修正功能讲解，再发布源码/支持及分步页，最后核对导航与实际行为。当前结论、风险与延期需求见[审查记录](../delivery/CURRENT-REVIEW.md)。
