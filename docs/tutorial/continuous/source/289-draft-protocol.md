# services/collaboration/src/domain/draft_protocol.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/289-draft-protocol.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`draftPreviewBinding`、`draftBody`、`draftLineage`、`fixedDraft`、`computePull`、`computeDraftPreview`、`draftPreviewCutDigest`、`draftPreparedResult`、`validateDraftInstall`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { canonical, normalizeContent, resourceIds } from '../../../../packages/content/index.mjs';
import { validID, bad } from './shared_protocol.mjs';
import { hash } from './shared_protocol.mjs';
import { captureProvenance, storeContent } from './capture_protocol.mjs';
import { mergeContent } from './content_merge.mjs';
import { RPCFailure } from './shared_protocol.mjs';

export function draftPreviewBinding(input) {
  const context = input.context,
    target = context?.target;
  const rawFingerprint = context?.request_fingerprint;
  const fingerprint =
    typeof rawFingerprint === 'string'
      ? Buffer.from(rawFingerprint, 'base64')
      : Buffer.from(rawFingerprint ?? []);
  const positive = (value) =>
    typeof value === 'string' &&
    /^[1-9][0-9]{0,19}$/.test(value) &&
    BigInt(value) <= 18446744073709551615n;
  if (
    !validID(context?.operation_id) ||
    !positive(context?.actor_id) ||
    !['PULL_MAIN', 'DISCARD_PREVIEW', 'REVERSE', 'CONFLICT_RESOLUTION'].includes(target?.purpose) ||
    !['document_id', 'unit_id', 'branch_id', 'structure_id', 'workspace_epoch'].every((key) =>
      positive(target[key]),
    ) ||
    input.expected_branch_id !== target.branch_id ||
    input.expected_structure_id !== target.structure_id ||
    !positive(input.expected_lineage_id) ||
    fingerprint.length !== 32 ||
    (typeof rawFingerprint === 'string' && fingerprint.toString('base64') !== rawFingerprint) ||
    !context.identity_admission_ref ||
    !context.document_admission_ref
  )
    throw bad('DRAFT_PREVIEW_INVALID');
  return {
    ...input,
    context: {
      ...context,
      request_fingerprint: fingerprint.toString('base64'),
    },
  };
}

// 支线由样式修订控制字体；拉取正文不能把 MAIN 独立字体标记注入禁止该能力的草稿。
export function draftBody(body) {
  const value = structuredClone(body);
  const visit = (node) => {
    if (node.marks) {
      node.marks = node.marks.filter((mark) => mark.type !== 'font');
      if (!node.marks.length) delete node.marks;
    }
    for (const child of node.content ?? []) visit(child);
  };
  visit(value);
  return normalizeContent(value);
}

// 真实MAIN基线可能带管理字体；旧固定来源只在读取比较时投影，不改历史artifact的字节。
// 版本/谱系/已接纳切点保持原身份。客户端输入仍必须经过严格BRANCH正文校验。
export function draftLineage(lineage) {
  return { ...lineage, body: draftBody(lineage.body) };
}

export function fixedDraft(body, context, cut, lineage, ref) {
  const bytes = Buffer.from(canonical(body));
  return storeContent({
    document_id: context.target.document_id,
    unit_id: context.target.unit_id,
    branch_id: context.target.branch_id,
    structure_id: context.target.structure_id,
    workspace_epoch: context.target.workspace_epoch,
    capture_id: ref,
    operation_id: context.operation_id,
    lineage_ref: lineage.lineage_id,
    cut_json_utf8: Buffer.from(canonical(cut)),
    body_json_utf8: bytes,
    schema_version: 'content-v1',
    canonicalization_version: 'canonical-v1',
    hash_algorithm: 'SHA-256',
    body_hash: hash(bytes),
    byte_size: String(bytes.length),
    resource_ids: resourceIds(body),
    provenance_json_utf8: Buffer.from(
      canonical(captureProvenance(body, lineage, context.operation_id)),
    ),
    provenance_schema: 'provenance-v1',
  });
}

export function computePull(base, draft, main) {
  const result = mergeContent({
    base: draftBody(base),
    submitted: draftBody(draft),
    main: draftBody(main),
  });
  return { ...result, body: result.body && draftBody(result.body) };
}

// 策略由Doc原操作用途决定。丢弃预览保留旧稿，候选仅为同一可靠MAIN；拉取仍沿原三方算法。
export function computeDraftPreview(purpose, base, draft, main) {
  if (purpose === 'PULL_MAIN') return computePull(base, draft, main);
  if (purpose === 'DISCARD_PREVIEW') return { body: draftBody(main), conflicts: [] };
  throw bad('DRAFT_PREVIEW_PURPOSE_INVALID');
}

