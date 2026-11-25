import { get, post, json, operation } from './workspace.mjs';
import { completedMainControl } from './main-controls.mjs';

export async function unlockStructurePreview(f) {
  const document = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
  if (!document.main_locked) return document;
  await completedMainControl(
    await post(
      `/v1/documents/${f.d.id}/main-lock`,
      {
        operation_id: operation('structure-preview-unlock'),
        expected_control_revision: document.control_revision,
        locked: false,
      },
      f.user.headers,
    ),
    f.user,
  );
  return json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
}

export function structurePreviewInput(f, document, type = 'REMOVE') {
  return {
    operation_id: operation('structure-preview-boundary'),
    expected_structure_id: document.current_structure_id,
    expected_control_revision: document.control_revision,
    changes: [{ type, chapter_id: f.d.main.structure.entries[0].chapter_id }],
  };
}

export function structurePreviewWire(fixed) {
  const context = fixed.Context;
  // Go持久元数据采用普通JSON整数，protojson/RPC边界使用十进制文本；摘要始终同一base64字节。
  return {
    context: {
      ...context,
      actor_id: String(context.actor_id),
      sources: context.sources.map((v) => ({ ...v, epoch: String(v.epoch) })),
      target: Object.fromEntries(
        Object.entries({ unit_id: 0, branch_id: 0, ...context.target }).map(([key, value]) => [
          key,
          key === 'purpose' ? value : String(value),
        ]),
      ),
    },
    expected_branch_id: '0',
    expected_lineage_id: '0',
    expected_structure_id: String(fixed.Structure),
  };
}
