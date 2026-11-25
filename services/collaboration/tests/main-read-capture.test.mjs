import test from 'node:test';
import assert from 'node:assert/strict';
import { MainCaptures } from '../src/application/main_workflows.mjs';
import { RPCFailure } from '../src/domain/shared_protocol.mjs';

const input = (purpose) => ({
  context: {
    operation_id: 'main-reading-original-0001',
    target: { purpose },
    request_fingerprint: Buffer.alloc(32),
  },
  usage: 'MAIN_READ_VIEW:1',
});

// 不属于协议的用途须明确INVALID，不能因为Object原型上的同名属性触发TypeError或进入SQL。
test('MAIN capture purpose validation rejects inherited object names and unknown usages', async () => {
  const service = new MainCaptures(
    {
      run: () => {
        assert.fail('invalid purpose reached SQL');
      },
    },
    null,
    null,
  );
  for (const purpose of ['__proto__', 'constructor', 'toString', 'UNKNOWN', undefined])
    await assert.rejects(
      service.capture(input(purpose)),
      (error) => error.code === 3 && error.message === 'MAIN_CAPTURE_INVALID',
    );
  for (const usage of ['MAIN_CONTROL:1', 'MAIN_READ_VIEW:2', 'MAIN_READ_VIEW:01'])
    await assert.rejects(
      service.capture({ ...input('READ_MAIN'), usage }),
      (error) => error.code === 3,
    );
});

test('a stored readonly capture still requires its original admission while write capture replay keeps its existing semantics', async () => {
  const prior = { state: 'FIXED' },
    failure = new RPCFailure(7, 'MAIN_READ_EXPIRED');
  let checks = 0;
  const service = new MainCaptures({ run: async () => prior }, null, {
    http: async () => {
      checks++;
      throw failure;
    },
  });
  await assert.rejects(service.capture(input('READ_MAIN')), (error) => error === failure);
  assert.equal(checks, 1);
  const merge = { ...input('MERGE'), usage: 'MERGE_ATTEMPT:1' };
  assert.equal(await service.capture(merge), prior);
  assert.equal(checks, 1, '后台写操作的既有固定事实重放不能被普通读取续期规则改写');
});
