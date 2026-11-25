# services/collaboration/src/domain/structure_preparation.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/304-structure-preparation.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`structurePreparation`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { canonical } from '../../../../packages/content/index.mjs';
import { hash } from './shared_protocol.mjs';
import { cutDigest } from './main_protocol.mjs';
import { RPCFailure } from './shared_protocol.mjs';

// 从同一固定预览缩小准备范围，不重采样、不改变候选。没有实体房间的支线也需要持久入口屏障。
export function structurePreparation(fixed, authority, binding, now = Date.now()) {
  const unknown = () => {
    throw new RPCFailure(14, 'STRUCTURE_PREPARATION_UNKNOWN');
  };
  if (
    !fixed ||
    !authority ||
    !binding?.context ||
    !fixed.binding?.context ||
    !fixed.observed_cut ||
    !Array.isArray(fixed.observed_cut.rooms) ||
    !Array.isArray(fixed.source?.drafts) ||
    !Array.isArray(fixed.migration?.changes) ||
    fixed.migration.schema_version !== 'structure-migration-v1'
  )
    unknown();
  const { input_digest: digest, ...original } = fixed;
  if (
    hash(canonical(original)).toString('base64') !== digest ||
    authority.digest !== digest ||
    authority.reference !== fixed.preview_cut_ref ||
    authority.expires !== fixed.expires_unix_ms
  )
    unknown();
  if (!/^[1-9][0-9]{0,15}$/.test(authority.expires)) unknown();
  if (Number(authority.expires) <= now) throw new RPCFailure(9, 'STRUCTURE_PREVIEW_EXPIRED');
  const prior = fixed.binding.context,
    current = binding.context;
  if (
    current.target?.purpose !== 'MAIN_CONTROL' ||
    current.actor_id !== prior.actor_id ||
    current.target.document_id !== prior.target.document_id ||
    current.target.structure_id !== prior.target.structure_id ||
    current.target.workspace_epoch !== prior.target.workspace_epoch ||
    canonical(current.sources) !== canonical(prior.sources)
  )
    unknown();
  const affected = new Set(fixed.migration.changes.flatMap((v) => v.source_units));
  if (!affected.size) unknown();
  const drafts = new Map(),
    roomOwners = new Map();
  for (const unit of fixed.source.drafts) {
    if (
      !unit?.unit_id ||
      !unit.branch_id ||
      !unit.room_id ||
      !unit.workspace_epoch ||
      !unit.lineage ||
      drafts.has(unit.unit_id) ||
      roomOwners.has(unit.room_id)
    )
      unknown();
    drafts.set(unit.unit_id, unit);
    roomOwners.set(unit.room_id, unit.unit_id);
  }
  const rooms = new Map(fixed.observed_cut.rooms.map((v) => [v.room_id, v]));
  if (
    rooms.size !== fixed.observed_cut.rooms.length ||
    cutDigest(fixed.observed_cut.rooms) !== fixed.observed_cut.dependency_digest
  )
    unknown();
  const branches = [...affected].map((id) => {
    const unit = drafts.get(id);
    if (!unit) unknown();
    const cut = rooms.get(unit.room_id);
    if (cut && cut.workspace_epoch !== unit.workspace_epoch) unknown();
    return { ...unit, present: !!cut, ...(cut ? { cut } : {}) };
  });
  const selected = fixed.observed_cut.rooms.filter(
    (v) => !roomOwners.has(v.room_id) || affected.has(roomOwners.get(v.room_id)),
  );
  return {
    cut: { ...fixed.observed_cut, rooms: selected, dependency_digest: cutDigest(selected) },
    branches,
  };
}
````

UTF-8/LF全文SHA-256：`e9db9a4cd497bd110f1e70ed399c991f117ddfdcd02ffe79c975a20dc5fc6438`。[参考实现](../../../../services/collaboration/src/domain/structure_preparation.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
