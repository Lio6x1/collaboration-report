import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
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
import { openReviewWorkspace, writeDraftBody, reviewDatabase } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { uploadPNG } from '../helpers/resource.mjs';
import { publishRelease, releaseInput } from '../helpers/publication.mjs';
import { chromium, expect } from '@playwright/test';
import { origin } from '../helpers/account.mjs';

function operator(user, action, expected = '', original = '') {
  const args = [
    '--config',
    resolve(runtime, 'services/account.json'),
    '--user-id',
    user,
    '--action',
    action,
  ];
  if (action !== 'status') args.push('--expected-revision', expected, '--operation-id', original);
  const result = spawnSync(resolve(runtime, 'bin/platform-governance.exe'), args, {
    cwd: root,
    windowsHide: true,
    encoding: 'utf8',
    timeout: 10000,
  });
  assert.equal(result.status, 0, result.stderr);
  return JSON.parse(result.stdout);
}

async function finish(operationID, headers, kind = 'operations') {
  const deadline = Date.now() + 20000;
  let value;
  while (Date.now() < deadline) {
    const response = await get(`/v1/${kind}/${operationID}`, headers);
    assert.equal(response.status, 200, response.body);
    value = json(response);
    if (value.state === 'SUCCEEDED' || value.state === 'REJECTED' || value.state === 'CANCELLED')
      break;
    await pause(100);
  }
  assert.equal(value.state, 'SUCCEEDED', JSON.stringify(value.error ?? value.last_error));
  return value;
}

