import { UserFacingError, api, ApiError, uploadBinary } from '../../lib/http';
import type { Quota, Resource } from '../../../../contracts/http/generated';

export const resourceUploadPath = (document: string) =>
  '/v1/documents/' + encodeURIComponent(document) + '/resources';
export const resourceCompletePath = (resource: string) =>
  '/v1/resources/' + encodeURIComponent(resource) + '/complete';
export const readQuota = (space: string) =>
  api<Quota>('/v1/spaces/' + encodeURIComponent(space) + '/quota');
export function uploadResourceBytes(address: string, file: File) {
  // 服务端句柄返回绝对地址；只把同源、精确上传路由归一化为共享HTTP允许的路径。
  // 不把CSRF凭据发送到其他源，也不放宽通用上传函数的路径保护。
  let url: URL;
  try {
    url = new URL(address, location.origin);
  } catch {
    throw new ApiError('INVALID_RESOURCE_UPLOAD_URL', 400);
  }
  if (
    url.origin !== location.origin ||
    url.username ||
    url.password ||
    url.href.includes('?') ||
    url.href.includes('#') ||
    !/^\/v1\/resources\/[1-9][0-9]{0,19}\/upload$/.test(url.pathname)
  )
    throw new ApiError('INVALID_RESOURCE_UPLOAD_URL', 400);
  return uploadBinary<Resource>(url.pathname, file);
}

export const maxImageBytes = 32 * 1024 * 1024;
export const imageTypes = ['image/png', 'image/jpeg', 'image/gif', 'image/webp'];

export async function imageChecksum(file: File): Promise<string> {
  // 先限制输入，避免对任意大文件分配内存；真实格式和完整性仍由服务端核验。
  if (!imageTypes.includes(file.type))
    throw new UserFacingError('请选择 PNG、JPEG、GIF 或 WebP 图片');
  if (file.size <= 0 || file.size > maxImageBytes)
    throw new UserFacingError('图片须大于 0 字节且不超过 32 MiB');
  const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer());
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('');
}
