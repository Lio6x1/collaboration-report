import test from 'node:test';
import assert from 'node:assert/strict';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root, runtime, state, pause } from '../../scripts/runtime-lib.mjs';
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
const sha = (bytes) => createHash('sha256').update(bytes).digest();
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
function canonical(value) {
  if (typeof value === 'number')
    assert.ok(Number.isSafeInteger(value), 'FIXTURE_JSON_INTEGER_MUST_BE_EXACT');
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object')
    return Object.fromEntries(
      Object.keys(value)
        .sort()
        .map((key) => [key, canonical(value[key])]),
    );
  return value;
}

// 反向RPC只证明已有Account关闭事实；实际退出/注销由原业务协调器完成。
// 读取真实Doc ACK、父receipt与原完整目标，不手写门闩/终态或把空ACK算成功。
test(
  'account authorization RPCs prove original logout and account-child scopes with exact target and ACK',
  { timeout: 60000 },
  async () => {
    const caller = peer('document'),
      forbidden = peer('collaboration'),
      db = await reviewDatabase('account');
    try {
      for (const kind of ['LOGOUT', 'ACCOUNT_CLOSE']) {
        const user = await createUser(),
          id = operation('authorization-rpc');
        if (kind === 'LOGOUT') {
          const response = await post('/v1/account/logout', { operation_id: id }, user.headers);
          assert.equal(response.status, 202, response.body);
          assert.equal((await waitOperation(response, user)).state, 'SUCCEEDED');
        } else {
          const me = json(await get('/v1/account/me', user.headers));
          const response = await post(
            '/v1/account/closures',
            {
              operation_id: id,
              expected_identity_epoch: me.identity_epoch,
              current_password: user.password,
            },
            user.headers,
          );
          assert.equal(response.status, 202, response.body);
          let completed;
          const deadline = Date.now() + 30000;
          do {
            const progress = await get('/v1/account/closures/' + id, user.headers);
            assert.equal(progress.status, 200, progress.body);
            completed = json(progress);
            if (completed.state === 'SUCCEEDED') break;
            await pause(100);
          } while (Date.now() < deadline);
          assert.equal(completed.state, 'SUCCEEDED');
        }
        const [facts] = await db.query(
          `SELECT a.revocation_id,a.scope_key,CAST(a.epoch AS CHAR) epoch,a.target_permission,
          t.operation_id receipt_operation_id,o.target_json,o.request_fingerprint,k.result_digest,k.completed_at
          FROM authorization_change a JOIN durable_task t ON t.business_key=a.revocation_id
          AND t.kind IN ('SESSION_LOGOUT','ACCOUNT_CLOSE_FENCE') JOIN operation_receipt o ON o.operation_id=t.operation_id
          JOIN authorization_ack k ON k.revocation_id=a.revocation_id AND k.participant='document' AND k.scope_key=a.scope_key
          WHERE t.operation_id=? AND a.state='SUCCEEDED'`,
          [id],
        );
        assert.ok(facts.length > 0, 'ORIGINAL_AUTHORIZATION_FACTS_REQUIRED');
        for (const fact of facts) {
          assert.ok(fact.completed_at);
          const targetDigest = fact.scope_key.startsWith('session:')
            ? sha('SESSION_LOGOUT\n' + fact.scope_key.slice(8))
            : sha('AUTHORIZATION_CHANGE\n' + fact.scope_key + '\n' + fact.epoch);
          const fence = {
            revocation_id: fact.revocation_id,
            closed_scopes: [{ scope_key: fact.scope_key, epoch: fact.epoch }],
            generation: '1',
            target_digest: targetDigest,
          };
          const ack = await rpcCall(caller, 'FenceAccess', fence);
          assert.equal(ack.state, 'ACKNOWLEDGED');
          assert.deepEqual(
            ack.result_digest,
            sha(
              'bootstrap-fence-v1\naccount\n' +
                fact.revocation_id +
                '\n' +
                fact.scope_key +
                '\n' +
                fact.epoch +
                '\n1\n' +
                targetDigest.toString('hex'),
            ),
          );
          assert.deepEqual(await rpcCall(caller, 'FenceAccess', fence), ack);
          const binding = {
            receipt_operation_id: fact.receipt_operation_id,
            authorization_target: canonical(fact.target_permission),
            operation_target: canonical(fact.target_json),
          };
          const query = {
            operation_id: fact.revocation_id,
            request_fingerprint: fact.request_fingerprint,
            target: {
              document_id: '0',
              unit_id: '0',
              branch_id: '0',
              structure_id: '0',
              workspace_epoch: fact.epoch,
              purpose:
                'ACCOUNT_AUTHORIZATION_CHANGE:' +
                fact.scope_key +
                ':' +
                sha(JSON.stringify(binding)).toString('hex'),
            },
          };
          const result = await rpcCall(caller, 'GetAuthorizationChange', query);
          assert.equal(result.state, 'SUCCEEDED');
          assert.equal(result.generation, '1');
          assert.equal(result.terminal_revision, '1');
          assert.deepEqual(result.result_digest, sha(result.fixed_result_json_utf8));
          assert.deepEqual(await rpcCall(caller, 'GetAuthorizationChange', query), result);
          await assert.rejects(
            rpcCall(caller, 'GetAuthorizationChange', {
              ...query,
              target: { ...query.target, document_id: '1' },
            }),
            (error) => error.code === grpc.status.INVALID_ARGUMENT,
          );
          await assert.rejects(
            rpcCall(caller, 'GetAuthorizationChange', {
              ...query,
              request_fingerprint: Buffer.alloc(32),
            }),
            (error) => error.code === grpc.status.INVALID_ARGUMENT,
          );
          await assert.rejects(
            rpcCall(forbidden, 'FenceAccess', fence),
            (error) => error.code === grpc.status.PERMISSION_DENIED,
          );
          await assert.rejects(
            rpcCall(caller, 'FenceAccess', { ...fence, generation: '2' }),
            (error) => error.code === grpc.status.INVALID_ARGUMENT,
          );
        }
        console.log('AUTHORIZATION_RPC_REAL_PASS: ' + kind + ' original scopes=' + facts.length);
      }
    } finally {
      caller.close();
      forbidden.close();
      await db.end();
    }
  },
);
