import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from '../../scripts/runtime-lib.mjs';
import { reviewFixture, writeDraftBody, reviewDatabase } from '../helpers/review.mjs';
import { uploadPNG, binaryResourceGet, resourcePNG } from '../helpers/resource.mjs';

// 冷备份必须包含一个真正仅存在云端、且由真实草稿引用的对象。保留此样本，
// 不在 finally 中删除；后续恢复验证使用归档内的原业务身份和引用读取。
const probePath = resolve(runtime, 'services/restore-oss-probe.json');
assert.equal(existsSync(probePath), false, 'OSS_BACKUP_PROBE_ALREADY_EXISTS');
const config = JSON.parse(readFileSync(resolve(runtime, 'services/document.json')));
assert.equal(config.resources.oss?.bucket, 'collaboration-report');
assert.ok(config.resources.oss.prefix.startsWith('collabreport-tests/'));
const f = await reviewFixture();
const database = await reviewDatabase();
try {
  const resource = await uploadPNG(f, resourcePNG);
  const saved = await writeDraftBody(f, {
    type: 'doc',
    content: [
      {
        type: 'paragraph',
        attrs: { id: 'oss-backup-reference' },
        content: [
          {
            type: 'image',
            attrs: {
              resourceId: resource,
              alt: '真实云备份图片',
              title: null,
              width: 1,
              height: 1,
            },
          },
        ],
      },
    ],
  });
  assert.equal(saved.type, 'SAVED');
  const path =
    `/v1/resources/${resource}/content?` +
    new URLSearchParams({
      document_id: f.d.id,
      context_kind: 'BRANCH',
      context_id: f.session.branch_id,
    });
  const downloaded = await binaryResourceGet(path, f.user.headers);
  assert.equal(downloaded.status, 200);
  assert.deepEqual(downloaded.body, resourcePNG);
  const [[row]] = await database.query(
    'SELECT storage_key,byte_size,LOWER(HEX(checksum)) checksum,state FROM resource WHERE id=?',
    [resource],
  );
  assert.equal(row.state, 'READY');
  assert.equal(
    existsSync(resolve(config.resources.root, row.storage_key)),
    false,
    'PROBE_MUST_HAVE_NO_LOCAL_OBJECT_FALLBACK',
  );
  assert.equal(row.checksum, createHash('sha256').update(resourcePNG).digest('hex'));
  // 只写私密运行目录，口令/会话不输出日志；云 AccessKey 不进入探针或归档。
  writeFileSync(
    probePath,
    JSON.stringify(
      {
        email: f.user.email,
        password: f.user.password,
        document: f.d.id,
        resource,
        branch: f.session.branch_id,
        path,
        key: row.storage_key,
        byteSize: Number(row.byte_size),
        checksum: row.checksum,
      },
      null,
      2,
    ) + '\n',
    { flag: 'wx' },
  );
  console.log(
    JSON.stringify({
      state: 'OSS_BACKUP_PROBE_READY',
      cloudOnly: true,
      byteSize: Number(row.byte_size),
    }),
  );
} finally {
  f.close();
  await database.end();
}
