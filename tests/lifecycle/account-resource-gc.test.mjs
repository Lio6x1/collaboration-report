import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime, pause } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { reviewDatabase, openReviewWorkspace, writeDraftBody } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { publishRelease, releaseInput } from '../helpers/publication.mjs';
import { uploadPNG, resourcePNG, binaryResourceGet } from '../helpers/resource.mjs';
import { lockOwnedResource } from '../helpers/owned-resource-lock.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

function imageBody(resource, text) {
  return {
    type: 'doc',
    content: [
      {
        type: 'paragraph',
        attrs: { id: 'gc-image-block' },
        content: [
          { type: 'text', text },
          {
            type: 'image',
            attrs: { resourceId: resource, alt: '真实回收图片', title: null, width: 1, height: 1 },
          },
        ],
      },
    ],
  };
}

// 真实上传/WSS引用→正式版保留→后来私有图片→注销释放真实引用。
// 仅推进自己原上传收据的GC测试时钟；真实Windows删除失败和Doc退出验证持久DELETING恢复。
test(
  'lifecycle resource GC preserves the actual public image and resumes deletion after a real Windows failure and process exit',
  { timeout: 180000 },
  async () => {
    const f = await organizationFixture(0),
      d = await documentFixture(f);
    const db = await reviewDatabase();
    let main,
      held,
      stopped = false;
    try {
      await unlockStructurePreview({ d, user: f.owner });
      main = await openReviewWorkspace(f, d, f.owner, 'MAIN');
      const fixture = { f, d, user: f.owner };
      const publicResource = await uploadPNG(fixture);
      await writeDraftBody(main, imageBody(publicResource, '固定公开图片'));
      const release = (await publishRelease(d.id, f.owner, await releaseInput(d.id, f.owner)))
        .result.entity_id;
      const privateResource = await uploadPNG(fixture);
      await writeDraftBody(main, imageBody(privateResource, '后来私有图片'));
      main.close();
      main = undefined;
      for (const path of [`/v1/knowledge-bases/${f.kb}`, `/v1/documents/${d.id}`]) {
        const current = json(await get(path, f.owner.headers));
        await waitAuthorization(
          await post(
            path + '/visibility',
            {
              operation_id: operation('gc-public'),
              expected_visibility_revision: current.visibility_revision,
              policy: 'PUBLIC',
            },
            f.owner.headers,
          ),
          f.owner,
        );
      }
      const me = json(await get('/v1/account/me', f.owner.headers));
      const input = {
        operation_id: operation('gc-close'),
        expected_identity_epoch: me.identity_epoch,
        current_password: f.owner.password,
      };
      const started = await post('/v1/account/closures', input, f.owner.headers);
      assert.equal(started.status, 202, started.body);
      let plan;
      const closeDeadline = Date.now() + 20000;
      while (Date.now() < closeDeadline) {
        const progress = await get(`/v1/account/closures/${input.operation_id}`, f.owner.headers);
        assert.equal(progress.status, 200, progress.body);
        plan = json(progress);
        if (plan.state === 'SUCCEEDED' || plan.state === 'REJECTED') break;
        await pause(100);
      }
      assert.equal(plan?.state, 'SUCCEEDED', `GC_ACCOUNT_CLOSE_NOT_TERMINAL ${plan?.state}`);
      assert.deepEqual(plan.retained_release_ids, [release]);
      const [resources] = await db.query(
        'SELECT CAST(id AS CHAR) id,storage_key,state,CAST(uploader_id AS CHAR) uploader,CAST(space_id AS CHAR) space_id FROM resource WHERE id IN (?,?)',
        [publicResource, privateResource],
      );
      assert.equal(resources.length, 2);
      const retained = resources.find((r) => r.id === publicResource),
        removed = resources.find((r) => r.id === privateResource);
      for (const row of resources) {
        assert.equal(row.state, 'READY');
        assert.equal(row.uploader, f.owner.id);
        assert.equal(row.space_id, f.space);
        assert.match(row.storage_key, /^[a-f0-9]{64}$/);
        assert.equal(existsSync(resolve(runtime, 'resources/document', row.storage_key)), true);
      }
      assert.notEqual(retained.storage_key, removed.storage_key);
      const [[pins]] = await db.query(
        `SELECT COUNT(*) n FROM resource_reference WHERE resource_id=? AND owner_kind='RETENTION'`,
        [publicResource],
      );
      assert.equal(Number(pins.n), 1);
      const [[privatePins]] = await db.query(
        `SELECT COUNT(*) n FROM resource_reference WHERE resource_id=? AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD')`,
        [privateResource],
      );
      assert.equal(Number(privatePins.n), 0, 'production lifecycle must release all private pins');
      held = await lockOwnedResource(removed.storage_key);
      const [expired] = await db.query(
        `UPDATE quota_reservation q JOIN resource r ON r.upload_operation_id=q.operation_id SET q.expires_at=UTC_TIMESTAMP(6)-INTERVAL 2 HOUR
       WHERE r.id IN (?,?) AND r.uploader_id=? AND r.space_id=? AND q.state='COMMITTED'`,
        [publicResource, privateResource, f.owner.id, f.space],
      );
      assert.equal(expired.affectedRows, 2);
      const deletingDeadline = Date.now() + 10000;
      let deleting;
      while (Date.now() < deletingDeadline) {
        held.assertHeld();
        const [[row]] = await db.query('SELECT state FROM resource WHERE id=?', [privateResource]);
        if (row.state === 'DELETING') {
          deleting = row;
          break;
        }
        await pause(100);
      }
      assert.equal(
        deleting?.state,
        'DELETING',
        'actual failed deletion must persist original deleting intent',
      );
      assert.equal(existsSync(resolve(runtime, 'resources/document', removed.storage_key)), true);
      const [[beforeQuota]] = await db.query(
        'SELECT CAST(used_bytes AS CHAR) used,CAST(reserved_bytes AS CHAR) reserved FROM quota_account WHERE space_id=?',
        [f.space],
      );
      assert.equal(
        beforeQuota.used,
        String(2 * resourcePNG.length),
        'unknown physical deletion must not release used quota',
      );
      stopOwnedRole('document');
      stopped = true;
      await held.release();
      held = undefined;
      console.log(
        'RESOURCE_GC_ACTUAL_EXIT: production private pins removed; real Windows delete refusal; durable DELETING and unchanged used quota; original Doc owner exited',
      );
      restartOwnedRuntime();
      stopped = false;
      const deletedDeadline = Date.now() + 12000;
      let deleted;
      while (Date.now() < deletedDeadline) {
        const [[row]] = await db.query('SELECT state FROM resource WHERE id=?', [privateResource]);
        if (row.state === 'DELETED') {
          deleted = row;
          break;
        }
        await pause(100);
      }
      assert.equal(
        deleted?.state,
        'DELETED',
        'original physical deletion must recover after restart',
      );
      assert.equal(existsSync(resolve(runtime, 'resources/document', removed.storage_key)), false);
      const [[remaining]] = await db.query('SELECT state FROM resource WHERE id=?', [
        publicResource,
      ]);
      assert.equal(remaining.state, 'READY');
      assert.equal(existsSync(resolve(runtime, 'resources/document', retained.storage_key)), true);
      const [[quota]] = await db.query(
        'SELECT CAST(used_bytes AS CHAR) used,CAST(reserved_bytes AS CHAR) reserved FROM quota_account WHERE space_id=?',
        [f.space],
      );
      assert.equal(quota.used, String(resourcePNG.length));
      assert.equal(quota.reserved, '0');
      const publicImage = await binaryResourceGet(
        `/v1/resources/${publicResource}/content?` +
          new URLSearchParams({ document_id: d.id, context_kind: 'PUBLIC', context_id: release }),
      );
      assert.equal(publicImage.status, 200, publicImage.body.toString('utf8'));
      assert.deepEqual(publicImage.body, resourcePNG);
      const [[reservation]] = await db.query(
        'SELECT q.state FROM quota_reservation q JOIN resource r ON r.upload_operation_id=q.operation_id WHERE r.id=?',
        [privateResource],
      );
      assert.equal(reservation.state, 'COMMITTED', 'original upload recovery receipt is preserved');
      console.log(
        'RESOURCE_GC_ACTUAL_PASS: real uploads/WSS and lifecycle release; original physical deletion recovered; retained PUBLIC image bytes survive; quota released exactly for private object; original upload receipt preserved',
      );
    } finally {
      main?.close();
      await held?.release();
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
