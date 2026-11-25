import assert from 'node:assert/strict';
import test from 'node:test';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { state, root, runtime, pause } from '../../scripts/runtime-lib.mjs';
import { createUser, get, post, json, operation } from '../helpers/account.mjs';
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
  return new api.DocumentInternal(
    '127.0.0.1:' + state().ports.documentRpc,
    grpc.credentials.createSsl(
      readFileSync(resolve(runtime, 'pki/ca.pem')),
      readFileSync(resolve(runtime, `pki/${role}-key.pem`)),
      readFileSync(resolve(runtime, `pki/${role}.pem`)),
    ),
    { 'grpc.ssl_target_name_override': 'localhost' },
  );
}

// 业务注销沿已经验收的HTTP协调器真正完成；三个RPC只补原权威计划的传输闭环。
// 阶段推进/未知提交反例由直接契约测试覆盖，这里不再故障重演整套注销矩阵。
test(
  'lifecycle RPCs replay the actual original plan after identity closure without changing generation',
  { timeout: 60000 },
  async () => {
    const caller = peer('account'),
      forbidden = peer('collaboration'),
      db = await reviewDatabase();
    try {
      const user = await createUser(),
        me = json(await get('/v1/account/me', user.headers));
      const id = operation('lifecycle-rpc');
      const started = await post(
        '/v1/account/closures',
        {
          operation_id: id,
          expected_identity_epoch: me.identity_epoch,
          current_password: user.password,
        },
        user.headers,
      );
      assert.equal(started.status, 202, started.body);
      let current;
      const deadline = Date.now() + 30000;
      do {
        const response = await get('/v1/account/closures/' + id, user.headers);
        assert.equal(response.status, 200, response.body);
        current = json(response);
        if (current.state === 'SUCCEEDED') break;
        await pause(100);
      } while (Date.now() < deadline);
      assert.equal(current.state, 'SUCCEEDED');
      assert.equal((await get('/v1/account/me', user.headers)).status, 401);
      const [[original]] = await db.query(
        'SELECT CAST(fixed_plan AS CHAR) plan,state FROM lifecycle_operation WHERE operation_id=?',
        [id],
      );
      assert.equal(original.state, 'SUCCEEDED');
      const exact = Buffer.from(original.plan);
      const digest = createHash('sha256').update(exact).digest();
      const input = {
        operation_id: id,
        user_id: user.id,
        closing_organization_ids: [],
        generation: '7',
        fixed_plan_digest: Buffer.alloc(0),
      };
      const reserved = await rpcCall(caller, 'ReserveAdminRetirement', input);
      const preparedInput = { ...input, fixed_plan_digest: reserved.plan_digest };
      const prepared = await rpcCall(caller, 'PrepareLifecyclePlan', preparedInput);
      const appliedInput = { ...input, fixed_plan_digest: prepared.plan_digest };
      const applied = await rpcCall(caller, 'ApplyLifecyclePlan', appliedInput);
      for (const result of [reserved, prepared, applied]) {
        assert.equal(result.operation_id, id);
        assert.equal(result.generation, '7');
        assert.equal(result.phase, 'SUCCEEDED');
        assert.deepEqual(result.plan_json_utf8, exact);
        assert.deepEqual(result.plan_digest, digest);
      }
      assert.deepEqual(await rpcCall(caller, 'ReserveAdminRetirement', input), reserved);
      assert.deepEqual(await rpcCall(caller, 'PrepareLifecyclePlan', preparedInput), prepared);
      assert.deepEqual(await rpcCall(caller, 'ApplyLifecyclePlan', appliedInput), applied);
      for (const [method, request] of [
        ['ReserveAdminRetirement', { ...input, generation: '8' }],
        ['PrepareLifecyclePlan', { ...preparedInput, fixed_plan_digest: Buffer.alloc(32) }],
        ['ApplyLifecyclePlan', { ...appliedInput, closing_organization_ids: ['1'] }],
      ])
        await assert.rejects(
          rpcCall(caller, method, request),
          (error) => error.code === grpc.status.ALREADY_EXISTS,
        );
      await assert.rejects(
        rpcCall(forbidden, 'ReserveAdminRetirement', input),
        (error) => error.code === grpc.status.PERMISSION_DENIED,
      );
      const [[after]] = await db.query(
        'SELECT CAST(fixed_plan AS CHAR) plan,state FROM lifecycle_operation WHERE operation_id=?',
        [id],
      );
      assert.deepEqual(after, original);
      console.log(
        'LIFECYCLE_RPC_REAL_PASS: original closed identity; exact full plan/digest; independent generation; three terminal replays and rejected mutations',
      );
    } finally {
      caller.close();
      forbidden.close();
      await db.end();
    }
  },
);
