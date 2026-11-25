import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime, state } from '../../scripts/runtime-lib.mjs';
import { request } from '../helpers/https.mjs';

// 此轮只增加正文副本边界，生产入口仍顺序核验原Node→资源→历史回执。
// 前轮真实历史环/图片保护矩阵不重复；原身份冻结与Node撤权场景仍共用。
export async function verifyAccountPayloadCleanup(doc, fixture) {
  const operation = fixture.operation + '-payload';
  const unknown = operation + '-unknown';
  const other = operation + '-surviving';
  const tls = {
    key: readFileSync(resolve(runtime, 'pki/account-key.pem')),
    cert: readFileSync(resolve(runtime, 'pki/account.pem')),
  };
  const input = { operation_id: fixture.operation, document_id: fixture.document };
  const call = (value = input, options = {}) =>
    request(state().ports.documentHttp, '/infra/lifecycle/document-clean-payloads', {
      method: 'POST',
      ...tls,
      ...options,
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(value),
    });
  const body = (document, text) => ({
    context: { target: { document_id: document } },
    private_body: { text },
  });
  const task = (kind, key, op, value, state = 'SUCCEEDED') =>
    doc.query(
      'INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR))',
      [kind, key, op, JSON.stringify(value), state],
    );
  try {
    await task(
      'MAIN_READ_CONTEXT',
      operation,
      operation,
      body(fixture.document, '待清理的主线标题与正文'),
    );
    // 子固定输入不带文档号，必须通过原operation关联；不能按数值id/字符串前缀猜。
    await task('MERGE_ATTEMPT_FIXED', operation + '-child', operation, {
      private_body: { text: '独立固定输入的正文副本' },
    });
    await task(
      'MAIN_READ_CONTEXT',
      unknown,
      unknown,
      body(fixture.document, '未知结果的原字节'),
      'UNKNOWN',
    );
    const survivingBody = body(fixture.surviving, '存续组织的正文副本');
    await task('MAIN_READ_CONTEXT', other, other, survivingBody);
    assert.equal((await call(input, { cert: undefined, key: undefined })).status, 403);
    const blocked = await call();
    assert.equal(blocked.status, 409, blocked.body);
    assert.equal(JSON.parse(blocked.body).code, 'ACCOUNT_PAYLOAD_RESULT_UNKNOWN');
    const [[preserved]] = await doc.query(
      "SELECT payload FROM durable_task WHERE kind='MAIN_READ_CONTEXT' AND business_key=?",
      [operation],
    );
    assert.deepEqual(
      preserved.payload.private_body,
      { text: '待清理的主线标题与正文' },
      '未知结果时不部分改写正文副本',
    );
    await doc.query('DELETE FROM durable_task WHERE operation_id=?', [unknown]);
    const response = await call();
    assert.equal(response.status, 200, response.body);
    const result = JSON.parse(response.body);
    assert.equal(result.state, 'PAYLOADS_REMOVED');
    assert.deepEqual(JSON.parse((await call()).body), result, '原号重放');
    assert.notEqual(
      (await call({ ...input, operation_id: fixture.operation + '-different' })).status,
      200,
    );
    const [cleaned] = await doc.query(
      'SELECT payload,state FROM durable_task WHERE operation_id=? ORDER BY id',
      [operation],
    );
    assert.equal(cleaned.length, 2);
    for (const row of cleaned) {
      assert.equal(row.state, 'SUCCEEDED', '原任务真实终态必须保留');
      assert.equal(row.payload.body_removed, true);
      assert.equal(row.payload.private_body, undefined);
      assert.equal(row.payload.context.target.document_id, fixture.document);
      assert.equal(row.payload.cleanup_context.operation_id, fixture.operation);
    }
    const [[surviving]] = await doc.query(
      "SELECT payload FROM durable_task WHERE kind='MAIN_READ_CONTEXT' AND business_key=?",
      [other],
    );
    assert.deepEqual(surviving.payload, survivingBody);
    const [[original]] = await doc.query(
      'SELECT state FROM lifecycle_operation WHERE operation_id=?',
      [fixture.operation],
    );
    assert.equal(original.state, 'CLEANING');
    console.log(
      'DOC_PAYLOAD_CLEANUP_ACTUAL_PASS: original Node/resource/history chain; unknown retained atomically; exact operation children scrubbed; terminal headers/replay preserved; surviving payload unchanged',
    );
  } finally {
    await doc.query('DELETE FROM durable_task WHERE operation_id IN (?,?,?)', [
      operation,
      unknown,
      other,
    ]);
  }
}
