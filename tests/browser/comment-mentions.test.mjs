import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { resolve } from 'node:path';
import { reviewFixture, openReviewWorkspace, writeDraftBody } from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { uploadPNG, binaryResourceGet } from '../helpers/resource.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';

test(
  '浏览器真实提及打开原主线讨论及历史图片，撤权后原目标一并拒绝',
  { timeout: 90000 },
  async () => {
    const f = await reviewFixture(1);
    const member = f.f.members[0];
    let main, browser;
    try {
      const grant = {
        user_id: member.id,
        scope_kind: 'DOCUMENT',
        scope_id: f.d.id,
        capability: 'COMMENT',
      };
      async function permission(action) {
        const current = await get(`/v1/documents/${f.d.id}`, f.user.headers);
        assert.equal(current.status, 200, current.body);
        const result = await waitAuthorization(
          await post(
            `/v1/documents/${f.d.id}/authorization-changes`,
            {
              operation_id: operation('browser-mention-permission'),
              expected_acl_revision: json(current).acl_revision,
              changes: [{ action, grant }],
            },
            f.user.headers,
          ),
          f.user,
        );
        assert.equal(result.state, 'SUCCEEDED');
      }
      await permission('GRANT');
      const resource = await uploadPNG(f);
      await unlockStructurePreview(f);
      main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      const oldText = '提及当时的原始主线正文 🧪';
      const newText = '后来主线已经改写且删除图片';
      const body = (text, image) => ({
        type: 'doc',
        content: [
          {
            type: 'paragraph',
            attrs: { id: 'mention-history-paragraph' },
            content: [
              { type: 'text', text },
              ...(image
                ? [
                    {
                      type: 'image',
                      attrs: {
                        resourceId: resource,
                        alt: '提及时原始图片',
                        title: null,
                        width: 1,
                        height: 1,
                      },
                    },
                  ]
                : []),
            ],
          },
        ],
      });
      assert.equal((await writeDraftBody(main, body(oldText, true))).type, 'SAVED');
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const errors = [];
      async function login(user) {
        const context = await browser.newContext({
          ignoreHTTPSErrors: true,
          viewport: { width: 1280, height: 900 },
        });
        const page = await context.newPage();
        page.setDefaultTimeout(10000);
        page.on('pageerror', (error) => errors.push(error.message));
        await page.goto('https://localhost:' + state().ports.edge);
        await page.getByLabel('登录邮箱', { exact: true }).fill(user.email);
        await page.getByLabel('登录密码', { exact: true }).fill(user.password);
        await page.getByRole('button', { name: '登录', exact: true }).click();
        await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
        return page;
      }
      const ownerPage = await login(f.user);
      await ownerPage
        .getByRole('group', { name: '空间类型', exact: true })
        .getByRole('button', { name: '组织空间', exact: true })
        .click();
      await ownerPage.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
      await ownerPage.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
      await ownerPage.getByRole('button', { name: '协作报告', exact: true }).click();
      // 图片节点带有可访问状态文字，不能把整段textContent等同于正文文本。
      // 精确核对段落直接文本节点，避免图片包装影响定位，同时保留原文断言。
      async function originalParagraph(scope) {
        await expect
          .poll(() =>
            scope
              .locator('.content-view .ProseMirror p')
              .first()
              .evaluate((node) =>
                [...node.childNodes]
                  .filter((child) => child.nodeType === Node.TEXT_NODE)
                  .map((child) => child.textContent)
                  .join(''),
              ),
          )
          .toBe(oldText);
      }
      await originalParagraph(ownerPage);
      const panel = ownerPage.getByRole('region', { name: '普通批注与讨论', exact: true });
      const discussion = '请查看这份原始文字与图片，历史锚点不能漂移。';
      await panel.getByLabel('对这份正文发起讨论', { exact: true }).fill(discussion);
      // 通过真实成员目录按钮选择，不向组件状态直接注入用户ID。
      const picker = panel.getByRole('group', { name: '@ 提及协作者（可选）', exact: true });
      await picker.getByLabel('查找要提及的协作者', { exact: true }).focus();
      await picker.locator('button[data-user-id="' + member.id + '"]').click();
      const responsePromise = ownerPage.waitForResponse(
        (response) =>
          response.request().method() === 'POST' &&
          new URL(response.url()).pathname === `/v1/documents/${f.d.id}/comment-threads`,
      );
      await panel.getByRole('button', { name: '发起讨论', exact: true }).click();
      const response = await responsePromise;
      assert.equal(response.status(), 201, await response.text());
      const input = response.request().postDataJSON();
      assert.deepEqual(input.mentions, [member.id]);
      assert.equal(input.context_kind, 'MAIN');
      const thread = (await response.json()).result.entity_id;
      await panel.getByRole('button', { name: new RegExp(`^讨论 ${thread} ·`) }).click();
      await panel.getByText(discussion, { exact: true }).waitFor();

      // 用真实Yjs SAVED推进当前MAIN；接收者从未取得原清单read_view。
      assert.equal((await writeDraftBody(main, body(newText, false))).type, 'SAVED');
      const memberPage = await login(member);
      await memberPage.getByRole('button', { name: '通知', exact: true }).click();
      const notifications = memberPage.getByRole('region', { name: '通知', exact: true });
      const notice = notifications
        .getByRole('listitem')
        .filter({ hasText: '有人在讨论中提到了你' });
      await expect(async () => {
        await notifications.getByRole('button', { name: '刷新通知', exact: true }).click();
        await expect(notice).toBeVisible({ timeout: 1000 });
      }).toPass({ timeout: 10000, intervals: [200, 500] });
      await notice.getByRole('button', { name: '打开相关内容', exact: true }).click();
      const destination = memberPage.getByRole('region', { name: '通知原始讨论', exact: true });
      await originalParagraph(destination);
      assert.equal(await destination.getByText(newText, { exact: true }).count(), 0);
      await destination
        .getByText(`固定正文版本 ${input.anchor.content_version_id} · MAIN ${input.context_id}`, {
          exact: true,
        })
        .waitFor();
      await destination.getByText(discussion, { exact: true }).waitFor();
      const image = destination.getByRole('img', { name: '提及时原始图片', exact: true });
      await expect
        .poll(() => image.evaluate((node) => node.complete && node.naturalWidth === 1), {
          timeout: 10000,
        })
        .toBe(true);
      const originalResourcePath =
        `/v1/resources/${resource}/content?` +
        new URLSearchParams({
          document_id: f.d.id,
          context_kind: 'MAIN',
          context_id: input.context_id,
        });
      const displayed = new URL(await image.getAttribute('src'), memberPage.url());
      assert.equal(displayed.pathname + displayed.search, originalResourcePath);
      const reply = '已查看原始图片并回复 🙂';
      await destination.getByLabel('发表批注', { exact: true }).fill(reply);
      await destination.getByRole('button', { name: '发表批注', exact: true }).click();
      await destination.getByText(reply, { exact: true }).waitFor();
      await destination
        .getByText(
          `原始正文版本 ${input.anchor.content_version_id} · 原始主线清单 ${input.context_id}`,
          { exact: true },
        )
        .waitFor();
      assert.deepEqual(errors, []);
      await memberPage.screenshot({
        path: resolve(root, 'artifacts/batches/06/comment-mention-browser.png'),
        fullPage: true,
      });

      await permission('REVOKE');
      const deniedThread = await get(`/v1/comment-threads/${thread}`, member.headers);
      assert.equal(deniedThread.status, 403, deniedThread.body);
      const deniedImage = await binaryResourceGet(originalResourcePath, member.headers);
      assert.equal(deniedImage.status, 403, deniedImage.body.toString('utf8'));
      assert.deepEqual(errors, []);
    } finally {
      await browser?.close();
      main?.close();
      f.close();
    }
  },
);
