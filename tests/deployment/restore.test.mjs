import assert from 'node:assert/strict';
import test from 'node:test';
import https from 'node:https';
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { restoredConfig, restoredDatabase } from './restore-baseline.mjs';
import { readMigrationChain } from '../../scripts/migration-chain.mjs';

const upgrading = process.env.COLLAB_UPGRADE_VALIDATION === '1';

function request(target, port, path, options = {}) {
  return new Promise((done, reject) => {
    const ca = readFileSync(resolve(target, '.runtime/pki/ca.pem'));
    const body = options.body === undefined ? undefined : JSON.stringify(options.body);
    const req = https.request(
      {
        hostname: 'localhost',
        port,
        path,
        ca,
        method: options.method ?? 'GET',
        headers: {
          ...options.headers,
          ...(body
            ? { 'content-type': 'application/json', 'content-length': Buffer.byteLength(body) }
            : {}),
        },
      },
      (response) => {
        const chunks = [];
        let size = 0;
        response.on('data', (chunk) => {
          size += chunk.length;
          if (size > 32 << 20) req.destroy(Error('RESTORE_RESPONSE_TOO_LARGE'));
          else chunks.push(chunk);
        });
        response.once('error', reject);
        response.once('aborted', () => reject(Error('RESTORE_RESPONSE_INCOMPLETE')));
        response.once('end', () => {
          clearTimeout(timer);
          done({
            status: response.statusCode,
            headers: response.headers,
            body: Buffer.concat(chunks),
          });
        });
      },
    );
    const timer = setTimeout(() => req.destroy(Error('RESTORE_REQUEST_TIMEOUT')), 3000);
    req.once('error', (error) => {
      clearTimeout(timer);
      reject(error);
    });
    req.end(body);
  });
}
const json = (response) => JSON.parse(response.body.toString('utf8'));