// 从真实注销留存开始验证；平台资格通过本机运维入口授予/撤回，绝不以SQL改成
// PUBLIC_RETAINED、写ACK或作业成功。原账号清理墓碑不能阻止新的治理原号清理。
test(
  'real platform qualification governs only isolated retained publications and revokes original access',
  { timeout: 120000 },
  async () => {
    const owner = await organizationFixture(0),
      administrator = await organizationFixture(1),
      d = await documentFixture(owner),
      active = await documentFixture(administrator),
      db = await reviewDatabase(),
      account = await reviewDatabase('account'),
      node = await reviewDatabase('collaboration');
    let main, browser;
    try {
      await unlockStructurePreview({ d, user: owner.owner });
      main = await openReviewWorkspace(owner, d, owner.owner, 'MAIN');
      const image = await uploadPNG(
        main,
        readFileSync(resolve(root, 'tests/fixtures/export-image.png')),
      );
      await writeDraftBody(main, {
        type: 'doc',
        content: [
          {
            type: 'paragraph',
            attrs: { id: 'retained-original' },
            content: [
              { type: 'text', text: '真实注销后保留的正文，管理员不能代写。' },
              {
                type: 'image',
                attrs: {
                  resourceId: image,
                  alt: '留存原图片',
                  title: null,
                  width: 320,
                  height: 80,
                },
              },
            ],
          },
        ],
      });
      const release = (
        await publishRelease(d.id, owner.owner, await releaseInput(d.id, owner.owner))
      ).result.entity_id;
      main.close();
      main = undefined;
      for (const path of [`/v1/knowledge-bases/${owner.kb}`, `/v1/documents/${d.id}`]) {
        const facts = json(await get(path, owner.owner.headers));
        await waitAuthorization(
          await post(
            path + '/visibility',
            {
              operation_id: operation('retained-public'),
              expected_visibility_revision: facts.visibility_revision,
              policy: 'PUBLIC',
            },
            owner.owner.headers,
          ),
          owner.owner,
        );
      }
      const me = json(await get('/v1/account/me', owner.owner.headers));
      const close = {
        operation_id: operation('retained-close'),
        expected_identity_epoch: me.identity_epoch,
        current_password: owner.owner.password,
      };
      const closing = await post('/v1/account/closures', close, owner.owner.headers);
      assert.equal(closing.status, 202, closing.body);
      const recovery = closing.headers['set-cookie']
        .find((c) => c.startsWith('__Host-collabRecovery='))
        .split(';')[0];
      const closeDeadline = Date.now() + 20000;
      let closed;
      while (Date.now() < closeDeadline) {
        const response = await get(`/v1/account/closures/${close.operation_id}`, {
          cookie: recovery,
        });
        assert.equal(response.status, 200, response.body);
        closed = json(response);
        if (closed.state === 'SUCCEEDED' || closed.state === 'REJECTED') break;
        await pause(100);
      }
      assert.equal(closed.state, 'SUCCEEDED');
      const [[retained]] = await db.query(
        'SELECT lifecycle,control_revision FROM document WHERE id=?',
        [d.id],
      );
      assert.equal(retained.lifecycle, 'PUBLIC_RETAINED');
      assert.equal(json(await get(`/v1/public/documents/${d.id}`)).release_id, release);
      const input = {
        operation_id: operation('retained-unpublish'),
        expected_revision: retained.control_revision,
      };
      const unpublishPath = `/v1/retained-public/documents/${d.id}/unpublish`;
      assert.equal(
        (await post(unpublishPath, input, administrator.members[0].headers)).status,
        403,
      );
      assert.equal(
        (await get(`/v1/retained-public/documents/${d.id}`, administrator.members[0].headers))
          .status,
        403,
      );
      assert.equal(
        (await post(unpublishPath, input, administrator.owner.headers)).status,
        403,
        '组织OWNER也不能冒充平台治理资格',
      );
      const grantOperation = operation('platform-grant');
      assert.equal(
        operator(administrator.owner.id, 'grant', '0', grantOperation).state,
        'SUCCEEDED',
      );
      const metadata = await get(
        `/v1/retained-public/documents/${d.id}`,
        administrator.owner.headers,
      );
      assert.equal(metadata.status, 200, metadata.body);
      assert.equal(json(metadata).control_revision, retained.control_revision);
      assert.equal(
        operator(administrator.owner.id, 'grant', '0', grantOperation).state,
        'SUCCEEDED',
      );
      const [[activeFacts]] = await db.query('SELECT control_revision FROM document WHERE id=?', [
        active.id,
      ]);
      assert.equal(
        (
          await post(
            `/v1/retained-public/documents/${active.id}/unpublish`,
            {
              operation_id: operation('platform-no-author'),
              expected_revision: activeFacts.control_revision,
            },
            administrator.owner.headers,
          )
        ).status,
        403,
        '平台管理员只能治理孤立留存，不能替活跃文档负责人操作',
      );
      // 留存本身仍可合法导出；治理删除必须真实排空这个制品及预算。
      const exportResponse = await post(`/v1/documents/${d.id}/exports`, {
        operation_id: operation('retained-export'),
        target: 'PUBLIC',
        format: 'MARKDOWN',
        expected_release_id: release,
      });
      assert.equal(exportResponse.status, 202, exportResponse.body);
      const exportJob = json(exportResponse);
      await finish(exportJob.id, undefined, 'export-jobs');
      const [[artifact]] = await db.query(
        'SELECT artifact_key FROM export_control WHERE job_id=?',
        [exportJob.id],
      );
      assert.equal(existsSync(resolve(runtime, 'exports/document', artifact.artifact_key)), true);
      const unpublished = await post(unpublishPath, input, administrator.owner.headers);
      assert.equal(unpublished.status, 202, unpublished.body);
      await finish(input.operation_id, administrator.owner.headers);
      assert.equal((await get(`/v1/public/documents/${d.id}`)).status, 404);
      assert.equal(
        json(await post(unpublishPath, input, administrator.owner.headers)).state,
        'SUCCEEDED',
      );
      assert.equal(
        (
          await post(
            unpublishPath,
            { ...input, expected_revision: '999999' },
            administrator.owner.headers,
          )
        ).status,
        409,
      );
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 900 },
      });
      await page.goto(origin);
      await page.getByLabel('登录邮箱', { exact: true }).fill(administrator.owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(administrator.owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
      await page.getByRole('button', { name: '留存治理', exact: true }).click();
      assert.equal(new URL(page.url()).pathname, '/governance');
      await page.reload();
      const panel = page.getByRole('region', { name: '公开留存治理', exact: true });
      await panel.getByLabel('留存文档编号', { exact: true }).fill(d.id);
      await panel.getByRole('button', { name: '查询留存文章', exact: true }).click();
      await expect(panel.getByText('已撤下正式发布', { exact: false })).toBeVisible();
      await panel.getByRole('button', { name: '永久删除留存', exact: true }).click();
      const confirm = panel.getByRole('button', { name: '确认永久删除留存', exact: true });
      await expect(confirm).toBeDisabled();
      await panel
        .getByRole('checkbox', { name: '我确认永久删除这篇留存文章及其内容历史', exact: true })
        .check();
      const originals = [];
      let loseReply = true;
      await page.route(`**/v1/retained-public/documents/${d.id}`, async (route) => {
        if (route.request().method() !== 'DELETE') return route.continue();
        originals.push(route.request().postDataJSON());
        if (loseReply) {
          loseReply = false;
          const response = await route.fetch();
          assert.equal(response.status(), 202);
          await route.abort('failed');
        } else await route.continue();
      });
      await confirm.click();
      await page.getByRole('button', { name: '确认操作结果', exact: true }).click();
      await expect(panel.getByText('留存文章及其内容历史已永久删除', { exact: true })).toBeVisible({
        timeout: 20000,
      });
      assert.equal(originals.length, 2);
      assert.deepEqual(originals[0], originals[1]);
      const purge = originals[0];
      await finish(purge.operation_id, administrator.owner.headers);
      await panel.screenshot({ path: resolve(root, 'artifacts/retained-governance-browser.png') });
      const [[gone]] = await db.query('SELECT lifecycle FROM document WHERE id=?', [d.id]);
      assert.equal(gone.lifecycle, 'PURGED');
      const [[object]] = await db.query('SELECT state,storage_key FROM resource WHERE id=?', [
        image,
      ]);
      assert.equal(object.state, 'DELETED');
      assert.equal(existsSync(resolve(runtime, 'resources/document', object.storage_key)), false);
      assert.equal(existsSync(resolve(runtime, 'exports/document', artifact.artifact_key)), false);
      const [[controls]] = await db.query('SELECT COUNT(*) n FROM export_control WHERE job_id=?', [
        exportJob.id,
      ]);
      assert.equal(String(controls.n), '0');
      const [[cleaned]] = await node.query(
        "SELECT state FROM durable_task WHERE kind='PURGE_DOCUMENT_CLEANUP' AND business_key=?",
        [d.id + ':' + purge.operation_id],
      );
      assert.equal(cleaned.state, 'SUCCEEDED');
      const [[oldCleanup]] = await node.query(
        "SELECT state FROM durable_task WHERE kind='ACCOUNT_DOCUMENT_CLEANUP' AND business_key=?",
        [d.id],
      );
      assert.equal(oldCleanup.state, 'SUCCEEDED');
      const revokeOperation = operation('platform-revoke');
      const revision = operator(administrator.owner.id, 'status').revision;
      assert.equal(
        operator(administrator.owner.id, 'revoke', revision, revokeOperation).state,
        'PREPARING',
      );
      const revokeDeadline = Date.now() + 15000;
      let revoked;
      while (Date.now() < revokeDeadline) {
        revoked = operator(administrator.owner.id, 'status');
        if (revoked.state === 'CLOSED') break;
        await pause(100);
      }
      assert.equal(revoked.state, 'CLOSED');
      const [[receipt]] = await account.query(
        'SELECT state FROM operation_receipt WHERE operation_id=?',
        [revokeOperation],
      );
      assert.equal(receipt.state, 'SUCCEEDED');
      assert.equal(
        (await get(`/v1/operations/${purge.operation_id}`, administrator.owner.headers)).status,
        403,
      );
      assert.equal((await post(unpublishPath, input, administrator.owner.headers)).status, 403);
      console.log(
        'RETAINED_GOVERNANCE_ACTUAL_PASS: real account retention; operator qualification; ordinary/org owner and active-document refusal; original unpublish/purge; Node original cleanup; physical image/export drain; real qualification revocation',
      );
    } finally {
      main?.close();
      await browser?.close();
      await Promise.all([db.end(), account.end(), node.end()]);
    }
  },
);
