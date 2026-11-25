import { ref, shallowRef, computed } from 'vue';
import type { Operation, AuthorizationResult } from '../../../contracts/http/generated';
let generation = 0;
let csrf = '';
let tabContext: string | undefined;
const tabContextKey = 'collaboration-report.session-context';
export function sessionContextID() {
  if (tabContext !== undefined) return tabContext;
  try {
    const saved = sessionStorage.getItem(tabContextKey);
    if (saved && /^[a-f0-9]{32}$/.test(saved)) tabContext = saved;
  } catch {
    // 浏览器禁用存储时仍允许当前页面使用内存上下文；刷新后需要重新登录。
  }
  tabContext ??= crypto.randomUUID().replaceAll('-', '');
  try {
    sessionStorage.setItem(tabContextKey, tabContext);
  } catch {
    /* 内存上下文继续有效。 */
  }
  return tabContext;
}
export function rotateSessionContext() {
  // 复制标签可能复制sessionStorage；每次主动登录使用新slot，不能覆盖原标签的账号。
  tabContext = crypto.randomUUID().replaceAll('-', '');
  try {
    sessionStorage.setItem(tabContextKey, tabContext);
  } catch {
    /* 不持久化登录秘密。 */
  }
  csrf = '';
  generation++;
  epoch.value = generation;
}
export function sessionContentURL(path: string) {
  const url = new URL(path, 'https://session-context.invalid');
  if (
    !/^\/v1\/(resources\/[1-9][0-9]*\/content|export-jobs\/[1-9][0-9]*\/download)$/.test(
      url.pathname,
    )
  )
    throw new UserFacingError('文件读取入口无效，请重新打开正文');
  // 选择器不是登录凭据，原img和下载链接仍依赖浏览器的HttpOnly Cookie与服务端权限。
  url.searchParams.set('session_context', sessionContextID());
  return url.pathname + url.search;
}
const epoch = ref(0);
// 只用于让只读页面及时清空旧会话结果，真实权限仍由服务端Cookie核验。
export const transportEpoch = computed(() => epoch.value);
export function setTransportToken(token: string) {
  if (csrf !== token) {
    csrf = token;
    generation++;
    epoch.value = generation;
  }
}
export function transportGeneration() {
  return generation;
}
const activeRuns = ref(0);
// 待确认操作由应用级内存持有；组件卸载不能丢掉原编号，也不能借新登录重试旧写入。
export const recoveryAction = shallowRef<ReturnType<typeof useAction> | null>(null);
export const actionsBlocked = computed(() => activeRuns.value > 0 || recoveryAction.value !== null);
export class ApiError extends Error {
  constructor(
    public code: string,
    public status: number,
  ) {
    super(code);
  }
}

// 只有前端自己编写的校验提示才可直接展示；浏览器、驱动和服务端原始message一律不可信。
export class UserFacingError extends Error {}

// 保留确定业务终态及原冲突入口，供审核页面展示原计算输入；错误不变成一次新写入。
export class OperationRejected extends ApiError {
  constructor(public operation: Operation) {
    super(operation.error?.code ?? 'OPERATION_REJECTED', 409);
  }
}
class PendingOperation extends Error {
  constructor(
    public id: string,
    public resume: () => Promise<unknown>,
  ) {
    super('操作结果尚未确认，请查询或重试原操作');
  }
}
export const passwordPolicyMessage =
  '密码至少8个字符，英文字母、数字、特殊字符至少包含两类，区分大小写；UTF-8编码不能超过72字节。';
