import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root, pause } from '../../scripts/runtime-lib.mjs';
import {
  createUser,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { openReviewWorkspace, writeDraftBody, reviewDatabase } from '../helpers/review.mjs';
import { uploadPNG, binaryResourceGet } from '../helpers/resource.mjs';
import { chromium, expect } from '@playwright/test';
import { origin } from '../helpers/account.mjs';

async function personalFixture(user) {
  user ??= await createUser();
  const directory = await get('/v1/me/spaces?limit=1', user.headers);
  assert.equal(directory.status, 200, directory.body);
  const space = json(directory).items.find((s) => s.kind === 'PERSONAL');
  assert.ok(space);
  const made = await post(
    `/v1/spaces/${space.id}/knowledge-bases`,
    { operation_id: operation('personal-kb'), name: '接收个人文档' },
    user.headers,
  );
  assert.equal(made.status, 201, made.body);
  return { owner: user, space: space.id, kb: json(made).result.entity_id };
}
function body(text, resource) {
  return {
    type: 'doc',
    content: [
      {
        type: 'paragraph',
        attrs: { id: 'personal-transfer-original' },
        content: [
          { type: 'text', text },
          ...(resource
            ? [
                {
                  type: 'image',
                  attrs: {
                    resourceId: resource,
                    alt: '稳定原图片',
                    title: null,
                    width: 320,
                    height: 80,
                  },
                },
              ]
            : []),
        ],
      },
    ],
  };
}
async function propose(f, d, receiver) {
  const current = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
  const input = {
    operation_id: operation('personal-offer'),
    expected_revision: current.control_revision,
    target_user_id: receiver.id,
  };
  const offered = await post(`/v1/documents/${d.id}/personal-transfers`, input, f.owner.headers);
  assert.equal(offered.status, 202, offered.body);
  assert.equal(json(offered).state, 'SUCCEEDED');
  const replay = await post(`/v1/documents/${d.id}/personal-transfers`, input, f.owner.headers);
  assert.deepEqual(json(replay), json(offered));
  return json(
    await get(`/v1/personal-transfers/${json(offered).result.entity_id}`, receiver.headers),
  );
}
async function finish(input, user) {
  const deadline = Date.now() + 20000;
  let result;
  while (Date.now() < deadline) {
    const r = await get(`/v1/operations/${input.operation_id}`, user.headers);
    assert.equal(r.status, 200, r.body);
    result = json(r);
    if (['SUCCEEDED', 'REJECTED', 'CANCELLED'].includes(result.state)) return result;
    await pause(100);
  }
  assert.fail(`TRANSFER_NOT_TERMINAL ${result?.state}`);
}

// 真实个人来源、接收操作、Node持久计量与额度落定组成同一场景。SQL只读业务事实；
// 唯一写入是本样本接收空间的真实容量限制，并在finally恢复，不写权限/ACK/成功状态。
test(
  'personal transfer retains ownership until actual acceptance, preserves fixed data and rejects insufficient persistent storage',
  { timeout: 120000 },
  async () => {
    const source = await personalFixture(),
      receiver = await personalFixture(),
      outsider = await personalFixture();
    const d = await documentFixture(source),
      sibling = await documentFixture(source);
    const db = await reviewDatabase(),
      nodeDB = await reviewDatabase('collaboration');
    let main, siblingMain, failedMain, restoredLimit, browser;
    const imageBytes = readFileSync(resolve(root, 'tests/fixtures/export-image.png'));
    try {
      main = await openReviewWorkspace(source, d, source.owner, 'MAIN');
      const image = await uploadPNG(main, imageBytes);
      await writeDraftBody(main, body('转让前固定正文', image));
      const firstMain = json(await get(`/v1/documents/${d.id}/main`, source.owner.headers));
      const version = firstMain.units[0].content_version_id;
      const [[originalVersion]] = await db.query(
        'SELECT LOWER(HEX(body_hash)) hash,canonical_body FROM content_version WHERE id=?',
        [version],
      );
      siblingMain = await openReviewWorkspace(source, sibling, source.owner, 'MAIN');
      await writeDraftBody(siblingMain, body('另一个文档共享原图片', image));
      await get(`/v1/documents/${sibling.id}/main`, source.owner.headers);
      siblingMain.close();
      siblingMain = undefined;
      const offer = await propose(source, d, receiver.owner);
      const [[beforeOwner]] = await db.query(
        'SELECT CAST(responsible_user_id AS CHAR) owner,CAST(space_id AS CHAR) space,lifecycle FROM document WHERE id=?',
        [d.id],
      );
      assert.equal(beforeOwner.owner, source.owner.id);
      assert.equal(beforeOwner.space, source.space);
      assert.equal(beforeOwner.lifecycle, 'ACTIVE');
      const denied = await post(
        `/v1/personal-transfers/${offer.id}/accept`,
        {
          operation_id: operation('wrong-receiver'),
          expected_revision: offer.revision,
          target_knowledge_base_id: outsider.kb,
          expected_quota_revision: json(
            await get(`/v1/spaces/${outsider.space}/quota`, outsider.owner.headers),
          ).revision,
        },
        outsider.owner.headers,
      );
      assert.equal(denied.status, 403, denied.body);
      // 原固定历史不变，MAIN后来仍有尚未生成canonical版本的真实保存更新。
      await writeDraftBody(main, body('接受前最后持久保存的内容', image));
      main.close();
      main = undefined;
      const targetQuota = json(
        await get(`/v1/spaces/${receiver.space}/quota`, receiver.owner.headers),
      );
      const input = {
        operation_id: operation('personal-accept'),
        expected_revision: offer.revision,
        target_knowledge_base_id: receiver.kb,
        expected_quota_revision: targetQuota.revision,
      };
      const accepted = await post(
        `/v1/personal-transfers/${offer.id}/accept`,
        input,
        receiver.owner.headers,
      );
      assert.equal(accepted.status, 202, accepted.body);
      assert.equal((await finish(input, receiver.owner)).state, 'SUCCEEDED');
      assert.equal(
        (await post(`/v1/personal-transfers/${offer.id}/accept`, input, receiver.owner.headers))
          .status,
        202,
      );
      assert.equal(
        (
          await post(
            `/v1/personal-transfers/${offer.id}/accept`,
            { ...input, target_knowledge_base_id: outsider.kb },
            receiver.owner.headers,
          )
        ).status,
        409,
      );
      const [[afterOwner]] = await db.query(
        'SELECT CAST(responsible_user_id AS CHAR) owner,CAST(space_id AS CHAR) space,CAST(knowledge_base_id AS CHAR) kb,lifecycle FROM document WHERE id=?',
        [d.id],
      );
      assert.deepEqual(afterOwner, {
        owner: receiver.owner.id,
        space: receiver.space,
        kb: receiver.kb,
        lifecycle: 'ACTIVE',
      });
      assert.equal((await get(`/v1/documents/${d.id}`, source.owner.headers)).status, 403);
      const fixed = await get(`/v1/content-versions/${version}`, receiver.owner.headers);
      assert.equal(fixed.status, 200, fixed.body);
      const [[unchangedVersion]] = await db.query(
        'SELECT LOWER(HEX(body_hash)) hash,canonical_body FROM content_version WHERE id=?',
        [version],
      );
      assert.deepEqual(unchangedVersion, originalVersion);
      const receiverView = json(await get(`/v1/documents/${d.id}/main`, receiver.owner.headers));
      const download = await binaryResourceGet(
        `/v1/resources/${image}/content?document_id=${d.id}&context_kind=MAIN&context_id=${receiverView.manifest_id}`,
        receiver.owner.headers,
      );
      assert.equal(download.status, 200, download.body.toString());
      assert.deepEqual(download.body, imageBytes);
      const siblingView = json(await get(`/v1/documents/${sibling.id}/main`, source.owner.headers));
      const siblingImage = await binaryResourceGet(
        `/v1/resources/${image}/content?document_id=${sibling.id}&context_kind=MAIN&context_id=${siblingView.manifest_id}`,
        source.owner.headers,
      );
      assert.equal(siblingImage.status, 200);
      assert.deepEqual(siblingImage.body, imageBytes);
      const [[capacity]] = await db.query(
        'SELECT CAST(node_bytes AS CHAR) node,CAST(space_id AS CHAR) space FROM document_capacity WHERE document_id=?',
        [d.id],
      );
      assert.equal(capacity.space, receiver.space);
      assert.ok(BigInt(capacity.node) > 0n);
      const [[measurement]] = await nodeDB.query(
        "SELECT state,payload FROM durable_task WHERE kind='PERSONAL_TRANSFER_STORAGE_MEASUREMENT' AND business_key=?",
        [input.operation_id],
      );
      assert.equal(measurement.state, 'SUCCEEDED');
      assert.equal(measurement.payload.result.source_closed, true);
      assert.equal(measurement.payload.result.storage_bytes, capacity.node);
      main = await openReviewWorkspace(receiver, d, receiver.owner, 'MAIN');
      await writeDraftBody(main, body('转让后接收人继续保存原图片', image));
      main.close();
      main = undefined;
      const savedMain = json(await get(`/v1/documents/${d.id}/main`, receiver.owner.headers));
      const saved = await get(
        `/v1/content-versions/${savedMain.units[0].content_version_id}`,
        receiver.owner.headers,
      );
      assert.equal(saved.status, 200, saved.body);
      assert.match(saved.body, /继续保存原图片/);
      const [allocations] = await db.query(
        'SELECT CAST(space_id AS CHAR) space,state FROM resource_space_allocation WHERE resource_id=? ORDER BY space_id',
        [image],
      );
      assert.equal(allocations.filter((a) => a.state === 'ACTIVE').length, 2);

      const failedDocument = await documentFixture(source);
      failedMain = await openReviewWorkspace(source, failedDocument, source.owner, 'MAIN');
      await writeDraftBody(failedMain, body('完整容量必须包含原持久草稿'));
      await get(`/v1/documents/${failedDocument.id}/main`, source.owner.headers);
      failedMain.close();
      failedMain = undefined;
      const failedOffer = await propose(source, failedDocument, outsider.owner);
      const [[size]] = await db.query(
        // 固定阅读副本不再消耗用户额度；仍给接收方恰好业务历史容量，真实 Node 存储必须额外拒绝。
        'SELECT CAST(SUM(v.byte_size) AS CHAR) bytes FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL',
        [failedDocument.id],
      );
      const quotaBefore = json(
        await get(`/v1/spaces/${outsider.space}/quota`, outsider.owner.headers),
      );
      restoredLimit = quotaBefore.limit_bytes;
      await db.query(
        'UPDATE quota_account SET limit_bytes=used_bytes+reserved_bytes+?,revision=revision+1 WHERE space_id=?',
        [size.bytes, outsider.space],
      );
      const tightQuota = json(
        await get(`/v1/spaces/${outsider.space}/quota`, outsider.owner.headers),
      );
      const failedInput = {
        operation_id: operation('personal-capacity-reject'),
        expected_revision: failedOffer.revision,
        target_knowledge_base_id: outsider.kb,
        expected_quota_revision: tightQuota.revision,
      };
      const prepared = await post(
        `/v1/personal-transfers/${failedOffer.id}/accept`,
        failedInput,
        outsider.owner.headers,
      );
      assert.equal(prepared.status, 202, prepared.body);
      const rejected = await finish(failedInput, outsider.owner);
      assert.equal(rejected.state, 'REJECTED');
      assert.equal(rejected.error.code, 'QUOTA_EXCEEDED');
      const [[kept]] = await db.query(
        'SELECT CAST(responsible_user_id AS CHAR) owner,CAST(space_id AS CHAR) space,lifecycle FROM document WHERE id=?',
        [failedDocument.id],
      );
      assert.deepEqual(kept, { owner: source.owner.id, space: source.space, lifecycle: 'ACTIVE' });
      const refunded = json(
        await get(`/v1/spaces/${outsider.space}/quota`, outsider.owner.headers),
      );
      assert.equal(refunded.reserved_bytes, quotaBefore.reserved_bytes);
      assert.equal(refunded.used_bytes, quotaBefore.used_bytes);
      assert.equal(
        (await get(`/v1/documents/${failedDocument.id}`, source.owner.headers)).status,
        200,
      );

      // 接收空间清理最后一个消费者，原空间 sibling 仍用同一物理图片。
      // 真实 Node CLEANED 后只退接收空间，不能等物理 GC 才退共享的空范围。
      const receiverBeforePurge = json(
        await get(`/v1/spaces/${receiver.space}/quota`, receiver.owner.headers),
      );
      const sourceBeforePurge = json(
        await get(`/v1/spaces/${source.space}/quota`, source.owner.headers),
      );
      const [[paidDocument]] = await db.query(
        'SELECT CAST(bytes AS CHAR) bytes FROM document_capacity WHERE document_id=?',
        [d.id],
      );
      const [[uploadTombstone]] = await db.query(
        'SELECT q.* FROM quota_reservation q JOIN resource r ON r.upload_operation_id=q.operation_id WHERE r.id=?',
        [image],
      );
      const currentD = json(await get(`/v1/documents/${d.id}`, receiver.owner.headers));
      await waitAuthorization(
        await post(
          `/v1/documents/${d.id}`,
          {
            operation_id: operation('transferred-doc-trash'),
            expected_revision: currentD.control_revision,
          },
          receiver.owner.headers,
          'DELETE',
        ),
        receiver.owner,
      );
      const trashPage = json(
        await get(`/v1/spaces/${receiver.space}/trash?limit=100`, receiver.owner.headers),
      );
      const trashedD = trashPage.items.find((item) => item.id === d.id);
      assert.equal(trashedD?.lifecycle, 'TRASHED');
      const purgeInput = {
        operation_id: operation('transferred-doc-purge'),
        expected_control_revision: trashedD.control_revision,
        confirmed: true,
      };
      const purging = await post(`/v1/documents/${d.id}/purge`, purgeInput, receiver.owner.headers);
      assert.equal(purging.status, 202, purging.body);
      assert.equal((await finish(purgeInput, receiver.owner)).state, 'SUCCEEDED');
      const [[realNodeCleaned]] = await nodeDB.query(
        "SELECT state FROM durable_task WHERE kind='PURGE_DOCUMENT_CLEANUP' AND business_key=?",
        [d.id + ':' + purgeInput.operation_id],
      );
      assert.equal(realNodeCleaned?.state, 'SUCCEEDED', '退款要求真实 Node CLEANED 原任务');
      const [[purged]] = await db.query('SELECT lifecycle FROM document WHERE id=?', [d.id]);
      assert.equal(purged.lifecycle, 'PURGED');
      const receiverAfterPurge = json(
        await get(`/v1/spaces/${receiver.space}/quota`, receiver.owner.headers),
      );
      assert.equal(
        BigInt(receiverAfterPurge.used_bytes),
        BigInt(receiverBeforePurge.used_bytes) -
          BigInt(paidDocument.bytes) -
          BigInt(imageBytes.length),
      );
      assert.equal(receiverAfterPurge.reserved_bytes, receiverBeforePurge.reserved_bytes);
      const sourceAfterPurge = json(
        await get(`/v1/spaces/${source.space}/quota`, source.owner.headers),
      );
      assert.equal(sourceAfterPurge.used_bytes, sourceBeforePurge.used_bytes);
      const [afterPurgeAllocations] = await db.query(
        'SELECT CAST(space_id AS CHAR) space,state FROM resource_space_allocation WHERE resource_id=? ORDER BY space_id',
        [image],
      );
      assert.deepEqual(afterPurgeAllocations, [
        { space: source.space, state: 'ACTIVE' },
        { space: receiver.space, state: 'RELEASED' },
      ]);
      const [[releasedProof]] = await db.query(
        'SELECT state FROM document_resource_allocation WHERE resource_id=? AND document_id=? AND space_id=?',
        [image, d.id, receiver.space],
      );
      assert.equal(releasedProof.state, 'RELEASED');
      const [[keptImage]] = await db.query('SELECT state FROM resource WHERE id=?', [image]);
      assert.equal(keptImage.state, 'READY');
      const [[keptUploadTombstone]] = await db.query(
        'SELECT q.* FROM quota_reservation q JOIN resource r ON r.upload_operation_id=q.operation_id WHERE r.id=?',
        [image],
      );
      assert.deepEqual(keptUploadTombstone, uploadTombstone);
      const stillShared = await binaryResourceGet(
        `/v1/resources/${image}/content?document_id=${sibling.id}&context_kind=MAIN&context_id=${siblingView.manifest_id}`,
        source.owner.headers,
      );
      assert.equal(stillShared.status, 200);
      assert.deepEqual(stillShared.body, imageBytes);
      // 原永久清理重送不能重复退款。
      const purgeReplay = await post(
        `/v1/documents/${d.id}/purge`,
        purgeInput,
        receiver.owner.headers,
      );
      assert.equal(purgeReplay.status, 202, purgeReplay.body);
      assert.equal(json(purgeReplay).state, 'SUCCEEDED');
      assert.equal(
        json(await get(`/v1/spaces/${receiver.space}/quota`, receiver.owner.headers)).used_bytes,
        receiverAfterPurge.used_bytes,
      );

      // 已释放的目标分配不能被下一次转入当作已付费；最后一个原空间消费者
      // 转入时必须重新真实收图片与完整正文。浏览器丢失202后重送原接受输入。
      const browserOffer = await propose(source, sibling, receiver.owner);
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 900 },
      });
      await page.goto(origin);
      await page.getByLabel('登录邮箱', { exact: true }).fill(receiver.owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(receiver.owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
      await page.getByRole('button', { name: '个人转让', exact: true }).click();
      assert.equal(new URL(page.url()).pathname, '/transfers');
      await page.reload();
      await page.getByLabel('接收知识库', { exact: true }).selectOption(receiver.kb);
      const row = page
        .getByRole('list', { name: '转让申请列表' })
        .getByRole('listitem')
        .filter({ hasText: `文档 ${sibling.id} ·` });
      await row.getByRole('button', { name: '接受文档转入', exact: true }).click();
      const confirm = row.getByRole('button', { name: '确认接受转让', exact: true });
      await expect(confirm).toBeDisabled();
      await row
        .getByRole('checkbox', { name: '我确认接收这篇文档并承担对应存储容量', exact: true })
        .check();
      const originals = [];
      let loseReply = true;
      await page.route(`**/v1/personal-transfers/${browserOffer.id}/accept`, async (route) => {
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
      await expect(
        page.getByText('文档已转入你的个人知识库，旧访问已关闭', { exact: true }),
      ).toBeVisible({ timeout: 20000 });
      assert.equal(originals.length, 2);
      assert.deepEqual(originals[0], originals[1]);
      const [[movedSibling]] = await db.query(
        'SELECT CAST(space_id AS CHAR) space FROM document WHERE id=?',
        [sibling.id],
      );
      assert.equal(movedSibling.space, receiver.space);
      const [finalAllocations] = await db.query(
        'SELECT CAST(space_id AS CHAR) space,state FROM resource_space_allocation WHERE resource_id=? ORDER BY space_id',
        [image],
      );
      assert.deepEqual(
        finalAllocations.filter((a) => a.state === 'ACTIVE').map((a) => a.space),
        [receiver.space],
      );
      const [[siblingCapacity]] = await db.query(
        'SELECT CAST(bytes AS CHAR) bytes FROM document_capacity WHERE document_id=?',
        [sibling.id],
      );
      const receiverAfterSibling = json(
        await get(`/v1/spaces/${receiver.space}/quota`, receiver.owner.headers),
      );
      assert.equal(
        BigInt(receiverAfterSibling.used_bytes),
        BigInt(receiverAfterPurge.used_bytes) +
          BigInt(siblingCapacity.bytes) +
          BigInt(imageBytes.length),
      );
      await page.reload();
      await expect(
        page
          .getByRole('list', { name: '转让申请列表' })
          .getByRole('listitem')
          .filter({ hasText: `文档 ${sibling.id} ·` }),
      ).toContainText('已接受');
      // 截图前沿页面本身的选择控件等待目标读取完成；不把5秒默认截图断言
      // 当成业务保存/恢复期限，也不拍下空选项的加载中页面。
      await page.getByLabel('接收知识库', { exact: true }).selectOption(receiver.kb);
      await expect(page.getByRole('button', { name: '刷新接收容量', exact: true })).toBeEnabled();
      await page.screenshot({ path: resolve(root, 'artifacts/personal-transfer-browser.png') });
    } finally {
      main?.close();
      siblingMain?.close();
      failedMain?.close();
      await browser?.close();
      if (restoredLimit)
        await db.query(
          'UPDATE quota_account SET limit_bytes=?,revision=revision+1 WHERE space_id=?',
          [restoredLimit, outsider.space],
        );
      await Promise.all([db.end(), nodeDB.end()]);
    }
  },
);
