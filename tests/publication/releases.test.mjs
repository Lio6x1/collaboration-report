import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import {
  reviewFixture,
  reviewDatabase,
  openReviewWorkspace,
  writeDraft,
} from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';

async function storedRelease(db, document, release) {
  const [rows] = await db.query(
    'SELECT CAST(r.id AS CHAR) release_id,r.title,CAST(r.manifest_id AS CHAR) manifest_id,CAST(e.unit_id AS CHAR) unit_id,CAST(e.content_version_id AS CHAR) version_id,v.body_json FROM `release` r JOIN manifest_entry e ON e.manifest_id=r.manifest_id JOIN content_version v ON v.id=e.content_version_id WHERE r.document_id=? AND r.id=? ORDER BY e.unit_id',
    [document, release],
  );
  assert.ok(rows.length);
  return rows;
}

// 正式版来自真实SAVED尾部，不从滞后业务manifest拿正文；S2和支线未合入内容都不能改写已发布S1。
test(
  'publication fixes reliable MAIN content and title without advancing MAIN, and original replay creates one event',
  { timeout: 90000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase();
    let main;
    try {
      await unlockStructurePreview(f);
      await writeDraft(f, '尚未合入的支线内容');
      main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      await writeDraft(main, '正式发布S1可靠尾部');
      const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      const input = await releaseInput(f.d.id, f.user);
      const first = await publishRelease(f.d.id, f.user, input);
      const fixed = await storedRelease(db, f.d.id, first.result.entity_id);
      assert.match(JSON.stringify(fixed[0].body_json), /正式发布S1可靠尾部/);
      assert.doesNotMatch(JSON.stringify(fixed), /尚未合入的支线内容/);
      assert.equal(fixed[0].title, before.title);
      await writeDraft(main, '后来主线S2可靠尾部');
      assert.deepEqual(await storedRelease(db, f.d.id, first.result.entity_id), fixed);
      const after = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      for (const field of [
        'current_manifest_id',
        'current_structure_id',
        'current_style_id',
        'control_revision',
        'main_locked',
      ])
        assert.deepEqual(after[field], before[field]);
      assert.equal(after.current_release_id, first.result.entity_id);
      assert.equal(BigInt(after.publication_epoch), BigInt(before.publication_epoch) + 1n);
      const replay = await post(`/v1/documents/${f.d.id}/releases`, input, f.user.headers);
      assert.equal(replay.status, 202, json(replay).code);
      assert.deepEqual(json(replay), first);
      const [[counts]] = await db.query(
        "SELECT (SELECT COUNT(*) FROM `release` WHERE operation_id=?) releases,(SELECT COUNT(*) FROM outbox WHERE aggregate_kind='DOCUMENT' AND aggregate_id=?) events",
        [input.operation_id, f.d.id],
      );
      assert.equal(Number(counts.releases), 1);
      assert.equal(Number(counts.events), 1);
      const changed = await post(
        `/v1/documents/${f.d.id}/releases`,
        {
          ...input,
          expected_control_revision: String(BigInt(input.expected_control_revision) + 1n),
        },
        f.user.headers,
      );
      assert.equal(changed.status, 409);
      assert.equal(json(changed).code, 'OPERATION_ID_CONFLICT');
    } finally {
      main?.close();
      f.close();
      await db.end();
    }
  },
);

// 切回和撤下只改当前正式指针；历史和MAIN均保留，跨文档正式版不可借用。
test(
  'publication switches retained release, rejects foreign or stale input and unpublishes with monotonic outbox',
  { timeout: 90000 },
  async () => {
    const f = await organizationFixture(0),
      d = await documentFixture(f),
      other = await documentFixture(f),
      db = await reviewDatabase();
    try {
      const first = await publishRelease(d.id, f.owner, await releaseInput(d.id, f.owner));
      const second = await publishRelease(d.id, f.owner, await releaseInput(d.id, f.owner));
      const foreign = await publishRelease(
        other.id,
        f.owner,
        await releaseInput(other.id, f.owner),
      );
      const before = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
      const path = `/v1/documents/${d.id}/publication`;
      const denied = await post(
        path,
        {
          operation_id: operation('foreign-release'),
          expected_publication_revision: before.publication_epoch,
          action: 'SWITCH',
          release_id: foreign.result.entity_id,
        },
        f.owner.headers,
      );
      assert.equal(denied.status, 404);
      const switchedId = operation('switch-release');
      const switched = await post(
        path,
        {
          operation_id: switchedId,
          expected_publication_revision: before.publication_epoch,
          action: 'SWITCH',
          release_id: first.result.entity_id,
        },
        f.owner.headers,
      );
      assert.equal(switched.status, 202, json(switched).code);
      assert.equal((await pollPublication(switchedId, f.owner)).state, 'SUCCEEDED');
      const current = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
      assert.equal(current.current_release_id, first.result.entity_id);
      assert.equal(current.current_manifest_id, before.current_manifest_id);
      const stale = await post(
        path,
        {
          operation_id: operation('stale-release'),
          expected_publication_revision: before.publication_epoch,
          action: 'UNPUBLISH',
        },
        f.owner.headers,
      );
      assert.equal(stale.status, 409);
      const unpublishId = operation('unpublish-release');
      const response = await post(
        path,
        {
          operation_id: unpublishId,
          expected_publication_revision: current.publication_epoch,
          action: 'UNPUBLISH',
        },
        f.owner.headers,
      );
      assert.equal(response.status, 202, json(response).code);
      assert.equal((await pollPublication(unpublishId, f.owner)).state, 'SUCCEEDED');
      const after = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
      assert.equal(after.publication_state, 'UNPUBLISHED');
      assert.ok(after.current_release_id == null);
      assert.equal(after.current_manifest_id, before.current_manifest_id);
      assert.ok(await storedRelease(db, d.id, second.result.entity_id));
      const [events] = await db.query(
        "SELECT CAST(aggregate_seq AS CHAR) seq,event_type,payload FROM outbox WHERE aggregate_kind='DOCUMENT' AND aggregate_id=? ORDER BY aggregate_seq",
        [d.id],
      );
      assert.deepEqual(
        events.map((event) => event.seq),
        ['1', '2', '3', '4'],
      );
      assert.equal(events.at(-1).event_type, 'DOCUMENT_UNPUBLISHED');
      const [[row]] = await db.query(
        'SELECT CAST(event_seq AS CHAR) seq FROM document WHERE id=?',
        [d.id],
      );
      assert.equal(row.seq, '4');
    } finally {
      await db.end();
    }
  },
);
