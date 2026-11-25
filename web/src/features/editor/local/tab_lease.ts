// 复制标签会复制sessionStorage；实际Web Lock隔离同账号的client/update队列，直到控制器退出才释放。
export async function acquireDraftTab(
  actor: string,
): Promise<{ tab: string; release: () => void }> {
  const key = 'collaboration-report.tab:' + actor;
  let tab = sessionStorage.getItem(key) ?? crypto.randomUUID();
  let acquired = false,
    releaseTab!: () => void;
  while (!acquired) {
    await new Promise<void>((done, reject) => {
      navigator.locks
        .request(
          'collaboration-report.tab:' + actor + ':' + tab,
          { ifAvailable: true },
          async (lock) => {
            if (!lock) {
              tab = crypto.randomUUID();
              sessionStorage.setItem(key, tab);
              done();
              return;
            }
            acquired = true;
            const held = new Promise<void>((release) => {
              releaseTab = release;
            });
            done();
            await held;
          },
        )
        .catch(reject);
    });
  }
  sessionStorage.setItem(key, tab);
  return { tab, release: releaseTab };
}
