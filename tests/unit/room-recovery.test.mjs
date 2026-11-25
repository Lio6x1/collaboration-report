import assert from 'node:assert/strict';
import test from 'node:test';
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import { schema, toPM } from '../../packages/content/index.mjs';
import { encodeCheckpoint } from '../../services/collaboration/src/domain/room_replay.mjs';
import { RoomSnapshots } from '../../services/collaboration/src/application/room_snapshots.mjs';

function checkpoint() {
  const doc = prosemirrorJSONToYDoc(
    schema,
    toPM({
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'recovery-unit-paragraph' },
          content: [{ type: 'text', text: '真实 Yjs 检查点及内容校验' }],
        },
      ],
    }),
  );
  try {
    const encoded = encodeCheckpoint(doc);
    return {
      yjs_state: encoded.bytes,
      checksum: encoded.checksum,
      format_version: 'collab-yjs-v1',
      covered_seq: '0',
    };
  } finally {
    doc.destroy();
  }
}

function rooms(count) {
  return Array.from({ length: count }, (_, index) => ({
    id: String(index + 1),
    epoch: '1',
    last_durable_seq: '0',
    workspace_kind: 'MAIN',
    state: index % 2 ? 'RETIRED' : 'OPEN',
  }));
}

test('startup unit: independent room replay overlaps bounded reads and still verifies every retained room', async () => {
  const held = Promise.withResolvers(),
    entered = Promise.withResolvers();
  const visited = new Set(),
    validated = new Set(),
    fixed = checkpoint();
  let active = 0,
    maximum = 0;
  const store = { pool: {}, roomsRecovered: false };
  // SQL 适配器只在隔离单元模拟 I/O；Yjs/checksum/完整正文校验沿真实生产恢复函数。
  const snapshots = new RoomSnapshots(
    {
      listRooms: async () => [rooms(6)],
      latestCheckpoint: async (_connection, id) => {
        visited.add(id);
        active++;
        maximum = Math.max(maximum, active);
        if (id === '1') {
          entered.resolve();
          await held.promise;
        }
        active--;
        return [[fixed]];
      },
      updatesBetween: async (_connection, id) => {
        return [[]];
      },
    },
    store,
  );
  const restore = snapshots.restore.bind(snapshots);
  snapshots.restore = async (connection, room) => {
    const doc = await restore(connection, room);
    validated.add(room.id);
    return doc;
  };
  const verification = snapshots.verifyRooms();
  try {
    await entered.promise;
    await new Promise((done) => setImmediate(done));
    assert.equal(visited.has('2'), true, '另一个房间不应等待首个房间的独立读 I/O');
    assert.equal(store.roomsRecovered, false, '全部恢复前不能 READY');
    assert.ok(maximum <= 4, '限制在四个恢复通道，避免同时载入全部正文');
    held.resolve();
    await verification;
    assert.deepEqual([...validated].sort(), ['1', '2', '3', '4', '5', '6']);
    assert.equal(store.roomsRecovered, true);
  } finally {
    held.resolve();
    await verification.catch(() => {});
  }
});

test('startup unit: one replay failure waits for active work, preserves checkpoint-before-log order and never READY', async () => {
  const held = Promise.withResolvers(),
    entered = Promise.withResolvers();
  const fixed = checkpoint(),
    visited = new Set(),
    logReads = new Set();
  const store = { pool: {}, roomsRecovered: false };
  const snapshots = new RoomSnapshots(
    {
      listRooms: async () => [rooms(6)],
      latestCheckpoint: async (_connection, id) => {
        visited.add(id);
        if (id === '1') {
          entered.resolve();
          await held.promise;
        }
        return [[id === '2' ? { ...fixed, checksum: Buffer.alloc(32) } : fixed]];
      },
      updatesBetween: async (_connection, id) => {
        logReads.add(id);
        return [[]];
      },
    },
    store,
  );
  const verification = snapshots.verifyRooms();
  let settled = false;
  verification.then(
    () => {
      settled = true;
    },
    () => {
      settled = true;
    },
  );
  try {
    await entered.promise;
    await new Promise((done) => setImmediate(done));
    assert.equal(visited.has('2'), true);
    assert.equal(settled, false, '失败时先等待已经开始的恢复释放自身资源');
    assert.equal(logReads.has('2'), false, '损坏 checkpoint 先拒绝，不读后续日志掩盖');
    held.resolve();
    await assert.rejects(verification, /CHECKPOINT_INVALID/);
    assert.equal(store.roomsRecovered, false);
    assert.equal(visited.has('5'), false, '初始四项中发现失败后不再启动新的房间');
  } finally {
    held.resolve();
    await verification.catch(() => {});
  }
});

