import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';
import https from 'node:https';
import { get, post, json, operation, env, origin } from '../helpers/account.mjs';
import { documentFixture, waitAuthorization } from '../helpers/workspace.mjs';
import { reviewFixture, writeDraftBody, submitDraft, reviewDatabase } from '../helpers/review.mjs';
import { pollMerge } from '../helpers/merge.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';
import { request } from '../helpers/https.mjs';
import { runtime } from '../../scripts/runtime-lib.mjs';

// 下载必须保留原始 Buffer；UTF-8 解码会替换 PNG 非文本字节，掩盖传输损坏。
function binaryGet(path, headers = {}) {
  return new Promise((resolveResponse, reject) => {
    let deadline;
    let settled = false;
    const finish = (error, response) => {
      if (settled) return;
      settled = true;
      clearTimeout(deadline);
      if (error) reject(error);
      else resolveResponse(response);
    };
    const req = https.request(
      {
        hostname: 'localhost',
        port: env.ports.edge,
        path,
        method: 'GET',
        ca: readFileSync(resolve(runtime, 'pki/ca.pem')),
        headers,
      },
      (response) => {
        const chunks = [];
        response.on('data', (chunk) => chunks.push(chunk));
        response.once('error', (error) => finish(error));
        response.once('aborted', () => finish(Error('RESOURCE_BODY_INCOMPLETE')));
        response.once('close', () => {
          if (!response.complete) finish(Error('RESOURCE_BODY_INCOMPLETE'));
        });
        response.once('end', () => {
          if (!response.complete) return finish(Error('RESOURCE_BODY_INCOMPLETE'));
          finish(null, {
            status: response.statusCode,
            headers: response.headers,
            body: Buffer.concat(chunks),
          });
        });
      },
    );
    deadline = setTimeout(() => req.destroy(Error('RESOURCE_DOWNLOAD_TIMEOUT')), 10000);
    req.once('error', (error) => finish(error));
    req.end();
  });
}

