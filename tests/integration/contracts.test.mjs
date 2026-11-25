import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import loader from '@grpc/proto-loader';
import { createHash } from 'node:crypto';
import { root, runtime } from '../../scripts/runtime-lib.mjs';
import { loadHTTPContract, mergeReadExtensions } from '../../scripts/load-http-contract.mjs';
import { mergeImplementationExtensions } from '../../scripts/http-implementation-extensions.mjs';
test('Node to Go protobuf preserves uint64 maximum in actual wire bytes', () => {
  const schema = loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), {
    longs: String,
    defaults: true,
    keepCase: true,
  });
  const type = schema['collaboration.v1.IdentityAdmission'];
  const bytes = type.serialize({
    actor_id: '18446744073709551615',
    sources: [{ scope_key: 'test', epoch: '18446744073709551615' }],
  });
  const result = spawnSync('go', ['-C', resolve(root, 'contracts'), 'run', './cmd/golden'], {
    windowsHide: true,
    input: bytes,
    env: { ...process.env, GOWORK: 'off', GOCACHE: resolve(runtime, 'go-cache') },
    timeout: 60000,
  });
  assert.equal(result.status, 0);
  const value = JSON.parse(result.stdout);
  assert.equal(value.actor_id, '18446744073709551615');
  assert.equal(value.epoch, value.actor_id);
});
test('copied contracts retain finalized validation bytes', () => {
  // 固定清单让独立clone也能核验，不要求机器上保留研究目录。
  const baseline = JSON.parse(readFileSync(resolve(root, 'contracts/BASELINE.json')));
  for (const [file, sha] of Object.entries(baseline.files))
    assert.equal(
      createHash('sha256')
        .update(readFileSync(resolve(root, 'contracts', file)))
        .digest('hex'),
      sha,
      file,
    );
});

test('personal acceptance extends only new routes and models without replacing frozen mutations', () => {
  // 接收动作是已确认业务但冻结API缺少的入口。扩展只能追加该组新协议，不能覆盖
  // 老转交动作、User模型或用路径参数别名绕过同一路由的合同核验。
  const baseline = JSON.parse(readFileSync(resolve(root, 'contracts/http/openapi.json')));
  const extension = JSON.parse(
    readFileSync(resolve(root, 'contracts/http/implementation-extensions.json')),
  );
  const api = loadHTTPContract(root);
  assert.ok(api.paths['/v1/personal-transfers/{transferId}/accept'].post);
  assert.deepEqual(
    api.paths['/v1/documents/{documentId}/responsible-transfers'],
    baseline.paths['/v1/documents/{documentId}/responsible-transfers'],
  );
  assert.equal(Object.hasOwn(baseline.components.schemas, 'PersonalTransferAccept'), false);
  assert.throws(
    () =>
      mergeImplementationExtensions(baseline, {
        ...extension,
        schemas: { User: { type: 'object' } },
      }),
    /MODEL_INVALID/,
  );
  for (const path of [
    '/v1/documents/{documentId}/responsible-transfers',
    '/v1/personal-transfers/{otherId}/accept',
  ]) {
    assert.throws(
      () =>
        mergeImplementationExtensions(baseline, {
          ...extension,
          paths: { [path]: extension.paths['/v1/personal-transfers/{transferId}/accept'] },
        }),
      /PATH_INVALID/,
    );
  }
});

test('closure identity generation extends only the read response without replacing frozen fields', () => {
  const api = loadHTTPContract(root);
  assert.equal(api.components.schemas.User.properties.identity_epoch.readOnly, true);
  assert.ok(api.components.schemas.User.required.includes('identity_epoch'));
  const baseline = JSON.parse(readFileSync(resolve(root, 'contracts/http/openapi.json')));
  const extension = JSON.parse(readFileSync(resolve(root, 'contracts/http/read-extensions.json')));
  for (const fields of [
    { User: { credential_epoch: { type: 'string', readOnly: true } } },
    {
      User: {
        identity_epoch: { ...extension.response_fields.User.identity_epoch, readOnly: false },
      },
    },
    { AccountClose: { identity_epoch: extension.response_fields.User.identity_epoch } },
  ])
    assert.throws(
      () => mergeReadExtensions(baseline, { ...extension, response_fields: fields }),
      /HTTP_READ_EXTENSION_FIELD_INVALID/,
    );
  assert.equal(Object.hasOwn(baseline.components.schemas.User.properties, 'identity_epoch'), false);
});

