import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { reviewFixture, writeDraftBody, openReviewWorkspace } from '../helpers/review.mjs';
import { post, get, json, operation } from '../helpers/workspace.mjs';
import { pollMerge } from '../helpers/merge.mjs';
import { mergedReverseFixture as mergedFixture, pollReverse } from '../helpers/reverse-changes.mjs';
import { persistedPull } from '../helpers/pull-main.mjs';
import { request } from '../helpers/https.mjs';
import { runtime, state } from '../../scripts/runtime-lib.mjs';

const digest = (raw) => createHash('sha256').update(raw).digest('hex');

test('reverse retains reliable MAIN additions omitted from the original manifest and preserves unpulled draft S2', async () => {
  const { f, db, fixed, merged, record, originalBlockId } = await mergedFixture({ mainTail: true });
  let next;
  try {
    // 原正式版本尚无新增；原合并的可靠MAIN捕获已有新增。反向必须使用后者作为历史来源。
    const [[originalVersion]] = await db.query(
      'SELECT canonical_body FROM content_version WHERE id=?',
      [f.d.main.units[0].content_version_id],
    );
    assert.doesNotMatch(
      originalVersion.canonical_body.toString(),
      /原合并前可靠保存但未物化的MAIN新增/,
    );
    const [[submittedVersion]] = await db.query(
      'SELECT canonical_body FROM content_version WHERE id=?',
      [fixed.content_version_id],
    );
    assert.doesNotMatch(
      submittedVersion.canonical_body.toString(),
      /原合并前可靠保存但未物化的MAIN新增/,
    );
    // S2位于稳定原块之前，MAIN新增在其后：两个边界有明确顺序，不能误删未拉入的MAIN新增。
    await writeDraftBody(f, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'reverse-s2-before-anchor' },
          content: [{ type: 'text', text: '未拉入MAIN前可靠保存的S2' }],
        },
        {
          type: 'paragraph',
          attrs: { id: originalBlockId },
          content: [{ type: 'text', text: '这项已经合并的修改需要反向' }],
        },
      ],
    });
    const id = operation('reverse-unpulled-main'),
      accepted = await post(
        `/v1/documents/${f.d.id}/reverse-changes`,
        {
          operation_id: id,
          expected_manifest_id: merged.result.manifest_id,
          merge_record_id: String(record.id),
          reason: '可靠MAIN历史来源与后来S2都必须保留',
        },
        f.user.headers,
      );
    assert.equal(accepted.status, 202, accepted.body);
    const result = await pollReverse(id, f.user.headers);
    assert.equal(result.state, 'SUCCEEDED', JSON.stringify(result));
    const [[round]] = await db.query(
      'SELECT v.canonical_body FROM review_round r JOIN content_version v ON v.id=r.content_version_id WHERE r.id=?',
      [result.result.round_id],
    );
    assert.match(round.canonical_body.toString(), /原合并前可靠保存但未物化的MAIN新增/);
    assert.match(round.canonical_body.toString(), /未拉入MAIN前可靠保存的S2/);
    assert.doesNotMatch(round.canonical_body.toString(), /这项已经合并的修改需要反向/);
    assert.equal(
      json(await get(`/v1/documents/${f.d.id}`, f.user.headers)).current_manifest_id,
      merged.result.manifest_id,
    );
    next = await openReviewWorkspace(f.f, f.d);
    assert.equal(BigInt(next.session.workspace_epoch), BigInt(f.session.workspace_epoch) + 1n);
  } finally {
    next?.close();
    f.close();
    await db.end();
  }
});

