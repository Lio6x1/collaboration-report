import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { chromium, expect } from '@playwright/test';
import { organizationFixture, documentFixture, post, json } from '../helpers/workspace.mjs';
import { existingTestAccount } from '../helpers/account.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

const png = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGPwKAj+DwAEGgILBkOQigAAAABJRU5ErkJggg==',
  'base64',
);

async function paste(editor, buffer = png, type = 'image/png') {
  await editor.evaluate(
    (element, { bytes, type }) => {
      const data = new DataTransfer();
      data.items.add(new File([new Uint8Array(bytes)], '粘贴图片.png', { type }));
      element.dispatchEvent(
        new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }),
      );
    },
    { bytes: [...buffer], type },
  );
}
async function openEditor(page, kind = 'BRANCH') {
  await page.getByRole('button', { name: '编辑文档', exact: true }).click();
  const path = await page
    .getByRole('link', { name: kind === 'MAIN' ? '编辑主线' : '编辑共同支线', exact: true })
    .first()
    .getAttribute('href');
  // 同页导航由工作台场景覆盖；本文件沿公开href核验上传、恢复与图片位置。
  await page.goto(new URL(path, page.url()).href);
  await expect(
    page.getByRole('region', {
      name: kind === 'MAIN' ? '主线编辑区' : '共同支线编辑区',
      exact: true,
    }),
  ).toHaveAttribute('aria-busy', 'false', { timeout: 20000 });
  await expect(
    page.getByRole('textbox', {
      name: kind === 'MAIN' ? '主线编辑器' : '共同支线编辑器',
      exact: true,
    }),
  ).toBeEditable();
}

// 已知的配额冲突没有建立预留；必须读取新版本并新建请求，不能永远重送旧版本。
// 只让一次配额读取过期，409拒绝、后续上传和保存全部来自真实服务。
test(
  'a rejected quota revision refreshes and retries the selected image without reserving twice',
  { timeout: 45000 },
  async () => {
    const f = await organizationFixture(0, await existingTestAccount());
    const doc = await documentFixture(f);
    const browser = await chromium.launch({ channel: 'msedge', headless: true });
    try {
      const context = await browser.newContext({ ignoreHTTPSErrors: true });
      const page = await context.newPage();
      page.setDefaultTimeout(8000);
      await page.goto('https://localhost:' + state().ports.edge + '/documents/' + doc.id);
      await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('heading', { name: '协作报告', exact: true }).waitFor();
      await openEditor(page);
      await page.getByText('服务端已保存', { exact: true }).waitFor();
      const requests = [],
        rejected = [];
      page.on('request', (request) => {
        if (
          /\/documents\/\d+\/resources$/.test(new URL(request.url()).pathname) &&
          request.method() === 'POST'
        )
          requests.push(request.postDataJSON());
      });
      page.on('response', async (response) => {
        if (
          /\/documents\/\d+\/resources$/.test(new URL(response.url()).pathname) &&
          response.status() === 409
        )
          rejected.push((await response.json()).code);
      });
      let stale = true;
      await page.route('**/v1/spaces/' + f.space + '/quota', async (route) => {
        const response = await route.fetch();
        if (!stale) return route.fulfill({ response });
        stale = false;
        const quota = await response.json();
        quota.revision = String(BigInt(quota.revision) + 1n);
        await route.fulfill({ response, json: quota });
      });
      await page.getByText('图片与上传', { exact: true }).click();
      await page
        .getByLabel('选择正文图片')
        .setInputFiles({ name: '配额重试图片.png', mimeType: 'image/png', buffer: png });
      await page.getByRole('button', { name: '上传图片', exact: true }).click();
      await expect.poll(() => requests.length).toBe(2);
      await page.getByRole('button', { name: '将已上传图片插入正文', exact: true }).click();
      assert.deepEqual(rejected, ['QUOTA_REVISION_CHANGED']);
      assert.notEqual(requests[0].operation_id, requests[1].operation_id);
      assert.notEqual(requests[0].expected_quota_revision, requests[1].expected_quota_revision);
      assert.equal(requests[0].checksum, requests[1].checksum);
      assert.equal(requests[0].byte_size, requests[1].byte_size);
      await page.getByRole('button', { name: '保存当前修改', exact: true }).click();
      await page.getByText('当前修改已保存', { exact: true }).waitFor();
      const image = page
        .getByRole('textbox', { name: '共同支线编辑器', exact: true })
        .locator('.resource-image img');
      await expect(image).toBeVisible();
      assert.equal(await image.evaluate((node) => node.complete && node.naturalWidth === 1), true);
      const quota = await page.evaluate(
        async (space) => (await fetch('/v1/spaces/' + space + '/quota')).json(),
        f.space,
      );
      assert.equal(quota.reserved_bytes, '0', '上传结束后没有遗留或重复预留');
      assert.ok(BigInt(quota.used_bytes) >= BigInt(png.length));
      await context.close();
    } finally {
      await browser.close();
    }
  },
);

