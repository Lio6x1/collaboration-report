import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import test from 'node:test';
import { WebSocket } from 'ws';
import { runtime } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  post,
  get,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { waitOperation } from '../helpers/account.mjs';
import { openReviewWorkspace, reviewDatabase, writeDraft } from '../helpers/review.mjs';
import { wire } from '../helpers/wss.mjs';
import { loginAgain, sessionId, pendingTicket, until } from '../helpers/authorization-fence.mjs';
import {
  assertSourceClosure,
  assertWorkspaceClosed,
  assertNewSource,
} from '../helpers/authorization-sources.mjs';

async function verification(email, purpose, headers = {}) {
  const response = await post('/v1/account/challenges', { email, purpose }, headers);
  assert.equal(response.status, 200);
  const challenge = json(response).challenge_id;
  let proof;
  await until(() => {
    const inbox = resolve(runtime, 'smtp/inbox');
    for (const name of readdirSync(inbox)) {
      const mail = JSON.parse(readFileSync(resolve(inbox, name)));
      if (!mail.body.includes('Challenge: ' + challenge)) continue;
      const match = /Code: (\d{6})/.exec(mail.body);
      if (match) proof = { challenge_id: challenge, code: match[1] };
    }
    return Boolean(proof);
  }, 'ACTUAL_VERIFICATION_MAIL_REQUIRED');
  return proof;
}

async function publicReset(input) {
  // 无登录的密码重置没有账号进度 GET；恢复只重发完全相同的原号和验证码输入。
  // 不能把仍在关闭的原操作换号，也不能借旧 Cookie 查询另一种操作的进度。
  for (let attempt = 0; attempt < 80; attempt++) {
    const response = await post('/v1/account/password-resets', input);
    assert.ok([200, 202].includes(response.status));
    const value = json(response);
    assert.equal(value.operation_id, input.operation_id);
    if (value.state === 'SUCCEEDED') return;
    assert.equal(value.state, 'PREPARING');
    await new Promise((done) => setTimeout(done, 100));
  }
  assert.fail('ORIGINAL_RESET_NOT_TERMINAL');
}

for (const kind of ['RESET_PASSWORD', 'CHANGE_EMAIL']) {
  test(`product credentials: ${kind} closes two actual WSS sessions and reopens only the new credential epoch`, async () => {
    const fixture = await organizationFixture(0);
    const document = await documentFixture(fixture);
    const independent = await loginAgain(fixture.owner);
    const a = await openReviewWorkspace(fixture, document);
    const b = await openReviewWorkspace(fixture, document, independent);
    const ticket = await pendingTicket(document, fixture.owner);
    const revocation = operation('credential-fence');
    const scope = 'credential:' + fixture.owner.id;
    const current = { ...fixture.owner };
    let fresh, late;
    try {
      if (kind === 'RESET_PASSWORD') {
        current.password = 'reset-source-password-123';
        const proof = await verification(current.email, kind);
        await publicReset({ operation_id: revocation, ...proof, new_password: current.password });
      } else {
        current.email = operation('changed-email') + '@example.test';
        const proof = await verification(current.email, kind, fixture.owner.headers);
        await waitOperation(
          await post(
            '/v1/account/email',
            {
              operation_id: revocation,
              expected_credential_epoch: '1',
              ...proof,
              new_email: current.email,
            },
            fixture.owner.headers,
          ),
          fixture.owner,
        );
      }
      await until(
        () =>
          a.w.socket.readyState === WebSocket.CLOSED && b.w.socket.readyState === WebSocket.CLOSED,
        'ORIGINAL_CREDENTIAL_SOCKETS_NOT_CLOSED',
      );
      await assertSourceClosure(revocation, scope, '1', ['account', 'document', 'collaboration']);
      await assertWorkspaceClosed(a);
      await assertWorkspaceClosed(b);
      const database = await reviewDatabase('account');
      try {
        for (const user of [fixture.owner, independent]) {
          const [[row]] = await database.query('SELECT state FROM session WHERE id=?', [
            sessionId(user),
          ]);
          assert.equal(row?.state, 'CLOSED');
          assert.equal((await get('/v1/account/me', user.headers)).status, 401);
        }
      } finally {
        await database.end();
      }
      late = wire(ticket.session, ticket.client, operation('credential-late-ticket'));
      await assert.rejects(late.ready);
      assert.equal(
        (
          await post('/v1/account/login', {
            email: fixture.owner.email,
            password: fixture.owner.password,
          })
        ).status,
        401,
      );
      const newlyLoggedIn = await loginAgain(current);
      fresh = await openReviewWorkspace(fixture, document, newlyLoggedIn);
      await assertNewSource(fresh, scope, '1');
      assert.equal((await writeDraft(fresh, '实际验证后的新凭据代次可靠保存')).durable_seq, '1');
    } finally {
      late?.close();
      fresh?.close();
      a.close();
      b.close();
    }
  });
}
