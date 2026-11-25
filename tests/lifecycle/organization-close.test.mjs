import test from 'node:test';
import assert from 'node:assert/strict';
import { pause, root } from '../../scripts/runtime-lib.mjs';
import { resolve } from 'node:path';
import { chromium, expect } from '@playwright/test';
import { origin, waitOperation } from '../helpers/account.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { reviewDatabase, openReviewWorkspace, writeDraft } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';

// 独立关闭不能借用账号注销：关闭组织的固定 PUBLIC 和私有数据后，
// 同一个真实账号、会话和个人空间必须仍能继续创建并保存文档。
test(
  'independent organization closure preserves account and personal work while cleaning its fixed scope',
  { timeout: 60000 },
  async () => {
    const shared = await organizationFixture(1);
    const sharedOrg = json(
      await get(`/v1/organizations/${shared.organization}`, shared.owner.headers),
    );
    const denied = await post(
      `/v1/organizations/${shared.organization}/closures`,
      {
        operation_id: operation('organization-close-denied'),
        expected_revision: sharedOrg.revision,
      },
      shared.owner.headers,
    );
    assert.equal(denied.status, 409, denied.body);
    assert.equal(json(denied).code, 'ORGANIZATION_OWNER_HANDOFF_REQUIRED');
    const f = await organizationFixture(0);
    const owner = f.owner;
    const publishedDocument = await documentFixture(f);
    const privateDocument = await documentFixture(f);
    const personalSpace = json(await get('/v1/me/spaces', owner.headers)).items.find(
      (s) => s.kind === 'PERSONAL',
    );
    assert.ok(personalSpace);
    const madeKB = await post(
      `/v1/spaces/${personalSpace.id}/knowledge-bases`,
      { operation_id: operation('personal-kb'), name: '组织关闭后继续写作' },
      owner.headers,
    );
    assert.equal(madeKB.status, 201, madeKB.body);
    const personal = { ...f, space: personalSpace.id, kb: json(madeKB).result.entity_id };
    const personalDocument = await documentFixture(personal);
    const beforeMe = json(await get('/v1/account/me', owner.headers));
    const docDB = await reviewDatabase(),
      accountDB = await reviewDatabase('account'),
      nodeDB = await reviewDatabase('collaboration');
    let main, browser;
    try {
      await unlockStructurePreview({ d: publishedDocument, user: owner });
      main = await openReviewWorkspace(f, publishedDocument, owner, 'MAIN');
      await writeDraft(main, '组织独立关闭前的固定公开版');
      const published = await publishRelease(
        publishedDocument.id,
        owner,
        await releaseInput(publishedDocument.id, owner),
      );
      const retainedRelease = published.result.entity_id;
      await writeDraft(main, '发布之后的私有内容不得被留存');
      main.close();
      main = undefined;
      for (const path of [`/v1/knowledge-bases/${f.kb}`, `/v1/documents/${publishedDocument.id}`]) {
        const before = json(await get(path, owner.headers));
        await waitAuthorization(
          await post(
            path + '/visibility',
            {
              operation_id: operation('org-public'),
              expected_visibility_revision: before.visibility_revision,
              policy: 'PUBLIC',
            },
            owner.headers,
          ),
          owner,
        );
      }
      const publicBefore = json(await get(`/v1/public/documents/${publishedDocument.id}`));
      assert.equal(publicBefore.release_id, retainedRelease);
      const beforeOrganization = json(
        await get(`/v1/organizations/${f.organization}`, owner.headers),
      );
      const input = {
        operation_id: operation('organization-close'),
        expected_revision: beforeOrganization.revision,
      };
      const started = await post(
        `/v1/organizations/${f.organization}/closures`,
        input,
        owner.headers,
      );
      assert.equal(started.status, 202, started.body);
      let final;
      const deadline = Date.now() + 20000;
      while (Date.now() < deadline) {
        const progress = await get(`/v1/account/operations/${input.operation_id}`, owner.headers);
        assert.equal(progress.status, 200, progress.body);
        final = json(progress);
        assert.equal(final.operation_id, input.operation_id);
        if (['SUCCEEDED', 'REJECTED'].includes(final.state)) break;
        await pause(100);
      }
      assert.equal(final?.state, 'SUCCEEDED', `ORGANIZATION_CLOSE_NOT_TERMINAL ${final?.state}`);
      const replay = await post(
        `/v1/organizations/${f.organization}/closures`,
        input,
        owner.headers,
      );
      assert.equal(replay.status, 202, replay.body);
      assert.deepEqual(json(replay), final);
      const conflicting = await post(
        `/v1/organizations/${f.organization}/closures`,
        { ...input, expected_revision: String(BigInt(input.expected_revision) + 1n) },
        owner.headers,
      );
      assert.equal(conflicting.status, 409, conflicting.body);
      const [[organization]] = await accountDB.query(
        'SELECT lifecycle FROM organization WHERE id=?',
        [f.organization],
      );
      assert.equal(organization.lifecycle, 'CLOSED');
      const [[account]] = await accountDB.query('SELECT state FROM user WHERE id=?', [owner.id]);
      assert.equal(account.state, 'ACTIVE');
      const me = await get('/v1/account/me', owner.headers);
      assert.equal(me.status, 200, me.body);
      assert.equal(json(me).identity_epoch, beforeMe.identity_epoch);
      const [[retained]] = await docDB.query(
        'SELECT lifecycle,responsible_user_id FROM document WHERE id=?',
        [publishedDocument.id],
      );
      assert.equal(retained.lifecycle, 'PUBLIC_RETAINED');
      assert.equal(retained.responsible_user_id, null);
      const afterPublic = await get(`/v1/public/documents/${publishedDocument.id}`);
      assert.equal(afterPublic.status, 200, afterPublic.body);
      assert.deepEqual(json(afterPublic).units, publicBefore.units);
      assert.doesNotMatch(JSON.stringify(json(afterPublic).units), /不得被留存/);
      const [[purged]] = await docDB.query(
        'SELECT lifecycle,current_manifest_id FROM document WHERE id=?',
        [privateDocument.id],
      );
      assert.equal(purged.lifecycle, 'PURGED');
      assert.equal(purged.current_manifest_id, null);
      const [cleanups] = await nodeDB.query(
        "SELECT state,payload FROM durable_task WHERE kind='ACCOUNT_DOCUMENT_CLEANUP' AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.operation_id'))=?",
        [input.operation_id],
      );
      assert.equal(cleanups.length, 2);
      assert.ok(
        cleanups.every(
          (t) =>
            t.state === 'SUCCEEDED' &&
            t.payload.operation_id === input.operation_id &&
            t.payload.plan_digest.length === 64,
        ),
      );
      assert.equal(
        (await get(`/v1/documents/${personalDocument.id}/main`, owner.headers)).status,
        200,
      );
      const continued = await documentFixture(personal);
      await unlockStructurePreview({ d: continued, user: owner });
      main = await openReviewWorkspace(personal, continued, owner, 'MAIN');
      await writeDraft(main, '组织已经关闭，个人账号仍然可以保存');
      main.close();
      main = undefined;
      const savedMain = json(await get(`/v1/documents/${continued.id}/main`, owner.headers));
      const [[savedVersion]] = await docDB.query(
        'SELECT canonical_body FROM content_version WHERE id=? AND document_id=?',
        [savedMain.units[0].content_version_id, continued.id],
      );
      assert.match(savedVersion.canonical_body.toString('utf8'), /仍然可以保存/);
      assert.equal((await get(`/v1/documents/${publishedDocument.id}`, owner.headers)).status, 404);
      // 真正的组织关闭202丢回复后，页面只续接原号；刷新后仍可查原终态。
      const browserFixture = await organizationFixture(1);
      const browserDocument = await documentFixture(browserFixture);
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 900 },
      });
      await page.goto(origin);
      await page.getByLabel('登录邮箱', { exact: true }).fill(browserFixture.owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(browserFixture.owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
      await page.getByRole('button', { name: '组织', exact: true }).click();
      await page.getByRole('button', { name: '集成验证组织', exact: true }).click();
      await page.getByRole('tab', { name: '成员与管理', exact: true }).click();
      await page.getByRole('button', { name: '更多操作', exact: true }).click();
      await page.getByRole('button', { name: '关闭组织…', exact: true }).click();
      const panel = page.getByRole('region', { name: '独立关闭组织' });
      const confirm = panel.getByRole('button', { name: '关闭组织并清理', exact: true });
      await expect(confirm).toBeDisabled();
      await panel.getByLabel('输入组织名称以确认关闭', { exact: true }).fill('集成验证组织');
      // 明确未受理的409必须允许完成成员处理后重新确认；未知请求仍保留原号。
      await confirm.click();
      await expect(panel.getByRole('alert')).toBeVisible();
      await expect(panel.getByRole('alert')).not.toContainText(
        'ORGANIZATION_OWNER_HANDOFF_REQUIRED',
      );
      await expect(confirm).toBeVisible();
      assert.equal(new URL(page.url()).searchParams.has('organization_close_operation'), false);
      const members = json(
        await get(
          `/v1/organizations/${browserFixture.organization}/members`,
          browserFixture.owner.headers,
        ),
      );
      const member = members.items.find((item) => item.user_id === browserFixture.members[0].id);
      assert.ok(member);
      await waitOperation(
        await post(
          `/v1/organizations/${browserFixture.organization}/members/${member.user_id}`,
          { operation_id: operation('close-remove-member'), expected_revision: member.revision },
          browserFixture.owner.headers,
          'DELETE',
        ),
        browserFixture.owner,
      );
      await panel.getByRole('button', { name: '取消', exact: true }).click();
      await page.getByRole('button', { name: '刷新组织', exact: true }).click();
      await page.getByRole('button', { name: '更多操作', exact: true }).click();
      await page.getByRole('button', { name: '关闭组织…', exact: true }).click();
      await panel.getByLabel('输入组织名称以确认关闭', { exact: true }).fill('集成验证组织');
      const originals = [];
      let loseReply = true;
      await page.route(
        `**/v1/organizations/${browserFixture.organization}/closures`,
        async (route) => {
          originals.push(route.request().postDataJSON());
          if (loseReply) {
            loseReply = false;
            const response = await route.fetch();
            assert.equal(response.status(), 202);
            await route.abort('failed');
          } else await route.continue();
        },
      );
      await confirm.click();
      await panel.getByRole('button', { name: '确认操作结果', exact: true }).click();
      await expect(panel.getByText('组织已关闭，个人账号继续有效', { exact: true })).toBeVisible({
        timeout: 20000,
      });
      assert.equal(originals.length, 2);
      assert.deepEqual(originals[0], originals[1]);
      assert.equal(
        new URL(page.url()).searchParams.get('organization_close_operation'),
        originals[0].operation_id,
      );
      await page.reload();
      await expect(
        page
          .getByRole('region', { name: '独立关闭组织' })
          .getByText('组织已关闭，个人账号继续有效', { exact: true }),
      ).toBeVisible();
      assert.equal((await get('/v1/account/me', browserFixture.owner.headers)).status, 200);
      const [[cleanedBrowserDocument]] = await docDB.query(
        'SELECT lifecycle FROM document WHERE id=?',
        [browserDocument.id],
      );
      assert.equal(cleanedBrowserDocument.lifecycle, 'PURGED');
      const [[refundedCapacity]] = await docDB.query(
        'SELECT COUNT(*) count FROM document_capacity WHERE document_id=?',
        [browserDocument.id],
      );
      assert.equal(Number(refundedCapacity.count), 0);
      await page.screenshot({
        path: resolve(root, 'artifacts/organization-close-browser.png'),
        fullPage: true,
      });
      await page
        .getByRole('region', { name: '独立关闭组织' })
        .getByRole('button', { name: '结束查看此操作', exact: true })
        .click();
      assert.equal(new URL(page.url()).searchParams.has('organization_close_operation'), false);
    } finally {
      main?.close();
      await browser?.close();
      await Promise.all([docDB.end(), accountDB.end(), nodeDB.end()]);
    }
  },
);
