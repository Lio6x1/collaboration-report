import test from 'node:test';
import assert from 'node:assert/strict';
import { accountUnleasedResponse } from '../../scripts/account-output-policy.mjs';
test('sensitive invitation and directory fields cannot hide inside an unleased operation receipt', () => {
  const receipt = {
    operation_id: 'operation-123456',
    state: 'SUCCEEDED',
    result: { entity_id: '12' },
  };
  assert.equal(accountUnleasedResponse('/v1/organizations', JSON.stringify(receipt)), true);
  for (const extra of [
    { share_url: 'secret' },
    { items: [{ name: 'secret' }] },
    { result: { entity_id: '12', email: 'secret' } },
  ])
    assert.equal(
      accountUnleasedResponse('/v1/organizations', JSON.stringify({ ...receipt, ...extra })),
      false,
    );
  assert.equal(
    accountUnleasedResponse('/v1/account/me', JSON.stringify({ id: '9', email: 'secret' })),
    false,
  );
});

test('only canonical original closure plans cross the frozen-session output boundary', () => {
  const plan = {
    operation_id: 'account-close-12345678',
    state: 'CLEANING',
    required_successions: [],
    retained_release_ids: ['12'],
  };
  const path = '/v1/account/closures/' + plan.operation_id;
  const raw = JSON.stringify(plan);
  assert.equal(accountUnleasedResponse(path, raw + '\n'), true);
  assert.equal(accountUnleasedResponse('/v1/account/closures', raw), true);
  for (const body of [
    JSON.stringify({ ...plan, email: 'secret' }),
    raw.replace('"state":', '"state":"secret-prefix","state":'),
    JSON.stringify({ ...plan, retained_release_ids: ['18446744073709551616'] }),
    JSON.stringify({
      ...plan,
      required_successions: [{ kind: 'ORGANIZATION_OWNER', target_id: '2', name: 'secret' }],
    }),
  ])
    assert.equal(accountUnleasedResponse(path, body), false);
  assert.equal(accountUnleasedResponse('/v1/organizations', raw), false);
  assert.equal(accountUnleasedResponse(path + '-another', raw), false);
});
