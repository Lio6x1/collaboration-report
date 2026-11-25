import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { chromium, expect } from '@playwright/test';
import { fromPM, canonical } from '../../packages/content/index.mjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  organizationFixture,
  documentFixture,
  post,
  get,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { createUser, origin } from '../helpers/account.mjs';
import { openReviewWorkspace, writeDraftBody, reviewDatabase } from '../helpers/review.mjs';
import { unlockStructurePreview, structurePreviewInput } from '../helpers/structure-previews.mjs';
import { pollMainControl } from '../helpers/main-controls.mjs';

// 真HTTP/WSS、两个实际Vue编辑器；只创建本次合成组织/文档，不修改用户原稿。
test(
  'real online editors preserve stale branch edits, reject changed previews and reopen split sections',
  { timeout: 120000 },
  async () => {
    let owner;
    const account = '.runtime/private/local-experience-account.json';
    if (existsSync(account)) {
      const credentials = JSON.parse(readFileSync(account, 'utf8'));
      const response = await post('/v1/account/login', {
        email: credentials.email,
        password: credentials.password,
      });
      assert.equal(response.status, 200, 'EXPERIENCE_LOGIN_FAILED');
      owner = {
        email: credentials.email,
        password: credentials.password,
        id: json(response).user_id,
        headers: {
          cookie: response.headers['set-cookie'].map((v) => v.split(';')[0]).join('; '),
          'x-csrf-token': json(response).csrf_token,
        },
      };
    } else owner = await createUser();
    const organization = await organizationFixture(0, owner),
      d = await documentFixture(organization);
    const f = await openReviewWorkspace(organization, d, owner);
    const base = fromPM(yDocToProsemirrorJSON(f.doc));
    const document = await unlockStructurePreview(f);
    const browser = await chromium.launch({ channel: 'msedge', headless: true });
    const database = await reviewDatabase();
    const contexts = [],
      pages = [],
      frames = [[], []],
      errors = [];
    let main, silent;
    try {
      for (let i = 0; i < 2; i++) {
        const context = await browser.newContext({
          ignoreHTTPSErrors: true,
          viewport: { width: 1440, height: 1000 },
        });
        contexts.push(context);
        await context.addCookies(
          owner.headers.cookie.split('; ').map((cookie) => {
            const split = cookie.indexOf('=');
            return {
              name: cookie.slice(0, split),
              value: cookie.slice(split + 1),
              domain: 'localhost',
              path: '/',
              secure: true,
              httpOnly: true,
              sameSite: 'Strict',
            };
          }),
        );
        const page = await context.newPage();
        pages.push(page);
        page.on('pageerror', (e) => errors.push(e.message));
        page.on('websocket', (socket) =>
          socket.on('framereceived', ({ payload }) => {
            try {
              const value = JSON.parse(String(payload));
              frames[i].push(value.type);
            } catch {}
          }),
        );
        page.setDefaultTimeout(15000);
        await page.goto(`${origin}/documents/${d.id}/edit/branch/${d.main.units[0].unit_id}`);
        // 产品按标签独立登录；只塞Cookie不会安装标签内会话，必须走真实登录入口。
        await page.getByLabel('登录邮箱', { exact: true }).fill(owner.email);
        await page.getByLabel('登录密码', { exact: true }).fill(owner.password);
        await page.getByRole('button', { name: '登录', exact: true }).click();
        await expect(
          page.getByRole('region', { name: '共同支线编辑区', exact: true }),
        ).toHaveAttribute('aria-busy', 'false', { timeout: 15000 });
        await expect(
          page.getByRole('textbox', { name: '共同支线编辑器', exact: true }),
        ).toBeEditable();
      }
      f.close();
      const editor = pages[0].getByRole('textbox', { name: '共同支线编辑器', exact: true });
      await editor.fill('同事正在写的未审核内容');
      await pages[0].getByRole('button', { name: '保存当前修改', exact: true }).click();
      await expect(pages[0].getByText('当前修改已保存', { exact: true })).toBeVisible();
      // 在线支线保持旧基线；只修改MAIN增加H2，不打开/拉取支线来人为满足旧条件。
      main = await openReviewWorkspace(organization, d, owner, 'MAIN');
      const accepted = structuredClone(base);
      accepted.content.push(
        {
          type: 'heading',
          attrs: { id: 'online-heading-a', level: 2 },
          content: [{ type: 'text', text: '第一节' }],
        },
        {
          type: 'paragraph',
          attrs: { id: 'online-body-a' },
          content: [{ type: 'text', text: '主线第一节正文' }],
        },
        {
          type: 'heading',
          attrs: { id: 'online-heading-b', level: 2 },
          content: [{ type: 'text', text: '第二节' }],
        },
        {
          type: 'paragraph',
          attrs: { id: 'online-body-b' },
          content: [{ type: 'text', text: '主线第二节正文' }],
        },
      );
      await writeDraftBody(main, accepted);
      main.close();
      main = null;
      await expect(editor.getByRole('heading', { level: 2 })).toHaveCount(0);
      const preview = async (type = 'SPLIT_LEVEL2') => {
        const current = json(await get(`/v1/documents/${d.id}`, owner.headers));
        const input = structurePreviewInput(f, current, type);
        const response = await post(
          `/v1/documents/${d.id}/structure-previews`,
          input,
          owner.headers,
        );
        assert.equal(response.status, 200, json(response).code);
        return { input, value: json(response) };
      };
      const confirm = async (fixed) => {
        const response = await post(
          `/v1/documents/${d.id}/structure-changes`,
          {
            ...fixed.input,
            operation_id: operation('online-confirm'),
            changes: fixed.input.changes.map((c) => ({
              ...c,
              preview_cut_ref: fixed.value.preview_cut_ref,
            })),
          },
          owner.headers,
        );
        assert.equal(response.status, 202, json(response).code);
        return pollMainControl(json(response).operation_id, owner, { timeout: 35000 });
      };
      const old = await preview();
      await editor.press('End');
      await editor.pressSequentially('，预览后又保存');
      await pages[0].getByRole('button', { name: '保存当前修改', exact: true }).click();
      const changed = await confirm(old);
      assert.equal(changed.state, 'REJECTED');
      assert.equal(changed.error.code, 'STRUCTURE_PREVIEW_CHANGED');
      for (const page of pages)
        await expect(
          page.getByRole('textbox', { name: '共同支线编辑器', exact: true }),
        ).toBeEditable();
      const fresh = await preview();
      const installed = await confirm(fresh);
      assert.equal(installed.state, 'SUCCEEDED', installed.error?.code);
      assert.equal(installed.result.main_sync_state, 'READY');
      for (let i = 0; i < pages.length; i++) {
        await pages[i].waitForURL(
          (url) =>
            url.pathname.includes('/edit/branch/') &&
            !url.pathname.endsWith('/' + d.main.units[0].unit_id),
        );
        await expect(
          pages[i].getByRole('textbox', { name: '共同支线编辑器', exact: true }),
        ).toBeEditable();
        assert.ok(frames[i].includes('STRUCTURE_PAUSE'));
        assert.ok(frames[i].includes('STRUCTURE_RESUME'));
        // ACL关闭可能先于安装通知；正文目录重读兜底也必须完成新房间导航。
      }
      await expect(
        pages[0].getByRole('textbox', { name: '共同支线编辑器', exact: true }),
      ).toContainText('预览后又保存');
      const current = json(await get(`/v1/documents/${d.id}/main`, owner.headers));
      assert.equal(current.structure.entries.filter((e) => e.kind === 'SECTION').length, 2);
      const [versions] = await database.query(
        'SELECT canonical_body FROM content_version WHERE id IN (?)',
        [current.units.map((u) => u.content_version_id)],
      );
      assert.doesNotMatch(
        versions.map((v) => Buffer.from(v.canonical_body).toString()).join(''),
        /同事正在写|预览后又保存/,
      );
      // 一个不应答的实际旧客户端不能被当成保存完成；等待有限期限后取消，原目录和在线编辑恢复。
      silent = await openReviewWorkspace(organization, { ...d, main: current }, owner);
      const joined = await preview('JOIN_CHAPTER');
      const refused = await confirm(joined);
      assert.equal(refused.state, 'REJECTED');
      assert.equal(refused.error.code, 'STRUCTURE_EDITORS_NOT_READY');
      const after = json(await get(`/v1/documents/${d.id}/main`, owner.headers));
      assert.equal(after.structure.id, current.structure.id);
      await expect(
        pages[0].getByRole('textbox', { name: '共同支线编辑器', exact: true }),
      ).toBeEditable();
      assert.deepEqual(errors, []);
      const folder = 'artifacts/reported-usability/safe-structure';
      mkdirSync(folder, { recursive: true });
      await pages[0].screenshot({ path: folder + '/preserved-branch.png', fullPage: true });
      writeFileSync(
        folder + '/result.json',
        JSON.stringify(
          {
            document: d.id,
            structure: after.structure.id,
            previewChanged: changed.error.code,
            ready: installed.result.main_sync_state,
            missingAck: refused.error.code,
            frames,
            errors,
          },
          null,
          2,
        ),
      );
    } catch (error) {
      const folder = 'artifacts/reported-usability/safe-structure';
      mkdirSync(folder, { recursive: true });
      writeFileSync(
        folder + '/failure.json',
        JSON.stringify(
          { message: error.message, frames, urls: pages.map((p) => p.url()), errors },
          null,
          2,
        ),
      );
      if (pages[0] && !pages[0].isClosed())
        await pages[0].screenshot({ path: folder + '/failure.png', fullPage: true });
      throw error;
    } finally {
      f.close();
      main?.close();
      silent?.close();
      for (const context of contexts) await context.close();
      await browser.close();
      await database.end();
    }
  },
);
