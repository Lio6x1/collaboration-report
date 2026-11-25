import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdirSync, copyFileSync, unlinkSync } from 'node:fs';
import { resolve } from 'node:path';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { root, runtime, state as environment } from '../../scripts/runtime-lib.mjs';
import { searchDatabase } from '../helpers/search.mjs';

const execute = promisify(execFile);
async function command(role, name, args) {
  try {
    const { stdout } = await execute(
      resolve(runtime, 'bin', name + '.exe'),
      ['--config', resolve(runtime, 'services', role + '.json'), ...args],
      { cwd: root, windowsHide: true, timeout: 30000 },
    );
    return JSON.parse(stdout);
  } catch (error) {
    const code = (error.stderr ?? '').trim();
    assert.fail(/^[A-Z][A-Z0-9_]{1,95}$/.test(code) ? code : 'REBUILD_CLI_FAILED');
  }
}

test('真实导入与Start/Get绑定原操作，独立检查点固定原L', { timeout: 90000 }, async () => {
  // 续接此前已验收封存操作，不再重扫Doc或生成另一个起点。
  const state = JSON.parse(
    readFileSync(resolve(root, 'artifacts/batches/06/rebuild-prepare-operation.json')),
  );
  assert.ok(state.snapshot && state.prepare);
  const digest = state.snapshot.split(':')[1];
  const inbox = resolve(runtime, 'rebuild/search/inbox');
  const accepted = resolve(runtime, 'rebuild/search/accepted');
  mkdirSync(inbox, { recursive: true });
  for (const suffix of ['snapshot.json', 'entries.ndjson']) {
    const name = digest + '.' + suffix;
    copyFileSync(resolve(runtime, 'rebuild/document/outbox', name), resolve(inbox, name));
  }
  const args = ['--operation', state.operation, '--snapshot-ref', state.snapshot];
  const imported = await command('search', 'search-rebuild-import', args);
  assert.equal(imported.reference, state.snapshot);
  assert.deepEqual(await command('search', 'search-rebuild-import', args), imported);
  // 已接受输入丢失本地副本时，只允许原摘要副本重生，不能接受另一份快照。
  const original = readFileSync(resolve(accepted, digest + '.entries.ndjson'));
  unlinkSync(resolve(accepted, digest + '.entries.ndjson'));
  assert.deepEqual(await command('search', 'search-rebuild-import', args), imported);
  assert.deepEqual(readFileSync(resolve(accepted, digest + '.entries.ndjson')), original);
  const started = await command('document', 'search-rebuild', [...args, '--action', 'start']);
  assert.equal(started.operation_id, state.operation);
  assert.equal(started.generation, state.generation);
  assert.equal(started.state, 'PROCESSING');
  assert.deepEqual(
    await command('document', 'search-rebuild', [...args, '--action', 'start']),
    started,
  );
  assert.deepEqual(
    await command('document', 'search-rebuild', [...args, '--action', 'status']),
    started,
  );
  const db = await searchDatabase();
  try {
    const [[binding]] = await db.query(
      'SELECT state,CAST(job_id AS CHAR) job_id,request_fingerprint,prepare_envelope FROM rebuild_binding WHERE operation_id=?',
      [state.operation],
    );
    assert.equal(binding.state, 'ACCEPTED');
    const prepare = JSON.parse(Buffer.from(JSON.parse(binding.prepare_envelope).payload, 'base64'));
    const [checkpoints] = await db.query(
      'SELECT partition_id,CAST(next_offset AS CHAR) next_offset FROM consumer_checkpoint WHERE consumer_name=? ORDER BY partition_id',
      ['rebuild-' + binding.job_id],
    );
    assert.deepEqual(
      checkpoints.map((x) => x.partition_id),
      [0, 1, 2],
    );
    assert.deepEqual(
      checkpoints.map((x) => x.next_offset),
      prepare.starts,
    );
    const api = grpc.loadPackageDefinition(
      loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), {
        longs: String,
        enums: String,
        keepCase: true,
        defaults: true,
      }),
    ).collaboration.v1;
    const client = new api.SearchInternal(
      '127.0.0.1:' + environment().ports.searchRpc,
      grpc.credentials.createSsl(
        readFileSync(resolve(runtime, 'pki/ca.pem')),
        readFileSync(resolve(runtime, 'pki/document-key.pem')),
        readFileSync(resolve(runtime, 'pki/document.pem')),
      ),
      { 'grpc.ssl_target_name_override': 'localhost' },
    );
    try {
      for (const query of [
        { operation_id: state.operation, request_fingerprint: Buffer.alloc(32) },
        {
          operation_id: state.operation,
          request_fingerprint: binding.request_fingerprint,
          target: { document_id: '1' },
        },
      ]) {
        const error = await new Promise((done) =>
          client.GetRebuild(query, { deadline: Date.now() + 3000 }, (err) => done(err)),
        );
        assert.ok(
          error &&
            [grpc.status.INVALID_ARGUMENT, grpc.status.FAILED_PRECONDITION].includes(error.code),
        );
      }
    } finally {
      client.close();
    }
  } finally {
    await db.end();
  }
});