test('reverse creates a real new review, preserves later S2 and changes MAIN only after ordinary approval', async () => {
  const { f, db, fixed, merged, record, originalBlockId } = await mergedFixture({ members: 1 });
  let next;
  try {
    // 原合并只接纳S1；后来新增的不同块是可靠S2，反向算法必须保留它。
    await writeDraftBody(f, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: originalBlockId },
          content: [{ type: 'text', text: '这项已经合并的修改需要反向' }],
        },
        {
          type: 'paragraph',
          attrs: { id: 'later-unmerged-s2' },
          content: [{ type: 'text', text: '后来可靠保存的S2必须保留' }],
        },
      ],
    });
    const id = operation('reverse-new-review'),
      input = {
        operation_id: id,
        expected_manifest_id: merged.result.manifest_id,
        merge_record_id: String(record.id),
        reason: '保留历史，创建新的反向审核',
      },
      path = `/v1/documents/${f.d.id}/reverse-changes`;
    // 普通组织成员不能凭历史合并号获得MANAGE；拒绝发生在登记服务器来源之前。
    const denied = await post(path, input, f.f.members[0].headers);
    assert.equal(denied.status, 403, denied.body);
    const [[beforeRegistration]] = await db.query(
      'SELECT COUNT(*) n FROM operation_receipt WHERE operation_id=?',
      [id],
    );
    assert.equal(Number(beforeRegistration.n), 0);
    const accepted = await post(path, input, f.user.headers);
    assert.equal(accepted.status, 202, accepted.body);
    const result = await pollReverse(id, f.user.headers);
    assert.equal(result.state, 'SUCCEEDED', JSON.stringify(result));
    assert.notEqual(result.result.request_id, fixed.request_id);
    const [[round]] = await db.query(
      'SELECT r.submission_operation_id,v.source_kind,v.canonical_body,s.flush_receipt_ref,s.state FROM review_round r JOIN content_version v ON v.id=r.content_version_id JOIN submission_operation s ON s.operation_id=r.submission_operation_id WHERE r.id=?',
      [result.result.round_id],
    );
    assert.equal(round.submission_operation_id, id);
    assert.equal(round.source_kind, 'REVERSE');
    assert.equal(round.state, 'SUCCEEDED');
    assert.equal(round.flush_receipt_ref, 'reverse:' + id);
    assert.match(round.canonical_body.toString(), /后来可靠保存的S2必须保留/);
    assert.doesNotMatch(round.canonical_body.toString(), /这项已经合并的修改需要反向/);
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    assert.equal(current.current_manifest_id, merged.result.manifest_id);
    const [[old]] = await db.query(
      'SELECT status,current_round_id FROM review_request WHERE id=?',
      [fixed.request_id],
    );
    assert.equal(old.status, 'MERGED');
    assert.equal(String(old.current_round_id), fixed.round_id);
    const [[same]] = await db.query(
      "SELECT CAST(JSON_OBJECT('operation',operation_id,'round',round_id,'input',input_manifest_id,'output',output_manifest_id,'version',result_version_id,'capture',accepted_capture_ref,'lineage',lineage_id) AS CHAR) raw FROM merge_record WHERE id=?",
      [record.id],
    );
    assert.equal(digest(same.raw), digest(record.raw));
    // 原号重放不能追加第二个审核；安装后的共同草稿与审核固定候选来自同一正文。
    assert.equal((await post(path, input, f.user.headers)).status, 202);
    assert.equal((await post(path, input, f.f.members[0].headers)).status, 403);
    const changedIntent = await post(
      path,
      { ...input, reason: '同号不可替换理由' },
      f.user.headers,
    );
    assert.equal(changedIntent.status, 409, changedIntent.body);
    assert.equal(json(changedIntent).code, 'OPERATION_ID_CONFLICT');
    next = await openReviewWorkspace(f.f, f.d);
    assert.equal(BigInt(next.session.workspace_epoch), BigInt(f.session.workspace_epoch) + 1n);
    const approved = operation('approve-reverse-review');
    assert.equal(
      (
        await post(
          `/v1/review-requests/${result.result.request_id}/approve-and-merge`,
          {
            operation_id: approved,
            round_id: result.result.round_id,
            content_version_id: result.result.content_version_id,
            own_changes_to_resolve: [],
          },
          f.user.headers,
        )
      ).status,
      202,
    );
    const completed = await pollMerge(approved, f.user.headers, db);
    assert.equal(completed.state, 'SUCCEEDED', JSON.stringify(completed));
    assert.notEqual(completed.result.manifest_id, merged.result.manifest_id);
    const [[source]] = await db.query(
      "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='REVERSE_CONTEXT' AND business_key=?",
      [id],
    );
    const context = persistedPull(source.raw).Context;
    // 普通批准后请求已MERGED，私有原号仍能读取原安装固定来源；不能要求它再次处于REVIEWING。
    const historical = await request(state().ports.documentHttp, '/infra/draft/committed', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      cert: readFileSync(resolve(runtime, 'pki/collaboration.pem')),
      key: readFileSync(resolve(runtime, 'pki/collaboration-key.pem')),
      body: JSON.stringify({
        operation_id: id,
        target: context.target,
        request_fingerprint: context.request_fingerprint,
      }),
    });
    assert.equal(historical.status, 200, historical.body);
    assert.equal(json(historical).content.capture_id, id);
    assert.match(
      Buffer.from(json(historical).content.body_json_utf8, 'base64').toString(),
      /后来可靠保存的S2必须保留/,
    );
  } finally {
    next?.close();
    f.close();
    await db.end();
  }
});

