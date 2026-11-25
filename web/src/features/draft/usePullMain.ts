import type { PullMain } from '../../../../contracts/http/generated';
import { useAction } from '../../lib/http';
import { pullMainPath } from './api';

type PullBoundary = Omit<PullMain, 'operation_id'>;

export function usePullMain(
  document: string,
  unit: string,
  prepare: () => Promise<PullBoundary>,
  reload: () => Promise<void>,
) {
  const action = useAction();
  async function pull() {
    let fixed: PullBoundary | undefined;
    await action.run(async () => {
      // 首次保存后固定原branch/lineage/structure；未知响应续接不再次FLUSH或采新谱系。
      fixed ??= await prepare();
      await action.mutate(pullMainPath(document, unit), fixed);
      // mutate同时等待实际新代次安装READY；原号终态确认后才刷新，读取未知仍保留同号恢复。
      await reload();
    }, '主线已拉入，新共同草稿已安装');
  }
  return { action, pull };
}
