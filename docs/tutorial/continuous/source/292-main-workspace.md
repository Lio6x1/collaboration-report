# services/collaboration/src/domain/main_workspace.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/292-main-workspace.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`matchesMainWorkspace`、`preparedMainSource`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`bb3f8f6b4cdd3d02be0b465f483652cb571c62dadc4f8e6f5a6dd3e943024dca`。[参考实现](../../../../services/collaboration/src/domain/main_workspace.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
