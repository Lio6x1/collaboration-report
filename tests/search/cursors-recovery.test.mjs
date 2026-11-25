import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createHash, randomBytes } from 'node:crypto';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { root, runtime, state, pause } from '../../scripts/runtime-lib.mjs';
import { stopOwnedRole, restartOwnedSearch } from '../helpers/owned-runtime.mjs';
import { searchDatabase } from '../helpers/search.mjs';

function documentClient() {
  const api = grpc.loadPackageDefinition(
    loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), {
      longs: String,
      enums: String,
      defaults: true,
      keepCase: true,
    }),
  ).collaboration.v1;
  return new api.SearchInternal(
    '127.0.0.1:' + state().ports.searchRpc,
    grpc.credentials.createSsl(
      readFileSync(resolve(runtime, 'pki/ca.pem')),
      readFileSync(resolve(runtime, 'pki/document-key.pem')),
      readFileSync(resolve(runtime, 'pki/document.pem')),
    ),
    { 'grpc.ssl_target_name_override': 'localhost' },
  );
}

function call(client, method, request) {
  return new Promise((done, reject) =>
    client[method](request, { deadline: Date.now() + 3000 }, (error, reply) =>
      error ? reject(error) : done(reply),
    ),
  );
}

function replay(anchor) {
  // 测试以协议公开framing选择REPLAY，不自行签发/修改锚点；真实Go端仍完整验签。
  const header = Buffer.from([83, 67, 69, 1, 2, 0, 0]);
  header.writeUInt16BE(anchor.length, 5);
  return Buffer.concat([header, anchor]);
}

test(
  '真实Search进程退出：已确认批按原锚点重放，持久在途链关闭且原绝对期限释放',
  { timeout: 90000 },
  async () => {
    const database = await searchDatabase();
    let client,
      stopped = false;
    const fixtures = [];
    try {
      client = documentClient();
      for (let i = 0; i < 2; i++) {
        const query = 'cursorrestartfixture' + randomBytes(12).toString('hex');
        const request = {
          query,
          query_digest: createHash('sha256').update(query).digest('hex'),
          page_size: 100,
          scope: {
            anonymous: true,
            scope_hash: createHash('sha256')
              .update('scope/' + query)
              .digest('hex'),
          },
        };
        const reply = await call(client, 'SearchCandidates', request);
        assert.equal(reply.complete, true);
        assert.equal(reply.candidates.length, 0);
        assert.equal(reply.has_more, false);
        assert.equal(reply.continuation.subarray(0, 4).toString('hex'), '53434201');
        const hash = createHash('sha256').update(reply.continuation.subarray(4, 36)).digest();
        fixtures.push({ request, reply, hash });
      }
      // 停止的是三项归属证明一致的真实进程。第二链的持久在途事实由精确SQL夹具
      // 构造，不声称是已捕获的ES响应/COMMIT丢失窗口；恢复器必须只读原持久事实。
      stopOwnedRole('search');
      stopped = true;
      client.close();
      client = undefined;
      const pending = fixtures[1];
      const control = {
        version: 1,
        phase: 'INFLIGHT',
        tail: null,
        nonce: randomBytes(32).toString('hex'),
        deadline: Date.now() + 250,
      };
      const [updated] = await database.execute(
        "UPDATE cursor_registry SET last_sort=? WHERE cursor_hash=? AND state='OPEN'",
        [JSON.stringify(control), pending.hash],
      );
      assert.equal(updated.affectedRows, 1);
      await pause(270);
      await restartOwnedSearch();
      stopped = false;
      client = documentClient();
      const confirmed = fixtures[0];
      const again = await call(client, 'SearchCandidates', {
        ...confirmed.request,
        cursor: replay(confirmed.reply.continuation),
      });
      assert.deepEqual(again, confirmed.reply);
      const [facts] = await database.execute(
        "SELECT state,CAST(UNIX_TIMESTAMP(expires_at)*1000 AS UNSIGNED) expiry,JSON_EXTRACT(last_sort,'$.uncertain') uncertain FROM cursor_registry WHERE cursor_hash=?",
        [pending.hash],
      );
      assert.equal(facts.length, 1);
      assert.equal(facts[0].state, 'CLOSING');
      assert.equal(String(facts[0].expiry), pending.reply.expires_unix_ms);
      assert.equal(Number(facts[0].uncertain), 1);
      await assert.rejects(
        call(client, 'SearchCandidates', {
          ...pending.request,
          cursor: replay(pending.reply.continuation),
        }),
        (error) => error.code === grpc.status.FAILED_PRECONDITION,
      );
      // 只等待真实原30秒期限；没有SQL提前到期、PIT续命或修改worker/reaper参数。
      await pause(Math.max(0, Number(pending.reply.expires_unix_ms) - Date.now()) + 20);
      const deadline = Date.now() + 3000;
      let unreleased = 1;
      while (Date.now() < deadline) {
        const [rows] = await database.execute(
          "SELECT COUNT(*) count FROM cursor_registry WHERE cursor_hash=? AND state<>'CLOSED'",
          [pending.hash],
        );
        unreleased = Number(rows[0].count);
        if (!unreleased) break;
        await pause(50);
      }
      assert.equal(unreleased, 0, 'REAL_RESTART_UNKNOWN_CHAIN_NOT_RELEASED');
    } finally {
      try {
        if (stopped) await restartOwnedSearch();
        client?.close();
        // 仅删除两个准确hash；故障时尚未到期的自有链先请求真实关闭，不能删OPEN冒充释放。
        const cleanup = documentClient();
        try {
          for (const fixture of fixtures) {
            const [rows] = await database.execute(
              'SELECT state FROM cursor_registry WHERE cursor_hash=?',
              [fixture.hash],
            );
            if (
              rows.length &&
              rows[0].state !== 'CLOSED' &&
              Date.now() < Number(fixture.reply.expires_unix_ms)
            ) {
              const closed = await call(cleanup, 'CloseCursor', {
                cursor: fixture.reply.continuation,
                scope: fixture.request.scope,
              });
              if (closed.state !== 'SUCCEEDED') {
                // PROCESSING表示未知句柄仍占额度，失败清理也不能用DELETE跳过原期限。
                await pause(Math.max(0, Number(fixture.reply.expires_unix_ms) - Date.now()) + 20);
              }
            }
            const until = Date.now() + 3000;
            let released = false;
            while (Date.now() < until) {
              const [current] = await database.execute(
                'SELECT state FROM cursor_registry WHERE cursor_hash=?',
                [fixture.hash],
              );
              if (!current.length || current[0].state === 'CLOSED') {
                released = true;
                break;
              }
              await pause(50);
            }
            assert.equal(released, true, 'CURSOR_CLEANUP_MUST_KEEP_UNRELEASED_FACT');
            await database.execute(
              "DELETE FROM cursor_registry WHERE cursor_hash=? AND state='CLOSED'",
              [fixture.hash],
            );
          }
        } finally {
          cleanup.close();
        }
      } finally {
        await database.end();
      }
    }
  },
);
