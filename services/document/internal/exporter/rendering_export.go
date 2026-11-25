// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package exporter

import (
	"bytes"
	domain "collaboration/document/internal/domain/document"
	logic "collaboration/document/internal/logic/document"
	"collaboration/document/internal/storage/local"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// limits.go：保留原实现，按业务合并文件。

// 在分配Pandoc节点之前估算展开量。正文的字节上限不足以保护转换器：
// 一串空格会变成大量Space节点，必须先有整个作业共享的节点预算。
func validateDocument(document domain.ExportDocument) error {
	style := document.Style.Style
	if (style.BodyFont != "SimSun" && style.BodyFont != "FangSong") || style.WesternFont != "Times New Roman" || style.BodySizePt != 12 || style.Page != "A4" || style.MarginCm != "2.54" || len(style.HeadingSizesPt) != 6 {
		return domain.Violation(domain.StateConflict, "EXPORT_STYLE_INVALID")
	}
	for _, size := range style.HeadingSizesPt {
		if size < 1 || size > 72 {
			return domain.Violation(domain.StateConflict, "EXPORT_STYLE_INVALID")
		}
	}
	if style.TitleFont != nil && *style.TitleFont != "SimHei" || style.TitleSizePt != nil && *style.TitleSizePt != 22 {
		return domain.Violation(domain.StateConflict, "EXPORT_STYLE_INVALID")
	}
	remaining := 500000
	var visit func(any) bool
	visit = func(value any) bool {
		remaining -= 8
		if remaining < 0 {
			return false
		}
		switch value := value.(type) {
		case string:
			remaining -= 2 * (strings.Count(value, " ") + strings.Count(value, "\n") + strings.Count(value, "\t"))
		case []any:
			for _, child := range value {
				if !visit(child) {
					return false
				}
			}
		case map[string]any:
			for _, child := range value {
				if !visit(child) {
					return false
				}
			}
		}
		return remaining >= 0
	}
	if !visit(document.Title) {
		return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
	}
	for _, entry := range document.Structure.Entries {
		if !visit(entry.Title) {
			return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
		}
	}
	for _, unit := range document.Units {
		if !visit(unit.Body) {
			return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
		}
	}
	return nil
}

// renderer.go：保留原实现，按业务合并文件。

type Config struct{ Root, Pandoc, Node, PDFScript, Browser string }
type Renderer struct {
	config Config
	files  *local.Store
}

func New(config Config) (*Renderer, error) {
	if !filepath.IsAbs(config.Root) || !filepath.IsAbs(config.Pandoc) || !filepath.IsAbs(config.Node) || !filepath.IsAbs(config.PDFScript) {
		return nil, errors.New("EXPORT_CONFIG_INVALID")
	}
	files, err := local.New(config.Root)
	if err != nil {
		return nil, err
	}
	for _, path := range []string{config.Pandoc, config.Node, config.PDFScript} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("EXPORT_CONVERTER_UNAVAILABLE")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var output limitedBuffer
	cmd := exec.CommandContext(ctx, config.Pandoc, "--version")
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err = runProgram(cmd); err != nil || !strings.HasPrefix(output.String(), "pandoc 3.8\n") && !strings.HasPrefix(output.String(), "pandoc 3.8\r\n") {
		return nil, errors.New("EXPORT_PANDOC_VERSION_REQUIRED")
	}
	return &Renderer{config: config, files: files}, nil
}

func (r *Renderer) Files() logic.ResourceFiles { return r.files }
func artifactKey(value domain.ExportRecord) string {
	hash := sha256.Sum256([]byte("EXPORT_ARTIFACT\n" + value.Operation))
	return hex.EncodeToString(hash[:])
}
func (r *Renderer) directory(value domain.ExportRecord) string {
	return filepath.Join(r.config.Root, "work-"+artifactKey(value))
}

func (r *Renderer) Cleanup(ctx context.Context, value domain.ExportRecord) error {
	key := artifactKey(value)
	unlock, err := r.files.Acquire(ctx, key)
	if err != nil {
		return err
	}
	defer unlock()
	// 目录来自固定原号hash，只清本作业；根目录的跨进程身份/符号链接校验由Store完成。
	if err = os.RemoveAll(r.directory(value)); err != nil {
		return errors.New("EXPORT_TEMPORARY_CLEANUP_FAILED")
	}
	if err = r.files.CleanupUpload(ctx, key); err != nil {
		return err
	}
	return r.files.Delete(ctx, key)
}

func (r *Renderer) Render(ctx context.Context, value domain.ExportRecord, document domain.ExportDocument) (string, uint64, [32]byte, error) {
	var empty [32]byte
	if err := r.Cleanup(ctx, value); err != nil {
		return "", 0, empty, err
	}
	directory := r.directory(value)
	if err := os.Mkdir(directory, 0700); err != nil {
		return "", 0, empty, errors.New("EXPORT_TEMPORARY_UNAVAILABLE")
	}
	defer os.RemoveAll(directory)
	assets, err := writeAssets(directory, document.Assets)
	if err != nil {
		return "", 0, empty, err
	}
	ast, err := buildAST(document, assets)
	if err != nil {
		return "", 0, empty, err
	}
	if len(ast) > 48<<20 {
		return "", 0, empty, domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
	}
	input := filepath.Join(directory, "source.json")
	if err = os.WriteFile(input, ast, 0600); err != nil {
		return "", 0, empty, err
	}
	// 每次转换共享一个60秒预算，不给每个步骤分别续满时限。
	attempt, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	path, err := r.convert(attempt, directory, input, document)
	if err != nil {
		return "", 0, empty, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", 0, empty, errors.New("EXPORT_ARTIFACT_MISSING")
	}
	if info.Size() <= 0 || info.Size() > 32<<20 {
		return "", 0, empty, domain.Violation(domain.StateConflict, "EXPORT_ARTIFACT_LIMIT")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, empty, err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err = io.Copy(digest, file); err != nil {
		return "", 0, empty, err
	}
	var checksum [32]byte
	copy(checksum[:], digest.Sum(nil))
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return "", 0, empty, err
	}
	key := artifactKey(value)
	unlock, err := r.files.Acquire(ctx, key)
	if err != nil {
		return "", 0, empty, err
	}
	defer unlock()
	if err = r.files.Put(ctx, key, file, uint64(info.Size()), checksum); err != nil {
		return "", 0, empty, err
	}
	return key, uint64(info.Size()), checksum, nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(data []byte) (int, error) {
	n := len(data)
	left := (32 << 10) - b.Len()
	if left > 0 {
		if left < len(data) {
			data = data[:left]
		}
		_, _ = b.Buffer.Write(data)
	}
	return n, nil
}

func (r *Renderer) pandoc(ctx context.Context, directory string, args ...string) error {
	var diagnostic limitedBuffer
	cmd := exec.CommandContext(ctx, r.config.Pandoc, args...)
	cmd.Dir = directory
	cmd.Stdout = &diagnostic
	cmd.Stderr = &diagnostic
	if err := runProgram(cmd); err != nil {
		return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_FAILED")
	}
	// texmath无法表示的公式不得被静默丢失；日志可能含用户公式，只保留固定错误码。
	if strings.Contains(diagnostic.String(), "Could not convert TeX math") {
		return domain.Violation(domain.StateConflict, "EXPORT_FORMULA_UNSUPPORTED")
	}
	return nil
}
