import assert from 'node:assert/strict';
import test from 'node:test';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { openReviewWorkspace, writeDraft, reviewDatabase } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { inspectDocument } from '../../packages/content/index.mjs';
import { createHash } from 'node:crypto';
import { pause, runtime, state } from '../../scripts/runtime-lib.mjs';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createPeers } from '../../services/collaboration/src/client/peers.mjs';
import { request } from '../helpers/https.mjs';

// 只填满本场景的空间，不改正文、房间序号或保存回执。一次真实流程同时验证
// 满额读取和新正文拒绝，避免以上传的配额测试代替实时保存的配额边界。
test('full space retains MAIN reading and rejects growing draft before durable commit', async () => {
  const f = await organizationFixture(0);
  const d = await documentFixture(f);
  const db = await reviewDatabase();
  const node = await reviewDatabase('collaboration');
  let peer, limit;
  try {
    await unlockStructurePreview({ d, user: f.owner });
    peer = await openReviewWorkspace(f, d, f.owner, 'MAIN');
    await writeDraft(peer, '满额前真实保存的正文。');
    const [[quota]] = await db.query('SELECT limit_bytes FROM quota_account WHERE space_id=?', [
      f.space,
    ]);
    limit = quota.limit_bytes;
    const [[paidBefore]] = await db.query(
      'SELECT used_bytes,reserved_bytes FROM quota_account WHERE space_id=?',
      [f.space],
    );
    await db.query(
      'UPDATE quota_account SET limit_bytes=used_bytes+reserved_bytes,revision=revision+1 WHERE space_id=?',
      [f.space],
    );
    const view = await get(`/v1/documents/${d.id}/main`, f.owner.headers);
    console.log(
      JSON.stringify({
        event: 'FULL_QUOTA_READ',
        status: view.status,
        code: view.status === 200 ? 'OK' : json(view).code,
      }),
    );
    const [[before]] = await node.query('SELECT last_durable_seq FROM room WHERE id=?', [
      peer.session.room_id,
    ]);
    let originalInput;
    const send = peer.w.send;
    peer.w.send = (input) => {
      if (input.type === 'UPDATE') originalInput = input;
      send(input);
    };
    let outcome;
    try {
      outcome = (await writeDraft(peer, '满额后不允许保存的新增正文。'.repeat(100))).type;
    } catch (error) {
      outcome = error.code ?? error.message;
    }
    const [[after]] = await node.query('SELECT last_durable_seq FROM room WHERE id=?', [
      peer.session.room_id,
    ]);
    console.log(
      JSON.stringify({
        event: 'FULL_QUOTA_SAVE',
        outcome,
        before: String(before.last_durable_seq),
        after: String(after.last_durable_seq),
      }),
    );
    assert.equal(view.status, 200, '满额仍必须允许获取可靠 MAIN 阅读版本');
    const fixed = json(view);
    const body = await get(
      `/v1/documents/${d.id}/read-views/${fixed.read_view_id}/units/${fixed.units[0].unit_id}`,
      f.owner.headers,
    );
    assert.equal(body.status, 200);
    assert.ok(JSON.stringify(json(body).body_json).includes('满额前真实保存的正文。'));
    assert.equal(outcome, 'QUOTA_EXCEEDED');
    assert.equal(String(after.last_durable_seq), String(before.last_durable_seq));
    assert.ok(
      inspectDocument(peer.doc).canonical.includes('满额后不允许保存的新增正文。'),
      '被拒绝的客户端正文仍在本地，不能冒充已保存或清空',
    );
    const [[paidAfterRead]] = await db.query(
      'SELECT used_bytes,reserved_bytes FROM quota_account WHERE space_id=?',
      [f.space],
    );
    assert.deepEqual(paidAfterRead, paidBefore, '读取和被拒绝保存均不能新增费用');

    // 额度恢复后发送完全相同的原更新号/字节；重复 SAVED 只重放原收据。
    await db.query('UPDATE quota_account SET limit_bytes=?,revision=revision+1 WHERE space_id=?', [
      limit,
      f.space,
    ]);
    // 在真实 Doc 保护提交、Node 尚未收到原 UPDATE 的边界停止：预留必须保留，
    // UNKNOWN 不能成为退款证明。随后原 WSS 更新接续同一保护，不新建费用。
    const originalClient = peer.grant.client_session_id;
    const updateHash = createHash('sha256')
      .update(Buffer.from(originalInput.update, 'base64'))
      .digest();
    const protectionId = createHash('sha256')
      .update(
        [
          peer.session.room_id,
          peer.session.workspace_epoch,
          originalClient,
          originalInput.client_update_id,
          peer.grant.grant_id,
        ].join('\n'),
      )
      .digest('hex');
    const measured = inspectDocument(peer.doc);
    const protectedInput = {
      protection: {
        context: {
          ...peer.grant.context,
          operation_id: protectionId,
          request_fingerprint: updateHash.toString('base64'),
        },
        grant_id: peer.grant.grant_id,
        room_id: peer.session.room_id,
        workspace_epoch: peer.session.workspace_epoch,
        generation: '1',
        resource_ids: measured.resources,
        update_hash: updateHash.toString('base64'),
        client_session_id: originalClient,
        client_update_id: originalInput.client_update_id,
      },
      body_bytes: String(Buffer.byteLength(measured.canonical)),
      body_hash: createHash('sha256').update(measured.canonical).digest('hex'),
    };
    const peers = createPeers(
      JSON.parse(readFileSync(resolve(runtime, 'services/collaboration.json'))),
    );
    try {
      assert.equal(
        (await peers.http('/infra/draft/protect-body', protectedInput, { deadlineMs: 2500 })).state,
        'PROTECTED',
      );
      const [[held]] = await db.query(
        'SELECT used_bytes,reserved_bytes FROM quota_account WHERE space_id=?',
        [f.space],
      );
      assert.equal(String(held.used_bytes), String(paidBefore.used_bytes));
      assert.ok(BigInt(held.reserved_bytes) > 0n);
      assert.equal(
        (await peers.http('/infra/draft/protect-body', protectedInput, { deadlineMs: 2500 })).state,
        'PROTECTED',
      );
      await assert.rejects(
        peers.http(
          '/infra/draft/protect-body',
          { ...protectedInput, body_bytes: String(BigInt(protectedInput.body_bytes) + 1n) },
          { deadlineMs: 2500 },
        ),
        { message: 'DRAFT_BODY_BINDING_CHANGED' },
      );
      const proof = await request(state().ports.collaborationHttp, '/infra/persistence/update', {
        method: 'POST',
        cert: readFileSync(resolve(runtime, 'pki/document.pem')),
        key: readFileSync(resolve(runtime, 'pki/document-key.pem')),
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({
          room_id: peer.session.room_id,
          workspace_epoch: peer.session.workspace_epoch,
          client_session_id: originalClient,
          client_update_id: originalInput.client_update_id,
        }),
      });
      assert.equal(proof.status, 200);
      assert.equal(json(proof).state, 'UNKNOWN');
      const [[retained]] = await db.query(
        'SELECT used_bytes,reserved_bytes FROM quota_account WHERE space_id=?',
        [f.space],
      );
      assert.deepEqual(retained, held);
    } finally {
      peers.close();
    }
    peer.w.send(originalInput);
    const saved = await peer.w.next('SAVED');
    assert.equal(saved.client_update_id, originalInput.client_update_id);
    assert.equal(BigInt(saved.durable_seq), BigInt(before.last_durable_seq) + 1n);
    const [[paidAfterSave]] = await db.query(
      'SELECT used_bytes,reserved_bytes FROM quota_account WHERE space_id=?',
      [f.space],
    );
    assert.ok(BigInt(paidAfterSave.used_bytes) > BigInt(paidBefore.used_bytes));
    assert.equal(String(paidAfterSave.reserved_bytes), '0');
    peer.w.send(originalInput);
    const duplicate = await peer.w.next('SAVED');
    assert.deepEqual(duplicate, saved);
    const [[paidDuplicate]] = await db.query(
      'SELECT used_bytes,reserved_bytes FROM quota_account WHERE space_id=?',
      [f.space],
    );
    assert.deepEqual(paidDuplicate, paidAfterSave);
    const [[ledger]] = await db.query(
      'SELECT body_bytes,LOWER(HEX(body_hash)) body_hash,state FROM draft_body_reservation WHERE room_id=? AND workspace_epoch=? ORDER BY body_bytes DESC LIMIT 1',
      [peer.session.room_id, peer.session.workspace_epoch],
    );
    const canonical = inspectDocument(peer.doc).canonical;
    assert.equal(ledger.state, 'COMMITTED');
    assert.equal(String(ledger.body_bytes), String(Buffer.byteLength(canonical)));
    assert.equal(ledger.body_hash, createHash('sha256').update(canonical).digest('hex'));
    await db.query(
      'UPDATE quota_account SET limit_bytes=used_bytes+reserved_bytes,revision=revision+1 WHERE space_id=?',
      [f.space],
    );
    assert.equal(
      (await writeDraft(peer, '缩小正文。')).type,
      'SAVED',
      '满额仍允许不增加占用的删除编辑',
    );
    const [[paidShrink]] = await db.query(
      'SELECT used_bytes,reserved_bytes FROM quota_account WHERE space_id=?',
      [f.space],
    );
    assert.deepEqual(paidShrink, paidAfterSave, '旧日志仍保留，缩小正文不能提前退款');
    const room = peer.session.room_id;
    peer.close();
    peer = undefined;

    // 复用实际生命周期接口；只有 Node CLEANED 和 Doc 历史清理都完成才释放容量。
    const current = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
    await waitAuthorization(
      await post(
        `/v1/documents/${d.id}`,
        { operation_id: operation('quota-trash'), expected_revision: current.control_revision },
        f.owner.headers,
        'DELETE',
      ),
      f.owner,
    );
    const trash = await get(`/v1/spaces/${f.space}/trash?limit=100`, f.owner.headers);
    assert.equal(trash.status, 200, trash.body);
    const trashed = json(trash).items.find((item) => item.id === d.id);
    assert.equal(trashed?.lifecycle, 'TRASHED');
    const purge = {
      operation_id: operation('quota-purge'),
      expected_control_revision: trashed.control_revision,
      confirmed: true,
    };
    const start = await post(`/v1/documents/${d.id}/purge`, purge, f.owner.headers);
    assert.equal(start.status, 202, start.body);
    const deadline = Date.now() + 30000;
    let terminal;
    while (Date.now() < deadline) {
      const progress = await get(`/v1/operations/${purge.operation_id}`, f.owner.headers);
      assert.equal(progress.status, 200, progress.body);
      terminal = json(progress);
      if (terminal.state !== 'PREPARING') break;
      await pause(100);
    }
    assert.equal(terminal.state, 'SUCCEEDED', JSON.stringify(terminal));
    const [[released]] = await db.query(
      'SELECT used_bytes,reserved_bytes FROM quota_account WHERE space_id=?',
      [f.space],
    );
    assert.equal(String(released.used_bytes), '0');
    assert.equal(String(released.reserved_bytes), '0');
    const [[tail]] = await node.query('SELECT COUNT(*) n FROM update_log WHERE room_id=?', [room]);
    assert.equal(String(tail.n), '0');
    const [[rows]] = await db.query(
      "SELECT (SELECT COUNT(*) FROM draft_body_capacity WHERE document_id=?) capacity,(SELECT COUNT(*) FROM draft_body_reservation WHERE document_id=? AND state<>'RELEASED') pending,(SELECT COUNT(*) FROM reading_body_capacity WHERE document_id=?) reading",
      [d.id, d.id, d.id],
    );
    assert.deepEqual(Object.values(rows).map(String), ['0', '0', '0']);
    console.log(
      'FULL_QUOTA_MAINLINE_PASS: fixed read; rejected growth keeps local body and durable sequence; original retry/duplicate settles once; shrink retains safe high water; actual CLEANED refunds both ledgers',
    );
  } finally {
    peer?.close();
    try {
      if (limit !== undefined)
        await db.query(
          'UPDATE quota_account SET limit_bytes=?,revision=revision+1 WHERE space_id=?',
          [limit, f.space],
        );
    } finally {
      await db.end();
      await node.end();
    }
  }
});