const messages: Record<string, string> = {
  EMAIL_ALREADY_REGISTERED: '该邮箱已注册，请直接登录；忘记密码可使用“找回密码”。',
  SIGNUP_CONFLICT: '该邮箱已注册，请直接登录；忘记密码可使用“找回密码”。',
  SIGNUP_INPUT_INVALID: '请检查邮箱、显示名称和密码后重新提交。',
  LOGIN_INPUT_INVALID: '请填写有效的邮箱和密码。',
  LOGIN_RATE_LIMITED: '登录尝试过于频繁，请稍后再试。',
  CHALLENGE_INPUT_INVALID: '邮箱格式不正确，请检查后重新发送验证码。',
  CHALLENGE_RATE_LIMITED: '验证码发送过于频繁，请等待倒计时结束后重试。',
  SMTP_DELIVERY_FAILED: '验证码邮件暂时未能发送，请稍后重新获取。',
  CHALLENGE_STORE_UNAVAILABLE: '验证码服务暂时不可用，请稍后重试。',
  PASSWORD_INVALID: '密码不能为空，且不能超过72字节，请调整后重试。',
  PASSWORD_POLICY_INVALID: passwordPolicyMessage,
  PASSWORD_INPUT_INVALID: '请检查当前密码和新密码后重新提交。',
  RESET_INPUT_INVALID: '请检查验证码和新密码后重新提交。',
  EMAIL_INPUT_INVALID: '新邮箱格式不正确，请检查后重试。',
  SERVICE_UNAVAILABLE: '服务暂时不可用，请稍后重试。',
  BROWSER_SESSION_LIMIT: '当前浏览器登录会话较多，请先在不用的标签页退出登录，再重新登录。',
  SESSION_CONTEXT_INVALID: '当前标签页的登录状态异常，请重新打开页面并登录。',
  DEPENDENCY_TIMEOUT: '服务响应较慢，请稍后重试。',
  REQUEST_TIMEOUT: '请求超时，请检查网络后重试。',
  NETWORK_UNAVAILABLE: '网络连接中断，请检查网络后重试。',
  REQUEST_CANCELLED: '请求已中断，请稍后重试。',
  RESPONSE_INVALID: '服务返回异常，请稍后重试。',
  SESSION_CHANGED: '登录账号已变化，请重新打开当前页面。',
  ORIGIN_FORBIDDEN: '当前页面的访问地址无效，请从网站首页重新打开。',
  REQUEST_INVALID: '提交的信息不完整或格式不正确，请检查后重试。',
  ENTITY_ID_INVALID: '目标已失效，请刷新列表后重新选择。',
  OPERATION_ID_CONFLICT: '这次提交与之前的操作不一致，请刷新页面后重新确认。',
  OPERATION_REJECTED: '这次操作未能完成，请刷新当前内容后重新确认。',
  RETRY_BINDING_CHANGED: '待确认的操作已变化，请先查询原操作结果。',
  OPERATION_PENDING: '操作仍在处理中，请稍后查询结果。',
  INVALID_REQUEST_PATH: '操作入口已失效，请刷新页面后重试。',
  INVALID_RESOURCE_UPLOAD_URL: '上传入口已失效，请重新选择图片。',
  QUOTA_EXCEEDED: '空间可用容量不足，请清理资源后重试',
  QUOTA_REVISION_CHANGED: '空间用量已变化，请刷新用量后重试',
  RESOURCE_CHECKSUM_MISMATCH: '文件内容与声明不一致，请重新选择原文件',
  RESOURCE_UPLOAD_INCOMPLETE: '文件尚未完整上传，请继续原上传',
  RESOURCE_UPLOAD_BUSY: '该文件正在上传或处理中，请稍后重试',
  RESOURCE_NOT_READY: '资源尚未就绪，暂时不能插入或读取',
  RESOURCE_REFERENCE_REQUIRED: '当前正文上下文没有该资源的有效引用',
  RESOURCE_CONTEXT_CHANGED: '资源阅读上下文已变化，请刷新正文',
  SESSION_INVALID: '会话已失效，请重新登录',
  SESSION_OR_CSRF_INVALID: '会话或安全凭据已失效，请重新登录',
  CSRF_INVALID: '安全凭据已变化，请重新登录',
  DOCUMENT_READ_NOT_ALLOWED: '当前没有文档读取资格',
  DOCUMENT_MANAGEMENT_REQUIRED: '此动作需要文档管理权',
  DOCUMENT_CREATION_NOT_ALLOWED: '请先取得本知识库的文档创建资格',
  IDENTITY_NOT_CURRENT: '当前身份不具备此动作资格',
  ADMIN_RETIREMENT_REQUIRED: '请先交接并解除自己的管理员角色',
  LAST_KB_ADMIN_REQUIRED: '知识库必须保留至少一名管理员',
  OWNER_CANNOT_BE_REMOVED_OR_DEMOTED: '请先转让组织所有者',
  CONTROL_REVISION_MISMATCH: '文档已变化，请刷新后操作',
  PLATFORM_GOVERNANCE_REQUIRED: '当前账号没有平台留存治理资格',
  ISOLATED_PUBLIC_RETAINED_REQUIRED: '此入口只处理注销后孤立的公开留存文章',
  RETAINED_DRAIN_PENDING: '上一个治理操作仍在关闭输出，请继续查询原操作',
  ACL_REVISION_MISMATCH: '授权已变化，请刷新后操作',
  KB_ACL_REVISION_MISMATCH: '知识库授权已变化，请刷新后操作',
  REVISION_CHANGED: '状态已变化，请刷新后操作',
  INVITATION_NOT_CURRENT: '邀请已关闭或过期',
  SHARING_FILTER_INVALID: '列表筛选条件无效，请重新选择。',
  SHARING_RECORD_CHANGED: '这条记录已被更新，请刷新列表后重试。',
  SHARING_RECORD_ACTIVE: '请先关闭有效邀请，或处理待审批申请，再删除记录。',
  APPLICATION_CHANGED: '这条申请已被处理，请刷新列表查看最新状态。',
  CHALLENGE_INVALID: '验证码不正确或已过期，请核对最新邮件中的验证码，或重新获取。',
  CHALLENGE_ALREADY_CONSUMED: '验证码已使用，请重新获取验证码。',
  CURRENT_PASSWORD_INVALID: '当前密码不正确',
  LOGIN_INVALID: '邮箱或密码不正确',
  CURSOR_STALE: '列表资格或游标已变化，请从第一页刷新',
  READ_VIEW_NOT_CURRENT: '阅读视图已过期或资格已变化，请刷新',
  LINEAGE_CHANGED: '共同草稿已更新，请重新打开后发起新的动作',
  DRAFT_CHANGED: '固定后有新的草稿输入，请保留这些内容并重新核对',
  PULL_MAIN_CONFLICT: '主线与草稿有冲突，请先核对并修改草稿后再拉取',
  DISCARD_PREVIEW_EXPIRED: '丢弃预览已过期，请关闭后重新核对',
  DISCARD_CHANGES_CHANGED: '修改要求已变化，请关闭预览后重新逐条核对',
  DISCARD_REVIEW_CHANGED: '审核轮次已变化，请刷新并重新核对',
  REVIEW_WITHDRAW_REQUIRED: '请先撤回当前审核，再处理这份草稿',
  MERGE_CONFLICT_REQUIRES_RESOLUTION: '本次固定输入有冲突，请查看原计算内容并处理后重新送审',
  CHANGES_PENDING: '仍有未解除的修改要求，请逐条处理后再通过',
  MERGE_APPROVAL_NOT_ALLOWED: '当前没有通过此请求的资格，请刷新审核记录',
  REVIEW_ROUND_CHANGED: '审核轮次已变化，请重新核对当前固定正文',
  MERGE_OPERATION_ACTIVE: '已有通过并合并操作，请继续查询原操作',
  DRAFT_MANAGE_REQUIRED: '此草稿动作需要当前文档管理权',
  MAIN_CONTEXT_CHANGED: '正式主线已变化，请重新核对后发起新的动作',
  REVERSE_REVIEW_PENDING: '该单元已有审核工作，请先处理当前请求',
  REVERSE_RANGE_CHANGED: '原合并范围已退役或删除，不能直接反向',
  REVERSE_CONFLICT: '反向修改与当前正文有冲突，请核对保留的固定输入',
  STRUCTURE_PREVIEW_CHANGED:
    '预览后有新的已保存修改或范围变化，请关闭此预览并重新生成，其他编辑者的内容已保留',
  STRUCTURE_EDITORS_NOT_READY: '有编辑者尚未完成保存或连接已中断，本次目录调整已取消，请稍后重试',
  STRUCTURE_CHANGE_PENDING: '正在调整章节结构，保存完成后将自动恢复编辑',
  STRUCTURE_DRAFT_CONFLICT:
    '主线与共同支线修改了同一处内容，无法自动合并。请先处理支线冲突，再重新生成预览',
  STRUCTURE_PREVIEW_EXPIRED: '目录预览已过期，请生成新的预览并重新核对',
  STRUCTURE_LEVEL2_MISSING:
    '本章主线缺少二级标题。请先在主线编辑器设置二级标题并保存，共同支线会在预览时自动对齐。',
  DRAFT_STRUCTURE_CHANGED:
    '共同支线有尚未合入主线的二级标题修改，自动对齐后边界仍不一致。请先审核或处理这些标题，再重新生成目录预览。',
  STRUCTURE_TITLE_INVALID: '小节标题不能为空或超过240个字，请修改二级标题后重新生成预览。',
  STRUCTURE_TITLE_CHANGED: '小节标题与原目录不一致，请先核对标题修改后再合章。',
  STRUCTURE_MODE_CHANGED: '章节的拆分状态已变化，请重新选择拆分或合章操作。',
  FULL_OUTPUT_DRAIN_REQUIRED: '关闭仍在处理中，请稍后查询',
  PUBLICATION_CHANGED: '当前正式版已变化，请刷新后重新确认',
  PUBLICATION_NOT_ALLOWED: '发布、切换和撤下需要当前文档管理权',
  MAIN_CONTROL_CHANGED: '主线管理状态已变化，请刷新后重新确认',
  PUBLICATION_NOT_PUBLISHED: '当前没有已发布的正式版',
  PUBLICATION_ALREADY_CURRENT: '选择的版本已经是当前正式版',
  PUBLICATION_TRANSFER_LIMIT: '整篇主线超过当前发布容量，请调整后再发布',
  PUBLICATION_SOURCE_CHANGED: '发布准备期间主线或管理资格发生变化，请重新核对',
  RELEASE_HISTORY_CHANGED: '正式版本列表已变化，请从第一页刷新',
  DOCUMENT_NOT_FOUND: '文档不存在或已被删除，请返回文档列表。',
  DOCUMENT_ACCESS_CHANGED: '文档访问权限已变化，请刷新页面后重试。',
  SCOPE_TRANSITION_PENDING: '权限调整仍在处理中，请稍后重试。',
  COLLABORATOR_NOT_FOUND: '该协作者已不在当前文档中，请刷新协作者列表。',
  INVITATION_EXPIRED: '邀请已过期，请联系邀请人重新发送。',
  INVITATION_NOT_FOUND: '邀请不存在或已被撤销，请联系邀请人确认。',
  SELF_APPROVAL_NOT_ALLOWED: '不能审批自己提交的修改，请邀请其他有审核权限的协作者处理。',
  EXPORT_MAIN_VIEW_EXPIRED: '阅读内容已更新，请刷新阅读后重新导出。',
  EXPORT_NOT_READY: '文件正在生成，请稍后查看导出任务。',
  EXPORT_NOT_ALLOWED: '当前没有导出权限，请联系文档管理员。',
  EXPORT_RENDER_FAILED: '文件生成失败，请稍后重试；若仍失败，请联系管理员。',
  EXPORT_CONVERSION_FAILED: '文件转换失败，请稍后重试；若仍失败，请联系管理员。',
  EXPORT_PDF_RENDER_FAILED: 'PDF生成失败，请稍后重新导出；若仍失败，请联系管理员。',
  EXPORT_CONVERTER_UNAVAILABLE: '文件转换服务暂时不可用，请稍后重新导出。',
  EXPORT_CONVERSION_LIMIT: '正文超过当前导出处理容量，请缩小导出范围或调整内容。',
  EXPORT_FORMAT_INVALID: '导出格式不支持，请重新选择 Word、PDF 或 Markdown。',
  EXPORT_SOURCE_CHANGED: '导出内容已变化，请刷新阅读后重新导出。',
  EXPORT_FAILED: '文件生成失败，请稍后重新导出。',
  RESOURCE_NOT_FOUND: '图片不存在或已被删除，请重新上传或联系文档管理员。',
  RESOURCE_CONTENT_TYPE_INVALID: '图片格式不支持，请选择 PNG、JPEG、GIF 或 WebP 图片。',
  RESOURCE_SIZE_INVALID: '图片大小不符合要求，请选择不超过32 MiB的有效图片。',
  TICKET_EXPIRED: '编辑连接已过期，正在重新连接。',
  TICKET_INVALID: '编辑连接已失效，请重新打开编辑页面。',
  ACCESS_CHANGED: '编辑权限已变化，本机输入会保留，请重新打开编辑页面。',
  WORKSPACE_CHANGED: '编辑内容已更新，本机输入会保留，请重新打开编辑页面。',
  SOURCE_CLOSED: '当前编辑权限已关闭，本机输入会保留，请联系文档管理员。',
  INVALID_CONTENT: '正文格式无法保存，请检查最近插入的内容后重试。',
  DEPENDENCIES_MISSING: '部分编辑内容尚未同步完成，请等待连接恢复后再次保存。',
  UPDATE_TOO_LARGE: '一次修改的内容过多，请分段粘贴或导入后重试。',
  WORKSPACE_EPOCH_CHANGED: '编辑内容已更新，本机未保存的输入会保留，请重新打开编辑页面。',
  MAIN_LOCKED: '主线已锁定，暂时无法编辑，请联系文档管理员。',
  UPLOAD_FAILED: '图片上传未完成，请检查网络后重试。',
  PERSONAL_TRANSFER_EXPIRED: '转让申请已过期，请联系发起人重新申请。',
};
export function explain(error: unknown, fallback = '操作未完成，请稍后重试。'): string {
  if (error instanceof PendingOperation)
    return '操作结果尚未确认，请查询或重试原操作，避免重复提交。';
  if (error instanceof UserFacingError) return error.message;
  if (error instanceof ApiError) {
    if (Object.hasOwn(messages, error.code)) return messages[error.code]!;
    if (error.status === 401) return '登录已过期，请重新登录后继续。';
    if (error.status === 403) return '你没有执行此操作的权限，请联系管理员。';
    if (error.status === 404) return '内容不存在或已无法访问，请返回列表重新选择。';
    if (error.status === 409) return '内容或状态已变化，请刷新页面后重新确认。';
    if (error.status === 413) return '提交的内容过大，请缩小文件或分段提交。';
    if (error.status === 429) return '操作过于频繁，请稍后再试。';
    if (error.status >= 500) return '服务暂时不可用，请稍后重试。';
    if (error.status >= 400) return '提交的信息有误，请检查后重试。';
  }
  return fallback;
}
export async function api<T>(
  path: string,
  method = 'GET',
  body?: unknown,
  onHeaders?: (headers: Headers) => void,
): Promise<T> {
  return requestJSON<T>(
    path,
    method,
    body === undefined ? undefined : JSON.stringify(body),
    'application/json',
    8000,
    onHeaders,
  );
}

