import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { waitOperation } from '../helpers/account.mjs';
import { reviewDatabase } from '../helpers/review.mjs';

test(
  '负责人离组遇到已关闭知识库时终结通知任务，保留系统回执且不产生通知',
  { timeout: 60000 },
  async () => {
    const f = await organizationFixture(1);
    const document = await documentFixture(f);
    const [departing] = f.members;
    let db;
    try {
      const current = await get(`/v1/documents/${document.id}`, f.owner.headers);
      assert.equal(current.status, 200, current.body);
      await waitAuthorization(
        await post(
          `/v1/documents/${document.id}/responsible-transfers`,
          {
            operation_id: operation('closed-context-responsible'),
            expected_revision: json(current).control_revision,
            target_user_id: departing.id,
          },
          f.owner.headers,
        ),
        f.owner,
      );

      // 通过真实关闭流程完成门禁排空；不直接改状态，也不伪造后台任务。
      const kb = await get(`/v1/knowledge-bases/${f.kb}`, f.owner.headers);
      assert.equal(kb.status, 200, kb.body);
      await waitAuthorization(
        await post(
          `/v1/knowledge-bases/${f.kb}`,
          {
            operation_id: operation('closed-context-kb'),
            expected_revision: json(kb).acl_revision,
          },
          f.owner.headers,
          'DELETE',
        ),
        f.owner,
      );
      assert.equal((await get(`/v1/knowledge-bases/${f.kb}`, f.owner.headers)).status, 404);

      const membership = await get(
        `/v1/organizations/${f.organization}/members?limit=100`,
        f.owner.headers,
      );
      assert.equal(membership.status, 200, membership.body);
      const member = json(membership).items.find((row) => row.user_id === departing.id);
      assert.ok(member);
      const departure = operation('closed-context-departure');
      await waitOperation(
        await post(
          `/v1/organizations/${f.organization}/members/${departing.id}`,
          { operation_id: departure, expected_revision: member.revision },
          f.owner.headers,
          'DELETE',
        ),
        f.owner,
      );

      // SQL仅观察持久结果。关闭上下文是确定的不可投递事实，应正常终结，
      // 不能被通用读取拒绝误判成依赖未知而永久RETRY。
      db = await reviewDatabase();
      const deadline = Date.now() + 12000;
      let task;
      for (;;) {
        const [rows] = await db.query(
          `SELECT t.state,t.attempts,t.last_error,t.operation_id AS event_id,
            JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.departed_user_id')) AS departed_user_id,
            JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.organization_id')) AS organization_id,
            JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.knowledge_base_id')) AS knowledge_base_id,
            o.actor_id,o.kind AS receipt_kind,o.state AS receipt_state
           FROM durable_task t JOIN lifecycle_operation o ON o.operation_id=t.operation_id
           WHERE t.kind='RESPONSIBLE_DEPARTURE_NOTIFICATION'
             AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.operation_id'))=?
             AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.document_id'))=?`,
          [departure, document.id],
        );
        assert.ok(rows.length <= 1, '同次离组的同一文档只能有一个持久通知任务');
        task = rows[0];
        if (task?.state === 'SUCCEEDED') break;
        assert.ok(
          Date.now() < deadline,
          'CLOSED_CONTEXT_DEPARTURE_TASK_NOT_FINISHED ' + JSON.stringify(task ?? null),
        );
        await new Promise((resolve) => setTimeout(resolve, 200));
      }
      assert.equal(task.actor_id, null, '离组清理是系统动作，不能把离组成员记为执行人');
      assert.equal(task.departed_user_id, departing.id, '历史事件仍须绑定实际离组成员');
      assert.equal(task.organization_id, f.organization);
      assert.equal(task.knowledge_base_id, f.kb);
      assert.equal(task.receipt_kind, 'OWNER_CHANGE');
      assert.equal(task.receipt_state, 'SUCCEEDED');
      const [[notifications]] = await db.query(
        'SELECT COUNT(*) AS count FROM notification WHERE event_id=?',
        [task.event_id],
      );
      assert.equal(Number(notifications.count), 0, '关闭知识库的历史通知不得实际投递');
    } finally {
      await db?.end();
    }
  },
);
