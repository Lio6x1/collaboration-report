# 分步编写 `packages/platform/store.go`

先按 [F03 · 配置、连接池、就绪与安全出口](../lessons/foundation/03-runtime-and-security.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/031-store.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

加载服务自身配置，创建有界数据库池，并以迁移和恢复事实判定就绪。

Ping只证明连接可用，不能证明版本正确或旧任务安全接管；应用只持本服务DSN。

写之前先按这条执行路线理解各段的连接：

1. OpenStore创建实例拥有的sql.DB并包装sqlx，最大/空闲连接均8，2秒Ping失败即关闭。
2. CheckReady精确比较schema_migration版本及SHA，再拒绝未接管非终态任务与未完成gate。
3. ManagedTasks仅在ReadyExtra能核恢复事实时排除基础拒绝，最后仍执行ReadyExtra。

本文件需要守住的边界：

- 不能复用全局同DSN池导致旧实例关闭新实例；不能把ManagedTasks登记当作免检清单。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/store.go') -and (Get-Item -LiteralPath 'packages/platform/store.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/store.go')) { New-Item -ItemType File -Path 'packages/platform/store.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:167 -->
````go
package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

````

## 第 2 步：Config

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

Config每个进程只读取自己的DSN与证书，不把迁移/root账号注入应用服务。

本步声明：`type Config struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Role</code> | <code>string</code> | 当前服务角色account/document/search/collaboration，Main核对与入口一致。<br>序列化/校验标记：<code>`json:&quot;role&quot;`</code> |
| <code>Host</code> | <code>string</code> | 本机监听地址，配合HTTPPort/RPCPort生成服务地址。<br>序列化/校验标记：<code>`json:&quot;host&quot;`</code> |
| <code>HTTPPort</code> | <code>int</code> | HTTPS业务与健康检查端口。<br>序列化/校验标记：<code>`json:&quot;httpPort&quot;`</code> |
| <code>RPCPort</code> | <code>int</code> | mTLS gRPC监听端口。<br>序列化/校验标记：<code>`json:&quot;rpcPort&quot;`</code> |
| <code>DSN</code> | <code>string</code> | 当前服务私有数据库连接串，含实例凭据，不得输出。<br>序列化/校验标记：<code>`json:&quot;dsn&quot;`</code> |
| <code>CertFile</code> | <code>string</code> | 本服务TLS证书文件路径，叶证书含精确服务URI。<br>序列化/校验标记：<code>`json:&quot;certFile&quot;`</code> |
| <code>KeyFile</code> | <code>string</code> | 与CertFile配对的私钥路径，只由进程加载。<br>序列化/校验标记：<code>`json:&quot;keyFile&quot;`</code> |
| <code>CAFile</code> | <code>string</code> | 验证内部服务证书的CA文件路径。<br>序列化/校验标记：<code>`json:&quot;caFile&quot;`</code> |
| <code>ExpectedSQLSHA</code> | <code>string</code> | 001基线原字节的SHA256十六进制值，CheckReady核对数据库记录。<br>序列化/校验标记：<code>`json:&quot;expectedSQLSHA&quot;`</code> |
| <code>ExpectedAdditionalMigrations</code> | <code>map&#91;int&#93;string</code> | 版本号到追加迁移SHA的可信发布映射，要求连续且无缺项。<br>序列化/校验标记：<code>`json:&quot;expectedAdditionalMigrations&quot;`</code> |
| <code>Origin</code> | <code>string</code> | 允许浏览器写入的精确Origin，用于防跨站。<br>序列化/校验标记：<code>`json:&quot;origin&quot;`</code> |
| <code>Root</code> | <code>string</code> | 项目或实例装配根目录，构造运行文件位置，不作为用户输入路径。<br>序列化/校验标记：<code>`json:&quot;root&quot;`</code> |
| <code>Peers</code> | <code>map&#91;string&#93;string</code> | 服务角色到RPC/内部HTTP地址的映射，例如document与documentHttp。<br>序列化/校验标记：<code>`json:&quot;peers&quot;`</code> |
| <code>Redis</code> | <code>*RedisConfig</code> | Account登录限流与验证码存储配置；为空时认证子功能未装配。<br>序列化/校验标记：<code>`json:&quot;redis&quot;`</code> |
| <code>SMTP</code> | <code>*SMTPConfig</code> | 验证码邮件通道配置；不是密码原文容器。<br>序列化/校验标记：<code>`json:&quot;smtp&quot;`</code> |
| <code>ChallengeKey</code> | <code>string</code> | 32字节挑战HMAC密钥的hex配置，亦派生邀请加密键。<br>序列化/校验标记：<code>`json:&quot;challengeKey&quot;`</code> |
| <code>Search</code> | <code>*SearchConfig</code> | Search所需Kafka/ES、索引及游标签名设置。<br>序列化/校验标记：<code>`json:&quot;search&quot;`</code> |
| <code>Resources</code> | <code>*ResourceConfig</code> | Document资源对象存储和额度配置。<br>序列化/校验标记：<code>`json:&quot;resources&quot;`</code> |
| <code>Exports</code> | <code>*ExportConfig</code> | Document导出工作目录、转换器路径与worker数。<br>序列化/校验标记：<code>`json:&quot;exports&quot;`</code> |

<!-- write-step: 2 bytes=167:1539 -->
````go
// Config每个进程只读取自己的DSN与证书，不把迁移/root账号注入应用服务。
type Config struct {
	Role                         string            `json:"role"`
	Host                         string            `json:"host"`
	HTTPPort                     int               `json:"httpPort"`
	RPCPort                      int               `json:"rpcPort"`
	DSN                          string            `json:"dsn"`
	CertFile                     string            `json:"certFile"`
	KeyFile                      string            `json:"keyFile"`
	CAFile                       string            `json:"caFile"`
	ExpectedSQLSHA               string            `json:"expectedSQLSHA"`
	ExpectedAdditionalMigrations map[int]string    `json:"expectedAdditionalMigrations"`
	Origin                       string            `json:"origin"`
	Root                         string            `json:"root"`
	Peers                        map[string]string `json:"peers"`
	Redis                        *RedisConfig      `json:"redis"`
	SMTP                         *SMTPConfig       `json:"smtp"`
	ChallengeKey                 string            `json:"challengeKey"`
	Search                       *SearchConfig     `json:"search"`
	Resources                    *ResourceConfig   `json:"resources"`
	Exports                      *ExportConfig     `json:"exports"`
}

````

## 第 3 步：ExportConfig

插入位置：第 2 步代码之后，紧接追加；完成稿约第 39 行。

转换程序路径显式装配，业务层不依赖开发者机器的安装目录。

本步声明：`type ExportConfig struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Root</code> | <code>string</code> | 导出临时/最终制品受控根目录。<br>序列化/校验标记：<code>`json:&quot;root&quot;`</code> |
| <code>Pandoc</code> | <code>string</code> | 显式配置的Pandoc可执行文件路径。<br>序列化/校验标记：<code>`json:&quot;pandoc&quot;`</code> |
| <code>Node</code> | <code>string</code> | 导出渲染过程使用的Node可执行文件。<br>序列化/校验标记：<code>`json:&quot;node&quot;`</code> |
| <code>Browser</code> | <code>string</code> | 导出PDF等渲染过程的浏览器可执行文件。<br>序列化/校验标记：<code>`json:&quot;browser&quot;`</code> |
| <code>Workers</code> | <code>int</code> | 导出并发工作数，0沿用单worker；实际预算按256MiB乘worker校验。<br>序列化/校验标记：<code>`json:&quot;workers&quot;`</code> |

<!-- write-step: 3 bytes=1539:1901 -->
````go
// 转换程序路径显式装配，业务层不依赖开发者机器的安装目录。
type ExportConfig struct {
	Root    string `json:"root"`
	Pandoc  string `json:"pandoc"`
	Node    string `json:"node"`
	Browser string `json:"browser"`
	// 0沿用默认单worker；并发及临时磁盘总预算由文档服务一起校验。
	Workers int `json:"workers"`
}
````

## 第 4 步：ResourceConfig

插入位置：第 3 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`type ResourceConfig struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Root</code> | <code>string</code> | 本机对象、锁和校验临时文件的受控根目录，启用OSS后仍需它。<br>序列化/校验标记：<code>`json:&quot;root&quot;`</code> |
| <code>OSS</code> | <code>*ResourceOSSConfig</code> | 可选OSS对象IO配置，未配置时走本地对象存储。<br>序列化/校验标记：<code>`json:&quot;oss,omitempty&quot;`</code> |
| <code>SpaceBytes</code> | <code>uint64</code> | 每个空间可用的资源/业务正文容量上限字节。<br>序列化/校验标记：<code>`json:&quot;spaceBytes&quot;`</code> |
| <code>UploadBytes</code> | <code>uint64</code> | 单次资源上传允许的最大字节数。<br>序列化/校验标记：<code>`json:&quot;uploadBytes&quot;`</code> |
| <code>TemporaryBytes</code> | <code>uint64</code> | 并发上传临时文件与预留总预算。<br>序列化/校验标记：<code>`json:&quot;temporaryBytes&quot;`</code> |
| <code>UploadTTLSeconds</code> | <code>int</code> | 上传预留的有效秒数，到期需按任务回收而非继续写入。<br>序列化/校验标记：<code>`json:&quot;uploadTTLSeconds&quot;`</code> |

<!-- write-step: 4 bytes=1901:2283 -->
````go
type ResourceConfig struct {
	Root             string             `json:"root"`
	OSS              *ResourceOSSConfig `json:"oss,omitempty"`
	SpaceBytes       uint64             `json:"spaceBytes"`
	UploadBytes      uint64             `json:"uploadBytes"`
	TemporaryBytes   uint64             `json:"temporaryBytes"`
	UploadTTLSeconds int                `json:"uploadTTLSeconds"`
}

````

## 第 5 步：ResourceOSSConfig

插入位置：第 4 步代码之后，紧接追加；完成稿约第 57 行。

OSS 只替换对象 IO；Root 仍是本机共享锁、校验临时文件与既有本地对象的目录。
私密配置只保存凭据文件路径，AK/SK 不进入服务配置或日志。

本步声明：`type ResourceOSSConfig struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Region</code> | <code>string</code> | 对象存储Bucket所在区域，用于请求签名与端点配置。<br>序列化/校验标记：<code>`json:&quot;region&quot;`</code> |
| <code>Endpoint</code> | <code>string</code> | OSS服务端点，供受控对象客户端连接。<br>序列化/校验标记：<code>`json:&quot;endpoint&quot;`</code> |
| <code>Bucket</code> | <code>string</code> | 本实例使用的对象桶名称。<br>序列化/校验标记：<code>`json:&quot;bucket&quot;`</code> |
| <code>Prefix</code> | <code>string</code> | 本实例对象键前缀，隔离不同部署的对象。<br>序列化/校验标记：<code>`json:&quot;prefix&quot;`</code> |
| <code>CredentialsFile</code> | <code>string</code> | 本地私密AK/SK文件路径，配置本身不携带明文AK/SK。<br>序列化/校验标记：<code>`json:&quot;credentialsFile&quot;`</code> |

<!-- write-step: 5 bytes=2283:2722 -->
````go
// OSS 只替换对象 IO；Root 仍是本机共享锁、校验临时文件与既有本地对象的目录。
// 私密配置只保存凭据文件路径，AK/SK 不进入服务配置或日志。
type ResourceOSSConfig struct {
	Region          string `json:"region"`
	Endpoint        string `json:"endpoint"`
	Bucket          string `json:"bucket"`
	Prefix          string `json:"prefix"`
	CredentialsFile string `json:"credentialsFile"`
}
````

## 第 6 步：SearchConfig

插入位置：第 5 步代码之后，紧接追加；完成稿约第 66 行。

本步声明：`type SearchConfig struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Owner</code> | <code>string</code> | 本部署搜索制品和物理索引的归属标识。<br>序列化/校验标记：<code>`json:&quot;owner&quot;`</code> |
| <code>Brokers</code> | <code>&#91;&#93;string</code> | Kafka broker地址列表。<br>序列化/校验标记：<code>`json:&quot;brokers&quot;`</code> |
| <code>ElasticEndpoint</code> | <code>string</code> | Elasticsearch HTTP服务地址。<br>序列化/校验标记：<code>`json:&quot;elasticEndpoint&quot;`</code> |
| <code>Topic</code> | <code>string</code> | 搜索事件Kafka主题名；运行时还需固定topic物理ID。<br>序列化/校验标记：<code>`json:&quot;topic&quot;`</code> |
| <code>Group</code> | <code>string</code> | Kafka消费组身份，决定消费位置归属。<br>序列化/校验标记：<code>`json:&quot;group&quot;`</code> |
| <code>Alias</code> | <code>string</code> | 对外查询别名，由重建切换到物理索引。<br>序列化/校验标记：<code>`json:&quot;alias&quot;`</code> |
| <code>InitialIndex</code> | <code>string</code> | 首次初始化使用的物理索引名，不可与alias混用。<br>序列化/校验标记：<code>`json:&quot;initialIndex&quot;`</code> |
| <code>CursorKey</code> | <code>string</code> | 搜索游标HMAC签名密钥配置。<br>序列化/校验标记：<code>`json:&quot;cursorKey&quot;`</code> |
| <code>CursorTTLSeconds</code> | <code>int</code> | 搜索游标可恢复的有效秒数。<br>序列化/校验标记：<code>`json:&quot;cursorTTLSeconds&quot;`</code> |

<!-- write-step: 6 bytes=2722:3163 -->
````go
type SearchConfig struct {
	Owner            string   `json:"owner"`
	Brokers          []string `json:"brokers"`
	ElasticEndpoint  string   `json:"elasticEndpoint"`
	Topic            string   `json:"topic"`
	Group            string   `json:"group"`
	Alias            string   `json:"alias"`
	InitialIndex     string   `json:"initialIndex"`
	CursorKey        string   `json:"cursorKey"`
	CursorTTLSeconds int      `json:"cursorTTLSeconds"`
}
````

## 第 7 步：SMTPConfig

插入位置：第 6 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`type SMTPConfig struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Address</code> | <code>string</code> | SMTP host:port，不是HTTP URL。<br>序列化/校验标记：<code>`json:&quot;address&quot;`</code> |
| <code>From</code> | <code>string</code> | 完整且无头注入字符的发件邮箱。<br>序列化/校验标记：<code>`json:&quot;from&quot;`</code> |
| <code>Security</code> | <code>string</code> | development/tls/starttls模式；development仅回环数值IP且不发凭据。<br>序列化/校验标记：<code>`json:&quot;security,omitempty&quot;`</code> |
| <code>CredentialsFile</code> | <code>string</code> | 远程SMTP用户名/授权码的本机绝对JSON路径。<br>序列化/校验标记：<code>`json:&quot;credentialsFile,omitempty&quot;`</code> |
| <code>CAFile</code> | <code>string</code> | 可选额外SMTP信任CA，不复用业务mTLS身份且不关闭系统校验。<br>序列化/校验标记：<code>`json:&quot;caFile,omitempty&quot;`</code> |

<!-- write-step: 7 bytes=3163:3610 -->
````go
type SMTPConfig struct {
	Address string `json:"address"`
	From    string `json:"from"`
	// 空Security只兼容旧的数值loopback开发收件器；远程必须明确选择tls或starttls。
	// 授权码只在私密JSON文件中，配置和HTTP响应均不携带用户名/密码。
	Security        string `json:"security,omitempty"`
	CredentialsFile string `json:"credentialsFile,omitempty"`
	CAFile          string `json:"caFile,omitempty"`
}
````

## 第 8 步：RedisConfig

插入位置：第 7 步代码之后，紧接追加；完成稿约第 86 行。

本步声明：`type RedisConfig struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Host</code> | <code>string</code> | Redis host:port，供验证码与登录Lua执行。<br>序列化/校验标记：<code>`json:&quot;host&quot;`</code> |
| <code>User</code> | <code>string</code> | Redis ACL用户。<br>序列化/校验标记：<code>`json:&quot;user&quot;`</code> |
| <code>Password</code> | <code>string</code> | Redis实例凭据，仅进程使用，禁止日志或教程复制。<br>序列化/校验标记：<code>`json:&quot;password&quot;`</code> |

<!-- write-step: 8 bytes=3610:3736 -->
````go
type RedisConfig struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password"`
}

