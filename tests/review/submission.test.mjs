import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraft, flushDraft, reviewDatabase } from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import { schema, toPM } from '../../packages/content/index.mjs';

async function terminal(id, headers) {
  for (let n = 0; n < 100; n++) {
    const r = await get('/v1/operations/' + id, headers);
    assert.equal(r.status, 200);
    const value = json(r);
    if (value.state !== 'PREPARING') return value;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  assert.fail('SUBMISSION_NOT_TERMINAL');
}

test('actual invalid flush produces a definite rejection and leaves no round', async () => {
  const f = await reviewFixture();
  try {
    const id = operation('invalid-flush');
    const response = await post(
      `/v1/documents/${f.d.id}/units/${f.grant.context.target.unit_id}/submissions`,
      {
        operation_id: id,
        mode: 'NEW',
        branch_id: f.session.branch_id,
        expected_structure_id: f.grant.context.target.structure_id,
        flush_receipt_id: operation('missing-flush'),
      },
      f.f.owner.headers,
    );
    assert.equal(response.status, 202);
    const result = await terminal(id, f.f.owner.headers);
    assert.equal(result.state, 'REJECTED');
    assert.equal(result.error.code, 'SUBMISSION_FLUSH_INVALID');
  } finally {
    f.close();
  }
});

// 实际 HTTP 入口只接受可靠 FLUSH 编号；正文来自 Node，不接受浏览器自己提交的 JSON。
test('actual HTTP submission creates one fixed round and replays the original result', async () => {
  const f = await reviewFixture();
  try {
    await writeDraft(f, 'HTTP 送审第一份正文');
    const flush = await flushDraft(f),
      input = {
        operation_id: operation('submission'),
        mode: 'NEW',
        branch_id: f.session.branch_id,
        expected_structure_id: f.grant.context.target.structure_id,
        flush_receipt_id: flush.receipt_id,
      },
      path = `/v1/documents/${f.d.id}/units/${f.grant.context.target.unit_id}/submissions`;
    const response = await post(path, input, f.f.owner.headers);
    assert.equal(response.status, 202, json(response).code);
    assert.equal(json(response).poll_url, '/v1/operations/' + input.operation_id);
    const result = await terminal(input.operation_id, f.f.owner.headers);
    assert.equal(result.state, 'SUCCEEDED');
    assert.ok(
      result.result.request_id && result.result.round_id && result.result.content_version_id,
    );
    const database = await reviewDatabase();
    try {
      const [[fixed]] = await database.query(
        'SELECT canonical_body FROM content_version WHERE id=?',
        [result.result.content_version_id],
      );
      assert.match(fixed.canonical_body.toString('utf8'), /HTTP 送审第一份正文/);
      const [[counts]] = await database.query(
        'SELECT (SELECT COUNT(*) FROM review_round WHERE submission_operation_id=?) AS rounds,(SELECT COUNT(*) FROM request_submitter WHERE request_id=?) AS authors',
        [input.operation_id, result.result.request_id],
      );
      assert.equal(Number(counts.rounds), 1);
      assert.equal(Number(counts.authors), 1);
      const [[task]] = await database.query(
        "SELECT payload FROM durable_task WHERE kind='SUBMISSION_RECOVER' AND business_key=?",
        [input.operation_id],
      );
      const context = (typeof task.payload === 'string' ? JSON.parse(task.payload) : task.payload)
        .capture.context;
      const peer = documentRPC();
      try {
        const query = {
          operation_id: input.operation_id,
          target: { ...f.grant.context.target, purpose: 'SUBMISSION' },
          request_fingerprint: Buffer.from(context.request_fingerprint, 'base64'),
        };
        const authority = await rpcCall(peer, 'GetOperationResult', query);
        assert.equal(authority.state, 'SUCCEEDED');
        assert.deepEqual(
          JSON.parse(Buffer.from(authority.fixed_result_json_utf8).toString('utf8')).result,
          result.result,
        );
        await assert.rejects(
          () =>
            rpcCall(peer, 'GetOperationResult', {
              ...query,
              target: { ...query.target, unit_id: (BigInt(query.target.unit_id) + 1n).toString() },
            }),
          (e) => e.code === 6,
        );
      } finally {
        peer.close();
      }
    } finally {
      await database.end();
    }
    await writeDraft(f, 'HTTP 送审之后的新草稿');
    const replay = await post(path, input, f.f.owner.headers);
    assert.equal(replay.status, 202);
    assert.deepEqual(json(replay).result, result.result);
    assert.equal(
      (
        await post(
          path,
          { ...input, flush_receipt_id: operation('other-flush') },
          f.f.owner.headers,
        )
      ).status,
      409,
    );
    assert.equal(
      (await post(path, { ...input, operation_id: operation('second-submit') }, f.f.owner.headers))
        .status,
      409,
    );
  } finally {
    f.close();
  }
});

test('actual fixed submission transfers more than the default 4 MiB gRPC limit', async () => {
  const f = await reviewFixture();
  try {
    const chunk = '正文'.repeat(1 << 16);
    await writeDraft(f, chunk);
    for (let n = 1; n < 12; n++) {
      const vector = Y.encodeStateVector(f.doc);
      // 每个文本节点仍遵守契约上限；多块正文总量超过 4 MiB，不能用非法超长单块测试 RPC。
      const source = prosemirrorJSONToYDoc(
        schema,
        toPM({
          type: 'doc',
          content: [
            {
              type: 'paragraph',
              attrs: { id: 'large-block-' + n },
              content: [{ type: 'text', text: chunk }],
            },
          ],
        }),
      );
      const fragment = f.doc.getXmlFragment('prosemirror');
      fragment.insert(
        fragment.length,
        source
          .getXmlFragment('prosemirror')
          .toArray()
          .map((node) => node.clone()),
      );
      source.destroy();
      f.w.send({
        type: 'UPDATE',
        client_update_id: operation('large-update'),
        update: Buffer.from(Y.encodeStateAsUpdate(f.doc, vector)).toString('base64'),
      });
      await f.w.next('SAVED');
    }
    const receipt = await flushDraft(f),
      id = operation('large-submission');
    const response = await post(
      `/v1/documents/${f.d.id}/units/${f.grant.context.target.unit_id}/submissions`,
      {
        operation_id: id,
        mode: 'NEW',
        branch_id: f.session.branch_id,
        expected_structure_id: f.grant.context.target.structure_id,
        flush_receipt_id: receipt.receipt_id,
      },
      f.f.owner.headers,
    );
    assert.equal(response.status, 202);
    const result = await terminal(id, f.f.owner.headers);
    assert.equal(result.state, 'SUCCEEDED');
    const database = await reviewDatabase();
    try {
      const [[row]] = await database.query(
        'SELECT byte_size,OCTET_LENGTH(canonical_body) AS actual FROM content_version WHERE id=?',
        [result.result.content_version_id],
      );
      assert.ok(BigInt(row.byte_size) > 4n << 20n);
      assert.equal(BigInt(row.byte_size), BigInt(row.actual));
    } finally {
      await database.end();
    }
  } finally {
    f.close();
  }
});

test('actual unchanged NEW is rejected and releases only its own guard', async () => {
  const f = await reviewFixture();
  try {
    const receipt = await flushDraft(f),
      id = operation('unchanged');
    const r = await post(
      `/v1/documents/${f.d.id}/units/${f.grant.context.target.unit_id}/submissions`,
      {
        operation_id: id,
        mode: 'NEW',
        branch_id: f.session.branch_id,
        expected_structure_id: f.grant.context.target.structure_id,
        flush_receipt_id: receipt.receipt_id,
      },
      f.f.owner.headers,
    );
    assert.equal(r.status, 202);
    const result = await terminal(id, f.f.owner.headers);
    assert.equal(result.state, 'REJECTED');
    assert.equal(result.error.code, 'SUBMISSION_NO_CHANGES');
    const database = await reviewDatabase();
    try {
      const [[guard]] = await database.query(
        'SELECT submission_operation_id,ongoing_request_id,resumable_request_id FROM unit_review_guard WHERE unit_id=?',
        [f.grant.context.target.unit_id],
      );
      assert.equal(guard.submission_operation_id, null);
      assert.equal(guard.ongoing_request_id, null);
      assert.equal(guard.resumable_request_id, null);
      const [[row]] = await database.query(
        'SELECT COUNT(*) AS count FROM review_round WHERE submission_operation_id=?',
        [id],
      );
      assert.equal(Number(row.count), 0);
    } finally {
      await database.end();
    }
  } finally {
    f.close();
  }
});
