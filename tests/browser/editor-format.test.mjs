import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import * as Y from 'yjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

async function open(page, f) {
  await page.goto('https://localhost:' + state().ports.edge);
  await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
  await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await page
    .getByRole('group', { name: '空间类型', exact: true })
    .getByRole('button', { name: '组织空间', exact: true })
    .click();
  await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
  await page.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
  await page.getByRole('button', { name: '协作报告', exact: true }).click();
  await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
  await page.getByRole('textbox', { name: '共同支线编辑器', exact: true }).waitFor();
}
async function body(page) {
  const snapshot = await page.evaluate(
    () =>
      new Promise((done, reject) => {
        const open = indexedDB.open('collaboration-report.drafts', 1);
        open.onerror = () => reject(open.error);
        open.onsuccess = () => {
          const r = open.result.transaction('queues').objectStore('queues').getAll();
          r.onsuccess = () => {
            done(r.result[0].snapshot);
            open.result.close();
          };
        };
      }),
  );
  const doc = new Y.Doc();
  try {
    Y.applyUpdate(doc, Buffer.from(snapshot, 'base64'));
    return yDocToProsemirrorJSON(doc);
  } finally {
    doc.destroy();
  }
}

test('changing an existing paragraph into a heading preserves its actual persisted block identity', async () => {
  const f = await organizationFixture(0);
  await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(8000);
    await open(page, f);
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await editor.fill('保留原块身份');
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    const before = await body(page);
    await editor.focus();
    await page.getByRole('button', { name: '二级标题', exact: true }).click();
    await editor.locator('h2').waitFor();
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    const after = await body(page);
    // 格式变化仍是同一块。换ID会让审核合并把它误认成删除旧块、创建新块。
    assert.equal(after.content[0].type, 'heading');
    assert.equal(after.content[0].attrs.id, before.content[0].attrs.id);
    await context.close();
  } finally {
    await browser.close();
  }
});

test('a real formula copy and paste keeps independent strong and strike atom attributes in persisted content', async () => {
  const f = await organizationFixture(0);
  await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({
        ignoreHTTPSErrors: true,
        permissions: ['clipboard-read', 'clipboard-write'],
      }),
      page = await context.newPage();
    page.setDefaultTimeout(8000);
    await open(page, f);
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await page.getByLabel('公式 LaTeX', { exact: true }).fill('x^2');
    await page.getByRole('button', { name: '插入行内公式', exact: true }).click();
    await editor.locator('.formula-atom').click();
    await page.getByRole('button', { name: '粗体', exact: true }).click();
    await page.getByRole('button', { name: '删除线', exact: true }).click();
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    const before = await body(page);
    assert.ok(before.content[0].content[0].attrs.atomStrong);
    assert.ok(before.content[0].content[0].attrs.atomStrike);
    // 让浏览器实际触发PM的DOMSerializer和parseDOM；不能以JSON往返代替剪贴板格式验证。
    await editor.focus();
    await page.keyboard.press('Control+A');
    await page.keyboard.press('Control+C');
    const copied = await page.evaluate(async () => {
      for (const item of await navigator.clipboard.read())
        if (item.types.includes('text/html')) return (await item.getType('text/html')).text();
      return '';
    });
    assert.match(copied, /data-formula=/);
    await page.keyboard.press('Control+End');
    // 用真实复制得到的HTML触发浏览器paste事件，避免并发测试争抢Windows全局剪贴板。
    await editor.evaluate((element, html) => {
      const data = new DataTransfer();
      data.setData('text/html', html);
      element.dispatchEvent(
        new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }),
      );
    }, copied);
    await editor.locator('.formula-atom').nth(1).waitFor();
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    const after = await body(page),
      atoms = after.content.flatMap((n) => n.content ?? []).filter((n) => n.type === 'formula');
    assert.equal(atoms.length, 2);
    for (const atom of atoms) {
      assert.equal(atom.attrs.latex, 'x^2');
      assert.equal(atom.attrs.atomStrong, true);
      assert.equal(atom.attrs.atomStrike, true);
    }
    await context.close();
  } finally {
    await browser.close();
  }
});

test('Markdown import reaches the real saved draft with headings, code language, formatted formula and explicit pending images', async () => {
  const f = await organizationFixture(0);
  await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(8000);
    await open(page, f);
    await page.locator('.markdown-import summary').click();
    await page
      .getByLabel('Markdown 正文', { exact: true })
      .fill(
        '## 小节\n\n**$x^2$**\n\n```go\nfmt.Println("Go")\n```\n\n![示意图](https://example.invalid/image.png)',
      );
    await page.getByRole('button', { name: '将 Markdown 加入当前草稿', exact: true }).click();
    await page
      .getByRole('textbox', { name: '共同支线编辑器', exact: true })
      .locator('h2')
      .waitFor();
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    const actual = await body(page),
      formula = actual.content.flatMap((n) => n.content ?? []).find((n) => n.type === 'formula');
    assert.equal(formula.attrs.atomStrong, true);
    assert.equal(actual.content.find((n) => n.type === 'code_block').attrs.language, 'go');
    assert.match(
      await page.getByRole('textbox', { name: '共同支线编辑器', exact: true }).innerText(),
      /图片待转存：示意图/,
    );
    assert.match(
      await page
        .locator('.unit-editor .hint')
        .allTextContents()
        .then((v) => v.join(' ')),
      /外部图片尚未转存/,
    );
    await context.close();
  } finally {
    await browser.close();
  }
});

test('formatting a mixed text and formula range changes independent atom fields rather than wrapping the atom in marks', async () => {
  const f = await organizationFixture(0);
  await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(8000);
    await open(page, f);
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await editor.fill('混合文本');
    await page.getByRole('button', { name: '插入行内公式', exact: true }).click();
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    await editor.focus();
    await page.keyboard.press('Control+A');
    await page.getByRole('button', { name: '粗体', exact: true }).click();
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    const bold = await body(page),
      atom = bold.content[0].content.find((n) => n.type === 'formula');
    assert.equal(atom.attrs.atomStrong, true);
    assert.ok(!atom.marks?.length);
    assert.ok(
      bold.content[0].content.find((n) => n.type === 'text').marks.some((m) => m.type === 'strong'),
    );
    await editor.focus();
    await page.keyboard.press('Control+A');
    await page.getByRole('button', { name: '粗体', exact: true }).click();
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    const plain = await body(page);
    assert.equal(
      plain.content[0].content.find((n) => n.type === 'formula').attrs.atomStrong,
      false,
    );
    assert.ok(
      !plain.content[0].content
        .find((n) => n.type === 'text')
        .marks?.some((m) => m.type === 'strong'),
    );
    await context.close();
  } finally {
    await browser.close();
  }
});