test('organization member creation status is a bounded read-only projection', () => {
  const api = loadHTTPContract(root);
  const baseline = JSON.parse(readFileSync(resolve(root, 'contracts/http/openapi.json')));
  const extension = JSON.parse(readFileSync(resolve(root, 'contracts/http/read-extensions.json')));
  const field = api.components.schemas.Member.properties.knowledge_base_creation;
  assert.deepEqual(field.enum, ['ROLE', 'GRANTED', 'NONE']);
  assert.equal(field.readOnly, true);
  assert.equal(api.components.schemas.Member.required.includes('knowledge_base_creation'), false);
  assert.equal(
    Object.hasOwn(baseline.components.schemas.Member.properties, 'knowledge_base_creation'),
    false,
  );
  for (const value of [
    { ...field, readOnly: false },
    { ...field, enum: ['ALLOWED'] },
  ]) {
    assert.throws(
      () =>
        mergeReadExtensions(baseline, {
          ...extension,
          response_fields: { Member: { knowledge_base_creation: value } },
        }),
      /HTTP_READ_EXTENSION_FIELD_INVALID/,
    );
  }
});

test('document collaborators add only a management read response without contact details', () => {
  // 文档协作者不是组织Member，不能伪造role/revision；也不能通过只读扩展覆盖旧写入或User。
  const api = loadHTTPContract(root);
  const baseline = JSON.parse(readFileSync(resolve(root, 'contracts/http/openapi.json')));
  const extension = JSON.parse(readFileSync(resolve(root, 'contracts/http/read-extensions.json')));
  const path = '/v1/documents/{documentId}/collaborators';
  const mentionPath = '/v1/documents/{documentId}/mention-candidates';
  assert.deepEqual(Object.keys(api.paths[path]), ['get']);
  assert.equal(api.paths[path].get['x-required-capability'], 'DOCUMENT_MANAGEMENT');
  assert.deepEqual(Object.keys(api.paths[mentionPath]), ['get']);
  assert.equal(api.paths[mentionPath].get['x-required-capability'], 'COMMENT');
  assert.equal(
    api.paths[mentionPath].get.responses['200'].content['application/json'].schema.$ref,
    '#/components/schemas/DocumentCollaboratorPage',
  );
  assert.deepEqual(Object.keys(api.components.schemas.DocumentCollaborator.properties), [
    'user_id',
    'display_name',
  ]);
  assert.deepEqual(
    api.paths['/v1/documents/{documentId}/authorization-changes'],
    baseline.paths['/v1/documents/{documentId}/authorization-changes'],
  );
  // 真实浏览器流程的收口校验由Python执行，不能只检查Node加载器可识别新模型。
  const responseCheck = spawnSync('python', [resolve(root, 'scripts/validate-responses.py')], {
    input: JSON.stringify([
      {
        schema: 'DocumentCollaboratorPage',
        value: { items: [{ user_id: '1', display_name: '协作者' }], has_more: false },
      },
    ]),
    encoding: 'utf8',
    windowsHide: true,
  });
  assert.equal(responseCheck.status, 0, responseCheck.stderr);
  assert.throws(
    () => mergeReadExtensions(baseline, { ...extension, schemas: { User: { type: 'object' } } }),
    /HTTP_READ_EXTENSION_SCHEMA_INVALID/,
  );
  assert.throws(
    () =>
      mergeReadExtensions(baseline, {
        ...extension,
        paths: { [path]: { post: extension.paths[path].get } },
      }),
    /HTTP_READ_EXTENSION_METHOD_INVALID/,
  );
  // 只读目录共用传输形状，不得把COMMENT门槛移到管理入口或把姓名查询放宽成登录即读。
  for (const [candidatePath, capability] of [
    [path, 'COMMENT'],
    [mentionPath, 'ACTIVE_ACCOUNT'],
  ]) {
    assert.throws(
      () =>
        mergeReadExtensions(baseline, {
          ...extension,
          paths: {
            [candidatePath]: {
              get: { ...extension.paths[candidatePath].get, 'x-required-capability': capability },
            },
          },
        }),
      /HTTP_READ_EXTENSION_CANDIDATE_INVALID/,
    );
  }
});

