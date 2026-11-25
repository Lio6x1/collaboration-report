import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync } from 'node:fs';
import { chromium, expect } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';
import { importMarkdown } from '../../packages/content/markdown.mjs';
import { splitChapterBody } from '../../services/collaboration/src/domain/structure_content.mjs';

// 复现仓库真实顺序：两个父章先返回，第一章的章前内容和子节随后返回。
// 使用真实拆分函数生成正文；业务接口拦截，不对用户原稿再次拆分或合章。
test('split chapter keeps its table, chapter heading and named editing ranges', async () => {
  let next = 0;
  const body = importMarkdown('|第一章表格|内容|\n|---|---|\n|甲|乙|\n\n## 1.1 子节\n\n子节正文', {
    newId: () => 'split-layout-' + ++next,
  }).body;
  const parts = splitChapterBody(body, { workspaceKind: 'MAIN' });
  assert.equal(parts[0].kind, 'INTRODUCTION');
  assert.deepEqual(parts[0].body.content[0], body.content[0], '表格节点原样保留在第一章章前内容');
  const document = {
    id: '91001',
    title: '拆分章节归属验证',
    space_id: '92001',
    knowledge_base_id: '93001',
    lifecycle: 'ACTIVE',
    main_locked: false,
    current_structure_id: '94001',
    current_style_id: '95001',
    current_manifest_id: '96001',
    publication_state: 'PUBLISHED',
    current_release_id: '99999',
    control_revision: '1',
    acl_revision: '1',
    publication_epoch: '1',
    visibility_policy: 'COLLABORATORS',
    visibility_revision: '1',
  };
  const entries = [
    {
      node_id: '98002',
      chapter_id: '98003',
      kind: 'CHAPTER',
      title: '第一章',
      sort_order: 1,
      split_level2: true,
    },
    {
      node_id: '98012',
      chapter_id: '98013',
      unit_id: '98011',
      kind: 'CHAPTER',
      title: '第二章',
      sort_order: 2,
      split_level2: false,
    },
    ...parts.map((part, index) => ({
      node_id: String(98100 + index),
      parent_node_id: '98002',
      chapter_id: '98003',
      unit_id: String(98200 + index),
      kind: part.kind,
      title: part.title,
      sort_order: index + 1,
      split_level2: false,
    })),
  ];
  const bodies = new Map(parts.map((part, index) => [String(98200 + index), part.body]));
  bodies.set('98011', importMarkdown('第二章自己的内容', { newId: () => 'second-chapter' }).body);
  const main = {
    read_view_id: '97001',
    expires_at: '2099-01-01T00:00:00Z',
    document,
    manifest_id: '96001',
    structure_id: '94001',
    style_id: '95001',
    structure: { id: '94001', document_id: document.id, schema_version: 'structure-v1', entries },
    units: [...bodies.keys()].map((unit_id, index) => ({
      unit_id,
      content_version_id: String(99000 + index),
    })),
    style: {
      id: '95001',
      document_id: document.id,
      schema_version: 'style-v1',
      style: {
        body_font: 'SimSun',
        western_font: 'Times New Roman',
        body_size_pt: 12,
        heading_sizes_pt: [22, 18, 16],
        page: 'A4',
        margin_cm: '2.54',
      },
    },
  };
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    viewport: { width: 1440, height: 1000 },
  });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', (error) => errors.push(error.message));
  try {
    await context.addInitScript(() => {
      const slot = '1234567890abcdef1234567890abcdef';
      sessionStorage.setItem('collaboration-report.session-context', slot);
      sessionStorage.setItem(
        'collaboration-report.csrf',
        JSON.stringify({ user_id: '90001', token: 'mock-split-layout', context: slot }),
      );
    });
    await page.route('**/v1/**', async (route) => {
      const path = new URL(route.request().url()).pathname;
      assert.equal(route.request().method(), 'GET', '界面验证不得修改原稿');
      if (path === '/v1/account/me')
        return route.fulfill({
          json: {
            id: '90001',
            display_name: '界面验证',
            email: 'mock@example.test',
            state: 'ACTIVE',
            identity_epoch: '1',
            credential_epoch: '1',
          },
        });
      if (path === '/v1/documents/' + document.id)
        return route.fulfill({
          headers: {
            'content-type': 'application/json',
            'X-Available-Actions': JSON.stringify(['CHANGE_AUTHORIZATION']),
          },
          body: JSON.stringify(document),
        });
      if (path.endsWith('/main')) return route.fulfill({ json: main });
      if (path.endsWith('/main-revision'))
        return route.fulfill({ json: { revision: 'b'.repeat(64) } });
      if (path === '/v1/documents/' + document.id + '/releases')
        return route.fulfill({
          json: {
            items: [
              {
                id: '99999',
                document_id: document.id,
                manifest_id: main.manifest_id,
                publisher_id: '90001',
                created_at: '2026-10-10T14:49:03Z',
              },
              {
                id: '99998',
                document_id: document.id,
                manifest_id: main.manifest_id,
                publisher_id: '90001',
                created_at: '2026-10-09T08:00:00Z',
              },
            ],
            next_cursor: 'older-releases',
            has_more: true,
          },
        });
      if (
        path === '/v1/public/documents/' + document.id ||
        path === '/v1/documents/' + document.id + '/releases/99998'
      )
        return route.fulfill({
          json: {
            document_id: document.id,
            release_id: path.endsWith('/99998') ? '99998' : '99999',
            publication_epoch: '1',
            title: document.title,
            manifest_id: main.manifest_id,
            structure: main.structure,
            style: main.style,
            units: main.units.map((unit) => ({
              id: unit.content_version_id,
              document_id: document.id,
              unit_id: unit.unit_id,
              structure_id: main.structure_id,
              schema_version: 'content-v1',
              canonicalization_version: 'canonical-v1',
              body_json: bodies.get(unit.unit_id),
              body_hash: 'a'.repeat(64),
              byte_size: '300',
              resource_ids: [],
              provenance: {},
            })),
          },
        });
      if (path.includes('/read-views/')) {
        const unit = main.units.find((unit) => path.endsWith('/units/' + unit.unit_id));
        assert.ok(unit);
        return route.fulfill({
          json: {
            id: unit.content_version_id,
            document_id: document.id,
            unit_id: unit.unit_id,
            structure_id: main.structure_id,
            schema_version: 'content-v1',
            canonicalization_version: 'canonical-v1',
            body_json: bodies.get(unit.unit_id),
            body_hash: 'a'.repeat(64),
            byte_size: '300',
            resource_ids: [],
            provenance: {},
          },
        });
      }
      if (path.endsWith('/editable-units'))
        return route.fulfill({
          json: {
            items: main.units.map((unit) => ({
              unit_id: unit.unit_id,
              branch_id: '99901',
              state: 'READY',
            })),
            next_cursor: null,
          },
        });
      if (path.endsWith('/comment-threads'))
        return route.fulfill({ json: { items: [], next_cursor: null } });
      return route.fulfill({ status: 403, json: { code: 'DOCUMENT_READ_NOT_ALLOWED' } });
    });
    await page.goto('https://localhost:' + state().ports.edge + '/documents/' + document.id);
    const first = page.getByRole('region', { name: '章节：第一章', exact: true });
    const second = page.getByRole('region', { name: '章节：第二章', exact: true });
    await expect(first.getByRole('heading', { name: '第一章', exact: true })).toBeVisible();
    await expect(first.getByRole('table')).toContainText('第一章表格');
    await expect(first.getByRole('heading', { name: '1.1 子节', exact: true })).toBeVisible();
    await expect(second).toContainText('第二章自己的内容');
    await expect(second.getByRole('table')).toHaveCount(0);
    await expect(page.locator('.document-heading')).not.toContainText('编号 ' + document.id);
    assert.deepEqual(await page.locator('.paper > section > h3').allTextContents(), [
      '第一章',
      '第二章',
    ]);
    const outline = page.getByRole('navigation', { name: '固定目录', exact: true });
    await expect(
      outline.locator('ol > li').first().getByRole('link', { name: '章前内容', exact: true }),
    ).toHaveAttribute('href', '#unit-98200');
    await page.getByRole('button', { name: '编辑文档', exact: true }).click();
    const choices = page.getByRole('region', { name: '写作章节：第一章', exact: true });
    await expect(choices.getByRole('heading', { name: '第一章', exact: true })).toBeVisible();
    await expect(choices.locator('.editor-choice-list li > span')).toHaveText([
      '章前内容',
      '1.1 子节',
    ]);
    await expect(choices.getByRole('link', { name: '编辑主线', exact: true })).toHaveCount(2);
    await expect(choices.getByRole('link', { name: '编辑共同支线', exact: true })).toHaveCount(2);
    await expect(
      choices.getByRole('link', { name: '编辑主线', exact: true }).first(),
    ).toHaveAttribute('href', '/documents/91001/edit/main/98200');
    const folder = 'artifacts/reported-usability/split-chapter-layout';
    mkdirSync(folder, { recursive: true });
    await page.screenshot({ path: folder + '/desktop.png', fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await page.screenshot({ path: folder + '/mobile.png', fullPage: true });
    // 日常版本用真实发布时间及状态区分；折叠的标识仍可供核对，切换目标保持原ID。
    await page.getByRole('tab', { name: '版本与发布', exact: true }).click();
    const history = page.getByRole('region', { name: '正式版本历史', exact: true });
    const currentRow = history.locator('[data-release-id="99999"]');
    const oldRow = history.locator('[data-release-id="99998"]');
    await expect(currentRow.locator('time')).toHaveText('2026年10月10日 22:49:03 发布');
    await expect(currentRow.getByText('当前正式版', { exact: true })).toBeVisible();
    await expect(oldRow.getByText('历史正式版', { exact: true })).toBeVisible();
    await expect(currentRow.locator('details')).toHaveJSProperty('open', false);
    await expect(currentRow.getByText('99999', { exact: true })).not.toBeVisible();
    await currentRow.getByText('版本详情', { exact: true }).click();
    await expect(currentRow.getByText('99999', { exact: true })).toBeVisible();
    await currentRow.getByText('版本详情', { exact: true }).click();
    await oldRow.getByRole('button', { name: '查看固定版本', exact: true }).click();
    await expect(page.getByRole('heading', { name: '历史版本预览', exact: true })).toBeVisible();
    await oldRow.getByRole('button', { name: '设为当前正式版', exact: true }).click();
    const confirmation = page.getByRole('dialog', { name: '确认切换正式版', exact: true });
    await expect(confirmation).toContainText('2026年10月9日 16:00:00 发布');
    await expect(confirmation.getByText('99998', { exact: true })).not.toBeVisible();
    await confirmation.getByRole('button', { name: '取消', exact: true }).click();
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await page.screenshot({ path: folder + '/version-history-mobile.png', fullPage: true });
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.screenshot({ path: folder + '/version-history-desktop.png', fullPage: true });
    // 正式版也消费父章在前、子节在后的目录，发布后不能重新出现同一错位。
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(
      'https://localhost:' +
        state().ports.edge +
        '/public/documents/' +
        document.id +
        '#chapter-98003',
    );
    const formal = page.getByRole('article', { name: '正式正文整篇', exact: true });
    await expect(
      formal.getByRole('region', { name: '章节：第一章', exact: true }).getByRole('table'),
    ).toContainText('第一章表格');
    await expect(
      formal.getByRole('region', { name: '章节：第二章', exact: true }).getByRole('table'),
    ).toHaveCount(0);
    assert.deepEqual(await formal.locator(':scope > section > h3').allTextContents(), [
      '第一章',
      '第二章',
    ]);
    await expect(formal.locator('#chapter-98003')).toBeVisible();
    await page.screenshot({ path: folder + '/publication.png', fullPage: true });
    assert.deepEqual(errors, []);
  } finally {
    await context.close();
    await browser.close();
  }
});
