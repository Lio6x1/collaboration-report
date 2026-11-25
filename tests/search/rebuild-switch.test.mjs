import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { root, runtime, pause } from '../../scripts/runtime-lib.mjs';
import {
  searchDatabase,
  searchConfiguration,
  waitSearchReceipt,
  searchProjection,
} from '../helpers/search.mjs';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';

test(
  '原重建任务完整核验后切换，固定结果可重复查询且新发布进入新代次',
  { timeout: 300000 },
  async (t) => {
    const original = JSON.parse(
      readFileSync(resolve(root, 'artifacts/batches/06/rebuild-prepare-operation.json')),
    );
    const db = await searchDatabase();
    try {
      let row,
        last = '',
        changed = Date.now(),
        reported = 0;
      for (;;) {
        [[row]] = await db.query(
          `SELECT j.state,j.source_index,j.target_index,CAST(j.generation AS CHAR) generation,
        CAST(p.revision AS CHAR) revision,HEX(p.validation_digest) digest,p.frontier_payload
        FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id JOIN rebuild_progress p ON p.job_id=j.id WHERE b.operation_id=?`,
          [original.operation],
        );
        assert.ok(row);
        if (row.state === 'SUCCEEDED') break;
        assert.ok(['REPLAYING', 'SWITCHING'].includes(row.state));
        const progress = row.state + ':' + row.revision;
        if (progress !== last) {
          last = progress;
          changed = Date.now();
        }
        assert.ok(Date.now() - changed < 60000, 'REBUILD_SWITCH_STALLED ' + progress);
        if (Date.now() - reported > 30000) {
          console.log('REBUILD_SWITCH_PROGRESS ' + progress);
          reported = Date.now();
        }
        await pause(500);
      }
      assert.match(row.digest, /^[0-9A-F]{64}$/);
      const [[active]] = await db.query(
        'SELECT physical_index,CAST(generation AS CHAR) generation FROM index_generation WHERE alias_name=?',
        [searchConfiguration().search.alias],
      );
      assert.equal(active.physical_index, row.target_index);
      assert.equal(active.generation, row.generation);
      const { search } = searchConfiguration();
      const aliases = await (
        await fetch(search.elasticEndpoint + '/_alias/' + search.alias)
      ).json();
      assert.deepEqual(Object.keys(aliases), [row.target_index]);
      assert.equal(
        (await fetch(search.elasticEndpoint + '/' + row.source_index, { method: 'HEAD' })).status,
        200,
      );
      // 通过Doc现有mTLS CLI读取同一操作，不能直接SQL拼出一次虚假的RPC成功。
      const command = promisify(execFile);
      const args = [
        '--config',
        resolve(runtime, 'services/document.json'),
        '--operation',
        original.operation,
        '--snapshot-ref',
        original.snapshot,
        '--action',
        'status',
      ];
      const status = async () => {
        try {
          return JSON.parse(
            (
              await command(resolve(runtime, 'bin/search-rebuild.exe'), args, {
                cwd: root,
                windowsHide: true,
                timeout: 30000,
              })
            ).stdout,
          );
        } catch {
          assert.fail('REBUILD_TERMINAL_RPC_UNAVAILABLE');
        }
      };
      const first = await status();
      assert.equal(first.state, 'SUCCEEDED');
      assert.equal(first.generation, row.generation);
      assert.ok(first.result_digest && first.fixed_result_json_utf8);
      assert.deepEqual(await status(), first);
      const fixture = await organizationFixture(0);
      const document = await documentFixture(fixture);
      const input = await releaseInput(document.id, fixture.owner);
      const publication = await publishRelease(document.id, fixture.owner, input);
      const receipt = await waitSearchReceipt(db, input.operation_id);
      assert.equal(receipt.physical, row.target_index);
      assert.equal(
        (await searchProjection(document.id, row.target_index)).release_id,
        publication.result.entity_id,
      );
      t.diagnostic(
        '原任务完成核验、alias与SQL代次切换、固定结果重复查询；新发布已进入目标，旧索引保留。',
      );
    } finally {
      await db.end();
    }
  },
);
