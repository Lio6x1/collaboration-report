import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync, rmSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { runtime, state, pause } from '../../scripts/runtime-lib.mjs';
import { request } from '../helpers/https.mjs';
import { resourcePNG, binaryResourceGet } from '../helpers/resource.mjs';
import { searchDatabase, waitSearchReceipt, searchProjection } from '../helpers/search.mjs';

// 复用原冻结/排空图，只增加最终留存关口和真实正式阅读/图片/消费/搜索。
// 图由私有SQL建立，图片字节为本测试自己的文件，不冒充上传或真实WSS发布。
export async function verifyAccountRetentionComplete(doc, fixture) {
  const tls = {
    key: readFileSync(resolve(runtime, 'pki/account-key.pem')),
    cert: readFileSync(resolve(runtime, 'pki/account.pem')),
  };
  const [[original]] = await doc.query(
    'SELECT fixed_plan FROM lifecycle_operation WHERE operation_id=?',
    [fixture.operation],
  );
  const plan = original.fixed_plan;
  const control = (path, body, options = {}) =>
    request(state().ports.documentHttp, '/infra/lifecycle/' + path, {
      method: 'POST',
      ...tls,
      ...options,
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(body),
    });
  const complete = (binding = plan.binding, options = {}) =>
    control('account-close-complete', binding, options);
  const publicPath = `/v1/public/documents/${fixture.document}`;
  assert.equal((await complete(plan.binding, { cert: undefined, key: undefined })).status, 403);
  const pending = await complete();
  assert.equal(pending.status, 409, pending.body);
  assert.equal(JSON.parse(pending.body).code, 'ACCOUNT_DOCUMENT_CLEANUP_PENDING');
  assert.equal((await request(state().ports.edge, publicPath)).status, 404);

  const config = JSON.parse(readFileSync(resolve(runtime, 'services/document.json')));
  const storage = resolve(
    config.resources?.root ?? resolve(config.root, '.runtime/resources/document'),
  );
  assert.equal(resolve(config.root), resolve(runtime, '..'), 'only this project storage');
  const key = createHash('sha256')
    .update(fixture.operation + ':retained-image')
    .digest('hex');
  const file = resolve(storage, key);
  assert.equal(dirname(file), storage);
  const checksum = createHash('sha256').update(resourcePNG).digest();
  const searchDB = await searchDatabase();
  let created = false;
  try {
    writeFileSync(file, resourcePNG, { flag: 'wx' });
    created = true;
    // 原版本只存资源号；测试为自有READY样本补真实文件元数据，不修改不可变正文。
    const [changed] = await doc.query(
      "UPDATE resource SET storage_key=?,byte_size=?,checksum=? WHERE id=? AND state='READY'",
      [key, resourcePNG.length, checksum, fixture.resource],
    );
    assert.equal(changed.affectedRows, 1);
    for (const item of plan.retention.documents) {
      const cleaned = await control('document-clean-payloads', {
        operation_id: fixture.operation,
        document_id: String(item.document_id),
      });
      assert.equal(cleaned.status, 200, cleaned.body);
      assert.equal(JSON.parse(cleaned.body).state, 'PAYLOADS_REMOVED');
    }
    const done = await complete();
    assert.equal(done.status, 200, done.body);
    assert.deepEqual(JSON.parse(done.body), { state: 'SUCCEEDED' });
    const replays = await Promise.all([complete(), complete()]);
    for (const replay of replays) assert.deepEqual(JSON.parse(replay.body), { state: 'SUCCEEDED' });
    assert.notEqual(
      (await complete({ ...plan.binding, user_id: plan.binding.user_id + 1 })).status,
      200,
    );
    const publication = await request(state().ports.edge, publicPath);
    assert.equal(publication.status, 200, publication.body);
    const value = JSON.parse(publication.body);
    assert.equal(value.title, '固定正式标题');
    assert.equal(value.manifest_id, fixture.manifest);
    assert.equal(value.units.length, 1);
    assert.equal(value.units[0].id, fixture.version);
    assert.deepEqual(value.units[0].resource_ids, [fixture.resource]);
    const imagePath =
      `/v1/resources/${fixture.resource}/content?` +
      new URLSearchParams({
        document_id: fixture.document,
        context_kind: 'PUBLIC',
        context_id: value.release_id,
      });
    const image = await binaryResourceGet(imagePath);
    assert.equal(image.status, 200, image.body.toString('utf8'));
    assert.deepEqual(image.body, resourcePNG, '留存图片真实原字节');
    assert.equal(
      (
        await binaryResourceGet(
          imagePath.replace(
            `/resources/${fixture.resource}/`,
            `/resources/${fixture.privateResource}/`,
          ),
        )
      ).status,
      403,
      '私有后续图片不能借正式版读取',
    );
    assert.equal(
      (await request(state().ports.edge, `/v1/documents/${fixture.document}`)).status,
      401,
      '留存不授予内部入口',
    );

    const activation = createHash('sha256')
      .update('ACCOUNT_RETENTION_COMPLETE\n' + fixture.operation)
      .digest('hex');
    const events = [];
    for (const item of plan.retention.documents) {
      const id = String(item.document_id);
      const event =
        'visibility_index_' +
        createHash('sha256').update(`retained-index-${activation}\n${id}`).digest('hex');
      const [[row]] = await doc.query(
        'SELECT lifecycle,CAST(event_seq AS CHAR) seq,CAST(publication_epoch AS CHAR) epoch FROM document WHERE id=?',
        [id],
      );
      assert.equal(row.lifecycle, item.release_id === 0 ? 'PURGED' : 'PUBLIC_RETAINED');
      const [[outbox]] = await doc.query(
        'SELECT CAST(aggregate_seq AS CHAR) seq,payload FROM outbox WHERE event_id=? AND aggregate_id=?',
        [event, id],
      );
      assert.equal(outbox.seq, row.seq);
      assert.equal(outbox.payload.publication_epoch, row.epoch);
      events.push({ id, event, retained: item.release_id !== 0, seq: row.seq });
      if (!item.release_id)
        assert.equal(
          (await request(state().ports.edge, `/v1/public/documents/${id}`)).status,
          404,
          '撤下/受限内容不复活',
        );
    }
    const receipts = await Promise.all(
      events.map((e) => waitSearchReceipt(searchDB, e.event, 5000)),
    );
    for (const [i, receipt] of receipts.entries()) {
      const event = events[i];
      const projection = await searchProjection(event.id, receipt.physical);
      assert.equal(projection.sequence, event.seq);
      assert.equal(projection.visible, event.retained);
    }
    const org = plan.retention.documents.find((d) => String(d.document_id) === fixture.document);
    const query =
      '/v1/search?' +
      new URLSearchParams({ q: '固定正式标题', space_id: String(org.space_id), limit: '20' });
    let page;
    const until = Date.now() + 5000;
    do {
      const response = await request(state().ports.edge, query);
      assert.equal(response.status, 200, response.body);
      page = JSON.parse(response.body);
      if (page.items.some((item) => item.document_id === fixture.document)) break;
      await pause(100);
    } while (Date.now() < until);
    assert.deepEqual(
      page.items.map((item) => item.document_id),
      [fixture.document],
    );
    assert.equal(page.items[0].title, '固定正式标题');
    const [[surviving]] = await doc.query('SELECT lifecycle FROM document WHERE id=?', [
      fixture.surviving,
    ]);
    assert.equal(surviving.lifecycle, 'ACTIVE');
    const [[identity]] = await doc.query('SELECT state FROM access_gate WHERE scope_key=?', [
      `identity:${fixture.user}`,
    ]);
    assert.equal(identity.state, 'BLOCKED');
    console.log(
      'ACCOUNT_PUBLIC_RETENTION_ACTUAL_PASS: all original cleanup receipts; atomic retained/purged states; original replay; fixed release and real image bytes; private images/withdrawn/restricted content refused; real consumer projections and anonymous search; surviving document unchanged; Account still frozen',
    );
  } finally {
    if (created) rmSync(file, { force: true });
    await searchDB.end();
  }
}
