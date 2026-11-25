import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createHash } from 'node:crypto';
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import { schema, toPM } from '../../packages/content/index.mjs';
import { runtime, state } from '../../scripts/runtime-lib.mjs';
import { request } from '../helpers/https.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { Transactions } from '../../services/collaboration/src/repository/sql.mjs';
import { lockDocument } from '../../services/collaboration/src/repository/main_workspace.mjs';
import { requireDocumentBodyPresent } from '../../services/collaboration/src/application/account_document_cleanup.mjs';
import { RoomSnapshots } from '../../services/collaboration/src/repository/room_snapshots.mjs';
import { verifyAccountResourceCleanup } from './account-resource-cleanup.mjs';
import { verifyAccountHistoryCleanup } from './account-history-cleanup.mjs';
import { verifyAccountPayloadCleanup } from './account-payload-cleanup.mjs';
import { verifyAccountRetentionComplete } from './account-retention-complete.mjs';

// 复用Go集中场景的真实CLEANING计划和ACK，Node私有SQL样本不冒充WSS编辑/发布。
// 清理走实际Doc/Node mTLS，失败恢复保留原输入，最后仅清理本脚本自己创建的样本。
const fixture = JSON.parse(process.argv[2]);
const node = await reviewDatabase('collaboration');
const doc = await reviewDatabase();
const tls = {
  key: readFileSync(resolve(runtime, 'pki/document-key.pem')),
  cert: readFileSync(resolve(runtime, 'pki/document.pem')),
};
const input = { operation_id: fixture.operation, document_id: fixture.document };
let callNumber = 0;
const call = async (value = input, options = {}) => {
  console.log('cleanup_control_call_' + ++callNumber);
  const result = await request(
    state().ports.collaborationHttp,
    '/infra/lifecycle/document-cleanup',
    {
      method: 'POST',
      ...tls,
      ...options,
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(value),
    },
  );
  console.log('cleanup_control_result_' + callNumber + ': ' + result.status);
  return result;
};
const task = fixture.operation + '-cleanup-pending';
const capture = fixture.operation + '-cleanup-capture';
const rooms = [];
let pending = false;
let cleaned = false;
try {
  for (const [document, unit] of [
    [fixture.document, fixture.unit],
    [fixture.surviving, fixture.survivingUnit],
  ]) {
    await node.query(
      "INSERT INTO room(document_id,unit_id,workspace_kind,workspace_id,epoch,last_durable_seq,state) VALUES(?,?,'MAIN',?,1,1,'OPEN')",
      [document, unit, unit],
    );
    const [[id]] = await node.query('SELECT CAST(LAST_INSERT_ID() AS CHAR) id');
    rooms.push(id.id);
    const body = {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'cleanup-block-' + unit },
          content: [{ type: 'text', text: '私有正文样本-' + document }],
        },
      ],
    };
    const ydoc = prosemirrorJSONToYDoc(schema, toPM(body));
    let bytes;
    try {
      bytes = Y.encodeStateAsUpdate(ydoc);
    } finally {
      ydoc.destroy();
    }
    const digest = createHash('sha256').update(bytes).digest();
    await node.query(
      "INSERT INTO checkpoint(room_id,epoch,covered_seq,yjs_state,checksum,format_version,validated_at) VALUES(?,1,1,?,?,'collab-yjs-v1',UTC_TIMESTAMP(6))",
      [id.id, bytes, digest],
    );
    const [[cp]] = await node.query('SELECT CAST(LAST_INSERT_ID() AS CHAR) id');
    await node.query('UPDATE room SET checkpoint_id=? WHERE id=?', [cp.id, id.id]);
    await node.query(
      'INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,1,1,?,?,?,?)',
      [id.id, fixture.operation, fixture.operation + '-update', bytes, digest],
    );
  }
  await node.query(
    "INSERT INTO capture_operation(operation_id,request_fingerprint,actor_id,room_id,epoch,cut_ref,lineage_ref,state,canonical_body,candidate_json,resource_refs) VALUES(?,?,?,?,1,?,?,'UNKNOWN',?,?,'[]')",
    [
      capture,
      Buffer.alloc(32, 1),
      fixture.user,
      rooms[0],
      JSON.stringify({ room_id: rooms[0], workspace_epoch: '1', durable_seq: '1' }),
      fixture.operation,
      Buffer.from('private-capture'),
      JSON.stringify({
        binding: { context: { target: { document_id: fixture.document } } },
        content: { body: 'private-capture' },
      }),
    ],
  );
  assert.equal((await call(input, { cert: undefined, key: undefined })).status, 403);
  assert.notEqual(
    (await call({ ...input, document_id: fixture.surviving })).status,
    200,
    '存续文档没有清理资格',
  );
  assert.notEqual((await call()).status, 200, 'UNKNOWN capture保留正文');
  const [[protectedCapture]] = await node.query(
    'SELECT state,canonical_body FROM capture_operation WHERE operation_id=?',
    [capture],
  );
  assert.equal(protectedCapture.state, 'UNKNOWN');
  assert.ok(protectedCapture.canonical_body);
  await node.query("UPDATE capture_operation SET state='CAPTURED' WHERE operation_id=?", [capture]);
  // 固定未来重试时间，不让实际恢复器消费此未终结故障样本。
  await doc.query(
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('MAIN_RECONCILE',?,?,?,'UNKNOWN',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR))",
    [
      task,
      task,
      JSON.stringify({
        context: { operation_id: task, target: { document_id: fixture.document } },
      }),
    ],
  );
  pending = true;
  assert.notEqual((await call()).status, 200, '原Go安装未知不能清理');
  await doc.query("DELETE FROM durable_task WHERE kind='MAIN_RECONCILE' AND business_key=?", [
    task,
  ]);
  pending = false;
  const fixed = JSON.stringify({
    binding: { context: { target: { document_id: fixture.document } } },
    units: [{ private_body: 'must-delete' }],
  });
  await node.query(
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('MAIN_VIEW_FIXED',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [task, task, fixed],
  );
  const done = await call();
  assert.equal(done.status, 200, done.body);
  const value = JSON.parse(done.body);
  assert.equal(value.state, 'CLEANED');
  cleaned = true;
  assert.equal(value.document_id, fixture.document);
  assert.equal(value.operation_id, fixture.operation);
  assert.deepEqual(JSON.parse((await call()).body), value, '原号重放不重复清理');
  assert.notEqual(
    (await call({ ...input, operation_id: fixture.operation + '-wrong' })).status,
    200,
  );
  const [[closed]] = await node.query('SELECT state,checkpoint_id FROM room WHERE id=?', [
    rooms[0],
  ]);
  assert.equal(closed.state, 'RETIRED');
  assert.equal(closed.checkpoint_id, null);
  for (const table of ['update_log', 'checkpoint']) {
    const [[row]] = await node.query('SELECT COUNT(*) n FROM ' + table + ' WHERE room_id=?', [
      rooms[0],
    ]);
    assert.equal(Number(row.n), 0, table + '实际正文已删除');
  }
  const [[captured]] = await node.query(
    'SELECT state,canonical_body,candidate_json FROM capture_operation WHERE operation_id=?',
    [capture],
  );
  assert.equal(captured.state, 'DELETED');
  assert.equal(captured.canonical_body, null);
  assert.equal(captured.candidate_json, null);
  const [[bodyTask]] = await node.query(
    "SELECT COUNT(*) n FROM durable_task WHERE kind='MAIN_VIEW_FIXED' AND business_key=?",
    [task],
  );
  assert.equal(Number(bodyTask.n), 0);
  const [[surviving]] = await node.query('SELECT state,checkpoint_id FROM room WHERE id=?', [
    rooms[1],
  ]);
  assert.equal(surviving.state, 'OPEN');
  assert.ok(surviving.checkpoint_id);
  const [recoverable] = await new RoomSnapshots(node).listRooms();
  assert.ok(!recoverable.some((row) => String(row.id) === rooms[0]));
  assert.ok(recoverable.some((row) => String(row.id) === rooms[1]));
  // 使用真实SQL验证系统安装器共同使用的墓碑，不能把已清理文档再物化成房间。
  const transaction = new Transactions({
    getConnection: async () => ({ query: node.query.bind(node), release() {} }),
  });
  await assert.rejects(
    transaction.run(async (c) => {
      await lockDocument(c, fixture.document);
      await requireDocumentBodyPresent(c, fixture.document);
    }),
    /ACCOUNT_DOCUMENT_CLEANED/,
  );
  if (process.env.COLLAB_TEST_RETENTION_COMPLETE === '1') {
    await verifyAccountRetentionComplete(doc, fixture);
  } else if (process.env.COLLAB_TEST_PAYLOAD_CLEANUP === '1') {
    await verifyAccountPayloadCleanup(doc, fixture);
  } else {
    if (process.env.COLLAB_TEST_RESOURCE_CLEANUP === '1')
      await verifyAccountResourceCleanup(doc, fixture, rooms[0]);
    if (process.env.COLLAB_TEST_HISTORY_CLEANUP === '1')
      await verifyAccountHistoryCleanup(doc, fixture);
  }
  console.log(
    'NODE_CLEANUP_ACTUAL_PASS: unknowns preserved; private body cleaned; surviving room preserved; original replay and installation tombstone verified',
  );
} finally {
  if (pending)
    await doc.query("DELETE FROM durable_task WHERE kind='MAIN_RECONCILE' AND business_key=?", [
      task,
    ]);
  if (!cleaned) await node.query('DELETE FROM capture_operation WHERE operation_id=?', [capture]);
  await node.query("DELETE FROM durable_task WHERE kind='MAIN_VIEW_FIXED' AND business_key=?", [
    task,
  ]);
  for (const room of rooms) {
    // 保留已清理的退休房间/墓碑/DELETED抓取审计，供实际服务重启核验。
    // 没有私有正文，也没有指向Doc或Account的跨库外键；存续样本正常精准释放。
    if (cleaned && room === rooms[0]) continue;
    await node.query('UPDATE room SET checkpoint_id=NULL WHERE id=?', [room]);
    await node.query('DELETE FROM update_log WHERE room_id=?', [room]);
    await node.query('DELETE FROM checkpoint WHERE room_id=?', [room]);
    await node.query('DELETE FROM room WHERE id=?', [room]);
  }
  // 留存原清理审计，与原Fence一样没有指向Doc/room的跨库外键。
  await node.end();
  await doc.end();
}
