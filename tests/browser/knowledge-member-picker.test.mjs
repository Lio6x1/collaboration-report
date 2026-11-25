import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync } from 'node:fs';
import { chromium, expect } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';

const pageOf = (items, cursor = null) => ({ items, next_cursor: cursor, has_more: !!cursor });
const member = (id, name, role = 'MEMBER', state = 'ACTIVE') => ({
  user_id: id,
  display_name: name,
  role,
  state,
  revision: '1',
});

// 本次验收：不用记账号 ID，库管理员可选择当前组织成员；分页、刷新与撤权不混入旧候选。
// 模拟授权接口而不写真实用户数据；原修订号/操作号和确认后刷新恢复也必须保留。
test('knowledge base member picker uses authorized organization names and preserves mutation recovery', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    viewport: { width: 1440, height: 1000 },
  });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  const organization = {
    id: '99101',
    name: '成员选择验证组织',
    owner_id: '99999',
    lifecycle: 'ACTIVE',
    revision: '1',
    member_revision: '1',
  };
  const library = {
    id: '99301',
    space_id: '99201',
    name: '成员选择验证知识库',
    lifecycle: 'ACTIVE',
    acl_revision: '7',
    visibility_policy: 'COLLABORATORS',
    visibility_revision: '1',
  };
  const mine = member('99001', '当前库管理员'),
    existing = member('99002', '已在库的成员'),
    later = member('99003', '分页找到的小林');
  const writes = [],
    errors = [];
  let failRefresh = false,
    denyCandidates = false,
    delayCandidate = false,
    releaseCandidate;
  try {
    await context.addInitScript(() => {
      const slot = '1234567890abcdef1234567890abcdef';
      sessionStorage.setItem('collaboration-report.session-context', slot);
      sessionStorage.setItem(
        'collaboration-report.csrf',
        JSON.stringify({ user_id: '99001', token: 'mock-member-picker', context: slot }),
      );
    });
    page.on('pageerror', (failure) => errors.push(failure.message));
    await page.route('**/v1/**', async (route) => {
      const request = route.request(),
        url = new URL(request.url()),
        path = url.pathname;
      const json = (value) => route.fulfill({ json: value });
      if (path === '/v1/account/me')
        return json({
          id: mine.user_id,
          email: 'mock@example.test',
          display_name: mine.display_name,
          state: 'ACTIVE',
          credential_epoch: '1',
          identity_epoch: '1',
        });
      if (path === '/v1/organizations') return json(pageOf([organization]));
      if (path === '/v1/organizations/' + organization.id) return json(organization);
      if (path === '/v1/account/applications') return json(pageOf([]));
      if (path === '/v1/me/spaces')
        return json(
          pageOf([
            {
              id: library.space_id,
              kind: 'ORGANIZATION',
              organization_id: organization.id,
              lifecycle: 'ACTIVE',
              actions: [],
            },
          ]),
        );
      if (path === '/v1/spaces/' + library.space_id + '/knowledge-bases')
        return json(pageOf([library]));
      if (path.endsWith('/documents')) return json(pageOf([]));
      if (path === '/v1/knowledge-bases/' + library.id + '/creation-grants') {
        writes.push(request.postDataJSON());
        failRefresh = true;
        return json({
          operation_id: writes.at(-1).operation_id,
          state: 'SUCCEEDED',
          scope_key: 'kb-acl:' + library.id,
          epoch: '8',
          missing_participants: [],
        });
      }
      if (path === '/v1/knowledge-bases/' + library.id) {
        if (failRefresh) {
          failRefresh = false;
          return route.fulfill({ status: 503, json: { code: 'SERVICE_UNAVAILABLE' } });
        }
        return route.fulfill({
          json: library,
          headers: {
            'X-Available-Actions': JSON.stringify([
              'READ',
              'VIEW_MEMBERS',
              'CHANGE_CREATION_GRANTS',
            ]),
          },
        });
      }
      if (path === '/v1/knowledge-bases/' + library.id + '/members')
        return json(pageOf([{ ...mine, role: 'ADMIN' }, existing]));
      if (path === '/v1/organizations/' + organization.id + '/members') {
        if (denyCandidates)
          return route.fulfill({ status: 403, json: { code: 'IDENTITY_NOT_CURRENT' } });
        if (delayCandidate) {
          delayCandidate = false;
          await new Promise((resolve) => {
            releaseCandidate = resolve;
          });
          return json(pageOf([member('99099', '迟到的旧候选')]));
        }
        return json(
          url.searchParams.has('cursor')
            ? pageOf([existing, later])
            : pageOf(
                [
                  mine,
                  existing,
                  member('99005', '同名成员'),
                  member('29900005', '同名成员'),
                  member('99004', '已退出组织', 'MEMBER', 'CLOSED'),
                ],
                'next-members',
              ),
        );
      }
      return route.fulfill({ status: 404, json: { code: 'ENTITY_NOT_FOUND' } });
    });
    await page.goto('https://localhost:' + state().ports.edge + '/organizations');
    await page.getByRole('button', { name: '打开知识库：' + library.name, exact: true }).click();
    await page.getByRole('button', { name: '知识库设置', exact: true }).click();
    await page.getByRole('tab', { name: '权限', exact: true }).click();
    const picker = page.locator('.member-picker'),
      target = picker.getByRole('group', { name: '目标成员', exact: true });
    await expect(target).not.toBeVisible();
    await picker.getByRole('button', { name: '选择目标成员', exact: true }).click();
    await expect(target.getByRole('radio', { name: /已在库的成员/ })).toBeVisible();
    await expect(target.getByRole('radio', { name: /已退出组织/ })).toHaveCount(0);
    // 成员显示页不代表整个库；候选未出现在此页时，只能说明其为组织成员。
    await expect(
      target.getByRole('radio', { name: '同名成员 · 账号 00099005 · 组织成员', exact: true }),
    ).toHaveCount(1);
    await expect(
      target.getByRole('radio', { name: '同名成员 · 账号 29900005 · 组织成员', exact: true }),
    ).toHaveCount(1);
    await expect(page.getByLabel('目标成员账号 ID', { exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '更新库成员角色', exact: true })).toHaveCount(0);
    await expect(
      page.getByRole('button', { name: '更新本库创建资格', exact: true }),
    ).toBeDisabled();
    // 输入后自动查找后页并保留查找词，不能把已加载候选当全量目录。
    await picker.getByLabel('按成员姓名或编号筛选', { exact: true }).fill('小林');
    await expect(target.getByRole('radio', { name: /分页找到的小林.*组织成员/ })).toBeVisible();
    await expect(picker.getByLabel('按成员姓名或编号筛选', { exact: true })).toHaveValue('小林');
    // 查找允许复制后的补零编号，选中值仍是原始ID，不能把展示格式当授权目标。
    await picker.getByLabel('按成员姓名或编号筛选', { exact: true }).fill('00099005');
    await expect(target.getByRole('radio')).toHaveCount(1);
    await expect(target.getByRole('radio')).toHaveAttribute('value', '99005');
    await picker.getByLabel('按成员姓名或编号筛选', { exact: true }).fill('小林');
    await expect(target.getByRole('radio')).toHaveCount(1);
    await target.getByRole('radio', { name: /分页找到的小林/ }).check();
    await expect(target).not.toBeVisible();
    await expect(picker.locator('.member-selected')).toContainText('已选：分页找到的小林');
    await page.getByRole('button', { name: '更新本库创建资格', exact: true }).click();
    await page.getByRole('button', { name: '确认操作结果', exact: true }).click();
    await expect(page.getByRole('button', { name: '刷新组织内容', exact: true })).toBeEnabled();
    // 资格重读会重新挂载管理区，返回权限分区后继续核对原写入结果。
    await page.getByRole('tab', { name: '权限', exact: true }).click();
    await expect(target).not.toBeVisible();
    await picker.getByRole('button', { name: '选择目标成员', exact: true }).click();
    await expect(target.getByRole('radio', { name: /已在库的成员/ })).toHaveCount(1);
    assert.equal(writes.length, 1, '业务已成功而刷新503时不能再次POST或要求重选目标');
    assert.equal(writes[0].user_id, later.user_id);
    assert.equal(writes[0].expected_revision, '7');
    assert.equal(writes[0].allowed, true);
    assert.match(writes[0].operation_id, /^[a-f0-9-]{36}$/);

    // 读候选不持有全局写锁；刷新知识库可以使正在飞行的旧候选失效。
    delayCandidate = true;
    const delayed = page.waitForRequest(
      (request) =>
        new URL(request.url()).pathname === '/v1/organizations/' + organization.id + '/members',
    );
    await picker.getByRole('button', { name: '刷新成员列表', exact: true }).click();
    await delayed;
    await page.evaluate(() => window.dispatchEvent(new Event('collab-refresh')));
    // 全局刷新会重建父目录及管理区，恢复当前知识库设置并重新核验候选。
    await expect(page.getByRole('button', { name: '刷新组织内容', exact: true })).toBeEnabled();
    await page.getByRole('tab', { name: '权限', exact: true }).click();
    await picker.getByRole('button', { name: '选择目标成员', exact: true }).click();
    await expect(target.getByRole('radio', { name: /已在库的成员/ })).toHaveCount(1);
    releaseCandidate();
    await expect(picker).toHaveAttribute('aria-busy', 'false');
    await expect(target.getByRole('radio', { name: /迟到的旧候选/ })).toHaveCount(0);
    await target.getByRole('radio', { name: /已在库的成员/ }).check();
    await picker.getByRole('button', { name: '更换目标成员', exact: true }).click();
    await picker.getByLabel('按成员姓名或编号筛选', { exact: true }).fill('当前');
    await expect(target.locator('input:checked')).toHaveCount(0);
    await expect(
      page.getByRole('button', { name: '更新本库创建资格', exact: true }),
    ).toBeDisabled();
    await picker.getByLabel('按成员姓名或编号筛选', { exact: true }).fill('');

    mkdirSync('artifacts/reported-usability/knowledge-member-picker', { recursive: true });
    await target.getByRole('radio').first().scrollIntoViewIfNeeded();
    await page.screenshot({
      path: 'artifacts/reported-usability/knowledge-member-picker/desktop.png',
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    await target.getByRole('radio').first().scrollIntoViewIfNeeded();
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await page.screenshot({
      path: 'artifacts/reported-usability/knowledge-member-picker/mobile.png',
      fullPage: true,
    });
    await picker.screenshot({
      path: 'artifacts/reported-usability/knowledge-member-picker/mobile-picker.png',
    });
    denyCandidates = true;
    await picker.getByRole('button', { name: '刷新成员列表', exact: true }).click();
    await expect(page.getByRole('alert')).toHaveText('当前身份不具备此动作资格');
    await expect(target).toHaveCount(0);
    await expect(page.getByRole('button', { name: '更新本库创建资格', exact: true })).toHaveCount(
      0,
    );
    assert.deepEqual(errors, []);
  } finally {
    releaseCandidate?.();
    await context.close();
    await browser.close();
  }
});
