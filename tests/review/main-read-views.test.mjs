import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraft,
  writeDraftBody,
  reviewDatabase,
  collaborationRPC,
} from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime, state } from '../../scripts/runtime-lib.mjs';
import { request } from '../helpers/https.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { uploadPNG } from '../helpers/resource.mjs';

async function readMain(f, user = f.user) {
  const response = await get(`/v1/documents/${f.d.id}/main`, user.headers);
  assert.equal(response.status, 200, json(response).code);
  return json(response);
}

async function readUnit(f, view, user = f.user, unit = view.units[0]) {
  const response = await get(
    `/v1/documents/${f.d.id}/read-views/${view.read_view_id}/units/${unit.unit_id}`,
    user.headers,
  );
  assert.equal(response.status, 200, json(response).code);
  const content = json(response);
  assert.equal(content.id, unit.content_version_id, '正文必须指向实际固定版本');
  return content;
}

// 如果MAIN读取仍只返回滞后清单，本例首个正文断言失败；如果读取推进业务指针，最后的元数据比较失败。
test('unlocked MAIN views include reliable S1, retain that immutable view after S2 and leave business pointers intact', async () => {
  const f = await reviewFixture();
  let main;
  try {
    await unlockStructurePreview(f);
    const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    await writeDraft(main, '阅读切点一：实际SAVED与FLUSH的S1');
    const first = await readMain(f);
    const original = await readUnit(f, first);
    assert.match(JSON.stringify(original.body_json), /实际SAVED与FLUSH的S1/);
    assert.notEqual(first.manifest_id, before.current_manifest_id, '只读快照独立于业务清单指针');
    await writeDraft(main, '阅读切点二：后来可靠保存的S2');
    assert.deepEqual(await readUnit(f, first), original, '旧视图不能替换成新尾部');
    const second = await readMain(f);
    assert.match(JSON.stringify((await readUnit(f, second)).body_json), /后来可靠保存的S2/);
    assert.notEqual(second.read_view_id, first.read_view_id);
    assert.notEqual(second.units[0].content_version_id, first.units[0].content_version_id);
    const after = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    for (const field of [
      'current_manifest_id',
      'current_structure_id',
      'current_style_id',
      'control_revision',
      'main_locked',
      'current_release_id',
      'publication_state',
    ]) {
      assert.deepEqual(after[field], before[field], `读取不能改变业务字段${field}`);
    }
  } finally {
    main?.close();
    f.close();
  }
});

async function originalRead(db, view) {
  const [[row]] = await db.query(
    "SELECT CAST(t.payload AS CHAR) payload FROM read_view v JOIN document_admission a ON a.admission_id=v.admission_id JOIN durable_task t ON t.operation_id=a.operation_id AND t.kind='MAIN_READ_CONTEXT' WHERE v.view_id=?",
    [view.read_view_id],
  );
  assert.ok(row, '本次输出必须有真实原只读上下文');
  const [[expiry]] = await db.query(
    'SELECT UNIX_TIMESTAMP(a.expires_at)*1000 admission_expiry FROM read_view v JOIN document_admission a ON a.admission_id=v.admission_id WHERE v.view_id=?',
    [view.read_view_id],
  );
  assert.ok(
    Date.parse(view.expires_at) <= Number(expiry.admission_expiry) + 1,
    '对外read_view期限不能晚于原准入实际期限',
  );
  assert.doesNotMatch(row.payload, /"Token"|"token"|"cookie"/, '持久只读上下文不能保存登录Token');
  return JSON.parse(row.payload);
}

function mainReadWire(fixed) {
  const context = fixed.context;
  return {
    context: {
      ...context,
      actor_id: String(context.actor_id),
      sources: context.sources.map((s) => ({ ...s, epoch: String(s.epoch) })),
      target: Object.fromEntries(
        Object.entries({ unit_id: 0, branch_id: 0, ...context.target }).map(([k, v]) => [
          k,
          k === 'purpose' ? v : String(v),
        ]),
      ),
    },
    expected_manifest_id: String(fixed.manifest),
    expected_structure_id: String(fixed.structure),
    expected_style_id: String(fixed.style),
    expected_control_revision: String(fixed.control),
    usage: 'MAIN_READ_VIEW:1',
  };
}

