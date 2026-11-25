import test from 'node:test';
import assert from 'node:assert/strict';
import {
  conflictedReviewFixture,
  conflictBody,
  conflictParagraph,
} from '../helpers/conflict-contexts.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { writeDraftBody, submitDraft } from '../helpers/review.mjs';
import { persistedPull } from '../helpers/pull-main.mjs';

test('conflict context reads the original complete fixed inputs and real versions after later reliable MAIN and draft changes', async () => {
  const { f, db, main, fixed, block, id } = await conflictedReviewFixture();
  try {
    const response = await get(`/v1/conflict-contexts/${id}`, f.user.headers);
    assert.equal(response.status, 200, response.body);
    const context = json(response);
    assert.equal(context.conflict_ref, id);
    assert.equal(context.operation_id, id);
    assert.equal(context.document_id, f.d.id);
    assert.equal(context.unit_id, f.d.main.units[0].unit_id);
    assert.equal(context.request_id, fixed.request_id);
    assert.equal(context.round_id, fixed.round_id);
    assert.match(context.attempt_id, /^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$/);
    assert.match(context.input_digest, /^[a-f0-9]{64}$/);
    assert.ok(Date.parse(context.expires_at) > Date.now());
    assert.ok(context.conflicts.length > 0);
    assert.ok(context.conflicts.some((value) => value.block_id === block));
    // 公共引用必须绑定真实原attempt；旧固定事实继续按实际主键读取，不能被接口编码替换。
    const [[attempt]] = await db.query(
      'SELECT CAST(id AS CHAR) id FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1',
      [id],
    );
    assert.equal(context.attempt_id, 'merge-attempt-' + attempt.id.padStart(20, '0'));
    const [[original]] = await db.query(
      "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='MERGE_ATTEMPT_FIXED' AND business_key=?",
      [id + ':' + attempt.id],
    );
    const input = persistedPull(original.raw).Input;
    for (const part of ['base', 'submitted', 'main']) {
      assert.deepEqual(
        context[part].body_json,
        JSON.parse(Buffer.from(input[part].body_json_utf8, 'base64').toString()),
      );
      assert.equal(
        context[part].body_hash,
        Buffer.from(input[part].body_hash, 'base64').toString('hex'),
      );
      const [[version]] = await db.query(
        'SELECT canonical_body,HEX(body_hash) hash FROM content_version WHERE id=? AND document_id=? AND unit_id=?',
        [context[part].id, f.d.id, f.d.main.units[0].unit_id],
      );
      assert.ok(version, part + ' must refer to a real immutable content version');
      assert.deepEqual(JSON.parse(version.canonical_body.toString()), context[part].body_json);
      assert.equal(version.hash.toLowerCase(), context[part].body_hash);
    }
    assert.match(JSON.stringify(context.main.body_json), /原计算的可靠MAIN修改/);
    assert.match(JSON.stringify(context.submitted.body_json), /原计算的固定提交S1/);
    // 后来两侧各收到真实SAVED，读取仍返回原attempt，不能重新采样或把现在草稿伪装成原submitted。
    await writeDraftBody(main, conflictBody(conflictParagraph(block, '后来可靠MAIN输入必须保留')));
    await writeDraftBody(
      f,
      conflictBody(
        conflictParagraph('conflict-later-s2', '后来独立草稿S2必须保留'),
        conflictParagraph(block, '原计算的固定提交S1'),
      ),
    );
    const repeat = await get(`/v1/conflict-contexts/${id}`, f.user.headers);
    assert.equal(repeat.status, 200, repeat.body);
    assert.deepEqual(json(repeat), context);
  } finally {
    main.close();
    f.close();
    await db.end();
  }
});

test('conflict context requires current original-scope access and a real conflict reference', async () => {
  const { f, db, main, id } = await conflictedReviewFixture({ members: 1 });
  try {
    const member = await get(`/v1/conflict-contexts/${id}`, f.f.members[0].headers);
    assert.equal(member.status, 403, member.body);
    assert.doesNotMatch(member.body, /原计算的可靠MAIN修改|原计算的固定提交S1/);
    const anonymous = await get(`/v1/conflict-contexts/${id}`);
    assert.equal(anonymous.status, 401, anonymous.body);
    const unknown = await get(
      `/v1/conflict-contexts/${operation('unknown-conflict')}`,
      f.user.headers,
    );
    assert.equal(unknown.status, 404, unknown.body);
  } finally {
    main.close();
    f.close();
    await db.end();
  }
});

