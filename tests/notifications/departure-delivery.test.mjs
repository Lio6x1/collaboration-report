import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { waitOperation } from '../helpers/account.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

test(
  '负责人离开保留组织文档，只提醒原所属库管理员并随管理员资格撤销隐藏',
  { timeout: 90000 },
  async () => {
    const f = await organizationFixture(3),
      [departing, kbAdmin, otherAdmin] = f.members;
    const document = await documentFixture(f);
    let browser;
    async function kbRole(kb, user, role) {
      const current = json(await get(`/v1/knowledge-bases/${kb}`, f.owner.headers));
      await waitAuthorization(
        await post(
          `/v1/knowledge-bases/${kb}/roles`,
          {
            operation_id: operation('departure-kb-role'),
            expected_acl_revision: current.acl_revision,
            user_id: user.id,
            role,
          },
          f.owner.headers,
        ),
        f.owner,
      );
    }
    async function notices(user) {
      const response = await get('/v1/notifications?limit=100', user.headers);
      assert.equal(response.status, 200, response.body);
      return json(response).items.filter(
        (row) => row.kind === 'RESPONSIBLE_DEPARTURE' && row.target_id === document.id,
      );
    }
    try {
      await kbRole(f.kb, kbAdmin, 'ADMIN');
      await waitOperation(
        await post(
          `/v1/organizations/${f.organization}/members/${otherAdmin.id}/role`,
          { operation_id: operation('departure-org-admin'), expected_revision: '1', role: 'ADMIN' },
          f.owner.headers,
        ),
        f.owner,
      );
      const otherKB = await post(
        `/v1/spaces/${f.space}/knowledge-bases`,
        { operation_id: operation('departure-other-kb'), name: '其他知识库' },
        f.owner.headers,
      );
      assert.equal(otherKB.status, 201, otherKB.body);
      await kbRole(json(otherKB).result.entity_id, otherAdmin, 'ADMIN');
      const current = json(await get(`/v1/documents/${document.id}`, f.owner.headers));
      await waitAuthorization(
        await post(
          `/v1/documents/${document.id}/responsible-transfers`,
          {
            operation_id: operation('departure-responsible'),
            expected_revision: current.control_revision,
            target_user_id: departing.id,
          },
          f.owner.headers,
        ),
        f.owner,
      );
      const before = json(await get(`/v1/documents/${document.id}`, f.owner.headers));
      const members = json(
        await get(`/v1/organizations/${f.organization}/members?limit=100`, f.owner.headers),
      );
      const input = {
        operation_id: operation('departure-remove'),
        expected_revision: members.items.find((row) => row.user_id === departing.id).revision,
      };
      const path = `/v1/organizations/${f.organization}/members/${departing.id}`;
      await waitOperation(await post(path, input, f.owner.headers, 'DELETE'), f.owner);
      const after = json(await get(`/v1/documents/${document.id}`, f.owner.headers));
      assert.equal(after.lifecycle, 'ACTIVE');
      assert.equal(after.responsible_user_id, undefined);
      assert.equal(after.current_manifest_id, before.current_manifest_id);
      assert.equal((await get(`/v1/documents/${document.id}`, departing.headers)).status, 403);
      let delivered = [];
      for (let n = 0; n < 100; n++) {
        delivered = await notices(kbAdmin);
        if (delivered.length) break;
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
      assert.equal(delivered.length, 1, '真正所属知识库管理员应收到一次提醒');
      assert.equal((await notices(f.owner)).length, 1, '创建者仍为本库ADMIN，应收到提醒');
      assert.equal(
        (await notices(otherAdmin)).length,
        0,
        '组织管理员或其他库管理员不能替代本库管理员',
      );
      await waitOperation(await post(path, input, f.owner.headers, 'DELETE'), f.owner);
      assert.equal((await notices(kbAdmin)).length, 1);
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const context = await browser.newContext({ ignoreHTTPSErrors: true });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      await page.goto('https://localhost:' + state().ports.edge + '/notifications');
      await page.getByLabel('登录邮箱', { exact: true }).fill(kbAdmin.email);
      await page.getByLabel('登录密码', { exact: true }).fill(kbAdmin.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      const row = page
        .getByRole('list', { name: '通知列表', exact: true })
        .locator('li')
        .filter({ hasText: '文档负责人已离开组织，请安排接任人' });
      await row.getByRole('button', { name: '打开相关内容', exact: true }).click();
      await expect(page.getByRole('heading', { name: '协作报告', exact: true })).toBeVisible();
      // 保留直接COMMENT，证明隐藏依据是本库管理员资格消失，而不只是整个文档失去可读性。
      const docNow = json(await get(`/v1/documents/${document.id}`, f.owner.headers));
      await waitAuthorization(
        await post(
          `/v1/documents/${document.id}/authorization-changes`,
          {
            operation_id: operation('departure-keep-comment'),
            expected_acl_revision: docNow.acl_revision,
            changes: [
              {
                action: 'GRANT',
                grant: {
                  user_id: kbAdmin.id,
                  scope_kind: 'DOCUMENT',
                  scope_id: document.id,
                  capability: 'COMMENT',
                },
              },
            ],
          },
          f.owner.headers,
        ),
        f.owner,
      );
      await kbRole(f.kb, kbAdmin, 'REMOVE');
      assert.equal((await get(`/v1/documents/${document.id}`, kbAdmin.headers)).status, 200);
      assert.equal((await notices(kbAdmin)).length, 0);
    } finally {
      await browser?.close();
    }
  },
);
