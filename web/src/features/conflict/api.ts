import { UserFacingError } from '../../lib/http';
import type { ConflictContext, ReviewRequest } from '../../../../contracts/http/generated';
import { readConflict } from '../merge/api';
import { readRequest } from '../review/api';

export const resolutionPath = (ref: string) =>
  '/v1/conflict-contexts/' + encodeURIComponent(ref) + '/resolutions';
export { readConflict };

export async function resolutionRequest(context: ConflictContext): Promise<ReviewRequest> {
  const current = await readRequest(context.request_id);
  if (
    current.document_id !== context.document_id ||
    current.unit_id !== context.unit_id ||
    current.current_round_id !== context.round_id ||
    current.archived_at ||
    current.status === 'MERGED'
  )
    throw new UserFacingError('原修复轮次或范围已变化，请保留候选并核对当前审核');
  return current;
}