test('conflict context rejects substituted immutable snapshot references before any body output', async () => {
  const { f, db, main, id } = await conflictedReviewFixture();
  let original, key;
  try {
    const response = await get(`/v1/conflict-contexts/${id}`, f.user.headers);
    assert.equal(response.status, 200, response.body);
    const [[attempt]] = await db.query(
      'SELECT CAST(id AS CHAR) id FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1',
      [id],
    );
    key = id + ':' + attempt.id;
    const [[artifact]] = await db.query(
      "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='CONFLICT_CONTEXT_VERSIONS' AND business_key=?",
      [key],
    );
    original = artifact.raw;
    const versions = persistedPull(original);
    // 修改可写恢复元数据，保留原attempt/digest，只替换成同文档的另一份真实不可变版本。
    // 两份元数据相互一致不足以证明来源正确；必须逐项核验实际版本与原固定prototype。
    for (const [field, replacement] of [
      ['Base', versions.Submitted],
      ['Submitted', versions.Main],
      ['Main', versions.Base],
    ]) {
      try {
        await db.query(
          "UPDATE durable_task SET payload=JSON_SET(payload,?,CAST(? AS UNSIGNED)) WHERE kind='CONFLICT_CONTEXT_VERSIONS' AND business_key=?",
          ['$.' + field, replacement, key],
        );
        const rejected = await get(`/v1/conflict-contexts/${id}`, f.user.headers);
        assert.equal(rejected.status, 503, field + ': ' + rejected.body);
        assert.equal(json(rejected).code, 'CONFLICT_VERSIONS_UNKNOWN');
        assert.doesNotMatch(rejected.body, /原计算的可靠MAIN修改|原计算的固定提交S1/);
      } finally {
        await db.query(
          "UPDATE durable_task SET payload=? WHERE kind='CONFLICT_CONTEXT_VERSIONS' AND business_key=?",
          [original, key],
        );
      }
    }
    const restored = await get(`/v1/conflict-contexts/${id}`, f.user.headers);
    assert.equal(restored.status, 200, restored.body);
    assert.deepEqual(json(restored), json(response));
  } finally {
    main.close();
    f.close();
    await db.end();
  }
});

test('conflict context follows current comment authorization and stops after real revocation', async () => {
  const { f, db, main, id } = await conflictedReviewFixture({ members: 1 });
  try {
    const reader = f.f.members[0];
    const grant = {
      user_id: reader.id,
      scope_kind: 'DOCUMENT',
      scope_id: f.d.id,
      capability: 'COMMENT',
    };
    const change = async (action) => {
      const document = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      await waitAuthorization(
        await post(
          `/v1/documents/${f.d.id}/authorization-changes`,
          {
            operation_id: operation('conflict-comment-acl'),
            expected_acl_revision: document.acl_revision,
            changes: [{ action, grant }],
          },
          f.user.headers,
        ),
        f.user,
      );
    };
    assert.equal((await get(`/v1/conflict-contexts/${id}`, reader.headers)).status, 403);
    await change('GRANT');
    const allowed = await get(`/v1/conflict-contexts/${id}`, reader.headers);
    assert.equal(allowed.status, 200, allowed.body);
    assert.match(JSON.stringify(json(allowed).submitted.body_json), /原计算的固定提交S1/);
    await change('REVOKE');
    const denied = await get(`/v1/conflict-contexts/${id}`, reader.headers);
    assert.equal(denied.status, 403, denied.body);
    assert.doesNotMatch(denied.body, /原计算的可靠MAIN修改|原计算的固定提交S1/);
  } finally {
    main.close();
    f.close();
    await db.end();
  }
});

