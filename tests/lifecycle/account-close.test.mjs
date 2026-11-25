import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { pause } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { reviewDatabase, openReviewWorkspace, writeDraft } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';
import { searchDatabase, waitSearchReceipt, searchProjection } from '../helpers/search.mjs';

// 这一场景从真实注册/会话/WSS发布发起完整注销，只POST一次后由后台推进；
// SQL只核对产品事实，不手工给清理回执、ACK或任务写成功。
test(
  'account closure persists and completes original public/private cleanup before deidentification',
  { timeout: 60000 },
  async () => {
    const f = await organizationFixture(0);
    const document = await documentFixture(f);
    const privateDocument = await documentFixture(f);
    const owner = f.owner;
    const docDB = await reviewDatabase();
    const accountDB = await reviewDatabase('account');
    const searchDB = await searchDatabase();
    let main;
    try {
      await unlockStructurePreview({ d: document, user: owner });
      main = await openReviewWorkspace(f, document, owner, 'MAIN');
      await writeDraft(main, '注销后保留的固定公开正文');
      const published = await publishRelease(
        document.id,
        owner,
        await releaseInput(document.id, owner),
      );
      const retainedRelease = published.result.entity_id;
      await writeDraft(main, '不允许留存的后来私有正文');
      main.close();
      main = undefined;
      for (const path of [`/v1/knowledge-bases/${f.kb}`, `/v1/documents/${document.id}`]) {
        const before = json(await get(path, owner.headers));
        await waitAuthorization(
          await post(
            path + '/visibility',
            {
              operation_id: operation('close-public'),
              expected_visibility_revision: before.visibility_revision,
              policy: 'PUBLIC',
            },
            owner.headers,
          ),
          owner,
        );
      }
      const beforePublic = json(await get(`/v1/public/documents/${document.id}`));
      assert.equal(beforePublic.release_id, retainedRelease);
      assert.match(JSON.stringify(beforePublic.units), /固定公开正文/);
      assert.doesNotMatch(JSON.stringify(beforePublic.units), /后来私有正文/);
      const me = json(await get('/v1/account/me', owner.headers));
      assert.match(me.identity_epoch, /^[1-9][0-9]*$/);
      // 同用户的另一个真实会话也不能查询原恢复进度。
      const sibling = await post('/v1/account/login', {
        email: owner.email,
        password: owner.password,
      });
      assert.equal(sibling.status, 200, sibling.body);
      const siblingHeaders = {
        cookie: sibling.headers['set-cookie'].map((c) => c.split(';')[0]).join('; '),
      };
      const input = {
        operation_id: operation('account-close'),
        expected_identity_epoch: me.identity_epoch,
        current_password: owner.password,
      };
      const bad = await post(
        '/v1/account/closures',
        { ...input, operation_id: operation('bad-close'), current_password: 'wrong-password-123' },
        owner.headers,
      );
      assert.equal(bad.status, 403, bad.body);
      assert.equal(json(bad).code, 'CURRENT_PASSWORD_INVALID');
      const noCSRF = await post('/v1/account/closures', input, { cookie: owner.headers.cookie });
      assert.equal(noCSRF.status, 403, noCSRF.body);
      assert.equal(json(noCSRF).code, 'CSRF_INVALID');
      const started = await post('/v1/account/closures', input, owner.headers);
      assert.equal(started.status, 202, started.body);
      assert.equal(json(started).state, 'PREPARING');
      const recoveryCookie = started.headers['set-cookie']
        .find((c) => c.startsWith('__Host-collabRecovery='))
        .split(';')[0];
      const progressPath = `/v1/account/closures/${input.operation_id}`;
      assert.equal((await get(progressPath)).status, 404);
      assert.equal((await get(progressPath, siblingHeaders)).status, 404);
      let final;
      const deadline = Date.now() + 20000;
      while (Date.now() < deadline) {
        const response = await get(progressPath, { cookie: recoveryCookie });
        assert.equal(response.status, 200, response.body);
        final = json(response);
        assert.equal(final.operation_id, input.operation_id);
        if (final.state === 'SUCCEEDED' || final.state === 'REJECTED') break;
        assert.ok(['PREPARING', 'FROZEN', 'PLANNED', 'CLEANING'].includes(final.state));
        await pause(100);
      }
      assert.equal(final?.state, 'SUCCEEDED', `ACCOUNT_CLOSE_NOT_TERMINAL ${final?.state}`);
      assert.deepEqual(final.required_successions, []);
      assert.deepEqual(final.retained_release_ids, [retainedRelease]);
      const [[account]] = await accountDB.query(
        `SELECT state,email,email_key,password_hash,password_algorithm,display_name,CAST(identity_epoch AS CHAR) identity_epoch,
      CAST(credential_epoch AS CHAR) credential_epoch FROM user WHERE id=?`,
        [owner.id],
      );
      assert.equal(account.state, 'CLOSED');
      for (const field of ['email', 'email_key', 'password_hash', 'password_algorithm'])
        assert.equal(account[field], null, field);
      assert.equal(account.display_name, '已注销用户');
      assert.equal(account.identity_epoch, String(BigInt(me.identity_epoch) + 1n));
      assert.equal(account.credential_epoch, String(BigInt(me.credential_epoch) + 1n));
      const [[organization]] = await accountDB.query(
        'SELECT lifecycle,name FROM organization WHERE id=?',
        [f.organization],
      );
      assert.equal(organization.lifecycle, 'CLOSED');
      assert.equal(organization.name, '已关闭组织');
      const [sessions] = await accountDB.query('SELECT state FROM session WHERE user_id=?', [
        owner.id,
      ]);
      assert.ok(sessions.length >= 2);
      assert.ok(sessions.every((s) => s.state === 'CLOSED'));
      const [[rootTask]] = await accountDB.query(
        "SELECT state,CAST(generation AS CHAR) generation,payload FROM durable_task WHERE kind='ACCOUNT_CLOSE_RECOVER' AND business_key=?",
        [input.operation_id],
      );
      assert.equal(rootTask.state, 'SUCCEEDED');
      assert.ok(BigInt(rootTask.generation) >= 6n);
      assert.doesNotMatch(JSON.stringify(rootTask.payload), /fixture-password/);
      const [[privateRow]] = await docDB.query(
        'SELECT lifecycle,current_manifest_id FROM document WHERE id=?',
        [privateDocument.id],
      );
      assert.equal(privateRow.lifecycle, 'PURGED');
      assert.equal(privateRow.current_manifest_id, null);
      const [[retained]] = await docDB.query(
        'SELECT lifecycle,CAST(event_seq AS CHAR) seq FROM document WHERE id=?',
        [document.id],
      );
      assert.equal(retained.lifecycle, 'PUBLIC_RETAINED');
      const afterPublic = await get(`/v1/public/documents/${document.id}`);
      assert.equal(afterPublic.status, 200, afterPublic.body);
      assert.deepEqual(json(afterPublic).units, beforePublic.units);
      assert.equal(json(afterPublic).release_id, retainedRelease);
      assert.equal((await get(`/v1/public/documents/${privateDocument.id}`)).status, 404);
      assert.equal((await get(`/v1/documents/${document.id}`, owner.headers)).status, 401);
      assert.equal((await get('/v1/account/me', owner.headers)).status, 401);
      assert.equal(
        (await post('/v1/account/login', { email: owner.email, password: owner.password })).status,
        401,
      );
      const replay = await post('/v1/account/closures', input, {
        ...owner.headers,
        cookie: recoveryCookie,
      });
      assert.equal(replay.status, 202, replay.body);
      assert.deepEqual(json(replay), final);
      assert.equal(
        (
          await post(
            '/v1/account/closures',
            { ...input, current_password: 'changed-password-123' },
            owner.headers,
          )
        ).status,
        409,
      );
      assert.equal((await get(progressPath, siblingHeaders)).status, 404);
      const [[replayed]] = await docDB.query(
        'SELECT CAST(event_seq AS CHAR) seq FROM document WHERE id=?',
        [document.id],
      );
      assert.equal(replayed.seq, retained.seq);
      const [[counts]] = await accountDB.query(
        "SELECT (SELECT COUNT(*) FROM audit_event WHERE operation_id=? AND kind='ACCOUNT_CLOSE_COMPLETE') audits,(SELECT COUNT(*) FROM durable_task WHERE kind='ACCOUNT_CLOSE_RECOVER' AND business_key=?) tasks",
        [input.operation_id, input.operation_id],
      );
      assert.equal(Number(counts.audits), 1);
      assert.equal(Number(counts.tasks), 1);
      const activation = createHash('sha256')
        .update('ACCOUNT_RETENTION_COMPLETE\n' + input.operation_id)
        .digest('hex');
      const event =
        'visibility_index_' +
        createHash('sha256').update(`retained-index-${activation}\n${document.id}`).digest('hex');
      const receipt = await waitSearchReceipt(searchDB, event, 5000);
      const projection = await searchProjection(document.id, receipt.physical);
      assert.equal(projection.visible, true);
      assert.equal(projection.sequence, retained.seq);
      assert.equal(projection.release_id, retainedRelease);
      console.log(
        'ACCOUNT_CLOSE_ACTUAL_PASS: real signup/login/WSS publication; one POST persists worker intent; original minimal recovery; fixed public/private cleanup; final credentials deidentified; blocked login/internal access; exact-session recovery and idempotent replay; real consumer projection',
      );
    } finally {
      main?.close();
      await Promise.all([docDB.end(), accountDB.end(), searchDB.end()]);
    }
  },
);
