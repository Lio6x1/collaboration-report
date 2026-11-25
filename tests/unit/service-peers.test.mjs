import test from 'node:test';
import assert from 'node:assert/strict';
import { localServicePeers } from '../../scripts/service-peers.mjs';

test('base bootstrap supplies the internal HTTP endpoints used after login setup', () => {
  // 追加迁移会重新生成配置；不能把登录阶段已经接通的正文与输出控制通道删除。
  const ports = {
    accountRpc: 21001,
    accountHttp: 21002,
    documentRpc: 21003,
    documentHttp: 21004,
    collaborationRpc: 21005,
    collaborationHttp: 21006,
    edge: 21007,
    searchRpc: 21008,
  };
  assert.deepEqual(localServicePeers(ports), {
    account: 'localhost:21001',
    accountHttp: 'https://127.0.0.1:21002',
    document: 'localhost:21003',
    documentHttp: 'https://127.0.0.1:21004',
    collaboration: 'localhost:21005',
    collaborationHttp: 'https://127.0.0.1:21006',
    edge: 'https://localhost:21007',
    search: 'localhost:21008',
  });
});