// 二进制上传复用相同会话代次与CSRF，不在功能模块复制身份状态。
export function uploadBinary<T>(path: string, body: Blob): Promise<T> {
  if (!/^\/v1\/resources\/[1-9][0-9]{0,19}\/upload$/.test(path))
    return Promise.reject(new ApiError('INVALID_REQUEST_PATH', 400));
  return requestJSON<T>(path, 'PUT', body, 'application/octet-stream', 35000);
}

async function requestJSON<T>(
  path: string,
  method: string,
  body: BodyInit | undefined,
  contentType: string,
  timeoutMs: number,
  onHeaders?: (headers: Headers) => void,
): Promise<T> {
  if (!path.startsWith('/v1/') || path.startsWith('//'))
    throw new ApiError('INVALID_REQUEST_PATH', 400);
  const atStart = generation,
    controller = new AbortController(),
    timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(path, {
      method,
      credentials: 'same-origin',
      cache: 'no-store',
      headers: {
        'X-Collab-Session-Context': sessionContextID(),
        ...(body !== undefined ? { 'Content-Type': contentType } : {}),
        ...(csrf ? { 'X-CSRF-Token': csrf } : {}),
      },
      body,
      signal: controller.signal,
    });
    // 网关错误可能返回HTML或空正文；先保留HTTP类别，不能把JSON解析异常原样交给用户。
    let value: unknown;
    try {
      value = await response.json();
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError')
        throw new ApiError('REQUEST_TIMEOUT', 0);
      if (error instanceof TypeError) throw new ApiError('NETWORK_UNAVAILABLE', 0);
      throw new ApiError(
        response.ok ? 'RESPONSE_INVALID' : 'REQUEST_FAILED',
        response.ok ? 502 : response.status,
      );
    }
    if (atStart !== generation) throw new ApiError('SESSION_CHANGED', 409);
    if (!response.ok) {
      const code = value && typeof value === 'object' && 'code' in value ? value.code : undefined;
      throw new ApiError(
        typeof code === 'string' && /^[A-Z][A-Z0-9_]{0,100}$/.test(code) ? code : 'REQUEST_FAILED',
        response.status,
      );
    }
    if (!value || typeof value !== 'object') throw new ApiError('RESPONSE_INVALID', 502);
    onHeaders?.(response.headers);
    return value as T;
  } catch (error) {
    if (error instanceof ApiError || error instanceof UserFacingError) throw error;
    // status=0表示尚未取得确定的HTTP结果；写请求恢复必须继续原编号，不能重新登记。
    throw new ApiError(
      error instanceof Error && error.name === 'AbortError'
        ? 'REQUEST_TIMEOUT'
        : 'NETWORK_UNAVAILABLE',
      0,
    );
  } finally {
    clearTimeout(timeout);
  }
}
async function settle<T extends Operation | AuthorizationResult>(value: T): Promise<T> {
  const completed = (current: Operation | AuthorizationResult) =>
    current.state === 'SUCCEEDED' &&
    (!('result' in current) || current.result?.main_sync_state !== 'SYNCING');
  if (completed(value)) return value;
  if (value.state === 'REJECTED' || value.state === 'CANCELLED')
    throw new OperationRejected(value as Operation);
  const path =
    'scope_key' in value
      ? '/v1/authorization-changes/' + value.operation_id
      : (value.poll_url ?? '/v1/operations/' + value.operation_id);
  let current: Operation | AuthorizationResult = value;
  for (let n = 0; n < 80; n++) {
    await new Promise((resolve) => setTimeout(resolve, 200));
    try {
      current = await api<Operation | AuthorizationResult>(path);
    } catch (e) {
      // 原号已登记，结构安装推进ACL时查询可能暂时拒绝旧来源。
      // 只继续这两种明确的关闭中状态；真正版本冲突/403仍立即停止，绝不重POST。
      if (
        e instanceof ApiError &&
        e.status === 409 &&
        ['DOCUMENT_ACCESS_CHANGED', 'SCOPE_TRANSITION_PENDING'].includes(e.code)
      )
        continue;
      // 进度请求丢失也属于未知结果；保留已知operation_id，不能允许另发新操作。
      if (!(e instanceof ApiError) || e.status === 0 || e.status >= 500)
        throw new PendingOperation(value.operation_id, () => settle(current));
      throw e;
    }
    if (completed(current)) return current as T;
    if (current.state === 'REJECTED' || current.state === 'CANCELLED')
      throw new OperationRejected(current as Operation);
  }
  throw new PendingOperation(value.operation_id, () => settle(current));
}
export async function write<T>(
  path: string,
  input: Record<string, unknown>,
  method = 'POST',
): Promise<T> {
  // 只在第一次调用生成编号。网络错误或503表示未知结果，重试闭包使用完全相同的输入，
  // 不重新生成operation_id，不把未知结果显示成成功，也不把密码/邀请token写进浏览器存储。
  const fixed = { ...input, operation_id: input.operation_id ?? crypto.randomUUID() },
    id = String(fixed.operation_id);
  let responseUnknown = false;
  async function attempt(): Promise<T> {
    try {
      return await api<T>(path, method, fixed);
    } catch (e) {
      if (
        !(e instanceof ApiError) ||
        e.status === 0 ||
        e.status >= 500 ||
        e.code === 'SESSION_CHANGED'
      ) {
        responseUnknown = true;
        throw new PendingOperation(id, attempt);
      }
      // 原响应未知后，结构安装推进ACL的短暂409并不能否定原操作。
      // 只保留这两个明确过渡码；首次确定冲突和其它终态照常交给调用方。
      if (
        responseUnknown &&
        e.status === 409 &&
        ['DOCUMENT_ACCESS_CHANGED', 'SCOPE_TRANSITION_PENDING'].includes(e.code)
      )
        throw new PendingOperation(id, attempt);
      throw e;
    }
  }
  return attempt();
}
export async function mutate(path: string, input: Record<string, unknown>, method = 'POST') {
  return settleMutation(() => write<Operation | AuthorizationResult>(path, input, method));
}