test(
  '真实上传图片在草稿、审核、主线与公开正式版间严格绑定并随撤下失效',
  { timeout: 90000 },
  async () => {
    const f = await reviewFixture();
    try {
      const png = Buffer.from(
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jC1kAAAAASUVORK5CYII=',
        'base64',
      );
      const checksum = createHash('sha256').update(png).digest('hex');
      const quota = await get(`/v1/spaces/${f.f.space}/quota`, f.user.headers);
      assert.equal(quota.status, 200);
      const prepared = await post(
        `/v1/documents/${f.d.id}/resources`,
        {
          operation_id: operation('content-upload'),
          expected_quota_revision: json(quota).revision,
          byte_size: String(png.length),
          media_type: 'image/png',
          checksum,
        },
        f.user.headers,
      );
      assert.equal(prepared.status, 201, prepared.body);
      const handle = json(prepared);
      const uploaded = await request(env.ports.edge, new URL(handle.upload_url).pathname, {
        method: 'PUT',
        headers: { origin, 'content-type': 'application/octet-stream', ...f.user.headers },
        body: png,
        timeoutMs: 10000,
      });
      assert.equal(uploaded.status, 200, uploaded.body);
      const completed = await post(
        `/v1/resources/${handle.resource_id}/complete`,
        { operation_id: operation('content-complete'), byte_size: String(png.length), checksum },
        f.user.headers,
      );
      assert.equal(completed.status, 200, completed.body);
      assert.equal(json(completed).state, 'READY');

      const path = (kind, context, document = f.d.id) =>
        `/v1/resources/${handle.resource_id}/content?` +
        new URLSearchParams({ document_id: document, context_kind: kind, context_id: context });
      const branchPath = path('BRANCH', f.session.branch_id);
      // 上传所有者身份不是下载凭据：READY 但没有当前 DRAFT 引用仍必须拒绝。
      assert.equal((await binaryGet(branchPath, f.user.headers)).status, 403);
      const saved = await writeDraftBody(f, {
        type: 'doc',
        content: [
          {
            type: 'paragraph',
            attrs: { id: 'download-image-paragraph' },
            content: [
              {
                type: 'image',
                attrs: {
                  resourceId: handle.resource_id,
                  alt: '实际上传图片',
                  title: null,
                  width: 1,
                  height: 1,
                },
              },
            ],
          },
        ],
      });
      assert.equal(saved.type, 'SAVED');
      const downloaded = await binaryGet(branchPath, f.user.headers);
      assert.equal(downloaded.status, 200, downloaded.body.toString('utf8'));
      assert.equal(downloaded.headers['content-type'], 'image/png');
      assert.deepEqual(downloaded.body, png);
      assert.equal((await binaryGet(branchPath)).status, 401);

      // 同一用户拥有另一文档也不能替换引用文档；MAIN 不得借用 BRANCH 草稿引用。
      const other = await documentFixture(f.f);
      assert.equal(
        (await binaryGet(path('BRANCH', f.session.branch_id, other.id), f.user.headers)).status,
        404,
      );
      assert.match(f.d.main.manifest_id, /^[1-9][0-9]*$/);
      assert.equal(
        (await binaryGet(path('MAIN', f.d.main.manifest_id), f.user.headers)).status,
        403,
      );

      // 真实 FLUSH/送审生成不可变 VERSION 引用，再从明确的 round_id 下载同一字节。
      const submitted = (await submitDraft(f)).value;
      assert.equal(submitted.state, 'SUCCEEDED');
      const reviewed = await binaryGet(path('REVIEW', submitted.result.round_id), f.user.headers);
      assert.equal(reviewed.status, 200, reviewed.body.toString('utf8'));
      assert.deepEqual(reviewed.body, png);

      // 合并走真实审核命令；等待 MAIN 同步完成后再取得新的固定阅读清单。
      const mergeOperation = operation('content-merge');
      const merging = await post(
        `/v1/review-requests/${submitted.result.request_id}/approve-and-merge`,
        {
          operation_id: mergeOperation,
          round_id: submitted.result.round_id,
          content_version_id: submitted.result.content_version_id,
          own_changes_to_resolve: [],
        },
        f.user.headers,
      );
      assert.equal(merging.status, 202, merging.body);
      // 此连接仅供既有轮询器在超时时读取诊断，不创建或修改资源/引用夹具。
      const db = await reviewDatabase();
      try {
        const merged = await pollMerge(mergeOperation, f.user.headers, db);
        assert.equal(merged.state, 'SUCCEEDED', JSON.stringify(merged));
      } finally {
        await db.end();
      }
      const mainResponse = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
      assert.equal(mainResponse.status, 200, mainResponse.body);
      const main = json(mainResponse);
      assert.match(main.manifest_id, /^[1-9][0-9]*$/);
      const mainImage = await binaryGet(path('MAIN', main.manifest_id), f.user.headers);
      assert.equal(mainImage.status, 200, mainImage.body.toString('utf8'));
      assert.deepEqual(mainImage.body, png);

      // 知识库与文档均通过真实可见性命令开放，匿名 PUBLIC 请求才有独立授权来源。
      for (const target of [`/v1/knowledge-bases/${f.f.kb}`, `/v1/documents/${f.d.id}`]) {
        const current = await get(target, f.user.headers);
        assert.equal(current.status, 200, current.body);
        const visibility = await waitAuthorization(
          await post(
            target + '/visibility',
            {
              operation_id: operation('content-public'),
              expected_visibility_revision: json(current).visibility_revision,
              policy: 'PUBLIC',
            },
            f.user.headers,
          ),
          f.user,
        );
        assert.equal(visibility.state, 'SUCCEEDED');
      }
      const release = await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
      const publicPath = path('PUBLIC', release.result.entity_id);
      const publicImage = await binaryGet(publicPath);
      assert.equal(publicImage.status, 200, publicImage.body.toString('utf8'));
      assert.deepEqual(publicImage.body, png);

      // 撤下完成后原 release_id 即使仍保留历史引用，也不能继续作为公开下载凭据。
      const current = await get(`/v1/documents/${f.d.id}`, f.user.headers);
      assert.equal(current.status, 200, current.body);
      const unpublishOperation = operation('content-unpublish');
      const withdrawn = await post(
        `/v1/documents/${f.d.id}/publication`,
        {
          operation_id: unpublishOperation,
          expected_publication_revision: json(current).publication_epoch,
          action: 'UNPUBLISH',
        },
        f.user.headers,
      );
      assert.equal(withdrawn.status, 202, withdrawn.body);
      const unpublished = await pollPublication(unpublishOperation, f.user);
      assert.equal(unpublished.state, 'SUCCEEDED', unpublished.error?.code);
      assert.equal((await binaryGet(publicPath)).status, 404);

      // 第二份图片仍通过真实上传完成，但不插入正文，用同一空间验证 GC 只释放孤儿。
      const beforeOrphan = await get(`/v1/spaces/${f.f.space}/quota`, f.user.headers);
      assert.equal(beforeOrphan.status, 200, beforeOrphan.body);
      const orphanPrepared = await post(
        `/v1/documents/${f.d.id}/resources`,
        {
          operation_id: operation('content-orphan-upload'),
          expected_quota_revision: json(beforeOrphan).revision,
          byte_size: String(png.length),
          media_type: 'image/png',
          checksum,
        },
        f.user.headers,
      );
      assert.equal(orphanPrepared.status, 201, orphanPrepared.body);
      const orphan = json(orphanPrepared);
      const orphanUploaded = await request(env.ports.edge, new URL(orphan.upload_url).pathname, {
        method: 'PUT',
        headers: { origin, 'content-type': 'application/octet-stream', ...f.user.headers },
        body: png,
        timeoutMs: 10000,
      });
      assert.equal(orphanUploaded.status, 200, orphanUploaded.body);
      const orphanCompleted = await post(
        `/v1/resources/${orphan.resource_id}/complete`,
        {
          operation_id: operation('content-orphan-complete'),
          byte_size: String(png.length),
          checksum,
        },
        f.user.headers,
      );
      assert.equal(orphanCompleted.status, 200, orphanCompleted.body);
      assert.equal(json(orphanCompleted).state, 'READY');

      const cleanupDB = await reviewDatabase();
      let canonicalBytes;
      try {
        // 只推进本例两个原收据的时钟，不伪造 READY、不删除真实历史/草稿引用。
        // 撤下不是删除历史；已合并图片即使超过插入宽限也必须保留。
        const [expired] = await cleanupDB.query(
          `UPDATE quota_reservation q JOIN resource r ON r.upload_operation_id=q.operation_id
           SET q.expires_at=UTC_TIMESTAMP(6)-INTERVAL 2 HOUR
           WHERE r.id IN (?,?) AND q.state='COMMITTED'`,
          [handle.resource_id, orphan.resource_id],
        );
        assert.equal(expired.affectedRows, 2);
        const deadline = Date.now() + 12000;
        for (;;) {
          const [rows] = await cleanupDB.query(
            `SELECT CAST(r.id AS CHAR) id,r.state, r.storage_key,q.state reservation_state
             FROM resource r JOIN quota_reservation q ON q.operation_id=r.upload_operation_id
             WHERE r.id IN (?,?)`,
            [handle.resource_id, orphan.resource_id],
          );
          const retained = rows.find((row) => row.id === handle.resource_id);
          const deleted = rows.find((row) => row.id === orphan.resource_id);
          assert.ok(retained && deleted, 'RESOURCE_CLEANUP_ROWS_MISSING');
          assert.equal(retained.state, 'READY', 'REFERENCED_RESOURCE_MUST_SURVIVE_GC');
          assert.equal(retained.reservation_state, 'COMMITTED');
          assert.equal(deleted.reservation_state, 'COMMITTED');
          if (deleted.state === 'DELETED') {
            assert.match(deleted.storage_key, /^[a-f0-9]{64}$/);
            assert.equal(
              existsSync(resolve(runtime, 'resources/document', deleted.storage_key)),
              false,
            );
            break;
          }
          assert.ok(
            Date.now() < deadline,
            `READY_ORPHAN_NOT_COLLECTED ${JSON.stringify(rows.map(({ id, state, reservation_state }) => ({ id, state, reservation_state })))}`,
          );
          await new Promise((done) => setTimeout(done, 200));
        }
        // 010起空间used同时包含真实历史正文。GC仅退孤儿图片，不能退仍存活的
        // 正文/已合并图片；从实际canonical版本大小核账，不沿用旧的“仅图片”预期。
        const [[stored]] = await cleanupDB.query(
          `SELECT CAST(COALESCE(SUM(v.byte_size),0) AS CHAR) bytes
          FROM content_version v JOIN document d ON d.id=v.document_id WHERE d.space_id=?`,
          [f.f.space],
        );
        canonicalBytes = BigInt(stored.bytes);
      } finally {
        await cleanupDB.end();
      }
      const finalQuota = await get(`/v1/spaces/${f.f.space}/quota`, f.user.headers);
      assert.equal(finalQuota.status, 200, finalQuota.body);
      assert.equal(json(finalQuota).used_bytes, (canonicalBytes + BigInt(png.length)).toString());
      assert.equal(json(finalQuota).reserved_bytes, '0');
    } finally {
      f.close();
    }
  },
);
