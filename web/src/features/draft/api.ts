import type { FixedContent } from '../../../../contracts/http/generated';
import { api } from '../../lib/http';

export const pullMainPath = (document: string, unit: string) =>
  `/v1/documents/${document}/units/${unit}/pull-main`;
export const discardPreviewPath = (document: string, unit: string) =>
  `/v1/documents/${document}/units/${unit}/discard-previews`;
export const discardDraftPath = (document: string, unit: string) =>
  `/v1/documents/${document}/units/${unit}/discard-draft`;
export const readFixedContent = (id: string) => api<FixedContent>('/v1/content-versions/' + id);
