export type PendingUpdate = {
  client_session_id: string;
  client_update_id: string;
  update_hash: string;
  update: string;
};
export type DraftQueue = {
  key: string;
  actor: string;
  document: string;
  unit: string;
  kind: 'BRANCH' | 'MAIN';
  tab: string;
  client: string;
  room: string;
  epoch: string;
  snapshot: string;
  entries: PendingUpdate[];
  updatedAt: number;
  retained?: boolean;
  archive?: boolean;
};
export type Partition = Pick<DraftQueue, 'actor' | 'document' | 'unit' | 'kind' | 'tab'>;
export const queueKey = (p: Partition) => [p.actor, p.document, p.unit, p.kind, p.tab].join(':');

export type DraftMessage = Record<string, any>;

// 送审只能交付已保存边界的身份；正文仍由 Node 固定，浏览器不自报正文。
export type SubmissionBoundary = {
  branch_id: string;
  expected_structure_id: string;
  flush_receipt_id: string;
};

export function belongs(q: DraftQueue, p: Partition) {
  return (
    q.actor === p.actor &&
    q.document === p.document &&
    q.unit === p.unit &&
    q.kind === p.kind &&
    q.tab === p.tab &&
    q.key === queueKey(p)
  );
}
