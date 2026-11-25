import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraft,
  reviewDatabase,
} from '../helpers/review.mjs';
import { get, json } from '../helpers/workspace.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

// 真正退出本owner的协作服务，不能把缺少可靠尾部的旧清单当作降级成功。
test(
  'unlocked MAIN refuses a stale fallback while its actual collaboration owner is unavailable',
  { timeout: 120000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase();
    let main,
      stopped = false;
    try {
      await unlockStructurePreview(f);
      main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      await writeDraft(main, '协作服务恢复后仍可读取的可靠正文');
      const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      const [[count]] = await db.query('SELECT COUNT(*) views FROM read_view WHERE document_id=?', [
        f.d.id,
      ]);
      stopOwnedRole('collaboration');
      stopped = true;
      const unavailable = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
      assert.equal(unavailable.status, 503, '无法固定可靠主线时必须明确不可用');
      const [[after]] = await db.query('SELECT COUNT(*) views FROM read_view WHERE document_id=?', [
        f.d.id,
      ]);
      assert.equal(after.views, count.views, '失败不能生成滞后read_view');
      const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      for (const field of ['current_manifest_id', 'control_revision', 'main_locked'])
        assert.equal(current[field], before[field]);
    } finally {
      main?.close();
      f.close();
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
    const fresh = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
    assert.equal(fresh.status, 200, fresh.body);
    const view = json(fresh);
    const unit = await get(
      `/v1/documents/${f.d.id}/read-views/${view.read_view_id}/units/${view.units[0].unit_id}`,
      f.user.headers,
    );
    assert.equal(unit.status, 200, unit.body);
    assert.match(JSON.stringify(json(unit).body_json), /恢复后仍可读取的可靠正文/);
  },
);
