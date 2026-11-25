import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { resolve } from 'node:path';
import { root, state } from '../../scripts/runtime-lib.mjs';
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
import { publishRelease, releaseInput, pollPublication } from '../helpers/publication.mjs';
import { searchDatabase, waitSearchReceipt, searchProjection } from '../helpers/search.mjs';
import { createHash } from 'node:crypto';

// 一个真实用户流程验证发布删除→旧输出关闭→分页/当前责任人→恢复私有→新MAIN保存。
// 原号丢回复恢复沿标准HTTP操作，不伪造TRASHED、ACK、正文或任务成功。
test(
  'published documents enter trash and restore privately with current authority and original-operation recovery',
  { timeout: 120000 },
  async () => {
    const f = await organizationFixture(3);
    const [responsible, oldReader, observer] = f.members;
    const d = await documentFixture(f),
      other = await documentFixture(f);
    const db = await reviewDatabase(),
      search = await searchDatabase();
    let main, browser;
    try {
      await unlockStructurePreview({ d, user: f.owner });
      main = await openReviewWorkspace(f, d, f.owner, 'MAIN');
      await writeDraft(main, '回收站之前的固定公开正文');
      const release = (await publishRelease(d.id, f.owner, await releaseInput(d.id, f.owner)))
        .result.entity_id;
      await writeDraft(main, '应在恢复后保留的后来主线正文');
      main.close();
      main = undefined;
      for (const path of [`/v1/knowledge-bases/${f.kb}`, `/v1/documents/${d.id}`]) {
        const facts = json(await get(path, f.owner.headers));
        await waitAuthorization(
          await post(
            path + '/visibility',
            {
              operation_id: operation('trash-public'),
              expected_visibility_revision: facts.visibility_revision,
              policy: 'PUBLIC',
            },
            f.owner.headers,
          ),
          f.owner,
        );
      }
      assert.equal((await get(`/v1/public/documents/${d.id}`)).status, 200);
      let before = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
      await waitAuthorization(
        await post(
          `/v1/documents/${d.id}/authorization-changes`,
          {
            operation_id: operation('trash-reader'),
            expected_acl_revision: before.acl_revision,
            changes: [
              {
                action: 'GRANT',
                grant: {
                  user_id: oldReader.id,
                  scope_kind: 'DOCUMENT',
                  scope_id: d.id,
                  capability: 'COMMENT',
                },
              },
            ],
          },
          f.owner.headers,
        ),
        f.owner,
      );
      before = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
      await waitAuthorization(
        await post(
          `/v1/documents/${d.id}/responsible-transfers`,
          {
            operation_id: operation('trash-responsible'),
            expected_revision: before.control_revision,
            target_user_id: responsible.id,
          },
          f.owner.headers,
        ),
        f.owner,
      );
      const source = json(await get(`/v1/documents/${d.id}/main`, responsible.headers));
      const fixedBodies = async (view) => {
        const bodies = [];
        for (const unit of view.units) {
          const [[body]] = await db.query(
            'SELECT HEX(canonical_body) body,HEX(body_hash) hash FROM content_version WHERE id=? AND document_id=? AND unit_id=?',
            [unit.content_version_id, d.id, unit.unit_id],
          );
          assert.ok(body, 'ACTUAL_FIXED_MAIN_VERSION_MISSING');
          bodies.push({ unit: unit.unit_id, ...body });
        }
        return bodies;
      };
      const sourceBodies = await fixedBodies(source);
      const counts = async () => {
        const [[row]] = await db.query(
          'SELECT (SELECT COUNT(*) FROM content_version WHERE document_id=?) versions,(SELECT COUNT(*) FROM `release` WHERE document_id=?) releases',
          [d.id, d.id],
        );
        return { ...row };
      };
      const retained = await counts();
      const deletes = [];
      for (const item of [d, other]) {
        const current = json(await get(`/v1/documents/${item.id}`, f.owner.headers));
        const input = {
          operation_id: operation('trash-delete'),
          expected_revision: current.control_revision,
        };
        const response = await post(`/v1/documents/${item.id}`, input, f.owner.headers, 'DELETE');
        assert.equal(response.status, 202, response.body);
        const settled = await pollPublication(input.operation_id, f.owner);
        assert.equal(settled.state, 'SUCCEEDED');
        deletes.push(input);
      }
      assert.equal((await get(`/v1/public/documents/${d.id}`)).status, 404);
      // 普通正文入口只接受ACTIVE，已回收文档与不存在文档均返回404。
      assert.equal((await get(`/v1/documents/${d.id}/main`, responsible.headers)).status, 404);
      const firstResponse = await get(`/v1/spaces/${f.space}/trash?limit=1`, f.owner.headers);
      assert.equal(firstResponse.status, 200, firstResponse.body);
      const first = json(firstResponse);
      assert.deepEqual(
        first.items.map((x) => x.id),
        [d.id],
      );
      assert.equal(first.has_more, true);
      const second = json(
        await get(
          `/v1/spaces/${f.space}/trash?limit=1&cursor=${encodeURIComponent(first.next_cursor)}`,
          f.owner.headers,
        ),
      );
      assert.deepEqual(
        second.items.map((x) => x.id),
        [other.id],
      );
      assert.equal(second.has_more, false);
      const owned = json(await get(`/v1/spaces/${f.space}/trash`, responsible.headers));
      assert.deepEqual(
        owned.items.map((x) => x.id),
        [d.id],
      );
      assert.deepEqual(json(await get(`/v1/spaces/${f.space}/trash`, observer.headers)).items, []);
      const restore = {
        operation_id: operation('trash-restore'),
        expected_control_revision: first.items[0].control_revision,
      };
      assert.equal(
        (
          await post(
            `/v1/documents/${d.id}/restore`,
            { ...restore, operation_id: operation('trash-forbidden') },
            oldReader.headers,
          )
        ).status,
        403,
      );
      assert.equal(
        (
          await post(
            `/v1/documents/${d.id}/restore`,
            {
              ...restore,
              operation_id: operation('trash-stale'),
              expected_control_revision: before.control_revision,
            },
            responsible.headers,
          )
        ).status,
        409,
      );
      const restored = await post(`/v1/documents/${d.id}/restore`, restore, responsible.headers);
      assert.equal(restored.status, 202, restored.body);
      assert.equal(json(restored).state, 'SUCCEEDED');
      const after = json(await get(`/v1/documents/${d.id}`, responsible.headers));
      assert.equal(after.lifecycle, 'ACTIVE');
      assert.equal(after.visibility_policy, 'COLLABORATORS');
      assert.equal(after.publication_state, 'UNPUBLISHED');
      assert.equal(after.current_release_id, undefined);
      assert.deepEqual(await counts(), retained);
      assert.equal((await get(`/v1/public/documents/${d.id}`)).status, 404);
      assert.equal((await get(`/v1/documents/${d.id}`, oldReader.headers)).status, 403);
      const replay = await post(`/v1/documents/${d.id}/restore`, restore, responsible.headers);
      assert.equal(replay.status, 202, replay.body);
      assert.deepEqual(json(replay), json(restored));
      assert.equal(
        (
          await post(
            `/v1/documents/${d.id}/restore`,
            { ...restore, reason: '同号改输入' },
            responsible.headers,
          )
        ).status,
        409,
      );
      const keptMainResponse = await get(`/v1/documents/${d.id}/main`, responsible.headers);
      assert.equal(keptMainResponse.status, 200, keptMainResponse.body);
      assert.deepEqual(await fixedBodies(json(keptMainResponse)), sourceBodies);
      main = await openReviewWorkspace(f, d, responsible, 'MAIN');
      await writeDraft(main, '恢复以后继续编辑并持久保存');
      main.close();
      main = undefined;
      const latest = await get(`/v1/documents/${d.id}/main`, responsible.headers);
      assert.equal(latest.status, 200, latest.body);
      assert.notDeepEqual(await fixedBodies(json(latest)), sourceBodies);
      const event =
        'visibility_index_' +
        createHash('sha256')
          .update(deletes[0].operation_id + '\n' + d.id)
          .digest('hex');
      const indexed = await waitSearchReceipt(search, event, 5000);
      assert.equal((await searchProjection(d.id, indexed.physical)).visible, false);
      assert.ok(release);
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const context = await browser.newContext({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 900 },
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      await page.goto('https://localhost:' + state().ports.edge);
      await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
      await page
        .getByRole('group', { name: '空间类型', exact: true })
        .getByRole('button', { name: '组织空间', exact: true })
        .click();
      await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
      // 回收站按空间管理；知识库已改为卡片，不再先找旧下拉框或页底折叠入口。
      const navigation = page.getByRole('navigation', { name: '工作空间导航', exact: true });
      await navigation.getByRole('button', { name: '回收站', exact: true }).click();
      const row = page
        .getByRole('list', { name: '回收文档列表', exact: true })
        .locator('li')
        .filter({ hasText: 'ID ' + other.id });
      await expect(row).toHaveCount(1);
      const operations = [];
      let lose = true;
      await page.route(`**/v1/documents/${other.id}/restore`, async (route) => {
        operations.push(route.request().postDataJSON().operation_id);
        if (lose) {
          lose = false;
          const response = await route.fetch();
          assert.equal(response.status(), 202);
          await route.abort('failed');
        } else await route.continue();
      });
      await row.getByRole('button', { name: '恢复文档', exact: true }).click();
      await page.getByRole('button', { name: '确认操作结果', exact: true }).click();
      await expect(row).toHaveCount(0);
      assert.equal(operations.length, 2);
      assert.equal(new Set(operations).size, 1);
      await page.reload();
      await navigation.getByRole('button', { name: '回收站', exact: true }).click();
      await expect(page.getByText('本页没有当前可管理的回收文档。', { exact: true })).toBeVisible();
      await page.screenshot({ path: resolve(root, 'artifacts/trash-restored-browser.png') });
      console.log(
        'TRASH_RESTORE_ACTUAL_PASS: published deletion and real ACK; private restoration/current responsibility; signed pages; no old grants; preserved history and new WSS save; exact original browser retry and actual hidden search',
      );
    } finally {
      main?.close();
      await browser?.close();
      await Promise.all([db.end(), search.end()]);
    }
  },
);
