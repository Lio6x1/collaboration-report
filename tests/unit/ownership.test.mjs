import test from 'node:test';
import assert from 'node:assert/strict';
import { assertContainerOwner } from '../../scripts/ownership.mjs';
const expected = { owner: 'report-dev-example', project: 'reportdevexample' };
test('management refuses a container from another project', () =>
  assert.throws(
    () => assertContainerOwner({ owner: expected.owner, project: 'other-project' }, expected),
    /CONTAINER_OWNER_MISMATCH/,
  ));
test('management refuses a container without its owner marker', () =>
  assert.throws(
    () => assertContainerOwner({ project: expected.project }, expected),
    /CONTAINER_OWNER_MISMATCH/,
  ));
test('management accepts only both matching ownership markers', () =>
  assert.doesNotThrow(() => assertContainerOwner(expected, expected)));
