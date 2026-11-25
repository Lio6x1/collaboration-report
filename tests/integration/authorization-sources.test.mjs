import assert from 'node:assert/strict';
import test from 'node:test';
import { WebSocket } from 'ws';
import {
  organizationFixture,
  documentFixture,
  post,
  get,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { waitOperation } from '../helpers/account.mjs';
import {
  openReviewWorkspace,
  reviewDatabase,
  writeDraft,
  collaborationRPC,
} from '../helpers/review.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import { wire } from '../helpers/wss.mjs';
import { loginAgain, pendingTicket, sessionId, until } from '../helpers/authorization-fence.mjs';
import {
  assertSourceClosure,
  assertWorkspaceClosed,
  assertNewSource,
  sourceFence,
} from '../helpers/authorization-sources.mjs';

const closed = (opened) =>
  until(() => opened.w.socket.readyState === WebSocket.CLOSED, 'ORIGINAL_SOCKET_NOT_CLOSED');
async function denyLate(ticket) {
  const late = wire(ticket.session, ticket.client, operation('late-source-connection'));
  try {
    await assert.rejects(late.ready);
  } finally {
    late.close();
  }
}
const editGrant = (member, kind, id) => ({
  user_id: member.id,
  scope_kind: kind,
  scope_id: id,
  capability: 'EDIT',
});
async function grantEdit(f, document, member) {
  await waitAuthorization(
    await post(
      '/v1/documents/' + document.id + '/authorization-changes',
      {
        operation_id: operation('source-edit-grant'),
        expected_acl_revision: '1',
        changes: [
          { action: 'GRANT', grant: editGrant(member, 'UNIT', document.main.units[0].unit_id) },
        ],
      },
      f.owner.headers,
    ),
    f.owner,
  );
}
async function changeRole(f, member, role) {
  const directory = json(
    await get('/v1/organizations/' + f.organization + '/members', f.owner.headers),
  );
  const current = directory.items.find((value) => value.user_id === member.id);
  assert.ok(current, 'ACTUAL_CURRENT_MEMBER_REQUIRED');
  const revocation = operation('source-role-change');
  await waitOperation(
    await post(
      '/v1/organizations/' + f.organization + '/members/' + member.id + '/role',
      {
        operation_id: revocation,
        expected_revision: current.revision,
        role,
      },
      f.owner.headers,
    ),
    f.owner,
  );
  return revocation;
}

test('product sources: password change closes two actual sessions and all original outputs, new credential works', async () => {
  const f = await organizationFixture(0),
    document = await documentFixture(f);
  const independent = await loginAgain(f.owner);
  const a = await openReviewWorkspace(f, document),
    b = await openReviewWorkspace(f, document, independent);
  const ticket = await pendingTicket(document, f.owner);
  const revocation = operation('source-password-change'),
    scope = 'credential:' + f.owner.id;
  let fresh;
  try {
    await waitOperation(
      await post(
        '/v1/account/password',
        {
          operation_id: revocation,
          expected_credential_epoch: '1',
          current_password: f.owner.password,
          new_password: 'actual-new-source-password-123',
        },
        f.owner.headers,
      ),
      f.owner,
    );
    await Promise.all([closed(a), closed(b)]);
    await assertSourceClosure(revocation, scope, '1', ['account', 'document', 'collaboration']);
    await assertWorkspaceClosed(a);
    await assertWorkspaceClosed(b);
    const database = await reviewDatabase('account');
    try {
      for (const user of [f.owner, independent]) {
        const [[session]] = await database.query('SELECT state FROM session WHERE id=?', [
          sessionId(user),
        ]);
        assert.equal(session.state, 'CLOSED');
        assert.equal((await get('/v1/account/me', user.headers)).status, 401);
      }
    } finally {
      await database.end();
    }
    await denyLate(ticket);
    assert.equal(
      (await post('/v1/account/login', { email: f.owner.email, password: f.owner.password }))
        .status,
      401,
    );
    const current = await loginAgain({ ...f.owner, password: 'actual-new-source-password-123' });
    fresh = await openReviewWorkspace(f, document, current);
    await assertNewSource(fresh, scope, '1');
    assert.equal((await writeDraft(fresh, '新凭据代次重新准入并可靠写入')).durable_seq, '1');
  } finally {
    fresh?.close();
    a.close();
    b.close();
  }
});

test('product sources: organization demotion closes old full grants but independent document EDIT remains usable', async () => {
  const f = await organizationFixture(1),
    member = f.members[0],
    document = await documentFixture(f);
  await changeRole(f, member, 'ADMIN');
  await grantEdit(f, document, member);
  const opened = await openReviewWorkspace(f, document, member),
    ticket = await pendingTicket(document, member);
  const scope = 'org-member:' + f.organization + ':' + member.id;
  const source = opened.grant.context.sources.find((value) => value.scope_key === scope);
  assert.ok(source, 'REAL_MEMBER_SOURCE_REQUIRED');
  let fresh;
  try {
    const revocation = await changeRole(f, member, 'MEMBER');
    await closed(opened);
    await assertSourceClosure(revocation, scope, source.epoch, [
      'account',
      'document',
      'collaboration',
    ]);
    await assertWorkspaceClosed(opened);
    await denyLate(ticket);
    fresh = await openReviewWorkspace(f, document, member);
    await assertNewSource(fresh, scope, source.epoch);
    const header = await get('/v1/documents/' + document.id, member.headers);
    assert.equal(header.status, 200);
    assert.equal(
      JSON.parse(header.headers['x-available-actions']).includes('CHANGE_AUTHORIZATION'),
      false,
    );
    assert.equal((await writeDraft(fresh, '组织角色降级后保留独立单元编辑权')).durable_seq, '1');
  } finally {
    fresh?.close();
    opened.close();
  }
});

test('product sources: unit revocation keeps chapter inheritance only through a new grant, old close and wrong binding cannot stop it', async () => {
  const f = await organizationFixture(1),
    member = f.members[0],
    document = await documentFixture(f);
  const unit = document.main.units[0].unit_id,
    chapter = document.main.structure.entries[0].chapter_id;
  await waitAuthorization(
    await post(
      '/v1/documents/' + document.id + '/authorization-changes',
      {
        operation_id: operation('source-inherited-grant'),
        expected_acl_revision: '1',
        changes: [
          { action: 'GRANT', grant: editGrant(member, 'CHAPTER', chapter) },
          { action: 'GRANT', grant: editGrant(member, 'UNIT', unit) },
        ],
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const opened = await openReviewWorkspace(f, document, member),
    ticket = await pendingTicket(document, member);
  const scope = 'doc-acl:' + document.id,
    revocation = operation('source-unit-revoke');
  let fresh, peer;
  try {
    await waitAuthorization(
      await post(
        '/v1/documents/' + document.id + '/authorization-changes',
        {
          operation_id: revocation,
          expected_acl_revision: '2',
          changes: [{ action: 'REVOKE', grant: editGrant(member, 'UNIT', unit) }],
        },
        f.owner.headers,
      ),
      f.owner,
    );
    await closed(opened);
    const original = await assertSourceClosure(revocation, scope, '2', [
      'document',
      'collaboration',
    ]);
    await assertWorkspaceClosed(opened);
    await denyLate(ticket);
    fresh = await openReviewWorkspace(f, document, member);
    await assertNewSource(fresh, scope, '2');
    peer = collaborationRPC();
    const ack = await rpcCall(peer, 'FenceAccess', original);
    assert.equal(ack.state, 'ACKNOWLEDGED');
    assert.ok(ack.revocation_id === revocation, 'ORIGINAL_CLOSE_ACK_REQUIRED');
    assert.equal(fresh.w.socket.readyState, WebSocket.OPEN);
    await assertNewSource(fresh, scope, '2');
    // 每份错误输入都仍使用原关闭号；代次/目标/范围错误不能被另一份历史成功ACK掩盖。
    for (const wrong of [
      { ...original, generation: '2' },
      { ...original, target_digest: Buffer.alloc(32, 9) },
      sourceFence(revocation, scope, '3'),
      sourceFence(revocation, 'kb-acl:' + f.kb, '2'),
    ]) {
      await assert.rejects(
        () => rpcCall(peer, 'FenceAccess', wrong),
        (error) => [3, 6].includes(error.code),
      );
      assert.equal(fresh.w.socket.readyState, WebSocket.OPEN);
      await assertNewSource(fresh, scope, '2');
    }
    assert.equal((await writeDraft(fresh, '旧关闭重放不影响按章节继承的新来源')).durable_seq, '1');
  } finally {
    peer?.close();
    fresh?.close();
    opened.close();
  }
});

test('product sources: removing knowledge-base admin closes old scope while explicit document EDIT survives', async () => {
  const f = await organizationFixture(1),
    member = f.members[0],
    document = await documentFixture(f);
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/roles',
      {
        operation_id: operation('source-kb-admin'),
        expected_acl_revision: '1',
        user_id: member.id,
        role: 'ADMIN',
      },
      f.owner.headers,
    ),
    f.owner,
  );
  await grantEdit(f, document, member);
  const opened = await openReviewWorkspace(f, document, member),
    ticket = await pendingTicket(document, member);
  const scope = 'kb-acl:' + f.kb,
    revocation = operation('source-kb-remove');
  let fresh;
  try {
    await waitAuthorization(
      await post(
        '/v1/knowledge-bases/' + f.kb + '/roles',
        {
          operation_id: revocation,
          expected_acl_revision: '2',
          user_id: member.id,
          role: 'REMOVE',
        },
        f.owner.headers,
      ),
      f.owner,
    );
    await closed(opened);
    await assertSourceClosure(revocation, scope, '2', ['document', 'collaboration']);
    await assertWorkspaceClosed(opened);
    await denyLate(ticket);
    fresh = await openReviewWorkspace(f, document, member);
    await assertNewSource(fresh, scope, '2');
    const header = await get('/v1/documents/' + document.id, member.headers);
    assert.equal(header.status, 200);
    assert.equal(
      JSON.parse(header.headers['x-available-actions']).includes('CHANGE_AUTHORIZATION'),
      false,
    );
    assert.equal(
      (await writeDraft(fresh, '库管理员角色移除后独立文档编辑权仍有效')).durable_seq,
      '1',
    );
  } finally {
    fresh?.close();
    opened.close();
  }
});
