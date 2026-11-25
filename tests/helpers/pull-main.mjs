import assert from 'node:assert/strict';
import { get, post, json, operation } from './workspace.mjs';
import { reviewDatabase } from './review.mjs';

export const pullPath = (f) =>
  `/v1/documents/${f.d.id}/units/${f.grant.context.target.unit_id}/pull-main`;
export const pullInput = (f, id = operation('pull-main')) => ({
  operation_id: id,
  expected_branch_id: f.session.branch_id,
  expected_lineage_id: f.session.lineage_id,
  expected_structure_id: f.grant.context.target.structure_id,
});
export async function pollPull(
  id,
  headers,
  { terminalOnly = false, timeout = 30000, retryUnavailable = false } = {},
) {
  const deadline = Date.now() + timeout;
  let value;
  do {
    let response;
    try {
      response = await get('/v1/operations/' + id, headers);
    } catch (error) {
      // 大正文的真实持久提交可能持有来源锁超过单次HTTP预算；丢响应不等于原操作失败。
      // 容量用例显式选择只重查这个原号，终态/实际版本/实际安装断言和总等待上限都保留。
      if (!retryUnavailable || error.message !== 'TEST_TIMEOUT') throw error;
      await new Promise((resolve) => setTimeout(resolve, 100));
      continue;
    }
    if (retryUnavailable && response.status === 503) {
      const unavailable = json(response);
      if (
        ['DEPENDENCY_TIMEOUT', 'SERVICE_UNAVAILABLE'].includes(unavailable.code) &&
        unavailable.retry_action === 'RETRY_SAME_OPERATION'
      ) {
        await new Promise((resolve) => setTimeout(resolve, 100));
        continue;
      }
    }
    assert.equal(response.status, 200, response.body);
    value = json(response);
    if (
      !['PREPARING', 'WAITING'].includes(value.state) ||
      (terminalOnly && value.state === 'WAITING')
    )
      return value;
    await new Promise((resolve) => setTimeout(resolve, 100));
  } while (Date.now() < deadline);
  const db = await reviewDatabase();
  try {
    const [tasks] = await db.query(
      "SELECT kind,state,last_error,attempts FROM durable_task WHERE operation_id=? AND state NOT IN ('SUCCEEDED','REJECTED')",
      [id],
    );
    assert.fail('PULL_NOT_TERMINAL ' + JSON.stringify({ value, tasks }));
  } finally {
    await db.end();
  }
}
export async function pullMain(f) {
  const input = pullInput(f),
    response = await post(pullPath(f), input, f.user.headers);
  assert.equal(response.status, 202, response.body);
  return { input, value: await pollPull(input.operation_id, f.user.headers) };
}

// 持久Go JSON中的BIGINT必须保留原十进制字面，不能先经过JavaScript Number再转回字符串。
export function persistedPull(raw) {
  return JSON.parse(typeof raw === 'string' ? raw : JSON.stringify(raw), (_key, value, context) =>
    typeof value === 'number' ? context.source : value,
  );
}
