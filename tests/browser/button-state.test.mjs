import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdirSync } from 'node:fs';
import mysql from 'mysql2/promise';
import { chromium, expect } from '@playwright/test';
import { organizationFixture, documentFixture, get, post, json } from '../helpers/workspace.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

async function owner() {
  const { email, password } = JSON.parse(
    readFileSync(process.env.COLLABORATION_TEST_ACCOUNT, 'utf8'),
  );
  const login = await post('/v1/account/login', { email, password });
  assert.equal(login.status, 200);
  return {
    email,
    password,
    id: json(login).user_id,
    headers: {
      cookie: login.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
      'x-csrf-token': json(login).csrf_token,
    },
  };
}

// 对应实际反馈：互斥操作不能同时显示为可用，也不能用点击后的本地猜测替代服务端状态。
// 只在新建测试组织内建立无邮箱、不可登录的成员，不发送验证码或修改已有用户关系。
test(
  'organization creation buttons follow real grants and inherited roles; editor reflects selection',
  { timeout: 60000 },
  async () => {
    const f = await organizationFixture(0, await owner());
    const cfg = JSON.parse(readFileSync('.runtime/services/account.json', 'utf8'));
    const db = await mysql.createConnection(cfg.database);
    try {
      for (const [name, role] of [
        ['按钮状态普通成员', 'MEMBER'],
        ['按钮状态管理员', 'ADMIN'],
      ]) {
        const [user] = await db.execute(
          "INSERT INTO user(display_name,password_hash,password_algorithm,state) VALUES(?,?,'bcrypt','ACTIVE')",
          [name, Buffer.from('test-identity-without-login')],
        );
        await db.execute(
          "INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,?,'ACTIVE')",
          [f.organization, user.insertId, role],
        );
      }
      await db.execute('UPDATE organization SET member_revision=member_revision+1 WHERE id=?', [
        f.organization,
      ]);
    } finally {
      await db.end();
    }
    const document = await documentFixture(f);
    await unlockStructurePreview({ d: document, user: f.owner });
    const browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({
      ignoreHTTPSErrors: true,
      viewport: { width: 1440, height: 1000 },
    });
    page.setDefaultTimeout(12000);
    const errors = [];
    page.on('pageerror', (e) => errors.push(e.message));
    try {
      // 专用体验账号已有大量历史测试组织，仅将真实目录收窄到本轮组织，详情和写入全部走真实服务。
      await page.route('**/v1/organizations?limit=20', async (route) => {
        const current = await get('/v1/organizations/' + f.organization, f.owner.headers);
        assert.equal(current.status, 200);
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          json: { items: [json(current)], has_more: false, next_cursor: null },
        });
      });
      await page.goto('https://localhost:' + state().ports.edge + '/organizations');
      await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('button', { name: '集成验证组织', exact: true }).click();
      const row = page.locator('.detail-panel tbody tr').filter({ hasText: '按钮状态普通成员' });
      await expect(row).toContainText('未授予');
      await expect(row.getByRole('button', { name: '授予建库资格', exact: true })).toHaveCount(1);
      await expect(row.getByRole('button', { name: '撤销建库资格', exact: true })).toHaveCount(0);
      const inherited = page.locator('.detail-panel tbody tr').filter({ hasText: '角色自带' });
      await expect(inherited).toHaveCount(2);
      await expect(inherited.getByRole('button', { name: /建库资格/ })).toHaveCount(0);
      await row.getByRole('button', { name: '授予建库资格', exact: true }).click();
      await expect(row).toContainText('已授予');
      await expect(row.getByRole('button', { name: '授予建库资格', exact: true })).toHaveCount(0);
      await page.getByRole('button', { name: '刷新组织', exact: true }).click();
      await expect(row).toContainText('已授予');
      await row.getByRole('button', { name: '撤销建库资格', exact: true }).click();
      await expect(row).toContainText('未授予');
      await expect(row.getByRole('button', { name: '撤销建库资格', exact: true })).toHaveCount(0);
      mkdirSync('artifacts/reported-usability', { recursive: true });
      await page
        .locator('.detail-panel')
        .screenshot({ path: 'artifacts/reported-usability/organization-button-states.png' });

      await page.goto('https://localhost:' + state().ports.edge + '/documents/' + document.id);
      await page.getByRole('button', { name: '编辑文档', exact: true }).click();
      await page.getByRole('link', { name: '编辑主线', exact: true }).first().click();
      const editor = page.getByRole('textbox', { name: '主线编辑器', exact: true });
      await expect(editor).toBeEditable();
      await expect(page.getByRole('region', { name: '主线编辑区', exact: true })).toHaveAttribute(
        'aria-busy',
        'false',
      );
      const button = (name) => page.getByRole('button', { name, exact: true });
      await editor.click();
      await page.keyboard.insertText('第一段格式');
      await expect(button('正文段落')).toHaveAttribute('aria-pressed', 'true');
      await expect(button('二级标题')).toHaveAttribute('aria-pressed', 'false');
      await expect(button('左对齐')).toHaveAttribute('aria-pressed', 'true');
      await button('居中对齐').click();
      await expect(button('居中对齐')).toHaveAttribute('aria-pressed', 'true');
      await expect(button('左对齐')).toHaveAttribute('aria-pressed', 'false');
      await expect(button('右对齐')).toHaveAttribute('aria-pressed', 'false');
      await button('首行缩进').click();
      await expect(button('首行缩进')).toHaveAttribute('aria-pressed', 'true');
      await expect(button('取消缩进')).toBeEnabled();
      await page.keyboard.press('End');
      await page.keyboard.press('Enter');
      await page.keyboard.insertText('第二段格式');
      await button('右对齐').click();
      // Enter新建段落按编辑器原规则取消首行缩进；当前已取消时按钮必须不可重复执行。
      await expect(button('取消缩进')).toBeDisabled();
      await expect(button('右对齐')).toHaveAttribute('aria-pressed', 'true');
      await expect(button('取消缩进')).toHaveAttribute('aria-pressed', 'true');
      await page.keyboard.press('ArrowUp');
      await expect(button('居中对齐')).toHaveAttribute('aria-pressed', 'true');
      await expect(button('首行缩进')).toHaveAttribute('aria-pressed', 'true');
      await button('二级标题').click();
      await expect(button('二级标题')).toHaveAttribute('aria-pressed', 'true');
      await expect(button('正文段落')).toHaveAttribute('aria-pressed', 'false');
      await button('粗体').click();
      await expect(button('粗体')).toHaveAttribute('aria-pressed', 'true');
      await expect(button('二级标题')).toHaveCSS('background-color', 'rgb(36, 92, 73)');
      await expect(button('正文段落')).toHaveCSS('background-color', 'rgb(255, 254, 250)');
      await expect(button('居中对齐')).toHaveCSS('background-color', 'rgb(36, 92, 73)');
      await expect(button('右对齐')).toHaveCSS('background-color', 'rgb(255, 254, 250)');
      await page
        .locator('.unit-editor')
        .screenshot({ path: 'artifacts/reported-usability/editor-button-states.png' });
      await button('保存并返回阅读').click();
      await expect(page.getByLabel('主线正文', { exact: true }).first()).toContainText(
        '第一段格式',
      );
      assert.deepEqual(errors, []);
    } finally {
      await browser.close();
    }
  },
);