test('startup unit: an actual durable log gap in a retained retired room is still fatal', async () => {
  const fixed = checkpoint(),
    retained = rooms(2);
  retained[1].last_durable_seq = '1';
  const store = { pool: {}, roomsRecovered: false };
  const snapshots = new RoomSnapshots(
    {
      listRooms: async () => [retained],
      latestCheckpoint: async () => [[fixed]],
      updatesBetween: async () => [[]],
    },
    store,
  );
  await assert.rejects(snapshots.verifyRooms(), /DURABLE_LOG_GAP/);
  assert.equal(store.roomsRecovered, false);
});

test('startup unit: a verified checkpoint at the exact cut needs no empty-range log query', async () => {
  // 可靠检查点已覆盖同一cut时后续日志区间为空；仍经真实Yjs/checksum/正文校验。
  // 空区间不应依赖另一轮SQL，也不能把未覆盖的真实尾部当作空区间。
  const fixed = checkpoint(),
    store = { pool: {}, roomsRecovered: false };
  let reads = 0;
  const snapshots = new RoomSnapshots(
    {
      listRooms: async () => [rooms(2)],
      latestCheckpoint: async () => [[fixed]],
      updatesBetween: async () => {
        reads++;
        throw Error('LOG_READ_UNAVAILABLE');
      },
    },
    store,
  );
  await snapshots.verifyRooms();
  assert.equal(reads, 0);
  assert.equal(store.roomsRecovered, true);
  const uncovered = { ...rooms(1)[0], last_durable_seq: '1' };
  await assert.rejects(snapshots.restore(store.pool, uncovered), /LOG_READ_UNAVAILABLE/);
  assert.equal(reads, 1);
});

test('startup unit: a large retained set uses four bounded I/O slots before awaiting any one room', async () => {
  const held = Promise.withResolvers(),
    entered = Promise.withResolvers(),
    fixed = checkpoint();
  const visited = new Set(),
    store = { pool: {}, roomsRecovered: false };
  let active = 0,
    maximum = 0;
  const snapshots = new RoomSnapshots(
    {
      listRooms: async () => [rooms(12)],
      latestCheckpoint: async (_connection, id) => {
        visited.add(id);
        active++;
        maximum = Math.max(maximum, active);
        if (visited.size === 4) entered.resolve();
        await held.promise;
        active--;
        return [[fixed]];
      },
      updatesBetween: async () => [[]],
    },
    store,
  );
  const verification = snapshots.verifyRooms();
  let deadline;
  try {
    await Promise.race([
      entered.promise,
      new Promise((_, reject) => {
        deadline = setTimeout(() => reject(Error('ROOM_RECOVERY_IO_SERIALIZED')), 250);
      }),
    ]);
    assert.equal(visited.size, 4, '全部正文不得同时入内存');
    assert.equal(maximum, 4);
    assert.equal(store.roomsRecovered, false);
    held.resolve();
    await verification;
    assert.equal(visited.size, 12);
    assert.equal(store.roomsRecovered, true);
  } finally {
    clearTimeout(deadline);
    held.resolve();
    await verification.catch(() => {});
  }
});
