import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { state, root } from '../../scripts/runtime-lib.mjs';
import { resolve } from 'node:path';
import { readFileSync } from 'node:fs';
import mysql from 'mysql2/promise';
import { runtime } from '../../scripts/runtime-lib.mjs';

async function open(page, f) {
  await page.goto('https://localhost:' + state().ports.edge);
  await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
  await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
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

test('real browser composition, pure deletion, immediate FLUSH and a refreshed offline IndexedDB queue reach durable WSS', async () => {
  const f = await organizationFixture(0);
  await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 900 },
      }),
      page = await context.newPage(),
      errors = [];
    page.setDefaultTimeout(8000);
    page.on('pageerror', (e) => errors.push(e.message));
    await open(page, f);
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await editor.focus();
    const cdp = await context.newCDPSession(page);
    await cdp.send('Input.imeSetComposition', {
      text: '中文候选',
      selectionStart: 4,
      selectionEnd: 4,
    });
    await cdp.send('Input.insertText', { text: '中文选词' });
    await page.getByRole('button', { name: '保存当前修改', exact: true }).click();
    await page.getByText('当前修改已保存', { exact: true }).waitFor();
    assert.equal(await editor.innerText(), '中文选词');
    await editor.focus();
    await page.keyboard.press('Control+A');
    await page.keyboard.press('Backspace');
    await page.getByRole('button', { name: '保存当前修改', exact: true }).click();
    await page.getByText('当前修改已保存', { exact: true }).waitFor();
    assert.equal((await editor.innerText()).trim(), '');
    await context.setOffline(true);
    await editor.fill('刷新后恢复的离线输入');
    await page.getByText(/本机已保留.*等待服务端保存/).waitFor();
    await context.setOffline(false);
    await page.reload();
    await page
      .getByRole('group', { name: '空间类型', exact: true })
      .getByRole('button', { name: '组织空间', exact: true })
      .click();
    await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
    await page.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
    await page.getByRole('button', { name: '协作报告', exact: true }).click();
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    await editor.waitFor();
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    assert.equal(await editor.innerText(), '刷新后恢复的离线输入');
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/03/editor-desktop.png'),
      fullPage: true,
    });
    await context.close();
  } finally {
    await browser.close();
  }
});

test('an actual offline identity refresh preserves the owned local editor and resumes through fresh authority', async () => {
  const f = await organizationFixture(0);
  await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(8000);
    await open(page, f);
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await context.setOffline(true);
    await editor.fill('离线第一段');
    await page.getByText(/本机已保留.*等待服务端保存/).waitFor();
    // 真实网络中断跨过应用10秒身份复核周期，不能用永久假me响应掩盖组件被卸载的问题。
    await page.waitForTimeout(11000);
    assert.equal(await editor.count(), 1);
    await editor.fill('离线第二段');
    await context.setOffline(false);
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    assert.equal(await editor.innerText(), '离线第二段');
    await context.close();
  } finally {
    await browser.close();
  }
});

test('the actual IndexedDB write transaction blocks the saved label until local persistence and durable ACK complete', async () => {
  const f = await organizationFixture(0);
  await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(8000);
    await open(page, f);
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    await page.evaluate(
      () =>
        new Promise((done, reject) => {
          const open = indexedDB.open('collaboration-report.drafts', 1);
          open.onerror = () => reject(open.error);
          open.onsuccess = () => {
            const db = open.result,
              tx = db.transaction('queues', 'readwrite'),
              store = tx.objectStore('queues');
            let hold = true;
            window.finishQueueBlock = () => {
              hold = false;
            };
            const pump = () => {
              const r = store.get('unused-test-key');
              r.onsuccess = () => {
                if (hold) pump();
              };
            };
            pump();
            tx.oncomplete = () => db.close();
            done();
          };
        }),
    );
    await page
      .getByRole('textbox', { name: '共同支线编辑器', exact: true })
      .fill('本机写事务尚未结束');
    await page.waitForTimeout(100);
    assert.notEqual(await page.locator('.save-state').innerText(), '服务端已保存');
    await page.evaluate(() => window.finishQueueBlock());
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    await context.close();
  } finally {
    await browser.close();
  }
});

test('a live reconnect to a newer workspace keeps old pending input visible and offers restoration only into the current authorized draft', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    browser = await chromium.launch({ channel: 'msedge', headless: true }),
    pool = mysql.createPool(
      JSON.parse(readFileSync(resolve(runtime, 'services/document.json'))).database,
    );
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(8000);
    await open(page, f);
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await context.setOffline(true);
    await editor.fill('旧代次待发正文');
    await page.getByText(/本机已保留.*等待服务端保存/).waitFor();
    // Task6公开控制动作尚未交付；本例只用所属Doc私有库建立更高workspace epoch的边界夹具。
    await pool.query(
      "UPDATE branch SET workspace_epoch=workspace_epoch+1 WHERE document_id=? AND lifecycle='ACTIVE'",
      [d.id],
    );
    await context.setOffline(false);
    await page.getByText('旧工作区输入仍在本机', { exact: true }).waitFor();
    assert.equal((await editor.innerText()).trim(), '');
    assert.equal(await page.locator('.recovery-box pre').innerText(), '旧代次待发正文');
    await page.getByRole('button', { name: '将旧输入复制到当前草稿', exact: true }).click();
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    assert.equal(await editor.innerText(), '旧代次待发正文');
    await context.close();
  } finally {
    await pool.end();
    await browser.close();
  }
});

test('a real UPDATE database timeout preserves the local identity and resumes through fresh authority without manual retyping', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    browser = await chromium.launch({ channel: 'msedge', headless: true }),
    pool = mysql.createPool(
      JSON.parse(readFileSync(resolve(runtime, 'services/collaboration.json'))).database,
    ),
    c = await pool.getConnection();
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage(),
      errors = [];
    let sent;
    const updateSent = new Promise((done) => {
      sent = done;
    });
    page.setDefaultTimeout(10000);
    page.on('websocket', (socket) => {
      socket.on('framesent', (event) => {
        try {
          if (JSON.parse(event.payload).type === 'UPDATE') sent();
        } catch {}
      });
      socket.on('framereceived', (event) => {
        try {
          const m = JSON.parse(event.payload);
          if (m.type === 'ERROR') errors.push(m.code);
        } catch {}
      });
    });
    await open(page, f);
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    const [[room]] = await pool.query(
      'SELECT CAST(id AS CHAR) AS id FROM room WHERE document_id=?',
      [d.id],
    );
    await c.query('START TRANSACTION');
    await c.query('SELECT id FROM room WHERE id=? FOR UPDATE', [room.id]);
    // 超过生产SQL三秒deadline，随后释放实际行锁，让ERROR能输出；不能用假503替代数据库未知结果。
    await page
      .getByRole('textbox', { name: '共同支线编辑器', exact: true })
      .fill('数据库超时后仍保留的输入');
    await updateSent;
    await new Promise((done) => setTimeout(done, 3500));
    await c.query('COMMIT');
    await page.getByText('服务端已保存', { exact: true }).waitFor({ timeout: 12000 });
    assert.ok(
      errors.includes('SERVICE_UNAVAILABLE'),
      '夹具必须实际产生UPDATE超时，不能仅验证慢保存',
    );
    assert.equal(
      await page.getByRole('textbox', { name: '共同支线编辑器', exact: true }).innerText(),
      '数据库超时后仍保留的输入',
    );
    await context.close();
  } finally {
    try {
      await c.query('ROLLBACK');
    } catch {}
    c.release();
    await pool.end();
    await browser.close();
  }
});
