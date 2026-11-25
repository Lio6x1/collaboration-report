import https from 'node:https';

// 监督器要求原owner的完整响应；收到响应头不代表输出结束，也不能借持续字节延长预算。
export function checkHTTPS(port, path, owner, { ca, timeoutMs = 2000 }) {
  return new Promise((resolve, reject) => {
    let settled = false;
    let deadline;
    const finish = (error) => {
      if (settled) return;
      settled = true;
      clearTimeout(deadline);
      if (error) {
        req.destroy();
        reject(error);
      } else resolve();
    };
    const req = https.get({ hostname: 'localhost', port, path, ca }, (response) => {
      const incomplete = () => finish(Error('PROBE_INCOMPLETE_BODY'));
      response.once('aborted', incomplete);
      response.once('error', (error) => finish(error));
      response.once('close', () => {
        if (!response.complete) incomplete();
      });
      response.once('end', () => {
        if (!response.complete) incomplete();
        else if (response.statusCode !== 200 || response.headers['x-dev-run-owner'] !== owner)
          finish(Error('NOT_READY'));
        else finish();
      });
      response.resume();
    });
    req.once('error', (error) => finish(error));
    // 从请求创建起覆盖DNS、TCP、TLS和全部响应；超时只表示未就绪，不证明服务已退出。
    deadline = setTimeout(() => finish(Error('PROBE_TIMEOUT')), timeoutMs);
  });
}
