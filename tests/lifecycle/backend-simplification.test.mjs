import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import { pause } from '../../scripts/runtime-lib.mjs';
import { get, post, json, operation, waitOperation } from '../helpers/account.mjs';
import { organizationFixture, documentFixture, waitAuthorization } from '../helpers/workspace.mjs';
import { reviewDatabase, openReviewWorkspace, writeDraft } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';

// 真实 SMTP 环境不向虚构邮箱发送验证码。只在测试库入口预置隔离身份，随后使用真实登录、
// HTTP/WSS 与后台恢复；不写 ACK、业务成功状态或清理回执，也不注销私密文件中的体验账号。
async function provisionIdentity(db, template) {
  const email = `simplification-${randomUUID()}@example.test`;
  const [made] = await db.execute(
    `INSERT INTO user(email,email_key,display_name,password_hash,password_algorithm,state,email_verified_at)
     SELECT ?,?,'后端整理验收',password_hash,password_algorithm,'ACTIVE',UTC_TIMESTAMP(6)
     FROM user WHERE email_key=? AND state='ACTIVE' AND password_hash IS NOT NULL`,
    [email, Buffer.from(email), Buffer.from(template.email.trim().toLowerCase())],
  );
  assert.equal(made.affectedRows, 1, 'TEST_IDENTITY_TEMPLATE_UNAVAILABLE');
  const response = await post('/v1/account/login', { email, password: template.password });
  assert.equal(response.status, 200, 'TEST_IDENTITY_LOGIN_FAILED');
  return {
    id: json(response).user_id,
    email,
    password: template.password,
    headers: {
      cookie: response.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
      'x-csrf-token': json(response).csrf_token,
    },
  };
}

async function personalFixture(user) {
  const response = await get('/v1/me/spaces?limit=5', user.headers);
  assert.equal(response.status, 200);
  const space = json(response).items.find((item) => item.kind === 'PERSONAL');
  assert.ok(space);
  const made = await post(
    `/v1/spaces/${space.id}/knowledge-bases`,
    { operation_id: operation('simplify-kb'), name: '整理验收个人库' },
    user.headers,
  );
  assert.equal(made.status, 201, made.body);
  return { owner: user, space: space.id, kb: json(made).result.entity_id };
}

