# 共享正文适配

浏览器和协作服务共用 `index.mjs` 的 PM schema、原子字段、规范字节和候选校验。业务 JSON 不保存永久外部图片 URL；`toPM` 补的 src 只服务显示，`fromPM` 去掉 src 再进入正式内容模型。

段落和标题可选保存 `attrs.align`（`left`、`center`、`right`）与 `attrs.firstLineIndent`（布尔值，`true` 表示首行两字符）。PM 默认值为 `left`/`false`；`fromPM` 移除这两个默认值，`normalizeContent` 不为旧正文补字段，保证未编辑正文的规范字节与哈希稳定。格式通过实际 DOM CSS、Word 原生段落样式及 PDF 输入 HTML 保留；Markdown 导出使用受控 HTML/CSS，阅读器需支持 HTML/CSS 才能显示对应格式。

`contracts/content/content.schema.json` 与五个原规范字节向量保留冻结字节。段落字段来自独立 `paragraph-formatting.extension.json`，新格式向量来自 `paragraph-formatting-vectors.json`；`scripts/load-content-contract.mjs` 和 Go 嵌入加载器只接受这两个节点/两个可选属性，拒绝覆盖冻结字段或放开未知属性。

`validate.mjs` 是冻结 JSON schema 与该受限扩展经 AJV 8.17.1 生成的代码。不能手改。运行时不会编译字符串，浏览器保留 `script-src 'self'`，无需 unsafe-eval。PowerShell 在项目根运行：

```powershell
node scripts/generate-content.mjs
node scripts/generate-content.mjs --check
node --test services/collaboration/tests/content.test.mjs services/collaboration/tests/markdown.test.mjs
```

Markdown 解析使用锁定的 markdown-it 14.3.0，公式规则只生成正文原子，KaTeX 显示关闭 trust；块身份由调用方提供的新 ID 生成器分配。外部图片没有 READY 资源映射时保持待转存文本，并返回损失说明及 URL 清单，不执行 HTML 或悄悄变成可引用图片。完整受控下载、转存和上传仍需真实资源接口；引擎转换测试不能替代资源/OSS验收。

命令工具使用 prosemirror-commands 1.7.2。库变更记录见 [Markdown 维护记录](https://github.com/markdown-it/markdown-it/blob/master/CHANGELOG.md) 和 [ProseMirror 官方变更记录](https://prosemirror.net/docs/changelog/)；已安装版本及完整性摘要以项目锁文件为准。
