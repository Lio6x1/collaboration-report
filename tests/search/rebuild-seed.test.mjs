import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root } from '../../scripts/runtime-lib.mjs';
import { searchDatabase, searchConfiguration } from '../helpers/search.mjs';

test('原重建任务完成真实快照种子，应用与进度落定但尚不切alias', { timeout: 900000 }, async (t) => {
  const original = JSON.parse(
    readFileSync(resolve(root, 'artifacts/batches/06/rebuild-prepare-operation.json')),
  );
  const db = await searchDatabase();
  try {
    let previous = '',
      lastChange = Date.now(),
      lastReport = 0,
      row;
    for (;;) {
      [[row]] = await db.query(
        `SELECT j.state,CAST(j.id AS CHAR) id,j.target_index,j.source_index,p.snapshot_after,CAST(b.snapshot_count AS CHAR) count FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id JOIN rebuild_progress p ON p.job_id=j.id WHERE b.operation_id=?`,
        [original.operation],
      );
      assert.ok(row);
      if (row.state === 'REPLAYING') break;
      assert.equal(row.state, 'SNAPSHOT');
      if (String(row.snapshot_after) !== previous) {
        previous = String(row.snapshot_after);
        lastChange = Date.now();
      }
      assert.ok(Date.now() - lastChange < 60000, 'REBUILD_SEED_STALLED after=' + previous);
      if (Date.now() - lastReport > 30000) {
        console.log('REBUILD_SEED_PROGRESS after=' + previous);
        lastReport = Date.now();
      }
      await new Promise((r) => setTimeout(r, 1000));
    }
    const [[applied]] = await db.query(
      'SELECT COUNT(*) count FROM index_application WHERE physical_index=? AND OCTET_LENGTH(authority_digest)=32 AND authority_digest<>?',
      [row.target_index, Buffer.alloc(32)],
    );
    assert.equal(String(applied.count), row.count);
    const response = await fetch(
      searchConfiguration().search.elasticEndpoint + '/' + row.target_index + '/_count',
    );
    assert.equal(response.status, 200);
    // ES的近实时refresh可能晚于最后一次成功update，显式refresh后检查完整目标数量。
    const refreshed = await fetch(
      searchConfiguration().search.elasticEndpoint + '/' + row.target_index + '/_refresh',
      { method: 'POST' },
    );
    assert.equal(refreshed.status, 200);
    const count = await (
      await fetch(searchConfiguration().search.elasticEndpoint + '/' + row.target_index + '/_count')
    ).json();
    assert.equal(String(count.count), row.count);
    const [[active]] = await db.query(
      'SELECT physical_index FROM index_generation WHERE alias_name=?',
      [searchConfiguration().search.alias],
    );
    assert.equal(active.physical_index, row.source_index);
    const [[events]] = await db.query(
      'SELECT COUNT(*) count FROM consumed_event WHERE consumer_name=?',
      ['rebuild-' + row.id],
    );
    assert.equal(Number(events.count), 0, '种子不能伪造消息回执');
    t.diagnostic('原快照 ' + row.count + ' 条完成Doc→ES→SQL种子，原alias保留，下一阶段独立重放。');
  } finally {
    await db.end();
  }
});
