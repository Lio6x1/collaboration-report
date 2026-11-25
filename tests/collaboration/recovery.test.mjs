import test from 'node:test';
import assert from 'node:assert/strict';
import * as Y from 'yjs';
import { restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import {
  organizationFixture,
  documentFixture,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { wire } from '../helpers/wss.mjs';

// 在真实日志已有提交、浏览器尚未消费SAVED时断开，并由owner supervisor重启全组进程。
// 保留数据库与原client/update身份，验证恢复读取持久状态而非空房间或新序号。
test('actual owned process restart restores durable checkpoints and logs and replays a lost acknowledgement as the original sequence', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    client = operation('client'),
    unit = d.main.units[0].unit_id;
  async function mint() {
    const r = await post(
      `/v1/documents/${d.id}/units/${unit}/edit-sessions`,
      {
        operation_id: operation('edit'),
        workspace_kind: 'BRANCH',
        client_session_id: client,
        expected_structure_id: d.main.structure.id,
      },
      f.owner.headers,
    );
    assert.equal(r.status, 200);
    return json(r);
  }
  const session = await mint(),
    w = wire(session, client, operation('connection')),
    doc = new Y.Doc();
  let later,
    restartPending = false;
  try {
    Y.applyUpdate(doc, Buffer.from((await w.ready).update, 'base64'));
    const vector = Y.encodeStateVector(doc),
      p = doc.getXmlFragment('prosemirror').get(0),
      text = new Y.XmlText();
    p.push([text]);
    text.insert(0, '进程重启后保留');
    const bytes = Buffer.from(Y.encodeStateAsUpdate(doc, vector)),
      id = operation('lost-ack');
    w.socket.on('message', (raw) => {
      if (JSON.parse(raw).type === 'UPDATE') w.close();
    });
    w.send({ type: 'UPDATE', client_update_id: id, update: bytes.toString('base64') });
    await w.next('UPDATE');
    assert.equal(
      w.messages.some((x) => x.type === 'SAVED'),
      false,
    );
    // 重启仍是本用例的实际验收动作；非零退出必须保留为失败。
    // finally只恢复后续用例所需的环境，不能将本次失败改成成功。
    restartPending = true;
    restartOwnedRuntime();
    restartPending = false;
    const again = await mint();
    assert.equal(again.room_id, session.room_id);
    later = wire(again, client, operation('new-connection'));
    const restored = await later.ready;
    assert.equal(restored.durable_seq, '1');
    const recovered = new Y.Doc();
    Y.applyUpdate(recovered, Buffer.from(restored.update, 'base64'));
    assert.equal(
      recovered.getXmlFragment('prosemirror').get(0).get(0).toString(),
      '进程重启后保留',
    );
    recovered.destroy();
    later.send({ type: 'UPDATE', client_update_id: id, update: bytes.toString('base64') });
    assert.equal((await later.next('SAVED')).durable_seq, '1');
  } finally {
    w.close();
    later?.close();
    doc.destroy();
    if (restartPending) restartOwnedRuntime();
  }
});
