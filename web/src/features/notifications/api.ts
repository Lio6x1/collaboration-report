import type { NotificationPage } from '../../../../contracts/http/generated';
import { api } from '../../lib/http';

export const notificationReadPath = (id: string) =>
  '/v1/notifications/' + encodeURIComponent(id) + '/read';

export function readNotifications(cursor: string | null = null) {
  return api<NotificationPage>(
    '/v1/notifications?limit=10' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
  );
}
