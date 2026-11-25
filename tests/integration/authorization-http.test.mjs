import assert from 'node:assert/strict';
import test from 'node:test';
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc, yDocToProsemirrorJSON } from 'y-prosemirror';
import { schema, toPM, fromPM, canonical } from '../../packages/content/index.mjs';
import { state } from '../../scripts/runtime-lib.mjs';
import { request } from '../helpers/https.mjs';
import { reviewFixture, openReviewWorkspace, reviewDatabase } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { get, json, operation } from '../helpers/workspace.mjs';
import { waitOperation } from '../helpers/account.mjs';
import { until, sessionId, assertClosureFacts } from '../helpers/authorization-fence.mjs';
import { prepareOwnedBlocking } from '../helpers/owned-blocking.mjs';
import { pausedRead } from '../helpers/paused-http.mjs';

test('product HTTP: registered fixed MAIN read waits through actual Edge block and original settlement before ACK', async () => {
  const f = await reviewFixture();
  const database = await reviewDatabase('document');
  const blocker = await reviewDatabase('document');
  const account = await reviewDatabase('account');
  let main, reading, fault;
  try {
    await unlockStructurePreview(f);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    const fragment = main.doc.getXmlFragment('prosemirror');
    for (let index = 0; index < 34; index++) {
      const before = Y.encodeStateVector(main.doc);
      const part = prosemirrorJSONToYDoc(
        schema,
        toPM({
          type: 'doc',
          content: [
            {
              type: 'paragraph',
              attrs: { id: 'http-in-flight-' + index },
              content: [{ type: 'text', text: '在途'.repeat(22000) }],
            },
          ],
        }),
      );
      fragment.insert(
        fragment.length,
        part
          .getXmlFragment('prosemirror')
          .toArray()
          .map((node) => node.clone()),
      );
      part.destroy();
      main.w.send({
        type: 'UPDATE',
        client_update_id: operation('http-body-part'),
        update: Buffer.from(Y.encodeStateAsUpdate(main.doc, before)).toString('base64'),
      });
      assert.equal((await main.w.next('SAVED')).durable_seq, String(index + 1));
    }
    const viewResponse = await get('/v1/documents/' + f.d.id + '/main', f.user.headers);
    assert.equal(viewResponse.status, 200);
    const view = json(viewResponse);
    const [[version]] = await database.query('SELECT byte_size FROM content_version WHERE id=?', [
      view.units[0].content_version_id,
    ]);
    assert.ok(
      Number(version.byte_size) > 4 * 1024 * 1024 && Number(version.byte_size) < 5 * 1024 * 1024,
    );
    const path =
      '/v1/documents/' +
      f.d.id +
      '/read-views/' +
      view.read_view_id +
      '/units/' +
      view.units[0].unit_id;
    const completeResponse = await get(path, f.user.headers);
    assert.equal(completeResponse.status, 200, 'LEGAL_FIXED_BODY_READ_REQUIRED');
    const completeContent = json(completeResponse);
    assert.equal(completeContent.id, view.units[0].content_version_id);
    assert.ok(
      canonical(completeContent.body_json) === canonical(fromPM(yDocToProsemirrorJSON(main.doc))),
      'FULL_CANONICAL_BODY_REQUIRED',
    );
    const scope = 'session:' + sessionId(f.user).toString('hex');
    fault = await prepareOwnedBlocking('edge');
    await blocker.beginTransaction();
    await blocker.query('SELECT state FROM read_view WHERE view_id=? FOR UPDATE', [
      view.read_view_id,
    ]);
    reading = pausedRead(path, f.user);
    assert.equal((await reading.sent).tlsVerified, true);
    // 真实读取在持有本来源锁时等待原 read_view，证明请求已进入业务事务而非仅发出TCP字节。
    await until(async () => {
      try {
        await database.query('SELECT state FROM access_gate WHERE scope_key=? FOR UPDATE NOWAIT', [
          scope,
        ]);
        return false;
      } catch (error) {
        if (error.errno === 3572) return true;
        throw Error('ACTUAL_READ_SOURCE_LOCK_PROBE_FAILED');
      }
    }, 'ACTUAL_HTTP_READ_TRANSACTION_NOT_WAITING').catch(async (error) => {
      // 只记录实际状态和剩余期限，区分未准入/到期/请求未知；不输出身份、正文或驱动异常。
      reading.resume();
      const ended = await reading.completed;
      const [[fixed]] = await database.query(
        'SELECT v.state view_state,a.state admission_state,TIMESTAMPDIFF(MICROSECOND,NOW(6),LEAST(v.expires_at,a.expires_at)) remaining_us FROM read_view v JOIN document_admission a ON a.admission_id=v.admission_id WHERE v.view_id=?',
        [view.read_view_id],
      );
      console.log(
        JSON.stringify({
          stage: 'ACTUAL_HTTP_WAIT_NOT_OBSERVED',
          status: ended.status ?? null,
          reason: ended.reason ?? 'UNKNOWN',
          viewState: fixed.view_state,
          admissionState: fixed.admission_state,
          remainingMs: Math.trunc(Number(fixed.remaining_us) / 1000),
        }),
      );
      throw error;
    });
    await fault.block();
    const revocation = operation('http-in-flight-logout');
    // Edge本身被真实暂停时，使用账号原公开处理器的最小receipt路径；Cookie/Origin/CSRF仍实际核验。
    // 敏感正文读取仍经过真实Edge，不能暂停网关后改用另一个无输出门闩的正文入口。
    const input = JSON.stringify({ operation_id: revocation });
    const response = await request(state().ports.accountHttp, '/v1/account/logout', {
      method: 'POST',
      headers: {
        ...f.user.headers,
        origin: 'https://localhost:' + state().ports.edge,
        'content-type': 'application/json',
      },
      body: input,
    });
    assert.equal(response.status, 202);
    assert.equal(json(response).operation_id, revocation);
    await blocker.rollback();
    let leases;
    await until(async () => {
      [leases] = await database.query(
        "SELECT t.id,CAST(t.payload AS CHAR) raw,d.delivery_id,d.instance_id FROM durable_task t JOIN delivery_session d ON d.delivery_id=t.business_key JOIN document_admission a ON a.admission_id=d.admission_id WHERE t.kind='OUTPUT_LEASE' AND d.document_id=? AND a.purpose='READ_FIXED_UNIT' AND t.instance_id LIKE '%-edge'",
        [f.d.id],
      );
      return leases.length === 2;
    }, 'ACTUAL_HTTP_OUTPUT_NOT_REGISTERED');
    assert.equal(leases.length, 2, 'EXACT_ACTUAL_READ_OUTPUTS_REQUIRED');
    const lease = leases.sort((a, b) => (BigInt(a.id) > BigInt(b.id) ? -1 : 1))[0];
    // Edge在真实业务读取等待时已经暂停；Doc提交原输出后才会尝试同实例open，不能结束该输出或编造ACK。
    await blocker.beginTransaction();
    await blocker.query('SELECT id FROM durable_task WHERE id=? FOR UPDATE', [lease.id]);
    await until(async () => {
      const [[gate]] = await database.query(
        'SELECT state,last_revocation_id FROM access_gate WHERE scope_key=?',
        [scope],
      );
      return gate?.state === 'BLOCKED' && gate.last_revocation_id === revocation;
    }, 'ACTUAL_HTTP_SOURCE_CLOSE_REQUIRED');
    for (const connection of [database, account]) {
      const [[pending]] = await connection.query(
        'SELECT state FROM authorization_change WHERE revocation_id=?',
        [revocation],
      );
      assert.equal(pending?.state, 'FENCING');
    }
    await fault.release();
    fault = null;
    reading.resume();
    const ended = await reading.completed;
    assert.notEqual(ended.failed, true, '实际TLS请求必须已送达并有结束事实');
    assert.ok(ended.status !== 200 || !ended.complete, '在途旧响应不能得到完整敏感正文');
    assert.notEqual(ended.reason, 'TIMEOUT', '客户端预算到期不能冒充实际撤权停止');
    const [[ack]] = await database.query(
      "SELECT COUNT(*) n FROM authorization_ack WHERE revocation_id=? AND participant='edge'",
      [revocation],
    );
    assert.equal(Number(ack.n), 0, '原持久结算被锁住时不能提前记录 Edge ACK');
    const [[pending]] = await account.query(
      'SELECT state FROM authorization_change WHERE revocation_id=?',
      [revocation],
    );
    assert.equal(pending?.state, 'FENCING');
    const [[delivery]] = await database.query(
      'SELECT state FROM delivery_session WHERE delivery_id=?',
      [lease.delivery_id],
    );
    assert.notEqual(delivery.state, 'SETTLED');
    await blocker.rollback();
    await waitOperation(response, f.user);
    await assertClosureFacts(f.user, main, revocation);
    const [[finished]] = await database.query(
      'SELECT state FROM delivery_session WHERE delivery_id=?',
      [lease.delivery_id],
    );
    assert.equal(finished.state, 'SETTLED');
  } finally {
    await fault?.release();
    await reading?.close();
    await blocker.rollback();
    main?.close();
    f.close();
    await Promise.all([database.end(), blocker.end(), account.end()]);
  }
});
