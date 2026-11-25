import assert from 'node:assert/strict';
import test from 'node:test';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { publicSnapshotRPC } from '../helpers/public-snapshot.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';

test(
  '未发布隐藏快照使用同一只读一致事实，KB政策行被其他事务锁定也在原RPC预算内返回',
  { timeout: 60000 },
  async () => {
    const fixture = await organizationFixture(0);
    const document = await documentFixture(fixture);
    const blocker = await reviewDatabase();
    const peer = publicSnapshotRPC();
    let locked = false;
    try {
      // 只锁本测试新KB，不写政策、正文或水位。保持锁直到RPC结束，
      // 使Doc→KB的FOR UPDATE隐藏路径必然等待，而一致只读快照应立即取得已提交事实。
      await blocker.beginTransaction();
      locked = true;
      const [rows] = await blocker.query(
        'SELECT CAST(id AS CHAR) id FROM knowledge_base WHERE id=? FOR UPDATE',
        [fixture.kb],
      );
      assert.equal(rows.length, 1);
      assert.equal(rows[0].id, fixture.kb);

      // rpcCall保留原2.5秒deadline；不能放大预算、提前释放KB锁或把超时当成隐藏成功。
      // 使用Search真实mTLS身份，匿名HTTP或伪造角色不能替代索引者的权限边界。
      const snapshot = await rpcCall(peer, 'GetPublicSnapshot', {
        document_id: document.id,
        expected_release_id: '0',
        aggregate_seq: '0',
      });
      assert.equal(locked, true);
      assert.equal(snapshot.document_id, document.id);
      assert.equal(snapshot.visible, false);
      assert.equal(snapshot.aggregate_seq, '0');
      assert.equal(snapshot.publication_epoch, '1');
      assert.equal(snapshot.visibility_policy, 'COLLABORATORS');
      assert.equal(snapshot.visibility_revision, '1');
      assert.equal(snapshot.knowledge_base_visibility_revision, '1');
      assert.equal(snapshot.release_id, '0');
      assert.equal(snapshot.structure_id, '0');
      assert.equal(snapshot.title, '');
      assert.deepEqual(snapshot.units, []);
    } finally {
      try {
        if (locked) await blocker.rollback();
      } finally {
        peer.close();
        await blocker.end();
      }
    }
  },
);