// Go的CutBinding按数值room_id计算依赖摘要。房间ID不是创建时间，不能假设BRANCH总排在MAIN前。
// 原预览的cut数组/完整input_digest不改写；旧PULL协议保留它原来的摘要排列。
export function draftPreviewCutDigest(purpose, cuts) {
  const values =
    purpose === 'DISCARD_PREVIEW'
      ? [...cuts].sort((left, right) =>
          BigInt(left.room_id) < BigInt(right.room_id)
            ? -1
            : BigInt(left.room_id) > BigInt(right.room_id)
              ? 1
              : 0,
        )
      : cuts;
  return hash(canonical(values)).toString('base64');
}

export function draftPreparedResult(fixed) {
  const result = {
    state: 'PREPARED',
    preview_cut_ref: fixed.preview_cut_ref,
    input_digest: fixed.input_digest,
    draft_cut: fixed.draft_cut,
    candidate: fixed.candidate,
    main: fixed.main,
  };
  // 预算按真实base64 JSON传输测量，并预留其余协议字段；超限发生在房间BLOCKED之前。
  if (Buffer.byteLength(JSON.stringify(result)) > 24 << 20)
    throw new RPCFailure(8, 'DRAFT_TRANSFER_LIMIT');
  return result;
}

// 新代次只允许重编码原固定候选与真实MAIN基线，不能因两个Go输入彼此一致就替换可靠正文。
export function validateDraftInstall(fixed, source) {
  const conflict = () => {
    throw new RPCFailure(6, 'DRAFT_INSTALL_CONFLICT');
  };
  const target = fixed.binding.context.target,
    content = source.content,
    candidate = fixed.candidate;
  if (
    !content ||
    !candidate ||
    !source.lineage ||
    source.old_room_id !== fixed.room_id ||
    source.new_epoch !== (BigInt(target.workspace_epoch) + 1n).toString() ||
    content.workspace_epoch !== source.new_epoch ||
    source.new_room_id === fixed.room_id ||
    !['document_id', 'unit_id', 'branch_id', 'structure_id'].every(
      (key) => content[key] === target[key],
    ) ||
    content.operation_id !== candidate.operation_id ||
    content.capture_id !== candidate.capture_id ||
    content.lineage_ref !== source.lineage.lineage_id ||
    source.lineage.base_manifest_id !==
      (target.purpose === 'CONFLICT_RESOLUTION'
        ? fixed.source.resolution_base_manifest_id
        : fixed.source.main_manifest_id)
  )
    conflict();
  for (const key of [
    'body_json_utf8',
    'body_hash',
    'byte_size',
    'resource_ids',
    'schema_version',
    'canonicalization_version',
    'hash_algorithm',
    'provenance_schema',
  ]) {
    if (canonical(content[key]) !== canonical(candidate[key])) conflict();
  }
  const bytes = Buffer.from(content.body_json_utf8, 'base64');
  if (
    hash(bytes).toString('base64') !== content.body_hash ||
    String(bytes.length) !== content.byte_size ||
    bytes.length > 16 << 20
  )
    conflict();
  const body = normalizeContent(JSON.parse(bytes));
  if (
    canonical(body) !== bytes.toString() ||
    canonical(source.lineage.body) !==
      (target.purpose === 'CONFLICT_RESOLUTION'
        ? canonical(draftBody(JSON.parse(wireResolutionBaseline(fixed))))
        : Buffer.from(fixed.main.body_json_utf8, 'base64').toString()) ||
    (target.purpose === 'CONFLICT_RESOLUTION' &&
      source.lineage.base_version_id !== fixed.source.resolution_base_version_id)
  )
    conflict();
  const expectedCut = {
    room_id: source.new_room_id,
    workspace_epoch: source.new_epoch,
    durable_seq: '0',
  };
  if (Buffer.from(content.cut_json_utf8, 'base64').toString() !== canonical(expectedCut))
    conflict();
  const original = JSON.parse(Buffer.from(candidate.provenance_json_utf8, 'base64'));
  const provenance = JSON.parse(Buffer.from(content.provenance_json_utf8, 'base64'));
  if (
    canonical(provenance) !== canonical(source.lineage.provenance) ||
    provenance.lineage_id !== content.lineage_ref ||
    provenance.base_manifest_id !== source.lineage.base_manifest_id ||
    provenance.base_content_version_id !== source.lineage.base_version_id ||
    provenance.source_operation_id !== candidate.operation_id ||
    canonical(provenance.accepted_captures) !== canonical(original.accepted_captures) ||
    canonical(provenance.block_sources) !== canonical(original.block_sources)
  )
    conflict();
}

function wireResolutionBaseline(fixed) {
  const raw = fixed.source.resolution_baseline?.body_json_utf8;
  if (typeof raw !== 'string') throw new RPCFailure(6, 'DRAFT_INSTALL_CONFLICT');
  return Buffer.from(raw, 'base64').toString();
}
````

UTF-8/LF全文SHA-256：`ea07c46ec9aa06911f2046dc3dfcbad439091f3eb7aa3d04b7589d9ff27112a0`。[参考实现](../../../../services/collaboration/src/domain/draft_protocol.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
