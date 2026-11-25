import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdirSync } from 'node:fs';
import { chromium, expect } from '@playwright/test';
import { organizationFixture, documentFixture, get, post, json } from '../helpers/workspace.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

// 复用私密文件中的专用体验身份，只创建隔离业务文档；真实SMTP环境不新建测试账号。
async function owner() {
  const { email, password } = JSON.parse(
    readFileSync(process.env.COLLABORATION_TEST_ACCOUNT, 'utf8'),
  );
  const response = await post('/v1/account/login', { email, password });
  assert.equal(response.status, 200);
  return {
    email,
    password,
    id: json(response).user_id,
    headers: {
      cookie: response.headers['set-cookie'].map((v) => v.split(';')[0]).join('; '),
      'x-csrf-token': json(response).csrf_token,
    },
  };
}

test(
  'same-tab editors survive refresh and branch reopening preserves both sides',
  { timeout: 120000 },
  async () => {
    const f = await organizationFixture(0, await owner());
    const doc = await documentFixture(f);
    const browser = await chromium.launch({ channel: 'msedge', headless: true });
    const editorTrace = [];
    try {
      const context = await browser.newContext({
        ignoreHTTPSErrors: true,
        viewport: { width: 1440, height: 1000 },
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      let paginateEditor = false,
        sawSecondPage = false,
        editableResponse;
      // 用真实单元制造两页读取边界，不创建101个无关章节；最终编辑票据与保存仍走真实后端。
      await context.route('**/v1/documents/' + doc.id + '/editable-units?**', async (route) => {
        if (!paginateEditor || !route.request().frame().url().includes('/edit/branch/'))
          return route.continue();
        const cursor = new URL(route.request().url()).searchParams.get('cursor');
        if (cursor === 'browser-editable-page-2') {
          sawSecondPage = true;
          return route.fulfill(editableResponse);
        }
        const response = await route.fetch();
        const value = await response.json();
        editableResponse = { response, json: value };
        return route.fulfill({
          response,
          json: { ...value, items: [], has_more: true, next_cursor: 'browser-editable-page-2' },
        });
      });
      const errors = [];
      context.on('page', (tab) => tab.on('pageerror', (error) => errors.push(error.message)));
      page.on('pageerror', (error) => errors.push(error.message));
      await page.goto('https://localhost:' + state().ports.edge + '/documents/' + doc.id);
      await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('heading', { name: '协作报告', exact: true }).waitFor();
      await expect(page.getByRole('textbox', { name: '共同支线编辑器', exact: true })).toHaveCount(
        0,
      );
      await expect(page.getByRole('region', { name: '主线控制', exact: true })).not.toBeVisible();
      await page.getByRole('tab', { name: '文档设置', exact: true }).click();
      await page.getByRole('button', { name: '解锁主线', exact: true }).click();
      await page.getByRole('button', { name: '锁定主线', exact: true }).waitFor();
      await page.getByRole('tab', { name: '正文', exact: true }).click();

      async function openEditor(kind) {
        const choose = page.getByRole('button', { name: '编辑文档', exact: true });
        if ((await choose.getAttribute('aria-expanded')) !== 'true') await choose.click();
        const link = page
          .getByRole('link', { name: kind === 'MAIN' ? '编辑主线' : '编辑共同支线', exact: true })
          .first();
        assert.equal(await link.getAttribute('target'), null);
        await link.click();
        const tab = page;
        assert.equal(context.pages().length, 1, '编辑应在当前标签页打开');
        tab.on('response', async (response) => {
          const path = new URL(response.url()).pathname;
          if (/edit-sessions|pull-main|\/operations\/|content-versions/.test(path)) {
            const value = await response.json().catch(() => ({}));
            editorTrace.push({
              path,
              status: response.status(),
              state: value.state,
              code: value.code,
            });
          }
        });
        tab.on('websocket', (socket) => {
          socket.on('framereceived', ({ payload }) => {
            try {
              const value = JSON.parse(String(payload));
              if (value.type !== 'UPDATE')
                editorTrace.push({
                  type: value.type,
                  code: value.code,
                  epoch: value.workspace_epoch,
                });
            } catch {}
          });
          socket.on('close', () => editorTrace.push({ socket: 'closed' }));
        });
        tab.setDefaultTimeout(12000);
        try {
          await tab.getByText('服务端已保存', { exact: true }).waitFor();
        } catch (error) {
          throw Error(
            '编辑页未就绪：' +
              tab.url() +
              '\n' +
              (await tab.locator('main').innerText()) +
              '\n页面异常：' +
              errors.join('; '),
            { cause: error },
          );
        }
        const editor = tab.getByRole('textbox', {
          name: kind === 'MAIN' ? '主线编辑器' : '共同支线编辑器',
          exact: true,
        });
        // 自动拉取包括保存、合并、关闭旧房间和安装新代次；等待整个打开过程结束。
        await expect(
          tab.getByRole('region', {
            name: kind === 'MAIN' ? '主线编辑区' : '共同支线编辑区',
            exact: true,
          }),
        ).toHaveAttribute('aria-busy', 'false', { timeout: 20000 });
        await expect(editor).toBeEditable();
        return { tab, editor };
      }
      let edit = await openEditor('MAIN');
      await edit.editor.fill('共同基线第一段');
      await edit.editor.press('End');
      await edit.editor.press('Enter');
      await edit.editor.pressSequentially('共同基线第二段');
      await edit.tab.getByRole('button', { name: '保存当前修改', exact: true }).click();
      await edit.tab.getByText('当前修改已保存', { exact: true }).waitFor();
      await edit.tab.getByRole('button', { name: '保存并返回阅读', exact: true }).click();
      await page.getByRole('tab', { name: '正文', exact: true }).waitFor();
      await page.getByRole('button', { name: '刷新阅读', exact: true }).click();
      await expect(page.getByLabel('主线正文', { exact: true }).first()).toContainText(
        '共同基线第二段',
      );

      edit = await openEditor('BRANCH');
      await expect(edit.editor).toContainText('共同基线第二段');
      await edit.editor.locator('p').first().click();
      await edit.editor.press('Home');
      await edit.editor.pressSequentially('支线保留内容 ');
      await edit.tab.getByRole('button', { name: '保存当前修改', exact: true }).click();
      await edit.tab.getByText('当前修改已保存', { exact: true }).waitFor();
      await edit.tab.getByRole('button', { name: '保存并返回阅读', exact: true }).click();
      await page.getByRole('tab', { name: '正文', exact: true }).waitFor();

      edit = await openEditor('MAIN');
      await edit.editor.press('Control+End');
      await edit.editor.press('Enter');
      await edit.editor.pressSequentially('主线最新追加内容');
      await edit.tab.getByRole('button', { name: '保存当前修改', exact: true }).click();
      await edit.tab.getByText('当前修改已保存', { exact: true }).waitFor();
      await edit.tab.getByRole('button', { name: '保存并返回阅读', exact: true }).click();
      await page.getByRole('tab', { name: '正文', exact: true }).waitFor();
      await page.getByRole('button', { name: '刷新阅读', exact: true }).click();
      await expect(page.getByLabel('主线正文', { exact: true }).first()).toContainText(
        '主线最新追加内容',
      );
      paginateEditor = true;
      edit = await openEditor('BRANCH');
      assert.equal(sawSecondPage, true, '编辑路由应按游标找到首页之外的目标单元');
      paginateEditor = false;
      await expect(edit.editor).toContainText('支线保留内容');
      await expect(edit.editor).toContainText('主线最新追加内容');
      await expect(page).toHaveURL(new RegExp('/documents/' + doc.id + '/edit/branch/'));
      const editingURL = page.url();
      await page.reload();
      await expect(edit.editor).toBeEditable();
      await expect(edit.editor).toContainText('支线保留内容');
      await expect(edit.editor).toContainText('主线最新追加内容');
      assert.equal(page.url(), editingURL, '刷新仍保留共同支线编辑路由');
      mkdirSync('artifacts/reported-usability', { recursive: true });
      await edit.tab.screenshot({
        path: 'artifacts/reported-usability/editor-desktop.png',
        fullPage: true,
      });
      await edit.tab.setViewportSize({ width: 390, height: 844 });
      await expect
        .poll(() => edit.tab.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
        .toBe(true);
      await edit.tab.screenshot({
        path: 'artifacts/reported-usability/editor-mobile.png',
        fullPage: true,
      });
      await edit.tab.getByRole('button', { name: '保存并返回阅读', exact: true }).click();
      await page.getByRole('tab', { name: '正文', exact: true }).waitFor();

      await page.getByRole('button', { name: '导出文档', exact: true }).click();
      await expect(page.getByRole('button', { name: '创建导出', exact: true })).toBeVisible();
      await page.getByRole('tab', { name: '协作', exact: true }).click();
      await expect(page.getByLabel('选择协作者', { exact: true })).toBeVisible();
      await expect(page.getByRole('heading', { name: '审核', exact: true })).toHaveCount(0);
      await page.getByRole('tab', { name: '评论', exact: true }).click();
      await expect(page.getByRole('heading', { name: '评论', exact: true })).toBeVisible();
      const commentDraft = page.getByLabel('对这份正文发起讨论', { exact: true });
      await commentDraft.fill('尚未提交的评论');
      await page.evaluate(() => window.dispatchEvent(new Event('focus')));
      await expect(commentDraft).toHaveValue('尚未提交的评论');
      await expect(page.getByLabel('选择协作者', { exact: true })).toHaveCount(0);
      await page.getByRole('tab', { name: '审核', exact: true }).click();
      await expect(page.getByRole('heading', { name: '审核', exact: true })).toBeVisible();
      await expect(page.getByRole('heading', { name: '评论', exact: true })).not.toBeVisible();
      await page.getByRole('tab', { name: '评论', exact: true }).click();
      await expect(commentDraft).toHaveValue('尚未提交的评论');
      await commentDraft.fill('');
      const collaborators = await get(
        '/v1/documents/' + doc.id + '/collaborators',
        f.owner.headers,
      );
      assert.equal(collaborators.status, 200);
      assert.ok(json(collaborators).items.some((candidate) => candidate.user_id === f.owner.id));
      assert.ok(
        [401, 403].includes((await get('/v1/documents/' + doc.id + '/collaborators')).status),
      );

      mkdirSync('artifacts/reported-usability', { recursive: true });
      await page.getByRole('tab', { name: '正文', exact: true }).click();
      await page.screenshot({
        path: 'artifacts/reported-usability/document-desktop.png',
        fullPage: true,
      });
      await page.setViewportSize({ width: 390, height: 844 });
      await expect
        .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
        .toBe(true);
      await page.screenshot({
        path: 'artifacts/reported-usability/document-mobile.png',
        fullPage: true,
      });
      assert.deepEqual(errors, []);
    } catch (error) {
      // 只输出状态与错误码；票据、Cookie和正文帧不进入诊断记录。
      console.error('EDITOR_TRACE', JSON.stringify(editorTrace.slice(-60)));
      throw error;
    } finally {
      await browser.close();
    }
  },
);