test(
  'refactored receipts, previews, personal transfer and both retention paths preserve actual behavior',
  { timeout: 90000 },
  async () => {
    const template = JSON.parse(readFileSync(process.env.COLLABORATION_TEST_ACCOUNT, 'utf8'));
    const accountDB = await reviewDatabase('account');
    const docDB = await reviewDatabase();
    let editor;
    try {
      const owner = await provisionIdentity(accountDB, template);
      const receiver = await provisionIdentity(accountDB, template);
      const source = await personalFixture(owner);
      const target = await personalFixture(receiver);
      const transferred = await documentFixture(source);
      editor = await openReviewWorkspace(source, transferred, owner, 'MAIN');
      await writeDraft(editor, '转移后必须保持的可靠正文');
      editor.close();
      editor = undefined;
      const current = json(await get(`/v1/documents/${transferred.id}`, owner.headers));
      const offerInput = {
        operation_id: operation('simplify-offer'),
        expected_revision: current.control_revision,
        target_user_id: receiver.id,
      };
      const offered = await post(
        `/v1/documents/${transferred.id}/personal-transfers`,
        offerInput,
        owner.headers,
      );
      assert.equal(offered.status, 202, offered.body);
      assert.deepEqual(
        json(
          await post(
            `/v1/documents/${transferred.id}/personal-transfers`,
            offerInput,
            owner.headers,
          ),
        ),
        json(offered),
      );
      const offer = json(
        await get(`/v1/personal-transfers/${json(offered).result.entity_id}`, receiver.headers),
      );
      const acceptInput = {
        operation_id: operation('simplify-accept'),
        expected_revision: offer.revision,
        target_knowledge_base_id: target.kb,
        expected_quota_revision: json(
          await get(`/v1/spaces/${target.space}/quota`, receiver.headers),
        ).revision,
      };
      const accepted = await post(
        `/v1/personal-transfers/${offer.id}/accept`,
        acceptInput,
        receiver.headers,
      );
      assert.equal(accepted.status, 202, accepted.body);
      assert.equal((await pollPublication(acceptInput.operation_id, receiver)).state, 'SUCCEEDED');
      assert.equal((await get(`/v1/documents/${transferred.id}`, owner.headers)).status, 403);
      const transferredMain = await get(`/v1/documents/${transferred.id}/main`, receiver.headers);
      assert.equal(transferredMain.status, 200, transferredMain.body);
      const transferredVersion = await get(
        `/v1/content-versions/${json(transferredMain).units[0].content_version_id}`,
        receiver.headers,
      );
      assert.equal(transferredVersion.status, 200, transferredVersion.body);
      assert.match(transferredVersion.body, /转移后必须保持/);
      const [[quota]] = await docDB.query(
        'SELECT reserved_bytes FROM quota_account WHERE space_id=?',
        [target.space],
      );
      assert.equal(Number(quota.reserved_bytes), 0);

      const shared = await organizationFixture(0, owner);
      const document = await documentFixture(shared);
      await unlockStructurePreview({ d: document, user: owner });
      editor = await openReviewWorkspace(shared, document, owner);
      await writeDraft(editor, '不可变预览草稿');
      const discardPath = `/v1/documents/${document.id}/units/${document.main.units[0].unit_id}/discard-previews`;
      const discardInput = {
        operation_id: operation('simplify-discard-preview'),
        expected_branch_id: editor.session.branch_id,
        expected_structure_id: document.main.structure.id,
      };
      const preview = await post(discardPath, discardInput, owner.headers);
      assert.equal(preview.status, 200, preview.body);
      await writeDraft(editor, '后来保存不能覆盖旧预览');
      assert.deepEqual(json(await post(discardPath, discardInput, owner.headers)), json(preview));
      const metadata = json(await get(`/v1/documents/${document.id}`, owner.headers));
      const structurePreview = await post(
        `/v1/documents/${document.id}/structure-previews`,
        {
          operation_id: operation('simplify-structure-preview'),
          expected_structure_id: metadata.current_structure_id,
          expected_control_revision: metadata.control_revision,
          changes: [
            {
              type: 'RENAME',
              chapter_id: document.main.structure.entries[0].chapter_id,
              title: '只用于预览的名称',
            },
          ],
        },
        owner.headers,
      );
      assert.equal(structurePreview.status, 200, structurePreview.body);
      editor.close();
      editor = await openReviewWorkspace(shared, document, owner, 'MAIN');
      await writeDraft(editor, '关闭组织后保留的公开正文');
      const published = await publishRelease(
        document.id,
        owner,
        await releaseInput(document.id, owner),
      );
      editor.close();
      editor = undefined;
      for (const path of [`/v1/knowledge-bases/${shared.kb}`, `/v1/documents/${document.id}`]) {
        const before = json(await get(path, owner.headers));
        await waitAuthorization(
          await post(
            path + '/visibility',
            {
              operation_id: operation('simplify-public'),
              expected_visibility_revision: before.visibility_revision,
              policy: 'PUBLIC',
            },
            owner.headers,
          ),
          owner,
        );
      }
      const orgBefore = json(await get(`/v1/organizations/${shared.organization}`, owner.headers));
      const closed = await post(
        `/v1/organizations/${shared.organization}/closures`,
        {
          operation_id: operation('simplify-close-org'),
          expected_revision: orgBefore.revision,
        },
        owner.headers,
      );
      assert.equal(closed.status, 202, closed.body);
      await waitOperation(closed, owner);
      const retained = await get(`/v1/public/documents/${document.id}`);
      assert.equal(retained.status, 200, retained.body);
      assert.equal(json(retained).release_id, published.result.entity_id);
      assert.match(retained.body, /关闭组织后保留/);
      assert.equal((await get('/v1/account/me', owner.headers)).status, 200);

      // 另建个人私有文档，真实账号注销必须完成 Doc/Node 清理；原转移已完成的文档不在其范围。
      const privateDocument = await documentFixture(source);
      const me = json(await get('/v1/account/me', owner.headers));
      const closeInput = {
        operation_id: operation('simplify-close-account'),
        expected_identity_epoch: me.identity_epoch,
        current_password: owner.password,
      };
      const started = await post('/v1/account/closures', closeInput, owner.headers);
      assert.equal(started.status, 202, started.body);
      const recoveryCookie = started.headers['set-cookie']
        .find((value) => value.startsWith('__Host-collabRecovery='))
        .split(';')[0];
      const deadline = Date.now() + 20000;
      let final;
      while (Date.now() < deadline) {
        const response = await get(`/v1/account/closures/${closeInput.operation_id}`, {
          cookie: recoveryCookie,
        });
        assert.equal(response.status, 200, response.body);
        final = json(response);
        if (['SUCCEEDED', 'REJECTED'].includes(final.state)) break;
        await pause(100);
      }
      assert.equal(final?.state, 'SUCCEEDED');
      const [[purged]] = await docDB.query('SELECT lifecycle FROM document WHERE id=?', [
        privateDocument.id,
      ]);
      assert.equal(purged.lifecycle, 'PURGED');
      assert.equal((await get(`/v1/documents/${transferred.id}`, receiver.headers)).status, 200);
      assert.equal((await get(`/v1/public/documents/${document.id}`)).status, 200);
    } finally {
      editor?.close();
      await docDB.end();
      await accountDB.end();
    }
  },
);