// 在真实编辑器中贯穿剪贴板文件→原资源上传→Yjs保存→重新打开。
// 只新建自己的测试文档；真实SMTP环境使用显式提供的体验账号，不发测试邮件。
test(
  'clipboard images keep their paste position, survive upload retry and reopen in branch and main',
  { timeout: 60000 },
  async () => {
    const f = await organizationFixture(0, await existingTestAccount());
    const doc = await documentFixture(f);
    const browser = await chromium.launch({ channel: 'msedge', headless: true });
    let releaseUpload;
    try {
      const context = await browser.newContext({ ignoreHTTPSErrors: true });
      const page = await context.newPage();
      page.setDefaultTimeout(8000);
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.goto('https://localhost:' + state().ports.edge + '/documents/' + doc.id);
      await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
      async function openDocument() {
        await page.goto('https://localhost:' + state().ports.edge + '/documents/' + doc.id);
      }
      await openDocument();
      await openEditor(page);
      const branch = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
      await page.getByText('服务端已保存', { exact: true }).waitFor();
      await branch.fill('原粘贴位置');
      await page.getByText('服务端已保存', { exact: true }).waitFor();
      await branch.press('Control+End');
      let attempts = 0;
      const uploadGate = new Promise((resolve) => {
        releaseUpload = resolve;
      });
      const handles = [];
      page.on('request', (request) => {
        if (
          /\/documents\/\d+\/resources$/.test(new URL(request.url()).pathname) &&
          request.method() === 'POST'
        )
          handles.push(request.postDataJSON());
      });
      await page.route('**/v1/resources/*/upload', async (route) => {
        attempts++;
        if (attempts === 1) {
          await uploadGate;
          await route.abort('failed');
        } else await route.continue();
      });
      await paste(branch);
      try {
        await expect.poll(() => attempts).toBe(1);
      } catch (error) {
        throw Error(await page.locator('.resource-upload').innerText(), { cause: error });
      }
      // 上传未完成时在前方插字并移动光标，异步结果仍应跟随原来的协作位置。
      // 上传浮层切换只隐藏视图，原上传与粘贴锚点不能被销毁。
      await page.getByRole('button', { name: '导入 Markdown', exact: true }).click();
      const markdown = page.getByRole('dialog', { name: '导入 Markdown', exact: true });
      await markdown.getByLabel('Markdown 正文', { exact: true }).fill('尚未插入的草稿');
      await page.keyboard.press('Escape');
      await expect(markdown).not.toBeVisible();
      await expect(page.getByRole('button', { name: '导入 Markdown', exact: true })).toBeFocused();
      await page.getByRole('button', { name: '导入 Markdown', exact: true }).click();
      await expect(markdown.getByLabel('Markdown 正文', { exact: true })).toHaveValue(
        '尚未插入的草稿',
      );
      await page.getByRole('button', { name: '图片与上传', exact: true }).click();
      await expect(markdown).not.toBeVisible();
      await branch.press('Control+Home');
      await page.keyboard.insertText('前缀');
      releaseUpload();
      await page
        .locator('.resource-upload')
        .getByRole('button', { name: '确认操作结果', exact: true })
        .click();
      const branchImages = branch.locator('.resource-image img');
      await expect(branchImages).toHaveCount(1);
      await page.getByRole('button', { name: '保存当前修改', exact: true }).click();
      await page.getByText('当前修改已保存', { exact: true }).waitFor();
      await expect(branchImages).toBeVisible();
      assert.equal(
        await branchImages.evaluate((node) => node.complete && node.naturalWidth === 1),
        true,
      );
      const beforeImage = await branchImages.evaluate((node) => {
        const range = document.createRange();
        range.selectNodeContents(node.closest('p'));
        range.setEndBefore(node.closest('.resource-image'));
        return range.toString();
      });
      assert.equal(beforeImage, '前缀原粘贴位置');
      assert.equal(
        new Set(handles.map((handle) => handle.operation_id)).size,
        1,
        '重试沿用原配额预留编号',
      );
      await page.reload();
      await expect(branch).toBeEditable();
      await expect(branchImages).toBeVisible();
      assert.equal(
        await branchImages.evaluate((node) => node.complete && node.naturalWidth === 1),
        true,
      );

      await page.getByRole('button', { name: '保存并返回阅读', exact: true }).click();
      await page.getByRole('tab', { name: '文档设置', exact: true }).click();
      await page.getByRole('button', { name: '解锁主线', exact: true }).click();
      await page.getByRole('button', { name: '锁定主线', exact: true }).waitFor();
      await openEditor(page, 'MAIN');
      const main = page.getByRole('textbox', { name: '主线编辑器', exact: true });
      await page.getByText('服务端已保存', { exact: true }).waitFor();
      await main.focus();
      await paste(main);
      const mainImages = main.locator('.resource-image img');
      await expect(mainImages).toHaveCount(1);
      await page.getByRole('button', { name: '保存当前修改', exact: true }).click();
      await page.getByText('当前修改已保存', { exact: true }).waitFor();
      await expect(mainImages).toBeVisible();
      assert.equal(
        await mainImages.evaluate((node) => node.complete && node.naturalWidth === 1),
        true,
      );

      // 编辑正文后外点关闭浮层，重新打开仍可继续原资源上传用例。
      await page.getByRole('button', { name: '图片与上传', exact: true }).click();
      // 文件选择入口仍走同一上传流程，补齐粘贴不能破坏原来的手动上传。
      await page
        .getByLabel('选择正文图片')
        .setInputFiles({ name: '手动图片.png', mimeType: 'image/png', buffer: png });
      await page.getByRole('button', { name: '上传图片', exact: true }).click();
      await page.getByRole('button', { name: '将已上传图片插入正文', exact: true }).click();
      await expect(mainImages).toHaveCount(2);
      await page.getByRole('button', { name: '保存当前修改', exact: true }).click();
      await page.getByText('当前修改已保存', { exact: true }).waitFor();
      await page.reload();
      await expect(main).toBeEditable();
      await expect(mainImages).toHaveCount(2);
      for (const image of await mainImages.all()) {
        await expect(image).toBeVisible();
        assert.equal(
          await image.evaluate((node) => node.complete && node.naturalWidth === 1),
          true,
        );
        assert.match(await image.getAttribute('src'), /^\/v1\/resources\/\d+\/content\?/);
      }
      // 小四与段落格式走真实Yjs保存，刷新不能只保留视觉样式而丢业务属性。
      await main.locator('p').first().click();
      await main.press('Home');
      await page.getByRole('button', { name: '居中对齐', exact: true }).click();
      await page.getByRole('button', { name: '首行缩进', exact: true }).click();
      await expect(main.locator('p').first()).toHaveCSS('text-align', 'center');
      await expect(main.locator('p').first()).toHaveCSS('text-indent', '32px');
      await expect(main).toHaveCSS('font-size', '16px');
      const image = mainImages.last();
      await image.click();
      const handle = page.getByRole('button', { name: '等比例调整图片大小', exact: true });
      await expect(handle).toBeVisible();
      const start = await handle.boundingBox();
      await page.mouse.move(start.x + start.width / 2, start.y + start.height / 2);
      await page.mouse.down();
      await page.mouse.move(start.x + start.width / 2 + 96, start.y + start.height / 2 + 45, {
        steps: 4,
      });
      await page.mouse.up();
      const size = await image.evaluate((node) => ({
        width: Number(node.getAttribute('width')),
        height: Number(node.getAttribute('height')),
      }));
      assert.ok(size.width >= 90, '拖动应更新规范正文中的图片尺寸');
      assert.equal(size.width, size.height, '拖动纵向偏移不能改变原图片比例');
      await page.getByRole('button', { name: '保存当前修改', exact: true }).click();
      await page.getByText('当前修改已保存', { exact: true }).waitFor();
      const editingURL = page.url();
      // 文件仍存在时一次网络中断不能让图片永久停在占位提示；第二次读取仍走真实鉴权和OSS。
      const resourcePath = new URL(await mainImages.last().getAttribute('src'), page.url())
        .pathname;
      let readAttempts = 0;
      await page.route('**' + resourcePath + '?**', async (route) => {
        if (++readAttempts === 1) await route.abort('timedout');
        else await route.continue();
      });
      await page.reload();
      await expect(main).toBeEditable();
      await expect(mainImages.last()).toBeVisible();
      assert.equal(
        await mainImages.last().evaluate((node) => node.complete && node.naturalWidth === 1),
        true,
        '网络恢复后无需编辑正文或手动刷新即可显示原图',
      );
      assert.equal(readAttempts, 2, '只重试本次失败的图片');
      await page.unroute('**' + resourcePath + '?**');
      assert.equal(page.url(), editingURL, '刷新直接恢复主线编辑');
      await expect(main.locator('p').first()).toHaveCSS('text-align', 'center');
      await expect(main.locator('p').first()).toHaveCSS('text-indent', '32px');
      await expect(mainImages.last()).toHaveAttribute('width', String(size.width));
      await expect(mainImages.last()).toHaveAttribute('height', String(size.height));
      await page.getByRole('button', { name: '保存并返回阅读', exact: true }).click();
      const reader = page.getByLabel('主线正文', { exact: true }).first();
      await expect(reader.locator('p').first()).toHaveCSS('text-align', 'center');
      await expect(reader.locator('p').first()).toHaveCSS('text-indent', '32px');
      await expect(reader.locator('.resource-image img').last()).toHaveAttribute(
        'width',
        String(size.width),
      );
      await expect(
        page.getByRole('button', { name: '等比例调整图片大小', exact: true }),
      ).toHaveCount(0);
      assert.deepEqual(errors, []);
      mkdirSync('artifacts/reported-usability', { recursive: true });
      writeFileSync(
        'artifacts/reported-usability/authoring-document.json',
        JSON.stringify({ id: doc.id }),
      );
      await context.close();
    } finally {
      releaseUpload?.();
      await browser.close();
    }
  },
);
