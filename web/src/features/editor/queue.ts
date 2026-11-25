import { queueKey, belongs, type Partition, type DraftQueue } from './domain/model';
import { UserFacingError } from '../../lib/http';
export { queueKey } from './domain/model';
export type { Partition, DraftQueue, PendingUpdate } from './domain/model';

let database: Promise<IDBDatabase> | undefined;
function db() {
  return (database ??= new Promise<IDBDatabase>((done, reject) => {
    const request = indexedDB.open('collaboration-report.drafts', 1);
    request.onupgradeneeded = () => request.result.createObjectStore('queues', { keyPath: 'key' });
    request.onerror = () =>
      reject(new UserFacingError('浏览器无法保存本机草稿，请检查存储权限并备份当前内容'));
    request.onsuccess = () => {
      request.result.onversionchange = () => request.result.close();
      done(request.result);
    };
  }));
}

// IndexedDB事务提交完成才称“本机已保留”。账号/单元/工作区/标签共同分区，不能借新登录发送旧输入。
export async function readQueue(p: Partition): Promise<DraftQueue | null> {
  const connection = await db();
  return new Promise((done, reject) => {
    const transaction = connection.transaction('queues', 'readonly'),
      request = transaction.objectStore('queues').get(queueKey(p));
    request.onerror = () => reject(request.error);
    request.onsuccess = () => {
      const q = request.result as DraftQueue | undefined;
      done(q && belongs(q, p) ? q : null);
    };
  });
}
// sessionStorage随标签关闭而丢失，IndexedDB仍在。只列当前账号和目标的输入，不展示其他账号。
export async function retainedQueues(p: Omit<Partition, 'tab'>): Promise<DraftQueue[]> {
  const connection = await db();
  return new Promise((done, reject) => {
    const transaction = connection.transaction('queues', 'readonly'),
      request = transaction.objectStore('queues').openCursor(),
      found: DraftQueue[] = [];
    request.onerror = () => reject(request.error);
    request.onsuccess = () => {
      const cursor = request.result;
      if (!cursor) {
        done(found.sort((a, b) => b.updatedAt - a.updatedAt));
        return;
      }
      const q = cursor.value as DraftQueue;
      if (
        q.actor === p.actor &&
        q.document === p.document &&
        q.unit === p.unit &&
        q.kind === p.kind &&
        belongs(q, { ...p, tab: q.tab }) &&
        q.entries.length &&
        !q.retained
      )
        found.push(q);
      cursor.continue();
    };
  });
}
export async function changeQueue(
  p: Partition,
  change: (q: DraftQueue | null) => DraftQueue | null,
): Promise<DraftQueue | null> {
  const connection = await db();
  return new Promise((done, reject) => {
    const transaction = connection.transaction('queues', 'readwrite'),
      store = transaction.objectStore('queues'),
      request = store.get(queueKey(p));
    let value: DraftQueue | null = null,
      problem: unknown;
    request.onsuccess = () => {
      try {
        const old = request.result as DraftQueue | undefined;
        if (old && !belongs(old, p))
          throw new UserFacingError('本机草稿与当前账号或文档不一致，请重新打开编辑器');
        value = change(old ?? null);
        if (value) {
          if (
            !belongs(value, p) ||
            value.entries.length > 10000 ||
            value.snapshot.length > 24 * 1024 * 1024
          )
            throw new UserFacingError('本机草稿无法继续保存，请保留页面并复制当前内容备份');
          store.put(value);
        } else store.delete(queueKey(p));
      } catch (e) {
        problem = e;
        transaction.abort();
      }
    };
    transaction.oncomplete = () => done(value);
    transaction.onabort = () =>
      reject(
        problem ?? transaction.error ?? new UserFacingError('本机草稿尚未保留，请备份当前内容'),
      );
    transaction.onerror = () => {};
  });
}
export async function acknowledge(
  p: Partition,
  binding: {
    room: string;
    epoch: string;
    client_session_id: string;
    client_update_id: string;
    update_hash: string;
  },
) {
  return changeQueue(p, (q) => {
    if (!q) return null;
    if (
      q.room !== binding.room ||
      q.epoch !== binding.epoch ||
      q.client !== binding.client_session_id
    )
      throw new UserFacingError('编辑状态已变化，尚未保存的输入会继续保留，请重新打开编辑器');
    const found = q.entries.find((e) => e.client_update_id === binding.client_update_id);
    if (found && found.update_hash !== binding.update_hash)
      throw new UserFacingError('当前修改的保存结果尚未确认，请保留页面并重新尝试');
    q.entries = q.entries.filter((e) => e.client_update_id !== binding.client_update_id);
    return q;
  });
}