const certificate = (role) => ({
  key: readFileSync(resolve(runtime, `pki/${role}-key.pem`)),
  cert: readFileSync(resolve(runtime, `pki/${role}.pem`)),
});
const mainSource = (wire, role = 'collaboration') =>
  request(state().ports.documentHttp, '/infra/main/context', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(wire),
    ...certificate(role),
  });
const capture = (peer, wire) =>
  rpcCall(peer, 'CaptureMainView', {
    ...wire,
    context: {
      ...wire.context,
      request_fingerprint: Buffer.from(wire.context.request_fingerprint, 'base64'),
    },
  });

test('readonly MAIN capture requires its actual mTLS caller, original binding and OPEN original admission', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase(),
    peer = collaborationRPC();
  let original;
  try {
    await unlockStructurePreview(f);
    const view = await readMain(f),
      fixed = await originalRead(db, view),
      wire = mainReadWire(fixed);
    assert.equal((await mainSource(wire, 'document')).status, 403);
    assert.equal((await mainSource(wire)).status, 200);
    assert.equal((await capture(peer, wire)).state, 'FIXED');
    for (const changed of [
      { ...wire, expected_manifest_id: '999999999' },
      { ...wire, context: { ...wire.context, actor_id: '999999999' } },
      { ...wire, usage: 'MAIN_READ_VIEW:2' },
      {
        ...wire,
        context: { ...wire.context, request_fingerprint: Buffer.alloc(32).toString('base64') },
      },
    ])
      assert.equal((await mainSource(changed)).status, 409, '不能改用另一份原输入');
    const admission = wire.context.document_admission_ref;
    const [[row]] = await db.query('SELECT state FROM document_admission WHERE admission_id=?', [
      admission,
    ]);
    original = { id: admission, state: row.state };
    // 单独注入本夹具准入关闭，证明已有固定捕获也不会重开原准入。
    await db.query("UPDATE document_admission SET state='CLOSED' WHERE admission_id=?", [
      admission,
    ]);
    assert.equal((await mainSource(wire)).status, 403);
    await assert.rejects(capture(peer, wire), (error) => [7, 14].includes(error.code));
  } finally {
    if (original)
      await db.query('UPDATE document_admission SET state=? WHERE admission_id=?', [
        original.state,
        original.id,
      ]);
    peer.close();
    f.close();
    await db.end();
  }
});

