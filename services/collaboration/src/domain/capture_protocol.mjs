import { canonical } from '../../../../packages/content/index.mjs';
import { validID, bad } from './shared_protocol.mjs';
import { RPCFailure } from './shared_protocol.mjs';

const positive = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;
export function captureBinding(input) {
  const context = input.context,
    target = context?.target;
  if (
    !validID(context?.operation_id) ||
    !positive(context?.actor_id) ||
    !target ||
    !['document_id', 'unit_id', 'branch_id', 'structure_id', 'workspace_epoch'].every((key) =>
      positive(target[key]),
    ) ||
    target.purpose !== 'SUBMISSION' ||
    !validID(input.flush_receipt_id) ||
    !positive(input.guard_generation) ||
    !positive(input.lineage_ref) ||
    Buffer.from(context.request_fingerprint ?? []).length !== 32 ||
    !context.document_admission_ref ||
    !context.identity_admission_ref
  )
    throw bad('CAPTURE_INVALID');
  return {
    context: {
      ...context,
      request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
    },
    flush_receipt_id: input.flush_receipt_id,
    guard_generation: input.guard_generation,
    lineage_ref: input.lineage_ref,
  };
}

export function sameBinding(expected, actual) {
  if (canonical(expected) !== canonical(actual)) throw new RPCFailure(6, 'CAPTURE_ID_CONFLICT');
}

export function matchCaptureQuery(row, binding, query) {
  if (
    row.operation_id !== query.operation_id ||
    row.request_fingerprint.toString('base64') !==
      Buffer.from(query.request_fingerprint ?? []).toString('base64') ||
    canonical(binding.context.target) !== canonical(query.target)
  )
    throw new RPCFailure(6, 'CAPTURE_ID_CONFLICT');
}

const binaryFields = ['cut_json_utf8', 'body_json_utf8', 'body_hash', 'provenance_json_utf8'];
export function storeContent(content) {
  return {
    ...content,
    ...Object.fromEntries(
      binaryFields.map((key) => [key, Buffer.from(content[key]).toString('base64')]),
    ),
  };
}
export function wireContent(content) {
  return {
    ...content,
    ...Object.fromEntries(binaryFields.map((key) => [key, Buffer.from(content[key], 'base64')])),
  };
}

// 来源描述保留真实旧基线和已接纳切点；新增块才把当前capture作为来源。
export function captureProvenance(body, lineage, operation) {
  const baseBlocks = new Map(),
    currentBlocks = new Map();
  const collect = (node, map) => {
    if (node.attrs?.id) map.set(node.attrs.id, node);
    for (const child of node.content ?? []) collect(child, map);
  };
  collect(lineage.body, baseBlocks);
  collect(body, currentBlocks);
  const previous = new Map(
    lineage.provenance.block_sources.map((source) => [source.block_id, source]),
  );
  const blockSources = [];
  for (const [id, node] of currentBlocks) {
    const base = baseBlocks.get(id),
      old = previous.get(id);
    if (base && old && canonical(base) === canonical(node)) blockSources.push(structuredClone(old));
    else
      blockSources.push({
        block_id: id,
        origin_block_id: id,
        ...(base
          ? { origin_content_version_id: lineage.base_version_id }
          : { origin_capture_ref: operation }),
        relation: base ? 'EDITED' : 'INSERTED',
        source_range: { start: 0, end: 0 },
      });
  }
  for (const [id] of baseBlocks)
    if (!currentBlocks.has(id))
      blockSources.push({
        block_id: id,
        origin_block_id: id,
        origin_content_version_id: lineage.base_version_id,
        relation: 'DELETED',
        source_range: { start: 0, end: 0 },
      });
  return {
    schema_version: 'provenance-v1',
    lineage_id: lineage.lineage_id,
    base_manifest_id: lineage.base_manifest_id,
    base_content_version_id: lineage.base_version_id,
    source_operation_id: operation,
    accepted_captures: structuredClone(lineage.provenance.accepted_captures),
    block_sources: blockSources,
  };
}
