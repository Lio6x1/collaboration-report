import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraftBody, submitDraft, reviewDatabase } from '../helpers/review.mjs';
import { get, json, operation } from '../helpers/workspace.mjs';

// 本例隔离固定 VERSION 的资源引用读取。READY 元数据为数据库夹具，未证明 Task 8 的物理上传/下载。
test('actual fixed review content returns the resource references of its own immutable version', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase(),
    key = operation('resource-metadata');
  try {
    await db.execute(
      "INSERT INTO resource(space_id,uploader_id,storage_key,checksum,byte_size,media_type,state,upload_operation_id) VALUES(?,?,?,UNHEX(REPEAT('00',32)),1,'image/png','READY',?)",
      [f.f.space, f.user.id, key, key],
    );
    const [[resource]] = await db.query(
      'SELECT CAST(id AS CHAR) AS id FROM resource WHERE storage_key=?',
      [key],
    );
    await writeDraftBody(f, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'resource-paragraph' },
          content: [
            {
              type: 'image',
              attrs: {
                resourceId: resource.id,
                alt: '固定引用',
                title: null,
                width: 160,
                height: 100,
              },
            },
          ],
        },
      ],
    });
    const submitted = (await submitDraft(f)).value;
    assert.equal(submitted.state, 'SUCCEEDED');
    const fixed = submitted.result;
    const [references] = await db.query(
      "SELECT CAST(resource_id AS CHAR) AS id FROM resource_reference WHERE document_id=? AND owner_kind='VERSION' AND owner_ref=? AND purpose='CONTENT' ORDER BY resource_id",
      [f.d.id, fixed.content_version_id],
    );
    assert.deepEqual(
      references.map((x) => x.id),
      [resource.id],
    );
    const response = await get(
      `/v1/review-requests/${fixed.request_id}/rounds/${fixed.round_id}`,
      f.user.headers,
    );
    assert.equal(response.status, 200);
    assert.deepEqual(json(response).content.resource_ids, [resource.id]);
  } finally {
    f.close();
    await db.end();
  }
});
