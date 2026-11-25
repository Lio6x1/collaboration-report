import assert from 'node:assert/strict';
import test from 'node:test';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { waitOperation } from '../helpers/account.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';
import {
  searchDatabase,
  waitSearchReceipt,
  searchProjection,
  searchConfiguration,
} from '../helpers/search.mjs';
import { until } from '../helpers/authorization-fence.mjs';
import {
  searchPageFixture,
  searchPagePath,
  checkedSearchPage,
  assertPageIDs,
  pageVisibility,
  waitKBVisibilityConsumption,
  reviewDatabase,
  outputLeaseEvidence,
} from '../helpers/search-pages.mjs';

async function rejected(path, headers, status, code) {
  const response = await get(path, headers);
  assert.equal(response.status, status, json(response).code);
  const error = json(response);
  if (code) assert.equal(error.code, code);
  assert.equal(Object.hasOwn(error, 'items'), false);
  assert.equal(Object.hasOwn(error, 'next_cursor'), false);
}

async function walkOldCursor(f, user, cursor, firstID) {
  const items = [];
  const seen = new Set([firstID]);
  // 上限来自本例确切七篇夹具，只防测试在坏cursor上循环；不限制产品候选扫描。
  for (let pageNumber = 0; pageNumber < f.docs.length; pageNumber++) {
    const page = await checkedSearchPage(f, user, { limit: 1, cursor });
    for (const item of page.items) {
      assert.ok(!seen.has(item.document_id), 'OLD_CURSOR_DUPLICATED_ALREADY_RETURNED_DOCUMENT');
      seen.add(item.document_id);
      items.push(item);
    }
    if (!page.has_more) {
      assert.equal(page.next_cursor, null);
      return items;
    }
    assert.equal(page.items.length, 1, 'OLD_CURSOR_MUST_NOT_RETURN_A_PARTIAL_PAGE');
    cursor = page.next_cursor;
  }
  assert.fail('SEVEN_DOCUMENT_CURSOR_DID_NOT_REACH_EOF');
}

function assertRemainingIDs(items, documents) {
  assert.deepEqual(
    items.map((item) => item.document_id).sort(),
    documents.map((doc) => doc.id).sort(),
  );
}

async function waitNewReleaseQueryable(document, physical) {
  const { search } = searchConfiguration();
  // realtime _doc已更新不等于自然refresh完成。只在建立fresh PIT之前读精确新版本，
  // 旧cursor遍历已结束；不主动refresh、不续期，也不重试任何产品页失败。
  await until(
    async () => {
      const response = await fetch(`${search.elasticEndpoint}/${physical}/_count`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({
          query: {
            bool: {
              filter: [
                { term: { visible: true } },
                { term: { document_id: document.id } },
                { term: { release_id: document.release } },
              ],
            },
          },
        }),
        signal: AbortSignal.timeout(3000),
      });
      assert.equal(response.status, 200, 'NEW_RELEASE_SEARCHABILITY_READ_FAILED');
      const reader = response.body.getReader();
      const chunks = [];
      let bytes = 0;
      try {
        while (true) {
          const part = await reader.read();
          if (part.done) break;
          bytes += part.value.byteLength;
          if (bytes > 4096) {
            await reader.cancel();
            assert.fail('NEW_RELEASE_SEARCHABILITY_RESPONSE_LIMIT');
          }
          chunks.push(Buffer.from(part.value));
        }
      } finally {
        reader.releaseLock();
      }
      const value = JSON.parse(Buffer.concat(chunks, bytes).toString('utf8'));
      assert.equal(value._shards.failed, 0);
      assert.ok(Number.isInteger(value._shards.total) && value._shards.total > 0);
      assert.equal(value._shards.successful, value._shards.total);
      assert.ok(value.count === 0 || value.count === 1);
      return value.count === 1;
    },
    'NEW_RELEASE_NATURAL_REFRESH_NOT_OBSERVED',
    3000,
  );
}

