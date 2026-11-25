# services/document/internal/exporter/formats.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/121-formats.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：转换固定版本，管理进程和产物；不能重新读取变化中的正文。

主要声明：`convert`、`markdownPackage`、`embedPDFAssets`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package exporter

import (
	"archive/zip"
	domain "collaboration/document/internal/domain/document"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func (r *Renderer) convert(ctx context.Context, directory, input string, document domain.ExportDocument) (string, error) {
	switch document.Format {
	case "DOCX":
		path := filepath.Join(directory, "document.docx")
		if err := r.pandoc(ctx, directory, input, "--from=json", "--to=docx", "--output="+path, "--resource-path="+directory); err != nil {
			return "", err
		}
		if err := styleDOCX(path, document); err != nil {
			return "", err
		}
		return path, nil
	case "MARKDOWN":
		path := filepath.Join(directory, "document.md")
		// Markdown本身没有对齐/首行缩进语法。由Pandoc将受控格式块转成完整HTML，
		// 避免fenced Div在普通阅读器里显示为文本，或把标题/强调留在HTML内当作字面量。
		filter := filepath.Join(directory, "paragraph-formatting.lua")
		if err := os.WriteFile(filter, []byte(markdownParagraphFilter), 0600); err != nil {
			return "", err
		}
		if err := r.pandoc(ctx, directory, input, "--from=json", "--to=markdown", "--standalone", "--wrap=none", "--lua-filter="+filter, "--output="+path); err != nil {
			return "", err
		}
		return markdownPackage(directory, document)
	case "PDF":
		path := filepath.Join(directory, "document.html")
		if err := r.pandoc(ctx, directory, input, "--from=json", "--to=html5", "--standalone", "--mathjax", "--output="+path); err != nil {
			return "", err
		}
		if err := embedPDFAssets(path, directory); err != nil {
			return "", err
		}
		settings, err := json.Marshal(document.Style.Style)
		if err != nil {
			return "", err
		}
		if err = os.WriteFile(filepath.Join(directory, "style.json"), settings, 0600); err != nil {
			return "", err
		}
		var diagnostic limitedBuffer
		cmd := exec.CommandContext(ctx, r.config.Node, "--no-maglev", "--max-old-space-size=256", r.config.PDFScript, directory, r.config.Browser)
		cmd.Dir = directory
		cmd.Stdout = &diagnostic
		cmd.Stderr = &diagnostic
		if err = runProgram(cmd); err != nil {
			return "", domain.Violation(domain.StateConflict, "EXPORT_PDF_RENDER_FAILED")
		}
		return filepath.Join(directory, "document.pdf"), nil
	}
	return "", domain.Violation(domain.InvalidRequest, "EXPORT_FORMAT_INVALID")
}

// 过滤器内容是固定程序，用户只能提供已由共享schema校验的属性；HTML仍由成熟转换器转义。
const markdownParagraphFilter = `function Div(block)
  if block.attributes["data-paragraph-format"] == "true" then
    block.attributes["data-paragraph-format"] = nil
    block.attributes["custom-style"] = nil
    return pandoc.RawBlock("html", pandoc.write(pandoc.Pandoc({block}), "html5"))
  end
end
`

func markdownPackage(directory string, document domain.ExportDocument) (string, error) {
	path := filepath.Join(directory, "document.zip")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	archive := zip.NewWriter(file)
	names := []string{"document.md"}
	items, err := os.ReadDir(filepath.Join(directory, "assets"))
	if err != nil {
		archive.Close()
		file.Close()
		return "", err
	}
	for _, item := range items {
		names = append(names, "assets/"+item.Name())
	}
	for _, name := range names {
		body, err := os.Open(filepath.Join(directory, filepath.FromSlash(name)))
		if err != nil {
			archive.Close()
			file.Close()
			return "", err
		}
		entry, writeErr := archive.Create(name)
		if writeErr == nil {
			_, writeErr = io.Copy(entry, body)
		}
		body.Close()
		if writeErr != nil {
			archive.Close()
			file.Close()
			return "", writeErr
		}
	}
	entry, err := archive.Create("README.txt")
	if err == nil {
		_, err = io.WriteString(entry, "固定版本导出\n目标："+document.Target+"\n清单："+document.Manifest+"\n正式版："+document.Release+"\n时间："+document.Captured.UTC().Format("2006-01-02 15:04:05 UTC")+"\n解压整个压缩包后打开document.md；图片只引用同包assets。动画图采用首帧，公式以LaTeX保留。段落对齐与首行缩进保留为受控HTML/CSS；字体与段落格式的实际显示取决于阅读器是否支持HTML/CSS。\n")
	}
	closeErr := archive.Close()
	fileErr := file.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if fileErr != nil {
		return "", fileErr
	}
	return path, nil
}

var localImage = regexp.MustCompile(`src="(assets/resource-[1-9][0-9]*\.png)"`)

func embedPDFAssets(path, directory string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(body) > 48<<20 {
		return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
	}
	cache := map[string]string{}
	size := len(body)
	for _, match := range localImage.FindAllSubmatch(body, -1) {
		name := string(match[1])
		data, exists := cache[name]
		if !exists {
			bytes, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(name)))
			if err != nil {
				return err
			}
			data = "data:image/png;base64," + base64.StdEncoding.EncodeToString(bytes)
			cache[name] = data
		}
		size += len(data) - len(name)
		if size > 64<<20 {
			return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
		}
	}
	result := localImage.ReplaceAllStringFunc(string(body), func(source string) string {
		match := localImage.FindStringSubmatch(source)
		return `src="` + cache[match[1]] + `"`
	})
	// Pandoc的MathJax远程脚本不执行；后续本地KaTeX统一处理，浏览器禁用任意网络。
	scripts := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`)
	result = scripts.ReplaceAllString(result, "")
	if strings.Contains(result, `src="assets/`) {
		return domain.Violation(domain.StateConflict, "EXPORT_RESOURCE_INVALID")
	}
	return os.WriteFile(path, []byte(result), 0600)
}
````

UTF-8/LF全文SHA-256：`1ecb62b7db622e86692f87ce3dddd675695628611b74c4180a8e2cb1880741be`。[参考实现](../../../../services/document/internal/exporter/formats.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
