import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime, state } from '../../scripts/runtime-lib.mjs';
import { request } from '../helpers/https.mjs';

// 与已通过的Node清理使用同一份原计划。仅新增草稿保护、正式图片留存和
// 未完成导出的衔接反例；这里不冒充实际上传、对象GC或完整账号注销验收。
export async function verifyAccountResourceCleanup(doc, fixture, room) {
  const operation = fixture.operation + '-resource-protection';
  const exportOperation = fixture.operation + '-pending-export';
  const input = { operation_id: fixture.operation, document_id: fixture.document };
  const tls = {
    key: readFileSync(resolve(runtime, 'pki/account-key.pem')),
    cert: readFileSync(resolve(runtime, 'pki/account.pem')),
  };
  const call = (value = input, options = {}) =>
    request(state().ports.documentHttp, '/infra/lifecycle/document-release-draft-resources', {
      method: 'POST',
      ...tls,
      ...options,
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(value),
    });
  try {
    await doc.query(
      "INSERT INTO resource_protection(operation_id,document_id,room_id,grant_id,workspace_epoch,generation,request_fingerprint,resource_ids,source_epochs,state) VALUES(?,?,?,?,1,1,?,JSON_ARRAY(CAST(? AS UNSIGNED)),'[]','UNKNOWN')",
      [operation, fixture.document, room, operation, Buffer.alloc(32, 7), fixture.privateResource],
    );
    await doc.query(
      "INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,'OPERATION',?,'DRAFT_UPDATE',1)",
      [fixture.privateResource, fixture.document, operation],
    );
    for (const resource of [fixture.resource, fixture.privateResource])
      await doc.query(
        "INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,'DRAFT',?,'ROOM_DRAFT',1)",
        [resource, fixture.document, room + ':1'],
      );
    assert.equal((await call(input, { cert: undefined, key: undefined })).status, 403);
    assert.notEqual((await call({ ...input, document_id: fixture.surviving })).status, 200);
    assert.notEqual((await call()).status, 200, 'UNKNOWN保护不能靠Node清理/TTL推成已释放');
    const [[protectedState]] = await doc.query(
      'SELECT state FROM resource_protection WHERE operation_id=?',
      [operation],
    );
    assert.equal(protectedState.state, 'UNKNOWN');
    const [[drafts]] = await doc.query(
      "SELECT COUNT(*) n FROM resource_reference WHERE document_id=? AND owner_kind='DRAFT'",
      [fixture.document],
    );
    assert.equal(Number(drafts.n), 2);
    // 将自己注入的未知样本恢复到已知保护；不是替生产未知操作伪造成功回执。
    await doc.query("UPDATE resource_protection SET state='PROTECTED' WHERE operation_id=?", [
      operation,
    ]);
    await doc.query(
      "INSERT INTO export_job(requester_id,document_id,manifest_id,target_kind,format,state,operation_id) VALUES(?,?,?,'MAIN','PDF','QUEUED',?)",
      [fixture.user, fixture.document, fixture.manifest, exportOperation],
    );
    assert.notEqual((await call()).status, 200, '未完成导出仍占有固定依赖');
    await doc.query('DELETE FROM export_job WHERE operation_id=?', [exportOperation]);
    const response = await call();
    assert.equal(response.status, 200, response.body);
    const result = JSON.parse(response.body);
    assert.equal(result.state, 'DRAFT_RESOURCES_RELEASED');
    assert.equal(result.operation_id, fixture.operation);
    assert.equal(result.document_id, fixture.document);
    assert.deepEqual(JSON.parse((await call()).body), result, '原号重放不重复建立RETENTION');
    assert.notEqual(
      (await call({ ...input, operation_id: fixture.operation + '-different' })).status,
      200,
    );
    const [[released]] = await doc.query(
      'SELECT state FROM resource_protection WHERE operation_id=?',
      [operation],
    );
    assert.equal(released.state, 'RELEASED');
    const [refs] = await doc.query(
      'SELECT CAST(resource_id AS CHAR) resource,owner_kind,purpose FROM resource_reference WHERE document_id=? ORDER BY resource_id,owner_kind',
      [fixture.document],
    );
    assert.equal(
      refs.filter((r) => r.owner_kind === 'DRAFT' || r.purpose === 'DRAFT_UPDATE').length,
      0,
    );
    assert.deepEqual(
      refs.filter((r) => r.owner_kind === 'RETENTION').map((r) => r.resource),
      [fixture.resource],
    );
    assert.equal(
      refs.filter((r) => r.owner_kind === 'VERSION').length,
      2,
      '仍存在的版本继续保护图片',
    );
    const [[life]] = await doc.query('SELECT state FROM lifecycle_operation WHERE operation_id=?', [
      fixture.operation,
    ]);
    assert.equal(life.state, 'CLEANING', '资源阶段不能提前终结整份注销');
    console.log(
      'DOC_RESOURCE_CLEANUP_ACTUAL_PASS: original Node receipt; unknown/export preserved; public image retained; draft protection released; version pins preserved; replay verified',
    );
  } finally {
    await doc.query('DELETE FROM export_job WHERE operation_id=?', [exportOperation]);
    await doc.query('DELETE FROM resource_protection WHERE operation_id=?', [operation]);
    await doc.query(
      "DELETE FROM resource_reference WHERE document_id=? AND owner_kind='OPERATION' AND owner_ref=?",
      [fixture.document, operation],
    );
  }
}
