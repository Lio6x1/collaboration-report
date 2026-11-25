package exporter

import (
	"archive/zip"
	"bytes"
	domain "collaboration/document/internal/domain/document"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

var styleBlock = regexp.MustCompile(`(?s)<w:style\b[^>]*>.*?</w:style>`)
var runProperties = regexp.MustCompile(`(?s)<w:rPr>.*?</w:rPr>`)
var fontProperties = regexp.MustCompile(`<w:(?:rFonts|sz|szCs|color)\b[^>]*/>`)
var sectionProperties = regexp.MustCompile(`(?s)<w:sectPr(?:\s[^>]*)?>.*?</w:sectPr>`)

// 在成熟转换器生成的OOXML上设置既定版式，保留其原生OMML、表格和图片关系。
// 不用截图伪装可编辑Word，也不把用户内容作为XML拼接；字体和字号来自固定枚举。
func styleDOCX(path string, document domain.ExportDocument) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	files := map[string][]byte{}
	var bytesCount uint64
	for _, file := range reader.File {
		if file.UncompressedSize64 > 64<<20 || bytesCount > (64<<20)-file.UncompressedSize64 {
			reader.Close()
			return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
		}
		bytesCount += file.UncompressedSize64
		body, err := file.Open()
		if err != nil {
			reader.Close()
			return err
		}
		data, err := io.ReadAll(io.LimitReader(body, 64<<20))
		body.Close()
		if err != nil {
			reader.Close()
			return err
		}
		files[file.Name] = data
	}
	reader.Close()
	styles, exists := files["word/styles.xml"]
	if !exists {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCX_INVALID")
	}
	fixed := document.Style.Style
	font := func(family string, points int64) string {
		return fmt.Sprintf(`<w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:eastAsia="%s" w:cs="Times New Roman"/><w:sz w:val="%d"/><w:szCs w:val="%d"/><w:color w:val="000000"/>`, family, points*2, points*2)
	}
	changed := styleBlock.ReplaceAllStringFunc(string(styles), func(block string) string {
		id := regexp.MustCompile(`w:styleId="([^"]+)"`).FindStringSubmatch(block)
		if len(id) != 2 {
			return block
		}
		family, points := fixed.BodyFont, int64(12)
		switch id[1] {
		case "Title":
			family = "SimHei"
			points = 22
		case "Normal", "BodyText", "FirstParagraph", "Compact":
		default:
			matched := false
			for index, size := range fixed.HeadingSizesPt {
				if id[1] == fmt.Sprintf("Heading%d", index+1) {
					family = "SimHei"
					points = size
					matched = true
					break
				}
			}
			if !matched {
				return block
			}
		}
		properties := `<w:rPr>` + font(family, points) + `</w:rPr>`
		if runProperties.MatchString(block) {
			// 只替换字体/字号，保留转换器已有的粗体、斜体及其它排版属性。
			return runProperties.ReplaceAllStringFunc(block, func(old string) string {
				return strings.Replace(fontProperties.ReplaceAllString(old, ""), `</w:rPr>`, font(family, points)+`</w:rPr>`, 1)
			})
		}
		return strings.Replace(block, `</w:style>`, properties+`</w:style>`, 1)
	})
	for _, family := range []string{"SimSun", "FangSong", "Times New Roman", "SimHei"} {
		name := "Export" + strings.ReplaceAll(family, " ", "")
		// 删除Pandoc可能自动产生的同名占位，避免两个styleId让Word随机选择。
		changed = styleBlock.ReplaceAllStringFunc(changed, func(block string) string {
			if strings.Contains(block, `w:styleId="`+name+`"`) {
				return ""
			}
			return block
		})
		properties := font(family, 12)
		if family == "Times New Roman" {
			properties = `<w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:eastAsia="Times New Roman"/><w:sz w:val="24"/>`
		}
		changed = strings.Replace(changed, `</w:styles>`, `<w:style w:type="character" w:styleId="`+name+`"><w:name w:val="`+name+`"/><w:rPr>`+properties+`</w:rPr></w:style></w:styles>`, 1)
	}
	// Pandoc支持Para的custom-style，但Header始终写HeadingN。先建立继承原正文/
	// 标题样式的段落格式，再按受控书签定位标题，保留大纲级别、字号、公式和图片。
	formats := map[string]string{}
	headingFormats := map[string]string{}
	var collect func(string, map[string]any)
	collect = func(unit string, item map[string]any) {
		kind := stringValue(item["type"])
		attrs, _ := item["attrs"].(map[string]any)
		if kind == "paragraph" || kind == "heading" {
			name, _ := paragraphFormat(kind, attrs)
			if name != "" {
				base, size := "BodyText", int64(12)
				if kind == "heading" {
					level := integer(attrs["level"])
					base = fmt.Sprintf("Heading%d", level)
					size = fixed.HeadingSizesPt[level-1]
					headingFormats[paragraphBookmark(unit, stringValue(attrs["id"]))] = name
				}
				align := stringValue(attrs["align"])
				if align != "center" && align != "right" {
					align = "left"
				}
				firstLine, characters := int64(0), 0
				if indent, _ := attrs["firstLineIndent"].(bool); indent {
					firstLine, characters = size*40, 200
				}
				// firstLineChars以百分之一字符计，200即两字符；twip值照顾旧阅读器。
				properties := fmt.Sprintf(`<w:pPr><w:jc w:val="%s"/><w:ind w:firstLine="%d" w:firstLineChars="%d"/></w:pPr>`, align, firstLine, characters)
				formats[name] = `<w:style w:type="paragraph" w:customStyle="1" w:styleId="` + name + `"><w:name w:val="` + name + `"/><w:basedOn w:val="` + base + `"/>` + properties + `</w:style>`
			}
		}
		children, _ := item["content"].([]any)
		for _, child := range children {
			if node, ok := child.(map[string]any); ok {
				collect(unit, node)
			}
		}
	}
	for _, unit := range document.Units {
		collect(unit.ID, unit.Body)
	}
	formatNames := make([]string, 0, len(formats))
	for name := range formats {
		formatNames = append(formatNames, name)
	}
	sort.Strings(formatNames)
	for _, name := range formatNames {
		changed = styleBlock.ReplaceAllStringFunc(changed, func(block string) string {
			if strings.Contains(block, `w:styleId="`+name+`"`) {
				return ""
			}
			return block
		})
		changed = strings.Replace(changed, `</w:styles>`, formats[name]+`</w:styles>`, 1)
	}
	files["word/styles.xml"] = []byte(changed)
	body := string(files["word/document.xml"])
	paragraphStyle := regexp.MustCompile(`<w:pStyle\b[^>]*/>`)
	markedParagraph := regexp.MustCompile(`(?s)<w:bookmarkStart\b[^>]*w:name="(exportformat[0-9a-f]{24})"[^>]*/>\s*<w:p>.*?</w:p>`)
	applied := map[string]bool{}
	// 一次扫描整篇OOXML，避免正文标题较多时按标题反复扫描整个Word文件。
	body = markedParagraph.ReplaceAllStringFunc(body, func(block string) string {
		bookmark := markedParagraph.FindStringSubmatch(block)[1]
		name, exists := headingFormats[bookmark]
		if !exists || !paragraphStyle.MatchString(block) {
			return block
		}
		applied[bookmark] = true
		return paragraphStyle.ReplaceAllString(block, `<w:pStyle w:val="`+name+`"/>`)
	})
	if len(applied) != len(headingFormats) {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCX_INVALID")
	}
	// Pandoc默认表格只有横线，统一补齐浅灰网格和单元格留白，Word实际排版仍可编辑。
	tableProperties := regexp.MustCompile(`(?s)<w:tblPr>.*?</w:tblPr>`)
	oldTableBorders := regexp.MustCompile(`(?s)<w:tblBorders>.*?</w:tblBorders>`)
	oldTableMargins := regexp.MustCompile(`(?s)<w:tblCellMar>.*?</w:tblCellMar>`)
	borders := `<w:tblBorders>`
	for _, edge := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		borders += `<w:` + edge + ` w:val="single" w:sz="4" w:color="D9D9D9"/>`
	}
	borders += `</w:tblBorders><w:tblCellMar><w:top w:w="100" w:type="dxa"/><w:left w:w="100" w:type="dxa"/><w:bottom w:w="100" w:type="dxa"/><w:right w:w="100" w:type="dxa"/></w:tblCellMar>`
	body = tableProperties.ReplaceAllStringFunc(body, func(properties string) string {
		properties = oldTableMargins.ReplaceAllString(oldTableBorders.ReplaceAllString(properties, ""), "")
		return strings.Replace(properties, `</w:tblPr>`, borders+`</w:tblPr>`, 1)
	})
	section := `<w:sectPr><w:footerReference w:type="default" r:id="rIdExportFooter"/><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`
	if !sectionProperties.MatchString(body) {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCX_INVALID")
	}
	files["word/document.xml"] = []byte(sectionProperties.ReplaceAllString(body, section))
	files["word/footer-export.xml"] = []byte(`<?xml version="1.0" encoding="UTF-8"?><w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="20"/></w:rPr><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p></w:ftr>`)
	files["word/_rels/document.xml.rels"] = []byte(strings.Replace(string(files["word/_rels/document.xml.rels"]), `</Relationships>`, `<Relationship Id="rIdExportFooter" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer-export.xml"/></Relationships>`, 1))
	files["[Content_Types].xml"] = []byte(strings.Replace(string(files["[Content_Types].xml"]), `</Types>`, `<Override PartName="/word/footer-export.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/></Types>`, 1))
	// 转换时间/机器用户名不进入导出包；固定版本时间已显示在正文。
	if core, ok := files["docProps/core.xml"]; ok {
		private := regexp.MustCompile(`(?s)<(?:dc:creator|cp:lastModifiedBy)\b[^>]*>.*?</(?:dc:creator|cp:lastModifiedBy)>`)
		files["docProps/core.xml"] = private.ReplaceAll(core, nil)
	}
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetModTime(document.Captured.UTC())
		entry, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err = entry.Write(files[name]); err != nil {
			return err
		}
	}
	if err = archive.Close(); err != nil {
		return err
	}
	if output.Len() > 32<<20 {
		return domain.Violation(domain.StateConflict, "EXPORT_ARTIFACT_LIMIT")
	}
	return os.WriteFile(path, output.Bytes(), 0600)
}
