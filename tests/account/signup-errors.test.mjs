import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createHash, createHmac, randomUUID } from 'node:crypto';
import { state } from '../../scripts/runtime-lib.mjs';
import { redisCommand } from '../../scripts/redis-client.mjs';
import { request } from '../helpers/https.mjs';

// 复用专用体验账号，独立构造测试挑战，不发送真实邮件，也不创建或修改已有用户。
// 错误验证码先被拒绝；只有完成邮箱验证后才能明确提示邮箱已注册。
test('verified duplicate signup is a stable email conflict and leaves the existing login unchanged', async () => {
  assert.ok(process.env.COLLABORATION_TEST_ACCOUNT, '专用体验账号配置路径必填');
  const actor = JSON.parse(readFileSync(process.env.COLLABORATION_TEST_ACCOUNT, 'utf8'));
  const env = state();
  const email = actor.email.trim().toLowerCase();
  const challenge = randomUUID();
  const key = 'acct:challenge:' + challenge;
  const code = '493826';
  // ChallengeMeta的JSON字段沿用服务端结构，密钥及验证码只在进程内使用，禁止打印。
  const metadata = {
    purpose: 'SIGNUP',
    subject: createHash('sha256').update(email).digest('hex'),
    user_id: '0',
    epoch: '1',
  };
  const digest = createHmac('sha256', Buffer.from(env.passwords.challengeKey, 'hex'))
    .update(challenge + '\n' + code)
    .digest('hex');
  const redis = (commands) =>
    redisCommand(env.ports.redis, 'bootstrap_admin', env.passwords.redisAdmin, commands);
  const post = (path, input) =>
    request(env.ports.edge, path, {
      method: 'POST',
      headers: {
        origin: 'https://localhost:' + env.ports.edge,
        'content-type': 'application/json',
      },
      body: JSON.stringify(input),
    });
  try {
    await redis(['HSET', key, 'digest', digest, 'meta', JSON.stringify(metadata), 'attempts', '0']);
    await redis(['EXPIRE', key, '60']);
    const input = {
      email: email.toUpperCase(),
      password: 'duplicate-signup-must-not-replace-password',
      display_name: '重复注册错误验证',
      challenge_id: challenge,
      code,
    };
    const invalid = await post('/v1/account/signup', {
      ...input,
      code: '000000',
      operation_id: randomUUID(),
    });
    assert.equal(invalid.status, 400);
    assert.equal(JSON.parse(invalid.body).code, 'CHALLENGE_INVALID');
    for (let attempt = 0; attempt < 2; attempt++) {
      const duplicate = await post('/v1/account/signup', { ...input, operation_id: randomUUID() });
      assert.equal(duplicate.status, 409);
      assert.equal(JSON.parse(duplicate.body).code, 'EMAIL_ALREADY_REGISTERED');
    }
    const login = await post('/v1/account/login', { email, password: actor.password });
    assert.equal(login.status, 200, '重复注册不能覆盖原密码或使原账号不可登录');
  } finally {
    await redis(['DEL', key]);
  }
});