````

## 第 9 步：LoadConfig

插入位置：第 8 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`func LoadConfig(file string) (Config, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>file</code> | <code>string</code> | 在此声明中的实际使用：<code>os.ReadFile(file)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Config</code> | 使用 <code>Config</code> 类型；其定义处的职责：Config每个进程只读取自己的DSN与证书，不把迁移/root账号注入应用服务。<br>实际返回表达式：<code>cfg</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SERVICE_CONFIG_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SERVICE_CONFIG_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return cfg, errors.New(&quot;SERVICE_CONFIG_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;cfg) != nil</code>。条件成立时执行 <code>return cfg, errors.New(&quot;SERVICE_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return cfg, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3736:4018 -->
````go
func LoadConfig(file string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(file)
	if err != nil {
		return cfg, errors.New("SERVICE_CONFIG_UNAVAILABLE")
	}
	if json.Unmarshal(raw, &cfg) != nil {
		return cfg, errors.New("SERVICE_CONFIG_INVALID")
	}
	return cfg, nil
}

````

## 第 10 步：Store

插入位置：第 9 步代码之后，紧接追加；完成稿约第 104 行。

本步声明：`type Store struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Context</code> | <code>context.Context</code> | 服务生命周期context，退出时取消所有恢复worker。 |
| <code>ReadyExtra</code> | <code>func(context.Context) error</code> | 业务组合层的恢复一致性屏障；nil时不得登记ManagedTasks豁免。 |
| <code>ManagedTasks</code> | <code>&#91;&#93;string</code> | 本实例已实际装配并由ReadyExtra验证的持久任务种类。 |
| <code>Role</code> | <code>string</code> | 当前服务角色，决定是否存在gate及特定就绪检查。 |
| <code>Conn</code> | <code>sqlx.SqlConn</code> | 包装同DB池的go-zero SQL接口；不能另开不相关池。 |
| <code>DB</code> | <code>*sql.DB</code> | 实例拥有的最多8连接sql.DB，所有本库事务由此开始。 |
| <code>ExpectedSQLSHA</code> | <code>string</code> | 可信001基线摘要副本。 |
| <code>ExpectedAdditionalMigrations</code> | <code>map&#91;int&#93;string</code> | 复制的追加迁移期望集合，防配置map被后续修改。 |

