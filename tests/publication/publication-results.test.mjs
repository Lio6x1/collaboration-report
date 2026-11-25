import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';

async function originalQuery(db, id) {
  // 从MySQL直接读取十进制文本，避免把原uint64目标先经过JSON.parse的Number再送回RPC。
  const fields = ['document_id', 'unit_id', 'branch_id', 'structure_id', 'workspace_epoch'];
  const columns = fields.map(
    (field) =>
      `COALESCE(JSON_UNQUOTE(JSON_EXTRACT(payload,'$.main.context.target.${field}')),'0') ${field}`,
  );
  const [[row]] = await db.query(
    `SELECT ${columns.join(',')},JSON_UNQUOTE(JSON_EXTRACT(payload,'$.main.context.target.purpose')) purpose,JSON_UNQUOTE(JSON_EXTRACT(payload,'$.main.context.request_fingerprint')) fingerprint FROM durable_task WHERE kind='PUBLICATION_CONTEXT' AND business_key=?`,
    [id],
  );
  assert.ok(row, 'ORIGINAL_PUBLICATION_CONTEXT_REQUIRED');
  return {
    operation_id: id,
    target: Object.fromEntries([...fields, 'purpose'].map((field) => [field, row[field]])),
    request_fingerprint: Buffer.from(row.fingerprint, 'base64'),
  };
}

async function assertFixedResult(db, peer, query, expectedEntity) {
  const value = await rpcCall(peer, 'GetOperationResult', query);
  const [[receipt]] = await db.query(
    'SELECT CAST(generation AS CHAR) generation,CAST(result_json AS CHAR) result_json FROM operation_receipt WHERE operation_id=?',
    [query.operation_id],
  );
  // 比较固定字节与实际SQL摘要；断言只输出布尔，不把摘要或私有上下文打印到TAP。
  assert.equal(value.state, 'SUCCEEDED');
  assert.equal(value.operation_id, query.operation_id);
  assert.equal(value.generation, String(receipt.generation));
  assert.equal(value.terminal_revision, value.generation);
  const bytes = Buffer.from(receipt.result_json);
  assert.ok(value.fixed_result_json_utf8.equals(bytes), 'ORIGINAL_SQL_RESULT_BYTES_REQUIRED');
  assert.ok(
    value.result_digest.equals(createHash('sha256').update(bytes).digest()),
    'ORIGINAL_SQL_RESULT_DIGEST_REQUIRED',
  );
  assert.deepEqual(value.result_ids, expectedEntity ? [expectedEntity] : []);
  const again = await rpcCall(peer, 'GetOperationResult', query);
  assert.ok(again.fixed_result_json_utf8.equals(value.fixed_result_json_utf8));
  assert.ok(again.result_digest.equals(value.result_digest));
  return value;
}

test(
  'private publication results bind original target and fingerprint, and survive actual user logout',
  { timeout: 90000 },
  async () => {
    const f = await organizationFixture(0),
      d = await documentFixture(f),
      db = await reviewDatabase(),
      peer = documentRPC();
    let originalFingerprint, firstInput;
    try {
      firstInput = await releaseInput(d.id, f.owner);
      const first = await publishRelease(d.id, f.owner, firstInput);
      const query = await originalQuery(db, firstInput.operation_id);
      await assertFixedResult(db, peer, query, first.result.entity_id);
      for (const wrong of [
        { ...query, request_fingerprint: Buffer.alloc(32) },
        { ...query, target: { ...query.target, document_id: '18446744073709551615' } },
        { ...query, target: { ...query.target, purpose: 'READ_MAIN' } },
      ])
        await assert.rejects(
          rpcCall(peer, 'GetOperationResult', wrong),
          (error) => error.code === 6 && error.details === 'OPERATION_ID_CONFLICT',
        );
      const [[receipt]] = await db.query(
        'SELECT request_fingerprint FROM operation_receipt WHERE operation_id=?',
        [query.operation_id],
      );
      originalFingerprint = receipt.request_fingerprint;
      await db.query('UPDATE operation_receipt SET request_fingerprint=? WHERE operation_id=?', [
        Buffer.alloc(32),
        query.operation_id,
      ]);
      await assert.rejects(
        rpcCall(peer, 'GetOperationResult', query),
        (error) => error.code === 14 && error.details === 'PUBLICATION_BINDING_UNKNOWN',
      );
      await db.query('UPDATE operation_receipt SET request_fingerprint=? WHERE operation_id=?', [
        originalFingerprint,
        query.operation_id,
      ]);
      originalFingerprint = undefined;
      await publishRelease(d.id, f.owner, await releaseInput(d.id, f.owner));
      for (const action of ['SWITCH', 'UNPUBLISH']) {
        const id = operation('result-' + action.toLowerCase());
        const current = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
        const input = {
          operation_id: id,
          expected_publication_revision: current.publication_epoch,
          action,
          ...(action === 'SWITCH' ? { release_id: first.result.entity_id } : {}),
        };
        const response = await post(`/v1/documents/${d.id}/publication`, input, f.owner.headers);
        assert.equal(response.status, 202, json(response).code);
        assert.equal((await pollPublication(id, f.owner)).state, 'SUCCEEDED');
        await assertFixedResult(
          db,
          peer,
          await originalQuery(db, id),
          action === 'SWITCH' ? first.result.entity_id : undefined,
        );
      }
      const logout = await post(
        '/v1/account/logout',
        { operation_id: operation('publication-result-logout') },
        f.owner.headers,
      );
      assert.equal(logout.status, 202, json(logout).code);
      assert.equal(
        (await get(`/v1/operations/${query.operation_id}`, f.owner.headers)).status,
        401,
      );
      await assertFixedResult(db, peer, query, first.result.entity_id);
    } finally {
      if (originalFingerprint)
        await db.query('UPDATE operation_receipt SET request_fingerprint=? WHERE operation_id=?', [
          originalFingerprint,
          firstInput.operation_id,
        ]);
      peer.close();
      await db.end();
    }
  },
);
