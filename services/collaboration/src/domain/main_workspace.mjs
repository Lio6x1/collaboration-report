import { canonical } from '../../../../packages/content/index.mjs';

const positive = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;

// Doc管理修订可以只改变标题/负责人；这不改变物理MAIN房间。
// 允许该差异的依据是权威manifest的实际workspace_epoch，而不是任意“小于当前control”的房间。
export function matchesMainWorkspace(current, room, source) {
  if (
    !current ||
    !room ||
    !source ||
    !positive(room.epoch) ||
    !positive(source.workspace_epoch) ||
    !positive(source.control_revision) ||
    BigInt(source.workspace_epoch) > BigInt(source.control_revision)
  )
    return false;
  return (
    current.control_revision === room.epoch &&
    room.epoch === source.workspace_epoch &&
    current.manifest_id === source.manifest_id &&
    current.structure_id === source.structure_id &&
    current.style_id === source.style_id
  );
}

// 只比较独立持久捕获的标量绑定与完整cut；准备请求不能自己给旧代次背书。
export function preparedMainSource(binding, facts) {
  const expected = binding.expected_cut;
  const fact = facts.find(
    (candidate) =>
      positive(candidate.manifest) &&
      candidate.control === expected.control_revision &&
      canonical(candidate.context) === canonical(binding.context) &&
      canonical(candidate.cut) === canonical(expected),
  );
  return fact
    ? {
        manifest_id: fact.manifest,
        structure_id: expected.structure_id,
        style_id: expected.style_id,
        control_revision: expected.control_revision,
      }
    : null;
}
