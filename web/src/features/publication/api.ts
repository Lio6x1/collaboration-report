import type {
  Document,
  Publication,
  PublicDocument,
  Release,
  ReleaseCreate,
  ReleasePage,
} from '../../../../contracts/http/generated';
import { api } from '../../lib/http';
export const releasesPath = (document: string) => '/v1/documents/' + document + '/releases';
export const publicationPath = (document: string) => '/v1/documents/' + document + '/publication';
export function readReleases(document: string, cursor: string | null = null) {
  return api<ReleasePage>(
    releasesPath(document) + '?limit=20' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
  );
}
export const readFixedRelease = (document: string, release: string) =>
  api<PublicDocument>(releasesPath(document) + '/' + release);
// 操作编号由共享useAction首次生成；这里只构建生成契约中的固定业务输入，不另发编号。
export function releaseInput(document: Document): Omit<ReleaseCreate, 'operation_id'> {
  return {
    expected_publication_revision: document.publication_epoch,
    expected_control_revision: document.control_revision,
  };
}
export function publicationInput(
  document: Document,
  release?: Release,
): Omit<Publication, 'operation_id'> {
  return {
    expected_publication_revision: document.publication_epoch,
    action: release ? 'SWITCH' : 'UNPUBLISH',
    ...(release ? { release_id: release.id } : {}),
  };
}