async function settleMutation(
  send: () => Promise<Operation | AuthorizationResult>,
): Promise<Operation | AuthorizationResult> {
  try {
    return await settle(await send());
  } catch (error) {
    if (error instanceof PendingOperation) {
      // 登记响应未知时续接同号POST，登记已知后续接GET；恢复闭包总是包含尚未完成的后续步骤。
      throw new PendingOperation(error.id, () =>
        settleMutation(async () => (await error.resume()) as Operation | AuthorizationResult),
      );
    }
    throw error;
  }
}
export async function publicMutation(
  path: string,
  input: Record<string, unknown>,
): Promise<Operation> {
  const fixed = { ...input, operation_id: input.operation_id ?? crypto.randomUUID() };
  async function resume(): Promise<Operation> {
    for (let n = 0; n < 80; n++) {
      let v: Operation;
      try {
        v = await write<Operation>(path, fixed);
      } catch (error) {
        if (error instanceof PendingOperation)
          throw new PendingOperation(String(fixed.operation_id), resume);
        throw error;
      }
      if (v.state === 'SUCCEEDED') return v;
      if (v.state === 'REJECTED' || v.state === 'CANCELLED') throw new OperationRejected(v);
      await new Promise((r) => setTimeout(r, 200));
    }
    throw new PendingOperation(String(fixed.operation_id), resume);
  }
  // 公开找回流程没有正常会话；用原验证码和同号请求重取最小结果，不借用私有操作列表。
  return resume();
}
export function useAction() {
  const busy = ref(false),
    error = ref(''),
    notice = ref(''),
    pending = ref(''),
    retry = ref<null | (() => Promise<void>)>(null);
  let requests: Array<{
      path: string;
      method: string;
      input: Record<string, unknown>;
      complete?: boolean;
      value?: unknown;
      resume?: () => Promise<unknown>;
    }> = [],
    cursor = 0;
  let actorGeneration = 0;
  type Lazy<T> = T | (() => T);
  function bind(
    path: Lazy<string>,
    input: Lazy<Record<string, unknown>>,
    method: string,
  ): (typeof requests)[number] {
    const index = cursor++,
      previous = requests[index];
    if (previous) {
      if (previous.method !== method) throw new ApiError('RETRY_BINDING_CHANGED', 409);
      return previous;
    }
    const resolvedPath = typeof path === 'function' ? path() : path;
    const resolvedInput = typeof input === 'function' ? input() : input;
    const fixed = JSON.parse(
      JSON.stringify({
        ...resolvedInput,
        operation_id: resolvedInput.operation_id ?? crypto.randomUUID(),
      }),
    );
    return (requests[index] = { path: resolvedPath, method, input: fixed });
  }
  async function execute<T>(
    path: Lazy<string>,
    input: Lazy<Record<string, unknown>>,
    method: string,
    send: (path: string, input: Record<string, unknown>, method: string) => Promise<T>,
  ): Promise<T> {
    if (actorGeneration !== generation) throw new ApiError('SESSION_CHANGED', 409);
    const request = bind(path, input, method);
    // 业务终态已知后只续接后续读取，不能再次POST；输入求值也推迟到第一次发送。
    if (request.complete) return request.value as T;
    try {
      const value = request.resume
        ? ((await request.resume()) as T)
        : await send(request.path, request.input, request.method);
      request.value = value;
      request.complete = true;
      request.resume = undefined;
      return value;
    } catch (error) {
      if (error instanceof PendingOperation) request.resume = error.resume;
      throw error;
    }
  }
  const scopedWrite = <T>(
    path: Lazy<string>,
    input: Lazy<Record<string, unknown>>,
    method = 'POST',
  ) => execute<T>(path, input, method, write<T>);
  const scopedMutate = (
    path: Lazy<string>,
    input: Lazy<Record<string, unknown>>,
    method = 'POST',
  ) => execute(path, input, method, mutate);
  const scopedPublic = (path: Lazy<string>, input: Lazy<Record<string, unknown>>) =>
    execute(path, input, 'POST', publicMutation);
  async function run(task: () => Promise<unknown>, success = '操作已完成', replaying = false) {
    if (busy.value || (!replaying && (pending.value || recoveryAction.value))) return;
    if (replaying && actorGeneration !== generation) {
      error.value = '登录身份已变化，不能用新身份续接旧操作；请用原账号查询原操作编号';
      return;
    }
    busy.value = true;
    activeRuns.value++;
    error.value = '';
    notice.value = '';
    pending.value = '';
    retry.value = null;
    if (!replaying) {
      requests = [];
      actorGeneration = generation;
    }
    cursor = 0;
    try {
      await task();
      notice.value = success;
    } catch (e) {
      // 重试整个业务续接（如注册之后登录），每个写请求复用第一次的固定输入和编号。
      // 不能只重取receipt就宣布外层流程成功，也不能重新执行回调时生成第二个操作号。
      error.value = explain(e);
      const completed = requests.find((r) => r.complete);
      if (
        e instanceof PendingOperation ||
        (completed && (!(e instanceof ApiError) || e.status === 0 || e.status >= 500))
      ) {
        pending.value =
          e instanceof PendingOperation ? e.id : String(completed!.input.operation_id);
        if (completed) notice.value = '业务操作已确认完成，页面刷新尚未完成';
        retry.value = () => run(task, success, true);
        recoveryAction.value = action;
      } else if (completed && e instanceof ApiError && [403, 404].includes(e.status)) {
        // 转交或撤销自己的管理权后，旧管理页403是确认后的资格变化。
        error.value = '';
        notice.value = success + '；当前资格已变化，旧视图已关闭';
      }
    } finally {
      busy.value = false;
      activeRuns.value--;
      if (!pending.value && recoveryAction.value === action) recoveryAction.value = null;
      // 确定终态后释放固定输入（可能含验证码/密码）；仅未知操作继续留在内存等待续接。
      if (!pending.value) requests = [];
    }
  }
  const action = {
    busy,
    error,
    notice,
    pending,
    retry,
    run,
    write: scopedWrite,
    mutate: scopedMutate,
    publicMutation: scopedPublic,
  };
  return action;
}
