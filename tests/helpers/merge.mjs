import assert from 'node:assert/strict';
import { get, json } from './workspace.mjs';

export async function pollMerge(id, headers, db) {
  const deadline = Date.now() + 30000;
  let value;
  do {
    const response = await get(`/v1/merge-operations/${id}`, headers);
    assert.equal(response.status, 200, json(response).code);
    value = json(response);
    if (value.state !== 'PREPARING' && value.result?.main_sync_state !== 'SYNCING') return value;
    await new Promise((resolve) => setTimeout(resolve, 100));
  } while (Date.now() < deadline);
  const [tasks] = await db.query(
    "SELECT kind,state,last_error,attempts FROM durable_task WHERE operation_id=? AND state NOT IN ('SUCCEEDED','REJECTED')",
    [id],
  );
  assert.fail('MERGE_NOT_READY ' + JSON.stringify({ value, tasks }));
}
