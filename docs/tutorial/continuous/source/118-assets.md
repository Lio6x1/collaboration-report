# services/document/internal/exporter/assets.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/118-assets.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：转换固定版本，管理进程和产物；不能重新读取变化中的正文。

主要声明：`writeAssets`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`814ec67bc97c139f1420585d44dbedda036d6a4afb73fe4a4fb53cdb035854d8`。[参考实现](../../../../services/document/internal/exporter/assets.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
