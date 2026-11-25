import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
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
  '组织转让与文档通知统一分页，原号不重复、本人已读且退出后隐藏',
  { timeout: 90000 },
  async () => {
    const f = await organizationFixture(1),
      member = f.members[0];
    const document = await documentFixture(f);
    let browser;
    async function transfer(actor, target) {
      const org = await get(`/v1/organizations/${f.organization}`, actor.headers);
      assert.equal(org.status, 200, org.body);
      const input = {
        operation_id: operation('notice-org-owner'),
        expected_revision: json(org).revision,
        target_user_id: target.id,
      };
      const path = `/v1/organizations/${f.organization}/owner-transfers`;
      await waitOperation(await post(path, input, actor.headers), actor);
      return { path, input };
    }
    async function list() {
      const result = await get('/v1/notifications?limit=100', member.headers);
      assert.equal(result.status, 200, result.body);
      return json(result).items;
    }
    try {
      const current = await get(`/v1/documents/${document.id}`, f.owner.headers);
      await waitAuthorization(
        await post(
          `/v1/documents/${document.id}/responsible-transfers`,
          {
            operation_id: operation('notice-doc-owner'),
            expected_revision: json(current).control_revision,
            target_user_id: member.id,
          },
          f.owner.headers,
        ),
        f.owner,
      );
      const changed = await transfer(f.owner, member);
      let orgNotice;
      for (let n = 0; n < 100; n++) {
        orgNotice = (await list()).find(
          (row) => row.kind === 'ORG_OWNER_TRANSFER' && row.target_id === f.organization,
        );
        if (orgNotice) break;
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
      assert.ok(orgNotice, '真实Account事件应可靠进入Doc统一收件箱');
      assert.equal(
        orgNotice.event_id,
        'account-owner-' + createHash('sha256').update(changed.input.operation_id).digest('hex'),
      );
      // 原成功操作恢复不制造第二条Account记录或第二个统一通知ID。
      await waitOperation(await post(changed.path, changed.input, f.owner.headers), f.owner);
      assert.equal((await list()).filter((row) => row.kind === 'ORG_OWNER_TRANSFER').length, 1);
      const first = json(await get('/v1/notifications?limit=1', member.headers));
      assert.equal(first.items.length, 1);
      assert.ok(first.has_more && first.next_cursor);
      const second = json(
        await get(
          '/v1/notifications?limit=1&cursor=' + encodeURIComponent(first.next_cursor),
          member.headers,
        ),
      );
      assert.deepEqual([first.items[0].kind, second.items[0].kind].sort(), [
        'ORG_OWNER_TRANSFER',
        'RESPONSIBLE_TRANSFER',
      ]);
      assert.equal(
        (
          await post(
            `/v1/notifications/${orgNotice.id}/read`,
            { operation_id: operation('wrong-notice-reader') },
            f.owner.headers,
          )
        ).status,
        404,
      );
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const context = await browser.newContext({ ignoreHTTPSErrors: true });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      await page.goto('https://localhost:' + state().ports.edge + '/notifications');
      await page.getByLabel('登录邮箱', { exact: true }).fill(member.email);
      await page.getByLabel('登录密码', { exact: true }).fill(member.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      const row = page
        .getByRole('list', { name: '通知列表', exact: true })
        .locator('li')
        .filter({ hasText: '组织所有者已转交给你' });
      await row.getByRole('button', { name: '标记已读', exact: true }).click();
      await expect(row.getByText('已读', { exact: true })).toBeVisible();
      await row.getByRole('button', { name: '打开相关内容', exact: true }).click();
      await expect(page.getByRole('button', { name: '转让组织所有者', exact: true })).toBeVisible();
      await page.getByRole('button', { name: '返回通知', exact: true }).click();
      await expect(row.getByText('已读', { exact: true })).toBeVisible();
      // 退出前先把负责人和所有者交回，避免把职责阻止退出的规则误当通知失败。
      const docNow = json(await get(`/v1/documents/${document.id}`, member.headers));
      await waitAuthorization(
        await post(
          `/v1/documents/${document.id}/responsible-transfers`,
          {
            operation_id: operation('notice-doc-return'),
            expected_revision: docNow.control_revision,
            target_user_id: f.owner.id,
          },
          member.headers,
        ),
        member,
      );
      await transfer(member, f.owner);
      const members = json(
        await get(`/v1/organizations/${f.organization}/members?limit=100`, f.owner.headers),
      );
      const ownMember = members.items.find((row) => row.user_id === member.id);
      await waitOperation(
        await post(
          `/v1/organizations/${f.organization}/members/${member.id}`,
          {
            operation_id: operation('notice-member-remove'),
            expected_revision: ownMember.revision,
          },
          f.owner.headers,
          'DELETE',
        ),
        f.owner,
      );
      assert.equal(
        (await list()).some((row) => row.id === orgNotice.id),
        false,
      );
      assert.equal((await get(`/v1/organizations/${f.organization}`, member.headers)).status, 404);
      assert.equal(
        (
          await post(
            `/v1/notifications/${orgNotice.id}/read`,
            { operation_id: operation('read-hidden-org-notice') },
            member.headers,
          )
        ).status,
        200,
      );
    } finally {
      await browser?.close();
    }
  },
);
