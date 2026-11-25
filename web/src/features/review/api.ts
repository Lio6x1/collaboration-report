import { UserFacingError, api } from '../../lib/http';
import type {
  ReviewRequest,
  ReviewRequestPage,
  ReviewRoundPage,
  ChangeRequestPage,
  FixedContent,
} from '../../../../contracts/http/generated';

export const requestPath = (id: string) => '/v1/review-requests/' + id;
export const readRequest = (id: string) => api<ReviewRequest>(requestPath(id));
export const readReviewBase = (version: string) =>
  api<FixedContent>('/v1/content-versions/' + encodeURIComponent(version));
export const readRounds = (id: string, cursor?: string | null) =>
  api<ReviewRoundPage>(
    requestPath(id) + '/rounds?limit=20' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
  );
export const readChanges = (id: string, cursor?: string | null) =>
  api<ChangeRequestPage>(
    requestPath(id) +
      '/change-requests?limit=20' +
      (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
  );
export const readRequests = (document: string, cursor?: string | null, limit = 20) =>
  api<ReviewRequestPage>(
    '/v1/documents/' +
      document +
      '/review-requests?limit=' +
      limit +
      (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
  );

// 只定位默认对象，不预取各请求的正文；显示目录仍由独立的20条分页读取。
export async function findDefaultRequest(document: string, current: () => boolean) {
  let cursor: string | null = null,
    latest: ReviewRequest | null = null,
    active: ReviewRequest | null = null,
    reviewing: ReviewRequest | null = null;
  const visited = new Set<string>();
  // 每次最多读取20页、每页100条元数据；预算不足时不能把已扫描的末条冒充全局最新。
  for (let pages = 0; pages < 20; pages++) {
    if (!current()) return null;
    const page = await readRequests(document, cursor, 100);
    if (!current()) return null;
    // 服务端按请求ID升序返回；优先最新未归档审核中请求，其次未归档请求，再看历史。
    for (const request of page.items) {
      if (request.document_id !== document) throw new UserFacingError('审核请求不属于当前文档');
      latest = request;
      if (!request.archived_at) {
        active = request;
        if (request.status === 'REVIEWING') reviewing = request;
      }
    }
    cursor = page.next_cursor;
    if ((page.has_more && !cursor) || (cursor && visited.has(cursor)))
      throw new UserFacingError('审核请求分页尚未完成，请刷新或从列表选择审核请求');
    if (!cursor) return reviewing ?? active ?? latest;
    visited.add(cursor);
  }
  throw new UserFacingError('审核请求较多，尚未确定最新审核，请从分页列表选择请求或重试刷新');
}

// 丢弃时必须核验实际branch的完整请求列表；遗漏后页不能把可续接审核猜成全新工作。
export async function currentUnitRequest(document: string, unit: string, branch: string) {
  let cursor: string | null = null;
  let found: ReviewRequest | null = null;
  const visited = new Set<string>();
  do {
    const page = await readRequests(document, cursor, 100);
    for (const request of page.items) {
      if (request.unit_id !== unit || request.archived_at || request.status === 'MERGED') continue;
      if (request.branch_id !== branch || (found && found.id !== request.id))
        throw new UserFacingError('当前单元审核归属已变化，请刷新后重新核对');
      found = request;
    }
    cursor = page.next_cursor;
    if (cursor && visited.has(cursor)) throw new UserFacingError('审核分页尚未完成，请刷新后重试');
    if (cursor) visited.add(cursor);
  } while (cursor);
  return found;
}

// 请求列表不包含草稿正文。全部候选逐页检查，不能因可续接请求落在第二页就误发 NEW。
export async function submissionMode(document: string, unit: string) {
  let cursor: string | null = null;
  do {
    const page = await readRequests(document, cursor, 100);
    for (const request of page.items) {
      if (request.unit_id !== unit || request.archived_at || request.status === 'MERGED') continue;
      if (request.status === 'REVIEWING')
        throw new UserFacingError('本单元正在审核，请先处理当前轮次');
      return {
        mode: 'RESUBMIT' as const,
        request_id: request.id,
        expected_round_id: request.current_round_id,
      };
    }
    cursor = page.next_cursor;
  } while (cursor);
  return { mode: 'NEW' as const };
}
