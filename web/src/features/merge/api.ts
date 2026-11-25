import { UserFacingError, api } from '../../lib/http';
import type {
  ChangeRequest,
  ConflictContext,
  ReviewRequest,
} from '../../../../contracts/http/generated';
import { readChanges, readRequest, requestPath } from '../review/api';

export const approvePath = (request: string) => requestPath(request) + '/approve-and-merge';
export const readConflict = (ref: string) =>
  api<ConflictContext>('/v1/conflict-contexts/' + encodeURIComponent(ref));

// 资格来自服务器，逐页读取只为了把本次所有拟解除项展示给人；不能将第一页当作完整集合。
export async function ownMergeOpinions(expected: ReviewRequest, actor: string) {
  const values: ChangeRequest[] = [],
    visited = new Set<string>();
  let cursor: string | null = null;
  do {
    const page = await readChanges(expected.id, cursor);
    values.push(...page.items);
    cursor = page.next_cursor;
    if (cursor && visited.has(cursor))
      throw new UserFacingError('修改要求分页尚未完成，请刷新后重新核对');
    if (cursor) visited.add(cursor);
  } while (cursor);
  // 分页期间并发意见/换轮次不能把原页面静默升级成新的批准对象。
  const current = await readRequest(expected.id);
  if (
    current.revision !== expected.revision ||
    current.current_round_id !== expected.current_round_id ||
    current.current_round?.content_version_id !== expected.current_round?.content_version_id ||
    !current.actions.includes('APPROVE_AND_MERGE')
  )
    throw new UserFacingError('审核资格或固定轮次已变化，请刷新后重新核对');
  const open = values.filter((change) => change.status === 'OPEN');
  if (
    open.some((change) => change.author_id !== actor) ||
    open.length > 100 ||
    (current.blocking_change_count !== undefined && current.blocking_change_count !== open.length)
  )
    throw new UserFacingError('完整修改要求尚不满足本人一并解除条件，请先逐条处理并刷新');
  return open;
}
