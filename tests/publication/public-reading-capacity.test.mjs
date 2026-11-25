import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { canonical } from '../../packages/content/index.mjs';
import { reviewFixture } from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';
import { populatePublicUnit } from '../helpers/public-capacity.mjs';
import { publicSnapshotRPC } from '../helpers/public-snapshot.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import {
  searchDatabase,
  waitSearchReceipt,
  searchProjection,
  assertOriginalReceipt,
  assertKafkaCommitted,
} from '../helpers/search.mjs';

// 整篇公共DTO和内部RPC分别核验完整正文；原3秒HTTP与32MiB预算均保持，不能以单元读取代替。
test(
  'actual anonymous and Search delivery preserve a complete publication larger than 19MiB',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture();
    let peer, search;
    try {
      // 外部连接取得也在清理范围内；Search首次连接拒绝不能遗留已打开的WSS/Y.Doc。
      peer = publicSnapshotRPC();
      search = await searchDatabase();
      await unlockStructurePreview(f);
      const metadata = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      await completedMainControl(
        await post(
          `/v1/documents/${f.d.id}/chapters`,
          {
            operation_id: operation('public-capacity-chapter'),
            expected_structure_id: metadata.current_structure_id,
            title: '公共整篇的第二单元',
          },
          f.user.headers,
        ),
        f.user,
      );
      const viewResponse = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
      assert.equal(viewResponse.status, 200, json(viewResponse).code);
      const view = json(viewResponse),
        expected = [];
      assert.equal(view.units.length, 2);
      for (const unit of view.units) expected.push(await populatePublicUnit(f, view, unit));
      assert.ok(expected.reduce((sum, body) => sum + Buffer.byteLength(body), 0) > 19 << 20);
      const release = await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
      let visibility;
      for (const path of [`/v1/knowledge-bases/${f.f.kb}`, `/v1/documents/${f.d.id}`]) {
        const current = json(await get(path, f.user.headers));
        visibility = {
          operation_id: operation('public-capacity-visibility'),
          expected_visibility_revision: current.visibility_revision,
          policy: 'PUBLIC',
        };
        await waitAuthorization(
          await post(path + '/visibility', visibility, f.user.headers),
          f.user,
        );
      }
      const response = await get(`/v1/public/documents/${f.d.id}`);
      assert.equal(response.status, 200, json(response).code);
      assert.ok(Buffer.byteLength(response.body) > 19 << 20);
      assert.ok(Buffer.byteLength(response.body) <= 32 << 20);
      const value = json(response);
      assert.equal(value.release_id, release.result.entity_id);
      assert.equal(value.units.length, 2);
      const historical = await get(
        `/v1/documents/${f.d.id}/releases/${value.release_id}`,
        f.user.headers,
      );
      assert.equal(historical.status, 200, json(historical).code);
      assert.deepEqual(json(historical).units, value.units);
      const snapshot = await rpcCall(peer, 'GetPublicSnapshot', {
        document_id: f.d.id,
        expected_release_id: '0',
        aggregate_seq: '0',
      });
      assert.equal(snapshot.release_id, value.release_id);
      assert.equal(snapshot.units.length, 2);
      for (let index = 0; index < expected.length; index++) {
        assert.equal(canonical(value.units[index].body_json), expected[index]);
        assert.ok(
          snapshot.units[index].content.body_json_utf8.equals(Buffer.from(expected[index])),
        );
      }
      // 同一份实际WSS固定正文继续验证原Canal→Go快照客户端→ES全量提取，
      // 不用直接RPC成功代替消费者成功，也不另造较小正文或放大生产期限。
      const event =
        'visibility_index_' +
        createHash('sha256')
          .update(visibility.operation_id + '\n' + f.d.id)
          .digest('hex');
      const receipt = await waitSearchReceipt(search, event);
      assertOriginalReceipt(receipt, event, f.d.id);
      const projection = await searchProjection(f.d.id, receipt.physical);
      assert.equal(projection.release_id, value.release_id);
      assert.equal(projection.sequence, snapshot.aggregate_seq);
      assert.equal(projection.visibility_policy, 'PUBLIC');
      assert.equal(projection.chapters.length, 2);
      const plain = (node) =>
        node.type === 'text' ? node.text : (node.content ?? []).map(plain).join('') + '\n';
      const texts = expected.map((body) => plain(JSON.parse(body)).trim());
      for (let index = 0; index < texts.length; index++) {
        assert.equal(projection.chapters[index].unit_id, view.units[index].unit_id);
        assert.equal(projection.chapters[index].text, texts[index]);
      }
      assert.equal(projection.text, texts.map((text) => text + '\n').join(''));
      assert.ok(Buffer.byteLength(projection.text) > 19 << 20);
      await assertKafkaCommitted(receipt);
    } finally {
      peer?.close();
      f.close();
      await search?.end();
    }
  },
);
