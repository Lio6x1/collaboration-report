# services/collaboration/src/transport/rpc_authorization.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/351-rpc-authorization.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：接入HTTP/RPC/WSS；只在业务确认后返回相应成功事实。

主要声明：`authorizeCall`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { readFileSync } from 'node:fs';
const policy = JSON.parse(
  readFileSync(new URL('../../../../contracts/proto/caller-policy.json', import.meta.url)),
);
const failure = (code, details) => ({ code, details });
export function authorizeCall(auth, method, deadline) {
  // grpc-js在mTLS握手成功后提供证书；应用继续检查URI身份及具体方法权限。
  if (auth?.transportSecurityType !== 'ssl' || !auth.sslPeerCertificate)
    return failure(16, 'SERVICE_IDENTITY_REQUIRED');
  const known = ['account', 'document', 'search', 'collaboration'];
  const identities = (auth.sslPeerCertificate.subjectaltname ?? '')
    .split(/,\s*/)
    .filter((x) => x.startsWith('URI:spiffe://collaboration-report/'))
    .map((x) => x.slice('URI:spiffe://collaboration-report/'.length));
  const roles = [...new Set(identities.filter((x) => known.includes(x)))];
  if (roles.length !== 1) return failure(16, 'SERVICE_IDENTITY_REQUIRED');
  const end = deadline instanceof Date ? deadline.getTime() : deadline;
  if (!Number.isFinite(end) || end <= Date.now() || end - Date.now() > 5000)
    return failure(3, 'FINITE_DEADLINE_REQUIRED');
  if (!policy[method]?.includes(roles[0])) return failure(7, 'SERVICE_METHOD_FORBIDDEN');
  return null;
}
````

UTF-8/LF全文SHA-256：`068d5900a3ca0059ba9a6b244c55ae9039f2e3c62cade8677188c014ba5657e6`。[参考实现](../../../../services/collaboration/src/transport/rpc_authorization.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
