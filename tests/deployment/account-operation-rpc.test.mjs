import assert from 'node:assert/strict';
import test from 'node:test';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { state, root, runtime, pause } from '../../scripts/runtime-lib.mjs';
import { createUser, get, post, json, operation, waitOperation } from '../helpers/account.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';

const api = grpc.loadPackageDefinition(
  loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), {
    longs: String,
    enums: String,
    defaults: true,
    keepCase: true,
  }),
).collaboration.v1;
function peer(role) {
  return new api.IdentityInternal(
    '127.0.0.1:' + state().ports.accountRpc,
    grpc.credentials.createSsl(
      readFileSync(resolve(runtime, 'pki/ca.pem')),
      readFileSync(resolve(runtime, `pki/${role}-key.pem`)),
      readFileSync(resolve(runtime, `pki/${role}.pem`)),
    ),
    { 'grpc.ssl_target_name_override': 'localhost' },
  );
}
function token(user) {
  const value = /(?:^|;\s*)__Host-collabSession=([^;]+)/.exec(user.headers.cookie)?.[1];
  assert.ok(value, 'REAL_SESSION_COOKIE_REQUIRED');
  return Buffer.from(value);
}

// 新增RPC只补传输/原结果投影；用真实退出及空账号关闭证明恢复秘密的权限，
// 不再重跑既有关闭故障矩阵，也不以SQL人工成功或哈希冒充会话秘密。
test(
  'account operation RPC queries original logout and closure with exact recovery authority',
  { timeout: 60000 },
  async () => {
    const account = peer('document'),
      forbidden = peer('collaboration'),
      db = await reviewDatabase('account');
    try {
      const outsider = await createUser();
      for (const kind of ['LOGOUT', 'ACCOUNT_CLOSE']) {
        const user = await createUser(),
          originalToken = token(user),
          id = operation('rpc-account-result');
        if (kind === 'LOGOUT') {
          const response = await post('/v1/account/logout', { operation_id: id }, user.headers);
          assert.equal(response.status, 202, response.body);
          assert.equal((await waitOperation(response, user)).state, 'SUCCEEDED');
        } else {
          const me = await get('/v1/account/me', user.headers);
          assert.equal(me.status, 200, me.body);
          const response = await post(
            '/v1/account/closures',
            {
              operation_id: id,
              expected_identity_epoch: json(me).identity_epoch,
              current_password: user.password,
            },
            user.headers,
          );
          assert.equal(response.status, 202, response.body);
          let result;
          const deadline = Date.now() + 30000;
          do {
            const current = await get('/v1/account/closures/' + id, user.headers);
            assert.equal(current.status, 200, current.body);
            result = json(current);
            if (result.state === 'SUCCEEDED') break;
            await pause(100);
          } while (Date.now() < deadline);
          assert.equal(result.state, 'SUCCEEDED');
        }
        assert.equal((await get('/v1/account/me', user.headers)).status, 401);
        const input = {
          operation_id: id,
          session_token: Buffer.alloc(0),
          recovery_receipt: originalToken,
        };
        const result = await rpcCall(account, 'GetAccountOperation', input);
        assert.equal(result.state, 'SUCCEEDED');
        assert.equal(result.operation_id, id);
        assert.deepEqual(JSON.parse(result.fixed_result_json_utf8), {
          operation_id: id,
          state: 'SUCCEEDED',
        });
        assert.deepEqual(
          result.result_digest,
          createHash('sha256').update(result.fixed_result_json_utf8).digest(),
        );
        const [[receipt]] = await db.query(
          'SELECT CAST(generation AS CHAR) generation,terminal_at FROM operation_receipt WHERE operation_id=?',
          [id],
        );
        assert.ok(receipt.terminal_at);
        assert.equal(result.generation, receipt.generation);
        assert.equal(result.terminal_revision, receipt.generation);
        const replay = await rpcCall(account, 'GetAccountOperation', input);
        assert.deepEqual(replay, result);
        await assert.rejects(
          rpcCall(account, 'GetAccountOperation', { ...input, recovery_receipt: token(outsider) }),
          (error) => error.code === grpc.status.NOT_FOUND,
        );
        await assert.rejects(
          rpcCall(forbidden, 'GetAccountOperation', input),
          (error) => error.code === grpc.status.PERMISSION_DENIED,
        );
        console.log('ACCOUNT_RPC_REAL_PASS: ' + kind);
      }
    } finally {
      account.close();
      forbidden.close();
      await db.end();
    }
  },
);
