# 分步编写 `services/document/internal/storage/oss/store.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/137-store.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

装配官方OSS客户端并固定Bucket/prefix身份与本地锁目录。

当前方案仍是单机文件锁协调，使用云存储并不自动变成多机安全部署。

写之前先按这条执行路线理解各段的连接：

1. validConfig仅接受区域对应官方HTTPS端点
2. New读取私有凭据、关闭SDK日志和长重试
3. BindBackend固定非秘密地址
4. requireUnversioned检查版本功能。

本文件需要守住的边界：

- Suspended仍可能保留历史版本，必须拒绝
- 只有404且NoSuchKey映射不存在，不能泄露SDK原始错误。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/storage/oss' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/storage/oss/store.go') -and (Get-Item -LiteralPath 'services/document/internal/storage/oss/store.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/storage/oss/store.go')) { New-Item -ItemType File -Path 'services/document/internal/storage/oss/store.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:464 -->
````go
// Package oss 用官方 SDK 实现单机私有资源存储。业务仍以原 SQL key/摘要为权威，
// Bucket 不公开；本机文件锁覆盖对象 IO 和业务层最后的 SQL 确认。
package oss

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"collaboration/document/internal/storage/local"
	sdk "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

````

## 第 2 步：Config

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`type Config struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Region</code> | <code>string</code> | OSS地域，与允许的官方HTTPS Endpoint必须一致。 |
| <code>Endpoint</code> | <code>string</code> | 受限的官方OSS HTTPS地址，不能把签名凭据发往任意配置主机。 |
| <code>Bucket</code> | <code>string</code> | 私有资源Bucket名称，不将资源桶公开。 |
| <code>Prefix</code> | <code>string</code> | 本项目对象统一前缀，和资源键一起确定对象身份。 |
| <code>CredentialsFile</code> | <code>string</code> | 私密访问凭据文件路径；凭据轮换不应改变Region/Bucket/Prefix绑定。 |

<!-- write-step: 2 bytes=464:546 -->
````go
type Config struct {
	Region, Endpoint, Bucket, Prefix, CredentialsFile string
}

````

## 第 3 步：objectClient

插入位置：第 2 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`type objectClient interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>PutObject</code> | <code>(context.Context, *sdk.PutObjectRequest, ...func(*sdk.Options)) (*sdk.PutObjectResult, error)</code> | ctx、SDK写入请求及可选调用配置输入，返回SDK写入结果/错误；未知结果还须按原键HEAD核验。 |
| <code>GetObject</code> | <code>(context.Context, *sdk.GetObjectRequest, ...func(*sdk.Options)) (*sdk.GetObjectResult, error)</code> | ctx、SDK读取请求、可选配置输入，返回对象响应/错误；调用者关闭Body并核验摘要。 |
| <code>HeadObject</code> | <code>(context.Context, *sdk.HeadObjectRequest, ...func(*sdk.Options)) (*sdk.HeadObjectResult, error)</code> | ctx、SDK元数据请求、可选配置输入，返回对象元数据；存在不等于内容摘要已核验。 |
| <code>DeleteObject</code> | <code>(context.Context, *sdk.DeleteObjectRequest, ...func(*sdk.Options)) (*sdk.DeleteObjectResult, error)</code> | ctx、SDK删除请求、可选配置输入，返回删除响应/错误；后续核验原键状态才能确认清理。 |
| <code>GetBucketVersioning</code> | <code>(context.Context, *sdk.GetBucketVersioningRequest, ...func(*sdk.Options)) (*sdk.GetBucketVersioningResult, error)</code> | ctx、Bucket版本配置请求、可选配置输入，返回版本配置；实现只接受未启用版本管理的桶，Suspended也拒绝。 |

<!-- write-step: 3 bytes=546:1141 -->
````go
type objectClient interface {
	PutObject(context.Context, *sdk.PutObjectRequest, ...func(*sdk.Options)) (*sdk.PutObjectResult, error)
	GetObject(context.Context, *sdk.GetObjectRequest, ...func(*sdk.Options)) (*sdk.GetObjectResult, error)
	HeadObject(context.Context, *sdk.HeadObjectRequest, ...func(*sdk.Options)) (*sdk.HeadObjectResult, error)
	DeleteObject(context.Context, *sdk.DeleteObjectRequest, ...func(*sdk.Options)) (*sdk.DeleteObjectResult, error)
	GetBucketVersioning(context.Context, *sdk.GetBucketVersioningRequest, ...func(*sdk.Options)) (*sdk.GetBucketVersioningResult, error)
}

````

## 第 4 步：Store

插入位置：第 3 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`type Store struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>local</code> | <code>*local.Store</code> | 使用 <code>local.Store</code> 类型；其定义处的职责：在专用本地目录实现不可变对象Put/Open/Delete。 |
| <code>client</code> | <code>objectClient</code> | 受限SDK对象操作接口，便于测试替换；不把业务权限交给Bucket公开策略。 |
| <code>config</code> | <code>Config</code> | 已核验且绑定存储根的Region/Endpoint/Bucket/Prefix配置。 |

<!-- write-step: 4 bytes=1141:1221 -->
````go
type Store struct {
	local  *local.Store
	client objectClient
	config Config
}

````

## 第 5 步：ErrConfig、ErrCredentials、ErrVersioning、regionPattern、bucketPattern、prefixPattern

插入位置：第 4 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`var (`。

<!-- write-step: 5 bytes=1221:1631 -->
````go
var (
	ErrConfig      = errors.New("RESOURCE_OSS_CONFIG_INVALID")
	ErrCredentials = errors.New("RESOURCE_OSS_CREDENTIALS_UNAVAILABLE")
	ErrVersioning  = errors.New("RESOURCE_OSS_VERSIONING_UNSUPPORTED")
	regionPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)+$`)
	bucketPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)
	prefixPattern  = regexp.MustCompile(`^(?:[a-zA-Z0-9_-]+/)+$`)
)

