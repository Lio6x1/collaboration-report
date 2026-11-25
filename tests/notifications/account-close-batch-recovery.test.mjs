import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { waitOperation } from '../helpers/account.mjs';
import { reviewDatabase, openReviewWorkspace, writeDraft } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { publishRelease, releaseInput } from '../helpers/publication.mjs';
import { until } from '../helpers/authorization-fence.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

// 一个实际注销场景集中验证存续组织、精确KB范围、100人批次和真实Doc退出恢复。
// 100个目录候选由自己的SQL夹具创建，权限与资格查询真实执行；不冒称101次注册或浏览器登录。
test(
  'account closure preserves surviving content and resumes the original 101-recipient notification',
  { timeout: 180000 },
  async () => {
    const f = await organizationFixture(2);
    const [successor, observer] = f.members;
    const d = await documentFixture(f);
    const account = await reviewDatabase('account');
    const doc = await reviewDatabase();
    let lock,
      main,
      stopped = false;
    try {
      const kb = json(await get(`/v1/knowledge-bases/${f.kb}`, f.owner.headers));
      await waitAuthorization(
        await post(
          `/v1/knowledge-bases/${f.kb}/roles`,
          {
            operation_id: operation('close-batch-kb-admin'),
            expected_acl_revision: kb.acl_revision,
            user_id: successor.id,
            role: 'ADMIN',
          },
          f.owner.headers,
        ),
        f.owner,
      );
      const org = json(await get(`/v1/organizations/${f.organization}`, f.owner.headers));
      await waitOperation(
        await post(
          `/v1/organizations/${f.organization}/owner-transfers`,
          {
            operation_id: operation('close-batch-owner'),
            expected_revision: org.revision,
            target_user_id: successor.id,
          },
          f.owner.headers,
        ),
        f.owner,
      );
      await unlockStructurePreview({ d, user: f.owner });
      main = await openReviewWorkspace(f, d, f.owner, 'MAIN');
      await writeDraft(main, '存续组织的固定正式正文');
      const release = (await publishRelease(d.id, f.owner, await releaseInput(d.id, f.owner)))
        .result.entity_id;
      await writeDraft(main, '负责人注销后仍须保留的私有MAIN');
      main.close();
      main = undefined;
      const before = json(await get(`/v1/documents/${d.id}`, successor.headers));
      const [[versions]] = await doc.query(
        'SELECT COUNT(*) n FROM content_version WHERE document_id=?',
        [d.id],
      );
      const candidates = [successor.id];
      // 不复制会话或生成假通知。真实私有库分别登记身份/成员及KB管理员关系。
      // 批量目录只服务这个组织，普通观察成员没有KB ADMIN，不进入固定收件集合。
      await account.beginTransaction();
      try {
        for (let n = 0; n < 100; n++) {
          const email = `${operation('notification-scale')}@example.test`;
          const [created] = await account.query(
            `INSERT INTO user(email,email_key,display_name,password_hash,password_algorithm,state,email_verified_at)
          SELECT ?,?, '通知批次候选',password_hash,password_algorithm,'ACTIVE',UTC_TIMESTAMP(6) FROM user WHERE id=? AND state='ACTIVE'`,
            [email, Buffer.from(email), f.owner.id],
          );
          assert.equal(created.affectedRows, 1);
          const id = String(created.insertId);
          candidates.push(id);
          await account.query(
            "INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,'MEMBER','ACTIVE')",
            [f.organization, id],
          );
        }
        await account.query(
          'UPDATE organization SET member_revision=member_revision+1 WHERE id=?',
          [f.organization],
        );
        await account.commit();
      } catch (error) {
        await account.rollback();
        throw error;
      }
      await doc.beginTransaction();
      try {
        for (const id of candidates.slice(1))
          await doc.query(
            "INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')",
            [f.kb, id],
          );
        await doc.query('UPDATE knowledge_base SET acl_revision=acl_revision+1 WHERE id=?', [f.kb]);
        await doc.commit();
      } catch (error) {
        await doc.rollback();
        throw error;
      }
      candidates.sort((a, b) => (BigInt(a) < BigInt(b) ? -1 : 1));
      lock = await reviewDatabase('account');
      await lock.beginTransaction();
      await lock.query('SELECT id FROM user WHERE id=? FOR UPDATE', [candidates.at(-1)]);
      const me = json(await get('/v1/account/me', f.owner.headers));
      const input = {
        operation_id: operation('close-batch'),
        expected_identity_epoch: me.identity_epoch,
        current_password: f.owner.password,
      };
      const started = await post('/v1/account/closures', input, f.owner.headers);
      assert.equal(started.status, 202, started.body);
      const event =
        'departure-' +
        createHash('sha256')
          .update(input.operation_id + ':' + d.id)
          .digest('hex');
      const readTask = async () => {
        const [[row]] = await doc.query(
          "SELECT CAST(id AS CHAR) id,state,attempts,payload FROM durable_task WHERE kind='RESPONSIBLE_DEPARTURE_NOTIFICATION' AND business_key=?",
          [event],
        );
        return row;
      };
      const partial = await until(
        async () => {
          const row = await readTask();
          if (
            !row ||
            String(row.payload.delivery_after) !== candidates[99] ||
            row.state !== 'RETRY'
          )
            return false;
          const [[count]] = await doc.query(
            'SELECT COUNT(*) n FROM notification WHERE event_id=?',
            [event],
          );
          return Number(count.n) === 100 && row;
        },
        'NOTIFICATION_FIRST_BATCH_NOT_DURABLE',
        12000,
      );
      assert.deepEqual(partial.payload.recipients.map(String), candidates);
      stopOwnedRole('document');
      stopped = true;
      await lock.rollback();
      await lock.end();
      lock = undefined;
      console.log(
        'NOTIFICATION_BATCH_ACTUAL_EXIT: real first 100 recipients and cursor committed; last Account candidate locked caused RETRY; original Doc owner exited',
      );
      restartOwnedRuntime();
      stopped = false;
      const terminal = await until(
        async () => {
          const row = await readTask();
          return row?.state === 'SUCCEEDED' && row;
        },
        'NOTIFICATION_ORIGINAL_BATCH_NOT_RECOVERED',
        12000,
      );
      assert.equal(terminal.id, partial.id);
      assert.deepEqual(terminal.payload.recipients.map(String), candidates);
      const [delivered] = await doc.query(
        'SELECT CAST(recipient_id AS CHAR) recipient,kind FROM notification WHERE event_id=? ORDER BY recipient_id',
        [event],
      );
      assert.deepEqual(
        delivered.map((row) => row.recipient),
        candidates,
      );
      assert.ok(delivered.every((row) => row.kind === 'RESPONSIBLE_DEPARTURE'));
      const final = await until(
        async () => {
          const response = await get(`/v1/account/closures/${input.operation_id}`, f.owner.headers);
          assert.equal(response.status, 200, response.body);
          const value = json(response);
          return value.state === 'SUCCEEDED' && value;
        },
        'SURVIVING_ORG_ACCOUNT_CLOSE_NOT_TERMINAL',
        20000,
      );
      assert.deepEqual(final.retained_release_ids, []);
      const after = json(await get(`/v1/documents/${d.id}`, successor.headers));
      assert.equal(after.lifecycle, 'ACTIVE');
      assert.equal(after.current_manifest_id, before.current_manifest_id);
      assert.equal(after.current_release_id, release);
      assert.equal(after.responsible_user_id, undefined);
      const [[kept]] = await doc.query(
        'SELECT COUNT(*) n FROM content_version WHERE document_id=?',
        [d.id],
      );
      assert.equal(Number(kept.n), Number(versions.n));
      const [[identity]] = await account.query(
        'SELECT state,email,password_hash FROM user WHERE id=?',
        [f.owner.id],
      );
      assert.equal(identity.state, 'CLOSED');
      assert.equal(identity.email, null);
      assert.equal(identity.password_hash, null);
      const currentOrg = json(await get(`/v1/organizations/${f.organization}`, successor.headers));
      assert.equal(currentOrg.lifecycle, 'ACTIVE');
      assert.equal(currentOrg.owner_id, successor.id);
      assert.equal((await get(`/v1/documents/${d.id}`, f.owner.headers)).status, 401);
      const inbox = json(await get('/v1/notifications?limit=100', successor.headers));
      assert.equal(inbox.items.filter((row) => row.event_id === event).length, 1);
      const other = json(await get('/v1/notifications?limit=100', observer.headers));
      assert.equal(
        other.items.some((row) => row.event_id === event),
        false,
      );
      const replay = await post('/v1/account/closures', input, f.owner.headers);
      assert.equal(replay.status, 202, replay.body);
      const [[unique]] = await doc.query('SELECT COUNT(*) n FROM notification WHERE event_id=?', [
        event,
      ]);
      assert.equal(Number(unique.n), 101);
      console.log(
        'NOTIFICATION_BATCH_ACTUAL_PASS: original 101-recipient task recovered without truncation/duplication; actual account closed; surviving MAIN/versions/release and organization remain; exact KB scope',
      );
    } finally {
      main?.close();
      if (lock) {
        await lock.rollback();
        await lock.end();
      }
      await Promise.all([account.end(), doc.end()]);
      if (stopped) restartOwnedRuntime();
    }
  },
);
