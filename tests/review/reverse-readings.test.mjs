import test from 'node:test';
import assert from 'node:assert/strict';
import { mergedReverseFixture, pollReverse } from '../helpers/reverse-changes.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';

// 新审核可读性必须走真实公开接口；私有安装READY并不能证明Vue能读取该固定轮次的样式。
test('reverse new fixed round is readable through request detail, history and exact round with its frozen style', async () => {
  const { f, db, record, merged } = await mergedReverseFixture();
  try {
    const id = operation('reverse-readable-round');
    const registered = await post(
      `/v1/documents/${f.d.id}/reverse-changes`,
      {
        operation_id: id,
        expected_manifest_id: merged.result.manifest_id,
        merge_record_id: String(record.id),
        reason: '新固定审核必须可完整阅读',
      },
      f.user.headers,
    );
    assert.equal(registered.status, 202, registered.body);
    const result = await pollReverse(id, f.user.headers);
    assert.equal(result.state, 'SUCCEEDED', JSON.stringify(result));
    const fixed = result.result;
    const detail = await get(`/v1/review-requests/${fixed.request_id}`, f.user.headers);
    assert.equal(detail.status, 200, detail.body);
    const round = json(detail).current_round;
    assert.equal(round.id, fixed.round_id);
    assert.equal(round.content.id, fixed.content_version_id);
    assert.equal(round.style.id, f.d.main.style_id);
    for (const [path, pick] of [
      [`/v1/review-requests/${fixed.request_id}/rounds?limit=20`, (value) => value.items[0]],
      [`/v1/review-requests/${fixed.request_id}/rounds/${fixed.round_id}`, (value) => value],
    ]) {
      const read = await get(path, f.user.headers);
      assert.equal(read.status, 200, read.body);
      assert.deepEqual(pick(json(read)), round);
    }
    const [[context]] = await db.query(
      "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='REVIEW_ROUND_CONTEXT' AND business_key=? AND operation_id=?",
      [fixed.round_id, id],
    );
    assert.equal(JSON.parse(context.raw).style_id, f.d.main.style_id, '新生产者须输出规范字符串ID');
  } finally {
    f.close();
    await db.end();
  }
});
