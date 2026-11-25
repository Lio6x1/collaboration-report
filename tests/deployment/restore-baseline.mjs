import assert from 'node:assert/strict';
import mysql from 'mysql2/promise';
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve, isAbsolute } from 'node:path';
import { pathToFileURL } from 'node:url';

export function restoredConfig(target, role) {
  return JSON.parse(readFileSync(resolve(target, '.runtime/services', role + '.json')));
}
export async function restoredDatabase(target, role) {
  assert.ok(['account', 'document', 'collaboration', 'search'].includes(role));
  return mysql.createConnection(restoredConfig(target, role).database);
}

export async function captureBaseline(target) {
  assert.ok(isAbsolute(target), 'RESTORE_ABSOLUTE_PATH_REQUIRED');
  const result = {};
  for (const role of ['account', 'document', 'collaboration', 'search']) {
    const db = await restoredDatabase(target, role);
    try {
      const [migrations] = await db.query(
        'SELECT version,HEX(checksum) checksum FROM schema_migration ORDER BY version',
      );
      result[role] = { migrations };
      if (role === 'document') {
        const [publicDocs] = await db.query(
          "SELECT id,current_release_id,visibility_policy,lifecycle FROM document WHERE publication_state='PUBLISHED' AND visibility_policy='PUBLIC' AND lifecycle IN ('ACTIVE','PUBLIC_RETAINED') ORDER BY id LIMIT 20",
        );
        const [resources] = await db.query(
          "SELECT id,storage_key,byte_size,HEX(checksum) checksum FROM resource WHERE state='READY' ORDER BY id",
        );
        const [versions] = await db.query(
          "SELECT v.id,HEX(v.body_hash) body_hash FROM content_version v JOIN document d ON d.id=v.document_id WHERE d.lifecycle IN ('ACTIVE','PUBLIC_RETAINED') ORDER BY v.id",
        );
        const [receipts] = await db.query(
          "SELECT operation_id,actor_id,kind,state,HEX(request_fingerprint) fingerprint FROM operation_receipt WHERE state='SUCCEEDED' ORDER BY operation_id LIMIT 40",
        );
        Object.assign(result[role], { publicDocs, resources, versions, receipts });
      }
      if (role === 'account') {
        const [users] = await db.query(
          "SELECT id,email FROM user WHERE state='ACTIVE' AND email LIKE 'fixture-%@example.test' ORDER BY id LIMIT 5",
        );
        result[role].users = users;
      }
      if (role === 'search') {
        const [checkpoints] = await db.query(
          'SELECT consumer_name,kafka_topic,partition_id,next_offset FROM consumer_checkpoint ORDER BY consumer_name,kafka_topic,partition_id',
        );
        result[role].checkpoints = checkpoints;
      }
    } finally {
      await db.end();
    }
  }
  // 测试身份只留在恢复环境的私密忽略目录，公开日志只给出数量。
  writeFileSync(
    resolve(target, '.runtime/restore-baseline.json'),
    JSON.stringify(result, null, 2) + '\n',
  );
  return result;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const result = await captureBaseline(resolve(process.argv[2]));
  console.log(
    JSON.stringify({
      state: 'BASELINE_CAPTURED',
      schemas: Object.keys(result),
      publicDocuments: result.document.publicDocs.length,
      resources: result.document.resources.length,
      canonicalVersions: result.document.versions.length,
      originalReceipts: result.document.receipts.length,
      consumerPartitions: result.search.checkpoints.length,
    }),
  );
}