test('expired public conflict context does not erase original private fixed inputs', async () => {
  const { f, db, main, id } = await conflictedReviewFixture();
  let expires;
  try {
    const response = await get(`/v1/conflict-contexts/${id}`, f.user.headers);
    assert.equal(response.status, 200, response.body);
    const [[receipt]] = await db.query(
      "SELECT CAST(expires_at AS CHAR) expires FROM operation_receipt WHERE operation_id=? AND kind='MERGE'",
      [id],
    );
    expires = receipt.expires;
    const facts = () =>
      db.query(
        "SELECT kind,business_key,SHA2(payload,256) hash FROM durable_task WHERE operation_id=? AND kind IN ('MERGE_ATTEMPT_FIXED','MERGE_ATTEMPT_RANGE_FIXED','CONFLICT_CONTEXT_VERSIONS') ORDER BY kind,business_key",
        [id],
      );
    const [before] = await facts();
    assert.ok(before.length >= 2);
    // 只推进测试操作的用户查询期限；不删除系统事实或修改原输入、摘要及轮次。
    await db.query(
      "UPDATE operation_receipt SET expires_at=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE operation_id=? AND kind='MERGE'",
      [id],
    );
    const expired = await get(`/v1/conflict-contexts/${id}`, f.user.headers);
    assert.equal(expired.status, 404, expired.body);
    assert.equal(json(expired).code, 'CONFLICT_CONTEXT_NOT_FOUND');
    assert.doesNotMatch(expired.body, /原计算的可靠MAIN修改|原计算的固定提交S1/);
    const [after] = await facts();
    assert.deepEqual(after, before);
  } finally {
    if (expires)
      await db.query(
        "UPDATE operation_receipt SET expires_at=? WHERE operation_id=? AND kind='MERGE'",
        [expires, id],
      );
    main.close();
    f.close();
    await db.end();
  }
});

test('conflict snapshot binding rejects identical body from another real submission operation', async () => {
  const { f, db, main, id, fixed } = await conflictedReviewFixture();
  let original, key;
  try {
    const response = await get(`/v1/conflict-contexts/${id}`, f.user.headers);
    assert.equal(response.status, 200, response.body);
    const context = json(response);
    const withdrawn = await post(
      `/v1/review-requests/${fixed.request_id}/withdrawals`,
      { operation_id: operation('conflict-source-withdraw'), expected_round_id: fixed.round_id },
      f.user.headers,
    );
    assert.equal(withdrawn.status, 200, withdrawn.body);
    const submitted = (
      await submitDraft(f, {
        mode: 'RESUBMIT',
        request_id: fixed.request_id,
        expected_round_id: fixed.round_id,
      })
    ).value;
    assert.equal(submitted.state, 'SUCCEEDED', JSON.stringify(submitted));
    const version = submitted.result.content_version_id;
    assert.notEqual(version, context.submitted.id);
    const [[actual]] = await db.query(
      'SELECT HEX(body_hash) hash FROM content_version WHERE id=?',
      [version],
    );
    assert.equal(actual.hash.toLowerCase(), context.submitted.body_hash);
    const [[attempt]] = await db.query(
      'SELECT CAST(id AS CHAR) id FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1',
      [id],
    );
    key = id + ':' + attempt.id;
    const [[artifact]] = await db.query(
      "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='CONFLICT_CONTEXT_VERSIONS' AND business_key=?",
      [key],
    );
    original = artifact.raw;
    // 正文/hash完全一样，另一次真实送审的来源仍不同；仅比较正文摘要不能通过这个反例。
    await db.query(
      "UPDATE durable_task SET payload=JSON_SET(payload,'$.Submitted',CAST(? AS UNSIGNED)) WHERE kind='CONFLICT_CONTEXT_VERSIONS' AND business_key=?",
      [version, key],
    );
    const denied = await get(`/v1/conflict-contexts/${id}`, f.user.headers);
    assert.equal(denied.status, 503, denied.body);
    assert.equal(json(denied).code, 'CONFLICT_VERSIONS_UNKNOWN');
  } finally {
    if (original)
      await db.query(
        "UPDATE durable_task SET payload=? WHERE kind='CONFLICT_CONTEXT_VERSIONS' AND business_key=?",
        [original, key],
      );
    main.close();
    f.close();
    await db.end();
  }
});
