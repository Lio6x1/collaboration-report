import test from 'node:test';
import assert from 'node:assert/strict';
import { Readiness } from '../src/application/readiness.mjs';

const base = '9c9ea8ea0a1ef726f4fa205af99402ce6a88ee25a7d3049842eb8187eaac6d8d';
const index = '1'.repeat(64);
const repository = (versions) => ({
  async versions() {
    return versions;
  },
  async pendingTasks() {
    return 0;
  },
  async pendingGates() {
    return 0;
  },
  async unboundDraftMainGates() {
    return 0;
  },
  async rooms() {
    return 0;
  },
});

test('readiness accepts every configured migration with its original checksum', async () => {
  // SQL没有ORDER BY；完整集合核验不能把实际行返回顺序当版本身份。
  await new Readiness(
    repository([
      { version: 2, checksum: index },
      { version: 1, checksum: base },
    ]),
    base,
    { 2: index },
  ).check({});
});

for (const [name, additional, versions] of [
  ['missing additional', { 2: index }, [{ version: 1, checksum: base }]],
  [
    'changed additional',
    { 2: index },
    [
      { version: 1, checksum: base },
      { version: 2, checksum: base },
    ],
  ],
  [
    'unknown additional',
    {},
    [
      { version: 1, checksum: base },
      { version: 2, checksum: index },
    ],
  ],
  [
    'duplicate version',
    { 2: index },
    [
      { version: 1, checksum: base },
      { version: 1, checksum: index },
    ],
  ],
  [
    'noncontiguous expected set',
    { 3: index },
    [
      { version: 1, checksum: base },
      { version: 3, checksum: index },
    ],
  ],
])
  test(`readiness rejects ${name}`, async () => {
    await assert.rejects(
      new Readiness(repository(versions), base, additional).check({}),
      /SCHEMA_VERSION_MISMATCH/,
    );
  });
