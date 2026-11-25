import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root, runtime, pause } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { reviewDatabase, openReviewWorkspace, writeDraftBody } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { uploadPNG } from '../helpers/resource.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import { chromium, expect } from '@playwright/test';
import { origin } from '../helpers/account.mjs';

async function trash(f, d) {
  const current = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
  await waitAuthorization(
    await post(
      `/v1/documents/${d.id}`,
      { operation_id: operation('purge-trash'), expected_revision: current.control_revision },
      f.owner.headers,
      'DELETE',
    ),
    f.owner,
  );
  const page = json(await get(`/v1/spaces/${f.space}/trash?limit=100`, f.owner.headers));
  const document = page.items.find((item) => item.id === d.id);
  assert.equal(document?.lifecycle, 'TRASHED');
  return document;
}

async function terminal(input, user) {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    const response = await get(`/v1/operations/${input.operation_id}`, user.headers);
    assert.equal(response.status, 200, response.body);
    const value = json(response);
    if (value.state !== 'PREPARING') {
      assert.equal(value.state, 'SUCCEEDED', JSON.stringify(value.error));
      return value;
    }
    await pause(100);
  }
  assert.fail('PURGE_ORIGINAL_NOT_TERMINAL');
}

// 真实永久删除→Node缺席期间原任务待恢复→实际Doc退出→原号自动恢复→30天到期。
// 仅推进自身到期夹具的数据库时钟，任何ACK、清理回执与成功状态均由真实worker产生。
test(
  'purge recovers its original task and automatic expiry physically removes only the fixed documents',
  { timeout: 180000 },
  async () => {
    const f = await organizationFixture(1),
      d = await documentFixture(f),
      expired = await documentFixture(f),
      sibling = await documentFixture(f);
    const db = await reviewDatabase(),
      node = await reviewDatabase('collaboration');
    let main,
      browser,
      failure,
      stopped = false;
    try {
      await unlockStructurePreview({ d, user: f.owner });
      main = await openReviewWorkspace(f, d, f.owner, 'MAIN');
      const resource = await uploadPNG(
        main,
        readFileSync(resolve(root, 'tests/fixtures/export-image.png')),
      );
      await writeDraftBody(main, {
        type: 'doc',
        content: [
          {
            type: 'paragraph',
            attrs: { id: 'purge-image' },
            content: [
              { type: 'text', text: '应由原号永久清理的正文' },
              {
                type: 'image',
                attrs: {
                  resourceId: resource,
                  alt: '真实固定图片',
                  title: null,
                  width: 320,
                  height: 80,
                },
              },
            ],
          },
        ],
      });
      main.close();
      main = undefined;
      const trashed = await trash(f, d);
      const input = {
        operation_id: operation('purge-original'),
        expected_control_revision: trashed.control_revision,
        confirmed: true,
      };
      assert.equal(
        (await post(`/v1/documents/${d.id}/purge`, { ...input, confirmed: false }, f.owner.headers))
          .status,
        400,
      );
      assert.equal(
        (
          await post(
            `/v1/documents/${d.id}/purge`,
            { ...input, operation_id: operation('purge-denied') },
            f.members[0].headers,
          )
        ).status,
        403,
      );
      stopOwnedRole('collaboration');
      stopped = true;
      const started = await post(`/v1/documents/${d.id}/purge`, input, f.owner.headers);
      assert.equal(started.status, 202, started.body);
      assert.equal(json(started).state, 'PREPARING');
      const [[original]] = await db.query(
        "SELECT t.id,t.generation,t.state,d.lifecycle FROM durable_task t JOIN document d ON d.id=? WHERE t.kind='PURGE_RECOVER' AND t.operation_id=?",
        [d.id, input.operation_id],
      );
      assert.ok(original);
      assert.equal(original.lifecycle, 'CLOSING');
      assert.notEqual(original.state, 'SUCCEEDED');
      const [[before]] = await db.query(
        'SELECT COUNT(*) n FROM content_version WHERE document_id=?',
        [d.id],
      );
      assert.ok(BigInt(before.n) > 0n, '未知Node回执期间不能提前删除正文');
      stopOwnedRole('document');
      restartOwnedRuntime();
      stopped = false;
      await terminal(input, f.owner);
      const replay = await post(`/v1/documents/${d.id}/purge`, input, f.owner.headers);
      assert.equal(replay.status, 202, replay.body);
      assert.equal(json(replay).state, 'SUCCEEDED');
      assert.equal(
        (
          await post(
            `/v1/documents/${d.id}/purge`,
            {
              ...input,
              expected_control_revision: String(BigInt(input.expected_control_revision) + 1n),
            },
            f.owner.headers,
          )
        ).status,
        409,
      );
      const [[finished]] = await db.query(
        "SELECT t.id,t.generation,t.state,d.lifecycle FROM durable_task t JOIN document d ON d.id=? WHERE t.kind='PURGE_RECOVER' AND t.operation_id=?",
        [d.id, input.operation_id],
      );
      assert.equal(finished.id, original.id);
      assert.equal(finished.state, 'SUCCEEDED');
      assert.equal(finished.lifecycle, 'PURGED');
      for (const table of ['content_version', 'branch', 'main_manifest', 'release']) {
        const [[count]] = await db.query(
          `SELECT COUNT(*) n FROM \`${table}\` WHERE document_id=?`,
          [d.id],
        );
        assert.equal(String(count.n), '0', table + '仍有被删除正文引用');
      }
      const [pins] = await db.query(
        'SELECT resource_id,owner_kind,purpose FROM resource_reference WHERE document_id=?',
        [d.id],
      );
      // 原UPLOAD恢复墓碑只保留上传身份与审计，不是正文引用。其它DRAFT/VERSION/
      // RELEASE等必须全部清除，随后用真实GC终态和文件消失验证物理回收。
      assert.deepEqual(
        pins.map((pin) => ({ ...pin })),
        [{ resource_id: resource, owner_kind: 'OPERATION', purpose: 'UPLOAD' }],
      );
      const [[cleaned]] = await node.query(
        "SELECT state FROM durable_task WHERE kind='PURGE_DOCUMENT_CLEANUP' AND business_key=?",
        [d.id + ':' + input.operation_id],
      );
      assert.equal(cleaned?.state, 'SUCCEEDED', '要求真实Node CLEANED原任务');
      const [reservation] = await db.query(
        "UPDATE quota_reservation q JOIN resource r ON r.upload_operation_id=q.operation_id SET q.expires_at=UTC_TIMESTAMP(6)-INTERVAL 2 HOUR WHERE r.id=? AND r.uploader_id=? AND r.space_id=? AND q.state='COMMITTED'",
        [resource, f.owner.id, f.space],
      );
      assert.equal(reservation.affectedRows, 1);
      const deadline = Date.now() + 15000;
      let object;
      while (Date.now() < deadline) {
        [[object]] = await db.query('SELECT state,storage_key FROM resource WHERE id=?', [
          resource,
        ]);
        if (object.state === 'DELETED') break;
        await pause(100);
      }
      assert.equal(object.state, 'DELETED');
      assert.equal(existsSync(resolve(runtime, 'resources/document', object.storage_key)), false);
      await trash(f, expired);
      const [aged] = await db.query(
        "UPDATE document SET purge_after=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE id=? AND space_id=? AND lifecycle='TRASHED'",
        [expired.id, f.space],
      );
      assert.equal(aged.affectedRows, 1);
      let automatic;
      const expiryDeadline = Date.now() + 20000;
      while (Date.now() < expiryDeadline) {
        const [[row]] = await db.query('SELECT lifecycle FROM document WHERE id=?', [expired.id]);
        if (row.lifecycle === 'PURGED') {
          [[automatic]] = await db.query(
            "SELECT operation_id,actor_id,state,fixed_plan FROM lifecycle_operation WHERE kind='PURGE' AND JSON_UNQUOTE(JSON_EXTRACT(fixed_plan,'$.document_id'))=?",
            [expired.id],
          );
          break;
        }
        await pause(100);
      }
      assert.equal(automatic?.state, 'SUCCEEDED');
      assert.equal(automatic.actor_id, null);
      const plan =
        typeof automatic.fixed_plan === 'string'
          ? JSON.parse(automatic.fixed_plan)
          : automatic.fixed_plan;
      assert.equal(plan.automatic, true);
      assert.match(automatic.operation_id, /^purge_expiry_/);
      assert.equal(
        (await get(`/v1/documents/${sibling.id}`, f.owner.headers)).status,
        200,
        '同库其它文档不得被清理',
      );
      const browserDoc = await documentFixture(f);
      await trash(f, browserDoc);
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 900 },
      });
      await page.goto(origin);
      await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
      await page
        .getByRole('group', { name: '空间类型', exact: true })
        .getByRole('button', { name: '组织空间', exact: true })
        .click();
      await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
      await page.getByText('打开文档回收站', { exact: true }).click();
      const row = page
        .getByRole('list', { name: '回收文档列表', exact: true })
        .locator('li')
        .filter({ hasText: 'ID ' + browserDoc.id });
      await row.getByRole('button', { name: '永久删除', exact: true }).click();
      const confirm = row.getByRole('button', { name: '确认永久删除', exact: true });
      await expect(confirm).toBeDisabled();
      await row
        .getByRole('checkbox', { name: '我确认永久删除这篇文档及其内容历史', exact: true })
        .check();
      await expect(confirm).toBeEnabled();
      const originalOperations = [];
      let loseReply = true;
      await page.route(`**/v1/documents/${browserDoc.id}/purge`, async (route) => {
        originalOperations.push(route.request().postDataJSON().operation_id);
        if (loseReply) {
          loseReply = false;
          const response = await route.fetch();
          assert.equal(response.status(), 202);
          await route.abort('failed');
        } else await route.continue();
      });
      await confirm.click();
      await page.getByRole('button', { name: '确认操作结果', exact: true }).click();
      await expect(row).toHaveCount(0, { timeout: 20000 });
      await expect(page.getByText('文档及其内容历史已永久删除', { exact: true })).toBeVisible();
      const [[browserFinished]] = await db.query('SELECT lifecycle FROM document WHERE id=?', [
        browserDoc.id,
      ]);
      assert.equal(browserFinished.lifecycle, 'PURGED');
      assert.equal(
        json(await get(`/v1/operations/${originalOperations[0]}`, f.owner.headers)).state,
        'SUCCEEDED',
      );
      assert.equal(originalOperations.length, 2);
      assert.equal(new Set(originalOperations).size, 1);
      await page
        .getByRole('region', { name: '文档回收站', exact: true })
        .screenshot({ path: resolve(root, 'artifacts/purge-browser.png') });
      console.log(
        'PURGE_ACTUAL_PASS: original task/process recovery; real Node cleanup; private history and physical image removed; automatic expiry and sibling preserved',
      );
    } catch (error) {
      failure = error;
      throw error;
    } finally {
      main?.close();
      await browser?.close();
      try {
        if (stopped) restartOwnedRuntime();
      } catch (error) {
        // 环境恢复失败不能覆盖最先触发的断言；连接也必须归还，避免测试一直不退出。
        if (!failure) throw error;
        console.error('PURGE_ENVIRONMENT_RESTORE_FAILED');
      } finally {
        await Promise.all([db.end(), node.end()]);
      }
    }
  },
);
