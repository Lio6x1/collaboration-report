import test from 'node:test';
import assert from 'node:assert/strict';
import { requireBranchOpen } from '../src/application/admission_gates.mjs';

function connection(gate) {
  const calls = [];
  return {
    calls,
    async query({ sql }, args) {
      calls.push({ sql, args });
      if (sql.includes("kind='STRUCTURE_BRANCH_GATE'"))
        return [[...(gate ? [{ payload: gate }] : [])]];
      return [[]];
    },
  };
}

test('branch entry blocks prepared and retired identities before any room row is read', async () => {
  for (const state of ['PREPARED', 'INSTALLED']) {
    const c = connection({
      document_id: '3',
      operation_id: 'original-confirm',
      generation: '2',
      state,
    });
    await assert.rejects(
      requireBranchOpen(c, '3', '11', '1'),
      new RegExp(state === 'PREPARED' ? 'STRUCTURE_BRANCH_GATE_PENDING' : 'WORKSPACE_CHANGED'),
    );
    assert.equal(c.calls[0].args[0], 'main:3');
    assert.equal(c.calls.at(-1).args[0], '11:1');
    assert.ok(c.calls.every((v) => !v.sql.includes(' FROM room ')));
  }
});

test('unaffected branches and the original aborted preparation remain open', async () => {
  for (const gate of [
    null,
    { document_id: '3', operation_id: 'original-confirm', generation: '2', state: 'ABORTED' },
  ]) {
    await requireBranchOpen(connection(gate), '3', '11', '1');
  }
  for (const gate of [
    { document_id: '4', state: 'ABORTED' },
    { document_id: '3', state: 'UNKNOWN' },
  ]) {
    await assert.rejects(
      requireBranchOpen(connection(gate), '3', '11', '1'),
      /STRUCTURE_BRANCH_GATE_UNKNOWN/,
    );
  }
});
