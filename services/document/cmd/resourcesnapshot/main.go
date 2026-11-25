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
