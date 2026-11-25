import test from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { readFileSync } from 'node:fs';
import mysql from 'mysql2/promise';
import { chromium, expect } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';
import { post, json } from '../helpers/workspace.mjs';

// 验收的是同一个BrowserContext内的普通标签，不能用两个隐身环境伪造隔离。
// 第二身份只在本机测试库中建立，借用专用体验账号的哈希，不发送真实注册邮件。
test(
  'two normal tabs keep separate accounts across login, refresh, writes and logout',
  { timeout: 60000 },
  async () => {
    const owner = JSON.parse(readFileSync(process.env.COLLABORATION_TEST_ACCOUNT, 'utf8'));
    const logged = await post('/v1/account/login', {
      email: owner.email,
      password: owner.password,
    });
    assert.equal(logged.status, 200);
    const firstID = json(logged).user_id;
    const config = JSON.parse(readFileSync('.runtime/services/account.json', 'utf8'));
    const db = await mysql.createConnection(config.database);
    const secondEmail = `tab-${randomUUID()}@example.test`;
    let secondID;
    try {
      const [insert] = await db.execute(
        `INSERT INTO user(email,email_key,display_name,password_hash,password_algorithm,state,email_verified_at)
       SELECT ?,?,'标签页隔离验证',password_hash,password_algorithm,'ACTIVE',UTC_TIMESTAMP(6) FROM user WHERE id=?`,
        [secondEmail, Buffer.from(secondEmail), firstID],
      );
      assert.equal(insert.affectedRows, 1);
      secondID = String(insert.insertId);
    } finally {
      await db.end();
    }
    const browser = await chromium.launch({ channel: 'msedge', headless: true });
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    const errors = [];
    const a = await context.newPage(),
      b = await context.newPage();
    for (const page of [a, b]) {
      page.setDefaultTimeout(10000);
      page.on('pageerror', (error) => errors.push(error.message));
    }
    const origin = 'https://localhost:' + state().ports.edge;
    const signIn = async (page, email) => {
      await page.goto(origin);
      await page.getByLabel('登录邮箱', { exact: true }).fill(email);
      await page.getByLabel('登录密码', { exact: true }).fill(owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await expect(page.getByRole('button', { name: '退出登录', exact: true })).toBeVisible();
    };
    const request = (page, path, method = 'GET', body, override = {}) =>
      page.evaluate(
        async ({ path, method, body, override }) => {
          const slot = sessionStorage.getItem('collaboration-report.session-context');
          const proof = JSON.parse(sessionStorage.getItem('collaboration-report.csrf') ?? 'null');
          const response = await fetch(path, {
            method,
            credentials: 'same-origin',
            headers: {
              'Content-Type': 'application/json',
              'X-Collab-Session-Context': slot,
              'X-CSRF-Token': proof?.token ?? '',
              ...override,
            },
            ...(body ? { body: JSON.stringify(body) } : {}),
          });
          return { status: response.status, value: await response.json() };
        },
        { path, method, body, override },
      );
    try {
      await signIn(a, owner.email);
      await signIn(b, secondEmail);
      assert.equal((await request(a, '/v1/account/me')).value.id, firstID);
      assert.equal((await request(b, '/v1/account/me')).value.id, secondID);
      const slots = await Promise.all(
        [a, b].map((page) =>
          page.evaluate(() => sessionStorage.getItem('collaboration-report.session-context')),
        ),
      );
      assert.notEqual(slots[0], slots[1]);
      const cookies = await context.cookies();
      for (const slot of slots) {
        const session = cookies.find((cookie) => cookie.name === '__Host-collabSession.' + slot);
        assert.ok(session?.httpOnly && session.secure && session.sameSite === 'Strict');
      }
      assert.equal(
        (await a.evaluate(() => document.cookie)).includes('__Host-collabSession.'),
        false,
      );
      await a.reload();
      await b.reload();
      await expect(a.getByRole('button', { name: '退出登录', exact: true })).toBeVisible();
      await expect(b.getByRole('button', { name: '退出登录', exact: true })).toBeVisible();
      for (const [page, expectedOwner] of [
        [a, firstID],
        [b, secondID],
      ]) {
        const made = await request(page, '/v1/organizations', 'POST', {
          operation_id: randomUUID(),
          name: '标签独立写入验证',
        });
        assert.equal(made.status, 201);
        const read = await request(page, '/v1/organizations/' + made.value.result.entity_id);
        assert.equal(read.value.owner_id, expectedOwner);
      }
      // 无对应Cookie绝不能落回另一个账号；选择另一个slot但使用本标签CSRF也必须拒绝写入。
      assert.equal(
        (
          await request(a, '/v1/account/me', 'GET', undefined, {
            'X-Collab-Session-Context': 'f'.repeat(32),
          })
        ).status,
        401,
      );
      assert.equal(
        (
          await request(
            a,
            '/v1/organizations',
            'POST',
            { operation_id: randomUUID(), name: '不得建立' },
            { 'X-Collab-Session-Context': slots[1] },
          )
        ).status,
        403,
      );
      await b.getByRole('button', { name: '退出登录', exact: true }).click();
      await expect(b.getByLabel('登录邮箱', { exact: true })).toBeVisible();
      assert.equal((await request(a, '/v1/account/me')).value.id, firstID);
      await expect(a.getByRole('button', { name: '退出登录', exact: true })).toBeVisible();
      await a.reload();
      await expect(a.getByRole('button', { name: '退出登录', exact: true })).toBeVisible();
      assert.equal((await request(a, '/v1/account/me')).value.id, firstID);
      assert.deepEqual(errors, []);
    } finally {
      await browser.close();
    }
  },
);
