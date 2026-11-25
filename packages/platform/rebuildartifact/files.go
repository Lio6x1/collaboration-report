package rebuildartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var artifactFile = regexp.MustCompile(`^[0-9a-f]{64}\.(prepare\.json|snapshot\.json|entries\.ndjson)$`)
var fileFailure = errors.New("REBUILD_ARTIFACT_FILE_FAILED")

// Reference只计算结构合法制品的内容地址，不代替首次Verify或数据库接受事实。
func Reference(raw []byte, purpose string) (string, error) {
	e, err := decodeEnvelope(raw)
	if err != nil || purposeRole(purpose) == "" || e.Purpose != purpose {
		return "", ErrEnvelope
	}
	digest := sha256.Sum256(raw)
	return "rebuild-" + purpose + ":" + hex.EncodeToString(digest[:]), nil
}

func FileName(reference string) (string, error) {
	for _, kind := range []string{"prepare", "snapshot"} {
		prefix := "rebuild-" + kind + "-v1:"
		if strings.HasPrefix(reference, prefix) {
			digest := strings.TrimPrefix(reference, prefix)
			name := digest + "." + kind + ".json"
			if artifactFile.MatchString(name) {
				return name, nil
			}
		}
	}
	return "", ErrEnvelope
}

// ReadFile只在装配层给出的本服务inbox内按内容地址读取，不接受路径或URL作为引用。
func ReadFile(dir, reference string) ([]byte, error) {
	name, err := FileName(reference)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxEnvelopeBytes {
		return nil, fileFailure
	}
	raw, err := readBounded(path)
	if err != nil {
		return nil, fileFailure
	}
	purpose := PrepareV1
	if strings.HasSuffix(name, ".snapshot.json") {
		purpose = SnapshotV1
	}
	actual, err := Reference(raw, purpose)
	if err != nil || actual != reference {
		return nil, ErrEnvelope
	}
	return raw, nil
}

// PublishFile先写受控临时文件并Sync，成功后才发布。Windows重试遇到原文件时核原字节摘要，
// 不覆盖不同内容。dir只由本服务可信配置构造，name只能是内容地址文件名。
func PublishFile(dir, name string, write func(io.Writer) error) error {
	if !artifactFile.MatchString(name) || write == nil {
		return fileFailure
	}
	if os.MkdirAll(dir, 0700) != nil {
		return fileFailure
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fileFailure
	}
	temp, err := os.CreateTemp(dir, ".rebuild-*")
	if err != nil {
		return fileFailure
	}
	path := temp.Name()
	defer os.Remove(path)
	defer temp.Close()
	hash := sha256.New()
	if err := write(io.MultiWriter(temp, hash)); err != nil {
		return err
	}
	if temp.Sync() != nil || temp.Close() != nil {
		return fileFailure
	}
	target := filepath.Join(dir, name)
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() {
			return fileFailure
		}
		existing, err := os.Open(target)
		if err != nil {
			return fileFailure
		}
		defer existing.Close()
		original := sha256.New()
		if _, err := io.Copy(original, existing); err != nil {
			return fileFailure
		}
		if string(original.Sum(nil)) != string(hash.Sum(nil)) {
			return errors.New("REBUILD_ARTIFACT_FILE_CONFLICT")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fileFailure
	}
	if os.Rename(path, target) != nil {
		return fileFailure
	}
	return nil
}
