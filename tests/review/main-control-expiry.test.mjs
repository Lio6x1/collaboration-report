import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';

// 用户查询遵守原回执期限；系统收尾仍读取固定终态，不能因为用户查询过期重复安装或阻塞恢复。
test('expired main control receipt closes user polling and preserves the authoritative system result', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    db = await reviewDatabase(),
    peer = documentRPC();
  const id = operation('expired-control');
  let expiry;
  try {
    const current = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
    assert.equal(
      (
        await completedMainControl(
          await post(
            `/v1/documents/${d.id}/main-lock`,
            {
              operation_id: id,
              expected_control_revision: current.control_revision,
              locked: false,
            },
            f.owner.headers,
          ),
          f.owner,
        )
      ).state,
      'SUCCEEDED',
    );
    const [[row]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='MAIN_CONTROL_CONTEXT' AND business_key=?",
      [id],
    );
    const fixed = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
    const query = {
      operation_id: id,
      target: fixed.context.target,
      request_fingerprint: fixed.context.request_fingerprint,
    };
    const before = await rpcCall(peer, 'GetOperationResult', query);
    assert.equal(before.state, 'SUCCEEDED');
    const [[receipt]] = await db.query(
      'SELECT expires_at FROM operation_receipt WHERE operation_id=?',
      [id],
    );
    expiry = receipt.expires_at;
    await db.query(
      'UPDATE operation_receipt SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE operation_id=?',
      [id],
    );
    const expired = await get(`/v1/operations/${id}`, f.owner.headers);
    assert.equal(expired.status, 404, json(expired).code);
    assert.equal(json(expired).code, 'OPERATION_EXPIRED');
    assert.deepEqual(await rpcCall(peer, 'GetOperationResult', query), before);
  } finally {
    if (expiry)
      await db.query('UPDATE operation_receipt SET expires_at=? WHERE operation_id=?', [
        expiry,
        id,
      ]);
    peer.close();
    await db.end();
  }
});
