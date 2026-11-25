import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import {
  reviewFixture,
  writeDraft,
  flushDraft,
  captureInput,
  collaborationRPC,
} from '../helpers/review.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';

// 正文必须固定在服务端可靠切点，第二次编辑不能影响同号候选。
test('actual capture fixes durable S1 and replay stays S1 after S2', async () => {
  const f = await reviewFixture(),
    peer = collaborationRPC();
  try {
    await writeDraft(f, '第一份送审正文');
    const input = await captureInput(f, await flushDraft(f));
    const first = await rpcCall(peer, 'CaptureSubmission', input);
    assert.equal(first.state, 'FIXED');
    const bytes = Buffer.from(first.content.body_json_utf8);
    assert.match(bytes.toString('utf8'), /第一份送审正文/);
    const provenance = JSON.parse(Buffer.from(first.content.provenance_json_utf8).toString('utf8'));
    assert.equal(provenance.lineage_id, f.session.lineage_id);
    assert.equal(provenance.base_manifest_id, f.session.base_manifest_id);
    assert.equal(provenance.base_content_version_id, f.session.base_version_id);
    assert.equal(provenance.source_operation_id, input.context.operation_id);
    assert.ok(
      provenance.block_sources.some(
        (source) => source.origin_capture_ref === input.context.operation_id,
      ),
    );
    assert.equal(
      Buffer.from(first.content.body_hash).toString('hex'),
      createHash('sha256').update(bytes).digest('hex'),
    );
    await writeDraft(f, '继续编辑的新正文');
    assert.deepEqual(await rpcCall(peer, 'CaptureSubmission', input), first);
    assert.deepEqual(
      await rpcCall(peer, 'GetCapture', {
        operation_id: input.context.operation_id,
        target: input.context.target,
        request_fingerprint: input.context.request_fingerprint,
      }),
      first,
    );
    await assert.rejects(
      () =>
        rpcCall(peer, 'CaptureSubmission', {
          ...input,
          guard_generation: '2',
        }),
      (e) => e.code === 6,
    );
    await assert.rejects(
      () =>
        rpcCall(peer, 'GetCapture', {
          operation_id: input.context.operation_id,
          target: input.context.target,
          request_fingerprint: Buffer.alloc(32, 7),
        }),
      (e) => e.code === 6,
    );
  } finally {
    peer.close();
    f.close();
  }
});

test('actual capture rejects another actor receipt and a mismatching epoch', async () => {
  const f = await reviewFixture(),
    peer = collaborationRPC();
  try {
    await writeDraft(f, '绑定真实操作者');
    const input = await captureInput(f, await flushDraft(f));
    await assert.rejects(
      () =>
        rpcCall(peer, 'CaptureSubmission', {
          ...input,
          context: {
            ...input.context,
            actor_id: (BigInt(f.grant.context.actor_id) + 1n).toString(),
          },
        }),
      (e) => [7, 9].includes(e.code),
    );
    await assert.rejects(
      () =>
        rpcCall(peer, 'CaptureSubmission', {
          ...input,
          context: {
            ...input.context,
            target: { ...input.context.target, workspace_epoch: '987654321' },
          },
        }),
      (e) => [7, 9].includes(e.code),
    );
  } finally {
    peer.close();
    f.close();
  }
});