test('review summaries extend only the optional fixed read projection', () => {
  const api = loadHTTPContract(root);
  const baseline = JSON.parse(readFileSync(resolve(root, 'contracts/http/openapi.json')));
  const extension = JSON.parse(readFileSync(resolve(root, 'contracts/http/read-extensions.json')));
  const summary = api.components.schemas.ReviewRequest.properties.summary;
  assert.equal(summary.$ref, '#/components/schemas/ReviewRequestSummary');
  assert.equal(summary.readOnly, true);
  assert.equal(api.components.schemas.ReviewRequest.required.includes('summary'), false);
  assert.equal(
    Object.hasOwn(baseline.components.schemas.ReviewRequest.properties, 'summary'),
    false,
  );
  assert.deepEqual(
    api.components.schemas.ReviewRequest.properties.current_round,
    baseline.components.schemas.ReviewRequest.properties.current_round,
  );
  assert.deepEqual(Object.keys(api.components.schemas.ReviewRequestSummary.properties), [
    'title',
    'excerpt',
    'round_no',
    'submitter_id',
    'submitted_at',
  ]);
  assert.equal(api.components.schemas.ReviewRequestSummary.properties.excerpt.maxLength, 141);
  for (const fields of [
    { ReviewRequest: { status: summary } },
    { ReviewRound: { summary } },
    { ReviewRequest: { summary: { ...summary, $ref: '#/components/schemas/ReviewRound' } } },
    { ReviewRequest: { summary: { ...summary, readOnly: false } } },
  ])
    assert.throws(
      () => mergeReadExtensions(baseline, { ...extension, response_fields: fields }),
      /HTTP_READ_EXTENSION_FIELD_INVALID/,
    );
  const invalid = structuredClone(extension);
  invalid.schemas.ReviewRequestSummary.properties.body_json = { type: 'object' };
  assert.throws(() => mergeReadExtensions(baseline, invalid), /HTTP_READ_EXTENSION_SCHEMA_INVALID/);
  // 真实响应校验须共同识别新字段，并拒绝超长摘要及夹带完整正文。
  const value = {
    id: '99',
    document_id: '10',
    unit_id: '50',
    branch_id: '70',
    status: 'WITHDRAWN',
    current_round_id: '88',
    revision: '1',
    archived_at: null,
    actions: [],
    summary: {
      title: '固定章节',
      excerpt: '中😀'.repeat(70) + '…',
      round_no: '2',
      submitter_id: '7',
      submitted_at: '2026-10-09T08:20:30.123Z',
    },
  };
  const validate = (response) =>
    spawnSync('python', [resolve(root, 'scripts/validate-responses.py')], {
      input: JSON.stringify([{ schema: 'ReviewRequest', value: response }]),
      encoding: 'utf8',
      windowsHide: true,
    });
  const result = validate(value);
  assert.equal(result.status, 0, result.stderr || result.stdout);
  assert.notEqual(
    validate({ ...value, summary: { ...value.summary, excerpt: '中'.repeat(142) } }).status,
    0,
  );
  assert.notEqual(validate({ ...value, summary: { ...value.summary, body_json: {} } }).status, 0);
});