test('READ without MANAGE sees a complete reliable multi-unit MAIN view and loses it after actual grant revocation', async () => {
  const f = await reviewFixture(1),
    opened = [];
  try {
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/chapters`,
        {
          operation_id: operation('main-read-chapter'),
          expected_structure_id: current.current_structure_id,
          title: '第二个可靠阅读单元',
        },
        f.user.headers,
      ),
      f.user,
    );
    f.d.main = await readMain(f);
    assert.equal(f.d.main.units.length, 2);
    await unlockStructurePreview(f);
    for (const [index, unit] of f.d.main.units.entries()) {
      const d = { ...f.d, main: { ...f.d.main, units: [unit] } };
      const workspace = await openReviewWorkspace(f.f, d, f.user, 'MAIN');
      opened.push(workspace);
      await writeDraft(workspace, `全篇可靠单元${index + 1}`);
    }
    const reader = f.f.members[0],
      grant = {
        user_id: reader.id,
        scope_kind: 'DOCUMENT',
        scope_id: f.d.id,
        capability: 'COMMENT',
      };
    const change = async (action) => {
      const d = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      return waitAuthorization(
        await post(
          `/v1/documents/${f.d.id}/authorization-changes`,
          {
            operation_id: operation('main-read-grant'),
            expected_acl_revision: d.acl_revision,
            changes: [{ action, grant }],
          },
          f.user.headers,
        ),
        f.user,
      );
    };
    await change('GRANT');
    const view = await readMain(f, reader);
    assert.equal(view.units.length, 2);
    for (const [index, unit] of view.units.entries())
      assert.match(
        JSON.stringify((await readUnit(f, view, reader, unit)).body_json),
        new RegExp(`全篇可靠单元${index + 1}`),
      );
    const denied = await post(
      `/v1/documents/${f.d.id}/main-lock`,
      {
        operation_id: operation('read-is-not-manage'),
        expected_control_revision: view.document.control_revision,
        locked: true,
      },
      reader.headers,
    );
    assert.equal(denied.status, 403);
    await change('REVOKE');
    assert.equal((await get(`/v1/documents/${f.d.id}/main`, reader.headers)).status, 403);
    assert.equal(
      (
        await get(
          `/v1/documents/${f.d.id}/read-views/${view.read_view_id}/units/${view.units[0].unit_id}`,
          reader.headers,
        )
      ).status,
      403,
    );
  } finally {
    for (const workspace of opened) workspace.close();
    f.close();
  }
});

for (const controlKind of ['STYLE', 'CHAPTER']) {
  test(`a concurrent ${controlKind} control cannot publish a reading assembled from the earlier context`, async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase(),
      node = await reviewDatabase('collaboration');
    let held = false;
    let pending;
    try {
      const document = await unlockStructurePreview(f);
      const [[before]] = await db.query(
        'SELECT COUNT(*) views FROM read_view WHERE document_id=?',
        [f.d.id],
      );
      await node.beginTransaction();
      held = true;
      // 锁住本夹具文档的真实统一入口，使Node可靠切点暂缓；不改任务状态、房间seq或业务指针。
      const [gates] = await node.query(
        'SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE',
        ['main:' + f.d.id],
      );
      assert.equal(gates.length, 1);
      pending = get(`/v1/documents/${f.d.id}/main`, f.user.headers);
      const deadline = Date.now() + 1500;
      let registered = false;
      do {
        const [[row]] = await db.query(
          "SELECT COUNT(*) n FROM durable_task WHERE kind='MAIN_READ_CONTEXT' AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.context.target.document_id'))=?",
          [f.d.id],
        );
        if (Number(row.n) > 0) {
          registered = true;
          break;
        }
        await new Promise((resolve) => setTimeout(resolve, 1));
      } while (Date.now() < deadline);
      assert.ok(registered, '原读取上下文必须在RPC前真实登记');
      const suffix = controlKind === 'STYLE' ? 'styles' : 'chapters';
      const input =
        controlKind === 'STYLE'
          ? {
              operation_id: operation('concurrent-reading-style'),
              expected_style_id: document.current_style_id,
              expected_control_revision: document.control_revision,
              style: { ...f.d.main.style.style, body_font: 'FangSong' },
            }
          : {
              operation_id: operation('concurrent-reading-chapter'),
              expected_structure_id: document.current_structure_id,
              title: '并发新增的新主线单元',
            };
      const accepted = await post(`/v1/documents/${f.d.id}/${suffix}`, input, f.user.headers);
      assert.equal(accepted.status, 202, accepted.body);
      await node.commit();
      held = false;
      const changed = await pending;
      pending = null;
      assert.ok(
        [409, 503].includes(changed.status),
        '原上下文失效不能发布混合read_view: ' + changed.body,
      );
      const [[after]] = await db.query('SELECT COUNT(*) views FROM read_view WHERE document_id=?', [
        f.d.id,
      ]);
      assert.equal(after.views, before.views);
      const result = await completedMainControl(accepted, f.user);
      assert.equal(result.state, 'SUCCEEDED');
      const fresh = await readMain(f);
      if (controlKind === 'STYLE') {
        assert.equal(fresh.style.style.body_font, 'FangSong');
        assert.equal(fresh.style.id, fresh.document.current_style_id);
      } else {
        assert.equal(fresh.units.length, 2, '新读取必须是安装后的完整新单元集合');
        assert.equal(fresh.structure.id, fresh.document.current_structure_id);
      }
    } finally {
      if (held) await node.rollback();
      if (pending) await pending.catch(() => {});
      f.close();
      await node.end();
      await db.end();
    }
  });
}

test('a zero-room fixed MAIN view stays immutable after a later real MAIN room joins and saves', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase(),
    node = await reviewDatabase('collaboration');
  let main;
  let restore;
  try {
    const [[row]] = await db.query('SELECT main_locked FROM document WHERE id=?', [f.d.id]);
    restore = row.main_locked;
    const [[count]] = await node.query(
      "SELECT COUNT(*) rooms FROM room WHERE document_id=? AND workspace_kind='MAIN'",
      [f.d.id],
    );
    assert.equal(Number(count.rooms), 0);
    // 本夹具只设置MAIN输入布尔门槛以保留真实零房间初始状态；公开解锁会安装MAIN房间，不能冒充此边界。
    await db.query('UPDATE document SET main_locked=FALSE WHERE id=?', [f.d.id]);
    const first = await readMain(f),
      original = await readUnit(f, first);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    await writeDraft(main, '晚到MAIN房间的真实可靠正文');
    assert.deepEqual(await readUnit(f, first), original, '零房间固定版本不能改为晚到正文');
    const next = await readMain(f);
    assert.match(JSON.stringify((await readUnit(f, next)).body_json), /晚到MAIN房间的真实可靠正文/);
    assert.equal(next.document.current_manifest_id, first.document.current_manifest_id);
    assert.equal(next.document.control_revision, first.document.control_revision);
  } finally {
    main?.close();
    if (restore !== undefined)
      await db.query('UPDATE document SET main_locked=? WHERE id=?', [restore, f.d.id]);
    f.close();
    await node.end();
    await db.end();
  }
});

// READY仅作资源元数据夹具；物理上传与下载仍由后续资源批次独立验证。
test('reliable MAIN reading commits image references with its actual immutable captured version', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let main;
  try {
    await unlockStructurePreview(f);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    // 010之后 READY 必须带真实空间分配；不能以一条SQL假图绕过当前资格。
    const id = await uploadPNG(main);
    await writeDraftBody(main, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'reading-image-paragraph' },
          content: [
            {
              type: 'image',
              attrs: { resourceId: id, alt: '可靠阅读引用', title: null, width: 160, height: 100 },
            },
          ],
        },
      ],
    });
    const view = await readMain(f),
      content = await readUnit(f, view);
    assert.deepEqual(content.resource_ids, [id]);
    const [references] = await db.query(
      "SELECT CAST(resource_id AS CHAR) id FROM resource_reference WHERE document_id=? AND owner_kind='VERSION' AND owner_ref=? AND purpose='CONTENT'",
      [f.d.id, content.id],
    );
    assert.deepEqual(
      references.map((r) => r.id),
      [id],
    );
    assert.match(JSON.stringify(content.body_json), /可靠阅读引用/);
  } finally {
    main?.close();
    f.close();
    await db.end();
  }
});

test(
  'ordinary MAIN reading never renews its original expired admission or replays expired fixed content',
  { timeout: 60000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase(),
      peer = collaborationRPC();
    try {
      await unlockStructurePreview(f);
      const view = await readMain(f),
        fixed = await originalRead(db, view),
        wire = mainReadWire(fixed);
      assert.equal((await mainSource(wire)).status, 200);
      await new Promise((resolve) => setTimeout(resolve, 31000));
      assert.equal((await mainSource(wire)).status, 403);
      await assert.rejects(capture(peer, wire), (error) => [7, 14].includes(error.code));
      assert.equal(
        (
          await get(
            `/v1/documents/${f.d.id}/read-views/${view.read_view_id}/units/${view.units[0].unit_id}`,
            f.user.headers,
          )
        ).status,
        403,
      );
      const fresh = await readMain(f);
      assert.notEqual(fresh.read_view_id, view.read_view_id);
    } finally {
      peer.close();
      f.close();
      await db.end();
    }
  },
);
