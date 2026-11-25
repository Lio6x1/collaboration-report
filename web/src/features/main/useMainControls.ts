import type { Document } from '../../../../contracts/http/generated';
import { useAction } from '../../lib/http';
import { mainLockPath } from './api';

export function useMainControls(
  current: () => Document,
  reload: () => Promise<void>,
  prepareLock: () => Promise<unknown>,
) {
  const action = useAction();
  async function changeLock() {
    const target = current();
    let prepared = target.main_locked;
    await action.run(
      async () => {
        if (!prepared) {
          // 当前MAIN的IME/待发队列先沿编辑器原FLUSH固定；未知控制重试不能再采一个新切点。
          await prepareLock();
          prepared = true;
        }
        // useAction固定首次参数与原号；安装/刷新失败续接时不能改用后来读到的control_revision。
        await action.mutate(mainLockPath(target.id), {
          expected_control_revision: target.control_revision,
          locked: !target.main_locked,
        });
        await reload();
      },
      target.main_locked ? '主线已解锁，可以编辑' : '可靠主线已固定并锁定',
    );
  }
  return { action, changeLock };
}
