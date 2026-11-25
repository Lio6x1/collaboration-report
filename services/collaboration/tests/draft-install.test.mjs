import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { canonical } from '../../../packages/content/index.mjs';
import { validateDraftInstall } from '../src/domain/draft_protocol.mjs';

const body = {
  type: 'doc',
  content: [
    {
      type: 'paragraph',
      attrs: { id: 'fixed-block' },
      content: [{ type: 'text', text: '固定草稿' }],
    },
  ],
};
const encoded = (value) => Buffer.from(canonical(value)).toString('base64');
function fixture() {
  const oldProvenance = {
    schema_version: 'provenance-v1',
    lineage_id: '11',
    base_manifest_id: '13',
    base_content_version_id: '12',
    source_operation_id: 'pull-original-operation',
    accepted_captures: [],
    block_sources: [],
  };
  const bytes = Buffer.from(canonical(body));
  const candidate = {
    document_id: '3',
    unit_id: '4',
    branch_id: '5',
    structure_id: '6',
    workspace_epoch: '1',
    operation_id: 'pull-original-operation',
    capture_id: 'draft-preview:pull-original-operation:candidate',
    lineage_ref: '11',
    body_json_utf8: encoded(body),
    body_hash: createHash('sha256').update(bytes).digest('base64'),
    byte_size: String(bytes.length),
    resource_ids: [],
    schema_version: 'content-v1',
    canonicalization_version: 'canonical-v1',
    hash_algorithm: 'SHA-256',
    provenance_schema: 'provenance-v1',
    provenance_json_utf8: encoded(oldProvenance),
  };
  const provenance = {
    ...oldProvenance,
    lineage_id: '21',
    base_manifest_id: '13',
    base_content_version_id: '22',
  };
  const content = {
    ...candidate,
    workspace_epoch: '2',
    lineage_ref: '21',
    cut_json_utf8: encoded({ room_id: '42', workspace_epoch: '2', durable_seq: '0' }),
    provenance_json_utf8: encoded(provenance),
  };
  return {
    fixed: {
      binding: {
        context: {
          target: {
            document_id: '3',
            unit_id: '4',
            branch_id: '5',
            structure_id: '6',
            workspace_epoch: '1',
          },
        },
      },
      source: { main_manifest_id: '13' },
      candidate,
      main: { body_json_utf8: encoded(body) },
      room_id: '9',
    },
    source: {
      old_room_id: '9',
      new_room_id: '42',
      new_epoch: '2',
      content,
      lineage: {
        lineage_id: '21',
        base_manifest_id: '13',
        base_version_id: '22',
        body,
        provenance,
      },
    },
  };
}

test('draft installation rejects substituted reliable content, baseline or accepted provenance even when the Go source is internally consistent', () => {
  const valid = fixture();
  assert.doesNotThrow(() => validateDraftInstall(valid.fixed, valid.source));
  for (const change of [
    (f) => {
      f.source.content.body_json_utf8 = encoded({ ...body, content: [] });
    },
    (f) => {
      f.source.lineage.body = { type: 'doc', content: [] };
    },
    (f) => {
      f.source.lineage.provenance.accepted_captures = [{ capture_id: 'invented' }];
      f.source.content.provenance_json_utf8 = encoded(f.source.lineage.provenance);
    },
    (f) => {
      f.source.new_epoch = '3';
      f.source.content.workspace_epoch = '3';
    },
    (f) => {
      f.source.content.cut_json_utf8 = encoded({
        room_id: '42',
        workspace_epoch: '2',
        durable_seq: '1',
      });
    },
  ]) {
    const f = fixture();
    change(f);
    assert.throws(
      () => validateDraftInstall(f.fixed, f.source),
      (error) => error.details === 'DRAFT_INSTALL_CONFLICT',
    );
  }
});
