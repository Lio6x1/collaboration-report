import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root, runtime, state, saveJSON } from './runtime-lib.mjs';
import { localServicePeers } from './service-peers.mjs';

// 给已有本项目Doc配置补搜索通道；不用再次bootstrap数据库/重生成私钥或秘密。
// 下次完整配置生成仍走同一个localServicePeers，防止升级又丢失此入口。
const environment = state();
const file = resolve(runtime, 'services/document.json');
const config = JSON.parse(readFileSync(file));
if (
  config.role !== 'document' ||
  resolve(config.root) !== resolve(root) ||
  config.rpcPort !== environment.ports.documentRpc ||
  config.database.port !== environment.ports.mysql ||
  !Number.isInteger(environment.ports.searchRpc) ||
  environment.ports.searchRpc < 1 ||
  environment.ports.searchRpc > 65535
)
  throw Error('SEARCH_PAGE_ENVIRONMENT_BINDING_INVALID');
const peer = localServicePeers(environment.ports).search;
config.peers = { ...config.peers, search: peer };
saveJSON(file, config);
console.log(JSON.stringify({ state: 'CONFIGURED', role: 'document', searchPeerPresent: true }));
