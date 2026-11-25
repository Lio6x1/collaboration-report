import assert from 'node:assert/strict';
import { get, json } from './workspace.mjs';

// 等待真实原号终态；Go成功与Node完成安装分别断言，不用另一个POST触发后台恢复。
export async function pollMainControl(id, user, { ready = true, timeout = 20000 } = {}) {
  const deadline = Date.now() + timeout;
  let value;
  while (Date.now() < deadline) {
    const response = await get(`/v1/operations/${id}`, user.headers);
    // 结构事务切换ACL时，新来源须等真实fence收尾；这两个明确瞬态只重查原号，不重POST或接受READY。
    if (
      response.status === 409 &&
      ['DOCUMENT_ACCESS_CHANGED', 'SCOPE_TRANSITION_PENDING'].includes(json(response).code)
    ) {
      await new Promise((resolve) => setTimeout(resolve, 100));
      continue;
    }
    assert.equal(response.status, 200, json(response).code);
    value = json(response);
    if (value.state !== 'PREPARING' && (!ready || value.result?.main_sync_state !== 'SYNCING'))
      return value;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  assert.fail(
    'MAIN_CONTROL_NOT_READY ' +
      JSON.stringify({
        state: value?.state,
        sync: value?.result?.main_sync_state,
        code: value?.error?.code,
      }),
  );
}

export async function completedMainControl(response, user) {
  assert.equal(response.status, 202, json(response).code);
  return pollMainControl(json(response).operation_id, user);
}
