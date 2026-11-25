import type { PublicDocument } from '../../../../contracts/http/generated';
import { api } from '../../lib/http';

export const publicDocumentPath = (document: string) => '/public/documents/' + document;
export const readCurrentPublication = (document: string) =>
  api<PublicDocument>('/v1/public/documents/' + document);

// 公开URL只携带文档身份；历史版号不能成为选择正文的参数。uint64始终保留十进制字符串。
export function publicDocumentFromPath(path: string): string | null {
  const match = /^\/public\/documents\/([1-9][0-9]*)\/?$/.exec(path);
  if (!match || BigInt(match[1]) > 18446744073709551615n) return null;
  return match[1];
}
