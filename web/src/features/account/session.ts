import { ref } from 'vue';
import type { User, LoginResult } from '../../../../contracts/http/generated';
import {
  api,
  mutate,
  setTransportToken,
  transportGeneration,
  ApiError,
  actionsBlocked,
  sessionContextID,
  rotateSessionContext,
} from '../../lib/http';
export const user = ref<User | null>(null),
  sessionReady = ref(false);
export const identityUnavailable = ref(false);
export function validNewPassword(value: string) {
  // 与Account的新密码规则一致；已有密码登录不经过这里，也不改变密码的大小写和空格。
  const classes = [/[a-zA-Z]/.test(value), /[0-9]/.test(value), /[\p{P}\p{S}]/u.test(value)];
  return (
    Array.from(value).length >= 8 &&
    new TextEncoder().encode(value).length <= 72 &&
    classes.filter(Boolean).length >= 2
  );
}
const storageKey = 'collaboration-report.csrf';
const channel =
  typeof BroadcastChannel === 'undefined'
    ? null
    : new BroadcastChannel('collaboration-report.identity');
function storedToken(id: string) {
  // 先经/me确认本标签的HttpOnly Cookie身份，再取同一上下文的CSRF Cookie。
  // 不能读取其他标签的CSRF来补全登录；CSRF本身也不是服务端授权证明。
  const context = sessionContextID();
  if (typeof document !== 'undefined') {
    const prefix = '__Host-collabCSRF.' + context + '=';
    const cookie = document.cookie
      .split(';')
      .map((part) => part.trim())
      .find((part) => part.startsWith(prefix));
    const token = cookie?.slice(prefix.length);
    if (token) {
      sessionStorage.setItem(storageKey, JSON.stringify({ user_id: id, token, context }));
      return token;
    }
  }
  try {
    const v = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null');
    return v?.user_id === id && v.context === context ? v.token : '';
  } catch {
    return '';
  }
}
function clearLocal() {
  user.value = null;
  identityUnavailable.value = false;
  setTransportToken('');
  sessionStorage.removeItem(storageKey);
}
// 登录互不打扰；退出只影响同一会话副本，改密和注销才通知同一账号的其他标签。
// 频道消息只是重读提示，所有真实请求仍由服务端核验Cookie与撤权代次。
channel?.addEventListener('message', (event) => {
  const message = event.data;
  if (message?.type === 'session-closed') {
    if (message.context !== sessionContextID()) return;
  } else if (message?.type === 'account-closed' || message?.type === 'credentials-changed') {
    if (message.actor !== localActor()) return;
  } else return;
  clearLocal();
  void refreshSession();
});

function localActor(): string | null {
  if (user.value) return user.value.id;
  try {
    return JSON.parse(sessionStorage.getItem(storageKey) ?? 'null')?.user_id ?? null;
  } catch {
    return null;
  }
}
export async function refreshSession() {
  const started = transportGeneration();
  try {
    const me = await api<User>('/v1/account/me'),
      token = storedToken(me.id);
    if (!token) {
      user.value = null;
      setTransportToken('');
      return;
    }
    setTransportToken(token);
    user.value = me;
    identityUnavailable.value = false;
  } catch (e) {
    // 旧请求已失去作用，不能清空后来登录的用户；依赖503也不能删除真实会话凭据。
    if (
      started !== transportGeneration() ||
      (e instanceof ApiError && e.code === 'SESSION_CHANGED')
    )
      return;
    if (e instanceof ApiError && [401, 403].includes(e.status)) {
      user.value = null;
      if (!actionsBlocked.value) clearLocal();
    } else {
      // 断网/503没有证明身份已撤销。保留所属账号的本机输入界面并标明待核验；
      // 重新联网的HTTP与WSS仍要取得当前准入，缓存账号绝不成为服务端授权依据。
      identityUnavailable.value = true;
    }
  } finally {
    sessionReady.value = true;
  }
}
export function prepareLogin(email: string, password: string) {
  // 在整次注册/登录动作绑定代次之前准备槽，避免注册已成功但登录断网后无法恢复。
  // 同一次续接复用槽；下一次主动登录另建槽，复制标签不会覆盖来源标签的Cookie。
  rotateSessionContext();
  const context = sessionContextID();
  return () => completeLogin(email, password, context);
}
export async function login(email: string, password: string) {
  return prepareLogin(email, password)();
}
async function completeLogin(email: string, password: string, context: string) {
  // 这个闭包只续接所属的登录动作，不能借另一个账号的新槽继续旧注册。
  if (sessionContextID() !== context) throw new ApiError('SESSION_CHANGED', 409);
  const result = await api<LoginResult>('/v1/account/login', 'POST', { email, password });
  sessionStorage.setItem(
    storageKey,
    JSON.stringify({
      user_id: result.user_id,
      token: result.csrf_token,
      context: sessionContextID(),
    }),
  );
  setTransportToken(result.csrf_token);
  await refreshSession();
}
export async function logout(operationId: string) {
  await mutate('/v1/account/logout', { operation_id: operationId });
  clearLocal();
  channel?.postMessage({ type: 'session-closed', context: sessionContextID() });
}
export function afterCredentialChange() {
  const actor = localActor();
  clearLocal();
  if (actor) channel?.postMessage({ type: 'credentials-changed', actor });
}

export function afterAccountClosure(actor: string) {
  if (localActor() !== actor) return;
  const wasActive = user.value?.id === actor;
  clearLocal();
  // /me先拒绝时也清掉原账号残留CSRF；匿名恢复读取无需反复广播。
  if (wasActive) channel?.postMessage({ type: 'account-closed', actor });
}
