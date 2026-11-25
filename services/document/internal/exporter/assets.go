package exporter

import (
	"bytes"
	domain "collaboration/document/internal/domain/document"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

// 所有合法图片转换成固定PNG，Word与PDF共用同一首帧；先看像素预算再解码，
// 防止小压缩文件诱发无限像素分配。资源已经由业务完整checksum核验。
func writeAssets(directory string, items []domain.ExportAsset) (map[string]string, error) {
	if err := os.Mkdir(filepath.Join(directory, "assets"), 0700); err != nil {
		return nil, err
	}
	paths := map[string]string{}
	var total int
	for _, item := range items {
		if _, err := domain.CanonicalID(item.ID); err != nil {
			return nil, err
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(item.Data))
		if err != nil || config.Width < 1 || config.Height < 1 || uint64(config.Width)*uint64(config.Height) > 16<<20 {
			return nil, domain.Violation(domain.StateConflict, "EXPORT_IMAGE_PIXEL_LIMIT")
		}
		picture, _, err := image.Decode(bytes.NewReader(item.Data))
		if err != nil {
			return nil, domain.Violation(domain.StateConflict, "EXPORT_RESOURCE_INVALID")
		}
		var output bytes.Buffer
		if err = png.Encode(&output, picture); err != nil {
			return nil, err
		}
		if output.Len() > 32<<20 || total > (32<<20)-output.Len() {
			return nil, domain.Violation(domain.StateConflict, "EXPORT_ASSET_LIMIT")
		}
		total += output.Len()
		name := "assets/resource-" + item.ID + ".png"
		if err = os.WriteFile(filepath.Join(directory, filepath.FromSlash(name)), output.Bytes(), 0600); err != nil {
			return nil, err
		}
		paths[item.ID] = name
	}
	return paths, nil
}
