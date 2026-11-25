import { readFile, writeFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { createRequire } from 'node:module';
import { chromium } from '@playwright/test';
import katex from 'katex';

// 参数仅由Doc的受控转换器提供；正文来自生成HTML，不能把用户URL送入浏览器。
const [directory, channel] = process.argv.slice(2);
const require = createRequire(import.meta.url);
const katexDirectory = dirname(require.resolve('katex'));
let browser;
try {
  const html = await readFile(resolve(directory, 'document.html'), 'utf8');
  const style = JSON.parse(await readFile(resolve(directory, 'style.json'), 'utf8'));
  if (Buffer.byteLength(html) > 64 << 20 || !['SimSun', 'FangSong'].includes(style.body_font))
    throw Error('EXPORT_PDF_INPUT_INVALID');
  let mathCSS = await readFile(resolve(katexDirectory, 'katex.min.css'), 'utf8');
  const fonts = new Map();
  for (const match of mathCSS.matchAll(/url\((fonts\/[A-Za-z0-9_-]+\.woff2)\)/g)) {
    if (!fonts.has(match[1]))
      fonts.set(
        match[1],
        'data:font/woff2;base64,' +
          (await readFile(resolve(katexDirectory, match[1]))).toString('base64'),
      );
  }
  mathCSS = mathCSS.replace(
    /url\((fonts\/[A-Za-z0-9_-]+\.woff2)\)/g,
    (_, name) => `url(${fonts.get(name)})`,
  );
  browser = await chromium.launch({ ...(channel ? { channel } : {}), headless: true });
  const context = await browser.newContext({ javaScriptEnabled: true });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  // 链接只作为文字/超链接保留；转换时没有来自正文的HTTP或file访问。
  await page.route('**/*', (route) => route.abort());
  await page.setContent(html, { waitUntil: 'load' });
  const formulas = await page.locator('.math').allTextContents();
  const rendered = formulas.map((formula) => {
    const displayMode = formula.startsWith('\\[');
    const source = formula.replace(/^\\[([]|\\[)\]]$/g, '');
    return katex.renderToString(source, {
      displayMode,
      throwOnError: true,
      trust: false,
      maxExpand: 1000,
      maxSize: 10,
      output: 'htmlAndMathml',
    });
  });
  await page.locator('.math').evaluateAll((items, values) => {
    items.forEach((item, index) => {
      item.innerHTML = values[index];
    });
  }, rendered);
  const headings = style.heading_sizes_pt
    .map((size, index) => `h${index + 1}{font-size:${size}pt}`)
    .join('');
  // Pandoc默认CSS面向网页：table的display:block会让空表收缩，代码的滚动容器与
  // break-inside:avoid会把长代码整体推到下一页。打印使用真实表格和可跨页的普通流。
  // 同时覆盖高亮源码的行span，避免长行被inline-block及默认悬挂缩进裁切。
  await page.addStyleTag({
    content:
      mathCSS +
      `
    @page{size:A4;margin:2.54cm} html,body{margin:0;padding:0;max-width:none}
    body{font-family:"Times New Roman","${style.body_font}";font-size:12pt;line-height:1.5;color:#111;overflow-wrap:anywhere}
    header .title{text-align:center;font-family:SimHei,"Times New Roman";font-size:22pt}
    h1,h2,h3,h4,h5,h6{font-family:SimHei,"Times New Roman";line-height:1.35;break-after:avoid}${headings}
    p{margin:0 0 0.7em} img{max-width:100%;height:auto} table{display:table;overflow:visible;border-collapse:collapse;width:100%;table-layout:fixed;break-inside:auto}
    th,td{border:1px solid #777;padding:6px;height:1.5em;vertical-align:top;overflow-wrap:anywhere} tr{break-inside:avoid} thead{display:table-header-group}
    pre,div.sourceCode{break-inside:auto;overflow:visible}
    pre{white-space:pre-wrap;background:#f3f3f3;padding:10px;font-family:Consolas,monospace;font-size:10pt;line-height:1.5;orphans:2;widows:2}
    pre>code,pre>code.sourceCode{font-size:inherit;white-space:pre-wrap;overflow-wrap:anywhere}
    pre>code.sourceCode>span{display:inline;line-height:inherit;text-indent:0;padding-left:0}
    blockquote{margin:1em 0;padding-left:1em;border-left:3px solid #aaa}.katex-display{overflow-wrap:normal;break-inside:avoid}
    [custom-style="ExportSimSun"]{font-family:SimSun}[custom-style="ExportFangSong"]{font-family:FangSong}[custom-style="ExportTimesNewRoman"]{font-family:"Times New Roman"}[custom-style="ExportSimHei"]{font-family:SimHei}
    .code{font-family:Consolas,monospace;background:#f3f3f3}
  `,
  });
  await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all([...document.images].map((image) => image.decode()));
  });
  const pdf = await page.pdf({
    format: 'A4',
    margin: { top: '2.54cm', bottom: '2.54cm', left: '2.54cm', right: '2.54cm' },
    printBackground: true,
    preferCSSPageSize: true,
    displayHeaderFooter: true,
    headerTemplate: '<span></span>',
    footerTemplate:
      '<div style="font:10pt Times New Roman;width:100%;text-align:center"><span class="pageNumber"></span> / <span class="totalPages"></span></div>',
    timeout: 15000,
  });
  if (pdf.length > 32 << 20) throw Error('EXPORT_PDF_ARTIFACT_LIMIT');
  await writeFile(resolve(directory, 'document.pdf'), pdf);
} catch {
  // 不回显公式、正文、临时绝对路径或浏览器日志。
  process.stderr.write('EXPORT_PDF_RENDER_FAILED\n');
  process.exitCode = 1;
} finally {
  await browser?.close();
}