// 七篇均由产品创建、真实MAIN保存和发布进入原Canal消费者。共享夹具防止
// 每个权限反例额外留下整套房间；没有SQL/ES造数，也不清理已有产品数据。
test('实际搜索页：固定正式摘要、当前受众、终批后缀与加密绑定', { timeout: 240000 }, async (t) => {
  const f = await searchPageFixture();
  const member = f.members[0];
  const publicDocs = f.docs.slice(0, 4);
  const db = await reviewDatabase();
  try {
    await t.test('匿名limit1遍历终批不吞lookahead；真实Go/Edge包含返回项及lookahead', async () => {
      const [[start]] = await db.query(
        'SELECT CAST(COALESCE(MAX(id),0) AS CHAR) id FROM durable_task',
      );
      const first = await checkedSearchPage(f, null, { limit: 1 });
      assert.equal(first.items.length, 1);
      assert.equal(first.has_more, true);
      // 先冻结本次登记的读证据，后续重放会产生自己的合法新租约。
      const [leaseRows] = await db.query(
        "SELECT CAST(MAX(id) AS CHAR) id FROM durable_task WHERE kind='OUTPUT_LEASE' AND id>?",
        [start.id],
      );
      assert.ok(leaseRows[0].id);
      const second = await checkedSearchPage(f, null, { limit: 1, cursor: first.next_cursor });
      assert.equal(second.items.length, 1);
      const returned = f.docs.find((d) => d.id === first.items[0].document_id);
      const lookahead = f.docs.find((d) => d.id === second.items[0].document_id);
      // 同时固定上下界，不能把后续重放的租约拼成首请求的联合来源。
      const sources = await outputLeaseEvidence(db, start.id, leaseRows[0].id, returned, lookahead);
      t.diagnostic(
        `实际四服务产品链：七项发布/原消费已确认；本页Go/Edge各一条租约，联合来源${sources}项（不代表在途ACK）`,
      );
      const replay = await checkedSearchPage(f, null, { limit: 1, cursor: first.next_cursor });
      assert.deepEqual(replay.items, second.items);
      assert.equal(replay.has_more, second.has_more);
      // AEAD每次随机nonce，重试比较业务项和边界，不要求密文完全相同。
      const ids = [...first.items, ...second.items].map((item) => item.document_id);
      let page = second;
      for (let n = 0; page.has_more && n < 7; n++) {
        page = await checkedSearchPage(f, null, { limit: 1, cursor: page.next_cursor });
        assert.equal(page.items.length, 1);
        ids.push(page.items[0].document_id);
      }
      assert.equal(page.has_more, false);
      assert.equal(page.next_cursor, null);
      assert.equal(new Set(ids).size, ids.length);
      assert.deepEqual(ids.sort(), publicDocs.map((d) => d.id).sort());
      assert.equal([...f.docs[0].snippet].length, 2000);
    });

    await t.test(
      'PUBLIC匿名与外人；SPACE真实成员；COLLABORATORS真实内部READ；KB交集及空间范围',
      async () => {
        assertPageIDs(await checkedSearchPage(f, null), publicDocs);
        assertPageIDs(await checkedSearchPage(f, f.outsider), publicDocs);
        assertPageIDs(await checkedSearchPage(f, member), f.docs);
        assertPageIDs(await checkedSearchPage(f, f.owner), f.docs);
        assert.equal((await get(`/v1/documents/${f.docs[5].id}/main`, member.headers)).status, 200);
        assert.equal((await get(`/v1/documents/${f.docs[4].id}/main`, member.headers)).status, 403);
        assertPageIDs(await checkedSearchPage(f, null, { space: f.space }), publicDocs.slice(0, 3));
        assertPageIDs(
          await checkedSearchPage(f, member, { space: f.space }),
          f.docs.filter((d) => d.space === f.space),
        );
        assertPageIDs(await checkedSearchPage(f, f.owner, { space: f.personal }), [f.docs[3]]);
        // space_id只缩小查询范围；PUBLIC正式版不因读者不是该空间成员而隐藏。
        assertPageIDs(await checkedSearchPage(f, f.outsider, { space: f.personal }), [f.docs[3]]);
        assertPageIDs(await checkedSearchPage(f, null, { space: f.personal }), [f.docs[3]]);
      },
    );

    await t.test('同原链q/limit/空间/会话改变明确失效；密文字节篡改不泄漏半页', async () => {
      const anonymous = await checkedSearchPage(f, null, { limit: 1 });
      const cursor = anonymous.next_cursor;
      await rejected(
        searchPagePath(f.q + 'changed', { limit: 1, cursor }),
        {},
        409,
        'CURSOR_STALE',
      );
      await rejected(searchPagePath(f.q, { limit: 2, cursor }), {}, 409, 'CURSOR_STALE');
      await rejected(
        searchPagePath(f.q, { limit: 1, cursor, space: f.space }),
        {},
        409,
        'CURSOR_STALE',
      );
      await rejected(
        searchPagePath(f.q, { limit: 1, cursor }),
        member.headers,
        409,
        'CURSOR_STALE',
      );
      const corrupted = Buffer.from(cursor, 'base64url');
      corrupted[20] ^= 1;
      await rejected(
        searchPagePath(f.q, { limit: 1, cursor: corrupted.toString('base64url') }),
        {},
        400,
        'CURSOR_INVALID',
      );
      const logged = await checkedSearchPage(f, member, { limit: 1 });
      const login = await post('/v1/account/login', {
        email: member.email,
        password: member.password,
      });
      assert.equal(login.status, 200, json(login).code);
      const newHeaders = {
        cookie: login.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
        'x-csrf-token': json(login).csrf_token,
      };
      await rejected(
        searchPagePath(f.q, { limit: 1, cursor: logged.next_cursor }),
        newHeaders,
        409,
        'CURSOR_STALE',
      );
    });

    await t.test('坏、重复、空Cookie以及真实注销的原Cookie都不能降级匿名', async () => {
      for (const cookie of [
        '__Host-collabSession=not-valid',
        f.outsider.headers.cookie + '; ' + f.outsider.headers.cookie,
        '__Host-collabSession=',
      ])
        await rejected(searchPagePath(f.q), { cookie }, 401);
      await waitOperation(
        await post(
          '/v1/account/logout',
          { operation_id: operation('search-page-logout') },
          f.outsider.headers,
        ),
        f.outsider,
      );
      await rejected(searchPagePath(f.q), f.outsider.headers, 401);
      assertPageIDs(await checkedSearchPage(f, null), publicDocs);
    });

    await t.test(
      '真实正式切版后旧PIT排除旧release/seq，剩余项无漏重；fresh查询返回新正式版',
      async () => {
        const first = await checkedSearchPage(f, member, { limit: 1 });
        assert.equal(first.items.length, 1);
        assert.equal(first.has_more, true);
        const firstID = first.items[0].document_id;
        // 不发布已经写入私有S2的docs[0]；两篇正常PUBLIC中总有一篇尚未返回。
        const target = [f.docs[1], f.docs[2]].find((doc) => doc.id !== firstID);
        assert.ok(target, 'UNRETURNED_FORMAL_RELEASE_TARGET_REQUIRED');
        const originalRelease = target.release;
        const input = await releaseInput(target.id, f.owner);
        const published = await publishRelease(target.id, f.owner, input);
        assert.notEqual(published.result.entity_id, originalRelease);
        const search = await searchDatabase();
        let physical;
        try {
          const receipt = await waitSearchReceipt(search, input.operation_id);
          const projection = await searchProjection(target.id, receipt.physical);
          assert.equal(projection.visible, true);
          assert.equal(projection.release_id, published.result.entity_id);
          assert.equal(projection.sequence, receipt.sequence);
          assert.equal(projection.title, target.title);
          physical = receipt.physical;
        } finally {
          await search.end();
        }
        target.release = published.result.entity_id;
        const suffix = await walkOldCursor(f, member, first.next_cursor, firstID);
        assertRemainingIDs(
          suffix,
          f.docs.filter((doc) => doc.id !== firstID && doc.id !== target.id),
        );
        await waitNewReleaseQueryable(target, physical);
        const fresh = await checkedSearchPage(f, member);
        assertPageIDs(fresh, f.docs);
        assert.equal(
          fresh.items.find((item) => item.document_id === target.id).release_id,
          target.release,
        );
      },
    );

    await t.test(
      'KB收紧后旧cursor排除lookahead/后缀中的当前拒绝项；Doc PUBLIC仍受KB交集约束',
      async () => {
        const first = await checkedSearchPage(f, member, { limit: 1 });
        assert.equal(first.items.length, 1);
        assert.equal(first.has_more, true);
        const firstID = first.items[0].document_id;
        const event = await pageVisibility(
          `/v1/knowledge-bases/${f.restrictedKB}`,
          f.owner,
          'COLLABORATORS',
        );
        await waitKBVisibilityConsumption(f, event);
        const suffix = await walkOldCursor(f, member, first.next_cursor, firstID);
        // 首页若已经返回KB文档，它不再属于待消费后缀；不能把已合法交付的项算成漏项。
        assertRemainingIDs(
          suffix,
          f.docs.slice(0, 6).filter((doc) => doc.id !== firstID),
        );
        assertPageIDs(await checkedSearchPage(f, member), f.docs.slice(0, 6));
        assertPageIDs(await checkedSearchPage(f, f.owner), f.docs);
        assertPageIDs(await checkedSearchPage(f, null), publicDocs);
      },
    );
  } finally {
    await db.end();
  }
});
