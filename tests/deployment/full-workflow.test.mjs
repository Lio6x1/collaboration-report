import assert from 'node:assert/strict';
import test from 'node:test';
import { randomBytes } from 'node:crypto';
import { readFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import * as Y from 'yjs';
import { chromium, expect } from '@playwright/test';
import { root, state, pause } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import {
  openReviewWorkspace,
  writeDraftBody,
  submitDraft,
  reviewDatabase,
} from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { pollMerge } from '../helpers/merge.mjs';
import { pullMain } from '../helpers/pull-main.mjs';
import { publishRelease, releaseInput } from '../helpers/publication.mjs';
import { pageVisibility, waitSearchPagesQueryable } from '../helpers/search-pages.mjs';
import {
  searchDatabase,
  waitSearchReceipt,
  searchProjection,
  assertKafkaCommitted,
} from '../helpers/search.mjs';
import { uploadPNG, binaryResourceGet } from '../helpers/resource.mjs';

async function until(read, accepts, code, budget = 20000) {
  const deadline = Date.now() + budget;
  while (Date.now() < deadline) {
    const value = await read();
    if (accepts(value)) return value;
    await pause(100);
  }
  assert.fail(code);
}
async function documentMetadata(d, user) {
  const response = await get(`/v1/documents/${d.id}`, user.headers);
  assert.equal(response.status, 200, response.body);
  return json(response);
}
async function fixedMain(d, user) {
  const response = await get(`/v1/documents/${d.id}/main`, user.headers);
  assert.equal(response.status, 200, response.body);
  const view = json(response);
  const content = await get(
    `/v1/documents/${d.id}/read-views/${view.read_view_id}/units/${view.units[0].unit_id}`,
    user.headers,
  );
  assert.equal(content.status, 200, content.body);
  return { view, body: json(content) };
}
async function trash(d, f) {
  const current = await documentMetadata(d, f.owner);
  const result = await waitAuthorization(
    await post(
      `/v1/documents/${d.id}`,
      { operation_id: operation('workflow-trash'), expected_revision: current.control_revision },
      f.owner.headers,
      'DELETE',
    ),
    f.owner,
  );
  assert.equal(result.state, 'SUCCEEDED');
  const page = await get(`/v1/spaces/${f.space}/trash?limit=100`, f.owner.headers);
  assert.equal(page.status, 200, page.body);
  const item = json(page).items.find((row) => row.id === d.id);
  assert.equal(item?.lifecycle, 'TRASHED');
  return item;
}

// 一个文档贯穿两人真实保存、固定审核合并、图片/通知、正式发布、真实消费搜索、
// 浏览器阅读、导出及回收生命周期；历史故障矩阵沿用原证据，不在这里重新排列。
test(
  'full product workflow retains one document across collaboration review publication search export and cleanup',
  { timeout: 180000 },
  async () => {
    const f = await organizationFixture(1),
      d = await documentFixture(f);
    const colleague = f.members[0],
      db = await reviewDatabase(),
      searchDB = await searchDatabase();
    const q = 'workflow' + randomBytes(8).toString('hex');
    const directory = resolve(root, 'artifacts/batches/08');
    mkdirSync(directory, { recursive: true });
    const evidence = { state: 'RUNNING', document: d.id, stages: [] };
    const record = (stage) => {
      if (stage) {
        evidence.stages.push(stage);
        console.log('FULL_WORKFLOW_STAGE_PASS: ' + stage);
      }
      writeFileSync(
        resolve(directory, 'full-workflow-last-attempt.json'),
        JSON.stringify(evidence, null, 2) + '\n',
      );
    };
    const peers = [];
    let browser;
    try {
      const current = await documentMetadata(d, f.owner);
      const granted = await post(
        `/v1/documents/${d.id}/authorization-changes`,
        {
          operation_id: operation('workflow-grants'),
          expected_acl_revision: current.acl_revision,
          // 正文编辑只能授权章节/单元，审核授权则绑定整文档；
          // 使用真实权限范围，避免把非法测试输入误报成产品故障。
          changes: [
            {
              action: 'GRANT',
              grant: {
                user_id: colleague.id,
                scope_kind: 'UNIT',
                scope_id: d.main.units[0].unit_id,
                capability: 'EDIT',
              },
            },
            {
              action: 'GRANT',
              grant: {
                user_id: colleague.id,
                scope_kind: 'DOCUMENT',
                scope_id: d.id,
                capability: 'REVIEW',
              },
            },
          ],
        },
        f.owner.headers,
      );
      assert.ok([200, 202].includes(granted.status), `${granted.status}: ${granted.body}`);
      const permissions = await waitAuthorization(granted, f.owner);
      assert.equal(permissions.state, 'SUCCEEDED');
      await unlockStructurePreview({ d, user: f.owner });
      const owner = await openReviewWorkspace(f, d, f.owner, 'MAIN');
      peers.push(owner);
      const second = await openReviewWorkspace(f, d, colleague, 'MAIN');
      peers.push(second);
      assert.equal(owner.session.room_id, second.session.room_id);
      const png = readFileSync(resolve(root, 'tests/fixtures/export-image.png'));
      const resource = await uploadPNG(owner, png);
      const text = `真实整站闭环 ${q} 主线作者保存；`;
      const body = (value) => ({
        type: 'doc',
        content: [
          {
            type: 'paragraph',
            attrs: { id: 'workflow-paragraph' },
            content: [
              { type: 'text', text: value },
              {
                type: 'image',
                attrs: {
                  resourceId: resource,
                  alt: '整站固定图片',
                  title: null,
                  width: 320,
                  height: 80,
                },
              },
            ],
          },
        ],
      });
      assert.equal((await writeDraftBody(owner, body(text))).type, 'SAVED');
      second.w.send({ type: 'SYNC' });
      Y.applyUpdate(second.doc, Buffer.from((await second.w.next('SYNC')).update, 'base64'));
      const vector = Y.encodeStateVector(second.doc);
      const node = second.doc.getXmlFragment('prosemirror').get(0).get(0);
      node.insert(node.length, '第二位作者共同保存；');
      second.w.send({
        type: 'UPDATE',
        client_update_id: operation('workflow-second-save'),
        update: Buffer.from(Y.encodeStateAsUpdate(second.doc, vector)).toString('base64'),
      });
      assert.equal((await second.w.next('SAVED')).type, 'SAVED');
      const shared = await fixedMain(d, f.owner);
      assert.match(JSON.stringify(shared.body.body_json), /第二位作者共同保存/);
      for (const peer of peers.splice(0)) peer.close();
      const latest = await documentMetadata(d, f.owner);
      const locked = await completedMainControl(
        await post(
          `/v1/documents/${d.id}/main-lock`,
          {
            operation_id: operation('workflow-lock'),
            expected_control_revision: latest.control_revision,
            locked: true,
          },
          f.owner.headers,
        ),
        f.owner,
      );
      assert.equal(locked.state, 'SUCCEEDED');
      assert.equal(locked.result.main_sync_state, 'READY');
      record('two real authors, durable shared MAIN and fixed baseline');

      let branch = await openReviewWorkspace(f, d, f.owner, 'BRANCH');
      peers.push(branch);
      // MAIN 直接保存不会偷偷改写旧支线的共同基线；作者先主动拉取，
      // 取得新的工作区代次后再编写增量，随后送审的是这份真实固定输入。
      const pulled = await pullMain(branch);
      assert.equal(pulled.value.state, 'SUCCEEDED', JSON.stringify(pulled.value.error));
      peers.pop().close();
      branch = await openReviewWorkspace(f, d, f.owner, 'BRANCH');
      peers.push(branch);
      const reviewedText = text + '第二位作者共同保存；已审核的支线增量。';
      assert.equal((await writeDraftBody(branch, body(reviewedText))).type, 'SAVED');
      const submitted = await submitDraft(branch);
      assert.equal(submitted.value.state, 'SUCCEEDED');
      const fixed = submitted.value.result;
      const notifications = async () => {
        const response = await get('/v1/notifications', colleague.headers);
        assert.equal(response.status, 200, response.body);
        return json(response).items;
      };
      const notices = await until(
        notifications,
        (rows) => rows.some((row) => row.event_id === submitted.input.operation_id),
        'WORKFLOW_REVIEW_NOTIFICATION',
      );
      const notice = notices.find((row) => row.event_id === submitted.input.operation_id);
      const round = await get(`/v1/notifications/${notice.id}/review-round`, colleague.headers);
      assert.equal(round.status, 200, round.body);
      assert.equal(json(round).content_version_id, fixed.content_version_id);
      assert.ok(JSON.stringify(json(round).content.body_json).includes(reviewedText));
      const commentInput = {
        operation_id: operation('workflow-comment'),
        context_kind: 'REVIEW',
        context_id: fixed.round_id,
        anchor: { schema_version: 'anchor-v1', content_version_id: fixed.content_version_id },
        body: '请核对本轮固定正文与图片。',
        mentions: [colleague.id],
      };
      const comment = await post(
        `/v1/documents/${d.id}/comment-threads`,
        commentInput,
        f.owner.headers,
      );
      assert.equal(comment.status, 201, comment.body);
      const mentions = await until(
        notifications,
        (rows) => rows.some((row) => row.event_id === commentInput.operation_id),
        'WORKFLOW_COMMENT_NOTIFICATION',
      );
      assert.equal(mentions.filter((row) => row.event_id === commentInput.operation_id).length, 1);
      assert.equal(
        mentions.find((row) => row.event_id === commentInput.operation_id).target_id,
        json(comment).result.entity_id,
      );
      const mergeInput = {
        operation_id: operation('workflow-merge'),
        round_id: fixed.round_id,
        content_version_id: fixed.content_version_id,
        own_changes_to_resolve: [],
      };
      const merge = await post(
        `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
        mergeInput,
        f.owner.headers,
      );
      assert.equal(merge.status, 202, merge.body);
      const merged = await pollMerge(mergeInput.operation_id, f.owner.headers, db);
      assert.equal(merged.state, 'SUCCEEDED', JSON.stringify(merged.error));
      assert.equal(merged.result.main_sync_state, 'READY');
      for (const peer of peers.splice(0)) peer.close();
      const mergedBody = await fixedMain(d, f.owner);
      assert.ok(JSON.stringify(mergedBody.body.body_json).includes(reviewedText));
      record('immutable review, image, exact notifications and installed merge');

      await pageVisibility(`/v1/knowledge-bases/${f.kb}`, f.owner, 'PUBLIC');
      await pageVisibility(`/v1/documents/${d.id}`, f.owner, 'PUBLIC');
      const releaseOperation = await releaseInput(d.id, f.owner);
      const published = await publishRelease(d.id, f.owner, releaseOperation);
      const publicResponse = await get(`/v1/public/documents/${d.id}`);
      assert.equal(publicResponse.status, 200, publicResponse.body);
      const formal = json(publicResponse);
      assert.equal(formal.release_id, published.result.entity_id);
      assert.ok(JSON.stringify(formal.units).includes(reviewedText));
      const receipt = await waitSearchReceipt(searchDB, releaseOperation.operation_id);
      const projection = await searchProjection(d.id, receipt.physical);
      assert.equal(projection.release_id, formal.release_id);
      assert.ok(projection.text.includes(reviewedText));
      await assertKafkaCommitted(receipt);
      await waitSearchPagesQueryable(receipt.physical, q, 1, d.id);
      const found = await get('/v1/search?q=' + q);
      assert.equal(found.status, 200, found.body);
      assert.equal(json(found).items.filter((item) => item.document_id === d.id).length, 1);
      const picture = await binaryResourceGet(
        `/v1/resources/${resource}/content?document_id=${d.id}&context_kind=PUBLIC&context_id=${formal.release_id}`,
      );
      assert.equal(picture.status, 200, picture.body.toString());
      assert.deepEqual(picture.body, png);
      record('fixed public release, original Kafka receipt and searchable ES projection');

      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1360, height: 960 },
      });
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.goto('https://localhost:' + state().ports.edge + '/search');
      await page.getByLabel('搜索词', { exact: true }).fill(q);
      await page.getByRole('button', { name: '搜索', exact: true }).click();
      const result = page.getByRole('region', { name: '搜索结果', exact: true }).getByRole('link');
      await expect(result).toHaveCount(1);
      await result.click();
      await expect(page.locator('.content-view .ProseMirror').first()).toContainText(reviewedText);
      await page.screenshot({
        path: resolve(directory, 'full-workflow-public.png'),
        fullPage: true,
      });
      assert.deepEqual(errors, []);
      await browser.close();
      browser = undefined;
      record('actual anonymous browser search and fixed chapter reading');

      const exportInput = {
        operation_id: operation('workflow-export'),
        target: 'PUBLIC',
        format: 'PDF',
        expected_release_id: formal.release_id,
      };
      const created = await post(`/v1/documents/${d.id}/exports`, exportInput, f.owner.headers);
      assert.equal(created.status, 202, created.body);
      const job = json(created);
      const exported = await until(
        async () => {
          const response = await get(`/v1/export-jobs/${job.id}`, f.owner.headers);
          assert.equal(response.status, 200, response.body);
          return json(response);
        },
        (row) => !['QUEUED', 'RUNNING'].includes(row.state),
        'WORKFLOW_EXPORT_TERMINAL',
        75000,
      );
      assert.equal(exported.state, 'SUCCEEDED', JSON.stringify(exported.last_error));
      const file = await binaryResourceGet(`/v1/export-jobs/${job.id}/download`, f.owner.headers);
      assert.equal(file.status, 200, file.body.toString());
      assert.equal(file.body.subarray(0, 5).toString(), '%PDF-');
      writeFileSync(resolve(directory, 'full-workflow-fixed.pdf'), file.body);
      const replay = await post(`/v1/documents/${d.id}/exports`, exportInput, f.owner.headers);
      assert.equal(replay.status, 202, replay.body);
      assert.equal(json(replay).id, job.id);
      record('actual fixed PDF and original export identity');

      const trashed = await trash(d, f);
      assert.equal((await get(`/v1/public/documents/${d.id}`)).status, 404);
      const restore = await post(
        `/v1/documents/${d.id}/restore`,
        {
          operation_id: operation('workflow-restore'),
          expected_control_revision: trashed.control_revision,
        },
        f.owner.headers,
      );
      assert.equal(restore.status, 202, restore.body);
      assert.equal(json(restore).state, 'SUCCEEDED');
      assert.equal((await documentMetadata(d, f.owner)).publication_state, 'UNPUBLISHED');
      const recovered = await fixedMain(d, f.owner);
      assert.deepEqual(recovered.body.body_json, mergedBody.body.body_json);
      const finalTrash = await trash(d, f);
      const purgeInput = {
        operation_id: operation('workflow-purge'),
        expected_control_revision: finalTrash.control_revision,
        confirmed: true,
      };
      const purged = await post(`/v1/documents/${d.id}/purge`, purgeInput, f.owner.headers);
      assert.equal(purged.status, 202, purged.body);
      const cleared = await until(
        async () => {
          const response = await get(`/v1/operations/${purgeInput.operation_id}`, f.owner.headers);
          assert.equal(response.status, 200, response.body);
          return json(response);
        },
        (value) => value.state !== 'PREPARING',
        'WORKFLOW_PURGE_TERMINAL',
        30000,
      );
      assert.equal(cleared.state, 'SUCCEEDED', JSON.stringify(cleared.error));
      const [[remaining]] = await db.query(
        'SELECT (SELECT COUNT(*) FROM content_version WHERE document_id=?) versions,(SELECT COUNT(*) FROM document_capacity WHERE document_id=?) capacity',
        [d.id, d.id],
      );
      assert.deepEqual(Object.values(remaining).map(String), ['0', '0']);
      record('trash, private restore, original purge and physical content accounting release');
      evidence.state = 'PASS';
      record();
      writeFileSync(
        resolve(directory, 'full-workflow.json'),
        JSON.stringify(evidence, null, 2) + '\n',
      );
    } finally {
      for (const peer of peers) peer.close();
      await browser?.close();
      await db.end();
      await searchDB.end();
      if (evidence.state !== 'PASS') {
        evidence.state = 'FAIL';
        record();
      }
    }
  },
);
