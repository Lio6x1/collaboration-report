# services/collaboration/src/domain/discard_confirmation.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/288-discard-confirmation.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`deriveDiscardConfirmation`、`sameReliableMainCut`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { canonical } from '../../../../packages/content/index.mjs';
import { hash } from './shared_protocol.mjs';
import { RPCFailure } from './shared_protocol.mjs';
import { validID } from './shared_protocol.mjs';

// 只派生新确认身份；正文、旧cut与历史来源仍来自原固定预览，原包不改写。
export function deriveDiscardConfirmation(preview, binding, authority) {
  const original = preview.binding?.context;
  const context = binding.context;
  if (
    original?.target?.purpose !== 'DISCARD_PREVIEW' ||
    context?.target?.purpose !== 'DISCARD_DRAFT' ||
    authority.purpose !== 'DISCARD_DRAFT' ||
    !validID(context.operation_id) ||
    context.operation_id === original.operation_id ||
    context.actor_id !== original.actor_id ||
    preview.preview_cut_ref !== authority.preview_cut_ref ||
    preview.input_digest !== authority.preview_input_digest ||
    canonical(original) !== canonical(authority.preview_context) ||
    !['document_id', 'unit_id', 'branch_id', 'structure_id', 'workspace_epoch'].every(
      (key) => context.target[key] === original.target[key],
    ) ||
    !['expected_branch_id', 'expected_structure_id', 'expected_lineage_id'].every(
      (key) => binding[key] === preview.binding[key],
    ) ||
    Buffer.from(context.request_fingerprint ?? '', 'base64').length !== 32 ||
    !context.identity_admission_ref ||
    !context.document_admission_ref ||
    !/^(0|[1-9][0-9]{0,19})$/.test(authority.expected_round_id)
  )
    throw new RPCFailure(6, 'DISCARD_CONFIRMATION_CONFLICT');
  const rebind = (content, suffix) => {
    const value = structuredClone(content);
    const provenance = JSON.parse(Buffer.from(value.provenance_json_utf8, 'base64'));
    provenance.source_operation_id = context.operation_id;
    for (const source of provenance.block_sources) {
      if (source.origin_capture_ref === original.operation_id)
        source.origin_capture_ref = context.operation_id;
    }
    value.operation_id = context.operation_id;
    value.capture_id = context.operation_id + suffix;
    value.provenance_json_utf8 = Buffer.from(canonical(provenance)).toString('base64');
    return value;
  };
  const fixed = {
    ...structuredClone(preview),
    binding,
    preview_input_digest: preview.input_digest,
    preview_binding: preview.binding,
    expected_round_id: authority.expected_round_id,
    draft: rebind(preview.draft, ''),
    main: rebind(preview.main, ':main'),
    candidate: rebind(preview.candidate, ':candidate'),
  };
  delete fixed.input_digest;
  fixed.input_digest = hash(canonical(fixed)).toString('base64');
  return fixed;
}

// 迟到的零尾MAIN房间也属于变化；不能仅比较已有房间的序号。
export function sameReliableMainCut(fixed, rows) {
  const actual = rows.map((room) => ({
    room_id: room.id,
    workspace_epoch: room.epoch,
    durable_seq: room.last_durable_seq,
  }));
  return (
    rows.every((room) => room.state === 'OPEN') && canonical(actual) === canonical(fixed.main_cut)
  );
}
````

UTF-8/LF全文SHA-256：`97465c44857d286f4cddc56f5ece50d8f127d8775685a3c87da7c64ee4207ec8`。[参考实现](../../../../services/collaboration/src/domain/discard_confirmation.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
