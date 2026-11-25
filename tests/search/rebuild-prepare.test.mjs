import test from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID, createHash } from 'node:crypto';
import {
  readFileSync,
  writeFileSync,
  existsSync,
  mkdirSync,
  copyFileSync,
  unlinkSync,
} from 'node:fs';
import { resolve } from 'node:path';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { root, runtime } from '../../scripts/runtime-lib.mjs';
import { searchDatabase } from '../helpers/search.mjs';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';

const checkpoint = resolve(root, 'artifacts/batches/06/rebuild-prepare-operation.json');
const save = (state) => writeFileSync(checkpoint, JSON.stringify(state, null, 2) + '\n');
const hash = (raw) => createHash('sha256').update(raw).digest('hex');
const execute = promisify(execFile);
async function command(role, executable, args) {
  // CLI封存期间保持事件循环运行，让HTTPS池及时处理空闲连接关闭。
  // 同步等待子进程会把已关闭的旧socket误留在池中，下一次真实请求随即ECONNRESET。
  let result;
  try {
    result = await execute(
      resolve(runtime, 'bin', executable + '.exe'),
      ['--config', resolve(runtime, 'services', role + '.json'), ...args],
      { cwd: root, windowsHide: true, encoding: 'utf8', timeout: 120000 },
    );
  } catch (result) {
    let code = 'CLI_FAILED';
    const plain = (result.stderr ?? '').trim();
    if (/^[A-Z][A-Z0-9_]{1,95}$/.test(plain)) code = plain;
    try {
      const value = JSON.parse(result.stderr);
      if (/^[A-Z0-9_]{1,96}$/.test(value.code)) code = value.code;
    } catch {}
    assert.fail(code);
  }
  return JSON.parse(result.stdout).reference;
}

test(
  '真实准备与Doc封存：原号不换L，原条目可重生且不混入后来文档',
  { timeout: 180000 },
  async (t) => {
    // 失败重跑也续接原操作；这些记录只含无秘密引用/ID，不保存账号凭据或私钥。
    let state;
    if (existsSync(checkpoint)) state = JSON.parse(readFileSync(checkpoint));
    else {
      const fixture = await organizationFixture(0);
      const document = await documentFixture(fixture);
      const db = await searchDatabase();
      try {
        const [[row]] = await db.query(
          'SELECT CAST(generation AS CHAR) generation FROM index_generation',
        );
        state = {
          operation: 'rebuild-' + randomUUID(),
          generation: String(BigInt(row.generation) + 1n),
          first: String(document.id),
        };
        save(state);
      } finally {
        await db.end();
      }
    }
    const args = ['--operation', state.operation, '--generation', state.generation];
    const prepare = await command('search', 'search-rebuild-prepare', args);
    assert.equal(await command('search', 'search-rebuild-prepare', args), prepare);
    const prepareName = prepare.split(':')[1] + '.prepare.json';
    const source = resolve(runtime, 'rebuild/search/outbox', prepareName);
    assert.equal(hash(readFileSync(source)), prepare.split(':')[1]);
    const inbox = resolve(runtime, 'rebuild/document/inbox');
    mkdirSync(inbox, { recursive: true });
    copyFileSync(source, resolve(inbox, prepareName));
    const snapshotArgs = ['--operation', state.operation, '--prepare-ref', prepare];
    const seal = await command('document', 'search-snapshot', snapshotArgs);
    const base = seal.split(':')[1];
    const entriesFile = resolve(runtime, 'rebuild/document/outbox', base + '.entries.ndjson');
    const sealFile = resolve(runtime, 'rebuild/document/outbox', base + '.snapshot.json');
    const entries = readFileSync(entriesFile);
    const envelope = readFileSync(sealFile);
    assert.equal(hash(envelope), base);
    const payload = JSON.parse(Buffer.from(JSON.parse(envelope).payload, 'base64'));
    assert.equal(payload.prepare_ref, prepare);
    assert.equal(payload.entries_digest, hash(entries));
    const rows = entries.toString('utf8').trimEnd().split('\n').filter(Boolean).map(JSON.parse);
    assert.equal(String(rows.length), payload.entry_count);
    assert.ok(
      rows.some((row) => row.document_id === state.first),
      'ORIGINAL_DOCUMENT_MISSING',
    );
    const later = await documentFixture(await organizationFixture(0));
    assert.equal(await command('document', 'search-snapshot', snapshotArgs), seal);
    assert.deepEqual(readFileSync(entriesFile), entries);
    assert.equal(
      rows.some((row) => row.document_id === String(later.id)),
      false,
    );
    // 只移除本测试原引用的导出副本，数据库封存条目保留；原号重跑必须重生相同字节。
    unlinkSync(entriesFile);
    unlinkSync(sealFile);
    assert.equal(await command('document', 'search-snapshot', snapshotArgs), seal);
    assert.deepEqual(readFileSync(entriesFile), entries);
    assert.deepEqual(readFileSync(sealFile), envelope);
    state.prepare = prepare;
    state.snapshot = seal;
    state.count = payload.entry_count;
    save(state);
    t.diagnostic(
      '真实证书、私有库、原号重试和原条目导出恢复通过；保留同一准备操作供后续导入/重建继续。',
    );
  },
);
