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
