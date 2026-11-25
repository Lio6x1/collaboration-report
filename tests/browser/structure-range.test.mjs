import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import { importMarkdown } from '../../packages/content/markdown.mjs';
import { canonical, fromPM, normalizeContent } from '../../packages/content/index.mjs';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraftBody,
  writeDraft,
} from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { get, json } from '../helpers/workspace.mjs';
import { pullMain } from '../helpers/pull-main.mjs';
import { state, root } from '../../scripts/runtime-lib.mjs';

// 如果确认不使用原预览引用、没有等待真实新支线安装或把S2写进MAIN，以下真实读/WSS会失败。
test(
  'real browser previews and confirms split, join and removal while retaining the reliable S2 draft',
  { timeout: 90000 },
  async () => {
    const f = await reviewFixture();
    const opened = [];
    let browser;
    let page;
    const errors = [];
    try {
      await unlockStructurePreview(f);
      const mainWorkspace = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      opened.push(mainWorkspace);
      let next = 0;
      const accepted = importMarkdown(
        '## 第一节\n\n已接纳一\n\n## 第二节\n\n**已接纳二**与 $x^2$',
        { newId: () => `browser-range-${++next}` },
      ).body;
      const pending = structuredClone(accepted);
      pending.content.push({
        type: 'paragraph',
        attrs: { id: 'browser-range-S2' },
        content: [{ type: 'text', text: '拆合后保留的未审核S2' }],
      });

      await writeDraftBody(mainWorkspace, accepted);
      assert.equal((await pullMain(f)).value.state, 'SUCCEEDED');
      f.close();
      const synced = await openReviewWorkspace(f.f, f.d, f.user);
      opened.push(synced);
      await writeDraftBody(synced, pending);
      synced.close();
      mainWorkspace.close();
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 960 },
      });
      page.on('pageerror', (error) => errors.push(error.message));
      page.setDefaultTimeout(8000);
      await page.goto('https://localhost:' + state().ports.edge);
      await page.getByLabel('登录邮箱', { exact: true }).fill(f.user.email);
      await page.getByLabel('登录密码', { exact: true }).fill(f.user.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
      await page.goto('https://localhost:' + state().ports.edge + '/documents/' + f.d.id);
      await page.getByRole('tab', { name: '文档设置', exact: true }).click();
      await page.getByRole('region', { name: '主线控制', exact: true }).waitFor();
      const controls = page.getByRole('region', { name: '目录管理', exact: true });
      await controls.waitFor();
      const apply = async (type) => {
        const before = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
        await controls.getByRole('button', { name: '章节操作：第一章', exact: true }).click();
        const label = { SPLIT_LEVEL2: '按二级标题拆分', JOIN_CHAPTER: '合章', REMOVE: '删除章节' }[
          type
        ];
        await controls
          .getByRole('button', { name: label, exact: true })
          .filter({ visible: true })
          .click();
        const dialog = page.getByRole('dialog', { name: label, exact: true });
        await dialog.getByRole('button', { name: '生成固定目录预览', exact: true }).click();
        const preview = dialog.getByRole('region', { name: '固定目录预览', exact: true });
        await preview.waitFor();
        assert.equal(
          await preview.getByRole('button', { name: '确认目录范围调整', exact: true }).isDisabled(),
          true,
        );
        assert.equal(
          json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers)).structure.id,
          before.structure.id,
          '生成预览不能切换目录',
        );
        await preview.getByLabel('我已确认目录范围变化', { exact: true }).check();
        await preview.getByRole('button', { name: '确认目录范围调整', exact: true }).click();
        await preview.waitFor({ state: 'detached' });
        return json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
      };
      const split = await apply('SPLIT_LEVEL2');
      assert.equal(split.units.length, 2);
      assert.ok(split.units.every((unit) => unit.unit_id !== f.d.main.units[0].unit_id));
      const joined = await apply('JOIN_CHAPTER');
      assert.equal(joined.units.length, 1);
      const fixed = json(
        await get(
          `/v1/documents/${f.d.id}/read-views/${joined.read_view_id}/units/${joined.units[0].unit_id}`,
          f.user.headers,
        ),
      );
      assert.equal(canonical(fixed.body_json), canonical(accepted));
      const branch = await openReviewWorkspace(f.f, { ...f.d, main: joined }, f.user);
      opened.push(branch);
      // PM/Yjs省略默认null属性；按既有传输边界补齐业务形式，再逐字比较全部原正文和独立S2。
      assert.equal(
        canonical(normalizeContent(fromPM(yDocToProsemirrorJSON(branch.doc)))),
        canonical(pending),
      );
      // 该测试会话不实现浏览器的冻结确认协议，核验完正文后主动退出再删除。
      branch.close();
      assert.deepEqual(errors, []);
      await page.screenshot({
        path: resolve(root, 'artifacts/batches/04/structure-range-browser.png'),
        fullPage: true,
      });
      const empty = await apply('REMOVE');
      assert.deepEqual(empty.units, []);
      assert.deepEqual(empty.structure.entries, []);
    } catch (error) {
      if (page) {
        console.error(
          JSON.stringify({
            pageErrors: errors,
            feedback: await page.locator('.feedback').allTextContents(),
          }),
        );
        await page.screenshot({
          path: resolve(root, 'artifacts/batches/04/structure-range-browser-failure.png'),
          fullPage: true,
        });
      }
      throw error;
    } finally {
      for (const workspace of opened) workspace.close();
      f.close();
      await browser?.close();
    }
  },
);

