// 底座升级和登录初始化共用同一服务地址表，避免重新生成配置时丢失内部HTTP通道。
// 身份仍由各客户端的mTLS校验负责；地址表只依据本项目已分配的端口生成。
export function localServicePeers(ports) {
  return {
    account: `localhost:${ports.accountRpc}`,
    accountHttp: `https://127.0.0.1:${ports.accountHttp}`,
    edge: `https://localhost:${ports.edge}`,
    document: `localhost:${ports.documentRpc}`,
    documentHttp: `https://127.0.0.1:${ports.documentHttp}`,
    collaboration: `localhost:${ports.collaborationRpc}`,
    collaborationHttp: `https://127.0.0.1:${ports.collaborationHttp}`,
    search: `localhost:${ports.searchRpc}`,
  };
}
