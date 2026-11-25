package oss

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"

	"collaboration/document/internal/storage/local"
	"collaboration/document/internal/storage/snapshot"
)

// 新增关口是云存储对备份的影响：保存原对象、记录真实缺失、网络未知时
// 不产出 COMPLETE，并且目标能独立用本地存储读出原完整字节。
func TestCloudSnapshotBecomesIndependentLocalObjects(t *testing.T) {
	sourceRoot, targetRoot := t.TempDir(), t.TempDir()
	s, h, key, body, digest := fixture(t, sourceRoot)
	h.objects[s.config.Prefix+key] = bytes.Clone(body)
	missingHash := sha256.Sum256([]byte("original metadata without physical object"))
	missingKey := hex.EncodeToString(missingHash[:])
	objects := []snapshot.Object{
		{Key: key, Size: uint64(len(body)), Checksum: digest, State: "READY"},
		{Key: missingKey, Size: 1, Checksum: missingHash, State: "READY"},
	}
	manifest, err := snapshot.Capture(context.Background(), s, objects, targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.State != "COMPLETE" || manifest.MissingReady != 1 || len(manifest.Objects) != 2 ||
		!manifest.Objects[0].Present || manifest.Objects[1].Present {
		t.Fatal("snapshot changed original physical inventory")
	}
	target, err := local.New(targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := target.RequireLocalBackend(context.Background()); err != nil {
		t.Fatal("snapshot retained cloud binding")
	}
	file, err := target.Open(context.Background(), key, uint64(len(body)), digest)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(file)
	file.Close()
	if err != nil || !bytes.Equal(actual, body) {
		t.Fatal("snapshot lost original bytes")
	}
	if !bytes.Equal(h.objects[s.config.Prefix+key], body) || h.deleteCount != 0 || h.putCount != 0 {
		t.Fatal("snapshot mutated source cloud objects")
	}
	assertNoTemporary(t, sourceRoot)
	h.getLostAfterPut, h.putCount = true, 1
	failedRoot := t.TempDir()
	failed, err := snapshot.Capture(context.Background(), s, objects, failedRoot)
	if err == nil || failed.State == "COMPLETE" {
		t.Fatal("unknown cloud read produced complete backup")
	}
	if _, err := os.Lstat(filepath.Join(failedRoot, "resource-snapshot.json")); !os.IsNotExist(err) {
		t.Fatal("failed snapshot manifest published")
	}
}

// 真实原库存在旧 key/NULL checksum 夹具：缺失可以如实保存；存在/未知
// 都不能伪造可读正文或生成 COMPLETE，正常上传的 key 规则也不能放宽。
func TestLegacyMetadataSnapshotRequiresProvenAbsence(t *testing.T) {
	root := t.TempDir()
	s, h, _, _, _ := fixture(t, root)
	key := "fixture-storage-original"
	objects := []snapshot.Object{{Key: key, Size: 1, State: "READY", MetadataOnly: true, ChecksumAbsent: true}}
	m, err := snapshot.Capture(context.Background(), s, objects, t.TempDir())
	if err != nil || m.MissingReady != 1 || m.Objects[0].Checksum != "" || !m.Objects[0].MetadataOnly {
		t.Fatal("original NULL checksum absence changed", err)
	}
	h.objects[s.config.Prefix+key] = []byte("unexpected cloud object")
	if _, err := snapshot.Capture(context.Background(), s, objects, t.TempDir()); err == nil {
		t.Fatal("metadata without checksum accepted existing cloud object")
	}
	delete(h.objects, s.config.Prefix+key)
	h.getLostAfterPut, h.putCount = true, 1
	if _, err := snapshot.Capture(context.Background(), s, objects, t.TempDir()); err == nil {
		t.Fatal("unknown cloud absence accepted")
	}
	h.getLostAfterPut = false
	if err := os.WriteFile(filepath.Join(root, key), []byte("existing local object"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshot.Capture(context.Background(), s, objects, t.TempDir()); err == nil {
		t.Fatal("metadata without checksum accepted existing local object")
	}
}