test(
  upgrading
    ? 'isolated upgrade preserves original migrations identities fixed bodies objects receipts and consumer watermarks'
    : 'isolated cold restore preserves real identities fixed bodies objects original receipts and consumer watermarks',
  { timeout: 120000 },
  async () => {
    assert.ok(process.env.COLLAB_RESTORE_ROOT, 'COLLAB_RESTORE_ROOT_REQUIRED');
    const target = resolve(process.env.COLLAB_RESTORE_ROOT);
    const environment = JSON.parse(readFileSync(resolve(target, '.runtime/dev.json')));
    const restoration = JSON.parse(readFileSync(resolve(target, '.runtime/restore.json')));
    const baseline = JSON.parse(readFileSync(resolve(target, '.runtime/restore-baseline.json')));
    assert.equal(restoration.state, 'RESTORED_STOPPED');
    assert.ok(environment.owner.startsWith('collaboration-report-restore-'));
    const evidence = {
      state: 'RUNNING',
      ready: [],
      migrations: {},
      resources: 0,
      originalReceipts: 0,
      canonicalVersions: 0,
      checkpoints: 0,
    };
    for (const role of ['account', 'document', 'collaboration', 'search']) {
      const ready = await request(target, environment.ports[role + 'Http'], '/health/ready');
      assert.equal(ready.status, 200, 'RESTORED_SERVICE_NOT_READY ' + role);
      evidence.ready.push(role);
      const db = await restoredDatabase(target, role);
      try {
        const [migrations] = await db.query(
          'SELECT version,HEX(checksum) checksum FROM schema_migration ORDER BY version',
        );
        // 冷恢复必须与旧基线完全相同；显式升级才允许追加，旧前缀仍逐字节
        // 保留，并且整个新序列须等于目标源码的已校验SHA链，不能接受任意迁移。
        assert.deepEqual(
          upgrading ? migrations.slice(0, baseline[role].migrations.length) : migrations,
          baseline[role].migrations,
          'RESTORED_MIGRATIONS_CHANGED ' + role,
        );
        if (upgrading) {
          const expected = readMigrationChain(resolve(target, 'db/migrations')).map(
            ({ manifest }) => ({
              version: manifest.version,
              checksum: manifest.sha256.toUpperCase(),
            }),
          );
          assert.ok(
            expected.length > baseline[role].migrations.length,
            'UPGRADE_MUST_APPEND_MIGRATION',
          );
          assert.deepEqual(migrations, expected, 'UPGRADE_MIGRATION_CHAIN_CHANGED ' + role);
        }
        evidence.migrations[role] = migrations.length;
        if (role === 'document') {
          const [versions] = await db.query(
            'SELECT id,HEX(body_hash) body_hash FROM content_version ORDER BY id',
          );
          const byID = new Map(versions.map((value) => [value.id, value.body_hash]));
          for (const version of baseline.document.versions)
            assert.equal(byID.get(version.id), version.body_hash, 'RESTORED_FIXED_VERSION_CHANGED');
          evidence.canonicalVersions = baseline.document.versions.length;
          for (const original of baseline.document.receipts) {
            const [[actual]] = await db.query(
              'SELECT operation_id,actor_id,kind,state,HEX(request_fingerprint) fingerprint FROM operation_receipt WHERE operation_id=?',
              [original.operation_id],
            );
            assert.deepEqual(actual, original, 'RESTORED_ORIGINAL_RECEIPT_CHANGED');
            evidence.originalReceipts++;
          }
          const files =
            restoredConfig(target, 'document').resources?.root ??
            resolve(target, '.runtime/resources/document');
          const [resourceRows] = await db.query(
            'SELECT id,storage_key,byte_size,HEX(checksum) checksum,state FROM resource ORDER BY id',
          );
          const restoredResources = new Map(resourceRows.map((row) => [row.id, row]));
          evidence.resourceMetadata = 0;
          evidence.metadataOnlyFixtures = 0;
          evidence.completedPhysicalGC = 0;
          for (const resource of baseline.document.resources) {
            const current = restoredResources.get(resource.id);
            assert.ok(current, 'RESTORED_RESOURCE_METADATA_MISSING');
            const { state, ...metadata } = current;
            assert.deepEqual(metadata, resource, 'RESTORED_RESOURCE_METADATA_CHANGED');
            evidence.resourceMetadata++;
            // 历史审核/协作SQL夹具明确只验证元数据，未走物理上传。
            // 保留并核对它们，不能把125个元数据夹具说成已恢复物理图片。
            if (!/^[a-f0-9]{64}$/.test(resource.storage_key)) {
              evidence.metadataOnlyFixtures++;
              continue;
            }
            if (state === 'DELETED') {
              assert.equal(existsSync(resolve(files, resource.storage_key)), false);
              evidence.completedPhysicalGC++;
              continue;
            }
            const bytes = readFileSync(resolve(files, resource.storage_key));
            assert.equal(BigInt(bytes.length), BigInt(resource.byte_size));
            assert.equal(
              createHash('sha256').update(bytes).digest('hex').toUpperCase(),
              resource.checksum,
            );
            evidence.resources++;
          }
        }
        if (role === 'search') {
          const [current] = await db.query(
            'SELECT consumer_name,kafka_topic,partition_id,next_offset FROM consumer_checkpoint ORDER BY consumer_name,kafka_topic,partition_id',
          );
          for (const original of baseline.search.checkpoints) {
            const actual = current.find(
              (row) =>
                row.consumer_name === original.consumer_name &&
                row.kafka_topic === original.kafka_topic &&
                row.partition_id === original.partition_id,
            );
            assert.ok(
              actual && BigInt(actual.next_offset) >= BigInt(original.next_offset),
              'RESTORED_CONSUMER_WATERMARK_REGRESSED',
            );
            evidence.checkpoints++;
          }
        }
      } finally {
        await db.end();
      }
    }
    assert.ok(
      evidence.resources > 0 &&
        evidence.originalReceipts > 0 &&
        evidence.canonicalVersions > 0 &&
        evidence.checkpoints > 0,
    );
    const origin = 'https://localhost:' + environment.ports.edge;
    let authenticated;
    for (const fixture of baseline.account.users) {
      const response = await request(target, environment.ports.edge, '/v1/account/login', {
        method: 'POST',
        headers: { origin },
        body: { email: fixture.email, password: 'fixture-password-123' },
      });
      if (response.status === 401) continue;
      assert.equal(response.status, 200, 'RESTORED_REAL_LOGIN_FAILED');
      assert.equal(json(response).user_id, fixture.id);
      authenticated = {
        fixture,
        cookie: response.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
      };
      break;
    }
    assert.ok(authenticated, 'RESTORED_FIXTURE_IDENTITY_UNAVAILABLE');
    const me = await request(target, environment.ports.edge, '/v1/account/me', {
      headers: { cookie: authenticated.cookie },
    });
    assert.equal(me.status, 200);
    assert.equal(json(me).id, authenticated.fixture.id);
    evidence.authenticatedUser = authenticated.fixture.id;
    // 只接受当前真正公开资格；文档曾公开不代表父知识库/空间现在仍允许。
    // 找到合法入口后逐单元核对真实不可变正文和摘要，不仅检查页面能打开。
    let publicVerified = false;
    const db = await restoredDatabase(target, 'document');
    try {
      for (const candidate of baseline.document.publicDocs) {
        const response = await request(
          target,
          environment.ports.edge,
          '/v1/public/documents/' + candidate.id,
        );
        if ([401, 403, 404].includes(response.status)) continue;
        assert.equal(response.status, 200, 'RESTORED_PUBLIC_READING_FAILED');
        const value = json(response);
        assert.equal(value.release_id, candidate.current_release_id);
        assert.ok(value.units.length > 0);
        for (const unit of value.units) {
          const [[fixed]] = await db.query(
            'SELECT canonical_body,HEX(body_hash) body_hash FROM content_version WHERE id=? AND document_id=?',
            [unit.id, candidate.id],
          );
          assert.ok(fixed);
          assert.equal(unit.body_hash.toUpperCase(), fixed.body_hash);
          assert.deepEqual(unit.body_json, JSON.parse(fixed.canonical_body.toString('utf8')));
        }
        evidence.publicDocument = candidate.id;
        evidence.publicRelease = value.release_id;
        publicVerified = true;
        break;
      }
    } finally {
      await db.end();
    }
    assert.ok(publicVerified, 'RESTORED_REAL_PUBLIC_CONTENT_REQUIRED');
    evidence.state = 'PASS';
    evidence.completedAt = new Date().toISOString();
    writeFileSync(
      resolve(
        target,
        upgrading
          ? 'artifacts/batch08-upgrade-verification.json'
          : 'artifacts/batch08-restore-verification.json',
      ),
      JSON.stringify(evidence, null, 2) + '\n',
    );
    console.log(
      JSON.stringify({
        event: upgrading ? 'ISOLATED_UPGRADE_ACTUAL_PASS' : 'ISOLATED_RESTORE_ACTUAL_PASS',
        ...evidence,
      }),
    );
  },
);
