import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createHash } from 'node:crypto';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import mysql from 'mysql2/promise';
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import { schema, toPM } from '../../packages/content/index.mjs';
import { root, runtime, state } from '../../scripts/runtime-lib.mjs';
import { organizationFixture, documentFixture, get, post, json, operation } from './workspace.mjs';
import { documentRPC, rpcCall } from './collaboration.mjs';
import { wire } from './wss.mjs';

export function collaborationRPC() {
  const api = grpc.loadPackageDefinition(
    loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), {
      longs: String,
      enums: String,
      defaults: true,
      keepCase: true,
    }),
  ).collaboration.v1;
  return new api.CollaborationInternal(
    '127.0.0.1:' + state().ports.collaborationRpc,
    grpc.credentials.createSsl(
      readFileSync(resolve(runtime, 'pki/ca.pem')),
      readFileSync(resolve(runtime, 'pki/document-key.pem')),
      readFileSync(resolve(runtime, 'pki/document.pem')),
    ),
    { 'grpc.ssl_target_name_override': 'localhost', 'grpc.max_receive_message_length': 32 << 20 },
  );
}

export async function reviewDatabase(role = 'document') {
  if (!['document', 'collaboration', 'account'].includes(role))
    throw Error('TEST_DATABASE_ROLE_INVALID');
  return mysql.createConnection(
    JSON.parse(readFileSync(resolve(runtime, 'services/' + role + '.json'))).database,
  );
}

export async function reviewFixture(memberCount = 0, existingOwner = null) {
  const f = await organizationFixture(memberCount, existingOwner),
    d = await documentFixture(f);
  return openReviewWorkspace(f, d);
}

export async function openReviewWorkspace(f, d, user = f.owner, workspaceKind = 'BRANCH') {
  const client = operation('client'),
    connection = operation('connection');
  const response = await post(
    `/v1/documents/${d.id}/units/${d.main.units[0].unit_id}/edit-sessions`,
    {
      operation_id: operation('edit'),
      workspace_kind: workspaceKind,
      client_session_id: client,
      expected_structure_id: d.main.structure.id,
    },
    user.headers,
  );
  if (response.status !== 200)
    throw Error('EDIT_SESSION_FAILED', {
      cause: { status: response.status, code: json(response).code },
    });
  const session = json(response),
    peer = documentRPC();
  let grant;
  try {
    // 与实际 WSS 使用同一 connection，票据幂等重取，不能伪造另一份 grant。
    grant = await rpcCall(peer, 'RedeemCollaborationTicket', {
      ticket: Buffer.from(session.ticket),
      client_session_id: client,
      connection_id: connection,
    });
  } finally {
    peer.close();
  }
  const w = wire(session, client, connection),
    doc = new Y.Doc();
  Y.applyUpdate(doc, Buffer.from((await w.ready).update, 'base64'));
  return {
    f,
    d,
    user,
    session,
    grant,
    w,
    doc,
    close() {
      w.close();
      doc.destroy();
    },
  };
}

export async function writeDraft(f, text) {
  return writeDraftBody(f, {
    type: 'doc',
    content: [
      { type: 'paragraph', attrs: { id: 'review-paragraph' }, content: [{ type: 'text', text }] },
    ],
  });
}

export async function writeDraftBody(f, body) {
  const before = Y.encodeStateVector(f.doc),
    source = prosemirrorJSONToYDoc(schema, toPM(body)),
    fragment = f.doc.getXmlFragment('prosemirror');
  fragment.delete(0, fragment.length);
  fragment.insert(
    0,
    source
      .getXmlFragment('prosemirror')
      .toArray()
      .map((n) => n.clone()),
  );
  source.destroy();
  f.w.send({
    type: 'UPDATE',
    client_update_id: operation('update'),
    update: Buffer.from(Y.encodeStateAsUpdate(f.doc, before)).toString('base64'),
  });
  return f.w.next('SAVED');
}

export async function flushDraft(f) {
  f.w.send({ type: 'FLUSH', operation_id: operation('flush'), dependencies: [] });
  return f.w.next('FLUSHED');
}

export async function submitDraft(f, overrides = {}) {
  const flush = await flushDraft(f);
  const input = {
    operation_id: operation('submission'),
    mode: 'NEW',
    branch_id: f.session.branch_id,
    expected_structure_id: f.grant.context.target.structure_id,
    flush_receipt_id: flush.receipt_id,
    ...overrides,
  };
  const response = await post(
    `/v1/documents/${f.d.id}/units/${f.grant.context.target.unit_id}/submissions`,
    input,
    f.user.headers,
  );
  if (response.status !== 202) throw Error('SUBMISSION_HTTP_' + response.status);
  for (let n = 0; n < 100; n++) {
    const response = await get('/v1/operations/' + input.operation_id, f.user.headers);
    if (response.status !== 200) throw Error('SUBMISSION_POLL_HTTP_' + response.status);
    const value = json(response);
    if (value.state !== 'PREPARING') return { input, value };
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw Error('SUBMISSION_NOT_TERMINAL');
}

export async function captureInput(f, receipt) {
  const input = {
    context: {
      ...f.grant.context,
      operation_id: operation('submit'),
      target: { ...f.grant.context.target, purpose: 'SUBMISSION' },
      request_fingerprint: createHash('sha256').update(operation('fingerprint')).digest(),
    },
    flush_receipt_id: receipt.receipt_id,
    guard_generation: '1',
    lineage_ref: f.session.lineage_id,
  };
  const api = grpc.loadPackageDefinition(
    loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), {
      longs: String,
      enums: String,
      defaults: true,
      keepCase: true,
    }),
  ).collaboration.v1;
  const identity = new api.IdentityInternal(
    '127.0.0.1:' + state().ports.accountRpc,
    grpc.credentials.createSsl(
      readFileSync(resolve(runtime, 'pki/ca.pem')),
      readFileSync(resolve(runtime, 'pki/document-key.pem')),
      readFileSync(resolve(runtime, 'pki/document.pem')),
    ),
    { 'grpc.ssl_target_name_override': 'localhost' },
  );
  try {
    // 编辑grant不含身份准入引用；送审必须取得实际账号准入，不能给测试填假编号。
    const proof = await rpcCall(identity, 'AcquireIdentityAdmission', {
      operation_id: input.context.operation_id,
      session_token: Buffer.from(
        /__Host-collabSession=([^;]+)/.exec(f.f.owner.headers.cookie)[1],
        'base64url',
      ),
      purpose: 'SUBMISSION',
      target: input.context.target,
      required_identity_capability: 'ACTIVE_ACCOUNT',
      identity_target: { kind: 'USER', user_id: f.f.owner.id },
    });
    input.context.identity_admission_ref = proof.admission_id;
    return input;
  } finally {
    identity.close();
  }
}
