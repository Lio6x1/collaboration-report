import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import {
  organizationFixture,
  documentFixture,
  post,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

async function select(page) {
  await page
    .getByRole('group', { name: '空间类型', exact: true })
    .getByRole('button', { name: '组织空间', exact: true })
    .click();
  await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
  await page.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
  await page.getByRole('button', { name: '协作报告', exact: true }).click();
  await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
  await page.getByRole('textbox', { name: '共同支线编辑器', exact: true }).waitFor();
  await page.getByText('服务端已保存', { exact: true }).waitFor();
}
async function login(page, user) {
  await page.getByLabel('登录邮箱', { exact: true }).fill(user.email);
  await page.getByLabel('登录密码', { exact: true }).fill(user.password);
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await select(page);
}
async function queues(page) {
  return page.evaluate(
    () =>
      new Promise((done, reject) => {
        const open = indexedDB.open('collaboration-report.drafts', 1);
        open.onerror = () => reject(open.error);
        open.onsuccess = () => {
          const r = open.result.transaction('queues').objectStore('queues').getAll();
          r.onsuccess = () => {
            done(
              r.result.map(({ actor, tab, client, entries }) => ({
                actor,
                tab,
                client,
                count: entries.length,
              })),
            );
            open.result.close();
          };
        };
      }),
  );
}

test('a copied live browser tab obtains a separate client queue and the real editors converge after offline concurrent input', async () => {
  const f = await organizationFixture(0);
  await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      first = await context.newPage();
    first.setDefaultTimeout(10000);
    await first.goto('https://localhost:' + state().ports.edge);
    await login(first, f.owner);
    // window.open真实复制sessionStorage，而两个普通newPage不会覆盖重复tab身份的反例。
    const popup = context.waitForEvent('page');
    await first.evaluate(() => window.open(location.href));
    const second = await popup;
    second.setDefaultTimeout(10000);
    await select(second);
    const all = await queues(first);
    assert.equal(all.length, 2);
    assert.notEqual(all[0].tab, all[1].tab);
    assert.notEqual(all[0].client, all[1].client);
    await context.setOffline(true);
    const a = first.getByRole('textbox', { name: '共同支线编辑器', exact: true }),
      b = second.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await Promise.all([a.fill('标签甲'), b.fill('标签乙')]);
    await first.getByText(/本机已保留.*等待服务端保存/).waitFor();
    await second.getByText(/本机已保留.*等待服务端保存/).waitFor();
    await context.setOffline(false);
    await Promise.all([
      first.getByText('服务端已保存', { exact: true }).waitFor(),
      second.getByText('服务端已保存', { exact: true }).waitFor(),
    ]);
    await expect(a).toHaveText(await b.innerText());
    assert.match(await a.innerText(), /标签甲/);
    assert.match(await a.innerText(), /标签乙/);
    assert.ok((await queues(first)).every((q) => q.count === 0));
    await context.close();
  } finally {
    await browser.close();
  }
});

test('switching the real logged-in actor never sends or displays the previous actor offline queue and restores it only on returning', async () => {
  const f = await organizationFixture(1),
    d = await documentFixture(f),
    member = f.members[0];
  await waitAuthorization(
    await post(
      `/v1/documents/${d.id}/authorization-changes`,
      {
        operation_id: operation('acl'),
        expected_acl_revision: '1',
        changes: [
          {
            grant: {
              user_id: member.id,
              scope_kind: 'UNIT',
              scope_id: d.main.units[0].unit_id,
              capability: 'EDIT',
            },
            action: 'GRANT',
          },
        ],
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(10000);
    await page.goto('https://localhost:' + state().ports.edge);
    await login(page, f.owner);
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await context.setOffline(true);
    await editor.fill('原账号离线输入');
    await page.getByText(/本机已保留.*等待服务端保存/).waitFor();
    await page.getByRole('button', { name: '收起编辑器', exact: true }).click();
    await context.setOffline(false);
    await page.getByRole('button', { name: '退出登录', exact: true }).click();
    await login(page, member);
    assert.equal((await editor.innerText()).trim(), '');
    assert.ok((await queues(page)).some((q) => q.actor === f.owner.id && q.count > 0));
    await editor.fill('成员已保存');
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    assert.equal(await editor.innerText(), '成员已保存');
    await page.getByRole('button', { name: '退出登录', exact: true }).click();
    await login(page, f.owner);
    await expect(editor).toContainText('原账号离线输入');
    await expect(editor).toContainText('成员已保存');
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    await context.close();
  } finally {
    await browser.close();
  }
});

test('a closed tab pending queue stays discoverable after losing sessionStorage and restores by explicit current-authority copy', async () => {
  const f = await organizationFixture(0);
  await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      old = await context.newPage();
    old.setDefaultTimeout(10000);
    await old.goto('https://localhost:' + state().ports.edge);
    await login(old, f.owner);
    await context.setOffline(true);
    await old
      .getByRole('textbox', { name: '共同支线编辑器', exact: true })
      .fill('关闭标签前的离线正文');
    await old.getByText(/本机已保留.*等待服务端保存/).waitFor();
    await old.close();
    await context.setOffline(false);
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    await page.goto('https://localhost:' + state().ports.edge);
    await login(page, f.owner);
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    assert.equal((await editor.innerText()).trim(), '');
    await page.getByText('本机保留的其他输入', { exact: true }).waitFor();
    await page.getByRole('button', { name: '查看该标签的输入', exact: true }).click();
    assert.equal(await page.locator('.recovery-box pre').innerText(), '关闭标签前的离线正文');
    await page.getByRole('button', { name: '将旧输入复制到当前草稿', exact: true }).click();
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    assert.equal(await editor.innerText(), '关闭标签前的离线正文');
    await context.close();
  } finally {
    await browser.close();
  }
});
