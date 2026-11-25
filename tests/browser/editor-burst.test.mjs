import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
test('an actual offline burst larger than the server queue limit drains with bounded in-flight UPDATEs', async () => {
  const f = await organizationFixture(0);
  await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(8000);
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
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await editor.waitFor();
    await context.setOffline(true);
    await editor.pressSequentially('字'.repeat(80), { delay: 1 });
    const count = await page.evaluate(
      () =>
        new Promise((done, reject) => {
          const r = indexedDB.open('collaboration-report.drafts', 1);
          r.onerror = () => reject(r.error);
          r.onsuccess = () => {
            const all = r.result.transaction('queues').objectStore('queues').getAll();
            all.onsuccess = () => {
              done(all.result.reduce((n, q) => n + q.entries.length, 0));
              r.result.close();
            };
          };
        }),
    );
    assert.ok(count > 32);
    // 80次键入都保留原幂等身份；检查真实线路的未确认数，不能用删队列/合并新身份消除背压。
    let maximum = 0,
      opened = 0,
      ready;
    const firstReady = new Promise((done) => {
        ready = done;
      }),
      frames = [];
    page.on('websocket', (socket) => {
      opened++;
      let outstanding = 0;
      socket.on('framesent', (event) => {
        try {
          if (['UPDATE', 'FLUSH'].includes(JSON.parse(event.payload).type)) {
            outstanding++;
            maximum = Math.max(maximum, outstanding);
          }
        } catch {}
      });
      socket.on('framereceived', (event) => {
        try {
          const m = JSON.parse(event.payload);
          if (['SAVED', 'WAITING', 'FLUSHED'].includes(m.type)) outstanding--;
          if (m.type === 'READY') ready();
          frames.push({ type: m.type, code: m.code, seq: m.durable_seq });
        } catch {}
      });
    });
    await context.setOffline(false);
    await firstReady;
    await page.getByRole('button', { name: '保存当前修改', exact: true }).click();
    try {
      await page.getByText('当前修改已保存', { exact: true }).waitFor({ timeout: 30000 });
    } catch (e) {
      console.log(
        'BURST_DIAGNOSTIC',
        JSON.stringify({
          notice: await page.locator('.save-state').innerText(),
          frames: frames.slice(-12),
          maximum,
          opened,
        }),
      );
      throw e;
    }
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    assert.equal(opened, 1, '保存边界不能用反复过载重连才能推进');
    assert.ok(maximum <= 32, `真实未确认帧${maximum}超过服务端32帧队列`);
    assert.equal(await editor.innerText(), '字'.repeat(80));
    await context.close();
  } finally {
    await browser.close();
  }
});