````

## 第 6 步：New

插入位置：第 5 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func New(root string, cfg Config) (*Store, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>root</code> | <code>string</code> | 在此声明中的实际使用：<code>newStore(root, cfg, sdk.NewClient(c))</code> |
| <code>cfg</code> | <code>Config</code> | 在此声明中的实际使用：<code>validConfig(cfg)</code>；<code>cfg.CredentialsFile</code>；<code>cfg.Region</code>；<code>cfg.Endpoint</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Store</code> | 由 <code>newStore(root, cfg, sdk.NewClient(c))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>newStore(root, cfg, sdk.NewClient(c))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>ErrConfig</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>!validConfig(cfg)</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return newStore(root, cfg, sdk.NewClient(c))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1631:2322 -->
````go
func New(root string, cfg Config) (*Store, error) {
	if !validConfig(cfg) {
		return nil, ErrConfig
	}
	secret, err := readCredentials(cfg.CredentialsFile)
	if err != nil {
		return nil, err
	}
	// 不从环境继承 SDK 调试日志；其中可能含对象地址或签名。一次请求未知后
	// 由本适配器核验原键，再由业务原任务重试，不叠加 SDK 内部长时间退避。
	c := sdk.NewConfig().WithRegion(cfg.Region).WithEndpoint(cfg.Endpoint).
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(secret.ID, secret.Secret, secret.Token)).
		WithRetryMaxAttempts(1)
	c.LogLevel = sdk.Ptr(sdk.LogOff)
	return newStore(root, cfg, sdk.NewClient(c))
}

````

## 第 7 步：newStore

