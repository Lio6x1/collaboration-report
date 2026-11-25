import type {
  CommentPage,
  CommentThread,
  CommentThreadPage,
  DocumentCollaboratorPage,
} from '../../../../contracts/http/generated';
import { api } from '../../lib/http';

export const threadsPath = (document: string) =>
  '/v1/documents/' + encodeURIComponent(document) + '/comment-threads';
export const commentsPath = (thread: string) =>
  '/v1/comment-threads/' + encodeURIComponent(thread) + '/comments';
export const threadStatePath = (thread: string) =>
  '/v1/comment-threads/' + encodeURIComponent(thread) + '/state';
export const commentPath = (comment: string) => '/v1/comments/' + encodeURIComponent(comment);
const pageQuery = (cursor: string | null) =>
  '?limit=20' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : '');

// 写接口统一返回Operation，调用方用useAction.mutate确认终态，不把回执强转成正文DTO。
export type ThreadFilter = {
  kind: 'MAIN' | 'BRANCH' | 'REVIEW';
  context?: string;
  version?: string;
};
export const readThreads = (
  document: string,
  cursor: string | null = null,
  filter?: ThreadFilter,
) => {
  const query = new URLSearchParams({ limit: '10' });
  if (cursor) query.set('cursor', cursor);
  if (filter) {
    query.set('context_kind', filter.kind);
    if (filter.context) query.set('context_id', filter.context);
    if (filter.version) query.set('content_version_id', filter.version);
  }
  return api<CommentThreadPage>(threadsPath(document) + '?' + query);
};
export const readComments = (thread: string, cursor: string | null = null, limit = 20) =>
  api<CommentPage>(
    commentsPath(thread) +
      '?limit=' +
      limit +
      (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
  );
export const readThread = (thread: string) =>
  api<CommentThread>('/v1/comment-threads/' + encodeURIComponent(thread));
export const mentionCandidates = (document: string, cursor: string | null = null) =>
  api<DocumentCollaboratorPage>(
    '/v1/documents/' + encodeURIComponent(document) + '/mention-candidates' + pageQuery(cursor),
  );
