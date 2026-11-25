import { canonical } from '../../../../packages/content/index.mjs';
import { validID, bad } from './shared_protocol.mjs';
import { hash } from './shared_protocol.mjs';

const positive = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;
export const cutDigest = (rooms) => hash(Buffer.from(canonical(rooms))).toString('base64');
export function storedCut(cut) {
  return {
    document_id: cut.document_id,
    structure_id: cut.structure_id,
    style_id: cut.style_id,
    control_revision: cut.control_revision,
    rooms: [...cut.rooms].sort((a, b) =>
      BigInt(a.room_id) < BigInt(b.room_id) ? -1 : BigInt(a.room_id) > BigInt(b.room_id) ? 1 : 0,
    ),
    dependency_digest: Buffer.from(cut.dependency_digest ?? []).toString('base64'),
  };
}
export const wireCut = (cut) => ({
  ...cut,
  dependency_digest: Buffer.from(cut.dependency_digest, 'base64'),
});

export function prepareBinding(input) {
  const context = input.context,
    target = context?.target,
    cut = input.expected_cut;
  if (
    !validID(context?.operation_id) ||
    !positive(context?.actor_id) ||
    !positive(input.barrier_generation) ||
    !target ||
    !positive(target.document_id) ||
    !positive(target.structure_id) ||
    typeof target.purpose !== 'string' ||
    !/^[A-Z_]{1,32}$/.test(target.purpose) ||
    !context.document_admission_ref ||
    !context.identity_admission_ref ||
    Buffer.from(context.request_fingerprint ?? []).length !== 32 ||
    !Array.isArray(context.sources) ||
    !context.sources.length ||
    !cut ||
    cut.document_id !== target.document_id ||
    cut.structure_id !== target.structure_id ||
    !positive(cut.style_id) ||
    !positive(cut.control_revision) ||
    !Array.isArray(cut.rooms) ||
    cut.rooms.length > 10000 ||
    Buffer.from(cut.dependency_digest ?? []).length !== 32
  )
    throw bad('MAIN_PREPARE_INVALID');
  for (const room of cut.rooms)
    if (
      !positive(room.room_id) ||
      !positive(room.workspace_epoch) ||
      !(room.durable_seq === '0' || positive(room.durable_seq))
    )
      throw bad('MAIN_CUT_INVALID');
  if (new Set(cut.rooms.map((room) => room.room_id)).size !== cut.rooms.length)
    throw bad('MAIN_CUT_INVALID');
  return {
    context: {
      ...context,
      request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
    },
    barrier_generation: input.barrier_generation,
    expected_cut: storedCut(cut),
  };
}