插入位置：第 6 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func newStore(root string, cfg Config, client objectClient) (*Store, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>root</code> | <code>string</code> | 在此声明中的实际使用：<code>local.New(root)</code> |
| <code>cfg</code> | <code>Config</code> | 在此声明中的实际使用：<code>validConfig(cfg)</code>；<code>cfg.Region</code>；<code>cfg.Endpoint</code>；<code>cfg.Bucket</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>client</code> | <code>objectClient</code> | 在此声明中的实际使用：<code>client == nil</code>；<code>client: client</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Store</code> | 实际返回表达式：<code>nil</code>；<code>&amp;Store{local: files, client: client, config: cfg}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrConfig</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!validConfig(cfg) &#124;&#124; client == nil</code>。条件成立时执行 <code>return nil, ErrConfig</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Store{local: files, client: client, config: cfg}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2322:2918 -->
````go
func newStore(root string, cfg Config, client objectClient) (*Store, error) {
	if !validConfig(cfg) || client == nil {
		return nil, ErrConfig
	}
	files, err := local.New(root)
	if err != nil {
		return nil, err
	}
	// 配置绑定不含凭据路径，允许轮换本机凭据而不改变对象身份。
	identity, _ := json.Marshal(struct{ Region, Endpoint, Bucket, Prefix string }{cfg.Region, cfg.Endpoint, cfg.Bucket, cfg.Prefix})
	if err := files.BindBackend(context.Background(), identity); err != nil {
		return nil, err
	}
	return &Store{local: files, client: client, config: cfg}, nil
}

````

## 第 8 步：validConfig

插入位置：第 7 步代码之后，紧接追加；完成稿约第 78 行。

本步声明：`func validConfig(cfg Config) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>Config</code> | 在此声明中的实际使用：<code>cfg.Region</code>；<code>cfg.Endpoint</code>；<code>cfg.Bucket</code>；<code>cfg.Prefix</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>len(cfg.Region) &lt;= 64 &amp;&amp; regionPattern.MatchString(cfg.Region) &amp;&amp; 		cfg.Endpoint == &quot;https://oss-&quot;+cfg.Region+&quot;.aliyuncs.com&quot; &amp;&amp; 		bucketPattern.MatchString(cfg.Bucket) &amp;&amp; len(cfg. …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return len(cfg.Region) &lt;= 64 &amp;&amp; regionPattern.MatchString(cfg.Region) &amp;&amp; 		cfg.Endpoint == &quot;https://oss-&quot;+cfg.Region+&quot;.aliyuncs.com&quot; &amp;&amp; 		bucketPattern.MatchString(cfg.Bucket) &amp;&amp; l …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2918:3300 -->
````go
func validConfig(cfg Config) bool {
	// 仅接受官方 HTTPS 外网 Endpoint，避免把签名凭据发给配置中的任意主机。
	return len(cfg.Region) <= 64 && regionPattern.MatchString(cfg.Region) &&
		cfg.Endpoint == "https://oss-"+cfg.Region+".aliyuncs.com" &&
		bucketPattern.MatchString(cfg.Bucket) && len(cfg.Prefix) <= 160 && prefixPattern.MatchString(cfg.Prefix)
}

````

## 第 9 步：Store.objectKey

插入位置：第 8 步代码之后，紧接追加；完成稿约第 85 行。

本步声明：`func (s *Store) objectKey(ctx context.Context, key string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Err</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>len(key)</code>；<code>strings.ToLower(key)</code>；<code>strings.ToLower(key) != key</code>；<code>hex.DecodeString(key)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>s.config.Prefix + key</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>local.ErrInvalidKey</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>len(key) != 64 &#124;&#124; strings.ToLower(key) != key</code>。条件成立时执行 <code>return &quot;&quot;, local.ErrInvalidKey</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, local.ErrInvalidKey</code>。当前函数在这里结束。
- 此处通过 <code>return s.config.Prefix + key, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3300:3644 -->
````go
func (s *Store) objectKey(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(key) != 64 || strings.ToLower(key) != key {
		return "", local.ErrInvalidKey
	}
	if _, err := hex.DecodeString(key); err != nil {
		return "", local.ErrInvalidKey
	}
	return s.config.Prefix + key, nil
}

````

## 第 10 步：Store.Acquire

插入位置：第 9 步代码之后，紧接追加；完成稿约第 98 行。

本步声明：`func (s *Store) Acquire(ctx context.Context, key string) (func(), error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.local.Acquire(ctx, key)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.local.Acquire(ctx, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>func()</code> | 由 <code>s.local.Acquire(ctx, key)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.local.Acquire(ctx, key)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.local.Acquire(ctx, key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3644:3756 -->
````go
func (s *Store) Acquire(ctx context.Context, key string) (func(), error) {
	return s.local.Acquire(ctx, key)
}

````

## 第 11 步：Store.AcquireRead

插入位置：第 10 步代码之后，紧接追加；完成稿约第 102 行。

本步声明：`func (s *Store) AcquireRead(ctx context.Context, key string) (func(), error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.local.AcquireRead(ctx, key)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.local.AcquireRead(ctx, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>func()</code> | 由 <code>s.local.AcquireRead(ctx, key)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.local.AcquireRead(ctx, key)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.local.AcquireRead(ctx, key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3756:3876 -->
````go
func (s *Store) AcquireRead(ctx context.Context, key string) (func(), error) {
	return s.local.AcquireRead(ctx, key)
}

````

## 第 12 步：Store.CleanupUpload

插入位置：第 11 步代码之后，紧接追加；完成稿约第 106 行。

本步声明：`func (s *Store) CleanupUpload(ctx context.Context, key string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.local.CleanupUpload(ctx, key)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>s.local.CleanupUpload(ctx, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>s.local.CleanupUpload(ctx, key)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.local.CleanupUpload(ctx, key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3876:3990 -->
````go
func (s *Store) CleanupUpload(ctx context.Context, key string) error {
	return s.local.CleanupUpload(ctx, key)
}

````

## 第 13 步：Store.requireUnversioned

插入位置：第 12 步代码之后，紧接追加；完成稿约第 110 行。

本步声明：`func (s *Store) requireUnversioned(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.client.GetBucketVersioning(ctx, &amp;sdk.GetBucketVersioningRequest{Bucket: sdk.Ptr(s.config.Bucket)})</code>；<code>storageError(ctx, err)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>storageError(ctx, err)</code>；<code>ErrVersioning</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return storageError(ctx, err)</code>。当前函数在这里结束。
- 检查 <code>result == nil &#124;&#124; (result.VersionStatus != nil &amp;&amp; *result.VersionStatus != &quot;&quot;)</code>。条件成立时执行 <code>return ErrVersioning</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=3990:4525 -->
````go
func (s *Store) requireUnversioned(ctx context.Context) error {
	result, err := s.client.GetBucketVersioning(ctx, &sdk.GetBucketVersioningRequest{Bucket: sdk.Ptr(s.config.Bucket)})
	if err != nil {
		return storageError(ctx, err)
	}
	// Suspended 也有历史版本与 delete marker，不能把 HEAD 404 当物理删除证明。
	// 写入/删除前检查；运维不得在服务运行时改变版本设置。
	if result == nil || (result.VersionStatus != nil && *result.VersionStatus != "") {
		return ErrVersioning
	}
	return nil
}

````

## 第 14 步：storageError

插入位置：第 13 步代码之后，紧接追加；完成稿约第 123 行。

本步声明：`func storageError(ctx context.Context, err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Err</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;service)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>local.ErrNotFound</code>；<code>local.ErrUnavailable</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>errors.As(err, &amp;service) &amp;&amp; service.StatusCode == 404 &amp;&amp; service.Code == &quot;NoSuchKey&quot;</code>。条件成立时执行 <code>return local.ErrNotFound</code>。当前函数在这里结束。
- 此处通过 <code>return local.ErrUnavailable</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=4525:4916 -->
````go
func storageError(ctx context.Context, err error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var service *sdk.ServiceError
	if errors.As(err, &service) && service.StatusCode == 404 && service.Code == "NoSuchKey" {
		return local.ErrNotFound
	}
	// AccessDenied/NoSuchBucket/连接失败不伪装成不存在，也不输出 SDK 原始错误。
	return local.ErrUnavailable
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/storage/oss/store.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
