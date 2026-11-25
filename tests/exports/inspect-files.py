"""实际下载制品的内容验收，不以扩展名或ZIP签名代替固定版本与格式语义。"""

import argparse
import json
import re
import zipfile
from pathlib import Path
from xml.etree import ElementTree as ET

from pypdf import PdfReader


def inspect(folder: Path):
    with zipfile.ZipFile(folder / "main-fixed.zip") as package:
        text = package.read("document.md").decode("utf-8")
        assert "S2 未发布" in text and "S3 不得" not in text
        assert "未正式发布" in text and "S1 正式发布" not in text

    with zipfile.ZipFile(folder / "public-fixed.zip") as package:
        markdown = package.read("document.md").decode("utf-8")
        assert "S1 正式发布" in markdown and "S2 未发布" not in markdown
        assert "E=mc^2" in markdown and r"\frac{a+b}{2}" in markdown
        assets = re.findall(r"assets/resource-[1-9][0-9]*\.png", markdown)
        assert assets and all(asset in package.namelist() for asset in assets)
        assert all(package.read(asset).startswith(b"\x89PNG\r\n\x1a\n") for asset in assets)
        assert "source.json" not in package.namelist()

    ns = {"w": "http://schemas.openxmlformats.org/wordprocessingml/2006/main", "m": "http://schemas.openxmlformats.org/officeDocument/2006/math"}
    with zipfile.ZipFile(folder / "public-fixed.docx") as package:
        document = ET.fromstring(package.read("word/document.xml"))
        styles = ET.fromstring(package.read("word/styles.xml"))
        footer = ET.fromstring(package.read("word/footer-export.xml"))
        text = "".join(document.itertext())
        assert "S1 正式发布" in text and "S2 未发布" not in text
        assert len(document.findall(".//m:oMath", ns)) >= 2, "Word公式必须保留原生可编辑OMML"
        assert document.findall(".//w:tbl", ns)
        assert any(name.startswith("word/media/") for name in package.namelist())
        page = document.find(".//w:sectPr/w:pgSz", ns)
        margins = document.find(".//w:sectPr/w:pgMar", ns)
        prefix = "{" + ns["w"] + "}"
        assert page.get(prefix + "w") == "11906" and page.get(prefix + "h") == "16838"
        assert all(margins.get(prefix + side) == "1440" for side in ["left", "right", "top", "bottom"])
        assert "PAGE" in "".join(footer.itertext())
        fang_song = next(style for style in styles.findall("w:style", ns) if style.get(prefix + "styleId") == "ExportFangSong")
        assert fang_song.find("w:rPr/w:rFonts", ns).get(prefix + "eastAsia") == "FangSong"

    pdf = PdfReader(folder / "public-fixed.pdf")
    text = "\n".join(page.extract_text() for page in pdf.pages)
    assert "S1" in text and "正式发布" in text
    assert "S2 未发布" not in text and "S3 不得" not in text
    assert all(abs(float(page.mediabox.width) - 595.28) < 1 and abs(float(page.mediabox.height) - 841.89) < 1 for page in pdf.pages)
    assert any(page.images for page in pdf.pages), "PDF必须包含固定图片"
    return {"state": "PASS", "formats": ["DOCX", "MARKDOWN", "PDF"], "pdf_pages": len(pdf.pages), "word_native_math": True, "fixed_versions": "MAIN S2 / PUBLIC S1"}


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("folder", type=Path)
    args = parser.parse_args()
    print(json.dumps(inspect(args.folder), ensure_ascii=False))
