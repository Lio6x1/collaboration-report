# 配置目录

整个项目的公开配置入口集中在本目录；实际运行配置由环境准备脚本生成，应用通过 `--config` 读取自己的 JSON。沿用已有 `configs` 名称，避免与新建的 `config` 目录并存。

```text
configs/
  README.md
  services/
    account.example.json         账号：数据库、Redis、邮件、验证码密钥
    document.example.json        文档：数据库、资源目录/配额、导出工具
    search.example.json          搜索：数据库、Kafka、ES、分页游标密钥
    collaboration.example.json   协作：数据库、TLS、其他服务地址
  oss/
    aliyun-storage.example.json
    aliyun-credentials.example.json
    aliyun-ram-policy.example.json
  smtp/
    qq.example.json                 QQ发信端设置；不限制注册收件地址
    credentials.example.json        私密凭据的公开占位格式
```

## 示例与实际配置

`services/*.example.json` 来自现有 `scripts/server-configuration.mjs` 的同一个生成函数，使用 Docker 服务器部署地址与纯占位值。它们用于解释完整字段，不是启动脚本直接加载的文件。`__...__` 必须由准备流程生成，迁移摘要、各服务私有库密码、TLS 证书路径和 owner 必须绑定同一部署，不能随意填写以绕过校验。

Windows 本机的实际配置放在项目 `.runtime/services/`：

```text
.runtime/services/account.json
.runtime/services/document.json
.runtime/services/search.json
.runtime/services/collaboration.json
```

现有启动脚本 `scripts/start.mjs` 会把对应文件传给各服务。Go 在 `packages/platform/store.go` 中定义和读取配置类型；Node 在 `services/collaboration/src/bootstrap/runtime.mjs` 中读取自己的配置。新增配置项时，修改类型/消费端及生成函数，保持本机和服务器生成逻辑一致；无需在业务 logic 中读取文件或写死环境地址。

Linux 的生成函数在 `scripts/server-configuration.mjs`，运行配置挂载到应用容器 `/app/.runtime/services/`。本机生成函数在 `scripts/dev-environment.mjs`，登录/搜索/资源准备流程补充相应能力的配置。部署编排文件集中在 `deploy/compose/`；Docker Compose 定义容器如何启动，服务 JSON 定义应用如何连接依赖，两者职责不同。

`exports.workers`配置固定导出工作池，默认1，范围1～8（省略或0沿用1）。本机仅调整这个字段时不必重复填写转换器路径；其余已配置字段保留。临时磁盘预算随worker数按256MiB递增，每任务预留192MiB；预算与内存是不同限制。当前部署使用默认1，修改后按[部署说明](../docs/operations/DEPLOYMENT.md)完整重启所属服务组。首次升级需应用012迁移并同步各服务版本绑定；示例的迁移字段仍为占位，准备脚本从完整迁移链生成001～012的实际摘要，不能只填写第12版。

## 私密内容

- 实际服务 JSON 的 DSN、Redis 密码等可能含秘密，留在被 Git 忽略的 `.runtime/`，不要复制到公开示例或提交。
- OSS AccessKey 放在 `.runtime/private/oss-credentials.json`；服务配置只引用凭据文件路径。账号信息、证书私钥及其他部署秘密也保留在私密运行目录。
- SMTP凭据放在 `.runtime/private/smtp-credentials.json`，字段为 `username`（完整发件邮箱或SMTP用户名）和 `password`（邮箱服务的SMTP授权码）。不要填普通登录密码，也不要把真实值写到 `configs/`、命令行或聊天。
- `configs/oss/aliyun-credentials.example.json` 只有字段说明占位值，不包含真正密钥。`configs/oss/aliyun-storage.example.json` 是存储参数示例，实际绑定以自己的运行配置为准。
- 本次补充示例没有读取、覆盖或重生成现有运行配置。体验实例继续按已有配置运行。

学习与部署入口：[本机体验](../docs/operations/LOCAL-EXPERIENCE.md)、[OSS 配置](../docs/operations/OSS.md)、[Linux 部署](../docs/operations/LINUX.md)、[学习版备份与恢复](../docs/tutorial/continuous/reference/backup-and-restore.md)。

## SMTP设置

`smtp.security` 支持 `tls`（连接即TLS）和 `starttls`（先升级TLS再认证）；两者都核验服务器证书，支持 PLAIN/LOGIN。`credentialsFile` 必须是本机绝对路径。`caFile` 可选，用于信任私有SMTP的CA证书，不提供关闭证书校验的选项。仅数值loopback可使用 `development` 无认证明文收件器；旧的空 `security` 只保留这一兼容方式。

Windows本机在当前项目目录中，先按两个SMTP示例创建私密凭据文件与设置文件，并把QQ示例的发件地址、项目绝对路径改成自己的值。账号服务必须已停止，再执行 `node scripts/configure-smtp.mjs --settings "D:/你的项目/.runtime/private/smtp-settings.json"`。脚本只替换原 `account.json` 的 `smtp` 段，并收紧凭据文件ACL；它不读取授权码内容、不改DSN/owner、不试发邮件、不启停服务。按原启动方式重新加载后，验证码响应才会使用新的投递模式。环境准备脚本会保留已有SMTP段。

QQ示例使用 `smtp.qq.com:465` 和 `security: "tls"`，见[腾讯云SMTP配置说明](https://intl.cloud.tencent.com/zh/document/product/1266/71700)。邮箱端须开启SMTP并生成授权码，参照[QQ邮箱授权码说明](https://help.mail.qq.com/detail/106/985)。QQ只作为发信账户，注册、找回和改邮箱的收件地址沿现有通用邮箱校验，不限QQ。
