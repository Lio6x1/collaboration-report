import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime, state } from '../../scripts/runtime-lib.mjs';
import { request } from '../helpers/https.mjs';

// 复用原CLEANING计划，私有SQL增加真实外键环/自引用样本；生产清理经mTLS调用。
// 不声称这些SQL样本等同真实送审/合并用户流程；该流程已有独立安装回归。
export async function verifyAccountHistoryCleanup(doc, fixture) {
  const tls = {
    key: readFileSync(resolve(runtime, 'pki/account-key.pem')),
    cert: readFileSync(resolve(runtime, 'pki/account.pem')),
  };
  const operation = fixture.operation + '-history-sample';
  const pendingNotice = operation + '-notice';
  const insert = async (sql, args) => {
    await doc.query(sql, args);
    const [[row]] = await doc.query('SELECT CAST(LAST_INSERT_ID() AS CHAR) id');
    return row.id;
  };
  const call = (document = fixture.document, options = {}) =>
    request(state().ports.documentHttp, '/infra/lifecycle/document-clean-history', {
      method: 'POST',
      ...tls,
      ...options,
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ operation_id: fixture.operation, document_id: document }),
    });
  let branch, lineage, previous, review, round, action, resolveAction, change, thread;
  try {
    branch = await insert(
      "INSERT INTO branch(document_id,unit_id,lifecycle,initial_base_version_id) VALUES(?,?,'RETIRED',?)",
      [fixture.document, fixture.unit, fixture.privateVersion],
    );
    const makeLineage = (parent, kind, suffix) =>
      insert(
        "INSERT INTO branch_lineage(branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,provenance_json,provenance_schema) VALUES(?,?,?,NULLIF(?,0),?,?,?,?,'{}','provenance-v1')",
        [
          branch,
          fixture.unit,
          fixture.document,
          parent,
          kind,
          operation + suffix,
          fixture.privateManifest,
          fixture.privateVersion,
        ],
      );
    previous = await makeLineage('0', 'CREATED', '-1');
    lineage = await makeLineage(previous, 'PULL_MAIN', '-2');
    await doc.query('UPDATE branch SET current_lineage_id=? WHERE id=?', [lineage, branch]);
    await doc.query('UPDATE collaboration_unit SET active_branch_id=? WHERE id=?', [
      branch,
      fixture.unit,
    ]);
    review = await insert(
      "INSERT INTO review_request(document_id,unit_id,branch_id,status) VALUES(?,?,?,'WITHDRAWN')",
      [fixture.document, fixture.unit, branch],
    );
    await doc.query(
      "INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,expires_at) VALUES(?,?,'SUBMISSION',?,?, 'SUCCEEDED',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR))",
      [
        operation,
        fixture.user,
        JSON.stringify({
          path: '/v1/documents/' + fixture.document + '/units/' + fixture.unit + '/submissions',
        }),
        Buffer.alloc(32, 8),
      ],
    );
    await doc.query(
      "INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,request_id,state,flush_receipt_ref,lineage_id) VALUES(?,?,?,?,?,?,'SUCCEEDED',?,?)",
      [operation, fixture.user, fixture.document, fixture.unit, branch, review, operation, lineage],
    );
    for (const kind of ['SUBMISSION_RECOVER', 'CAPTURE_FINALIZE'])
      await doc.query(
        "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
        [
          kind,
          operation,
          operation,
          JSON.stringify({ capture: { context: { target: { document_id: fixture.document } } } }),
        ],
      );
    round = await insert(
      'INSERT INTO review_round(request_id,document_id,unit_id,round_no,content_version_id,base_manifest_id,base_version_id,structure_id,lineage_id,capture_ref,submitter_id,submission_operation_id) VALUES(?,?,?,1,?,?,?,?,?,?,?,?)',
      [
        review,
        fixture.document,
        fixture.unit,
        fixture.privateVersion,
        fixture.privateManifest,
        fixture.privateVersion,
        fixture.structure,
        lineage,
        operation,
        fixture.user,
        operation,
      ],
    );
    await doc.query(
      'UPDATE submission_operation SET expected_round_id=?,result_round_id=? WHERE operation_id=?',
      [round, round, operation],
    );
    await doc.query('UPDATE review_request SET current_round_id=? WHERE id=?', [round, review]);
    await doc.query(
      'INSERT INTO request_submitter(request_id,user_id,first_round_id) VALUES(?,?,?)',
      [review, fixture.user, round],
    );
    action = await insert(
      "INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason) VALUES(?,?,?,'REQUEST_CHANGES',?,0,'私有审核意见')",
      [review, round, fixture.user, operation],
    );
    change = await insert(
      "INSERT INTO change_request(request_id,origin_round_id,created_action_id,author_id,status) VALUES(?,?,?,?,'OPEN')",
      [review, round, action, fixture.user],
    );
    resolveAction = await insert(
      "INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,target_change_id) VALUES(?,?,?,'RESOLVE_CHANGES',?,1,'私有处理意见',?)",
      [review, round, fixture.user, operation, change],
    );
    await doc.query("UPDATE change_request SET status='RESOLVED',resolved_action_id=? WHERE id=?", [
      resolveAction,
      change,
    ]);
    thread = await insert(
      "INSERT INTO comment_thread(document_id,context_kind,context_ref,content_version_id,anchor_json,anchor_schema,author_id,status) VALUES(?,'MAIN',?,?,'{}','anchor-v1',?,'OPEN')",
      [fixture.document, operation, fixture.privateVersion, fixture.user],
    );
    const comment = await insert('INSERT INTO comment(thread_id,author_id,body) VALUES(?,?,?)', [
      thread,
      fixture.user,
      '私有历史批注',
    ]);
    await insert('INSERT INTO comment(thread_id,author_id,reply_to_id,body) VALUES(?,?,?,?)', [
      thread,
      fixture.user,
      comment,
      '私有历史回复',
    ]);
    assert.equal((await call(fixture.document, { cert: undefined, key: undefined })).status, 403);
    assert.notEqual((await call(fixture.surviving)).status, 200);
    // 通知尚未获得真实终态时，历史清理必须等待，不能删掉原投递权威事实。
    await doc.query(
      "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('COMMENT_NOTIFICATION',?,?,?,'UNKNOWN',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR))",
      [pendingNotice, pendingNotice, JSON.stringify({ document_id: fixture.document })],
    );
    assert.notEqual((await call()).status, 200, '未知通知不能留下失去原事实的不可恢复任务');
    const [[stillPresent]] = await doc.query('SELECT COUNT(*) n FROM comment_thread WHERE id=?', [
      thread,
    ]);
    assert.equal(Number(stillPresent.n), 1);
    await doc.query('DELETE FROM durable_task WHERE business_key=? AND operation_id=?', [
      pendingNotice,
      pendingNotice,
    ]);
    const [[upload]] = await doc.query(
      'SELECT upload_operation_id operation FROM resource WHERE id=?',
      [fixture.privateResource],
    );
    await doc.query(
      "INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,'OPERATION',?,'UPLOAD',1)",
      [fixture.privateResource, fixture.document, upload.operation],
    );
    const response = await call();
    assert.equal(response.status, 200, response.body);
    assert.equal(JSON.parse(response.body).state, 'HISTORY_REMOVED');
    assert.deepEqual(JSON.parse((await call()).body), JSON.parse(response.body));
    const [[current]] = await doc.query(
      'SELECT title,CAST(current_manifest_id AS CHAR) manifest,lifecycle FROM document WHERE id=?',
      [fixture.document],
    );
    assert.equal(current.title, '固定正式标题');
    assert.equal(current.manifest, fixture.manifest);
    assert.equal(current.lifecycle, 'CLOSING', '操作正文副本/公开留存仍未完成');
    const [versions] = await doc.query(
      'SELECT CAST(id AS CHAR) id FROM content_version WHERE id IN (?,?)',
      [fixture.version, fixture.privateVersion],
    );
    assert.deepEqual(
      versions.map((r) => r.id),
      [fixture.version],
    );
    for (const [table, id] of [
      ['main_manifest', fixture.ancestorManifest],
      ['structure_revision', fixture.ancestorStructure],
      ['style_revision', fixture.ancestorStyle],
    ]) {
      const [[row]] = await doc.query('SELECT COUNT(*) n FROM ' + table + ' WHERE id=?', [id]);
      assert.equal(Number(row.n), 1, '保留真实外键所需祖先骨架');
    }
    const [[entries]] = await doc.query(
      'SELECT COUNT(*) n FROM structure_entry WHERE structure_id=?',
      [fixture.ancestorStructure],
    );
    assert.equal(Number(entries.n), 0, '旧祖先树标题必须真正删除');
    const [[ancestorEntries]] = await doc.query(
      'SELECT COUNT(*) n FROM manifest_entry WHERE manifest_id=?',
      [fixture.ancestorManifest],
    );
    assert.equal(Number(ancestorEntries.n), 0, '祖先manifest不保留私有正文依赖');
    const [refs] = await doc.query(
      "SELECT CAST(resource_id AS CHAR) resource FROM resource_reference WHERE document_id=? AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD')",
      [fixture.document],
    );
    assert.ok(
      refs.length >= 2 && refs.every((r) => r.resource === fixture.resource),
      '私有版本图片pin随正文移除',
    );
    const [[uploadBinding]] = await doc.query(
      "SELECT CAST(document_id AS CHAR) document FROM resource_reference WHERE resource_id=? AND owner_kind='OPERATION' AND purpose='UPLOAD'",
      [fixture.privateResource],
    );
    assert.equal(uploadBinding.document, fixture.document, 'GC/上传原号恢复绑定必须保持');
    const [[dependents]] = await doc.query(
      'SELECT (SELECT COUNT(*) FROM branch WHERE document_id=?)+(SELECT COUNT(*) FROM review_request WHERE document_id=?)+(SELECT COUNT(*) FROM comment_thread WHERE document_id=?) n',
      [fixture.document, fixture.document, fixture.document],
    );
    assert.equal(Number(dependents.n), 0);
    const privateResult = await call(fixture.privateDocument);
    assert.equal(privateResult.status, 200, privateResult.body);
    const [[erased]] = await doc.query(
      'SELECT current_manifest_id,current_release_id,lifecycle FROM document WHERE id=?',
      [fixture.privateDocument],
    );
    assert.equal(erased.current_manifest_id, null);
    assert.equal(erased.current_release_id, null);
    for (const table of [
      'main_manifest',
      'structure_revision',
      'style_revision',
      'chapter',
      'collaboration_unit',
      'resource_reference',
    ]) {
      const [[row]] = await doc.query('SELECT COUNT(*) n FROM ' + table + ' WHERE document_id=?', [
        fixture.privateDocument,
      ]);
      assert.equal(Number(row.n), 0, table + '私有文档全部移除');
    }
    const [[oldBody]] = await doc.query('SELECT COUNT(*) n FROM content_version WHERE id=?', [
      fixture.erasedVersion,
    ]);
    assert.equal(Number(oldBody.n), 0);
    const [[surviving]] = await doc.query(
      'SELECT lifecycle,current_manifest_id FROM document WHERE id=?',
      [fixture.surviving],
    );
    assert.equal(surviving.lifecycle, 'ACTIVE');
    assert.ok(surviving.current_manifest_id);
    console.log(
      'DOC_HISTORY_CLEANUP_ACTUAL_PASS: immutable leaf deletion; lineage/review/comment cycles; fixed release and ancestor skeleton retained; old titles/private bodies/version pins removed; original replay; surviving organization unchanged',
    );
  } finally {
    await doc.query('DELETE FROM durable_task WHERE business_key=? AND operation_id=?', [
      pendingNotice,
      pendingNotice,
    ]);
    // 失败时也精准释放本脚本的自有关系，不改变正式固定计划或其他文档。
    if (thread) {
      await doc.query('UPDATE comment SET reply_to_id=NULL WHERE thread_id=?', [thread]);
      await doc.query('DELETE FROM comment WHERE thread_id=?', [thread]);
      await doc.query('DELETE FROM comment_thread WHERE id=?', [thread]);
    }
    if (review) {
      await doc.query('UPDATE review_request SET current_round_id=NULL WHERE id=?', [review]);
      await doc.query(
        'UPDATE submission_operation SET expected_round_id=NULL,result_round_id=NULL WHERE operation_id=?',
        [operation],
      );
      await doc.query(
        "UPDATE change_request SET status='OPEN',resolved_action_id=NULL WHERE request_id=?",
        [review],
      );
      if (resolveAction) await doc.query('DELETE FROM review_action WHERE id=?', [resolveAction]);
      if (change) await doc.query('DELETE FROM change_request WHERE id=?', [change]);
      if (action) await doc.query('DELETE FROM review_action WHERE id=?', [action]);
      await doc.query('DELETE FROM request_submitter WHERE request_id=?', [review]);
      await doc.query('DELETE FROM review_round WHERE request_id=?', [review]);
      await doc.query('DELETE FROM submission_operation WHERE operation_id=?', [operation]);
      await doc.query('DELETE FROM review_request WHERE id=?', [review]);
    }
    if (branch) {
      await doc.query(
        'UPDATE collaboration_unit SET active_branch_id=NULL WHERE active_branch_id=?',
        [branch],
      );
      await doc.query('UPDATE branch SET current_lineage_id=NULL WHERE id=?', [branch]);
      if (lineage) await doc.query('DELETE FROM branch_lineage WHERE id=?', [lineage]);
      if (previous) await doc.query('DELETE FROM branch_lineage WHERE id=?', [previous]);
      await doc.query('DELETE FROM branch WHERE id=?', [branch]);
    }
    await doc.query('DELETE FROM durable_task WHERE operation_id=?', [operation]);
    await doc.query('DELETE FROM operation_receipt WHERE operation_id=?', [operation]);
  }
}
