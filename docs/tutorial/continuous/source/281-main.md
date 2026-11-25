# services/document/cmd/resourcesnapshot/main.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/281-main.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`run`、`main`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

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

UTF-8/LF全文SHA-256：`d2a7614100ea729bac4b75c776ccc59cb36f18deaeb39598bf3c19e36db0b09e`。[参考实现](../../../../services/document/cmd/resourcesnapshot/main.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
