import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from '../../scripts/runtime-lib.mjs';

// 此入口明确要求正在运行的原 Doc 配置为 OSS。复用同文档完整业务流程，
// 真实经过上传/版本资源引用/发布读取/导出/回收恢复/最后GC，不复制历史矩阵。
const cfg = JSON.parse(readFileSync(resolve(runtime, 'services/document.json')));
assert.equal(cfg.role, 'document');
assert.equal(
  cfg.resources?.oss?.bucket,
  'collaboration-report',
  'OSS_PRODUCT_CONFIGURATION_REQUIRED',
);
assert.equal(cfg.resources.oss.region, 'cn-shenzhen');
assert.ok(
  cfg.resources.oss.prefix.startsWith('collabreport-tests/'),
  'OSS_PRODUCT_TEST_PREFIX_REQUIRED',
);
await import('./full-workflow.test.mjs');
