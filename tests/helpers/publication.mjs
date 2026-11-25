import assert from 'node:assert/strict';
import { get, post, json, operation } from './workspace.mjs';

// 登记与查询复用原号；发布业务和旧输出关闭均完成才接受成功，不再POST推进恢复。
export async function pollPublication(id, user, timeout = 20000) {
  const deadline = Date.now() + timeout;
  let value;
  while (Date.now() < deadline) {
    let response;
    try {
      response = await get(`/v1/operations/${id}`, user.headers);
    } catch (error) {
      // 已登记操作的大正文最终事务可能挡住一次进度读取。传输未知只允许
      // 在原总预算内再次GET同号，不能重新POST、加长请求预算或把未知判为成功。
      if (
        !['TEST_TIMEOUT', 'TEST_INCOMPLETE_BODY'].includes(error.message) &&
        !['ECONNRESET', 'ECONNREFUSED', 'ETIMEDOUT'].includes(error.code)
      )
        throw error;
      await new Promise((resolve) => setTimeout(resolve, 100));
      continue;
    }
    if (
      response.status === 503 &&
      ['DEPENDENCY_TIMEOUT', 'SERVICE_UNAVAILABLE'].includes(json(response).code)
    ) {
      await new Promise((resolve) => setTimeout(resolve, 100));
      continue;
    }
    assert.equal(response.status, 200, json(response).code);
    value = json(response);
    if (value.state === 'SUCCEEDED' || value.state === 'REJECTED') return value;
    assert.equal(value.state, 'PREPARING');
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  assert.fail(`PUBLICATION_NOT_TERMINAL ${value?.state ?? 'UNKNOWN'}`);
}

export async function releaseInput(document, user) {
  const response = await get(`/v1/documents/${document}`, user.headers);
  assert.equal(response.status, 200, json(response).code);
  const current = json(response);
  return {
    operation_id: operation('release-create'),
    expected_publication_revision: current.publication_epoch,
    expected_control_revision: current.control_revision,
  };
}

export async function publishRelease(document, user, input) {
  const response = await post(`/v1/documents/${document}/releases`, input, user.headers);
  assert.equal(response.status, 202, json(response).code);
  const result = await pollPublication(input.operation_id, user);
  assert.equal(result.state, 'SUCCEEDED', result.error?.code);
  assert.ok(result.result.entity_id);
  return result;
}
