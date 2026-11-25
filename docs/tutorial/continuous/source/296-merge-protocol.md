# services/collaboration/src/domain/merge_protocol.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/296-merge-protocol.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`fixedContentBinding`、`mergeBinding`、`validateMergeInput`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { readFileSync } from 'node:fs';
import Ajv from 'ajv/dist/2020.js';
import { canonical, normalizeContent, resourceIds } from '../../../../packages/content/index.mjs';
import { hash } from './shared_protocol.mjs';
import { bad, validID } from './shared_protocol.mjs';

const provenanceSchema = JSON.parse(
  readFileSync(new URL('../../../../contracts/content/provenance.schema.json', import.meta.url)),
);
// oneOf/not的required引用父层已声明属性；关闭仅该编译告警，实际required/oneOf/未知字段校验不变。
const validateProvenance = new Ajv({ strict: true, strictRequired: false }).compile(
  provenanceSchema,
);
const positive = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;
const binary = (value) => Buffer.from(value ?? []);
const binaryHash = (value) => hash(binary(value)).toString('base64');
const targetKeys = ['document_id', 'unit_id', 'branch_id', 'structure_id', 'workspace_epoch'];
export function fixedContentBinding(content) {
  return {
    ...Object.fromEntries(targetKeys.map((key) => [key, content[key] ?? '0'])),
    capture_id: content.capture_id ?? '',
    operation_id: content.operation_id ?? '',
    lineage_ref: content.lineage_ref ?? '',
    schema_version: content.schema_version ?? '',
    canonicalization_version: content.canonicalization_version ?? '',
    hash_algorithm: content.hash_algorithm ?? '',
    byte_size: content.byte_size ?? '0',
    body_hash: binary(content.body_hash).toString('base64'),
    cut_digest: binaryHash(content.cut_json_utf8),
    provenance_digest: binaryHash(content.provenance_json_utf8),
    provenance_schema: content.provenance_schema ?? '',
    resource_ids: content.resource_ids ?? [],
  };
}
export function mergeBinding(input) {
  const context = input.context,
    cut = input.main_cut;
  return {
    attempt_id: input.attempt_id,
    context: {
      operation_id: context.operation_id,
      actor_id: context.actor_id,
      identity_admission_ref: context.identity_admission_ref,
      document_admission_ref: context.document_admission_ref,
      target: {
        ...Object.fromEntries(targetKeys.map((key) => [key, context.target[key] ?? '0'])),
        purpose: context.target.purpose,
      },
      sources: [...context.sources].sort((a, b) =>
        Buffer.compare(Buffer.from(a.scope_key), Buffer.from(b.scope_key)),
      ),
      request_fingerprint: binary(context.request_fingerprint).toString('base64'),
    },
    base: fixedContentBinding(input.base),
    submitted: fixedContentBinding(input.submitted),
    main: fixedContentBinding(input.main),
    main_cut: {
      document_id: cut.document_id,
      structure_id: cut.structure_id,
      style_id: cut.style_id,
      control_revision: cut.control_revision,
      rooms: [...(cut.rooms ?? [])].sort((a, b) =>
        BigInt(a.room_id) < BigInt(b.room_id) ? -1 : BigInt(a.room_id) > BigInt(b.room_id) ? 1 : 0,
      ),
      dependency_digest: binary(cut.dependency_digest).toString('base64'),
    },
  };
}
export const mergeInputDigest = (input) => hash(Buffer.from(canonical(mergeBinding(input))));

function fixedBody(content, target) {
  const bytes = binary(content.body_json_utf8);
  if (
    content.document_id !== target.document_id ||
    content.unit_id !== target.unit_id ||
    content.structure_id !== target.structure_id ||
    content.schema_version !== 'content-v1' ||
    content.canonicalization_version !== 'canonical-v1' ||
    content.hash_algorithm !== 'SHA-256' ||
    content.provenance_schema !== 'provenance-v1' ||
    bytes.length > 16 << 20 ||
    content.byte_size !== String(bytes.length) ||
    !hash(bytes).equals(binary(content.body_hash))
  )
    throw bad('MERGE_FIXED_CONTENT_INVALID');
  try {
    const body = normalizeContent(JSON.parse(bytes), { workspaceKind: 'MAIN' });
    const rawProvenance = binary(content.provenance_json_utf8),
      provenance = JSON.parse(rawProvenance);
    if (
      canonical(body) !== bytes.toString('utf8') ||
      !validateProvenance(provenance) ||
      canonical(provenance) !== rawProvenance.toString('utf8') ||
      canonical(resourceIds(body)) !== canonical(content.resource_ids ?? [])
    )
      throw Error('INVALID');
    return { body, provenance };
  } catch {
    throw bad('MERGE_FIXED_CONTENT_INVALID');
  }
}

export function validateMergeInput(input) {
  const context = input.context,
    target = context?.target,
    cut = input.main_cut;
  if (
    !validID(context?.operation_id) ||
    !positive(context?.actor_id) ||
    !positive(input.attempt_id) ||
    !target ||
    !targetKeys.every((key) => positive(target[key])) ||
    target.purpose !== 'MERGE' ||
    !context.identity_admission_ref ||
    !context.document_admission_ref ||
    binary(context.request_fingerprint).length !== 32 ||
    binary(input.input_digest).length !== 32 ||
    !Array.isArray(context.sources) ||
    !context.sources.length ||
    !cut ||
    cut.document_id !== target.document_id ||
    cut.structure_id !== target.structure_id ||
    !positive(cut.style_id) ||
    !positive(cut.control_revision) ||
    binary(cut.dependency_digest).length !== 32 ||
    !input.base ||
    !input.submitted ||
    !input.main
  )
    throw bad('MERGE_INPUT_INVALID');
  for (const room of cut.rooms ?? [])
    if (
      !positive(room.room_id) ||
      !positive(room.workspace_epoch) ||
      !(room.durable_seq === '0' || positive(room.durable_seq))
    )
      throw bad('MERGE_CUT_INVALID');
  if (new Set((cut.rooms ?? []).map((room) => room.room_id)).size !== (cut.rooms ?? []).length)
    throw bad('MERGE_CUT_INVALID');
  const content = Object.fromEntries(
    ['base', 'submitted', 'main'].map((key) => [key, fixedBody(input[key], target)]),
  );
  const binding = mergeBinding(input);
  if (!mergeInputDigest(input).equals(binary(input.input_digest)))
    throw bad('MERGE_INPUT_DIGEST_INVALID');
  return { binding, content };
}
````

UTF-8/LF全文SHA-256：`648d7c24ae47d7fbde5cb987c98f9ce4f3e202e599764905b67d5511bbdb11da`。[参考实现](../../../../services/collaboration/src/domain/merge_protocol.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
