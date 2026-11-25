import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { spawn } from 'node:child_process';
import { root, runtime } from './runtime-lib.mjs';
import { searchPageScaleFixture } from '../tests/helpers/search-page-scale.mjs';

// 只测SQL/RPC阶段，不替代真实HTTP/Edge验收。样本和清理由同一有界helper
// 绑定当前owner；Go诊断事务回滚，不制造输出租约、关闭任务或ACK。
const fixture = await searchPageScaleFixture((facts) => console.log(JSON.stringify(facts)));
try {
  writeFileSync(
    resolve(runtime, 'search-page-scale-profile.json'),
    JSON.stringify({ query: fixture.q }),
  );
  const child = spawn(
    'D:/go1.25.10/bin/go.exe',
    [
      '-C',
      resolve(root, 'services/document'),
      'test',
      '-mod=readonly',
      '-tags=search_profile',
      '-p=1',
      '-count=1',
      '-timeout=30s',
      '-v',
      './internal/logic/document',
      '-run',
      '^TestActualSearchPageScalePhaseProfile$',
    ],
    {
      cwd: root,
      windowsHide: true,
      env: {
        ...process.env,
        GOWORK: 'off',
        GOCACHE: resolve(runtime, 'go-cache'),
        COLLAB_TEST_DOCUMENT_CONFIG: resolve(runtime, 'services/document.json'),
      },
      stdio: 'inherit',
    },
  );
  process.exitCode = await new Promise((done, reject) => {
    child.once('error', reject);
    child.once('close', (code) => done(code ?? 1));
  });
} finally {
  await fixture.close();
}
