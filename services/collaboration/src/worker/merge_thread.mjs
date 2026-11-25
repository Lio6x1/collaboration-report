import { parentPort, workerData } from 'node:worker_threads';
import { mergeContent } from '../domain/content_merge.mjs';

// worker只做纯计算，不接触数据库或权限状态；终止计算不会留下未知提交。
try {
  parentPort.postMessage({ result: mergeContent(workerData) });
} catch {
  parentPort.postMessage({ error: 'MERGE_COMPUTE_INVALID' });
}
