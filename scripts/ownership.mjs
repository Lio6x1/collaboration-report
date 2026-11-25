export function assertContainerOwner(actual, expected) {
  if (
    !expected.owner ||
    !expected.project ||
    actual.owner !== expected.owner ||
    actual.project !== expected.project
  )
    throw Error('CONTAINER_OWNER_MISMATCH');
}
