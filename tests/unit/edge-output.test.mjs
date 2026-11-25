import test from 'node:test';
import assert from 'node:assert/strict';
import { EdgeOutputs } from '../../scripts/edge-outputs.mjs';

test('publication proxy gate accepts only canonical uint64 and waits for the original output', async () => {
  const outputs = new EdgeOutputs({ load: () => ({}), save: () => {} });
  const source = { scope_key: 'publication:18446744073709551615', epoch: '9007199254740993' };
  let done,
    stopped = false,
    completed = false;
  outputs.attach('publication-handler', {
    stop: () => {
      stopped = true;
    },
    closed: new Promise((resolve) => {
      done = resolve;
    }),
  });
  outputs.open({
    handler_id: 'publication-handler',
    lease_id: 'publication-lease',
    sources: [source],
  });
  const closing = outputs.fence(source).then(() => {
    completed = true;
  });
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(stopped, true);
  assert.equal(completed, false);
  done();
  await closing;
  assert.equal(completed, true);
  for (const scope_key of [
    'publication:0',
    'publication:01',
    'publication:+1',
    'publication:1:2',
    'publication:18446744073709551616',
  ]) {
    await assert.rejects(outputs.fence({ scope_key, epoch: '1' }), /SOURCE_INVALID/);
  }
  for (const epoch of ['0', '01', '+1', '18446744073709551616']) {
    await assert.rejects(outputs.fence({ scope_key: 'publication:1', epoch }), /SOURCE_INVALID/);
  }
});

test('fence waits for actual proxy settlement and rejects a late old admission', async () => {
  const outputs = new EdgeOutputs({ load: () => ({}), save: () => {} }),
    source = { scope_key: 'credential:7', epoch: '4' };
  let stopped = false,
    settle;
  const closed = new Promise((resolve) => {
    settle = resolve;
  });
  outputs.attach('handler-1', {
    stop: () => {
      stopped = true;
    },
    closed,
  });
  outputs.open({ handler_id: 'handler-1', lease_id: 'lease-1', sources: [source] });
  let acknowledged = false;
  const fencing = outputs.fence({ scope_key: source.scope_key, epoch: '4' }).then(() => {
    acknowledged = true;
  });
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(stopped, true);
  assert.equal(acknowledged, false);
  assert.throws(
    () => outputs.open({ handler_id: 'handler-1', lease_id: 'late-lease', sources: [source] }),
    /SOURCE_CLOSED/,
  );
  settle();
  await fencing;
  assert.equal(acknowledged, true);
});
test('a confirmed higher generation opens without an old close blocking it again', async () => {
  let persisted = {};
  const outputs = new EdgeOutputs({
    load: () => persisted,
    save: (value) => {
      persisted = structuredClone(value);
    },
  });
  await outputs.fence({ scope_key: 'org-member:2:7', epoch: '3' });
  outputs.attach('new', { stop: () => {}, closed: Promise.resolve() });
  outputs.open({
    handler_id: 'new',
    lease_id: 'new-lease',
    sources: [{ scope_key: 'org-member:2:7', epoch: '4' }],
  });
  await outputs.fence({ scope_key: 'org-member:2:7', epoch: '3' });
  assert.equal(outputs.allowed('new'), true);
  const restored = new EdgeOutputs({ load: () => persisted, save: () => {} });
  restored.attach('old', { stop: () => {}, closed: Promise.resolve() });
  assert.throws(
    () =>
      restored.open({
        handler_id: 'old',
        lease_id: 'old-lease',
        sources: [{ scope_key: 'org-member:2:7', epoch: '3' }],
      }),
    /SOURCE_CLOSED/,
  );
});
test('an unknown handler or altered registration never becomes a live proxy lease', () => {
  const outputs = new EdgeOutputs({ load: () => ({}), save: () => {} });
  assert.throws(
    () =>
      outputs.open({
        handler_id: 'absent',
        lease_id: 'lease',
        sources: [{ scope_key: 'identity:9', epoch: '1' }],
      }),
    /HANDLER_UNKNOWN/,
  );
  outputs.attach('one', { stop: () => {}, closed: Promise.resolve() });
  outputs.open({
    handler_id: 'one',
    lease_id: 'lease',
    sources: [{ scope_key: 'identity:9', epoch: '1' }],
  });
  assert.throws(
    () =>
      outputs.open({
        handler_id: 'one',
        lease_id: 'other',
        sources: [{ scope_key: 'identity:9', epoch: '1' }],
      }),
    /LEASE_CONFLICT/,
  );
});
