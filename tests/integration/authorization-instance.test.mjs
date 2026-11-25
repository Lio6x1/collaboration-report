import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync, mkdirSync, unlinkSync } from 'node:fs';
import { resolve } from 'node:path';
import test from 'node:test';
import { WebSocket } from 'ws';
import { root, runtime, nodeArguments } from '../../scripts/runtime-lib.mjs';
import { ProcessOwner } from '../../services/collaboration/src/client/process_owner.mjs';
import { organizationFixture, documentFixture, operation } from '../helpers/workspace.mjs';
import { openReviewWorkspace, reviewDatabase } from '../helpers/review.mjs';

async function waitNaturalDeliveryDeadline(database, delivery) {
  const deadline = Date.now() + 3000;
  while (Date.now() < deadline) {
    const [[row]] = await database.query(
      'SELECT expires_at<=UTC_TIMESTAMP(6) expired FROM delivery_session WHERE delivery_id=?',
      [delivery],
    );
    if (row.expired) return;
    await new Promise((done) => setTimeout(done, 50));
  }
  assert.fail('FUTURE_DELIVERY_DEADLINE_DID_NOT_PASS');
}

for (const fault of ['OLD_PROCESS_ALIVE', 'PROCESS_PROBE_UNAVAILABLE']) {
  test(`product instance: ${fault} never supplies exit proof even after actual delivery lease expires`, async () => {
    const fixture = await organizationFixture(0),
      document = await documentFixture(fixture);
    const opened = await openReviewWorkspace(fixture, document),
      database = await reviewDatabase('collaboration');
    const owned = JSON.parse(readFileSync(resolve(runtime, 'processes.json'))).find(
      (item) => item.role === 'collaboration',
    );
    assert.ok(owned, 'ACTUAL_OWNED_COLLABORATION_REQUIRED');
    const current = owned.owner + '-collaboration',
      candidate = operation('replacement');
    let delivery, privateConfig;
    try {
      [[delivery]] = await database.query(
        "SELECT delivery_id,CAST(expires_at AS CHAR) original_expiry FROM delivery_session WHERE grant_id=? AND state='OPEN'",
        [opened.grant.grant_id],
      );
      assert.ok(delivery);
      // 本用例的实际 delivery 单独设置未来短租约并真实等待；真实旧 socket 的 binding 仍有效。
      // 不能把数据库租约到期当作实际进程退出或输出停止，原字段在 finally 恢复。
      await database.query(
        'UPDATE delivery_session SET expires_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE delivery_id=?',
        [delivery.delivery_id],
      );
      await waitNaturalDeliveryDeadline(database, delivery.delivery_id);
      assert.equal(opened.w.socket.readyState, WebSocket.OPEN);
      assert.throws(
        () => new ProcessOwner(root).exitProof(owned),
        /OLD_OUTPUT_PROCESS_STILL_RUNNING/,
      );
      let config = resolve(runtime, 'services/collaboration.json');
      if (fault === 'PROCESS_PROBE_UNAVAILABLE') {
        const cfg = JSON.parse(readFileSync(config));
        // 私密副本只存 .runtime。让实际 Windows 身份脚本不可用，不改生产配置或伪造 ABSENT。
        cfg.root = resolve(runtime, 'missing-owner-probe-fixture');
        assert.throws(
          () => new ProcessOwner(cfg.root).exitProof(owned),
          /OUTPUT_EXIT_PROOF_UNAVAILABLE/,
        );
        mkdirSync(resolve(runtime, 'faults'), { recursive: true });
        privateConfig = resolve(runtime, 'faults', candidate + '.json');
        writeFileSync(privateConfig, JSON.stringify(cfg));
        config = privateConfig;
      }
      const attempted = spawnSync(
        process.execPath,
        nodeArguments([
          resolve(root, 'services/collaboration/src/main.mjs'),
          '--config',
          config,
          '--run-owner',
          candidate,
        ]),
        {
          cwd: root,
          windowsHide: true,
          encoding: 'utf8',
          timeout: 15000,
          env: { ...process.env, COLLAB_RUN_OWNER: candidate },
        },
      );
      assert.equal(attempted.status, 1, 'replacement must fail closed, not become READY');
      assert.match(attempted.stderr, /"code":"SERVICE_START_FAILED"/);
      const [[old]] = await database.query(
        "SELECT payload FROM durable_task WHERE kind='NODE_INSTANCE' AND business_key=?",
        [current],
      );
      const facts = typeof old.payload === 'string' ? JSON.parse(old.payload) : old.payload;
      assert.equal(Boolean(facts.exit_proof), false, 'alive/unknown must not gain EXIT_CONFIRMED');
      const [[replacement]] = await database.query(
        "SELECT COUNT(*) n FROM durable_task WHERE kind='NODE_INSTANCE' AND business_key=?",
        [candidate + '-collaboration'],
      );
      assert.equal(
        Number(replacement.n),
        0,
        'failed probe must not register a replacement instance',
      );
      const [[lease]] = await database.query(
        'SELECT state,stop_reason FROM delivery_session WHERE delivery_id=?',
        [delivery.delivery_id],
      );
      assert.equal(lease.state, 'OPEN');
      assert.notEqual(lease.stop_reason, 'EXIT_CONFIRMED');
      assert.equal(opened.w.socket.readyState, WebSocket.OPEN);
    } finally {
      if (delivery)
        await database.query('UPDATE delivery_session SET expires_at=? WHERE delivery_id=?', [
          delivery.original_expiry,
          delivery.delivery_id,
        ]);
      await database.end();
      opened.close();
      if (privateConfig) unlinkSync(privateConfig);
    }
  });
}
