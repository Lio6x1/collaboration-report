import test from 'node:test';
import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import { OutputSessions } from '../src/transport/output_sessions.mjs';

// 使用真实协调器和实际连接生命周期；SQL权限/保存协议另由真实端到端场景核验。
function fixture() {
  const outputs = new OutputSessions({ pool: {} }, 'test-instance', {}, {});
  outputs.settle = async () => {};
  const connect = (room, client) => {
    const socket = new EventEmitter();
    socket.readyState = 1;
    socket.messages = [];
    socket.send = (raw, done) => {
      socket.messages.push(JSON.parse(raw));
      done?.();
    };
    socket.terminate = () => {
      socket.readyState = 3;
      socket.emit('close');
    };
    const b = {
      room,
      epoch: '1',
      client,
      delivery: client,
      workspaceKind: 'BRANCH',
      grant: { context: { target: { document_id: '10', unit_id: '20' } } },
      expires: Date.now() + 60000,
    };
    outputs.attach(b, socket);
    return { b, socket };
  };
  const binding = { context: { operation_id: 'split-original', target: { document_id: '10' } } };
  const plan = { cut: { rooms: [{ room_id: '30' }] }, branches: [{ room_id: '30' }] };
  return { outputs, connect, binding, plan };
}

test('structure switch waits for each connection own real FLUSH and blocks new writes after ACK', async () => {
  const f = fixture(),
    a = f.connect('30', 'a'),
    b = f.connect('30', 'b');
  assert.equal(f.outputs.prepareStructure(f.binding, f.plan), 'WAITING');
  const pause = a.socket.messages[0];
  assert.equal(pause.type, 'STRUCTURE_PAUSE');
  // 客户端自报一个回执，不能证明已保存；他人的FLUSH也不能冒用。
  assert.throws(
    () => f.outputs.structureReady(a.b, { ...pause, receipt_id: 'fake', ready: true }),
    /STRUCTURE_SAVE_UNCONFIRMED/,
  );
  for (const c of [a, b]) {
    f.outputs.recordFlush(c.b, {
      type: 'FLUSHED',
      receipt_id: c.b.client,
      covered_cut: { room_id: '30', workspace_epoch: '1', durable_seq: '2' },
    });
    f.outputs.structureReady(c.b, {
      ...c.socket.messages[0],
      ready: true,
      receipt_id: c.b.client,
      block_id: 'paragraph-stable',
    });
  }
  assert.equal(f.outputs.prepareStructure(f.binding, f.plan), 'READY');
  assert.throws(() => f.outputs.assertStructureWrite(a.b), /STRUCTURE_CHANGE_PENDING/);
  const late = f.connect('30', 'late');
  assert.equal(late.socket.readyState, 3);
  f.outputs.finishStructure(f.binding);
  assert.equal(a.socket.messages.at(-1).type, 'STRUCTURE_RESUME');
  assert.doesNotThrow(() => f.outputs.assertStructureWrite(a.b));
  a.socket.terminate();
  b.socket.terminate();
});

test('unconfirmed disconnect cancels pause and a new operation can start without losing original inputs', () => {
  const f = fixture(),
    a = f.connect('30', 'a'),
    b = f.connect('30', 'b');
  f.outputs.prepareStructure(f.binding, f.plan);
  a.socket.terminate();
  assert.equal(f.outputs.prepareStructure(f.binding, f.plan), 'FAILED');
  assert.equal(b.socket.messages.at(-1).type, 'STRUCTURE_RESUME');
  f.outputs.finishStructure(f.binding);
  assert.equal(
    f.outputs.prepareStructure(
      { context: { ...f.binding.context, operation_id: 'new-split' } },
      f.plan,
    ),
    'WAITING',
  );
  b.socket.terminate();
});

test('installed navigation uses saved stable block ownership and does not send document content', async () => {
  const f = fixture(),
    a = f.connect('30', 'a');
  f.outputs.prepareStructure(f.binding, f.plan);
  f.outputs.recordFlush(a.b, {
    type: 'FLUSHED',
    receipt_id: 'saved',
    covered_cut: { room_id: '30', workspace_epoch: '1' },
  });
  f.outputs.structureReady(a.b, {
    ...a.socket.messages[0],
    ready: true,
    receipt_id: 'saved',
    block_id: 'paragraph-stable',
  });
  const originalSend = a.socket.send;
  let written;
  a.socket.send = (raw, done) => {
    originalSend(raw);
    written = done;
  };
  let completed = false;
  const installation = f.outputs.installedStructure(f.binding.context.operation_id, [
    { source_unit_id: '20', unit_id: '41', main_block_ids: ['other'], draft_block_ids: ['other'] },
    {
      source_unit_id: '20',
      unit_id: '42',
      main_block_ids: [],
      draft_block_ids: ['paragraph-stable'],
    },
  ]);
  installation.then(() => {
    completed = true;
  });
  await Promise.resolve();
  assert.equal(completed, false, '安装通知尚未写出时，不能允许调用方终止旧连接');
  written();
  await installation;
  const message = a.socket.messages.at(-1);
  assert.equal(message.type, 'STRUCTURE_INSTALLED');
  assert.equal(message.unit_id, '42');
  assert.equal(message.body, undefined);
  a.socket.terminate();
});
