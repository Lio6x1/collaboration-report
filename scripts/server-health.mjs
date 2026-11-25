import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import net from 'node:net';
import { runtime, state } from './runtime-lib.mjs';
import { checkHTTPS } from './https-readiness.mjs';

// 真实检查每个角色的完整响应与本次启动标记；READY状态文件不能代替业务就绪。
export async function checkServer(owner, environment = state()) {
  const ca = readFileSync(resolve(runtime, 'pki/ca.pem'));
  await Promise.all(
    ['account', 'document', 'search', 'collaboration'].map((role) =>
      checkHTTPS(environment.ports[role + 'Http'], '/health/ready', owner, { ca }),
    ),
  );
  await checkHTTPS(environment.ports.edge, '/', owner, { ca });
  if (environment.developmentSMTP === true) {
    await new Promise((done, reject) => {
      const socket = net.connect(environment.ports.smtp, '127.0.0.1');
      let banner = '';
      socket.setTimeout(1000, () => socket.destroy(Error('SMTP_NOT_READY')));
      socket.on('data', (raw) => {
        banner += raw.toString('utf8');
        if (banner.length > 256) socket.destroy(Error('SMTP_OWNER_MISMATCH'));
        else if (banner.includes('\r\n')) {
          socket.end();
          banner.includes('collaboration-dev ' + owner)
            ? done()
            : reject(Error('SMTP_OWNER_MISMATCH'));
        }
      });
      socket.once('error', reject);
      socket.once('end', () => {
        if (!banner.includes('\r\n')) reject(Error('SMTP_NOT_READY'));
      });
    });
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    const startup = JSON.parse(readFileSync(resolve(runtime, 'startup.json')));
    if (startup.state !== 'READY' || !startup.owner) throw Error('SERVER_NOT_READY');
    await checkServer(startup.owner);
    console.log('SERVER_READY');
  } catch {
    console.error('SERVER_NOT_READY');
    process.exitCode = 1;
  }
}
