import assert from 'node:assert/strict';
import { get, json, operation } from './workspace.mjs';

export const discardPreviewPath = (f) =>
  `/v1/documents/${f.d.id}/units/${f.d.main.units[0].unit_id}/discard-previews`;

export const discardPreviewInput = (f, id = operation('discard-preview')) => ({
  operation_id: id,
  expected_branch_id: f.session.branch_id,
  expected_structure_id: f.d.main.structure.id,
});

export const discardDraftPath = (preview) =>
  `/v1/documents/${preview.document_id}/units/${preview.unit_id}/discard-draft`;
export const discardDraftInput = (preview, id = operation('discard-confirm')) => ({
  operation_id: id,
  expected_branch_id: preview.branch_id,
  expected_structure_id: preview.structure_id,
  preview_cut_ref: preview.preview_cut_ref,
  changes_to_resolve: preview.blocking_changes.map((change) => ({
    id: change.id,
    revision: change.revision,
    reason: '明确保留旧稿并结束此项修改要求',
  })),
});

export async function pollDiscardDraft(id, headers, timeout = 20000) {
  const deadline = Date.now() + timeout;
  do {
    const response = await get(`/v1/operations/${id}`, headers);
    assert.equal(response.status, 200, response.body);
    const value = json(response);
    if (!['PREPARING', 'WAITING'].includes(value.state)) return value;
    await new Promise((resolve) => setTimeout(resolve, 100));
  } while (Date.now() < deadline);
  assert.fail('DISCARD_NOT_INSTALLED');
}

export function discardPreviewWire(fixed) {
  const context = fixed.Context;
  return {
    context: {
      ...context,
      actor_id: String(context.actor_id),
      sources: context.sources.map((source) => ({ ...source, epoch: String(source.epoch) })),
      target: Object.fromEntries(
        Object.entries(context.target).map(([key, value]) => [key, String(value)]),
      ),
    },
    expected_branch_id: String(fixed.Scope.Workspace.Workspace),
    expected_lineage_id: String(fixed.Scope.Workspace.Lineage),
    expected_structure_id: String(fixed.Scope.Structure),
  };
}

export async function pollDiscardPreview(id, headers) {
  const deadline = Date.now() + 20000;
  do {
    const response = await get(`/v1/operations/${id}`, headers);
    assert.equal(response.status, 200, response.body);
    const value = json(response);
    if (value.state !== 'PREPARING') return value;
    await new Promise((resolve) => setTimeout(resolve, 100));
  } while (Date.now() < deadline);
  assert.fail('DISCARD_PREVIEW_NOT_TERMINAL');
}