// 丢响应后的预览只能重取原号；后来可靠输入必须让原确认明确CHANGED，不能静默更新摘要。
test('a lost real structure preview resumes the same input and later saved content rejects its original confirmation', async () => {
  const f = await reviewFixture();
  let browser;
  try {
    await unlockStructurePreview(f);
    await writeDraft(f, '固定目录预览时的可靠草稿');
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true });
    page.setDefaultTimeout(8000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(f.user.email);
    await page.getByLabel('登录密码', { exact: true }).fill(f.user.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page.goto('https://localhost:' + state().ports.edge + '/documents/' + f.d.id);
    await page.getByRole('tab', { name: '文档设置', exact: true }).click();
    const controls = page.getByRole('region', { name: '目录管理', exact: true });
    await controls.waitFor();
    const inputs = [];
    let original;
    await page.route('**/v1/documents/*/structure-previews', async (route) => {
      inputs.push(route.request().postDataJSON());
      const response = await route.fetch();
      if (inputs.length === 1) {
        assert.equal(response.status(), 200);
        original = await response.json();
        await route.abort('failed');
      } else await route.fulfill({ response });
    });
    await controls.getByRole('button', { name: '章节操作：第一章', exact: true }).click();
    await controls
      .getByRole('button', { name: '删除章节', exact: true })
      .filter({ visible: true })
      .click();
    const dialog = page.getByRole('dialog', { name: '删除章节', exact: true });
    await dialog.getByRole('button', { name: '生成固定目录预览', exact: true }).click();
    await dialog.getByRole('button', { name: '确认操作结果', exact: true }).waitFor();
    await writeDraft(f, '原预览之后实际SAVED的S2');
    f.close();
    await dialog.getByRole('button', { name: '确认操作结果', exact: true }).click();
    const preview = dialog.getByRole('region', { name: '固定目录预览', exact: true });
    await preview.waitFor();
    assert.equal(inputs.length, 2);
    assert.deepEqual(inputs[1], inputs[0]);
    assert.equal(
      await preview.getByLabel('固定预览摘要', { exact: true }).innerText(),
      original.cut_digest,
    );
    await preview.getByLabel('我已确认目录范围变化', { exact: true }).check();
    await preview.getByRole('button', { name: '确认目录范围调整', exact: true }).click();
    await dialog
      .getByRole('alert')
      .filter({ hasText: '预览后有新的已保存修改或范围变化' })
      .waitFor();
    assert.equal(
      await preview.getByLabel('固定预览摘要', { exact: true }).innerText(),
      original.cut_digest,
    );
    const current = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    assert.equal(current.structure.id, f.d.main.structure.id);
    assert.equal(current.units[0].unit_id, f.d.main.units[0].unit_id);
    await preview.getByRole('button', { name: '关闭此预览', exact: true }).click();
    await dialog.getByRole('button', { name: '生成固定目录预览', exact: true }).click();
    await preview.waitFor();
    assert.equal(inputs.length, 3);
    assert.notEqual(inputs[2].operation_id, inputs[0].operation_id);
    assert.notEqual(
      await preview.getByLabel('固定预览摘要', { exact: true }).innerText(),
      original.cut_digest,
    );
  } finally {
    f.close();
    await browser?.close();
  }
});
