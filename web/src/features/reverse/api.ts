import { UserFacingError, api } from '../../lib/http';
import type {
  Document,
  ReviewAction,
  ReviewActionPage,
  ReviewRequest,
} from '../../../../contracts/http/generated';
import { readRequest, requestPath } from '../review/api';

export const reversePath = (document: string) => '/v1/documents/' + document + '/reverse-changes';

export async function reverseSource(expected: ReviewRequest) {
  const round = expected.current_round;
  if (expected.status !== 'MERGED' || !round || expected.archived_at)
    throw new UserFacingError('请选择当前可读的已合并审核');
  let cursor: string | null = null;
  let approval: ReviewAction | null = null;
  const visited = new Set<string>();
  do {
    const page: ReviewActionPage = await api<ReviewActionPage>(
      requestPath(expected.id) +
        '/actions?limit=20' +
        (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
    );
    for (const value of page.items) {
      if (value.request_id !== expected.id) throw new UserFacingError('审核动作归属已变化，请刷新');
      if (value.type !== 'APPROVE_AND_MERGE' || value.round_id !== expected.current_round_id)
        continue;
      if (
        !value.merge_record_id ||
        value.content_version_id !== round.content_version_id ||
        approval
      )
        throw new UserFacingError('原批准记录尚未核验，请刷新');
      approval = value;
    }
    cursor = page.next_cursor;
    if (cursor && visited.has(cursor))
      throw new UserFacingError('审核动作分页尚未完成，请重新核对');
    if (cursor) visited.add(cursor);
  } while (cursor);
  if (!approval) throw new UserFacingError('尚未找到本轮实际批准记录，请刷新');
  // 动作页是历史事实，仍须重验当前请求。期望清单来自业务Document，不能用每次物化的READ_VIEW。
  const [current, document] = await Promise.all([
    readRequest(expected.id),
    api<Document>('/v1/documents/' + expected.document_id),
  ]);
  if (
    current.document_id !== expected.document_id ||
    current.unit_id !== expected.unit_id ||
    current.branch_id !== expected.branch_id ||
    current.status !== 'MERGED' ||
    current.archived_at ||
    current.revision !== expected.revision ||
    current.current_round_id !== expected.current_round_id ||
    current.current_round?.content_version_id !== round.content_version_id ||
    document.id !== expected.document_id
  )
    throw new UserFacingError('原审核或当前文档已变化，请刷新后核对');
  return { approval, manifest: document.current_manifest_id };
}
