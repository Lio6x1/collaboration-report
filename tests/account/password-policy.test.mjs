import test from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { request } from '../helpers/https.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

// 通过正在运行的HTTPS服务验证后端规则，绕过浏览器也不能设置弱密码。
// 使用不存在的验证码：不发送邮件，不注册账号，不改变已有密码。
test('live signup and reset reject weak passwords before checking the challenge', async () => {
  const env = state();
  for (const path of ['/v1/account/signup', '/v1/account/password-resets']) {
    for (const [password, expected] of [
      ['abcdefgh', 'PASSWORD_POLICY_INVALID'],
      ['abc123!', 'PASSWORD_POLICY_INVALID'],
      ['Abcdef12', 'CHALLENGE_INVALID'],
    ]) {
      const input = {
        operation_id: randomUUID(),
        challenge_id: randomUUID(),
        code: '000000',
        ...(path.endsWith('/signup')
          ? { email: 'policy-check@example.test', display_name: '密码规则验证', password }
          : { new_password: password }),
      };
      const response = await request(env.ports.edge, path, {
        method: 'POST',
        headers: {
          origin: 'https://localhost:' + env.ports.edge,
          'content-type': 'application/json',
        },
        body: JSON.stringify(input),
      });
      assert.equal(response.status, 400);
      assert.equal(JSON.parse(response.body).code, expected);
    }
  }
});
