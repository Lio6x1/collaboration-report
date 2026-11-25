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