<!-- write-step: 10 bytes=4018:4470 -->
````go
type Store struct {
	Context    context.Context
	ReadyExtra func(context.Context) error
	// 仅由启动装配登记已接管的持久流程；其事实完整性由 ReadyExtra 核验，未知种类仍拒绝。
	ManagedTasks                 []string
	Role                         string
	Conn                         sqlx.SqlConn
	DB                           *sql.DB
	ExpectedSQLSHA               string
	ExpectedAdditionalMigrations map[int]string
}

````

## 第 11 步：OpenStore

插入位置：第 10 步代码之后，紧接追加；完成稿约第 116 行。

本步声明：`func OpenStore(cfg Config) (*Store, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>Config</code> | 使用 <code>Config</code> 类型；其定义处的职责：Config每个进程只读取自己的DSN与证书，不把迁移/root账号注入应用服务。<br>在此声明中的实际使用：<code>cfg.DSN</code>；<code>cfg.Role</code>；<code>cfg.ExpectedSQLSHA</code>；<code>cfg.ExpectedAdditionalMigrations</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Store</code> | 实际返回表达式：<code>nil</code>；<code>&amp;Store{ 		Context:                      context.Background(), 		Role:                         cfg.Role, 		Conn:                         conn, 		DB:                           db, 		 …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;DATABASE_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;DATABASE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(context.Background(), 2*time.Second)</code>。
- 登记 <code>done()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>db.PingContext(ctx) != nil</code>。
- 此处通过 <code>return nil, errors.New(&quot;DATABASE_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &amp;Store{ 		Context:                      context.Background(), 		Role:                         cfg.Role, 		Conn:                         conn, 		DB:                           …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4470:5787 -->
````go
func OpenStore(cfg Config) (*Store, error) {
	// go-zero 的 NewMysql 按 DSN 共享全局连接池；这里由服务实例拥有池的生命周期，
	// 避免一个实例退出后把同 DSN 的新实例也变成“连接池已关闭”。查询仍使用 go-zero sqlx。
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, errors.New("DATABASE_UNAVAILABLE")
	}
	conn := sqlx.NewSqlConnFromDB(db)
	db.SetMaxOpenConns(8)
	// 后台领取者会短时同时使用8个连接。空闲上限低于总上限会在每轮归还时
	// 主动关闭健康连接，下一轮重新建连，长期耗尽Windows临时TCP端口。
	// 保留同一个有界池的全部连接，不增加并发上限，也不改变查询或事务期限。
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, done := context.WithTimeout(context.Background(), 2*time.Second)
	defer done()
	if db.PingContext(ctx) != nil {
		_ = db.Close()
		return nil, errors.New("DATABASE_UNAVAILABLE")
	}
	return &Store{
		Context:                      context.Background(),
		Role:                         cfg.Role,
		Conn:                         conn,
		DB:                           db,
		ExpectedSQLSHA:               cfg.ExpectedSQLSHA,
		ExpectedAdditionalMigrations: maps.Clone(cfg.ExpectedAdditionalMigrations),
	}, nil
}
````

## 第 12 步：Store.CheckReady

插入位置：第 11 步代码之后，紧接追加；完成稿约第 145 行。

本步声明：`func (s *Store) CheckReady(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.DB.QueryContext(ctx, &quot;SELECT version,LOWER(HEX(checksum)) FROM schema_migration&quot;)</code>；<code>s.DB.QueryRowContext(ctx, query, args...)</code>；<code>s.DB.QueryRowContext(ctx, gateQuery)</code>；<code>s.ReadyExtra(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;DATABASE_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SCHEMA_VERSION_MISMATCH&quot;)</code>；<code>errors.New(&quot;RECOVERY_BARRIER_REQUIRED&quot;)</code>；<code>errors.New(&quot;RECOVERY_REQUIRED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.DB.QueryContext(ctx, &quot;SELECT version,LOWER(HEX(checksum)) FROM schema_migration&quot;)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;DATABASE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;version, &amp;checksum) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SCHEMA_VERSION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;version, &amp;checksum)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>duplicate</code>。条件成立时执行 <code>return errors.New(&quot;SCHEMA_VERSION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return errors.New(&quot;DATABASE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>!matchesSchemaMigrations(s.ExpectedSQLSHA, s.ExpectedAdditionalMigrations, installed)</code>。条件成立时执行 <code>return errors.New(&quot;SCHEMA_VERSION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>s.ExpectedAdditionalMigrations&#91;2&#93; != &quot;&quot;</code>。
- 检查 <code>s.Role == &quot;document&quot; &amp;&amp; s.ReadyExtra != nil</code>。
- 检查 <code>len(s.ManagedTasks) &gt; 0 &amp;&amp; s.ReadyExtra == nil</code>。条件成立时执行 <code>return errors.New(&quot;RECOVERY_BARRIER_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(excluded) &gt; 0</code>。

<!-- write-step: 12 bytes=5787:7609 -->
````go
func (s *Store) CheckReady(ctx context.Context) error {
	// 连通不代表可接业务：版本、原始迁移摘要和恢复事实必须同时成立。
	var version int
	var checksum string
	rows, err := s.DB.QueryContext(ctx, "SELECT version,LOWER(HEX(checksum)) FROM schema_migration")
	if err != nil {
		return errors.New("DATABASE_UNAVAILABLE")
	}
	defer rows.Close()
	installed := map[int]string{}
	for rows.Next() {
		if rows.Scan(&version, &checksum) != nil {
			return errors.New("SCHEMA_VERSION_MISMATCH")
		}
		if _, duplicate := installed[version]; duplicate {
			return errors.New("SCHEMA_VERSION_MISMATCH")
		}
		installed[version] = checksum
	}
	if rows.Err() != nil {
		return errors.New("DATABASE_UNAVAILABLE")
	}
	if !matchesSchemaMigrations(s.ExpectedSQLSHA, s.ExpectedAdditionalMigrations, installed) {
		return errors.New("SCHEMA_VERSION_MISMATCH")
	}
	var pending int
	query := "SELECT COUNT(*) FROM durable_task WHERE state NOT IN ('SUCCEEDED','REJECTED')"
	if s.ExpectedAdditionalMigrations[2] != "" {
		// 002提供覆盖索引；只读state/kind，容量写入不能迫使基础检查载入JSON聚簇页。
		query = "SELECT COUNT(*) FROM durable_task FORCE INDEX (task_pending) WHERE state NOT IN ('SUCCEEDED','REJECTED')"
	}
	var excluded []string
	if s.Role == "document" && s.ReadyExtra != nil {
		excluded = append(excluded, "OUTPUT_LEASE")
	} // 当前输出由实例恢复屏障核验，不能误当未恢复业务任务。
	if len(s.ManagedTasks) > 0 && s.ReadyExtra == nil {
		return errors.New("RECOVERY_BARRIER_REQUIRED")
	}
	excluded = append(excluded, s.ManagedTasks...)
	var args []any
	if len(excluded) > 0 {
		query += " AND kind NOT IN (" + strings.TrimSuffix(strings.Repeat("?,", len(excluded)), ",") + ")"
		for _, kind := range excluded {
			args = append(args, kind)
		}
	}
````

## 第 13 步：Store.CheckReady（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 191 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>s.DB.QueryRowContext(ctx, query, args...).Scan(&amp;pending) != nil</code>。条件成立时执行 <code>return errors.New(&quot;DATABASE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.DB.QueryRowContext(ctx, query, args...).Scan(&amp;pending)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>s.DB.QueryRowContext(ctx, query, args...)</code>。
- 检查 <code>pending != 0</code>。条件成立时执行 <code>return errors.New(&quot;RECOVERY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>s.Role != &quot;search&quot;</code>。
- 检查 <code>s.ExpectedAdditionalMigrations&#91;2&#93; != &quot;&quot;</code>。
- 检查 <code>s.DB.QueryRowContext(ctx, gateQuery).Scan(&amp;pending) != nil</code>。条件成立时执行 <code>return errors.New(&quot;DATABASE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.DB.QueryRowContext(ctx, gateQuery).Scan(&amp;pending)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>s.DB.QueryRowContext(ctx, gateQuery)</code>。
- 检查 <code>pending != 0</code>。条件成立时执行 <code>return errors.New(&quot;GATE_RECOVERY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>s.ReadyExtra != nil</code>。条件成立时执行 <code>return s.ReadyExtra(ctx)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=7609:8888 -->
````go
	if s.DB.QueryRowContext(ctx, query, args...).Scan(&pending) != nil {
		return errors.New("DATABASE_UNAVAILABLE")
	}
	// 未接管流程保持503。已装配恢复器允许正常排队，但最后仍须通过各自事实屏障。
	if pending != 0 {
		return errors.New("RECOVERY_REQUIRED")
	}
	// search没有gate表；其他服务的冻结事实在恢复器实现前必须保持不可用。
	if s.Role != "search" {
		gateQuery := "SELECT COUNT(*) FROM access_gate g LEFT JOIN authorization_change c ON c.revocation_id=g.last_revocation_id AND c.scope_key=g.scope_key AND c.epoch>=g.max_epoch WHERE g.state='BLOCKED' AND (c.state IS NULL OR c.state<>'SUCCEEDED')"
		if s.ExpectedAdditionalMigrations[2] != "" {
			gateQuery = "SELECT COUNT(*) FROM access_gate g FORCE INDEX (gate_recovery) LEFT JOIN authorization_change c FORCE INDEX (auth_recovery) ON c.revocation_id=g.last_revocation_id AND c.scope_key=g.scope_key AND c.epoch>=g.max_epoch WHERE g.state='BLOCKED' AND (c.state IS NULL OR c.state<>'SUCCEEDED')"
		}
		if s.DB.QueryRowContext(ctx, gateQuery).Scan(&pending) != nil {
			return errors.New("DATABASE_UNAVAILABLE")
		}
		if pending != 0 {
			return errors.New("GATE_RECOVERY_REQUIRED")
		}
	}
	if s.ReadyExtra != nil {
		return s.ReadyExtra(ctx)
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/store.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/03-runtime-and-security.md)。
