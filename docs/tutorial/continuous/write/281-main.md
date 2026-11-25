# 分步编写 `services/document/cmd/resourcesnapshot/main.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/281-main.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

为OSS冷备份读取SQL资源库存并调用对象快照复制。

备份工具只观察现有事实，不能通过清理状态或配额来掩盖缺失。

写之前先按这条执行路线理解各段的连接：

1. run验证document配置和.runtime/backups内目标
2. 只SELECT UPLOADING/READY/DELETING
3. 以原key/长度/SHA调用snapshot.Capture
4. main只输出安全状态码。

本文件需要守住的边界：

- 监督器必须先确认全部应用与Canal退出
- 15分钟离线预算不是用户请求期限
- 目标必须是已核验目录。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/cmd/resourcesnapshot' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/cmd/resourcesnapshot/main.go') -and (Get-Item -LiteralPath 'services/document/cmd/resourcesnapshot/main.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/cmd/resourcesnapshot/main.go')) { New-Item -ItemType File -Path 'services/document/cmd/resourcesnapshot/main.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:272 -->
````go
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	ossstore "collaboration/document/internal/storage/oss"
	"collaboration/document/internal/storage/snapshot"
	"collaboration/platform"
)

````

## 第 2 步：run

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`func run() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;RESOURCE_SNAPSHOT_CONFIG_INVALID&quot;)</code>；<code>errors.New(&quot;RESOURCE_SNAPSHOT_TARGET_INVALID&quot;)</code>；<code>errors.New(&quot;RESOURCE_SNAPSHOT_INVENTORY_FAILED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>cfg.Role != &quot;document&quot; &#124;&#124; cfg.Resources == nil &#124;&#124; cfg.Resources.OSS == nil &#124;&#124; !filepath.IsAbs(cfg.Root) &#124;&#124; !filepath.IsAbs(*target)</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_SNAPSHOT_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; offset == &quot;.&quot; &#124;&#124; offset == &quot;..&quot; &#124;&#124; strings.HasPrefix(offset, &quot;..&quot;+string(filepath.Separator)) &#124;&#124; filepath.IsAbs(offset)</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_SNAPSHOT_TARGET_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !info.IsDir() &#124;&#124; info.Mode()&amp;os.ModeSymlink != 0</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_SNAPSHOT_TARGET_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>store.DB.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>context.WithTimeout(context.Background(), 10*time.Second)</code>。
- 执行 <code>store.DB.QueryContext(ctx, `SELECT storage_key,byte_size,checksum,state FROM resource WHERE state IN (&#x27;UPLOADING&#x27;,&#x27;READY&#x27;,&#x27;DELETING&#x27;) ORDER BY id`)</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return errors.New(&quot;RESOURCE_SNAPSHOT_INVENTORY_FAILED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>rows.Scan(&amp;object.Key, &amp;object.Size, &amp;checksum, &amp;object.State) != nil &#124;&#124; (len(checksum) != 32 &amp;&amp; len(checksum) != 0)</code>。
- 执行 <code>rows.Scan(&amp;object.Key, &amp;object.Size, &amp;checksum, &amp;object.State)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return errors.New(&quot;RESOURCE_SNAPSHOT_INVENTORY_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=272:2269 -->
````go
func run() error {
	configFile := flag.String("config", "", "private document service config")
	target := flag.String("target", "", "new private backup resource staging directory")
	flag.Parse()
	cfg, err := platform.LoadConfig(*configFile)
	if err != nil {
		return err
	}
	if cfg.Role != "document" || cfg.Resources == nil || cfg.Resources.OSS == nil || !filepath.IsAbs(cfg.Root) || !filepath.IsAbs(*target) {
		return errors.New("RESOURCE_SNAPSHOT_CONFIG_INVALID")
	}
	base := filepath.Join(cfg.Root, ".runtime", "backups")
	offset, err := filepath.Rel(base, *target)
	if err != nil || offset == "." || offset == ".." || strings.HasPrefix(offset, ".."+string(filepath.Separator)) || filepath.IsAbs(offset) {
		return errors.New("RESOURCE_SNAPSHOT_TARGET_INVALID")
	}
	info, err := os.Lstat(*target)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("RESOURCE_SNAPSHOT_TARGET_INVALID")
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		return err
	}
	defer store.DB.Close()
	// 调用监督器先验证全部应用真实退出、停止Canal，再保持MySQL仅供读取。
	// CLI 只SELECT，既不改操作状态也不修改配额，现有查询期限沿DSN保持。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	rows, err := store.DB.QueryContext(ctx, `SELECT storage_key,byte_size,checksum,state FROM resource WHERE state IN ('UPLOADING','READY','DELETING') ORDER BY id`)
	if err != nil {
		cancel()
		return errors.New("RESOURCE_SNAPSHOT_INVENTORY_FAILED")
	}
	objects := make([]snapshot.Object, 0)
	for rows.Next() {
		var object snapshot.Object
		var checksum []byte
		if rows.Scan(&object.Key, &object.Size, &checksum, &object.State) != nil || (len(checksum) != 32 && len(checksum) != 0) {
			rows.Close()
			cancel()
			return errors.New("RESOURCE_SNAPSHOT_INVENTORY_INVALID")
		}
		copy(object.Checksum[:], checksum)
		_, keyErr := hex.DecodeString(object.Key)
		object.ChecksumAbsent = len(checksum) == 0
````

## 第 3 步：run（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 65 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_SNAPSHOT_INVENTORY_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(context.Background(), 15*time.Minute)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return json.NewEncoder(os.Stdout).Encode(map&#91;string&#93;any{&quot;state&quot;: manifest.State, &quot;objects&quot;: len(manifest.Objects), &quot;missingReady&quot;: manifest.MissingReady})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2269:3245 -->
````go
		object.MetadataOnly = object.ChecksumAbsent || len(object.Key) != 64 || strings.ToLower(object.Key) != object.Key || keyErr != nil
		objects = append(objects, object)
	}
	err = rows.Err()
	rows.Close()
	cancel()
	if err != nil {
		return errors.New("RESOURCE_SNAPSHOT_INVENTORY_FAILED")
	}
	o := cfg.Resources.OSS
	files, err := ossstore.New(cfg.Resources.Root, ossstore.Config{Region: o.Region, Endpoint: o.Endpoint, Bucket: o.Bucket, Prefix: o.Prefix, CredentialsFile: o.CredentialsFile})
	if err != nil {
		return err
	}
	// 整个离线备份预算不改变用户请求期限；每个对象仍使用其原 key/长度/SHA。
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	manifest, err := snapshot.Capture(ctx, files, objects, *target)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"state": manifest.State, "objects": len(manifest.Objects), "missingReady": manifest.MissingReady})
}

````

## 第 4 步：main

插入位置：第 3 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`func main()`。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 检查 <code>character != &#x27;_&#x27; &amp;&amp; (character &lt; &#x27;A&#x27; &#124;&#124; character &gt; &#x27;Z&#x27;)</code>。

<!-- write-step: 4 bytes=3245:3545 -->
````go
func main() {
	if err := run(); err != nil {
		code := err.Error()
		for _, character := range code {
			if character != '_' && (character < 'A' || character > 'Z') {
				code = "RESOURCE_SNAPSHOT_FAILED"
				break
			}
		}
		fmt.Println(`{"state":"FAILED","code":"` + code + `"}`)
		os.Exit(1)
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/cmd/resourcesnapshot/main.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
