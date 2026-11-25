// Package snapshot 在已经停止全部应用写者的冷边界复制完整对象。
// 数据库状态不在这里修改；缺失与完整字节都照原事实记录，未知/损坏则失败。
package snapshot

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"collaboration/document/internal/storage/local"
)

type Reader interface {
	AcquireRead(context.Context, string) (func(), error)
	Open(context.Context, string, uint64, [32]byte) (io.ReadCloser, error)
}

type Object struct {
	Key            string   `json:"key"`
	Size           uint64   `json:"size"`
	Checksum       [32]byte `json:"-"`
	State          string   `json:"state"`
	MetadataOnly   bool     `json:"-"`
	ChecksumAbsent bool     `json:"-"`
}

type Entry struct {
	Key          string `json:"key"`
	Size         uint64 `json:"size"`
	Checksum     string `json:"sha256"`
	State        string `json:"sourceState"`
	Present      bool   `json:"present"`
	MetadataOnly bool   `json:"metadataOnly,omitempty"`
}

type Manifest struct {
	Version      int     `json:"version"`
	State        string  `json:"state"`
	Objects      []Entry `json:"objects"`
	MissingReady int     `json:"missingReady"`
}

func Capture(ctx context.Context, source Reader, objects []Object, destination string) (Manifest, error) {
	manifest := Manifest{Version: 1, State: "PREPARING", Objects: make([]Entry, 0, len(objects))}
	target, err := local.New(destination)
	if err != nil {
		return manifest, err
	}
	seen := make(map[string]bool)
	for _, object := range objects {
		if seen[object.Key] || (object.State != "UPLOADING" && object.State != "READY" && object.State != "DELETING") {
			return manifest, errors.New("RESOURCE_SNAPSHOT_BINDING_INVALID")
		}
		seen[object.Key] = true
		entry, err := captureOne(ctx, source, target, object)
		if err != nil {
			return manifest, err
		}
		manifest.Objects = append(manifest.Objects, entry)
		if !entry.Present && object.State == "READY" {
			manifest.MissingReady++
		}
	}
	// 原环境包含历史 metadata-only 测试夹具时，明确记录实际缺失；复制不
	// 伪造正文、不改 READY、不声称缺失对象可读，恢复时仍核对同一事实。
	manifest.State = "COMPLETE"
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, errors.New("RESOURCE_SNAPSHOT_MANIFEST_FAILED")
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return manifest, errors.New("RESOURCE_SNAPSHOT_MANIFEST_FAILED")
	}
	defer root.Close()
	file, err := root.OpenFile(filepath.Base("resource-snapshot.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return manifest, errors.New("RESOURCE_SNAPSHOT_MANIFEST_FAILED")
	}
	_, writeErr := file.Write(append(raw, '\n'))
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return manifest, errors.New("RESOURCE_SNAPSHOT_MANIFEST_FAILED")
	}
	return manifest, nil
}

func captureOne(ctx context.Context, source Reader, target *local.Store, object Object) (Entry, error) {
	entry := Entry{Key: object.Key, Size: object.Size, Checksum: hex.EncodeToString(object.Checksum[:]), State: object.State}
	if object.MetadataOnly {
		if object.ChecksumAbsent {
			entry.Checksum = ""
		}
		entry.MetadataOnly = true
		verifier, ok := source.(interface {
			VerifyLegacyAbsence(context.Context, string) error
		})
		if !ok {
			return entry, errors.New("RESOURCE_SNAPSHOT_LEGACY_UNSUPPORTED")
		}
		return entry, verifier.VerifyLegacyAbsence(ctx, object.Key)
	}
	release, err := source.AcquireRead(ctx, object.Key)
	if err != nil {
		return entry, err
	}
	defer release()
	file, err := source.Open(ctx, object.Key, object.Size, object.Checksum)
	if errors.Is(err, local.ErrNotFound) {
		return entry, nil
	}
	if err != nil {
		return entry, err
	}
	// 本地目标也以独占发布/完整SHA确认，不能把中断的半份临时文件作为快照。
	putErr := target.Put(ctx, object.Key, file, object.Size, object.Checksum)
	closeErr := file.Close()
	if putErr != nil {
		return entry, putErr
	}
	if closeErr != nil {
		return entry, closeErr
	}
	entry.Present = true
	return entry, nil
}