test('reverse stops overlapping reliable draft input and preserves the original branch, MAIN and fixed source', async () => {
  const { f, db, merged, record, originalBlockId } = await mergedFixture();
  try {
    await writeDraftBody(f, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: originalBlockId },
          content: [{ type: 'text', text: '同一位置后来可靠输入，必须保留' }],
        },
      ],
    });
    const id = operation('reverse-overlap'),
      input = {
        operation_id: id,
        expected_manifest_id: merged.result.manifest_id,
        merge_record_id: String(record.id),
        reason: '同处竞争需要人工处理',
      };
    const accepted = await post(`/v1/documents/${f.d.id}/reverse-changes`, input, f.user.headers);
    assert.equal(accepted.status, 202, accepted.body);
    const result = await pollReverse(id, f.user.headers);
    assert.equal(result.state, 'REJECTED', JSON.stringify(result));
    assert.equal(result.error.code, 'REVERSE_CONFLICT');
    const [[branch]] = await db.query(
      'SELECT lifecycle,workspace_epoch,current_lineage_id FROM branch WHERE id=?',
      [f.session.branch_id],
    );
    assert.equal(branch.lifecycle, 'ACTIVE');
    assert.equal(String(branch.workspace_epoch), f.session.workspace_epoch);
    const [[fixed]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='REVERSE_FIXED' AND business_key=?",
      [id],
    );
    const value = typeof fixed.payload === 'string' ? JSON.parse(fixed.payload) : fixed.payload;
    assert.equal(value.candidate, null);
    assert.ok(value.conflicts.length > 0);
    assert.match(
      Buffer.from(value.draft.body_json_utf8, 'base64').toString(),
      /同一位置后来可靠输入/,
    );
    const [[reviews]] = await db.query(
      'SELECT COUNT(*) n FROM review_round WHERE submission_operation_id=?',
      [id],
    );
    assert.equal(Number(reviews.n), 0);
    assert.equal(
      json(await get(`/v1/documents/${f.d.id}`, f.user.headers)).current_manifest_id,
      merged.result.manifest_id,
    );
    await writeDraftBody(f, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: originalBlockId },
          content: [{ type: 'text', text: '冲突拒绝后原草稿继续可靠保存' }],
        },
      ],
    });
  } finally {
    f.close();
    await db.end();
  }
});

test('reverse rejects a stale manifest and a merge record belonging to another document before registering work', async () => {
  const { f, db, merged, record } = await mergedFixture(),
    other = await reviewFixture();
  try {
    for (const [document, manifest, expectedCode] of [
      [f.d.id, f.d.main.manifest_id, 'MAIN_CONTEXT_CHANGED'],
      [other.d.id, other.d.main.manifest_id, 'REVERSE_MERGE_NOT_FOUND'],
    ]) {
      const input = {
          operation_id: operation('invalid-reverse-source'),
          expected_manifest_id: manifest,
          merge_record_id: String(record.id),
          reason: '错误来源必须拒绝',
        },
        response = await post(
          `/v1/documents/${document}/reverse-changes`,
          input,
          document === f.d.id ? f.user.headers : other.user.headers,
        );
      assert.equal(response.status, 409, response.body);
      assert.equal(json(response).code, expectedCode);
      const [[count]] = await db.query(
        'SELECT COUNT(*) n FROM operation_receipt WHERE operation_id=?',
        [input.operation_id],
      );
      assert.equal(Number(count.n), 0);
    }
    assert.equal(
      json(await get(`/v1/documents/${f.d.id}`, f.user.headers)).current_manifest_id,
      merged.result.manifest_id,
    );
  } finally {
    other.close();
    f.close();
    await db.end();
  }
});
