import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { unlockStructurePreview, structurePreviewInput } from '../helpers/structure-previews.mjs';

test(
  'an actual empty manifest supports lock, unlock, style change and a fresh chapter',
  { timeout: 180000 },
  async () => {
    const organization = await organizationFixture(0),
      d = await documentFixture(organization),
      f = { f: organization, d, user: organization.owner };
    const current = await unlockStructurePreview(f),
      input = structurePreviewInput(f, current);
    const preview = await post(`/v1/documents/${d.id}/structure-previews`, input, f.user.headers);
    assert.equal(preview.status, 200, json(preview).code);
    const removed = await completedMainControl(
      await post(
        `/v1/documents/${d.id}/structure-changes`,
        {
          ...input,
          operation_id: operation('empty-controls-remove'),
          changes: input.changes.map((v) => ({
            ...v,
            preview_cut_ref: json(preview).preview_cut_ref,
          })),
        },
        f.user.headers,
      ),
      f.user,
    );
    assert.equal(removed.state, 'SUCCEEDED', removed.error?.code);
    const lock = async (locked) => {
      const before = json(await get(`/v1/documents/${d.id}`, f.user.headers));
      const result = await completedMainControl(
        await post(
          `/v1/documents/${d.id}/main-lock`,
          {
            operation_id: operation('empty-main-lock'),
            expected_control_revision: before.control_revision,
            locked,
          },
          f.user.headers,
        ),
        f.user,
      );
      assert.equal(result.state, 'SUCCEEDED', result.error?.code);
      assert.equal(result.result.main_sync_state, 'READY');
      const after = await get(`/v1/documents/${d.id}/main`, f.user.headers);
      assert.equal(after.status, 200, json(after).code);
      assert.equal(json(after).units.length, 0);
      assert.equal(json(after).document.main_locked, locked);
    };
    await lock(true);
    await lock(false);
    const empty = json(await get(`/v1/documents/${d.id}/main`, f.user.headers));
    const styled = await completedMainControl(
      await post(
        `/v1/documents/${d.id}/styles`,
        {
          operation_id: operation('empty-main-style'),
          expected_control_revision: empty.document.control_revision,
          expected_style_id: empty.style.id,
          style: { ...empty.style.style, body_font: 'FangSong' },
        },
        f.user.headers,
      ),
      f.user,
    );
    assert.equal(styled.state, 'SUCCEEDED', styled.error?.code);
    const after = json(await get(`/v1/documents/${d.id}/main`, f.user.headers));
    assert.equal(after.style.style.body_font, 'FangSong');
    assert.equal(after.units.length, 0);
    const created = await completedMainControl(
      await post(
        `/v1/documents/${d.id}/chapters`,
        {
          operation_id: operation('empty-controls-fresh'),
          expected_structure_id: after.structure.id,
          title: '删除后继续编写',
        },
        f.user.headers,
      ),
      f.user,
    );
    assert.equal(created.state, 'SUCCEEDED', created.error?.code);
    assert.equal(json(await get(`/v1/documents/${d.id}/main`, f.user.headers)).units.length, 1);
  },
);
