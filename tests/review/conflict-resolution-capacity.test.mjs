import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { canonical } from '../../packages/content/index.mjs';
import { conflictedReviewFixture, conflictParagraph } from '../helpers/conflict-contexts.mjs';
import { resolutionInput, withdrawConflict } from '../helpers/conflict-resolutions.mjs';
import { get, post, json } from '../helpers/workspace.mjs';
import { pollPull } from '../helpers/pull-main.mjs';
import { openReviewWorkspace } from '../helpers/review.mjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import { fromPM } from '../../packages/content/index.mjs';

test('manual repair transfers and installs more than 4 MiB of valid canonical content without truncation', async () => {
  const { f, db, main, fixed, block, id } = await conflictedReviewFixture();
  let reopened;
  let stage = 'read conflict';
  let repairOperation;
  try {
    const context = json(await get(`/v1/conflict-contexts/${id}`, f.user.headers));
    stage = 'withdraw original request';
    await withdrawConflict(f, fixed);
    const input = resolutionInput(context, block);
    repairOperation = input.operation_id;
    // 每个文本节点在契约长度内；通过多块正文检验HTTP框架、私有传输和实际安装的共同边界。
    const chunk = '正文'.repeat(1 << 16);
    for (let n = 0; n < 11; n++)
      input.resolution_content.content.push(conflictParagraph(`large-repair-${n}`, chunk));
    const wanted = canonical(input.resolution_content);
    assert.ok(Buffer.byteLength(wanted) > 4 << 20);
    stage = 'register large resolution';
    const response = await post(`/v1/conflict-contexts/${id}/resolutions`, input, f.user.headers);
    assert.equal(response.status, 202, response.body);
    const [[registered]] = await db.query(
      "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='RESOLUTION_CONTEXT' AND business_key=?",
      [input.operation_id],
    );
    const originalDigest = createHash('sha256').update(registered.raw).digest('hex');
    stage = 'poll actual installation';
    const result = await pollPull(input.operation_id, f.user.headers, {
      timeout: 60000,
      retryUnavailable: true,
    });
    assert.equal(result.state, 'SUCCEEDED', JSON.stringify(result));
    const [[same]] = await db.query(
      "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='RESOLUTION_CONTEXT' AND business_key=?",
      [input.operation_id],
    );
    assert.equal(createHash('sha256').update(same.raw).digest('hex'), originalDigest);
    stage = 'compare immutable installed version';
    const [[version]] = await db.query(
      'SELECT canonical_body,CAST(byte_size AS CHAR) size FROM content_version WHERE id=?',
      [result.result.content_version_id],
    );
    assert.equal(version.canonical_body.toString(), wanted);
    assert.equal(version.size, String(Buffer.byteLength(wanted)));
    stage = 'reopen actual installed draft';
    reopened = await openReviewWorkspace(f.f, f.d, f.user);
    assert.equal(canonical(fromPM(yDocToProsemirrorJSON(reopened.doc))), wanted);
  } catch (error) {
    error.message += ` (stage=${stage}, operation=${repairOperation ?? 'not registered'})`;
    throw error;
  } finally {
    reopened?.close();
    main.close();
    f.close();
    await db.end();
  }
});
