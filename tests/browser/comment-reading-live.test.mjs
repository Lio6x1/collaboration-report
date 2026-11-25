import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { chromium, expect } from '@playwright/test';
import { createUser, origin } from '../helpers/account.mjs';
import { organizationFixture, documentFixture, get, post, json } from '../helpers/workspace.mjs';
import { openReviewWorkspace, writeDraft, flushDraft } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';

// 本次只验证用户实际阅读链路：创建定位评论、回复、主线追加内容后重新定位，以及从组织打开文档。
// 所有写入限于新建的合成组织/文档，凭据仅在内存使用，不输出到日志或截图。
test('真实评论在正文更新后保留原锚点并可从组织内容打开', { timeout: 90000 }, async () => {
  const credentialsFile = '.runtime/private/local-experience-account.json';
  let owner;
  if (existsSync(credentialsFile)) {
    const credentials = JSON.parse(readFileSync(credentialsFile, 'utf8'));
    const login = await post('/v1/account/login', {
      email: credentials.email,
      password: credentials.password,
    });
    assert.equal(login.status, 200, 'EXPERIENCE_LOGIN_FAILED');
    owner = {
      ...credentials,
      id: json(login).user_id,
      headers: {
        cookie: login.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
        'x-csrf-token': json(login).csrf_token,
      },
    };
  } else owner = await createUser();
  const organization = await organizationFixture(0, owner);
  const document = await documentFixture(organization);
  const branch = await openReviewWorkspace(organization, document, owner);
  await unlockStructurePreview(branch);
  branch.close();
  let main = await openReviewWorkspace(organization, document, owner, 'MAIN');
  const text = '评论引用对应正文，保存新的内容后也应能找到这一处。';
  await writeDraft(main, text);
  await flushDraft(main);
  main.close();
  main = null;
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    viewport: { width: 1440, height: 1000 },
  });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', (error) => errors.push(error.message));
  try {
    await page.goto(`${origin}/documents/${document.id}`);
    await page.getByLabel('登录邮箱', { exact: true }).fill(owner.email);
    await page.getByLabel('登录密码', { exact: true }).fill(owner.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    const body = page.getByRole('document', { name: '主线正文', exact: true });
    await expect(body).toHaveText(text);
    await body.locator('p').evaluate((node) => {
      node.closest('[role="document"]').focus();
      const range = document.createRange();
      range.setStart(node.firstChild, 0);
      range.setEnd(node.firstChild, 4);
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      document.dispatchEvent(new Event('selectionchange'));
    });
    await page.getByRole('button', { name: '评论所选文字', exact: true }).click();
    const sidebar = page.getByRole('complementary', { name: '正文评论侧栏', exact: true });
    await sidebar
      .getByLabel('对这份正文发起讨论', { exact: true })
      .fill('这里显示已有评论，引用位置必须清晰。');
    const creating = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' &&
        new URL(response.url()).pathname === `/v1/documents/${document.id}/comment-threads`,
    );
    await sidebar.getByRole('button', { name: '发起讨论', exact: true }).click();
    const created = await creating;
    assert.equal(created.status(), 201);
    const input = created.request().postDataJSON();
    const thread = (await created.json()).result.entity_id;
    await sidebar.getByRole('button', { name: '收起新评论', exact: true }).click();
    const card = sidebar.getByRole('button', { name: '打开评论 ' + thread, exact: true });
    await card.scrollIntoViewIfNeeded();
    await expect(card).toContainText('这里显示已有评论，引用位置必须清晰。');
    await card.click();
    await expect(body.locator('.comment-anchor-highlight')).toHaveText('评论引用');
    const detail = sidebar.getByRole('region', { name: '当前讨论', exact: true });
    const originalComment = detail.locator('[data-comment-id]').first();
    const originalCommentID = await originalComment.getAttribute('data-comment-id');
    await originalComment.getByRole('button', { name: '回复这条批注', exact: true }).click();
    await detail.getByLabel('发表批注', { exact: true }).fill('回复也已可靠保存。');
    const replying = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' &&
        new URL(response.url()).pathname === '/v1/comment-threads/' + thread + '/comments',
    );
    await detail.getByRole('button', { name: '发表批注', exact: true }).click();
    const replied = await replying;
    assert.equal(replied.status(), 201);
    assert.equal(replied.request().postDataJSON().reply_to_id, originalCommentID);
    await expect(detail.getByRole('list', { name: '讨论内容', exact: true })).toContainText(
      '回复也已可靠保存。',
    );

    main = await openReviewWorkspace(organization, document, owner, 'MAIN');
    await writeDraft(main, text + '这是后来追加的主线正文。');
    await flushDraft(main);
    main.close();
    main = null;
    await page.reload();
    await expect(body).toContainText('这是后来追加的主线正文。');
    await page.getByRole('button', { name: '查看评论', exact: true }).click();
    await expect(card).toContainText('原引用仍在此处');
    await card.click();
    await expect(body.locator('.comment-anchor-highlight')).toHaveText('评论引用');
    await detail.getByRole('button', { name: '查看回复（1）', exact: true }).click();
    await expect(detail.getByRole('list', { name: '讨论内容', exact: true })).toContainText(
      '回复也已可靠保存。',
    );
    const persisted = json(await get('/v1/comment-threads/' + thread, owner.headers));
    assert.deepEqual(persisted.anchor, input.anchor, '显示位置投影不能改写原固定锚点');

    await page.getByRole('button', { name: '组织', exact: true }).click();
    await expect(page.getByRole('button', { name: '刷新组织', exact: true })).toBeEnabled();
    const choice = page.locator(
      '.organization-choice[data-organization-id="' + organization.organization + '"]',
    );
    for (let attempt = 0; attempt < 20 && !(await choice.count()); attempt++) {
      const more = page.getByRole('button', { name: '更多组织', exact: true });
      await more.waitFor();
      await more.click();
      await expect(page.getByRole('button', { name: '刷新组织', exact: true })).toBeEnabled();
    }
    await choice.click();
    const content = page.getByRole('tabpanel', { name: '内容', exact: true });
    // 体验身份已积累多页空间，等待真实游标解析结束，不用五秒的单页预算误判目录缺失。
    await expect(content.getByRole('button', { name: '刷新组织内容', exact: true })).toBeEnabled({
      timeout: 30000,
    });
    await expect(
      content.getByRole('button', { name: '打开知识库：技术实践库', exact: true }),
    ).toHaveCount(1);
    await content.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
    // 切库后子面板还要核验本库管理动作与成员候选；等当前读取完成再点击文档。
    await expect(content.getByRole('button', { name: '刷新组织内容', exact: true })).toBeEnabled();
    await content.locator('a[href="/documents/' + document.id + '"]').click();
    await expect(body).toContainText('这是后来追加的主线正文。');
    assert.deepEqual(errors, []);
    const folder = 'artifacts/reported-usability/comment-reading-live';
    mkdirSync(folder, { recursive: true });
    writeFileSync(
      folder + '/result.json',
      JSON.stringify(
        {
          document: document.id,
          organization: organization.organization,
          thread,
          originalAnchorPreserved: true,
          replyPersisted: true,
          organizationNavigation: true,
          errors,
        },
        null,
        2,
      ),
    );
  } finally {
    main?.close();
    await context.close();
    await browser.close();
  }
});
