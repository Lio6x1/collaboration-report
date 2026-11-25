import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync } from 'node:fs';
import { chromium, expect } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';

// 本次仅验证阅读入口、草稿复用和锚点展示，全部业务API拦截，不修改用户体验数据。
test('reading comments preserve one panel, fixed anchors and a full-width mobile body', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    viewport: { width: 1440, height: 1000 },
  });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  const errors = [],
    requests = [];
  let canComment = true,
    denied = false,
    historyDenied = false;
  const metadata = {
    id: '91001',
    title: '阅读评论界面验证',
    space_id: '92001',
    knowledge_base_id: '93001',
    lifecycle: 'ACTIVE',
    main_locked: true,
    current_structure_id: '94001',
    current_style_id: '95001',
    current_manifest_id: '96001',
    publication_state: 'UNPUBLISHED',
    control_revision: '1',
    acl_revision: '1',
    publication_epoch: '1',
    visibility_policy: 'COLLABORATORS',
    visibility_revision: '1',
  };
  const main = {
    read_view_id: '97001',
    expires_at: '2099-01-01T00:00:00Z',
    document: metadata,
    manifest_id: '96001',
    structure_id: '94001',
    style_id: '95001',
    units: [{ unit_id: '98001', content_version_id: '99001' }],
    structure: {
      id: '94001',
      document_id: metadata.id,
      schema_version: 'structure-v1',
      entries: [
        {
          node_id: '98002',
          chapter_id: '98003',
          unit_id: '98001',
          kind: 'CHAPTER',
          title: '第一章',
          sort_order: 1,
          split_level2: false,
        },
      ],
    },
    style: {
      id: '95001',
      document_id: metadata.id,
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
  const content = {
    id: '99001',
    document_id: metadata.id,
    unit_id: '98001',
    structure_id: '94001',
    schema_version: 'content-v1',
    canonicalization_version: 'canonical-v1',
    body_json: {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'reading-comments-block' },
          content: [{ type: 'text', text: '当前正文的定位评论' }],
        },
        {
          type: 'paragraph',
          attrs: { id: 'reading-comments-second' },
          content: [{ type: 'text', text: '阅读正文时保持完整宽度。' }],
        },
      ],
    },
    body_hash: 'a'.repeat(64),
    byte_size: '300',
    resource_ids: [],
    provenance: {},
  };
  const current = {
    id: '901',
    document_id: metadata.id,
    context_kind: 'MAIN',
    context_id: main.manifest_id,
    anchor: {
      schema_version: 'anchor-v1',
      content_version_id: content.id,
      block_id: 'reading-comments-block',
      start_offset: 0,
      end_offset: 4,
      quote: '当前正文',
    },
    author_id: '90001',
    status: 'OPEN',
    revision: '1',
  };
  const historical = {
    ...current,
    id: '902',
    context_id: '96000',
    anchor: { ...current.anchor, content_version_id: '99000', quote: '历史正文' },
  };
  const orphan = {
    ...current,
    id: '903',
    anchor: { ...current.anchor, block_id: 'retired-block' },
  };
  try {
    await context.addInitScript(() => {
      const slot = '1234567890abcdef1234567890abcdef';
      sessionStorage.setItem('collaboration-report.session-context', slot);
      sessionStorage.setItem(
        'collaboration-report.csrf',
        JSON.stringify({
          user_id: '90001',
          token: 'mock-reading-comments',
          context: slot,
        }),
      );
    });
    page.on('pageerror', (error) => errors.push(error.message));
    await page.route('**/v1/**', async (route) => {
      const request = route.request(),
        url = new URL(request.url()),
        path = url.pathname;
      requests.push({
        path,
        method: request.method(),
        cursor: url.searchParams.get('cursor'),
        limit: url.searchParams.get('limit'),
      });
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
      if (path.endsWith('/main-revision'))
        return route.fulfill(
          denied
            ? { status: 403, json: { code: 'DOCUMENT_READ_NOT_ALLOWED' } }
            : { json: { revision: 'b'.repeat(64) } },
        );
      if (path === '/v1/documents/' + metadata.id)
        return route.fulfill({
          headers: {
            'content-type': 'application/json',
            'X-Available-Actions': JSON.stringify(
              canComment ? ['COMMENT', 'CHANGE_AUTHORIZATION'] : [],
            ),
          },
          body: JSON.stringify(metadata),
        });
      if (path.endsWith('/main')) return route.fulfill({ json: main });
      if (path.includes('/read-views/')) return route.fulfill({ json: content });
      if (path.endsWith('/editable-units'))
        return route.fulfill({ json: { items: [], next_cursor: null } });
      if (path.endsWith('/comment-threads'))
        return route.fulfill({
          json: {
            items: url.searchParams.has('cursor')
              ? [orphan]
              : [
                  current,
                  historical,
                  {
                    ...current,
                    id: '905',
                    anchor: { ...current.anchor, content_version_id: '99000' },
                  },
                ],
            next_cursor: url.searchParams.has('cursor') ? null : 'next-comments-page',
          },
        });
      if (path.endsWith('/comments'))
        return route.fulfill({
          json: {
            items: [
              {
                id: '904',
                thread_id: path.split('/')[3],
                author_id: '90001',
                body: '已获授权的评论内容',
                revision: '1',
              },
              ...(path.includes('/901/') && url.searchParams.get('limit') !== '1'
                ? ['第一条回复', '第二条回复', '折叠的第三条回复'].map((body, index) => ({
                    id: String(910 + index),
                    thread_id: '901',
                    author_id: '90002',
                    body,
                    revision: '1',
                  }))
                : []),
            ],
            next_cursor: null,
          },
        });
      if (path === '/v1/comment-threads/902') return route.fulfill({ json: historical });
      if (path === '/v1/content-versions/99000' && !historyDenied)
        return route.fulfill({
          json: {
            ...content,
            id: '99000',
            body_json: {
              type: 'doc',
              content: Array.from({ length: 40 }, (_, index) => ({
                type: 'paragraph',
                attrs: { id: index === 0 ? 'reading-comments-block' : 'history-' + index },
                content: [
                  {
                    type: 'text',
                    text:
                      index === 0 ? '历史正文' : '这是较长的历史原文，预览不能撑开整个评论列表。',
                  },
                ],
              })),
            },
          },
        });
      // 历史原文必须经用户点击和服务端重新授权；撤权后不能继续借列表锚点读取。
      return route.fulfill({ status: 403, json: { code: 'DOCUMENT_READ_NOT_ALLOWED' } });
    });
    await page.goto('https://localhost:' + state().ports.edge + '/documents/' + metadata.id);
    const reading = page.getByLabel('主线正文', { exact: true });
    await expect(reading).toContainText('阅读正文时保持完整宽度。');
    await expect(reading).toHaveAttribute('contenteditable', 'false');
    const marker = reading.getByRole('button', { name: '查看讨论 901（进行中）', exact: true });
    await expect(marker).toBeVisible();
    assert.equal(requests.filter((item) => item.path.includes('/content-versions/')).length, 0);
    assert.equal(requests.filter((item) => item.path.endsWith('/comments')).length, 0);
    await page.getByRole('button', { name: '查看评论', exact: true }).click();
    const sidebar = page.getByRole('complementary', { name: '正文评论侧栏', exact: true });
    await expect(sidebar).toBeVisible();
    // 展开即看到已有评论，不能先穿过长说明和完整新建表单；历史引用只在稳定块/原偏移精确匹配时定位。
    const firstCard = sidebar.getByRole('button', { name: '打开评论 901', exact: true });
    await expect(firstCard).toContainText('已获授权的评论内容');
    await expect(firstCard).toContainText('当前正文');
    await expect(page.getByLabel('对这份正文发起讨论', { exact: true })).toBeHidden();
    await expect(reading.locator('.comment-location-range')).toHaveCount(1);
    await expect(
      reading.getByRole('button', { name: '查看讨论 905（进行中）', exact: true }),
    ).toBeVisible();
    mkdirSync('artifacts/reported-usability/reading-comments', { recursive: true });
    await page.screenshot({
      path: 'artifacts/reported-usability/reading-comments/overview.png',
      fullPage: true,
    });
    await marker.click();
    await expect(marker).toHaveAttribute('aria-pressed', 'true');
    await expect(reading.locator('.comment-anchor-highlight')).toHaveText('当前正文');
    await expect(
      sidebar
        .getByRole('list', { name: '讨论内容', exact: true })
        .getByText('已获授权的评论内容', { exact: true }),
    ).toBeVisible();
    const discussion = sidebar.getByRole('region', { name: '当前讨论', exact: true });
    await expect(discussion.locator('.reply-item')).toHaveCount(0);
    await expect(discussion.getByText('折叠的第三条回复', { exact: true })).toHaveCount(0);
    await discussion.getByRole('button', { name: '查看回复（3）', exact: true }).click();
    await expect(discussion.locator('.reply-item')).toHaveCount(3);
    await discussion.getByRole('button', { name: '收起回复', exact: true }).click();
    await expect(discussion.locator('.reply-item')).toHaveCount(0);
    // 修改/删除低频操作收进菜单，回复仍直接可达；展开输入不改变原回复目标。
    const rootComment = discussion.locator('[data-comment-id="904"]');
    await expect(rootComment.getByRole('button', { name: '修改', exact: true })).toBeHidden();
    await rootComment.getByRole('button', { name: '评论更多操作', exact: true }).click();
    await rootComment.getByRole('button', { name: '修改', exact: true }).click();
    await expect(rootComment.getByLabel('修改批注', { exact: true })).toHaveValue(
      '已获授权的评论内容',
    );
    await rootComment.getByRole('button', { name: '取消修改', exact: true }).click();
    await rootComment.getByRole('button', { name: '回复这条批注', exact: true }).click();
    const replyDraft = page.getByLabel('发表批注', { exact: true });
    await expect(replyDraft).toBeFocused();
    await replyDraft.fill('切换入口时保留的回复');
    await expect
      .poll(async () => {
        const bounds = await sidebar.boundingBox();
        const detail = await sidebar
          .getByRole('heading', { name: '讨论 901 · 进行中', exact: true })
          .boundingBox();
        return !!bounds && !!detail && detail.y >= bounds.y && detail.y < bounds.y + bounds.height;
      })
      .toBe(true);
    await expect(page.getByRole('tab', { name: '正文', exact: true })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await sidebar.getByRole('button', { name: '新增评论', exact: true }).click();
    const draft = page.getByLabel('对这份正文发起讨论', { exact: true });
    await draft.fill('在侧栏保留的未提交评论');
    await page.getByRole('button', { name: '收起评论侧栏', exact: true }).click();
    await page.getByRole('button', { name: '查看评论', exact: true }).click();
    await expect(draft).toHaveValue('在侧栏保留的未提交评论');
    await page.getByRole('tab', { name: '评论', exact: true }).click();
    await expect(draft).toHaveValue('在侧栏保留的未提交评论');
    await expect(replyDraft).toHaveValue('切换入口时保留的回复');
    await expect(
      page.getByRole('heading', { name: '讨论 901 · 进行中', exact: true }),
    ).toBeVisible();
    await page.getByRole('tab', { name: '正文', exact: true }).click();
    await expect(draft).toHaveValue('在侧栏保留的未提交评论');
    assert.equal(
      requests.filter((item) => item.path.endsWith('/901/comments') && item.limit !== '1').length,
      1,
      '侧栏和模块移动同一实例，不能重新读取或重建当前讨论',
    );
    const pager = sidebar.getByRole('navigation', { name: '评论分页', exact: true });
    await pager.getByRole('button', { name: '下一页', exact: true }).click();
    await expect(sidebar.locator('.thread-list > li')).toHaveCount(1);
    await expect(draft).toHaveValue('在侧栏保留的未提交评论');
    await expect(replyDraft).toHaveValue('切换入口时保留的回复');
    await pager.getByRole('button', { name: '上一页', exact: true }).click();
    await expect(sidebar.locator('.thread-list > li')).toHaveCount(3);
    await expect(replyDraft).toHaveValue('切换入口时保留的回复');
    assert.ok(
      requests
        .filter((item) => item.path.endsWith('/comment-threads'))
        .every((item) => item.limit === '10'),
    );
    await draft.fill('');
    await replyDraft.fill('');
    await discussion.getByRole('button', { name: '取消', exact: true }).click();
    await sidebar.getByRole('button', { name: '收起新评论', exact: true }).click();
    mkdirSync('artifacts/reported-usability/reading-comments', { recursive: true });
    await page.screenshot({
      path: 'artifacts/reported-usability/reading-comments/desktop.png',
      fullPage: true,
    });
    await pager.getByRole('button', { name: '下一页', exact: true }).click();
    await expect(sidebar.getByRole('button', { name: '打开评论 903', exact: true })).toBeVisible();
    await expect(reading.locator('.comment-anchor-marker')).toHaveCount(0);
    await pager.getByRole('button', { name: '上一页', exact: true }).click();
    await expect(reading.locator('.comment-anchor-marker')).toHaveCount(2);
    await sidebar.getByRole('button', { name: '打开评论 902', exact: true }).click();
    await expect(sidebar.getByText('这条评论引用历史正文', { exact: false })).toBeVisible();
    await expect(reading.locator('.comment-anchor-highlight')).toHaveCount(0);
    assert.equal(requests.filter((item) => item.path.includes('/content-versions/')).length, 0);
    await sidebar.getByRole('button', { name: '查看原文位置', exact: true }).click();
    const preview = sidebar.getByRole('region', { name: '历史原文预览', exact: true });
    await expect(preview).toBeVisible();
    await expect(preview.locator('.comment-anchor-highlight')).toHaveText('历史正文');
    const dimensions = await preview.evaluate((element) => ({
      height: element.clientHeight,
      full: element.scrollHeight,
    }));
    assert.ok(
      dimensions.height <= 320 && dimensions.full > dimensions.height,
      '长原文只在限高区域内滚动',
    );
    await page.screenshot({
      path: 'artifacts/reported-usability/reading-comments/historical-preview.png',
      fullPage: true,
    });
    await sidebar.getByRole('button', { name: '收起原文位置', exact: true }).click();
    await expect(preview).toHaveCount(0);
    historyDenied = true;
    await sidebar.getByRole('button', { name: '查看原文位置', exact: true }).click();
    await expect(sidebar.getByRole('alert')).toHaveText('当前没有文档读取资格');
    await expect(page.getByLabel('讨论原始锚点正文', { exact: true })).toHaveCount(0);

    await page.getByRole('tab', { name: '文档设置', exact: true }).click();
    // 目录组件按需加载；等待它实际挂载后比较顺序，避免把异步加载误报为目录缺失。
    await expect(page.getByRole('heading', { name: '目录管理', exact: true })).toBeVisible();
    const headings = await page.locator('#pane-settings h3').allTextContents();
    assert.equal(headings[0], '文档信息');
    assert.ok(headings.indexOf('文档信息') < headings.indexOf('目录管理'));
    await page.getByRole('tab', { name: '正文', exact: true }).click();
    await sidebar.getByRole('button', { name: '打开评论 901', exact: true }).click();
    await page.setViewportSize({ width: 390, height: 844 });
    const expandedWidth = (await page.locator('.reading-layout > .paper').boundingBox()).width;
    await page.getByRole('button', { name: '收起评论侧栏', exact: true }).click();
    const collapsedWidth = (await page.locator('.reading-layout > .paper').boundingBox()).width;
    assert.equal(expandedWidth, collapsedWidth, '手机展开评论不能压缩正文宽度');
    await page.getByRole('button', { name: '查看评论', exact: true }).click();
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await marker.click();
    await page.screenshot({
      path: 'artifacts/reported-usability/reading-comments/mobile.png',
      fullPage: true,
    });

    // 无COMMENT时仍可读有权讨论，但不展示写入口；确认撤权后清掉正文和侧栏旧内容。
    canComment = false;
    await page.reload();
    await page.getByRole('button', { name: '查看评论', exact: true }).click();
    await expect(page.getByText('当前仅可阅读讨论。', { exact: true })).toBeVisible();
    await expect(draft).toHaveCount(0);
    denied = true;
    await page.evaluate(() => document.dispatchEvent(new Event('visibilitychange')));
    await expect(reading).toHaveCount(0);
    await expect(sidebar).toHaveCount(0);
    assert.equal(
      requests.some((item) => item.method !== 'GET'),
      false,
    );
    assert.deepEqual(errors, []);
  } finally {
    await context.close();
    await browser.close();
  }
});
