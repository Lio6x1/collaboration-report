package exporter

import (
	"archive/zip"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 真实Pandoc回归覆盖之前的缺口：Header的custom-style被转换器忽略，
// Markdown fenced Div无法在普通阅读器中显示格式，图片高度也没有进入AST。
// PDF验证到浏览器消费的HTML边界，本测试不启动浏览器或本机服务。
func TestParagraphFormattingPandocExports(t *testing.T) {
	pandoc := os.Getenv("EXPORT_TEST_PANDOC")
	if pandoc == "" {
		var err error
		pandoc, err = exec.LookPath("pandoc")
		if err != nil {
			t.Fatal("Pandoc 3.8 required; set EXPORT_TEST_PANDOC to its executable")
		}
	}
	body, err := codec.Decode([]byte(`{"type":"doc","content":[
		{"type":"paragraph","attrs":{"id":"p","align":"center","firstLineIndent":true},"content":[{"type":"text","text":"CENTERED","marks":[{"type":"strong"}]},{"type":"image","attrs":{"resourceId":"1","alt":"picture","title":null,"width":120,"height":90,"atomStrong":false,"atomEm":false,"atomCode":false,"atomStrike":false,"atomLink":null}}]},
		{"type":"heading","attrs":{"id":"h","level":2,"align":"right","firstLineIndent":true},"content":[{"type":"text","text":"RIGHT_HEADING"}]},
		{"type":"paragraph","attrs":{"id":"old"},"content":[{"type":"text","text":"DEFAULT_PARAGRAPH"}]}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	validator, err := codec.NewValidator()
	if err != nil || validator.Body(body) != nil {
		t.Fatal("export fixture violates the shared content contract", err)
	}
	unit := "1"
	document := domain.ExportDocument{
		Title: "Paragraph formatting", Target: "MAIN", Manifest: "1", Captured: time.Unix(1700000000, 0),
		Structure: dto.StructureRevision{Entries: []dto.StructureEntry{{NodeId: "chapter", Title: "Chapter", UnitId: &unit}}},
		Units:     []domain.ExportUnit{{ID: unit, Body: body.(map[string]any)}},
	}
	if err = json.Unmarshal([]byte(`{"style":{"body_font":"SimSun","western_font":"Times New Roman","body_size_pt":12,"heading_sizes_pt":[20,18,16,14,13,12],"page":"A4","margin_cm":"2.54"}}`), &document.Style); err != nil {
		t.Fatal(err)
	}
	renderer := &Renderer{config: Config{Pandoc: pandoc}}
	directory := t.TempDir()
	if err = os.Mkdir(filepath.Join(directory, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(filepath.FromSlash("../../../../tests/fixtures/export-image.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "assets", "resource-1.png"), image, 0600); err != nil {
		t.Fatal(err)
	}
	ast, err := buildAST(document, map[string]string{"1": "assets/resource-1.png"})
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(directory, "source.json")
	if err = os.WriteFile(input, ast, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Run("editable-word", func(t *testing.T) {
		document.Format = "DOCX"
		path, err := renderer.convert(ctx, directory, input, document)
		if err != nil {
			t.Fatal(err)
		}
		files := readExportZIP(t, path)
		word := files["word/document.xml"]
		styles := files["word/styles.xml"]
		for _, name := range []string{"ExportParagraphCenterIndent", "ExportHeading2RightIndent"} {
			if !strings.Contains(word, `w:pStyle w:val="`+name+`"`) {
				t.Fatalf("native Word paragraph style missing: %s", name)
			}
		}
		for _, expected := range []string{`w:basedOn w:val="Heading2"`, `w:jc w:val="center"`, `w:jc w:val="right"`, `w:firstLineChars="200"`} {
			if !strings.Contains(styles, expected) {
				t.Fatalf("native Word paragraph property missing: %s", expected)
			}
		}
		if !strings.Contains(word, "CENTERED") || !strings.Contains(word, "RIGHT_HEADING") || !strings.Contains(word, "DEFAULT_PARAGRAPH") {
			t.Fatal("conversion lost body text")
		}
	})
	t.Run("markdown-retains-real-html-formatting", func(t *testing.T) {
		document.Format = "MARKDOWN"
		path, err := renderer.convert(ctx, directory, input, document)
		if err != nil {
			t.Fatal(err)
		}
		markdown := readExportZIP(t, path)["document.md"]
		for _, expected := range []string{`style="text-align:center;text-indent:2em"`, `style="text-align:right;text-indent:2em"`, "<strong>CENTERED</strong>", "<h2", "RIGHT_HEADING", "assets/resource-1.png", `width="120"`, `height="90"`} {
			if !strings.Contains(markdown, expected) {
				t.Fatalf("Markdown lost actual exported formatting: %s", expected)
			}
		}
		if strings.Contains(markdown, "::: {custom-style=") {
			t.Fatal("paragraph formatting leaked Pandoc-only fenced Div syntax")
		}
	})
	t.Run("pdf-input-html", func(t *testing.T) {
		path := filepath.Join(directory, "document.html")
		if err := renderer.pandoc(ctx, directory, input, "--from=json", "--to=html5", "--standalone", "--mathjax", "--output="+path); err != nil {
			t.Fatal(err)
		}
		if err := embedPDFAssets(path, directory); err != nil {
			t.Fatal(err)
		}
		html, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{`style="text-align:center;text-indent:2em"`, `style="text-align:right;text-indent:2em"`, "<h2", `width="120"`, `height="90"`, "data:image/png;base64,"} {
			if !strings.Contains(string(html), expected) {
				t.Fatalf("PDF browser input lost formatting: %s", expected)
			}
		}
	})
}

func readExportZIP(t *testing.T, path string) map[string]string {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	files := map[string]string{}
	for _, entry := range reader.File {
		file, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name] = string(body)
	}
	return files
}
