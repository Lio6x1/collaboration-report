import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import { createStore } from '../src/bootstrap/store.mjs';
import { RoomEngine } from '../src/rooms.mjs';
const config = JSON.parse(
  readFileSync(new URL('../../../.runtime/services/collaboration.json', import.meta.url)),
);
test('Node ready rejects a different installed-schema expectation', async () => {
  const store = createStore({ ...config, expectedSQLSHA: '0'.repeat(64) });
  try {
    await assert.rejects(store.checkReady(), /SCHEMA_VERSION_MISMATCH/);
  } finally {
    await store.pool.end();
  }
});
test('Node ready rejects unrecovered durable tasks', async () => {
  const store = createStore(config),
    key = 'node-ready-test-' + randomUUID();
  try {
    // 真实持久UNKNOWN记录是恢复障碍；移除恢复检查应使本例失败。
    await store.pool.query(
      "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('BOOTSTRAP_TEST',?,'{}','UNKNOWN',UTC_TIMESTAMP(6))",
      [key],
    );
    await assert.rejects(store.checkReady(), /RECOVERY_REQUIRED/);
  } finally {
    await store.pool.query(
      "DELETE FROM durable_task WHERE kind='BOOTSTRAP_TEST' AND business_key=?",
      [key],
    );
    await store.pool.end();
  }
});
test('Node ready accepts the current baseline only after actual durable room verification', async () => {
  const store = createStore(config);
  try {
    await new RoomEngine(store, config, null).verifyRooms();
    await store.checkReady();
  } finally {
    await store.pool.end();
  }
});
test('Node ready refuses unrestored blocked gate independently of tasks', async () => {
  const store = createStore(config),
    key = 'bootstrap-gate-' + randomUUID();
  try {
    await store.pool.query("INSERT INTO access_gate(scope_key,state) VALUES(?,'BLOCKED')", [key]);
    await assert.rejects(store.checkReady(), /GATE_RECOVERY_REQUIRED/);
  } finally {
    await store.pool.query('DELETE FROM access_gate WHERE scope_key=?', [key]);
    await store.pool.end();
  }
});
test('Node ready restores a terminal persisted closure without reopening its gate', async () => {
  const store = createStore(config),
    key = 'known-closed-' + randomUUID(),
    id = 'logout-' + randomUUID();
  try {
    await store.pool.query(
      "INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,1,'{}','SUCCEEDED','[]')",
      [id, key],
    );
    await store.pool.query(
      "INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,1,'BLOCKED',?)",
      [key, id],
    );
    await new RoomEngine(store, config, null).verifyRooms();
    await store.checkReady();
    const [[persisted]] = await store.pool.query(
      'SELECT state,max_epoch FROM access_gate WHERE scope_key=?',
      [key],
    );
    assert.equal(persisted.state, 'BLOCKED');
    assert.equal(String(persisted.max_epoch), '1');
  } finally {
    await store.pool.query('DELETE FROM access_gate WHERE scope_key=?', [key]);
    await store.pool.query('DELETE FROM authorization_change WHERE revocation_id=?', [id]);
    await store.pool.end();
  }
});
